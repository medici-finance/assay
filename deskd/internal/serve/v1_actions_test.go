package serve_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// ── fixtures ────────────────────────────────────────────────────────────────

// fixtureCredential is the person's per-request credential. Its token is a
// recognisable fixture value so the persistence audit below can grep for it.
const fixtureToken = "gho_fixture_percredential_9f3a"

type fixtureCred struct{ token string }

func (c fixtureCred) Token() (string, error) { return c.token, nil }
func (c fixtureCred) TokenActor() string     { return "operator" }
func (c fixtureCred) Present() bool          { return c.token != "" }

// recordingWriter is the platform writer, replacing GitHub. It records the
// TOKEN each write was authored with, which is how these tests tell "the write
// used the credential the caller supplied for THIS request" from "the write
// used something the service was holding".
type recordingWriter struct {
	tokensSeen []string
	comments   int
	closes     int
	reactions  int
	err        error
}

func (w *recordingWriter) Actor() string { return "operator" }

func (w *recordingWriter) note(ctx context.Context) error {
	cred, ok := answer.CredentialFrom(ctx)
	if !ok {
		w.tokensSeen = append(w.tokensSeen, "")
		return answer.ErrNoCredential
	}
	tok, err := cred.Token()
	if err != nil {
		return err
	}
	w.tokensSeen = append(w.tokensSeen, tok)
	return nil
}

func (w *recordingWriter) CurrentState(ctx context.Context, ref answer.Ref) (answer.IssueState, error) {
	if err := w.note(ctx); err != nil {
		return answer.IssueState{}, err
	}
	return answer.IssueState{}, w.err
}

func (w *recordingWriter) Comment(ctx context.Context, ref answer.Ref, body string) error {
	if err := w.note(ctx); err != nil {
		return err
	}
	w.comments++
	return w.err
}

func (w *recordingWriter) Close(ctx context.Context, ref answer.Ref) error {
	if err := w.note(ctx); err != nil {
		return err
	}
	w.closes++
	return w.err
}

func (w *recordingWriter) React(ctx context.Context, ref answer.Ref, content string) error {
	if err := w.note(ctx); err != nil {
		return err
	}
	w.reactions++
	return w.err
}

type fixtureLookup struct{}

func (fixtureLookup) Ref(id domain.EscalationID) (answer.Ref, bool, error) {
	if id != "IT1" {
		return answer.Ref{}, false, nil
	}
	return answer.Ref{Repo: "owner/name", Number: 7, URL: "https://github.com/owner/name/issues/7"}, true, nil
}

// harness builds the v1 surface with the named actions wired, plus a live
// session cookie for the authorized answerer.
func harness(t *testing.T, cred answer.Credential) (http.Handler, *recordingWriter, *http.Cookie) {
	t.Helper()
	w := &recordingWriter{}
	// NewService takes an unexported interface type, but every method on it is
	// exported — so a writer defined out here satisfies it, which is exactly how
	// the answer flow's own tests substitute one.
	svc := answer.NewService(w, nil, nil, nil)
	svc.SetAnswerAs("operator")
	sessions := answer.NewSessionStore(nil, func() bool { return true })
	sess, err := sessions.BeginAsPresent("operator", true)
	if err != nil {
		t.Fatalf("begin session: %v", err)
	}
	h := serve.NewV1(nil, nil, nil, &serve.NamedActions{
		Service:  svc,
		Sessions: sessions,
		Lookup:   fixtureLookup{},
		Credential: func(r *http.Request) (answer.Credential, bool) {
			if cred == nil {
				return nil, false
			}
			return cred, true
		},
	}, nil, nil, nil, nil)
	return h, w, &http.Cookie{Name: answer.SessionCookie, Value: sess.ID}
}

