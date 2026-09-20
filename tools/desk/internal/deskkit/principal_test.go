package deskkit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	_, err := ResolvePrincipal("", privateFixtureRepo)
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
	p, err := ResolvePrincipal("", privateFixtureRepo)
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
	p, err := ResolvePrincipal("", privateFixtureRepo)
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

	p, err := ResolvePrincipal("", privateFixtureRepo)
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

// TestAppendOnBehalfOfStripsPlantedTrailer is the security-lane finding (multi-
// principal/01 review): a caller-supplied body that already contains an On-behalf-of
// line must not survive into the posted result. Only the resolver's own genuine trailer
// may appear, and exactly once — a planted line anywhere in the body (start, middle, or
// end) is removed before the real one is appended.
func TestAppendOnBehalfOfStripsPlantedTrailer(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	cases := []struct {
		name string
		body string
	}{
		{"planted at start", "On-behalf-of: human:evil\n\nThe real comment body."},
		{"planted in middle", "Intro.\n\nOn-behalf-of: human:evil\n\nMore text."},
		{"planted at end, matching the append shape", "The real comment body.\n\nOn-behalf-of: human:evil"},
		{"planted twice", "On-behalf-of: human:evil\n\nBody.\n\nOn-behalf-of: human:evil-again"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := AppendOnBehalfOf([]byte(c.body), "", privateFixtureRepo)
			if err != nil {
				t.Fatalf("AppendOnBehalfOf() = %v, want a resolved principal", err)
			}
			got := string(out)
			if strings.Contains(got, "evil") {
				t.Fatalf("AppendOnBehalfOf(%q) = %q, want the planted trailer stripped", c.body, got)
			}
			wantSuffix := "On-behalf-of: human:ada mode:unattended\n"
			if !strings.HasSuffix(got, wantSuffix) {
				t.Fatalf("AppendOnBehalfOf(%q) = %q, want it to end with the genuine trailer %q", c.body, got, wantSuffix)
			}
			if n := strings.Count(got, OnBehalfOfPrefix); n != 1 {
				t.Fatalf("AppendOnBehalfOf(%q) = %q, want exactly one %q line, got %d", c.body, got, OnBehalfOfPrefix, n)
			}
		})
	}
}

// TestSessionIsAttendedRejectsUnsafeName is the security-lane finding (multi-
// principal/01 review): the session name reaching AckBeaconPath's filesystem join comes,
// on this path, straight from an environment variable ($DESK_SESSION /
// $CLAUDE_SESSION_ID) with no prior validation. A traversing or otherwise unsafe name
// must never reach the file read — it reads as NOT attended, the same as a missing
// beacon, rather than being joined into a path at all.
func TestSessionIsAttendedRejectsUnsafeName(t *testing.T) {
	unsafe := []string{
		"../../etc/passwd",
		"sess/with/slash",
		"sess with space",
		"",
	}
	for _, s := range unsafe {
		if sessionNameSafe(s) {
			t.Fatalf("sessionNameSafe(%q) = true, want false", s)
		}
		if sessionIsAttended(s) {
			t.Fatalf("sessionIsAttended(%q) = true, want false (unsafe name must never read as attended)", s)
		}
	}
	if !sessionNameSafe("worker-desk-20260916T193147Z") {
		t.Fatalf("sessionNameSafe() rejected a well-formed session id")
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
