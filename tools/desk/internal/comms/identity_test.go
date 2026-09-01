package comms

import (
	"testing"
	"time"
)

// identity_test.go — assertion mint/verify, and the crypto-side refusal battery.
// Each refusal asserts a DISTINCT typed error, because the sweep and the bypass
// battery need to tell them apart.

var testNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// TestMintVerifyRoundTrip is the positive control: a freshly minted assertion
// verifies. Without it, a battery of refusals could all pass against a Verify
// that refuses everything.
func TestMintVerifyRoundTrip(t *testing.T) {
	pub, signer := newKeypair(t)
	a, err := Mint("cell-a", "worker-desk", "msg-1", "nonce-1", testNow, time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	trust := Ed25519TrustStore{"cell-a": pub}
	if err := Verify(a, testNow.Add(10*time.Second), trust, DefaultSkew, NewMemReplayGuard()); err != nil {
		t.Fatalf("a freshly minted assertion did not verify: %v", err)
	}
}

// TestRefuseBadSignature: a tampered signature is refused with ErrBadSignature.
func TestRefuseBadSignature(t *testing.T) {
	pub, signer := newKeypair(t)
	a, err := Mint("cell-a", "worker-desk", "msg-1", "nonce-1", testNow, time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	a.Sig[0] ^= 0xff // flip a byte
	trust := Ed25519TrustStore{"cell-a": pub}
	err = Verify(a, testNow.Add(10*time.Second), trust, DefaultSkew, NewMemReplayGuard())
	if !isErr(err, ErrBadSignature) {
		t.Fatalf("want ErrBadSignature, got %v", err)
	}
}

// TestRefuseExpired: an assertion past its window is refused with ErrExpired.
func TestRefuseExpired(t *testing.T) {
	pub, signer := newKeypair(t)
	a, err := Mint("cell-a", "worker-desk", "msg-1", "nonce-1", testNow, 30*time.Second, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	trust := Ed25519TrustStore{"cell-a": pub}
	// now is a full minute past issue → past the 30s window.
	err = Verify(a, testNow.Add(time.Minute), trust, DefaultSkew, NewMemReplayGuard())
	if !isErr(err, ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

// TestRefuseReplayed: a second verification of the same nonce is refused with
// ErrReplay — single-use is enforced by the shared replay guard.
func TestRefuseReplayed(t *testing.T) {
	pub, signer := newKeypair(t)
	a, err := Mint("cell-a", "worker-desk", "msg-1", "nonce-1", testNow, time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	trust := Ed25519TrustStore{"cell-a": pub}
	guard := NewMemReplayGuard()
	// Pin the guard's prune clock to the synthetic test time so the still-valid
	// nonce (exp = testNow+1m) is not pruned between the two Use calls by the real
	// wall clock. In production, mint times are real-future, so the wall-clock
	// prune is correct; only the fixed synthetic testNow needs this.
	guard.now = func() time.Time { return testNow }
	if err := Verify(a, testNow.Add(10*time.Second), trust, DefaultSkew, guard); err != nil {
		t.Fatalf("first verification should succeed: %v", err)
	}
	err = Verify(a, testNow.Add(11*time.Second), trust, DefaultSkew, guard)
	if !isErr(err, ErrReplay) {
		t.Fatalf("want ErrReplay on the second use of nonce-1, got %v", err)
	}
}

// TestRefuseUnknownCell: an assertion from a cell the trust store does not
// recognise is refused with ErrUnknownCell, before any crypto runs.
func TestRefuseUnknownCell(t *testing.T) {
	_, signer := newKeypair(t)
	a, err := Mint("cell-x", "the-desk", "msg-1", "nonce-1", testNow, time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// trust store recognises cell-a only, not cell-x.
	pub, _ := newKeypair(t)
	trust := Ed25519TrustStore{"cell-a": pub}
	err = Verify(a, testNow.Add(10*time.Second), trust, DefaultSkew, NewMemReplayGuard())
	if !isErr(err, ErrUnknownCell) {
		t.Fatalf("want ErrUnknownCell, got %v", err)
	}
}

// TestRefuseNotYetValid: an assertion issued further ahead than the allowed skew
// is refused with ErrNotYetValid.
func TestRefuseNotYetValid(t *testing.T) {
	pub, signer := newKeypair(t)
	// issued 10 minutes in the future relative to the verifier's clock.
	future := testNow.Add(10 * time.Minute)
	a, err := Mint("cell-a", "worker-desk", "msg-1", "nonce-1", future, time.Minute, signer)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	trust := Ed25519TrustStore{"cell-a": pub}
	err = Verify(a, testNow, trust, time.Minute, NewMemReplayGuard())
	if !isErr(err, ErrNotYetValid) {
		t.Fatalf("want ErrNotYetValid, got %v", err)
	}
}

// TestRefuseMintEmptyInput: a mint missing a required field is refused rather than
// producing an assertion no verifier could accept.
func TestRefuseMintEmptyInput(t *testing.T) {
	_, signer := newKeypair(t)
	if _, err := Mint("cell-a", "", "msg-1", "nonce-1", testNow, time.Minute, signer); !isErr(err, ErrMintInput) {
		t.Fatalf("want ErrMintInput for an empty role, got %v", err)
	}
}
