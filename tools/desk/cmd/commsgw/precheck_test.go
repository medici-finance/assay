package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// testFixture builds a self-contained, in-memory-backed PreCheckDeps: a fresh
// ed25519 keypair for "cell-a", a fresh claims dir, and a permissive lane ACL
// matching the compiled matrix. Every test in this file starts from one of
// these so a failure in one test can never leak claim state into another.
type testFixture struct {
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
	deps  PreCheckDeps
	now   time.Time
	guard func() error // mutable so a test can arm/disarm the kill switch
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	acl := comms.Compiled()
	f := &testFixture{
		priv: priv,
		pub:  pub,
		// REAL wall-clock "now", not a fixed fictional date: MemReplayGuard's
		// default clock hook is unexported (comms package), so it always
		// prunes against actual time.Now() — a fixture "now" far from real
		// wall-clock would make a nonce's DefaultTTL (2m) window look
		// already-expired to the GUARD's pruning even while comms.Verify's own
		// (fixture-driven) expiry check still calls it valid, so replay
		// detection would silently never fire. Keeping fixture "now" close to
		// real wall-clock keeps both clocks in agreement for the ~seconds a
		// test runs.
		now: time.Now().UTC(),
	}
	f.guard = func() error { return nil }
	f.deps = PreCheckDeps{
		Trust:       comms.Ed25519TrustStore{"cell-a": pub},
		Skew:        comms.DefaultSkew,
		ReplayGuard: comms.NewMemReplayGuard(),
		ACL:         &acl,
		ClaimConfig: deskkit.ClaimConfig{
			ClaimsDir: filepath.Join(t.TempDir(), "claims"),
		},
		Owner:       "test",
		RateLimiter: NewRateLimiter(1000, time.Minute),
		GuardFn:     func() error { return f.guard() },
	}
	return f
}

// envelope builds a signed, well-formed within-cell envelope from cell-a/the-desk
// to cell-a/worker-desk carrying verb "handoff", using the fixture's key.
func (f *testFixture) envelope(t *testing.T, id string, mutate func(*wireEnvelope)) []byte {
	t.Helper()
	we := wireEnvelope{
		Schema: comms.Schema,
		ID:     id,
		Cell:   "cell-a",
		From:   comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:     comms.Lane{Cell: "cell-a", Role: "worker-desk"},
		Verb:   "handoff",
		Class:  "routine",
		Sent:   f.now,
	}
	if mutate != nil {
		mutate(&we)
	}
	a, err := comms.Mint(we.From.Cell, we.From.Role, we.ID, id+"-nonce", f.now, 0, comms.Ed25519Signer{Key: f.priv})
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	we.Sig = a
	raw, err := json.Marshal(we)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// wireEnvelope mirrors comms.Envelope's JSON shape (comms.Envelope itself is
// used for the parsed/authenticated result; this local type lets tests build
// the RAW wire bytes PreCheck consumes without depending on unexported parse
// internals).
type wireEnvelope struct {
	Schema string          `json:"schema"`
	ID     string          `json:"id"`
	Cell   string          `json:"cell"`
	From   comms.SenderID  `json:"from"`
	To     comms.Lane      `json:"to"`
	Verb   string          `json:"verb"`
	Class  string          `json:"class"`
	Sent   time.Time       `json:"sent"`
	Sig    comms.Assertion `json:"sig"`
}

// --- Verify row 2: PeerAuth -------------------------------------------------

func TestPeerAuth(t *testing.T) {
	f := newFixture(t)

	t.Run("unauthenticated peer refused", func(t *testing.T) {
		raw := f.envelope(t, "msg-unauth", nil)
		_, err := PreCheck(PreCheckInput{PeerAuthenticated: false, Raw: raw, Now: f.now}, f.deps)
		if !errors.Is(err, ErrPeerUnauthenticated) {
			t.Fatalf("want ErrPeerUnauthenticated, got %v", err)
		}
	})

	t.Run("bad signature refused distinctly", func(t *testing.T) {
		// Sign first, THEN tamper with the signature bytes — mutate runs
		// before signing for every other case, but a bad-signature fixture
		// must tamper AFTER a real signature exists.
		raw := f.envelope(t, "msg-badsig", nil)
		var we wireEnvelope
		if err := json.Unmarshal(raw, &we); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		we.Sig.Sig = append([]byte(nil), we.Sig.Sig...)
		we.Sig.Sig[0] ^= 0xFF // flip a bit: signature no longer verifies
		raw, err := json.Marshal(we)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		_, err = PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: f.now}, f.deps)
		if !errors.Is(err, ErrAssertionInvalid) || !errors.Is(err, comms.ErrBadSignature) {
			t.Fatalf("want ErrAssertionInvalid + ErrBadSignature, got %v", err)
		}
	})

	t.Run("expired assertion refused distinctly", func(t *testing.T) {
		raw := f.envelope(t, "msg-expired", nil)
		future := f.now.Add(comms.DefaultTTL + time.Minute)
		_, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: future}, f.deps)
		if !errors.Is(err, ErrAssertionInvalid) || !errors.Is(err, comms.ErrExpired) {
			t.Fatalf("want ErrAssertionInvalid + ErrExpired, got %v", err)
		}
	})

	t.Run("replayed assertion refused distinctly", func(t *testing.T) {
		raw := f.envelope(t, "msg-replay", nil)
		if _, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: f.now}, f.deps); err != nil {
			t.Fatalf("first delivery should accept, got %v", err)
		}
		raw2 := f.envelope(t, "msg-replay-2", func(we *wireEnvelope) {
			// Same nonce, different message id: the nonce (not the id) is what the
			// replay guard keys on, per identity.go's canonicalAssertionBytes.
		})
		// Re-sign msg-replay-2 with the SAME nonce as msg-replay by re-minting by hand.
		a, err := comms.Mint("cell-a", "the-desk", "msg-replay-2", "msg-replay-nonce", f.now, 0, comms.Ed25519Signer{Key: f.priv})
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		var we wireEnvelope
		if err := json.Unmarshal(raw2, &we); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		we.Sig = a
		raw2, _ = json.Marshal(we)
		_, err = PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw2, Now: f.now}, f.deps)
		if !errors.Is(err, ErrAssertionInvalid) || !errors.Is(err, comms.ErrReplay) {
			t.Fatalf("want ErrAssertionInvalid + ErrReplay, got %v", err)
		}
	})

	t.Run("unknown cell refused distinctly", func(t *testing.T) {
		raw := f.envelope(t, "msg-unknown-cell", func(we *wireEnvelope) {
			we.From.Cell = "cell-ghost"
			we.Cell = "cell-ghost"
		})
		// Re-sign as cell-ghost so parse/triple checks pass and only trust-store
		// lookup fails.
		a, err := comms.Mint("cell-ghost", "the-desk", "msg-unknown-cell", "n", f.now, 0, comms.Ed25519Signer{Key: f.priv})
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		var we wireEnvelope
		if err := json.Unmarshal(raw, &we); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		we.Sig = a
		raw, _ = json.Marshal(we)
		_, err = PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: raw, Now: f.now}, f.deps)
		if !errors.Is(err, ErrAssertionInvalid) || !errors.Is(err, comms.ErrUnknownCell) {
			t.Fatalf("want ErrAssertionInvalid + ErrUnknownCell, got %v", err)
		}
	})
}

