package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// gateway.go — the local cell gateway's loopback CLIENT surface, and the two
// value shapes it exchanges. deskcomms is a CLIENT: it hands a fully-formed,
// signed envelope to the gateway and reads that gateway's per-role mailbox.
//
// ENFORCEMENT IS THE GATEWAY'S. The preflight in send.go is a fail-fast
// convenience that calls the SAME internal/comms parse/ACL the gateway re-runs
// authoritatively; the transport here only carries bytes. There is deliberately
// NO local-spool fallback: an unreachable gateway is a refusal (fail closed), so
// that a submission is never silently queued on disk and reported as delivered.

// ErrGatewayUnreachable — the local cell gateway could not be reached (unset
// address, dial failure, or a transport error mid-exchange). It is COULD-NOT-SUBMIT:
// the message's fate is unknown, so the verb fails closed rather than writing any
// local fallback that would fabricate a delivery the gateway never accepted.
var ErrGatewayUnreachable = errors.New("deskcomms: local cell gateway is unreachable")

// Receipt is the gateway's acknowledgement of an accepted submission. Accepted is
// the gateway's authoritative yes/no; a client never treats "I sent the bytes" as
// "the gateway accepted them".
type Receipt struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	// Detail carries the gateway's own refusal reason when Accepted is false, so a
	// gateway-side refusal is reported with its cause rather than a bare "rejected".
	Detail string `json:"detail,omitempty"`
}

// Notice is one message the gateway holds in a role's mailbox. It is the read-side
// shape poll returns and ack references by ID. Payload is carried opaquely — the
// read side does not re-interpret a body the gateway already validated on ingress.
type Notice struct {
	ID      string          `json:"id"`
	From    comms.SenderID  `json:"from"`
	Verb    string          `json:"verb"`
	Class   string          `json:"class"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Sent    time.Time       `json:"sent"`
}

// Gateway is the local cell gateway's loopback client API. It is an interface so
// the network is never in the way of a test: a unit test injects a fake that
// records the bytes it was handed, and main.go wires the real socket client.
type Gateway interface {
	// Submit hands one fully-formed, signed cellmsg-v1 envelope to the gateway and
	// returns its Receipt. A transport failure is ErrGatewayUnreachable.
	Submit(raw []byte) (Receipt, error)
	// Poll returns the notices the gateway holds in (cell, role)'s mailbox.
	Poll(cell, role string) ([]Notice, error)
	// Ack acknowledges the notice id in (cell, role)'s mailbox. Acknowledgement
	// MOVES a notice (to an acked partition), it never deletes — the durability of
	// that move is the gateway's, not the client's.
	Ack(cell, role, id string) error
}

// gwRequest is the one request shape the loopback transport writes. op selects the
// operation; the other fields are populated per op. It is a closed shape so an
// unknown field is never sent on a guess.
type gwRequest struct {
	Op      string          `json:"op"`
	Cell    string          `json:"cell,omitempty"`
	Role    string          `json:"role,omitempty"`
	ID      string          `json:"id,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// gwResponse is the one response shape the transport reads. Exactly one of the
// populated fields is meaningful per op; Error, when non-empty, is the gateway's
// refusal reason.
type gwResponse struct {
	Receipt *Receipt `json:"receipt,omitempty"`
	Notices []Notice `json:"notices,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// socketGateway is the real loopback client: it dials a Unix-domain socket the
// local gateway listens on, writes one JSON request line, and reads one JSON
// response line. The address is operator config (see client.go); this type holds
// no default endpoint, because a guessed endpoint is a message sent somewhere
// nobody is listening.
type socketGateway struct {
	network string        // "unix" (loopback only)
	addr    string        // socket path
	timeout time.Duration // per-exchange dial+IO budget
}

// dialTimeout bounds a single request/response exchange. A gateway that does not
// answer inside it is treated as unreachable — a hung dial must not hang the desk.
const dialTimeout = 5 * time.Second

func (g socketGateway) exchange(req gwRequest) (gwResponse, error) {
	if g.addr == "" {
		return gwResponse{}, fmt.Errorf("%w: no gateway address configured", ErrGatewayUnreachable)
	}
	to := g.timeout
	if to <= 0 {
		to = dialTimeout
	}
	conn, err := net.DialTimeout(g.network, g.addr, to)
	if err != nil {
		// A dial failure is unreachable, full stop — never a fallback to a local
		// queue. This is the fail-closed branch the GatewayDown test pins.
		return gwResponse{}, fmt.Errorf("%w: %v", ErrGatewayUnreachable, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(to))

	line, err := json.Marshal(req)
	if err != nil {
		return gwResponse{}, fmt.Errorf("%w: encoding request: %v", ErrGatewayUnreachable, err)
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return gwResponse{}, fmt.Errorf("%w: writing request: %v", ErrGatewayUnreachable, err)
	}

	var resp gwResponse
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), comms.MaxEnvelopeBytes)
	if !sc.Scan() {
		if serr := sc.Err(); serr != nil {
			return gwResponse{}, fmt.Errorf("%w: reading response: %v", ErrGatewayUnreachable, serr)
		}
		return gwResponse{}, fmt.Errorf("%w: gateway closed the connection with no response", ErrGatewayUnreachable)
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return gwResponse{}, fmt.Errorf("%w: undecodable response: %v", ErrGatewayUnreachable, err)
	}
	return resp, nil
}

func (g socketGateway) Submit(raw []byte) (Receipt, error) {
	resp, err := g.exchange(gwRequest{Op: "submit", Message: json.RawMessage(raw)})
	if err != nil {
		return Receipt{}, err
	}
	if resp.Error != "" {
		return Receipt{}, fmt.Errorf("gateway refused: %s", resp.Error)
	}
	if resp.Receipt == nil {
		return Receipt{}, fmt.Errorf("%w: gateway returned no receipt", ErrGatewayUnreachable)
	}
	return *resp.Receipt, nil
}

func (g socketGateway) Poll(cell, role string) ([]Notice, error) {
	resp, err := g.exchange(gwRequest{Op: "poll", Cell: cell, Role: role})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("gateway refused: %s", resp.Error)
	}
	return resp.Notices, nil
}

func (g socketGateway) Ack(cell, role, id string) error {
	resp, err := g.exchange(gwRequest{Op: "ack", Cell: cell, Role: role, ID: id})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("gateway refused: %s", resp.Error)
	}
	return nil
}
