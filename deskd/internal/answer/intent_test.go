package answer

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Every key here is generated at test time; no key material is committed.

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

// ringFor enrols pub under login through the ONLY constructor the trust anchor
// has: the file bytes.
func ringFor(t *testing.T, login string, pub ed25519.PublicKey) *intentkeys.Ring {
	t.Helper()
	body := fmt.Sprintf("version: 1\nlogins:\n  %s:\n    - id: test-key\n      ed25519: %s\n", login, base64.StdEncoding.EncodeToString(pub))
	r, err := intentkeys.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parse ring: %v", err)
	}
	return r
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

var nonceSeq int

func signed(priv ed25519.PrivateKey, action contract.Action, login, opt, rev string, iat time.Time, life time.Duration) contract.SignedIntent {
	nonceSeq++
	si := contract.SignedIntent{
		Action: action, Issue: testRef.URL, Login: login,
		OptionID: domain.OptionID(opt), OptionsRev: rev,
		IssuedAt: iat, ExpiresAt: iat.Add(life),
		Nonce: fmt.Sprintf("nonce-%016d", nonceSeq),
	}
	si.Sign(priv)
	return si
}

// verifierFixture: a verifier trusting pub for "operator", started at t0, clock
// at t0+1s.
func verifierFixture(t *testing.T) (*IntentVerifier, ed25519.PrivateKey, *testClock, time.Time) {
	t.Helper()
	pub, priv := genKey(t)
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	clk := &testClock{t: t0}
	v := NewIntentVerifier(ringFor(t, "operator", pub), clk.now)
	clk.set(t0.Add(time.Second))
	return v, priv, clk, t0
}

// TestVerifierRefusals is ruling 1 at the verifier: unknown key, bad signature,
// expired, replayed nonce — plus the bounds that make replay closable (lifetime
// cap, start floor, nonce shape) — each refused, each wrapping
// ErrIntentRefused so the handler maps it to 403.
func TestVerifierRefusals(t *testing.T) {
	rev := strings.Repeat("a", 40)
	t.Run("unenrolled login", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "mallory", "keep-it", rev, t0.Add(time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentUnknownKey) {
			t.Fatalf("err = %v, want ErrIntentUnknownKey", err)
		}
	})
	t.Run("unenrolled key", func(t *testing.T) {
		v, _, _, t0 := verifierFixture(t)
		_, stranger := genKey(t)
		si := signed(stranger, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		_, err := v.Accept(si, contract.ActionAnswer, testRef.URL)
		if !errors.Is(err, ErrIntentRefused) || !strings.Contains(err.Error(), contract.ErrIntentBadSignature.Error()) {
			t.Fatalf("err = %v, want a refusal carrying the bad-signature cause", err)
		}
	})
	t.Run("tampered option", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		si.OptionID = "drop-it"
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentRefused) {
			t.Fatalf("err = %v, want refusal", err)
		}
	})
	t.Run("expired", func(t *testing.T) {
		v, priv, clk, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		clk.set(t0.Add(5 * time.Minute))
		_, err := v.Accept(si, contract.ActionAnswer, testRef.URL)
		if !errors.Is(err, ErrIntentRefused) || !strings.Contains(err.Error(), contract.ErrIntentExpired.Error()) {
			t.Fatalf("err = %v, want expired refusal", err)
		}
	})
	t.Run("replayed nonce", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAck, testRef.URL); err != nil {
			t.Fatalf("first use refused: %v", err)
		}
		if _, err := v.Accept(si, contract.ActionAck, testRef.URL); !errors.Is(err, ErrIntentReplayed) {
			t.Fatalf("replay err = %v, want ErrIntentReplayed", err)
		}
	})
	t.Run("check does not consume", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		for i := 0; i < 2; i++ {
			if err := v.Check(si, contract.ActionAck, testRef.URL); err != nil {
				t.Fatalf("Check #%d: %v", i, err)
			}
		}
		if _, err := v.Accept(si, contract.ActionAck, testRef.URL); err != nil {
			t.Fatalf("Accept after Check: %v", err)
		}
		if err := v.Check(si, contract.ActionAck, testRef.URL); !errors.Is(err, ErrIntentReplayed) {
			t.Fatalf("Check after Accept = %v, want ErrIntentReplayed", err)
		}
	})
	t.Run("predates start", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(-time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentPredates) {
			t.Fatalf("err = %v, want ErrIntentPredates", err)
		}
	})
	t.Run("lifetime over cap", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), MaxIntentLifetime+time.Second)
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentLifetime) {
			t.Fatalf("err = %v, want ErrIntentLifetime", err)
		}
	})
	t.Run("bad nonce", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		si.Nonce = "short"
		si.Sign(priv)
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentNonce) {
			t.Fatalf("err = %v, want ErrIntentNonce", err)
		}
	})
	t.Run("other action", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAnswer, testRef.URL); !errors.Is(err, ErrIntentRefused) {
			t.Fatalf("err = %v, want refusal", err)
		}
	})
	t.Run("other issue", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAck, testRef.URL+"0"); !errors.Is(err, ErrIntentRefused) {
			t.Fatalf("err = %v, want refusal", err)
		}
	})
	t.Run("empty ring", func(t *testing.T) {
		_, priv := genKey(t)
		v := NewIntentVerifier(&intentkeys.Ring{}, nil)
		si := signed(priv, contract.ActionAck, "operator", "", rev, time.Now().UTC().Add(time.Second), time.Minute)
		if _, err := v.Accept(si, contract.ActionAck, testRef.URL); !errors.Is(err, ErrIntentUnknownKey) {
			t.Fatalf("err = %v, want ErrIntentUnknownKey", err)
		}
	})
}

