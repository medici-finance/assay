package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// socket.go — the within-cell loopback transport: a Unix-domain socket server
// matching cmd/deskcomms/gateway.go's client wire shape FIELD-FOR-FIELD (the
// two are bound only by JSON tags, never by importing one `package main` from
// another). Within-cell sends traverse the SAME PreCheck pipeline cross-cell
// ones do (a2a.go) — ONE enforcement point.
//
// PEER AUTH ON THIS TRANSPORT. A Unix-domain socket is already a local-process
// trust boundary (filesystem permissions on the socket path gate who may
// dial it) — there is no second, per-connection credential to check the way
// mTLS supplies one for the network path, so PreCheckInput.PeerAuthenticated
// is always true here. The socket path itself (0700, owned by the gateway's
// operating user) IS this transport's access control.

// gwRequest / gwResponse / socketReceipt / socketNotice mirror
// cmd/deskcomms/gateway.go's gwRequest / gwResponse / Receipt / Notice.
type gwRequest struct {
	Op      string          `json:"op"`
	Cell    string          `json:"cell,omitempty"`
	Role    string          `json:"role,omitempty"`
	ID      string          `json:"id,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

type gwResponse struct {
	Receipt *socketReceipt `json:"receipt,omitempty"`
	Notices []socketNotice `json:"notices,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type socketReceipt struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	Detail   string `json:"detail,omitempty"`
}

type socketNotice struct {
	ID      string          `json:"id"`
	From    comms.SenderID  `json:"from"`
	Verb    string          `json:"verb"`
	Class   string          `json:"class"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Sent    time.Time       `json:"sent"`
}

// SocketServer serves the within-cell loopback protocol over a Unix-domain
// socket, dispatching accepted "submit"s through the same PreCheck pipeline
// a2a.go's cross-cell path uses.
type SocketServer struct {
	Root    string
	Deps    PreCheckDeps
	Emitter InboxEmitter
	Filer   IssueFiler
	Now     func() time.Time
}

func (s SocketServer) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// ListenAndServe binds the Unix socket at path (removing a stale socket file
// first — a leftover file from a crashed prior run must never make a fresh
// bind silently fail) and serves connections until the listener is closed.
func (s SocketServer) ListenAndServe(path string) error {
	_ = os.Remove(path) // stale socket from a prior crash; ignore "not exist".
	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("commsgw: cannot bind socket %s: %w", path, err)
	}
	defer ln.Close()
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("commsgw: cannot chmod socket %s: %w", path, err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s SocketServer) handle(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), comms.MaxEnvelopeBytes+4096)
	if !sc.Scan() {
		return
	}
	var req gwRequest
	resp := gwResponse{}
	if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
		resp.Error = fmt.Sprintf("commsgw: undecodable request: %v", err)
		writeLine(conn, resp)
		return
	}
	switch req.Op {
	case "submit":
		resp = s.handleSubmit(req)
	case "poll":
		resp = s.handlePoll(req)
	case "ack":
		resp = s.handleAck(req)
	default:
		resp.Error = fmt.Sprintf("commsgw: unknown op %q", req.Op)
	}
	writeLine(conn, resp)
}

func (s SocketServer) handleSubmit(req gwRequest) gwResponse {
	now := s.clock()
	env, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: []byte(req.Message), Now: now}, s.Deps)
	if err != nil {
		return gwResponse{Receipt: &socketReceipt{Accepted: false, Detail: err.Error()}}
	}
	if err := WriteAccepted(s.Root, *env, now); err != nil {
		return gwResponse{Receipt: &socketReceipt{ID: env.ID, Accepted: false,
			Detail: fmt.Sprintf("commsgw: accepted-queue write failed: %v", err)}}
	}
	_ = EmitCrossCellInboxItem(s.Root, env, now, s.Emitter, s.Filer) // no-op for within-cell (IsCrossCell false)
	return gwResponse{Receipt: &socketReceipt{ID: env.ID, Accepted: true}}
}

func (s SocketServer) handlePoll(req gwRequest) gwResponse {
	if req.Cell == "" || req.Role == "" {
		return gwResponse{Error: "commsgw: poll needs cell and role"}
	}
	notices, err := PollMailbox(s.Root, req.Cell, req.Role)
	if err != nil {
		return gwResponse{Error: err.Error()}
	}
	out := make([]socketNotice, 0, len(notices))
	for _, n := range notices {
		out = append(out, socketNotice{ID: n.ID, From: n.From, Verb: n.Verb, Class: n.Class, Payload: n.Payload, Sent: n.Sent})
	}
	return gwResponse{Notices: out}
}

func (s SocketServer) handleAck(req gwRequest) gwResponse {
	if req.Cell == "" || req.Role == "" || req.ID == "" {
		return gwResponse{Error: "commsgw: ack needs cell, role and id"}
	}
	if err := AckMailbox(s.Root, req.Cell, req.Role, req.ID); err != nil {
		return gwResponse{Error: err.Error()}
	}
	return gwResponse{}
}

func writeLine(conn net.Conn, resp gwResponse) {
	raw, err := json.Marshal(resp)
	if err != nil {
		raw = []byte(`{"error":"commsgw: response encoding failed"}`)
	}
	_, _ = conn.Write(append(raw, '\n'))
}
