package contract

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// SignedIntent is the custody-rule-(b) ALTERNATIVE to a per-request credential:
// a typed, CLIENT-SIGNED record naming exactly one action + resource + the
// decision. The client signs it with its private key and keeps the raw write
// credential on the client; the service verifies it against the client's public
// key and performs the write. It carries a SIGNATURE over a canonical record —
// NEVER a raw token (which is why [TestNoCredentialShapedFields] passes over it).
//
// v1 uses ed25519 as the signature primitive so the private key never leaves
// the client. The attribution point this comment used to leave open is RULED
// (the authorization contract gap 3): a signed-intent write runs under the bot gateway
// credential and is a bot comment CARRYING the human record — the verified
// signature is the gate:human act, the bot only transports it, and the signer is
// recorded on the answer record and the audit line. The service honours an
// intent only against a key enrolled for its login in the reviewed
// config/intent-keys.yaml (internal/answer.IntentVerifier); [Verify] here is the
// signature/scope/time check that verifier runs, not the trust decision.
type SignedIntent struct {
	Action     Action          `json:"action"`
	Issue      string          `json:"issue"` // platform deep-link, never issue.number
	Login      string          `json:"login"`
	OptionID   domain.OptionID `json:"option_id,omitempty"`
	OptionsRev string          `json:"options_rev"`
	IssuedAt   time.Time       `json:"iat"`
	ExpiresAt  time.Time       `json:"exp"`
	Nonce      string          `json:"nonce"`
	// Signature is ed25519 over [SignedIntent.signingBytes]. It is a signature
	// over the record above, not a credential — the raw write token stays on the
	// client.
	Signature []byte `json:"signature,omitempty"`
}

// Errors from intent verification. First-class so a handler maps each to a
// status and a log line without string matching.
var (
	ErrIntentBadSignature = errors.New("signed intent: signature does not verify against the presented key")
	ErrIntentExpired      = errors.New("signed intent: expired (now is past exp)")
	ErrIntentNotYetValid  = errors.New("signed intent: not yet valid (now is before iat)")
	ErrIntentActionScope  = errors.New("signed intent: names a different action than the one being invoked — an intent for one action cannot authorize another")
	ErrIntentResource     = errors.New("signed intent: names a different resource than the one being acted on")
	ErrIntentNotNamed     = errors.New("signed intent: action is not in the enumerated named-action allow-list")
)

// signingBytes is the CANONICAL serialization the signature covers: every field
// except Signature, length-prefixed and separated so no field content can forge
// a boundary (the same defense domain.OptionsRev uses for agent-authored text).
// Both Sign and Verify must build it identically, so it lives in exactly one
// place.
func (si SignedIntent) signingBytes() []byte {
	var b bytes.Buffer
	write := func(part string) {
		fmt.Fprintf(&b, "%d\x1f%s\x1f", len(part), part)
	}
	write("assay:intent:v1")
	write(string(si.Action))
	write(si.Issue)
	write(si.Login)
	write(string(si.OptionID))
	write(si.OptionsRev)
	write(si.IssuedAt.UTC().Format(time.RFC3339Nano))
	write(si.ExpiresAt.UTC().Format(time.RFC3339Nano))
	write(si.Nonce)
	b.WriteByte(0x1e)
	return b.Bytes()
}

// Sign fills Signature with an ed25519 signature over the canonical record.
func (si *SignedIntent) Sign(priv ed25519.PrivateKey) {
	si.Signature = ed25519.Sign(priv, si.signingBytes())
}

// Verify checks the intent against the action and resource actually being
// invoked. It fails — with a specific sentinel — when the signature does not
// verify, the intent is expired or not yet valid, the intent names a different
// action (X cannot authorize Y), the intent names a different resource, or the
// action is outside the closed allow-list. A nil error means the service may
// perform exactly wantAction on wantIssue on the person's behalf.
func (si SignedIntent) Verify(pub ed25519.PublicKey, now time.Time, wantAction Action, wantIssue string) error {
	if !IsNamedAction(si.Action) {
		return fmt.Errorf("%w: %q", ErrIntentNotNamed, si.Action)
	}
	if si.Action != wantAction {
		return fmt.Errorf("%w: intent=%q invoked=%q", ErrIntentActionScope, si.Action, wantAction)
	}
	if si.Issue != wantIssue {
		return fmt.Errorf("%w: intent=%q acting-on=%q", ErrIntentResource, si.Issue, wantIssue)
	}
	if now.Before(si.IssuedAt) {
		return ErrIntentNotYetValid
	}
	if now.After(si.ExpiresAt) {
		return ErrIntentExpired
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, si.signingBytes(), si.Signature) {
		return ErrIntentBadSignature
	}
	return nil
}
