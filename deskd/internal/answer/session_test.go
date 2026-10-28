package answer

import (
	"errors"
	"testing"
	"time"
)

// TestBeginRequiresCredential proves a live session cannot be minted with no
// operator credential held — a session that could never lead to a write is not a
// false promise of one.
func TestBeginRequiresCredential(t *testing.T) {
	st := NewSessionStore(time.Now, func() bool { return false })
	if _, err := st.Begin(); !errors.Is(err, ErrNoCredentialForSession) {
		t.Fatalf("expected ErrNoCredentialForSession, got %v", err)
	}
}

// TestLiveBitCannotBeForged proves the `live` bit is only ever set by Begin: a
// session assembled from a struct literal outside a Begin call is not live.
func TestLiveBitCannotBeForged(t *testing.T) {
	forged := &Session{ID: "x", Actor: humanActor(), ExpiresAt: time.Now().Add(time.Hour)}
	if forged.Live() {
		t.Fatal("a struct-literal session reported live — the gate is forgeable")
	}
	real, err := NewSessionStore(time.Now, func() bool { return true }).Begin()
	if err != nil {
		t.Fatal(err)
	}
	if !real.Live() {
		t.Fatal("a Begin-minted session is not live")
	}
}

// TestValidate covers the not-found, expired, and happy paths.
func TestValidate(t *testing.T) {
	now := time.Now().UTC()
	clock := now
	st := NewSessionStore(func() time.Time { return clock }, func() bool { return true })
	s, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.Validate(""); !errors.Is(err, ErrNoSession) {
		t.Fatalf("empty id: %v", err)
	}
	if _, err := st.Validate("nope"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("unknown id: %v", err)
	}
	if _, err := st.Validate(s.ID); err != nil {
		t.Fatalf("valid session rejected: %v", err)
	}

	clock = now.Add(2 * SessionTTL) // advance past expiry
	if _, err := st.Validate(s.ID); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

// TestSessionIDsAreUnique guards against a predictable/duplicated bearer.
func TestSessionIDsAreUnique(t *testing.T) {
	st := NewSessionStore(time.Now, func() bool { return true })
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s, err := st.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if seen[s.ID] {
			t.Fatalf("duplicate session id: %s", s.ID)
		}
		if len(s.ID) < 32 {
			t.Fatalf("session id too short to be unguessable: %q", s.ID)
		}
		seen[s.ID] = true
	}
}
