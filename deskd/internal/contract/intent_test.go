package contract

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func freshIntent(action Action, issue string, now time.Time) SignedIntent {
	return SignedIntent{
		Action:     action,
		Issue:      issue,
		Login:      "operator",
		OptionID:   "ship-it",
		OptionsRev: "abc123",
		IssuedAt:   now.Add(-time.Minute),
		ExpiresAt:  now.Add(5 * time.Minute),
		Nonce:      "n-0001",
	}
}

// TestSignedIntentValidVerifies: a well-formed, in-window intent signed by the
// client's key verifies for exactly its action + resource.
func TestSignedIntentValidVerifies(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(ActionAnswer, "https://github.com/o/r/issues/7", now)
	si.Sign(priv)
	if err := si.Verify(pub, now, ActionAnswer, "https://github.com/o/r/issues/7"); err != nil {
		t.Fatalf("valid intent did not verify: %v", err)
	}
}

// TestSignedIntentTamperedFails: any change to the signed record after signing
// breaks the signature.
func TestSignedIntentTamperedFails(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(ActionAnswer, "https://github.com/o/r/issues/7", now)
	si.Sign(priv)
	si.OptionID = "reject" // tamper after signing
	if err := si.Verify(pub, now, ActionAnswer, "https://github.com/o/r/issues/7"); !errors.Is(err, ErrIntentBadSignature) {
		t.Fatalf("tampered intent verified (or wrong error): %v", err)
	}
}

// TestSignedIntentExpiredFails: an intent past its exp does not verify.
func TestSignedIntentExpiredFails(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(ActionAnswer, "https://github.com/o/r/issues/7", now)
	si.ExpiresAt = now.Add(-time.Second)
	si.Sign(priv)
	if err := si.Verify(pub, now, ActionAnswer, "https://github.com/o/r/issues/7"); !errors.Is(err, ErrIntentExpired) {
		t.Fatalf("expired intent verified (or wrong error): %v", err)
	}
}

// TestSignedIntentActionMismatch: an intent naming action X is
// refused when action Y is being invoked — even with a valid signature and in
// window. This is the per-action scope property of custody (b).
func TestSignedIntentActionMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(ActionAnswer, "https://github.com/o/r/issues/7", now)
	si.Sign(priv)
	if err := si.Verify(pub, now, ActionAck, "https://github.com/o/r/issues/7"); !errors.Is(err, ErrIntentActionScope) {
		t.Fatalf("an answer-intent authorized an ack (or wrong error): %v", err)
	}
}

// TestSignedIntentResourceBinding: an intent for one resource cannot authorize
// a write against another.
func TestSignedIntentResourceBinding(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(ActionAnswer, "https://github.com/o/r/issues/7", now)
	si.Sign(priv)
	if err := si.Verify(pub, now, ActionAnswer, "https://github.com/o/r/issues/999"); !errors.Is(err, ErrIntentResource) {
		t.Fatalf("an intent for issue 7 authorized a write on issue 999 (or wrong error): %v", err)
	}
}

// TestSignedIntentUnknownActionRefused: an intent naming an action outside the
// closed allow-list is refused before any signature check.
func TestSignedIntentUnknownActionRefused(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	now := time.Now()
	si := freshIntent(Action("close"), "https://github.com/o/r/issues/7", now)
	si.Sign(priv)
	if err := si.Verify(pub, now, Action("close"), "https://github.com/o/r/issues/7"); !errors.Is(err, ErrIntentNotNamed) {
		t.Fatalf("an intent for a non-allow-listed action verified (or wrong error): %v", err)
	}
}