// TestServiceRefusesUnverified is the Service layer with the verifier (and
// every layer above it) BYPASSED: an act the verifier did not mint — the zero
// value, or one a caller filled in — is refused before any platform call, as
// are a signer who is not the answerer, a different issue, free text, and an
// option other than the signed one.
func TestServiceRefusesUnverified(t *testing.T) {
	rev := strings.Repeat("a", 40)
	forged := &SignedAct{login: "operator", action: contract.ActionAnswer, issue: testRef.URL, optionID: "keep-it", optionsRev: rev, expires: time.Now().Add(time.Hour)}
	cases := map[string]struct {
		act  func(t *testing.T) *SignedAct
		req  func(AnswerRequest) AnswerRequest
		want error
	}{
		"nil act":       {func(*testing.T) *SignedAct { return nil }, nil, ErrIntentUnverified},
		"zero act":      {func(*testing.T) *SignedAct { return &SignedAct{} }, nil, ErrIntentUnverified},
		"forged fields": {func(*testing.T) *SignedAct { return forged }, nil, ErrIntentUnverified},
		"not answerer": {func(t *testing.T) *SignedAct {
			pub, priv := genKey(t)
			v := NewIntentVerifier(ringFor(t, "mallory", pub), nil)
			act, err := v.Accept(signed(priv, contract.ActionAnswer, "mallory", "keep-it", rev, time.Now().UTC(), time.Minute), contract.ActionAnswer, testRef.URL)
			if err != nil {
				t.Fatal(err)
			}
			return act
		}, nil, ErrIntentNotAnswerer},
		"other issue": {acceptAnswer, func(r AnswerRequest) AnswerRequest {
			r.Ref = Ref{Repo: "o/n", Number: 9, URL: "https://github.com/o/n/issues/9"}
			return r
		}, ErrIntentMismatch},
		"unsigned text": {acceptAnswer, func(r AnswerRequest) AnswerRequest { r.Text = "and also"; return r }, ErrUnsignedText},
		"other option":  {acceptAnswer, func(r AnswerRequest) AnswerRequest { r.Option = "drop-it"; return r }, ErrIntentMismatch},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := &fakeWriter{}
			svc := NewService(w, nil, nil, nil)
			svc.SetAnswerAs("operator")
			req := AnswerRequest{ID: "01ID", Ref: testRef, Option: "keep-it", OptionsRev: rev}
			if tc.req != nil {
				req = tc.req(req)
			}
			_, err := svc.AnswerSigned(context.Background(), tc.act(t), req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if w.comments+w.closes+w.reacts != 0 {
				t.Fatalf("a refused act reached the platform: %+v", w)
			}
			if err := svc.AckSigned(context.Background(), &SignedAct{}, "01ID", testRef); !errors.Is(err, ErrIntentUnverified) || w.reacts != 0 {
				t.Fatalf("AckSigned(zero act) = %v, reacts=%d", err, w.reacts)
			}
		})
	}
}

