package serve_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// The signed-intent path end to end (the authorization contract gap 3), over the REAL
// verifier, Service and HumanWriter against a fake GitHub that records every
// request. Every key is generated at test time; the gateway token is a
// recognisable fixture value, not a credential.

const gatewayToken = "ghs_gateway_fixture_token_0c1d"

type ghReq struct{ Method, Path, Auth, Body string }

// fakeGH is GitHub for these tests: the issue reads open, every write succeeds,
// and every request is recorded so a test can assert "nothing was written".
type fakeGH struct {
	mu   sync.Mutex
	reqs []ghReq
}

func (f *fakeGH) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, ghReq{r.Method, r.URL.Path, r.Header.Get("Authorization"), string(body)})
	f.mu.Unlock()
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, `{"state":"open","updated_at":"2026-01-01T00:00:00Z"}`)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{}`)
}

// writes counts the non-GET requests: the platform side effects.
func (f *fakeGH) writes() []ghReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ghReq
	for _, r := range f.reqs {
		if r.Method != http.MethodGet {
			out = append(out, r)
		}
	}
	return out
}

// gatedPolicy is a private-repo cell: every surface-scoped route needs a viewer
// session unless the route's own Authenticated stand-in admits the request.
type gatedPolicy struct{}

func (gatedPolicy) AnswerAsForRepo(string, string) string    { return "operator" }
func (gatedPolicy) AnonymousReadAllowed(string, string) bool { return false }
func (gatedPolicy) SurfaceLoginGated() bool                  { return true }

type intentRig struct {
	v1      http.Handler // the /v1 surface, unguarded
	guarded http.Handler // the same surface behind the capability model guard
	named   *serve.NamedActions
	gh      *fakeGH
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	cookie  *http.Cookie
	reached *atomic.Int32 // requests the guard let through to the /v1 surface
}

// intentHarness wires the real stack. bypass=true replaces the guard's
// stand-in with one that admits EVERY session-less request — the upper layer
// bypassed — so a refusal can only come from a layer below the middleware.
func intentHarness(t *testing.T, bypass bool) *intentRig {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := intentkeys.Parse([]byte(fmt.Sprintf(
		"version: 1\nlogins:\n  operator:\n    - id: test-key\n      ed25519: %s\n", base64.StdEncoding.EncodeToString(pub))))
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	srv := httptest.NewServer(gh)
	t.Cleanup(srv.Close)

	writer := answer.NewPerRequestHumanWriter(srv.URL)
	gw, err := answer.NewGatewayCredential("gateway-app[bot]", func() (string, error) { return gatewayToken, nil })
	if err != nil {
		t.Fatal(err)
	}
	writer.SetIntentGateway(gw)
	svc := answer.NewService(writer, nil, nil, nil)
	svc.SetAnswerAs("operator")
	sessions := answer.NewSessionStore(nil, func() bool { return true })
	sess, err := sessions.BeginAsPresent("operator", true)
	if err != nil {
		t.Fatal(err)
	}
	named := &serve.NamedActions{
		Service:  svc,
		Sessions: sessions,
		Lookup:   fixtureLookup{},
		Credential: func(*http.Request) (answer.Credential, bool) {
			return fixtureCred{fixtureToken}, true
		},
		Intents: answer.NewIntentVerifier(ring, nil),
	}
	v1 := serve.NewV1(nil, nil, nil, named, nil, nil, nil, nil)
	authn := named.IntentAuthenticated
	if bypass {
		authn = func(*http.Request) bool { return true }
	}
	reached := &atomic.Int32{}
	spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		v1.ServeHTTP(w, r)
	})
	guard := &identity.Guard{Sessions: identity.NewSessionStore(nil), Policy: gatedPolicy{}}
	guarded := guard.Handler(identity.Route{Pattern: "/v1/", Min: identity.Commenter, Handler: spy, Authenticated: authn})
	return &intentRig{v1: v1, guarded: guarded, named: named, gh: gh, pub: pub, priv: priv,
		cookie: &http.Cookie{Name: answer.SessionCookie, Value: sess.ID}, reached: reached}
}

var intentNonce int

// sign builds an intent for the fixture escalation, signed by priv, issued now.
func sign(priv ed25519.PrivateKey, action contract.Action, login, option string, life time.Duration) *contract.SignedIntent {
	intentNonce++
	now := time.Now().UTC()
	si := &contract.SignedIntent{
		Action: action, Issue: "https://github.com/owner/name/issues/7", Login: login,
		OptionID: "", OptionsRev: "rev1", IssuedAt: now, ExpiresAt: now.Add(life),
		Nonce: fmt.Sprintf("serve-nonce-%08d", intentNonce),
	}
	si.OptionID = optionID(option)
	si.Sign(priv)
	return si
}

func optionID(s string) domain.OptionID { return domain.OptionID(s) }

func answerBody(si *contract.SignedIntent) contract.AnswerRequest {
	return contract.AnswerRequest{Option: si.OptionID, OptionsRev: si.OptionsRev, Intent: si}
}

const (
	answerPath = contract.Prefix + "/escalations/IT1/answer"
	ackPath    = contract.Prefix + "/escalations/IT1/ack"
)

// refusedFor asserts a refusal with the given status whose recorded error is
// the SPECIFIC cause (the authorization contract row 15): a leg must not pass on some
// other, unrelated refusal. The handler records err.Error(), and with
// multi-%w wrapping that text carries the cause's own message.
func refusedFor(t *testing.T, leg string, rec *httptest.ResponseRecorder, code int, cause error) {
	t.Helper()
	if rec.Code != code {
		t.Errorf("%s: answered %d, want %d: %s", leg, rec.Code, code, rec.Body.String())
		return
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Errorf("%s: refusal body is not JSON: %s", leg, rec.Body.String())
		return
	}
	if !strings.Contains(body.Error, cause.Error()) {
		t.Errorf("%s: refused for %q, want the cause %q", leg, body.Error, cause.Error())
	}
}

// The row-15 legs run with the upper layer BYPASSED: no cookie, no
// per-request credential, through a guard whose stand-in admits everything,
// so only the handler's verifier over the enrolled-key file stands. Each
// leg's intent is otherwise valid; each asserts a 4xx, ZERO requests of any
// kind to the platform, and its own cause.

// TestIntentUnenrolledKeyRefused: an intent validly signed for a login with NO
// enrolled key. (An ENROLLED login signed with a stranger's key carries no key
// id, so it can only surface as a bad signature — that is the bad-signature
// leg.)
func TestIntentUnenrolledKeyRefused(t *testing.T) {
	rig := intentHarness(t, true)
	_, stranger, _ := ed25519.GenerateKey(nil)
	si := sign(stranger, contract.ActionAnswer, "mallory", "1", time.Minute)
	refusedFor(t, "answer", postJSON(rig.guarded, answerPath, nil, answerBody(si)), http.StatusForbidden, answer.ErrIntentUnknownKey)
	ack := sign(stranger, contract.ActionAck, "mallory", "", time.Minute)
	refusedFor(t, "ack", postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: ack}), http.StatusForbidden, answer.ErrIntentUnknownKey)
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("an unenrolled login reached the platform: %+v", rig.gh.reqs)
	}
}

// TestIntentBadSignatureRefused: a signature that does not verify — the
// enrolled key's intent altered after signing, and the enrolled login signed
// by a key that is not the enrolled one.
func TestIntentBadSignatureRefused(t *testing.T) {
	rig := intentHarness(t, true)
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	si.OptionID = optionID("2")
	refusedFor(t, "tampered", postJSON(rig.guarded, answerPath, nil, answerBody(si)), http.StatusForbidden, contract.ErrIntentBadSignature)
	_, stranger, _ := ed25519.GenerateKey(nil)
	other := sign(stranger, contract.ActionAnswer, "operator", "1", time.Minute)
	refusedFor(t, "other key", postJSON(rig.guarded, answerPath, nil, answerBody(other)), http.StatusForbidden, contract.ErrIntentBadSignature)
	ack := sign(stranger, contract.ActionAck, "operator", "", time.Minute)
	refusedFor(t, "ack, other key", postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: ack}), http.StatusForbidden, contract.ErrIntentBadSignature)
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("a bad signature reached the platform: %+v", rig.gh.reqs)
	}
}

// TestIntentExpiredRefused: an intent past its exp.
func TestIntentExpiredRefused(t *testing.T) {
	rig := intentHarness(t, true)
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	refusedFor(t, "expired", postJSON(rig.guarded, answerPath, nil, answerBody(si)), http.StatusForbidden, contract.ErrIntentExpired)
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("an expired intent reached the platform: %+v", rig.gh.reqs)
	}
}

// TestIntentReplayedNonceRefused: the same signed intent twice. The first acts
// once; the second presentation of the accepted nonce is refused as a replay
// and sends nothing more to the platform.
func TestIntentReplayedNonceRefused(t *testing.T) {
	rig := intentHarness(t, true)
	si := sign(rig.priv, contract.ActionAck, "operator", "", time.Minute)
	if rec := postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: si}); rec.Code != http.StatusOK {
		t.Fatalf("first use answered %d: %s", rec.Code, rec.Body.String())
	}
	before := len(rig.gh.reqs)
	refusedFor(t, "replay", postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: si}), http.StatusForbidden, answer.ErrIntentReplayed)
	if n := len(rig.gh.reqs); n != before {
		t.Fatalf("the replay reached the platform: %d requests after the first use, want %d", n, before)
	}
}

// TestIntentOverlongLifetimeRefused: an intent whose exp - iat exceeds the
// 10-minute cap (gap-3 item (7)), otherwise valid and unexpired.
func TestIntentOverlongLifetimeRefused(t *testing.T) {
	rig := intentHarness(t, true)
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", answer.MaxIntentLifetime+time.Minute)
	refusedFor(t, "answer", postJSON(rig.guarded, answerPath, nil, answerBody(si)), http.StatusForbidden, answer.ErrIntentLifetime)
	ack := sign(rig.priv, contract.ActionAck, "operator", "", answer.MaxIntentLifetime+time.Minute)
	refusedFor(t, "ack", postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: ack}), http.StatusForbidden, answer.ErrIntentLifetime)
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("an over-long intent reached the platform: %+v", rig.gh.reqs)
	}
}

// TestBotWriteWithoutVerifiedIntentRefused: a named action with neither a
// per-request credential (no cookie session) nor a VERIFIED intent — no
// intent at all, or one that fails verification — with a gateway credential
// configured and the upper layer bypassed. Nothing is written and the gateway
// token is never used: there is no fallback to the bot credential.
func TestBotWriteWithoutVerifiedIntentRefused(t *testing.T) {
	rig := intentHarness(t, true)
	if rec := postJSON(rig.guarded, answerPath, nil, contract.AnswerRequest{Option: "1", OptionsRev: "rev1"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned bot answer answered %d, want 401: %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(rig.guarded, ackPath, nil, contract.AckRequest{}); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned bot ack answered %d, want 401", rec.Code)
	}
	_, stranger, _ := ed25519.GenerateKey(nil)
	failed := sign(stranger, contract.ActionAnswer, "operator", "1", time.Minute)
	if rec := postJSON(rig.guarded, answerPath, nil, answerBody(failed)); rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("failed-verification answer answered %d, want 4xx: %s", rec.Code, rec.Body.String())
	}
	failedAck := sign(stranger, contract.ActionAck, "operator", "", time.Minute)
	if rec := postJSON(rig.guarded, ackPath, nil, contract.AckRequest{Intent: failedAck}); rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("failed-verification ack answered %d, want 4xx", rec.Code)
	}
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("an unverified bot write reached the platform: %+v", rig.gh.reqs)
	}
}

// TestIntentOptionMismatchRefused (gap-3 item (8)): a request body whose
// option or options_rev differs from the signed intent's is refused 4xx with
// zero writes; the accepted write's option/rev are the intent's.
func TestIntentOptionMismatchRefused(t *testing.T) {
	rig := intentHarness(t, true)
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	body := answerBody(si)
	body.Option = optionID("2")
	if rec := postJSON(rig.guarded, answerPath, nil, body); rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("request option != signed option answered %d, want 4xx: %s", rec.Code, rec.Body.String())
	}
	si = sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	body = answerBody(si)
	body.OptionsRev = "rev2"
	if rec := postJSON(rig.guarded, answerPath, nil, body); rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("request options_rev != signed rev answered %d, want 4xx: %s", rec.Code, rec.Body.String())
	}
	if n := len(rig.gh.writes()); n != 0 {
		t.Fatalf("a mismatched request wrote to the platform: %d writes", n)
	}

	si = sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	if rec := postJSON(rig.guarded, answerPath, nil, answerBody(si)); rec.Code != http.StatusOK {
		t.Fatalf("matching request answered %d: %s", rec.Code, rec.Body.String())
	}
	rec, status := contract.ParseAnswerRecord(commentBody(t, rig.gh.writes()))
	if status != contract.Answered || rec.OptionID != si.OptionID || rec.OptionsRev != si.OptionsRev {
		t.Fatalf("written record option=%q rev=%q (%v), want the intent's %q/%q", rec.OptionID, rec.OptionsRev, status, si.OptionID, si.OptionsRev)
	}
}

// commentBody is the text of the single decision comment among writes.
func commentBody(t *testing.T, writes []ghReq) string {
	t.Helper()
	var found []string
	for _, w := range writes {
		if w.Method == http.MethodPost && strings.HasSuffix(w.Path, "/comments") {
			var c struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal([]byte(w.Body), &c); err != nil {
				t.Fatalf("comment request body is not JSON: %s", w.Body)
			}
			found = append(found, c.Body)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one decision comment, got %d in %+v", len(found), writes)
	}
	return found[0]
}

// TestIntentNoCookieFallback: a request carrying BOTH a valid cookie session
// AND a refused intent is refused — the intent is never ignored in favour of
// the cookie.
func TestIntentNoCookieFallback(t *testing.T) {
	rig := intentHarness(t, true)
	_, stranger, _ := ed25519.GenerateKey(nil)
	si := sign(stranger, contract.ActionAnswer, "operator", "1", time.Minute)
	if rec := postJSON(rig.v1, answerPath, rig.cookie, answerBody(si)); rec.Code != http.StatusForbidden {
		t.Fatalf("cookie + refused intent answered %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("a refused intent fell back to the cookie and wrote: %+v", rig.gh.reqs)
	}
}

// TestIntentAnswerRunsUnderBotWithSignerRecord is rulings 2 and 3's positive
// path through the REAL middleware stand-in: no viewer session, no cookie, no
// per-request credential, a verified intent. The guard admits it, exactly one
// comment + close commits under the gateway credential (never the human's),
// and the comment carries the SIGNED answer record, which re-verifies against
// the enrolled key: the human signature is the act, the bot only the gateway.
func TestIntentAnswerRunsUnderBotWithSignerRecord(t *testing.T) {
	rig := intentHarness(t, false)
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	rec := postJSON(rig.guarded, answerPath, nil, answerBody(si))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed answer answered %d: %s", rec.Code, rec.Body.String())
	}
	var res contract.ActionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.SignedBy != "operator" || res.AuthoredAs != "gateway-app[bot]" {
		t.Fatalf("result signed_by=%q authored_as=%q", res.SignedBy, res.AuthoredAs)
	}
	writes := rig.gh.writes()
	if len(writes) != 2 {
		t.Fatalf("want comment + close, got %+v", writes)
	}
	for _, r := range rig.gh.reqs {
		if r.Auth != "Bearer "+gatewayToken {
			t.Fatalf("%s %s authored with %q, want the gateway credential", r.Method, r.Path, r.Auth)
		}
	}
	comment := commentBody(t, writes)
	if !strings.Contains(comment, "signed by operator") {
		t.Fatalf("decision comment does not record the signer: %s", comment)
	}
	record, status := contract.ParseAnswerRecord(comment)
	if status != contract.Answered || record.Login != si.Login || record.OptionID != si.OptionID || record.OptionsRev != si.OptionsRev {
		t.Fatalf("answer record = %+v (%v), want the intent's login/option/rev", record, status)
	}
	if record.Signed == nil || record.Signed.Nonce != si.Nonce || record.Signed.Issue != si.Issue || record.Signed.Key != contract.KeyFingerprint(rig.pub) {
		t.Fatalf("answer record lacks the item (4) fields: %+v", record.Signed)
	}
	// Re-verify the POSTED record's sig= against the enrolled key, as the
	// poll will — not the in-memory intent.
	if err := contract.VerifyAnswerRecord(record, []ed25519.PublicKey{rig.pub}, contract.AnswerBinding{
		AnswerAs: "operator", Issue: si.Issue, PostedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("the posted answer record does not verify under the enrolled key: %v", err)
	}
}

// TestGuardRefusesUnverifiedIntent: the REAL middleware stand-in refuses a
// session-less request whose intent does not verify (401 before the handler),
// and a session-less request with no intent at all.
func TestGuardRefusesUnverifiedIntent(t *testing.T) {
	rig := intentHarness(t, false)
	_, stranger, _ := ed25519.GenerateKey(nil)
	tampered := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	tampered.OptionID = optionID("2")
	for name, si := range map[string]*contract.SignedIntent{
		"bad signature (other key)": sign(stranger, contract.ActionAnswer, "operator", "1", time.Minute),
		"bad signature (tampered)":  tampered,
		"unenrolled login":          sign(stranger, contract.ActionAnswer, "mallory", "1", time.Minute),
	} {
		if rec := postJSON(rig.guarded, answerPath, nil, answerBody(si)); rec.Code != http.StatusUnauthorized {
			t.Errorf("guard admitted an intent with a %s: %d", name, rec.Code)
		}
	}
	if n := rig.reached.Load(); n != 0 {
		t.Fatalf("%d unverified-intent requests passed the guard to the handler", n)
	}
	if rec := postJSON(rig.guarded, answerPath, nil, contract.AnswerRequest{Option: "1"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("guard admitted a session-less request with no intent: %d", rec.Code)
	}
	good := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	req := httptest.NewRequest(http.MethodGet, contract.Prefix+"/escalations", nil)
	if rig.named.IntentAuthenticated(req) {
		t.Error("the stand-in admitted a GET")
	}
	raw, _ := json.Marshal(answerBody(good))
	other := httptest.NewRequest(http.MethodPost, contract.Prefix+"/escalations/IT1/read", strings.NewReader(string(raw)))
	if rig.named.IntentAuthenticated(other) {
		t.Error("the stand-in admitted a route other than answer/ack")
	}
	if len(rig.gh.reqs) != 0 {
		t.Fatalf("a refused request reached the platform: %+v", rig.gh.reqs)
	}
}

// TestCookiePathUnchanged: the browser path through the real writer — cookie
// session, per-request credential, no intent — still authors with the HUMAN's
// per-request credential, never the gateway's, and records no signer.
func TestCookiePathUnchanged(t *testing.T) {
	rig := intentHarness(t, false)
	rec := postJSON(rig.v1, answerPath, rig.cookie, contract.AnswerRequest{Option: "1", OptionsRev: "rev1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie answer answered %d: %s", rec.Code, rec.Body.String())
	}
	var res contract.ActionResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.SignedBy != "" || res.AuthoredAs != "operator" {
		t.Fatalf("cookie path result signed_by=%q authored_as=%q", res.SignedBy, res.AuthoredAs)
	}
	for _, r := range rig.gh.reqs {
		if r.Auth != "Bearer "+fixtureToken {
			t.Fatalf("cookie path authored %s %s with %q, want the per-request credential", r.Method, r.Path, r.Auth)
		}
	}
}

// TestReadRouteIntentNoSession: the intent exemption is for the two named
// actions ONLY. Every /v1 READ route and the non-named write POST .../read,
// each carrying a VALID enrolled-key intent and no viewer session, gets 401
// from the REAL guard stand-in and never reaches its handler. The positive
// control — the same kind of intent on POST .../answer — does pass the guard,
// so the refusals are the route scoping, not a broken intent.
func TestReadRouteIntentNoSession(t *testing.T) {
	rig := intentHarness(t, false)
	for _, rt := range []struct{ method, path string }{
		{http.MethodGet, contract.Prefix + "/inbox"},
		{http.MethodGet, contract.Prefix + "/escalations"},
		{http.MethodGet, contract.Prefix + "/escalations/IT1"},
		{http.MethodPost, contract.Prefix + "/escalations/IT1/read"},
	} {
		si := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
		raw, _ := json.Marshal(answerBody(si))
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		rig.guarded.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with a valid intent and no session answered %d, want 401", rt.method, rt.path, rec.Code)
		}
	}
	if n := rig.reached.Load(); n != 0 {
		t.Fatalf("%d intent-carrying non-named requests reached the /v1 handler", n)
	}
	si := sign(rig.priv, contract.ActionAnswer, "operator", "1", time.Minute)
	if rec := postJSON(rig.guarded, answerPath, nil, answerBody(si)); rec.Code != http.StatusOK || rig.reached.Load() != 1 {
		t.Fatalf("positive control: answer with a valid intent answered %d, reached=%d", rec.Code, rig.reached.Load())
	}
}
