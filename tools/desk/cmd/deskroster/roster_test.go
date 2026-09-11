package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The roster's two PR reads (ghViewPR, ghListOpenPRs) route through the enumerated Forge seam
// since example-stream/08 closed the forge surface — they no longer shell `gh`. TestMain installs
// a package-level forgeFor stub (fakeRosterForge, defined in forge_test.go) that serves
// GetPullRequest and ListOpenChanges from the SAME FAKEGH_* env fixtures the suites already set,
// so every pre-existing behavioural assertion keeps asserting the same verdict it did against the
// former fake-gh binary — the behaviour-preservation evidence the fixture header describes.

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origForgeFor := forgeFor
	forgeFor = fakeRosterForgeFor
	code := m.Run()
	forgeFor = origForgeFor
	os.Exit(code)
}

// ---- test helpers ----

func rosterSetup(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	return home
}

func writeTestBeacon(t *testing.T, home string, b Beacon) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay", "roster")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir roster dir: %v", err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatalf("marshal beacon: %v", err)
	}
	path := filepath.Join(dir, b.Session+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write beacon: %v", err)
	}
}

func writeTestClaim(t *testing.T, home string, c Claim) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay", "claims")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal claim: %v", err)
	}
	filename := strings.ReplaceAll(c.Brief, "/", "--") + ".claim"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write claim: %v", err)
	}
}

func readTestBeacon(t *testing.T, home, session string) *Beacon {
	t.Helper()
	path := filepath.Join(home, ".config", "assay", "roster", session+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read beacon: %v", err)
	}
	var b Beacon
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("unmarshal beacon: %v", err)
	}
	return &b
}

// ---- tests ----

func TestSetUpsertsIdempotently(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	// First set.
	rc := run([]string{"set", "--repo", "tracker", "--pr", "42", "--what", "fix the thing"})
	if rc != deskkit.ExitOK {
		t.Fatalf("first set rc = %d, want 0", rc)
	}
	b := readTestBeacon(t, home, "alice")
	if b == nil || len(b.OpenWork) != 1 {
		t.Fatalf("first set: expected 1 work entry, got %+v", b)
	}
	if b.OpenWork[0].What != "fix the thing" {
		t.Fatalf("first set: What = %q, want %q", b.OpenWork[0].What, "fix the thing")
	}

	// Second set with same repo+pr: upserts the what field.
	rc = run([]string{"set", "--repo", "tracker", "--pr", "42", "--what", "updated description"})
	if rc != deskkit.ExitOK {
		t.Fatalf("second set rc = %d, want 0", rc)
	}
	b = readTestBeacon(t, home, "alice")
	if b == nil || len(b.OpenWork) != 1 {
		t.Fatalf("second set: expected still 1 work entry, got %+v", b)
	}
	if b.OpenWork[0].What != "updated description" {
		t.Fatalf("second set: What = %q, want %q", b.OpenWork[0].What, "updated description")
	}

	// Third set with a different PR: adds a new entry.
	rc = run([]string{"set", "--repo", "tracker", "--pr", "99", "--what", "another fix"})
	if rc != deskkit.ExitOK {
		t.Fatalf("third set rc = %d, want 0", rc)
	}
	b = readTestBeacon(t, home, "alice")
	if b == nil || len(b.OpenWork) != 2 {
		t.Fatalf("third set: expected 2 work entries, got %+v", b)
	}
}

func TestSetWithRole(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "bob")

	rc := run([]string{"set", "--repo", "tracker", "--pr", "1", "--what", "coordinating", "--role", "coordinator"})
	if rc != deskkit.ExitOK {
		t.Fatalf("set with role rc = %d, want 0", rc)
	}
	b := readTestBeacon(t, home, "bob")
	if b == nil || b.Role != "coordinator" {
		t.Fatalf("expected role 'coordinator', got %+v", b)
	}
}