func acceptAnswer(t *testing.T) *SignedAct {
	t.Helper()
	pub, priv := genKey(t)
	v := NewIntentVerifier(ringFor(t, "operator", pub), nil)
	act, err := v.Accept(signed(priv, contract.ActionAnswer, "operator", "keep-it", strings.Repeat("a", 40), time.Now().UTC(), time.Minute), contract.ActionAnswer, testRef.URL)
	if err != nil {
		t.Fatal(err)
	}
	return act
}

// TestSignedAnswerRecordsSigner is ruling 2's attribution: the decision comment
// carries the answer-record marker naming the SIGNER, and an audit line naming
// the key and nonce; the result carries signed_by.
func TestSignedAnswerRecordsSigner(t *testing.T) {
	w := &fakeWriter{}
	svc := NewService(w, nil, nil, nil)
	svc.SetAnswerAs("operator")
	act := acceptAnswer(t)
	res, err := svc.AnswerSigned(context.Background(), act, AnswerRequest{ID: "01ID", Ref: testRef, Option: "keep-it", OptionsRev: strings.Repeat("a", 40)})
	if err != nil {
		t.Fatalf("AnswerSigned: %v", err)
	}
	if res.SignedBy != "operator" || w.comments != 1 || w.closes != 1 {
		t.Fatalf("res=%+v comments=%d closes=%d", res, w.comments, w.closes)
	}
	rec, status := contract.ParseAnswerRecord(w.lastComment)
	if status != contract.Answered || rec.Login != "operator" || rec.OptionID != "keep-it" {
		t.Fatalf("answer record = %+v (%v) in %q", rec, status, w.lastComment)
	}
	if !strings.Contains(w.lastComment, "signed by operator") || !strings.Contains(w.lastComment, act.Nonce()) || !strings.Contains(w.lastComment, act.Fingerprint()) {
		t.Fatalf("audit line missing signer/key/nonce: %q", w.lastComment)
	}
	// Item (4): the marker carries the signed record itself, so the poll can
	// re-verify it (item (5)) without trusting the bot that posted it.
	if rec.Signed == nil || rec.Signed.Via != contract.SignedRecordVia || rec.Signed.Key != act.Fingerprint() || rec.Signed.Nonce != act.Nonce() {
		t.Fatalf("marker lacks the signed record: %+v in %q", rec.Signed, w.lastComment)
	}
}