func post(h http.Handler, path string, c *http.Cookie, form string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:5555"
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// postJSON posts a contract request type as the JSON body the /v1 contract
// (and contract.Client) sends.
func postJSON(h http.Handler, path string, c *http.Cookie, body any) *httptest.ResponseRecorder {
	raw, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:5555"
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ── the tests ───────────────────────────────────────────────────────────────

// TestNamedActionsAreMountedOnTheSeam closes the divergence the 2026-09-06
// audit named: the contract declared two KindNamedAction routes and
// internal/serve mounted none of them, so "the allow-list is closed" was a
// declaration no mounted route implemented. Every declared named action must
// now be a real route.
func TestNamedActionsAreMountedOnTheSeam(t *testing.T) {
	h, _, cookie := harness(t, fixtureCred{fixtureToken})
	for _, r := range contract.Routes {
		if r.Kind != contract.KindNamedAction {
			continue
		}
		if r.Action == contract.ActionSend || r.Action == contract.ActionSession {
			// The terminal actions mount from their OWN wiring (viewer identity,
			// not this harness's write credential); every declared route of
			// theirs is asserted mounted by TestTerminalNamedActionsAreMountedOnTheSeam.
			continue
		}
		path := strings.Replace(r.Path, "{id}", "IT1", 1)
		rec := post(h, path, cookie, "option=1")
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
			t.Errorf("POST %s returned %d — the contract declares this named action; a declared route that 404s/405s is the divergence the audit bounced", path, rec.Code)
		}
	}
}

// TestNoGeneralWriteRouteOnTheSeam is Verify row 4 as a unit test: a POST to a
// write route OUTSIDE the allow-list is a 404 — the route does not exist —
// never a 403 or a 405, which would say a route is there but declined.
func TestNoGeneralWriteRouteOnTheSeam(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	for _, path := range []string{
		contract.Prefix + "/escalations/IT1/comment",
		contract.Prefix + "/escalations/IT1/close",
		contract.Prefix + "/escalations/IT1/react",
		contract.Prefix + "/escalations/IT1/adopt",
	} {
		rec := post(h, path, cookie, "text=anything")
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s returned %d, want 404 — there is no general human write route on any resource (the service contract P2)", path, rec.Code)
		}
	}
	if w.comments+w.closes+w.reactions != 0 {
		t.Fatalf("a non-allow-listed route reached the write path (%d comments, %d closes, %d reactions)", w.comments, w.closes, w.reactions)
	}
}

// TestNamedActionUsesTheCallersCredential is P1 at the handler: the write is
// authored with the credential THIS request carried. A service holding its own
// credential would author with that one, and the token the writer saw would not
// be the caller's.
func TestNamedActionUsesTheCallersCredential(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	rec := postJSON(h, contract.Prefix+"/escalations/IT1/answer", cookie, contract.AnswerRequest{Option: "1", Text: "go"})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer returned %d: %s", rec.Code, rec.Body.String())
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("answer wrote %d comments and %d closes, want exactly one of each (the answer flow semantics, unchanged)", w.comments, w.closes)
	}
	for _, tok := range w.tokensSeen {
		if tok != fixtureToken {
			t.Fatalf("a write was authored with %q, not the caller's per-request credential %q", tok, fixtureToken)
		}
	}
	var res answer.AnswerResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.AuthoredAs != "operator" {
		t.Fatalf("AuthoredAs = %q, want operator — the relocation must not change WHOM the answer is authored as", res.AuthoredAs)
	}
}

// TestNamedActionRefusedWithoutCredential: with no credential on the request
// there is nothing to author with, and the service has none of its own to fall
// back to. The refusal is explicit — never a silent no-op that would look like
// a committed answer.
func TestNamedActionRefusedWithoutCredential(t *testing.T) {
	h, w, cookie := harness(t, nil)
	for _, path := range []string{
		contract.Prefix + "/escalations/IT1/answer",
		contract.Prefix + "/escalations/IT1/ack",
	} {
		rec := post(h, path, cookie, "option=1")
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s with no credential returned %d, want 403", path, rec.Code)
		}
	}
	if w.comments+w.closes+w.reactions != 0 {
		t.Fatalf("a credential-less request still wrote (%d comments, %d closes, %d reactions)", w.comments, w.closes, w.reactions)
	}
}

// TestNamedActionRequiresLiveSession: the credential alone is not authority.
// Holding the token proves the person authorized once; a live session proves
// they are present now, and both are still required after the relocation.
func TestNamedActionRequiresLiveSession(t *testing.T) {
	h, w, _ := harness(t, fixtureCred{fixtureToken})
	rec := post(h, contract.Prefix+"/escalations/IT1/answer", nil, "option=1")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("answer with no session returned %d, want 401", rec.Code)
	}
	if w.comments+w.closes != 0 {
		t.Fatal("a session-less request reached the write path")
	}
}