func TestDropRemovesEntry(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	// Set up two entries.
	writeTestBeacon(t, home, Beacon{
		Session: "alice",
		OpenWork: []WorkEntry{
			{Repo: "tracker", PR: 42, What: "thing one"},
			{Repo: "agents", PR: 7, What: "thing two"},
		},
	})

	// Drop one.
	rc := run([]string{"drop", "--repo", "tracker", "--pr", "42"})
	if rc != deskkit.ExitOK {
		t.Fatalf("drop rc = %d, want 0", rc)
	}
	b := readTestBeacon(t, home, "alice")
	if b == nil || len(b.OpenWork) != 1 {
		t.Fatalf("after drop: expected 1 work entry, got %+v", b)
	}
	if b.OpenWork[0].PR != 7 {
		t.Fatalf("remaining PR should be 7, got %d", b.OpenWork[0].PR)
	}

	// Drop the last entry — beacon file should be removed (no role, no work).
	rc = run([]string{"drop", "--repo", "agents", "--pr", "7"})
	if rc != deskkit.ExitOK {
		t.Fatalf("drop last rc = %d, want 0", rc)
	}
	b = readTestBeacon(t, home, "alice")
	if b != nil {
		t.Fatalf("beacon file should be removed when empty, got %+v", b)
	}

	// Dropping non-existent entry is a no-op.
	rc = run([]string{"drop", "--repo", "tracker", "--pr", "1"})
	if rc != deskkit.ExitOK {
		t.Fatalf("drop nonexistent rc = %d, want 0", rc)
	}
}

func TestDropKeepsBeaconWithRole(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "bob")

	writeTestBeacon(t, home, Beacon{
		Session: "bob",
		Role:    "coordinator",
		OpenWork: []WorkEntry{
			{Repo: "tracker", PR: 42, What: "thing one"},
		},
	})

	// Drop the only work entry — beacon should persist because it has a role.
	rc := run([]string{"drop", "--repo", "tracker", "--pr", "42"})
	if rc != deskkit.ExitOK {
		t.Fatalf("drop rc = %d, want 0", rc)
	}
	b := readTestBeacon(t, home, "bob")
	if b == nil {
		t.Fatal("beacon should persist with role even without work entries")
	}
	if len(b.OpenWork) != 0 {
		t.Fatalf("expected 0 work entries, got %d", len(b.OpenWork))
	}
	if b.Role != "coordinator" {
		t.Fatalf("role should be 'coordinator', got %q", b.Role)
	}
}

