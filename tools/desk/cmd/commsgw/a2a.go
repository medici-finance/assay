package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"iter"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// a2a.go — the cross-cell wire transport.
//
// PINNED SDK (a recorded design decision, implement, do not redecide):
// github.com/a2aproject/a2a-go/v2 @ v2.5.0. THIN BY DESIGN ("a thin house
// commsgw ... deskkit-integrated, the recommended start"): this file's
// AgentExecutor does no A2A task-lifecycle bookkeeping. A cellmsg-v1 envelope
// arrives as ONE raw Part on ONE Message; the gateway runs it through the SAME
// PreCheck pipeline the within-cell loopback path uses (socket.go) — ONE
// enforcement point — and replies with a single terminal a2a.Message
// (accept/refuse). There is no multi-turn conversation in this protocol:
// every cell message is one bounded exchange.
//
// mTLS IS THE TRANSPORT LAYER, not application code. tls.Config{ClientAuth:
// tls.RequireAndVerifyClientCert, ClientCAs: <house trust store>} refuses an
// unauthenticated peer's TLS HANDSHAKE — a different signal, at a different
// layer, than any content check (this gateway's single-point-of-failure map:
// "peer auth refuses before the pipeline on a different signal"). By the time
// GatewayAgent.Execute runs, the connection has already survived that
// handshake, so PreCheckInput.PeerAuthenticated is always true here — the flag
// exists in the pipeline for the within-cell socket path (no TLS layer at all,
// see socket.go), and its false branch is pinned directly in precheck_test.go's
// TestPeerAuth rather than by attempting to make the TLS layer itself lie.

// GatewayAgent is the a2asrv.AgentExecutor fronting the pre-check pipeline for
// cross-cell A2A traffic. It implements exactly Execute/Cancel — nothing else
// about A2A's task model (streaming artifacts, push notifications, task
// stores) is this brief's concern; NewHandler's defaults (an in-memory task
// store) are unused because every reply here is a single terminal Message.
type GatewayAgent struct {
	Root    string
	Deps    PreCheckDeps
	Emitter InboxEmitter
	Filer   IssueFiler
	// Now is a test seam; nil means time.Now (UTC).
	Now func() time.Time
}

func (g GatewayAgent) clock() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// wireReply is the JSON body of the single terminal reply Message —
// field-compatible with cmd/deskcomms/gateway.go's Receipt (Accepted/Detail),
// plus the accepted envelope's id, so a cross-cell peer's own gateway (running
// this same binary) reads a familiar shape.
type wireReply struct {
	ID       string `json:"id,omitempty"`
	Accepted bool   `json:"accepted"`
	Detail   string `json:"detail,omitempty"`
}

// Execute implements a2asrv.AgentExecutor. Every path yields EXACTLY one
// terminal a2a.Message (accept or refuse) — per a2asrv's contract, an
// a2a.Message with any payload stops event processing, so no task is ever
// created for this thin per-message exchange.
func (g GatewayAgent) Execute(_ context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		raw, err := rawEnvelopeBytes(ec)
		if err != nil {
			yield(replyMessage(wireReply{Accepted: false, Detail: err.Error()}), nil)
			return
		}

		now := g.clock()
		env, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: now}, g.Deps)
		if err != nil {
			yield(replyMessage(wireReply{Accepted: false, Detail: err.Error()}), nil)
			return
		}

		if err := WriteAccepted(g.Root, *env, now); err != nil {
			yield(replyMessage(wireReply{ID: env.ID, Accepted: false,
				Detail: fmt.Sprintf("commsgw: accepted-queue write failed: %v", err)}), nil)
			return
		}

		// Inbox emission failure quarantines the message (Verify row 14) but the
		// message is ALREADY durably accepted+queued at this point, so the reply
		// to the sender is still ACCEPTED — the failure is a downstream
		// visibility problem this gateway resolves by holding a quarantine copy
		// and filing an issue, not by un-accepting a message it already
		// committed to deliver.
		_ = EmitCrossCellInboxItem(g.Root, env, now, g.Emitter, g.Filer)

		yield(replyMessage(wireReply{ID: env.ID, Accepted: true}), nil)
	}
}