// --- Verify row 8: ReplayGuardWired, through the REAL construction path -----

func TestReplayGuardWired(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	trustPath := writeTrustStoreFixture(t, dir, "cell-a", pub)

	cfg := Config{
		Cell:       "cell-a",
		QueueDir:   filepath.Join(dir, "queue"),
		TrustStore: trustPath,
	}
	deps, err := NewPreCheckDeps(cfg) // the gateway's REAL construction path — not a hand-built Verify call.
	if err != nil {
		t.Fatalf("NewPreCheckDeps: %v", err)
	}
	if deps.ReplayGuard == nil {
		t.Fatalf("NewPreCheckDeps must never construct a nil ReplayGuard")
	}

	// REAL wall-clock now — see newFixture's doc comment on why a fixed
	// fictional date would make MemReplayGuard's (unexported-clock) pruning
	// disagree with comms.Verify's fixture-driven expiry check.
	now := time.Now().UTC()
	mkEnv := func(id, nonce string) []byte {
		we := wireEnvelope{
			Schema: comms.Schema, ID: id, Cell: "cell-a",
			From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
			To:   comms.Lane{Cell: "cell-a", Role: "worker-desk"},
			Verb: "handoff", Class: "routine", Sent: now,
		}
		a, err := comms.Mint(we.From.Cell, we.From.Role, id, nonce, now, 0, comms.Ed25519Signer{Key: priv})
		if err != nil {
			t.Fatalf("Mint: %v", err)
		}
		we.Sig = a
		raw, _ := json.Marshal(we)
		return raw
	}

	if _, err := PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: mkEnv("m1", "shared-nonce"), Now: now}, deps); err != nil {
		t.Fatalf("first delivery should accept through the real construction path, got %v", err)
	}
	_, err = PreCheck(PreCheckInput{PeerAuthenticated: true, Raw: mkEnv("m2", "shared-nonce"), Now: now}, deps)
	if !errors.Is(err, ErrAssertionInvalid) || !errors.Is(err, comms.ErrReplay) {
		t.Fatalf("replayed nonce through NewPreCheckDeps must refuse as ErrReplay, got %v", err)
	}
}

func writeTrustStoreFixture(t *testing.T, dir, cell string, pub ed25519.PublicKey) string {
	t.Helper()
	m := map[string]string{cell: base64.StdEncoding.EncodeToString(pub)}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal trust store: %v", err)
	}
	p := filepath.Join(dir, "trust.json")
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatalf("write trust store: %v", err)
	}
	return p
}