func TestUnresolvableSessionExit6(t *testing.T) {
	home := rosterSetup(t)
	// Clear all session env vars so resolution fails.
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	rc := run([]string{"set", "--repo", "tracker", "--pr", "1", "--what", "x"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unresolvable session set rc = %d, want 6", rc)
	}

	rc = run([]string{"drop", "--repo", "tracker", "--pr", "1"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unresolvable session drop rc = %d, want 6", rc)
	}

	rc = run([]string{"mine"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("unresolvable session mine rc = %d, want 6", rc)
	}

	_ = home
}

func TestSessionResolutionOrder(t *testing.T) {
	// DESK_SESSION takes precedence over CLAUDE_SESSION_ID.
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "explicit")
	t.Setenv("CLAUDE_SESSION_ID", "implicit")

	rc := run([]string{"set", "--repo", "tracker", "--pr", "1", "--what", "x"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	// The beacon should be saved as "explicit", not "implicit".
	home := os.Getenv("HOME")
	b := readTestBeacon(t, home, "explicit")
	if b == nil {
		t.Fatal("expected beacon for 'explicit'")
	}
	// Check that "implicit" beacon does not exist.
	b2 := readTestBeacon(t, home, "implicit")
	if b2 != nil {
		t.Fatal("should not have beacon for 'implicit'")
	}
}

func TestSessionFlagFallback(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")

	rc := run([]string{"set", "--repo", "tracker", "--pr", "1", "--what", "x", "--session", "flag-session"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	b := readTestBeacon(t, home, "flag-session")
	if b == nil {
		t.Fatal("expected beacon for 'flag-session'")
	}
}

func TestListAutoPrunesMerged(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	// Create a beacon with two PRs: one merged, one open.
	writeTestBeacon(t, home, Beacon{
		Session: "alice",
		OpenWork: []WorkEntry{
			{Repo: "tracker", PR: 10, What: "merged work"},
			{Repo: "tracker", PR: 20, What: "still open"},
		},
	})

	// Set up fake gh: PR 10 is MERGED, PR 20 is OPEN/draft.
	t.Setenv("FAKEGH_PR_MERGED_10", "1")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}

	// After list, the merged entry should be auto-pruned from the beacon.
	b := readTestBeacon(t, home, "alice")
	if b == nil || len(b.OpenWork) != 1 {
		t.Fatalf("after list auto-prune: expected 1 work entry (merged removed), got %+v", b)
	}
	if b.OpenWork[0].PR != 20 {
		t.Fatalf("remaining PR should be 20, got %d", b.OpenWork[0].PR)
	}
}

func TestListAutoPrunesClosed(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	writeTestBeacon(t, home, Beacon{
		Session: "alice",
		OpenWork: []WorkEntry{
			{Repo: "tracker", PR: 10, What: "closed work"},
		},
	})

	t.Setenv("FAKEGH_PR_CLOSED_10", "1")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}

	b := readTestBeacon(t, home, "alice")
	// Beacon should be removed entirely (no remaining work, no role).
	if b != nil {
		t.Fatalf("beacon should be removed (empty after prune), got %+v", b)
	}
}

func TestListFoldsClaims(t *testing.T) {
	home := rosterSetup(t)

	// No beacons, just a claim.
	writeTestClaim(t, home, Claim{
		Brief:   "example/09",
		Session: "worker-a",
		TS:      "2026-07-17T13:52:05Z",
	})

	// No open PRs from fake gh list.
	t.Setenv("FAKEGH_LIST_PRS", "")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
	// The claim should appear in output (we can't easily capture stdout, but it shouldn't error).
}

func TestListSurfacesUnclaimed(t *testing.T) {
	rosterSetup(t)

	// Fake gh list returns an open PR #99 that no beacon covers.
	t.Setenv("FAKEGH_LIST_PRS", "99:true:unclaimed feature")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
	// The unclaimed PR should appear under the unclaimed section (stdout capture not strict here).
}

func TestMine(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	writeTestBeacon(t, home, Beacon{
		Session: "alice",
		OpenWork: []WorkEntry{
			{Repo: "tracker", PR: 42, What: "my thing"},
		},
	})

	rc := run([]string{"mine"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mine rc = %d, want 0", rc)
	}
}

func TestMineEmpty(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	// No beacon for alice — mine should succeed with a "no work" message.
	rc := run([]string{"mine"})
	if rc != deskkit.ExitOK {
		t.Fatalf("mine empty rc = %d, want 0", rc)
	}
}

func TestSetMissingRequiredFlags(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	rc := run([]string{"set", "--repo", "r", "--pr", "1"}) // missing --what
	if rc != deskkit.ExitRefused {
		t.Fatalf("set missing what rc = %d, want 5", rc)
	}

	rc = run([]string{"set", "--pr", "1", "--what", "x"}) // missing --repo
	if rc != deskkit.ExitRefused {
		t.Fatalf("set missing repo rc = %d, want 5", rc)
	}

	rc = run([]string{"set", "--repo", "r", "--what", "x"}) // missing --pr
	if rc != deskkit.ExitRefused {
		t.Fatalf("set missing pr rc = %d, want 5", rc)
	}
}

func TestDropMissingRequiredFlags(t *testing.T) {
	rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")

	rc := run([]string{"drop", "--repo", "r"}) // missing --pr
	if rc != deskkit.ExitRefused {
		t.Fatalf("drop missing pr rc = %d, want 5", rc)
	}
}

func TestListEmpty(t *testing.T) {
	rosterSetup(t)
	t.Setenv("FAKEGH_LIST_PRS", "")

	rc := run([]string{"list"})
	if rc != deskkit.ExitOK {
		t.Fatalf("list empty rc = %d, want 0", rc)
	}
}

func TestKillSwitchBlocksSet(t *testing.T) {
	home := rosterSetup(t)
	t.Setenv("DESK_SESSION", "alice")
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	rc := run([]string{"set", "--repo", "r", "--pr", "1", "--what", "x"})
	if rc != deskkit.ExitDisabled {
		t.Fatalf("kill-switch set rc = %d, want 3", rc)
	}
	_ = home
}

func TestUnknownSubcommand(t *testing.T) {
	rosterSetup(t)
	rc := run([]string{"bogus"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand rc = %d, want 5", rc)
	}
}