// TestVerifierCausesUnwrap pins the multi-%w wrapping (the authorization contract row
// 15): every refusal is BOTH an ErrIntentRefused (the handler's 403 mapping)
// AND its specific cause, so a caller — and a test — can tell an unenrolled
// key from a bad signature from an expired intent with errors.Is, never by
// matching message text.
func TestVerifierCausesUnwrap(t *testing.T) {
	rev := strings.Repeat("a", 40)
	t.Run("bad signature", func(t *testing.T) {
		v, _, _, t0 := verifierFixture(t)
		_, stranger := genKey(t)
		si := signed(stranger, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		_, err := v.Accept(si, contract.ActionAnswer, testRef.URL)
		if !errors.Is(err, ErrIntentRefused) || !errors.Is(err, contract.ErrIntentBadSignature) {
			t.Fatalf("err = %v, want ErrIntentRefused wrapping ErrIntentBadSignature", err)
		}
	})
	t.Run("expired", func(t *testing.T) {
		v, priv, clk, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAnswer, "operator", "keep-it", rev, t0.Add(time.Second), time.Minute)
		clk.set(t0.Add(5 * time.Minute))
		_, err := v.Accept(si, contract.ActionAnswer, testRef.URL)
		if !errors.Is(err, ErrIntentRefused) || !errors.Is(err, contract.ErrIntentExpired) {
			t.Fatalf("err = %v, want ErrIntentRefused wrapping ErrIntentExpired", err)
		}
		if errors.Is(err, contract.ErrIntentBadSignature) {
			t.Fatalf("expired intent also reports a bad signature: %v", err)
		}
	})
	t.Run("other action", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		_, err := v.Accept(si, contract.ActionAnswer, testRef.URL)
		if !errors.Is(err, ErrIntentRefused) || !errors.Is(err, contract.ErrIntentActionScope) {
			t.Fatalf("err = %v, want ErrIntentRefused wrapping ErrIntentActionScope", err)
		}
	})
	t.Run("other issue", func(t *testing.T) {
		v, priv, _, t0 := verifierFixture(t)
		si := signed(priv, contract.ActionAck, "operator", "", rev, t0.Add(time.Second), time.Minute)
		_, err := v.Accept(si, contract.ActionAck, testRef.URL+"0")
		if !errors.Is(err, ErrIntentRefused) || !errors.Is(err, contract.ErrIntentResource) {
			t.Fatalf("err = %v, want ErrIntentRefused wrapping ErrIntentResource", err)
		}
	})
}

// TestWriterGatewayNeedsAct is the LOWEST layer, with the verifier, the
// Service, the handler and the middleware all bypassed: the real HumanWriter,
// configured with a gateway credential, is called directly. A bot write with
// NO verified signature — no act at all, or an act the verifier did not mint —
// gets no credential and sends nothing; only a verified act reaches the
// gateway token.
func TestWriterGatewayNeedsAct(t *testing.T) {
	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		rw.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	w := NewPerRequestHumanWriter(srv.URL)
	gw, err := NewGatewayCredential("gateway-app[bot]", staticToken("ghs_gateway_test_token"))
	if err != nil {
		t.Fatal(err)
	}
	w.SetIntentGateway(gw)

	if err := w.Comment(context.Background(), testRef, "x"); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("no act, no credential: err = %v, want ErrNoCredential", err)
	}
	if err := w.Comment(withSignedAct(context.Background(), &SignedAct{login: "operator"}), testRef, "x"); !errors.Is(err, ErrIntentUnverified) {
		t.Fatalf("unverified act: err = %v, want ErrIntentUnverified", err)
	}
	if len(auths) != 0 {
		t.Fatalf("a write without a verified act reached the platform: %v", auths)
	}
	if err := w.React(withSignedAct(context.Background(), acceptAnswer(t)), testRef, AckContent); err != nil {
		t.Fatalf("verified act: %v", err)
	}
	if len(auths) != 1 || auths[0] != "Bearer ghs_gateway_test_token" {
		t.Fatalf("verified act authored with %v, want the gateway token", auths)
	}

	// And with NO gateway configured, even a verified act has nothing to use.
	bare := NewPerRequestHumanWriter(srv.URL)
	if err := bare.React(withSignedAct(context.Background(), acceptAnswer(t)), testRef, AckContent); !errors.Is(err, ErrNoGateway) {
		t.Fatalf("no gateway: err = %v, want ErrNoGateway", err)
	}
}

// staticToken is a fixed gateway source for tests. Every token in this file is
// an obvious fixture, never a real credential.
func staticToken(tok string) func() (string, error) {
	return func() (string, error) { return tok, nil }
}

// TestGatewayCredentialRules: the gateway is a BOT credential, never blank and
// never the human write identity, built over a source (never a bare token).
func TestGatewayCredentialRules(t *testing.T) {
	good := staticToken("ghs_fixture_gateway_0c1d")
	for _, c := range []struct {
		login string
		read  func() (string, error)
	}{
		{"gateway-app[bot]", nil},
		{"gateway-app[bot]", staticToken("")},
		{"gateway-app[bot]", staticToken("   ")},
		{"", good},
		{"someone", good},
		{humanActor(), good},
		{"gateway-app[bot]", func() (string, error) { return "", errors.New("unreadable") }},
	} {
		if _, err := NewGatewayCredential(c.login, c.read); !errors.Is(err, ErrNoGateway) {
			t.Errorf("NewGatewayCredential(%q, …) = %v, want ErrNoGateway", c.login, err)
		}
	}
	if g, err := NewGatewayCredential("gateway-app[bot]", good); err != nil || g.Login() != "gateway-app[bot]" {
		t.Fatalf("valid gateway refused: %v", err)
	}
}

