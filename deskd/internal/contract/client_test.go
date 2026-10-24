package contract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestClientReadRoundTrip: Inbox sends the read-identity bearer on Authorization
// and decodes the typed response, lifting the ETag from the header.
func TestClientReadRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer read-tok" {
			t.Errorf("read Authorization = %q, want the read-identity bearer", got)
		}
		if r.URL.Path != Prefix+"/inbox" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("ETag", `"idx-1"`)
		_ = json.NewEncoder(w).Encode(InboxResponse{Cell: "tracker", Total: 0, Items: nil})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithReadToken("read-tok"), WithHTTPClient(srv.Client()))
	resp, err := c.Inbox(context.Background(), "")
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if resp.Cell != "tracker" {
		t.Fatalf("cell = %q", resp.Cell)
	}
	if resp.ETag != `"idx-1"` {
		t.Fatalf("ETag = %q, want it lifted from the header", resp.ETag)
	}
}

// TestClientNamedActionCredInHeaderNotBody is the custody-(b) client-side
// invariant: the per-request WRITE credential is placed on the Authorization
// header for that one request, NEVER in the JSON body, and the Client retains
// no reference to it after the call.
func TestClientNamedActionCredInHeaderNotBody(t *testing.T) {
	// A stand-in for the person's per-request write credential. Deliberately
	// not shaped like a real GitHub token prefix, so the secret scanner does
	// not flag this test fixture.
	const writeCred = "write-credential-fixture-abc123"
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_ = json.NewEncoder(w).Encode(ActionResult{ID: "esc-1", Action: ActionAnswer, AuthoredAs: "operator", At: time.Now()})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithHTTPClient(srv.Client()))
	req := AnswerRequest{Option: "ship-it", OptionsRev: "abc123", Text: "go"}
	res, err := c.Answer(context.Background(), "esc-1", req, writeCred)
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if res.AuthoredAs != "operator" {
		t.Fatalf("authored_as = %q", res.AuthoredAs)
	}
	if gotAuth != "Bearer "+writeCred {
		t.Fatalf("write credential not on Authorization header: got %q", gotAuth)
	}
	if strings.Contains(gotBody, writeCred) {
		t.Fatalf("write credential LEAKED into the JSON body: %q — custody (b) puts it on the header, never the body", gotBody)
	}
	// The Client holds no field that could retain the credential: it has only
	// http, base, and a READ token — never the write credential just used.
	if c.readToken == writeCred {
		t.Fatal("the write credential was retained as the Client's read token — it must not be stored at all")
	}
}

// TestClientNamedActionNeedsCredOrIntent: invoking a named action with
// neither a per-request credential nor a signed intent is refused client-side —
// the service performs no unauthenticated named action.
func TestClientNamedActionNeedsCredOrIntent(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer srv.Close()
	c := NewClient(srv.URL, WithHTTPClient(srv.Client()))
	if _, err := c.Answer(context.Background(), "esc-1", AnswerRequest{OptionsRev: "abc"}, ""); err == nil {
		t.Fatal("Answer with no credential and no intent succeeded — custody (b) requires one")
	}
	if reached {
		t.Fatal("an unauthenticated named action reached the service")
	}
}

// TestClientSignedIntentKeepsCredOffWire: with a signed intent and
// NO per-request credential, the Answer request carries no Authorization write
// bearer — the raw credential stays on the client, the intent (a signature over
// a record) travels in the body.
func TestClientSignedIntentKeepsCredOffWire(t *testing.T) {
	var gotAuth string
	var gotReq AnswerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(ActionResult{ID: "esc-1", Action: ActionAnswer})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithHTTPClient(srv.Client()))
	intent := &SignedIntent{Action: ActionAnswer, Issue: "https://github.com/o/r/issues/7", OptionsRev: "abc", Signature: []byte{1, 2, 3}}
	req := AnswerRequest{Option: "ship-it", OptionsRev: "abc", Intent: intent}
	if _, err := c.Answer(context.Background(), "esc-1", req, ""); err != nil {
		t.Fatalf("Answer via intent: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization header set on the signed-intent path (%q) — the raw credential should stay on the client", gotAuth)
	}
	if gotReq.Intent == nil || gotReq.Intent.Action != ActionAnswer {
		t.Fatalf("signed intent did not travel in the body: %+v", gotReq.Intent)
	}
}

