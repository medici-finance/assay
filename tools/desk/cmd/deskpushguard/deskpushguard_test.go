package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeGHSource is compiled in TestMain into a temp dir placed first on PATH.
// It reads FAKEGH_STATE env to return the right PR JSON or a "not found" error.
const fakeGHSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	state := os.Getenv("FAKEGH_STATE")
	if state == "CRASH" {
		fmt.Fprintln(os.Stderr, "gh: connection refused")
		os.Exit(1)
	}
	if state == "NONE" || state == "" {
		fmt.Fprintln(os.Stderr, "no pull requests found for branch test-branch")
		os.Exit(1)
	}
	if state == "GARBAGE" {
		fmt.Println("not json")
		os.Exit(0)
	}
	fmt.Printf("{\"state\":%q,\"number\":42}\n", state)
	os.Exit(0)
}
`

var (
	fakeGHDir string
	origPATH  string
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	origPATH = os.Getenv("PATH")
	dir, err := os.MkdirTemp("", "deskpushguard-fakegh")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fakegh\n\ngo 1.25\n"), 0o644); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}
	if werr := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeGHSource), 0o644); werr != nil {
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
	fakeGHDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func withFakeGH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", fakeGHDir+string(os.PathListSeparator)+origPATH)
	fixtureHome := t.TempDir()
	t.Setenv("HOME", fixtureHome)
	plantFixtureRoster(t, fixtureHome)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESKPUSHGUARD_FAKE_STATE", "") // clear test seam
	// Replace execCommand so the tool runs the real gh (our fake).
	oldExec := execCommand
	execCommand = exec.Command
	t.Cleanup(func() { execCommand = oldExec })
}

// stdinString returns a reader from a string.
func stdinString(s string) *strings.Reader {
	return strings.NewReader(s)
}

// --- Unit: parseRepo ---

func TestParseRepo(t *testing.T) {
	cases := map[string]string{
		"https://github.com/example-org/tracker.git":              "example-org/tracker",
		"https://github.com/example-org/agents":                   "example-org/agents",
		"git@github.com:example-org/examples.git":                 "example-org/examples",
		"ssh://git@github.com/example-org/example-reconciler.git": "example-org/example-reconciler",
	}
	for in, want := range cases {
		got, err := parseRepo(in)
		if err != nil || got != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// --- Unit: parseRef ---

func TestParseRef(t *testing.T) {
	tests := []struct {
		line   string
		branch string
		ok     bool
	}{
		{"refs/heads/feature-branch x refs/heads/feature-branch y", "feature-branch", true},
		{"refs/heads/merged-fixture x refs/heads/merged-fixture y", "merged-fixture", true},
		// Slashed house-convention branches must keep their FULL name so fetchPR
		// can find the PR — filepath.Base would collapse these to the leaf and
		// defeat the guard (#267).
		{"refs/heads/fix/issue-108 x refs/heads/fix/issue-108 y", "fix/issue-108", true},
		{"refs/heads/claude/session-42 x refs/heads/claude/session-42 y", "claude/session-42", true},
		{"refs/heads/brief/desk-10 x refs/heads/brief/desk-10 y", "brief/desk-10", true},
		{"refs/heads/docs/readme-fix x refs/heads/docs/readme-fix y", "docs/readme-fix", true},
		{"refs/tags/v1.0 x refs/tags/v1.0 y", "v1.0", true},
		{"", "", false},
	}
	for _, tt := range tests {
		ref, ok := parseRef(tt.line)
		if ok != tt.ok {
			t.Errorf("parseRef(%q) ok = %v, want %v", tt.line, ok, tt.ok)
		}
		if ok && ref.branch != tt.branch {
			t.Errorf("parseRef(%q) branch = %q, want %q", tt.line, ref.branch, tt.branch)
		}
	}
}

// --- Integration tests (using fake gh) ---

func TestMergedBranchBlocked(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "MERGED")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("MERGED branch rc = %d, want %d (ExitRefused). stderr: %s", rc, deskkit.ExitRefused, stderr.String())
	}
	if !strings.Contains(stderr.String(), "MERGED") {
		t.Errorf("expected stderr to mention MERGED, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "refusing") {
		t.Errorf("expected stderr to say 'refusing', got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "DONE") {
		t.Errorf("expected stderr to mention DONE/new branch, got: %s", stderr.String())
	}
}

func TestClosedBranchBlocked(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "CLOSED")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("CLOSED branch rc = %d, want %d (ExitRefused)", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(stderr.String(), "CLOSED") {
		t.Errorf("expected stderr to mention CLOSED, got: %s", stderr.String())
	}
}

func TestOpenBranchAllowed(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "OPEN")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("OPEN branch rc = %d, want %d (ExitOK). stderr: %s", rc, deskkit.ExitOK, stderr.String())
	}
}

func TestNoPRAllowed(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "NONE")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("no-PR branch rc = %d, want %d (ExitOK). stderr: %s", rc, deskkit.ExitOK, stderr.String())
	}
}

func TestGHErrorAllowedFailOpen(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "CRASH")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("gh-error branch rc = %d, want %d (ExitOK; fail-open). stderr: %s", rc, deskkit.ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "fail-open") {
		t.Errorf("expected stderr to mention fail-open on error, got: %s", stderr.String())
	}
}

func TestGarbageOutputAllowedFailOpen(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "GARBAGE")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("garbage-gh-output rc = %d, want 0 (ExitOK; fail-open)", rc)
	}
}

func TestDESKPUSHGUARD_OFF(t *testing.T) {
	withFakeGH(t)
	t.Setenv("DESKPUSHGUARD_OFF", "1")
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("DESKPUSHGUARD_OFF=1 rc = %d, want 0 (ExitOK)", rc)
	}
	if !strings.Contains(stderr.String(), "DESKPUSHGUARD_OFF=1") {
		t.Errorf("expected stderr to mention DESKPUSHGUARD_OFF=1 override, got: %s", stderr.String())
	}
}

func TestMultiRefBlockedIfAnyMerged(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKEGH_STATE", "MERGED")
	stdin := stdinString("refs/heads/branch-ok x refs/heads/branch-ok y\nrefs/heads/branch-bad x refs/heads/branch-bad y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("multi-ref with MERGED rc = %d, want %d (ExitRefused). stderr: %s", rc, deskkit.ExitRefused, stderr.String())
	}
	// Both branches are checked via fake gh so both return MERGED; ensure both named.
	if !strings.Contains(stderr.String(), "branch-ok") {
		t.Errorf("expected stderr to mention branch-ok, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "branch-bad") {
		t.Errorf("expected stderr to mention branch-bad, got: %s", stderr.String())
	}
}

func TestEmptyStdinAllowed(t *testing.T) {
	withFakeGH(t)
	stdin := stdinString("\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("empty stdin rc = %d, want 0 (ExitOK)", rc)
	}
}

func TestFAKESTATE_MERGED_Blocked(t *testing.T) {
	withFakeGH(t)
	t.Setenv("DESKPUSHGUARD_FAKE_STATE", "MERGED")
	stdin := stdinString("refs/heads/merged-fixture x refs/heads/merged-fixture y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitRefused {
		t.Fatalf("FAKE_STATE=MERGED rc = %d, want %d (ExitRefused)", rc, deskkit.ExitRefused)
	}
}

func TestFAKESTATE_OPEN_Allowed(t *testing.T) {
	withFakeGH(t)
	t.Setenv("DESKPUSHGUARD_FAKE_STATE", "OPEN")
	stdin := stdinString("refs/heads/open-fixture x refs/heads/open-fixture y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("FAKE_STATE=OPEN rc = %d, want 0 (ExitOK)", rc)
	}
}

func TestFAKESTATE_OFF_MERGED_Warns(t *testing.T) {
	withFakeGH(t)
	t.Setenv("DESKPUSHGUARD_FAKE_STATE", "MERGED")
	t.Setenv("DESKPUSHGUARD_OFF", "1")
	stdin := stdinString("refs/heads/merged-fixture x refs/heads/merged-fixture y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "https://github.com/example-org/tracker.git"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("FAKE_STATE=MERGED + OFF=1 rc = %d, want 0 (ExitOK)", rc)
	}
	if !strings.Contains(stderr.String(), "DESKPUSHGUARD_OFF=1") {
		t.Errorf("expected stderr to mention DESKPUSHGUARD_OFF=1 override warning, got: %s", stderr.String())
	}
}

func TestVersionPrints(t *testing.T) {
	var stderr strings.Builder
	rc := run([]string{"--version"}, strings.NewReader(""), &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("--version rc = %d, want 0", rc)
	}
}

func TestUnparseableRemoteAllowedFailOpen(t *testing.T) {
	withFakeGH(t)
	stdin := stdinString("refs/heads/test-branch x refs/heads/test-branch y\n")
	var stderr strings.Builder
	rc := run([]string{"origin", "not-a-valid-url"}, stdin, &stderr)
	if rc != deskkit.ExitOK {
		t.Fatalf("unparseable remote rc = %d, want 0 (ExitOK; fail-open)", rc)
	}
}
