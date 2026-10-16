package comms

import (
	"time"

	"github.com/medici-finance/assay/comms/contract"
)

// identity.go — the envelope-level peer identity (short-TTL signed assertion
// minted per message, verify-or-refuse) lives in the public contract module,
// github.com/medici-finance/assay/comms/contract. This file keeps the names this
// package's consumers have always used as thin aliases so they compile unchanged;
// there is ONE implementation (contract), and these names add none.
//
// The aliases are consumed by cmd/commsgw, cmd/commsloop, cmd/deskcomms and
// internal/commsqueue. A new consumer should import contract directly.
//
// KEY CUSTODY is unchanged: the mechanism holds no key and no key path; the
// signing key is per-role operator config under the App-PEM custody rules.

// Assertion is the signed identity assertion an envelope carries.
type Assertion = contract.Assertion

// Signer signs canonical assertion bytes; the key is supplied from outside.
type Signer = contract.Signer

// TrustStore resolves a cell to its public key.
type TrustStore = contract.TrustStore

// Ed25519Signer is the concrete signer over a caller-supplied private key.
type Ed25519Signer = contract.Ed25519Signer

// Ed25519TrustStore is the concrete cell -> public key trust store.
type Ed25519TrustStore = contract.Ed25519TrustStore

// ReplayGuard enforces single-use nonces.
type ReplayGuard = contract.ReplayGuard

// MemReplayGuard is the in-memory ReplayGuard.
type MemReplayGuard = contract.MemReplayGuard

// Assertion defaults.
const (
	DefaultTTL  = contract.DefaultTTL
	DefaultSkew = contract.DefaultSkew
)

// Identity refusals (distinct, typed).
var (
	ErrUnknownCell      = contract.ErrUnknownCell
	ErrBadSignature     = contract.ErrBadSignature
	ErrExpired          = contract.ErrExpired
	ErrNotYetValid      = contract.ErrNotYetValid
	ErrReplay           = contract.ErrReplay
	ErrIdentityMismatch = contract.ErrIdentityMismatch
	ErrMintInput        = contract.ErrMintInput
)

// Mint builds and signs an assertion; see contract.Mint.
func Mint(cell, role, msgID, nonce string, issued time.Time, ttl time.Duration, signer Signer) (Assertion, error) {
	return contract.Mint(cell, role, msgID, nonce, issued, ttl, signer)
}

// Verify is the full assertion verification; see contract.Verify.
func Verify(a Assertion, now time.Time, trust TrustStore, skew time.Duration, replay ReplayGuard) error {
	return contract.Verify(a, now, trust, skew, replay)
}

// VerifyIdentityOnly verifies the signature without the replay guard; see
// contract.VerifyIdentityOnly.
func VerifyIdentityOnly(a Assertion, trust TrustStore) error {
	return contract.VerifyIdentityOnly(a, trust)
}

// NewMemReplayGuard returns an empty in-memory replay guard.
func NewMemReplayGuard() *MemReplayGuard { return contract.NewMemReplayGuard() }

// NewMemReplayGuardWithClock returns a guard pinned to now (nil = time.Now).
func NewMemReplayGuardWithClock(now func() time.Time) *MemReplayGuard {
	return contract.NewMemReplayGuardWithClock(now)
}

// canonicalAssertionBytes is the in-package spelling the byte-layout tests pin.
func canonicalAssertionBytes(a Assertion) []byte { return contract.CanonicalAssertionBytes(a) }