// TestNamedActionCredentialNeverPersisted is the brief's named acceptance test.
// After N named-action requests the supplied credential appears in NO struct
// that outlives a request: the writer holds none between calls, and a request
// made afterwards with no credential cannot write — which it could if anything
// had retained the earlier one.
func TestNamedActionCredentialNeverPersisted(t *testing.T) {
	w := &recordingWriter{}
	svc := answer.NewService(w, nil, nil, nil)
	svc.SetAnswerAs("operator")
	sessions := answer.NewSessionStore(nil, func() bool { return true })
	sess, err := sessions.BeginAsPresent("operator", true)
	if err != nil {
		t.Fatalf("begin session: %v", err)
	}
	// The resolver hands out a credential only while `supply` is true — it is
	// the client, and it is allowed to stop supplying.
	supply := true
	h := serve.NewV1(nil, nil, nil, &serve.NamedActions{
		Service:  svc,
		Sessions: sessions,
		Lookup:   fixtureLookup{},
		Credential: func(r *http.Request) (answer.Credential, bool) {
			if !supply {
				return nil, false
			}
			return fixtureCred{fixtureToken}, true
		},
	}, nil, nil, nil, nil)
	cookie := &http.Cookie{Name: answer.SessionCookie, Value: sess.ID}

	const n = 5
	for i := 0; i < n; i++ {
		if rec := post(h, contract.Prefix+"/escalations/IT1/ack", cookie, ""); rec.Code != http.StatusOK {
			t.Fatalf("ack %d returned %d: %s", i, rec.Code, rec.Body.String())
		}
	}
	if w.reactions != n {
		t.Fatalf("%d reactions after %d acks", w.reactions, n)
	}

	// The client stops supplying. If ANY structure had kept the credential —
	// the writer, the service, the session, a cache — this would still write.
	supply = false
	rec := post(h, contract.Prefix+"/escalations/IT1/ack", cookie, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("after %d successful named actions, an ack with no credential returned %d, want 403 — something retained the credential across requests", n, rec.Code)
	}
	if w.reactions != n {
		t.Fatalf("the credential-less ack still wrote (%d reactions, want %d) — the service is holding a human credential between requests", w.reactions, n)
	}
	if !strings.Contains(rec.Body.String(), "no human credential") {
		t.Fatalf("the refusal does not name the reason: %s", rec.Body.String())
	}
}

// TestReadRoutesStayReadOnly is the paired control: adding two POST routes did
// not make the READ tree writable. A POST to a read route is still 405.
func TestReadRoutesStayReadOnly(t *testing.T) {
	h, _, cookie := harness(t, fixtureCred{fixtureToken})
	for _, path := range []string{contract.Prefix + "/inbox", contract.Prefix + "/cells", contract.Prefix + "/metrics"} {
		rec := post(h, path, cookie, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s returned %d, want 405 — the read surface stays structurally read-only", path, rec.Code)
		}
	}
}

// ── increment 4: the named-action body is the contract's JSON ───────────────

// TestV1AnswerDecodesContractJSON: the contract (AnswerRequest), the generated
// OpenAPI and contract.Client all send a JSON body; the handler used to parse a
// FORM, so a conformant client's answer arrived with every field empty. The
// decided option and text must reach the write exactly as sent.
func TestV1AnswerDecodesContractJSON(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	rec := postJSON(h, contract.Prefix+"/escalations/IT1/answer", cookie, contract.AnswerRequest{Option: "1", Text: "ship it"})
	if rec.Code != http.StatusOK {
		t.Fatalf("a contract-JSON answer returned %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("answer wrote %d comments and %d closes, want exactly one of each", w.comments, w.closes)
	}
	var res answer.AnswerResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.Option != "1" {
		t.Fatalf("the committed option is %q, want %q — the JSON body's decision did not reach the write", res.Option, "1")
	}
}

// TestAnswerThroughContractClient is the round trip: contract.Client.Answer —
// the one client a service caller uses — commits an answer over the real
// handler. (The live-answer session still rides a cookie here; accepting the
// per-request credential and a verified intent from the request itself is the
// deferred increment-4 item 3, not this test's subject.)
func TestAnswerThroughContractClient(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	srv := httptest.NewServer(h)
	defer srv.Close()
	hc := &http.Client{Transport: cookieTransport{cookie: cookie}, Timeout: 10 * time.Second}
	c := contract.NewClient(srv.URL, contract.WithHTTPClient(hc))
	if _, err := c.Answer(context.Background(), "IT1", contract.AnswerRequest{Option: "1", Text: "go"}, fixtureToken); err != nil {
		t.Fatalf("contract.Client.Answer: %v", err)
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("the client's answer wrote %d comments and %d closes, want one of each", w.comments, w.closes)
	}
}

type cookieTransport struct{ cookie *http.Cookie }

func (t cookieTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.AddCookie(t.cookie)
	return http.DefaultTransport.RoundTrip(r)
}

// TestAnswerRefusesNonContractBodies: a form body, an empty body, a body with
// a field the contract does not declare (a credential-shaped one included) and
// trailing garbage are each refused before anything is written — never
// half-decoded into an answer.
func TestAnswerRefusesNonContractBodies(t *testing.T) {
	cases := []struct {
		name, ctype, body string
		want              int
	}{
		{"form body", "application/x-www-form-urlencoded", "option=1&text=go", http.StatusUnsupportedMediaType},
		{"empty body", "application/json", "", http.StatusBadRequest},
		{"unknown field", "application/json", `{"option":"1","token":"smuggled-credential-value"}`, http.StatusBadRequest},
		{"two values", "application/json", `{"option":"1"}{"option":"2"}`, http.StatusBadRequest},
		// Decoder.More reports false before a closing delimiter, so these
		// trailing tokens are what a More()-only tail check lets through.
		{"trailing brace", "application/json", `{"option":"1"}}`, http.StatusBadRequest},
		{"trailing bracket", "application/json", `{"option":"1"} ]`, http.StatusBadRequest},
	}
	for _, c := range cases {
		h, w, cookie := harness(t, fixtureCred{fixtureToken})
		req := httptest.NewRequest(http.MethodPost, contract.Prefix+"/escalations/IT1/answer", strings.NewReader(c.body))
		req.Header.Set("Content-Type", c.ctype)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: answered %d, want %d: %s", c.name, rec.Code, c.want, rec.Body.String())
		}
		if w.comments+w.closes+w.reactions != 0 {
			t.Errorf("%s: a refused body still wrote (%d comments, %d closes)", c.name, w.comments, w.closes)
		}
	}
}