// Cancel implements a2asrv.AgentExecutor. There is no long-running task to
// cancel — every exchange already completed synchronously in Execute — so
// Cancel yields nothing.
func (g GatewayAgent) Cancel(context.Context, *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(func(a2a.Event, error) bool) {}
}

func replyMessage(r wireReply) *a2a.Message {
	raw, err := json.Marshal(r)
	if err != nil {
		raw = []byte(`{"accepted":false,"detail":"commsgw: reply encoding failed"}`)
	}
	return a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewRawPart(raw))
}

// rawEnvelopeBytes extracts the raw cellmsg-v1 wire bytes from the inbound A2A
// message. It refuses (rather than guesses at) a message with no parts, or
// whose first part is not a raw/data part — a malformed A2A carrier is refused
// exactly like a malformed envelope, before PreCheck's own parse stage ever
// runs.
func rawEnvelopeBytes(ec *a2asrv.ExecutorContext) ([]byte, error) {
	if ec == nil || ec.Message == nil || len(ec.Message.Parts) == 0 {
		return nil, fmt.Errorf("commsgw: A2A message carries no parts")
	}
	p := ec.Message.Parts[0]
	if raw := p.Raw(); len(raw) > 0 {
		return raw, nil
	}
	if d := p.Data(); d != nil {
		raw, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("commsgw: A2A data part does not re-encode: %w", err)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("commsgw: A2A message's first part carries neither raw nor data content")
}

// AgentCard is the minimal, honest self-description this gateway serves at
// a2asrv.WellKnownAgentCardPath. It advertises no skills a human would pick
// from a marketplace — this agent is a house-internal message gateway, not a
// conversational one — but the well-known path is part of the pinned
// transport contract, so it is served rather than 404ing.
func AgentCard(cell, listenAddr string) *a2a.AgentCard {
	return &a2a.AgentCard{
		Name:        "commsgw@" + cell,
		Description: "House cell-gateway inbound: deterministic pre-checks + drain consumer. Not a conversational agent.",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface("https://"+listenAddr+"/invoke", a2a.TransportProtocolJSONRPC),
		},
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"application/json"},
	}
}

// NewA2AServer builds the cross-cell mTLS-fronted A2A JSON-RPC server. It does
// not start listening — callers Serve() the returned *http.Server over a
// net.Listener wrapped in ServeTLS, or use ListenAndServeTLS below.
func NewA2AServer(cfg Config, agent GatewayAgent) (*http.Server, error) {
	tlsCfg, err := MutualTLSConfig(cfg.TLSCertPath, cfg.TLSKeyPath, cfg.ClientCAPath)
	if err != nil {
		return nil, err
	}
	handler := a2asrv.NewHandler(agent)
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(handler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(AgentCard(cfg.Cell, cfg.Listen)))
	return &http.Server{
		Addr:         cfg.Listen,
		Handler:      mux,
		TLSConfig:    tlsCfg,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, nil
}

// MutualTLSConfig builds the mTLS server config: this gateway's own identity
// (certPath/keyPath) and the house client-CA trust store (clientCAPath) that
// verifies-or-refuses every peer gateway's client certificate.
// ClientAuth: RequireAndVerifyClientCert — a connection presenting no
// certificate, or one that does not chain to clientCAPath, never completes the
// handshake, and this gateway's own handler code never runs for it.
func MutualTLSConfig(certPath, keyPath, clientCAPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("commsgw: cannot load gateway TLS identity (%s, %s): %w", certPath, keyPath, err)
	}
	caPEM, err := os.ReadFile(clientCAPath)
	if err != nil {
		return nil, fmt.Errorf("commsgw: cannot read client CA trust store %s: %w", clientCAPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("commsgw: client CA trust store %s contains no usable certificates", clientCAPath)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// ListenAndServeA2A binds cfg.Listen and serves the mTLS A2A server until ctx
// is done or an unrecoverable listener error occurs.
func ListenAndServeA2A(ctx context.Context, cfg Config, agent GatewayAgent) error {
	srv, err := NewA2AServer(cfg, agent)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("commsgw: cannot bind A2A listen address %s: %w", cfg.Listen, err)
	}
	tlsLn := tls.NewListener(ln, srv.TLSConfig)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(tlsLn) }()
	select {
	case <-ctx.Done():
		_ = srv.Close()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
