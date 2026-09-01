package comms

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// identity.go — the envelope-level peer identity: mint a short-TTL signed
// assertion binding {cell, role} to one message, and verify-or-refuse it.
//
// This is the SECOND, finer-grained identity check that sits on top of the
// gateway-to-gateway transport trust (mTLS, which lands with the gateway
// process). The transport proves "this connection comes from a gateway the trust
// store recognises"; this assertion proves "this specific message was minted by a
// specific role inside that cell", so a compromised or confused desk process
// inside an otherwise-legitimate connection still cannot forge another role's
// message.
//
// KEY CUSTODY. The signing key is per-role operator config under the App-PEM
// custody rules — mode 0600, never committed, never in the source tree. This file
// models the mint/verify mechanism and a trust-store SHAPE (cell -> public key);
// it holds no key and no key path. A concrete Ed25519 signer/trust-store is
// provided for callers and tests, but the KEY MATERIAL is always supplied from
// outside.

// Assertion is a short-TTL signed statement that binds a {cell, role} identity to
// exactly one message (by its id), for a bounded window, once. It is carried as
// the envelope's Sig block.
//
// The signature covers the canonical bytes of {cell, role, msgID, nonce, issued,
// expires} — see canonicalAssertionBytes. Every field the verifier relies on is
// therefore signed; a field an attacker could alter without invalidating the
// signature would be a field the verifier must not trust, so there is none.
type Assertion struct {
	// Cell is the asserting cell — the key used to verify Sig is looked up by it.
	Cell string `json:"cell"`
	// Role is the asserting desk role inside Cell.
	Role string `json:"role"`
	// MsgID binds the assertion to ONE envelope (its id). VerifyEnvelope refuses a
	// mismatch, so a valid assertion cannot be lifted onto another message.
	MsgID string `json:"msgId"`
	// Nonce is the single-use id. The replay guard consumes it on first use, so a
	// captured, still-valid, correctly-signed assertion cannot be replayed.
	Nonce string `json:"nonce"`
	// IssuedAt / ExpiresAt bound the validity window. Both are truncated to whole
	// seconds at mint, so the signed canonical form (which uses Unix seconds) fully
	// determines them and no sub-second bits ride along unsigned.
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	// Sig is the signature over canonicalAssertionBytes.
	Sig []byte `json:"sig"`
}

// Distinct typed refusal errors. The battery and the daily sweep need to tell an
// expired assertion from a replayed one from a forged one — a single "auth
// failed" error would erase exactly the distinction an incident review turns on.
var (
	// ErrUnknownCell — the asserting cell is not in the trust store, so there is
	// no key to verify against. Checked first: an unknown cell is refused before
	// any crypto runs.
	ErrUnknownCell = errors.New("comms: asserting cell is not in the trust store")
	// ErrBadSignature — the signature does not verify against the cell's key.
	ErrBadSignature = errors.New("comms: assertion signature does not verify")
	// ErrExpired — the assertion's window has closed (now is past ExpiresAt).
	ErrExpired = errors.New("comms: assertion has expired")
	// ErrNotYetValid — the assertion is issued further in the future than the
	// allowed clock skew. A far-future IssuedAt is refused, not honoured.
	ErrNotYetValid = errors.New("comms: assertion is not yet valid (issued beyond the allowed clock skew)")
	// ErrReplay — the nonce has already been consumed. Single-use is enforced.
	ErrReplay = errors.New("comms: assertion nonce has already been used (replay refused)")
	// ErrIdentityMismatch — the assertion's bound identity/message disagrees with
	// the envelope that carries it (raised by VerifyEnvelope).
	ErrIdentityMismatch = errors.New("comms: assertion does not match the envelope's declared sender")
	// ErrMintInput — a required mint input (cell, role, msgId, nonce, ttl) is
	// missing or invalid. A malformed mint is refused rather than producing an
	// assertion no verifier could ever accept.
	ErrMintInput = errors.New("comms: invalid assertion mint input")
)

