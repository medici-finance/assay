package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskHome points the tool at a fresh HOME (so its desk-tools/claims dir and audit log are
// isolated) and neutralises the ambient kill-switch / session env, mirroring the other cmd
// suites.
func deskHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "test-session")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	return home
}

func claimFile(home, item string) string {
	// Mirror deskkit.claimPath's segment sanitizer for the items these tests use (none of
	// which contain a parent-dir segment).
	safe := strings.ReplaceAll(item, "/", "_")
	return filepath.Join(home, ".config", "assay", "claims", safe+".claim")
}

func TestAcquireThenCollision(t *testing.T) {
	home := deskHome(t)

	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "stream-a/01"}); rc != deskkit.ExitOK {
		t.Fatalf("first acquire rc = %d, want 0", rc)
	}
	// The claim file exists at the sanitized path with the canonical shape.
	raw, err := os.ReadFile(claimFile(home, "stream-a/01"))
	if err != nil {
		t.Fatalf("claim file: %v", err)
	}
	var c deskkit.Claim
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse claim: %v", err)
	}
	if c.Kind != "dispatch" || c.Item != "stream-a/01" || c.Owner != "test-session" {
		t.Fatalf("claim shape wrong: %+v", c)
	}

	// A second acquire of the same LIVE item is refused (exit 5) — do not proceed.
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "stream-a/01"}); rc != deskkit.ExitRefused {
		t.Fatalf("collision rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
}

func TestAcquireBadKindRefused(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"acquire", "--kind", "bogus", "--item", "x"}); rc != deskkit.ExitRefused {
		t.Fatalf("bad kind rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestAcquireMissingFlagsRefused(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"acquire", "--kind", "dispatch"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing --item rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	if rc := run([]string{"acquire", "--item", "x"}); rc != deskkit.ExitRefused {
		t.Fatalf("missing --kind rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestReleaseThenReacquire(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "a"}); rc != deskkit.ExitOK {
		t.Fatalf("acquire rc = %d", rc)
	}
	if rc := run([]string{"release", "--item", "a"}); rc != deskkit.ExitOK {
		t.Fatalf("release rc = %d, want 0", rc)
	}
	// Releasing a missing claim is a no-op (exit 0).
	if rc := run([]string{"release", "--item", "a"}); rc != deskkit.ExitOK {
		t.Fatalf("release-missing rc = %d, want 0 (no-op)", rc)
	}
	// After release the item is claimable again.
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "a"}); rc != deskkit.ExitOK {
		t.Fatalf("re-acquire rc = %d, want 0", rc)
	}
}

func TestListReadsEveryWriterShape(t *testing.T) {
	home := deskHome(t)
	dir := filepath.Join(home, ".config", "assay", "claims")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Plant a legacy loopengine-shape claim and a legacy roster/bash-shape claim; list must
	// read both through the tolerant reader.
	if err := os.WriteFile(filepath.Join(dir, "stream-a_07.claim"),
		[]byte(`{"itemID":"stream-a/07","runner":"batch","branch":"feat/y","claimed":"2026-08-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "desk-tools--09.claim"),
		[]byte(`{"brief":"example/09","repo":"assay","session":"worker-a","ts":"2026-07-17T13:52:05Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// And a canonical one via the CLI itself.
	if rc := run([]string{"acquire", "--kind", "verify", "--item", "issue-841", "--owner", "verify-desk"}); rc != deskkit.ExitOK {
		t.Fatalf("acquire rc = %d", rc)
	}

	if rc := run([]string{"list"}); rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
}

func TestListEmptyIsOK(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"list"}); rc != deskkit.ExitOK {
		t.Fatalf("empty list rc = %d, want 0", rc)
	}
}

// TestAcquireFailsClosedWhenLockUnplaceable is the CLI-level #146 guard: when the claims
// directory cannot be created (a FILE sits where the dir must be), the lock cannot be
// placed, so acquire is Unverifiable (exit 6) and writes NOTHING — it never "assumes free".
func TestAcquireFailsClosedWhenLockUnplaceable(t *testing.T) {
	home := deskHome(t)
	deskTools := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(deskTools, 0o700); err != nil {
		t.Fatal(err)
	}
	// A regular FILE named "claims" — MkdirAll(.../claims) then fails deterministically.
	if err := os.WriteFile(filepath.Join(deskTools, "claims"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "x"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("acquire rc = %d, want %d (unverifiable) — a lock it cannot place is not permission to proceed", rc, deskkit.ExitUnverifiable)
	}
}

func TestUnknownVerbRefused(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"frobnicate"}); rc != deskkit.ExitRefused {
		t.Fatalf("unknown verb rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}

func TestVersionAndHelpAreUnguardedReads(t *testing.T) {
	deskHome(t)
	if rc := run([]string{"--version"}); rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
	if rc := run([]string{"help"}); rc != deskkit.ExitOK {
		t.Fatalf("help rc = %d, want 0", rc)
	}
	if rc := run(nil); rc != deskkit.ExitRefused {
		t.Fatalf("no-args rc = %d, want %d", rc, deskkit.ExitRefused)
	}
}
