package identity

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

const intentPassIssue = "https://github.com/owner/name/issues/7"

// intentStandIn is a route-owned authentication backed by the REAL intent
// verifier over an enrolled-key ring: it admits a session-less request only
// when its body carries an intent that verifies (Check, which does not consume
// the nonce). It restores the body so the handler still reads it.
func intentStandIn(v *answer.IntentVerifier) func(*http.Request) bool {
	return func(r *http.Request) bool {
		raw, err := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if err != nil {
			return false
		}
		var body contract.AnswerRequest
		if json.Unmarshal(raw, &body) != nil || body.Intent == nil {
			return false
		}
		return v.Check(*body.Intent, contract.ActionAnswer, intentPassIssue) == nil
	}
}

// TestMiddlewarePassesVerifiedIntentWithoutSession is gap-3 ruling 3 at the
// middleware: on a login-gated surface, a named-action request with NO viewer
// session but a verified enrolled-key intent reaches the handler; the same
// request without the intent is 401 and never does (fork 4 closes for the
// intent path only).
func TestMiddlewarePassesVerifiedIntentWithoutSession(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := intentkeys.Parse([]byte(fmt.Sprintf(
		"version: 1\nlogins:\n  operator:\n    - id: k\n      ed25519: %s\n", base64.StdEncoding.EncodeToString(pub))))
	if err != nil {
		t.Fatal(err)
	}
	verifier := answer.NewIntentVerifier(ring, nil)

	now := time.Now().UTC()
	si := contract.SignedIntent{
		Action: contract.ActionAnswer, Issue: intentPassIssue, Login: "operator",
		OptionID: "1", OptionsRev: "rev1", IssuedAt: now, ExpiresAt: now.Add(time.Minute),
		Nonce: "identity-nonce-00000001",
	}
	si.Sign(priv)
	withIntent, _ := json.Marshal(contract.AnswerRequest{Option: "1", OptionsRev: "rev1", Intent: &si})
	noIntent, _ := json.Marshal(contract.AnswerRequest{Option: "1", OptionsRev: "rev1"})

	var served int
	var seenBody string
	g, _ := guardWith("", false, fakeRepos{}, fakePolicy{answerAs: "operator", loginGated: true})
	h := g.Handler(Route{
		Pattern: "/v1/", Min: Commenter, ItemScoped: false,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			served++
			b, _ := io.ReadAll(r.Body)
			seenBody = string(b)
		}),
		Authenticated: intentStandIn(verifier),
	})
	const path = "/v1/escalations/IT1/answer"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(withIntent)))
	if served != 1 || rec.Code == http.StatusUnauthorized {
		t.Fatalf("verified intent, no session: served=%d code=%d, want passed to the handler", served, rec.Code)
	}
	if seenBody != string(withIntent) {
		t.Fatalf("the handler saw a different body than was sent: %q", seenBody)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(noIntent)))
	if served != 1 || rec.Code != http.StatusUnauthorized {
		t.Fatalf("no intent, no session: served=%d code=%d, want 401 and the handler not reached", served, rec.Code)
	}
}
