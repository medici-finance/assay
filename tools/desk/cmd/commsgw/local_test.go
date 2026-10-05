package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

func TestLocalOnlyConfigRequiresExplicitOptIn(t *testing.T) {
	env := map[string]string{EnvEnable: "1", EnvCell: "cell-a", EnvQueueDir: "queue", EnvSocket: "endpoint", EnvTrustStore: "trust"}
	get := func(k string) string { return env[k] }
	if _, err := LoadConfig(get); err == nil {
		t.Fatal("missing TLS accepted without local-only opt-in")
	}
	env["ASSAY_COMMS_LOCAL_ONLY"] = "1"
	if cfg, err := LoadConfig(get); err != nil || !cfg.LocalOnly {
		t.Fatalf("local mode: %+v %v", cfg, err)
	}
	env["ASSAY_COMMS_LOCAL_ONLY"] = "true"
	if _, err := LoadConfig(get); err == nil {
		t.Fatal("unknown mode accepted")
	}
	env["ASSAY_COMMS_LOCAL_ONLY"] = "1"
	delete(env, EnvTrustStore)
	if _, err := LoadConfig(get); err == nil {
		t.Fatal("local mode bypassed signing trust")
	}
}

func TestLocalGatewayQueuePollAck(t *testing.T) {
	g := newBypassGateway(t)
	s := g.server()
	s.Cell = "cell-a"
	s.LocalOnly = true
	endpoint := startSocketServer(t, s)
	raw := g.signedEnvelope(t, "local-roundtrip", "cell-a", "the-desk", "cell-a", "worker-desk", "notify", `{"kind":"advise","text":"Inspect the queue"}`)
	response := socketSubmit(t, endpoint, raw)
	if response.Receipt == nil || !response.Receipt.Accepted {
		t.Fatalf("submit: %+v", response)
	}
	items, err := commsqueue.ListAccepted(g.root)
	if err != nil || len(items) != 1 {
		t.Fatalf("durable acceptance: %+v %v", items, err)
	}
	// Exercise the same shared mailbox writer used by commsloop.Dispatch.
	// Router decisions are covered separately in cmd/commsloop's tests.
	e := items[0].Envelope
	n := commsqueue.Notice{ID: e.ID, From: e.From, Verb: e.Verb, Class: e.Class, Payload: e.Payload, Sent: e.Sent}
	if err := commsqueue.DeliverToMailbox(g.root, e.To.Cell, e.To.Role, n); err != nil {
		t.Fatal(err)
	}
	exchange := func(req gwRequest) gwResponse {
		t.Helper()
		b, _ := json.Marshal(req)
		reply := socketRawLine(t, endpoint, string(b)+"\n")
		var out gwResponse
		if err := json.Unmarshal([]byte(reply), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	poll := gwRequest{Op: "poll", Cell: e.To.Cell, Role: e.To.Role}
	got := exchange(poll)
	if got.Error != "" || len(got.Notices) != 1 {
		t.Fatalf("poll: %+v", got)
	}
	var wantPayload, gotPayload any
	if err := json.Unmarshal(e.Payload, &wantPayload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got.Notices[0].Payload, &gotPayload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantPayload, gotPayload) {
		t.Fatalf("payload changed: %v", gotPayload)
	}
	if again := exchange(poll); len(again.Notices) != 1 {
		t.Fatal("poll consumed the message")
	}
	if got := exchange(gwRequest{Op: "ack", Cell: e.To.Cell, Role: e.To.Role, ID: e.ID}); got.Error != "" {
		t.Fatal(got.Error)
	}
	if got := exchange(poll); len(got.Notices) != 0 {
		t.Fatal("acked message remained visible")
	}
	if ok, err := commsqueue.IsAcked(g.root, e.To.Cell, e.To.Role, e.ID); err != nil || !ok {
		t.Fatalf("ack not durable: %v", err)
	}
	foreign := g.signedEnvelope(t, "foreign", "cell-a", "the-desk", "cell-b", "the-desk", "notify", `{"kind":"advise"}`)
	if got := socketSubmit(t, endpoint, foreign); got.Receipt == nil || got.Receipt.Accepted {
		t.Fatalf("foreign submit: %+v", got)
	}
	if got := exchange(gwRequest{Op: "poll", Cell: "cell-b", Role: "the-desk"}); got.Error == "" {
		t.Fatal("foreign poll accepted")
	}
	if got := exchange(gwRequest{Op: "ack", Cell: "cell-b", Role: "the-desk", ID: e.ID}); got.Error == "" {
		t.Fatal("foreign ack accepted")
	}
}

// The local-only branches are tested against requests the pre-existing
// pipeline accepts: each control runs the same request on a gateway without
// local-only mode and must succeed, so only the new branch can refuse it.
func TestLocalOnlyRefusesCrossCellSubmit(t *testing.T) {
	legal := func(g *bypassGateway, id string) []byte {
		return g.signedEnvelope(t, id, "cell-a", "the-desk", "cell-b", "the-desk", "status", `{"note":"lane-legal"}`)
	}
	control := newBypassGateway(t)
	cs := control.server()
	cs.Cell = "cell-a"
	if got := socketSubmit(t, startSocketServer(t, cs), legal(control, "xcell-control")); got.Receipt == nil || !got.Receipt.Accepted {
		t.Fatalf("control: the pre-existing pipeline should accept this cross-cell send: %+v", got)
	}
	g := newBypassGateway(t)
	s := g.server()
	s.Cell, s.LocalOnly = "cell-a", true
	got := socketSubmit(t, startSocketServer(t, s), legal(g, "xcell-local"))
	if got.Receipt == nil || got.Receipt.Accepted || !strings.Contains(got.Receipt.Detail, "local-only") {
		t.Fatalf("local-only gateway did not refuse a cross-cell send: %+v", got)
	}
	if items, err := commsqueue.ListAccepted(g.root); err != nil || len(items) != 0 {
		t.Fatalf("cross-cell send reached the accepted queue: %+v %v", items, err)
	}
}

func TestLocalOnlyRefusesForeignAck(t *testing.T) {
	plant := func(g *bypassGateway) commsqueue.Notice {
		n := commsqueue.Notice{ID: "foreign-entry", From: comms.SenderID{Cell: "cell-b", Role: "worker-desk"}, Verb: "status", Class: "routine", Payload: json.RawMessage(`{}`), Sent: g.f.now}
		if err := commsqueue.DeliverToMailbox(g.root, "cell-b", "the-desk", n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	ack := func(endpoint, id string) gwResponse {
		b, _ := json.Marshal(gwRequest{Op: "ack", Cell: "cell-b", Role: "the-desk", ID: id})
		var out gwResponse
		if err := json.Unmarshal([]byte(socketRawLine(t, endpoint, string(b)+"\n")), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	control := newBypassGateway(t)
	n := plant(control)
	cs := control.server()
	cs.Cell = "cell-a"
	if got := ack(startSocketServer(t, cs), n.ID); got.Error != "" {
		t.Fatalf("control: the existing entry should be ackable without local-only mode: %s", got.Error)
	}
	g := newBypassGateway(t)
	n = plant(g)
	s := g.server()
	s.Cell, s.LocalOnly = "cell-a", true
	if got := ack(startSocketServer(t, s), n.ID); !strings.Contains(got.Error, "local-only") {
		t.Fatalf("local-only gateway did not refuse another cell's ack: %+v", got)
	}
	if ok, err := commsqueue.IsAcked(g.root, "cell-b", "the-desk", n.ID); err != nil || ok {
		t.Fatalf("another cell's entry was acknowledged: %v %v", ok, err)
	}
}
