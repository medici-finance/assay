package identity

import (
	"errors"
	"testing"
	"time"
)

func TestSessionStoreBeginValidate(t *testing.T) {
	clock := time.Now().UTC()
	st := NewSessionStore(func() time.Time { return clock })

	if _, err := st.Begin("", "tok"); err == nil {
		t.Fatal("begin with no login should fail")
	}
	if _, err := st.Begin("bob", ""); err == nil {
		t.Fatal("begin with no token should fail")
	}

	s, err := st.Begin("bob", "bob-token")
	if err != nil {
		t.Fatal(err)
	}
	if s.Login != "bob" || s.Token() != "bob-token" {
		t.Fatalf("session did not carry login/token: %+v", s)
	}
	if len(s.ID) < 32 {
		t.Fatalf("session id too short to be unguessable: %q", s.ID)
	}

	if _, err := st.Validate(""); !errors.Is(err, ErrNoSession) {
		t.Fatalf("empty id: %v", err)
	}
	if _, err := st.Validate("nope"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("unknown id: %v", err)
	}
	if got, err := st.Validate(s.ID); err != nil || got.Login != "bob" {
		t.Fatalf("valid session rejected: %v", err)
	}

	clock = clock.Add(2 * SessionTTL)
	if _, err := st.Validate(s.ID); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestSessionEndLogsOut(t *testing.T) {
	st := NewSessionStore(nil)
	s, _ := st.Begin("bob", "t")
	st.End(s.ID)
	if _, err := st.Validate(s.ID); !errors.Is(err, ErrNoSession) {
		t.Fatalf("ended session still validates: %v", err)
	}
}

// TestSessionIDsAreUnique guards against a predictable/duplicated bearer.
func TestSessionIDsAreUnique(t *testing.T) {
	st := NewSessionStore(nil)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		s, err := st.Begin("bob", "t")
		if err != nil {
			t.Fatal(err)
		}
		if seen[s.ID] {
			t.Fatalf("duplicate session id: %s", s.ID)
		}
		seen[s.ID] = true
	}
}

// TestTokenNeverSerialised proves the in-memory token is not exposed as an
// exported field (it is only reachable via the Token() method), so it cannot be
// marshalled into a response or template by accident (gate-why (5)).
func TestTokenNeverSerialised(t *testing.T) {
	// A struct-literal Session with an exported token field would let a JSON
	// encoder leak it; the field is unexported, so this compiles ONLY because
	// the test is in-package. External packages cannot set or read it.
	s := &Session{ID: "x", Login: "bob", token: "secret"}
	if s.Token() != "secret" {
		t.Fatal("Token() accessor broken")
	}
}