// failingBody is a request body whose read fails for a reason other than size.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("client went away") }

// TestActionBodyReadErrStatus: only a body over the limit is a
// 413; any other read failure is a 400, not a claim that the body was too big.
func TestActionBodyReadErrStatus(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	for name, c := range map[string]struct {
		body io.Reader
		want int
	}{
		"over the limit": {strings.NewReader(`{"text":"` + strings.Repeat("x", 70<<10) + `"}`), http.StatusRequestEntityTooLarge},
		"read error":     {failingBody{}, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, contract.Prefix+"/escalations/IT1/answer", c.body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: answered %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if w.comments+w.closes+w.reactions != 0 {
		t.Fatalf("a refused body still wrote (%d comments, %d closes)", w.comments, w.closes)
	}
}

// TestNamedActionRefusesUnverifiedIntent: with no verifier wired (no key
// enrolled, or no gateway credential — NamedActions.Intents nil), a body
// carrying a signed intent is REFUSED on both named actions — acting on the
// header credential while ignoring the intent would perform a write the client
// believed was bound by a record nobody checked. The verified path is
// v1_intent_test.go.
func TestNamedActionRefusesUnverifiedIntent(t *testing.T) {
	intent := &contract.SignedIntent{
		Action: contract.ActionAnswer, Issue: "https://github.com/owner/name/issues/7", Login: "operator",
		OptionID: "1", IssuedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Minute), Nonce: "n1",
		Signature: []byte("not-verified-by-anyone"),
	}
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	if rec := postJSON(h, contract.Prefix+"/escalations/IT1/answer", cookie, contract.AnswerRequest{Option: "1", Intent: intent}); rec.Code != http.StatusBadRequest {
		t.Errorf("answer with an unverified intent answered %d, want 400: %s", rec.Code, rec.Body.String())
	}
	ackIntent := *intent
	ackIntent.Action = contract.ActionAck
	if rec := postJSON(h, contract.Prefix+"/escalations/IT1/ack", cookie, contract.AckRequest{Intent: &ackIntent}); rec.Code != http.StatusBadRequest {
		t.Errorf("ack with an unverified intent answered %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if w.comments+w.closes+w.reactions != 0 {
		t.Fatalf("an unverified intent reached the write path (%d comments, %d closes, %d reactions)", w.comments, w.closes, w.reactions)
	}
}

// TestAckAcceptsEmptyAndContractBodies: the ack's body is optional — the
// empty body contract.Client-less callers send and the `{}` contract.Client.Ack
// sends both still ack exactly once each.
func TestAckAcceptsEmptyAndContractBodies(t *testing.T) {
	h, w, cookie := harness(t, fixtureCred{fixtureToken})
	if rec := post(h, contract.Prefix+"/escalations/IT1/ack", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("empty-body ack answered %d: %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(h, contract.Prefix+"/escalations/IT1/ack", cookie, contract.AckRequest{}); rec.Code != http.StatusOK {
		t.Fatalf("contract-JSON ack answered %d: %s", rec.Code, rec.Body.String())
	}
	if w.reactions != 2 {
		t.Fatalf("%d reactions after two acks, want 2", w.reactions)
	}
}
