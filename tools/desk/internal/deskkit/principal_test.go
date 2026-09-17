package deskkit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestPrincipalNeverFromEnv is Verify row 1 of multi-principal/01: DESK_PRINCIPAL is a
// FAKE env var this resolver must never read. With no roster configured it must resolve
// nothing (exit 5, Refused) even with DESK_PRINCIPAL=evil exported; adding the roster
// (ASSAY_BLESS_LOGIN, config-home file, no env involved) must resolve to the bless login
// — proving the env var never had any effect either way.
func TestPrincipalNeverFromEnv(t *testing.T) {
	t.Setenv("DESK_PRINCIPAL", "evil")

	// 1. No roster configured: unresolvable, exit 5.
	plantRoster(t, "")
	_, err := ResolvePrincipal("")
	if err == nil {
		t.Fatalf("ResolvePrincipal() with no roster and DESK_PRINCIPAL=evil = nil error, want a refusal")
	}
	var derr *DeskError
	if !errors.As(err, &derr) {
		t.Fatalf("ResolvePrincipal() error = %v (%T), want a *DeskError", err, err)
	}
	if derr.Code != ExitRefused {
		t.Fatalf("ResolvePrincipal() error code = %d, want ExitRefused (%d)", derr.Code, ExitRefused)
	}

	// 2. Perturb: add the roster. Resolves to the bless login — DESK_PRINCIPAL=evil is
	// still exported and must have contributed nothing.
	plantRoster(t, raisedByFixtureRoster)
	p, err := ResolvePrincipal("")
	if err != nil {
		t.Fatalf("ResolvePrincipal() with roster configured = %v, want a resolved principal", err)
	}
	if p.Login != "ada" {
		t.Fatalf("ResolvePrincipal().Login = %q, want %q (the roster's bless login, never DESK_PRINCIPAL)",
			p.Login, "ada")
	}
	if !p.State.Resolved() {
		t.Fatalf("ResolvePrincipal().State = %v, want a resolved state", p.State)
	}
}

// TestPrincipalUnattendedByDefault: with no roster beacon for the resolved session, the
// principal still resolves (to the bless login) but is tagged unattended, and the
// rendered trailer line carries mode:unattended.
func TestPrincipalUnattendedByDefault(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())
	p, err := ResolvePrincipal("")
	if err != nil {
		t.Fatalf("ResolvePrincipal() = %v, want a resolved principal", err)
	}
	if p.State != PrincipalUnattended {
		t.Fatalf("ResolvePrincipal().State = %v, want PrincipalUnattended (no beacon planted)", p.State)
	}
	if got, want := p.Line(), "On-behalf-of: human:ada mode:unattended"; got != want {
		t.Fatalf("Principal.Line() = %q, want %q", got, want)
	}
}

// TestPrincipalAttendedWithLiveBeacon: a session with a roster beacon carrying a
// non-empty role resolves attended, and the rendered trailer carries no mode suffix.
func TestPrincipalAttendedWithLiveBeacon(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	session := "sess-" + t.Name()
	t.Setenv("DESK_SESSION", session)
	// deskroster (cmd/deskroster, a separate package) is the writer of the `role` key in
	// normal operation (`deskroster set --role`); write it directly here so this test does
	// not depend on that binary.
	path, err := AckBeaconPath(session)
	if err != nil {
		t.Fatalf("AckBeaconPath: %v", err)
	}
	if werr := writeTestBeaconRole(path, "worker-desk"); werr != nil {
		t.Fatalf("writeTestBeaconRole: %v", werr)
	}

	p, err := ResolvePrincipal("")
	if err != nil {
		t.Fatalf("ResolvePrincipal() = %v, want a resolved principal", err)
	}
	if p.State != PrincipalAttended {
		t.Fatalf("ResolvePrincipal().State = %v, want PrincipalAttended (live beacon with role)", p.State)
	}
	if got, want := p.Line(), "On-behalf-of: human:ada"; got != want {
		t.Fatalf("Principal.Line() = %q, want %q", got, want)
	}
}

func writeTestBeaconRole(path, role string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"session": "x", "role": role})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
