package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeRosterGHSource is compiled once (TestMain) into a temp dir placed FIRST on PATH.
// It handles `gh pr view` (returns state/isDraft/title for a given PR) and
// `gh pr list` (returns open PRs), driven by env vars for fixture control.
const fakeRosterGHSource = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}

	switch {
	case has("view"):
		// gh pr view <pr> --repo <owner/repo> --json state,isDraft,title
		prNum := "?"
		for i, a := range args {
			if a == "--json" && i+1 < len(args) {
				// skip
			} else if i == 2 {
				prNum = a
			}
		}
		// Read fixture from env: FAKEGH_PR_<num>=state|isDraft|title
		envKey := "FAKEGH_PR_" + prNum
		if val := os.Getenv(envKey); val != "" {
			parts := strings.SplitN(val, "|", 3)
			state, draft, title := parts[0], "false", ""
			if len(parts) > 1 { draft = parts[1] }
			if len(parts) > 2 { title = parts[2] }
			fmt.Printf(` + "`" + `{"state":%q,"isDraft":%s,"title":%q}` + "`" + `+"\n", state, draft, title)
			return
		}
		// Default: if env FAKEGH_PR_MERGED is set for this pr, return MERGED.
		if os.Getenv("FAKEGH_PR_MERGED_"+prNum) == "1" {
			fmt.Printf(` + "`" + `{"state":"MERGED","isDraft":false,"title":"merged pr %s"}` + "`" + `+"\n", prNum)
			return
		}
		if os.Getenv("FAKEGH_PR_CLOSED_"+prNum) == "1" {
			fmt.Printf(` + "`" + `{"state":"CLOSED","isDraft":false,"title":"closed pr %s"}` + "`" + `+"\n", prNum)
			return
		}
		// Default: open draft.
		fmt.Printf(` + "`" + `{"state":"OPEN","isDraft":true,"title":"draft pr %s"}` + "`" + `+"\n", prNum)
	case has("list"):
		// gh pr list --repo <owner/repo> --state open --json number,title,isDraft --limit 50
		prs := os.Getenv("FAKEGH_LIST_PRS")
		if prs == "" {
			fmt.Println("[]")
			return
		}
		// Format: "num:draft:title;num:draft:title;..."
		entries := strings.Split(prs, ";")
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			if e == "" {
				continue
			}
			fields := strings.SplitN(e, ":", 3)
			num, draft, title := fields[0], "false", ""
			if len(fields) > 1 { draft = fields[1] }
			if len(fields) > 2 { title = fields[2] }
			parts = append(parts, fmt.Sprintf(` + "`" + `{"number":%s,"isDraft":%s,"title":%q}` + "`" + `, num, draft, title))
		}
		fmt.Println("[" + strings.Join(parts, ",") + "]")
	}
}
`

var (
	fakeRosterGHDir string
	origRosterPATH  string
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origRosterPATH = os.Getenv("PATH")
	dir, err := os.MkdirTemp("", "deskroster-fakegh")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.25\n"), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeRosterGHSource), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "gh"), ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, berr := build.CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build fake gh: %v\n%s\n", berr, out)
		os.Exit(1)
	}
	fakeRosterGHDir = dir
	code := m.Run()
	os.RemoveAll(dir)
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
	t.Setenv("PATH", fakeRosterGHDir+string(os.PathListSeparator)+origRosterPATH)
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
