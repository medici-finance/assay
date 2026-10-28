package custody

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestNoCredentialIsFirstClass: an absent credential is an explicit error, not
// an empty token that would author as nobody.
func TestNoCredentialIsFirstClass(t *testing.T) {
	var zero *Credential
	if zero.Present() {
		t.Fatal("the zero credential reports present")
	}
	if _, err := zero.Token(); err != ErrNoCredential {
		t.Fatalf("Token() on the zero credential = %v, want ErrNoCredential", err)
	}
	if _, err := FromContext(context.Background()); err != ErrNoCredential {
		t.Fatalf("FromContext on a bare context = %v, want ErrNoCredential", err)
	}
	if _, err := FromToken("   ", Actor()); err != ErrNoCredential {
		t.Fatalf("FromToken with a blank token = %v, want ErrNoCredential", err)
	}
}

// TestWrongAccountTokenRefused: a credential that authenticates as some other
// account is refused at CONSTRUCTION, so it can never reach the writer. The
// at-rest store made this check when it loaded the file; under model (b) it
// runs on every request instead.
func TestWrongAccountTokenRefused(t *testing.T) {
	if _, err := FromToken("gho_fixture", "some-commenter"); err == nil {
		t.Fatal("a credential authenticating as some-commenter was accepted — it would author under the wrong identity")
	} else if !strings.Contains(err.Error(), "wrong identity") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
	c, err := FromToken("gho_fixture", Actor())
	if err != nil {
		t.Fatalf("the actor's own credential was refused: %v", err)
	}
	if c.TokenActor() != Actor() {
		t.Fatalf("TokenActor() = %q, want %q", c.TokenActor(), Actor())
	}
}

// TestCredentialTravelsOnlyInTheRequestContext: the credential round-trips
// through a request context and is unreachable from a context that did not
// carry it. Nothing else in the process can see it — there is no store to ask.
func TestCredentialTravelsOnlyInTheRequestContext(t *testing.T) {
	c, err := FromToken("gho_fixture_token", Actor())
	if err != nil {
		t.Fatalf("FromToken: %v", err)
	}
	ctx := NewContext(context.Background(), c)
	got, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("FromContext on the carrying context: %v", err)
	}
	tok, err := got.Token()
	if err != nil || tok != "gho_fixture_token" {
		t.Fatalf("Token() = %q, %v — the credential did not survive its own request", tok, err)
	}
	// A sibling context (a different request) sees nothing.
	if _, err := FromContext(context.Background()); err != ErrNoCredential {
		t.Fatalf("a context that never carried the credential can read it: %v", err)
	}
}

// TestStatusNeverLeaksTheToken: the non-secret status view carries no token
// value under any field name, and states the not-at-rest guarantee.
func TestStatusNeverLeaksTheToken(t *testing.T) {
	c, err := FromToken("gho_super_secret_fixture", Actor())
	if err != nil {
		t.Fatalf("FromToken: %v", err)
	}
	raw, err := json.Marshal(StatusOf(c))
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if strings.Contains(string(raw), "gho_super_secret_fixture") {
		t.Fatalf("the status view leaked the token: %s", raw)
	}
	var st Status
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if !st.Present || st.Actor != Actor() || st.AtRest {
		t.Fatalf("status = %+v, want present, actor %q, at_rest false", st, Actor())
	}
}

// TestCredentialHasNoSerialisableForm: the credential type exposes no exported
// field, so `json.Marshal(credential)` can never emit the token even if a later
// commit hands one to an encoder by accident.
func TestCredentialHasNoSerialisableForm(t *testing.T) {
	c, err := FromToken("gho_super_secret_fixture", Actor())
	if err != nil {
		t.Fatalf("FromToken: %v", err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "{}" {
		t.Fatalf("the credential serialises to %s, want {} — no field of it may ever reach a wire or a file", raw)
	}
}