// DefaultTTL is the default assertion lifetime when a caller passes a zero ttl to
// Mint. Short by design: an assertion is bound to one message and consumed once,
// so its window need only cover mint-to-delivery, not a session.
const DefaultTTL = 2 * time.Minute

// DefaultSkew is the default clock-skew tolerance for Verify's not-yet-valid
// check when a caller passes zero. It bounds how far ahead of the verifier's
// clock a mint may legitimately be.
const DefaultSkew = 1 * time.Minute

// Signer signs the canonical bytes of an assertion. It is an interface so the
// KEY never enters this package: a caller supplies a signer backed by the role's
// custody-managed private key. Sign must be deterministic over its input for a
// given key (ed25519 is), so a re-signed assertion verifies identically.
type Signer interface {
	Sign(msg []byte) ([]byte, error)
}

// TrustStore resolves a cell's public verifying key. A cell absent from the store
// is UNKNOWN and fails closed (ErrUnknownCell) — it is never treated as "no key
// required".
type TrustStore interface {
	PublicKey(cell string) (ed25519.PublicKey, bool)
}

// Ed25519Signer is the concrete signer over an Ed25519 private key. The key is
// passed in by the caller from custody-managed material; this type does not read,
// write, or persist it.
type Ed25519Signer struct {
	Key ed25519.PrivateKey
}

// Sign implements Signer.
func (s Ed25519Signer) Sign(msg []byte) ([]byte, error) {
	if len(s.Key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: ed25519 private key is %d bytes, want %d", ErrMintInput, len(s.Key), ed25519.PrivateKeySize)
	}
	return ed25519.Sign(s.Key, msg), nil
}

// Ed25519TrustStore is a concrete cell -> public key trust store. It is a plain
// map so a caller can build it from custody-managed public keys; the keys are the
// only thing that grants a cell recognition, and they come from outside.
type Ed25519TrustStore map[string]ed25519.PublicKey

// PublicKey implements TrustStore.
func (t Ed25519TrustStore) PublicKey(cell string) (ed25519.PublicKey, bool) {
	k, ok := t[cell]
	if !ok || len(k) != ed25519.PublicKeySize {
		return nil, false
	}
	return k, true
}

// Mint builds and signs an assertion. It refuses malformed input rather than
// producing an assertion no verifier could accept. Times are truncated to whole
// seconds so the signed canonical form fully determines them.
func Mint(cell, role, msgID, nonce string, issued time.Time, ttl time.Duration, signer Signer) (Assertion, error) {
	if signer == nil {
		return Assertion{}, fmt.Errorf("%w: nil signer", ErrMintInput)
	}
	for _, f := range []struct{ name, val string }{
		{"cell", cell}, {"role", role}, {"msgId", msgID}, {"nonce", nonce},
	} {
		if strings.TrimSpace(f.val) == "" {
			return Assertion{}, fmt.Errorf("%w: %s is empty", ErrMintInput, f.name)
		}
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	iss := issued.UTC().Truncate(time.Second)
	a := Assertion{
		Cell:      cell,
		Role:      role,
		MsgID:     msgID,
		Nonce:     nonce,
		IssuedAt:  iss,
		ExpiresAt: iss.Add(ttl),
	}
	sig, err := signer.Sign(canonicalAssertionBytes(a))
	if err != nil {
		return Assertion{}, err
	}
	a.Sig = sig
	return a, nil
}

// Verify is verify-or-refuse for an assertion. It runs the checks in a deliberate
// order — cheapest and most fundamental first — and returns a DISTINCT typed
// error for each failure mode:
//
//  1. unknown cell (no key to verify against) — ErrUnknownCell;
//  2. signature does not verify — ErrBadSignature;
//  3. window closed — ErrExpired;
//  4. issued beyond the allowed skew — ErrNotYetValid;
//  5. nonce already consumed — ErrReplay.
//
// The signature is checked BEFORE the nonce is consumed: an unsigned or forged
// assertion must never be able to burn a legitimate nonce (a denial-of-service on
// the real sender). replay may be nil to skip the single-use check — but a nil
// replay guard means an assertion can be replayed, so a real gateway always
// supplies one; only a caller that has already de-duplicated upstream passes nil.
func Verify(a Assertion, now time.Time, trust TrustStore, skew time.Duration, replay ReplayGuard) error {
	if trust == nil {
		return fmt.Errorf("%w: nil trust store", ErrUnknownCell)
	}
	pub, ok := trust.PublicKey(a.Cell)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownCell, a.Cell)
	}
	if !ed25519.Verify(pub, canonicalAssertionBytes(a), a.Sig) {
		return ErrBadSignature
	}
	if now.After(a.ExpiresAt) {
		return fmt.Errorf("%w: expired at %s, now %s", ErrExpired, a.ExpiresAt.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	if skew <= 0 {
		skew = DefaultSkew
	}
	if now.Add(skew).Before(a.IssuedAt) {
		return fmt.Errorf("%w: issued at %s, now %s, skew %s", ErrNotYetValid, a.IssuedAt.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), skew)
	}
	if replay != nil && !replay.Use(a.Nonce, a.ExpiresAt) {
		return fmt.Errorf("%w: nonce %q", ErrReplay, a.Nonce)
	}
	return nil
}