// TestClientEventsResumeSendsLastEventID: opening the stream with a last id sets
// the Last-Event-ID header so the service replays from its ring buffer.
func TestClientEventsResumeSendsLastEventID(t *testing.T) {
	var gotLastID string
	at := time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLastID = r.Header.Get("Last-Event-ID")
		ev, _ := NewEvent("8", at, EventBadge, BadgeData{Open: 1})
		_ = ev.EncodeSSE(w)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithHTTPClient(srv.Client()))
	st, err := c.Events(context.Background(), "7")
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	defer st.Close()
	ev, err := st.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if gotLastID != "7" {
		t.Fatalf("Last-Event-ID header = %q, want 7", gotLastID)
	}
	if ev.ID != "8" || ev.Type != EventBadge {
		t.Fatalf("event = %+v, want id=8 badge", ev)
	}
}

// TestClientTerminalStreamPathAndResume (the service contract): the desk id carries
// a slash, so the client must path-escape it into ONE segment; the resume cursor
// rides Last-Event-ID; the read token rides Authorization; and the request has
// no body — the stream is read-only. A non-200 is an error before any frame.
func TestClientTerminalStreamPathAndResume(t *testing.T) {
	var gotPath, gotLastID, gotAuth string
	var gotBody int64
	at := time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "denied") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		gotPath, gotLastID, gotAuth, gotBody = r.URL.EscapedPath(), r.Header.Get("Last-Event-ID"), r.Header.Get("Authorization"), r.ContentLength
		ev, _ := NewEvent("12", at, EventTerminalOutput, TerminalOutputData{Cursor: 12, Bytes: []byte{0x1b, '[', 0xff, 0x00, '\n'}})
		_ = ev.EncodeSSE(w)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithHTTPClient(srv.Client()), WithReadToken("viewer-read-token"))
	st, err := c.TerminalStream(context.Background(), "assay/dispatch", "7")
	if err != nil {
		t.Fatalf("TerminalStream: %v", err)
	}
	defer st.Close()
	ev, err := st.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if gotPath != "/v1/desks/assay%2Fdispatch/terminal/stream" {
		t.Errorf("path = %q, want the desk id escaped into one segment", gotPath)
	}
	if gotLastID != "7" || gotAuth != "Bearer viewer-read-token" || gotBody > 0 {
		t.Errorf("Last-Event-ID=%q Authorization=%q body=%d — want 7, the read bearer, and no body", gotLastID, gotAuth, gotBody)
	}
	var d TerminalOutputData
	if err := json.Unmarshal(ev.Data, &d); err != nil {
		t.Fatalf("data: %v", err)
	}
	if ev.Type != EventTerminalOutput || st.LastEventID() != "12" || d.Cursor != 12 || string(d.Bytes) != "\x1b[\xff\x00\n" {
		t.Errorf("event = %+v data = %+v — raw non-UTF-8 terminal bytes must survive the base64 round trip", ev, d)
	}
	if !strings.Contains(string(ev.Data), `"bytes":"G1v/AAo="`) {
		t.Errorf("wire data = %s, want bytes as base64", ev.Data)
	}

	if _, err := c.TerminalStream(context.Background(), "denied/desk", ""); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("a 403 must be an error before any frame, got %v", err)
	}
}

// TestTerminalStreamRouteIsAReadOnlyEvent (the service contract): the stream is
// declared once, as a GET KindEvent with no named action — so it is outside the
// closed custody-(b) allow-list by construction — and its two payloads are
// registered wire types (the credential/target-shape walks cover them).
func TestTerminalStreamRouteIsAReadOnlyEvent(t *testing.T) {
	n := 0
	for _, r := range Routes {
		if !strings.HasSuffix(r.Path, "/terminal/stream") {
			continue
		}
		n++
		if r.Method != GET || r.Kind != KindEvent || r.Action != "" || r.Path != Prefix+"/desks/{id}/terminal/stream" {
			t.Errorf("stream route = %+v, want GET %s/desks/{id}/terminal/stream, KindEvent, no action", r, Prefix)
		}
	}
	if n != 1 {
		t.Fatalf("the contract declares %d terminal stream routes, want exactly 1", n)
	}
	have := map[string]bool{}
	for _, v := range WireTypes() {
		have[reflect.TypeOf(v).Name()] = true
	}
	for _, want := range []string{"TerminalOutputData", "TerminalStateData"} {
		if !have[want] {
			t.Errorf("%s is not in WireTypes(), so neither the OpenAPI document nor the credential walk sees it", want)
		}
	}
}