// TestGatewayRefusesUserBoundTokens is review S1 (#212): the gateway slot
// must not hold a human's credential. Only a GitHub App installation token
// (ghs_…) is accepted; every user-bound shape — and any unrecognised shape — is
// refused at build time, and the refusal never echoes the token.
func TestGatewayRefusesUserBoundTokens(t *testing.T) {
	for _, tok := range []string{
		"ghp_fixture_user_0c1d",        // classic PAT
		"github_pat_fixture_user_0c1d", // fine-grained PAT
		"gho_fixture_user_0c1d",        // OAuth access token
		"ghu_fixture_user_0c1d",        // user-to-server token
		"ghr_fixture_user_0c1d",        // refresh token
		"fixture_no_prefix_0c1d",       // unrecognised shape
		"ghs_",                         // prefix only
		"ghs_fixture 0c1d",             // whitespace inside
		"ghs_fixture\r\nX-Evil: 1",     // header smuggling
	} {
		_, err := NewGatewayCredential("gateway-app[bot]", staticToken(tok))
		if !errors.Is(err, ErrGatewayTokenShape) {
			t.Errorf("token shape %.4s… accepted as the gateway credential (err = %v), want ErrGatewayTokenShape", tok, err)
			continue
		}
		if len(tok) > 6 && strings.Contains(err.Error(), tok) {
			t.Errorf("the refusal echoes the token: %v", err)
		}
	}
	if err := CheckGatewayToken("ghs_fixture_gateway_0c1d"); err != nil {
		t.Fatalf("an installation-token shape was refused: %v", err)
	}
}

// TestGatewayRereadsEachUse is the S1 refresh path: the writer never holds one
// gateway token for its lifetime. The source is read on EVERY write, so a
// rotated file is used on the next write with no restart; a file rotated to a
// user-bound token, emptied, or removed refuses THAT write and never falls
// back to the previous token.
func TestGatewayRereadsEachUse(t *testing.T) {
	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		rw.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	file := filepath.Join(t.TempDir(), "gateway-token")
	rotate := func(tok string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(tok+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rotate("ghs_fixture_first_0c1d")
	gw, err := NewGatewayCredential("gateway-app[bot]", GatewayTokenFile(file))
	if err != nil {
		t.Fatal(err)
	}
	w := NewPerRequestHumanWriter(srv.URL)
	w.SetIntentGateway(gw)
	react := func() error {
		return w.React(withSignedAct(context.Background(), acceptAnswer(t)), testRef, AckContent)
	}

	if err := react(); err != nil {
		t.Fatalf("first write: %v", err)
	}
	rotate("ghs_fixture_second_0c1d")
	if err := react(); err != nil {
		t.Fatalf("write after rotation: %v", err)
	}
	if len(auths) != 2 || auths[0] != "Bearer ghs_fixture_first_0c1d" || auths[1] != "Bearer ghs_fixture_second_0c1d" {
		t.Fatalf("rotation not picked up per write: %v", auths)
	}

	rotate("ghp_fixture_user_0c1d")
	if err := react(); !errors.Is(err, ErrGatewayTokenShape) {
		t.Fatalf("file rotated to a user-bound token: err = %v, want ErrGatewayTokenShape", err)
	}
	rotate("")
	if err := react(); !errors.Is(err, ErrNoGateway) {
		t.Fatalf("emptied file: err = %v, want ErrNoGateway", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := react(); !errors.Is(err, ErrNoGateway) {
		t.Fatalf("removed file: err = %v, want ErrNoGateway", err)
	}
	if len(auths) != 2 {
		t.Fatalf("a refused gateway read still reached the platform (fell back to an earlier token?): %v", auths)
	}
}