// canonicalAssertionBytes is the deterministic byte string the signature covers.
// It is length-prefixed, not delimiter-joined: a role of "a\x00b" and a cell of
// "b" must never canonicalise to the same bytes as a cell of "a" and a role of
// "\x00b", which any separator-joined encoding would allow. A domain-separation
// prefix stops these bytes from ever being confused with some other signed
// structure that happened to share a layout.
func canonicalAssertionBytes(a Assertion) []byte {
	var b []byte
	b = append(b, "comms/assertion/cellmsg-v1\n"...)
	b = appendField(b, a.Cell)
	b = appendField(b, a.Role)
	b = appendField(b, a.MsgID)
	b = appendField(b, a.Nonce)
	b = appendUint(b, uint64(a.IssuedAt.UTC().Unix()))
	b = appendUint(b, uint64(a.ExpiresAt.UTC().Unix()))
	return b
}

func appendField(b []byte, s string) []byte {
	b = appendUint(b, uint64(len(s)))
	return append(b, s...)
}

func appendUint(b []byte, n uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	return append(b, buf[:]...)
}

// ReplayGuard enforces single-use of assertion nonces. Use records nonce as
// consumed and reports whether it was FRESH: true on first sight (and recorded),
// false if it was already used (a replay). The exp is passed so an implementation
// can prune a nonce once its assertion could no longer be valid anyway.
type ReplayGuard interface {
	Use(nonce string, exp time.Time) bool
}

// MemReplayGuard is an in-memory, concurrency-safe ReplayGuard. It keeps each
// consumed nonce until its assertion's expiry, then prunes it — a nonce whose
// assertion has expired can no longer verify (ErrExpired fires first), so
// forgetting it cannot re-open a replay window. It is the guard a single gateway
// process uses; a multi-process deployment substitutes a shared-store guard
// behind the same interface.
type MemReplayGuard struct {
	mu   sync.Mutex
	seen map[string]time.Time
	now  func() time.Time // test hook; nil -> time.Now
}

// NewMemReplayGuard returns an empty in-memory replay guard.
func NewMemReplayGuard() *MemReplayGuard {
	return &MemReplayGuard{seen: make(map[string]time.Time)}
}

func (g *MemReplayGuard) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

// Use implements ReplayGuard.
func (g *MemReplayGuard) Use(nonce string, exp time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.seen == nil {
		g.seen = make(map[string]time.Time)
	}
	now := g.clock()
	// Prune nonces whose window has closed; keeping them is pure memory cost.
	for k, e := range g.seen {
		if now.After(e) {
			delete(g.seen, k)
		}
	}
	if _, used := g.seen[nonce]; used {
		return false
	}
	g.seen[nonce] = exp
	return true
}
