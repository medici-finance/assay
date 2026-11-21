package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// printedProgress is the progress command deskdispatch's NOT LAUNCHED line prints for a
// branchless (verifier or review) claim, with the claim tool on PATH. The deskdispatch side
// pins that the printed line still parses to this argv (TestPrintedClaimArgv there).
const printedProgress = `deskclaim-ref progress "at--stream--05" --repo medici-finance/assay --branch - ` +
	`--token-file "$(desktoken --no-rotate desk --repo medici-finance/assay --cross-role "a deskdispatch claim command")"`

// shellArgv runs line through sh with desktoken stubbed to print tok, and returns the argv the
// claim tool would receive: the words after the leading tool name, split exactly as a shell does.
func shellArgv(t *testing.T, line, tok string) []string {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("could-not-check: no sh on PATH")
	}
	// Stubs on PATH, not shell functions: "deskclaim-ref" is not a portable function name.
	bin := t.TempDir()
	for name, body := range map[string]string{
		"desktoken":     `printf '%s' "$TOK"`,
		"deskclaim-ref": `printf '%s\0' "$@"`,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!"+sh+"\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(sh, "-c", line)
	cmd.Env = append(os.Environ(), "TOK="+tok, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("sh could not run the printed line: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
}

// noWorkSource fails the test if the claim tool resolves a work reference: a physical key, or
// an argv the parser refuses, never reaches the checkout-dependent work source.
func noWorkSource(t *testing.T) {
	t.Helper()
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(repo string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		t.Errorf("work source resolved for repo %q", repo)
		return deskkit.WorkCatalog{}, nil, false, deskkit.Unverifiable("unexpected", nil)
	}
}

// The command deskdispatch prints, run through a shell, advances the claim deskdispatch
// acquired: acquire runs with no --branch (as stepClaim runs it), progress as printed.
// regression: #6896
func TestPrintedProgressRuns(t *testing.T) {
	t.Setenv("DESK_SESSION", "o1")
	noWorkSource(t)
	tok := filepath.Join(t.TempDir(), "tok")
	f := newStore()
	run, _, se := harness(t, f)
	if rc := run("acquire", "at--stream--05", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("acquire rc = %d: %s", rc, se.String())
	}
	argv := shellArgv(t, printedProgress, tok)
	if rc := run(argv...); rc != exitOK {
		t.Fatalf("printed progress %q rc = %d: %s", argv, rc, se.String())
	}
	msg := f.claims["at--stream--05"].msg
	if fieldOf(msg, "state") != "dispatched" || fieldOf(msg, "branch") != "-" {
		t.Fatalf("claim not advanced as printed: %s", msg)
	}
}

// `--branch=-` is the same value as `--branch -`.
// regression: #6896
func TestBranchEqualsForm(t *testing.T) {
	t.Setenv("DESK_SESSION", "o1")
	noWorkSource(t)
	f := newStore()
	run, _, se := harness(t, f)
	if rc := run("acquire", "at--stream--05", "--repo=medici-finance/assay"); rc != exitOK {
		t.Fatalf("acquire rc = %d: %s", rc, se.String())
	}
	if rc := run("progress", "at--stream--05", "--repo", "medici-finance/assay", "--branch=-"); rc != exitOK {
		t.Fatalf("progress --branch=- rc = %d: %s", rc, se.String())
	}
	if msg := f.claims["at--stream--05"].msg; fieldOf(msg, "state") != "dispatched" || fieldOf(msg, "branch") != "-" {
		t.Fatalf("claim not advanced: %s", msg)
	}
}

// A value flag with its value missing names that flag and stops. It never takes the next
// option as its value, which shifts the rest: the two refusals the issue reports are what
// `--repo` swallowing the next option produces ("unknown flag -", and a work reference looked
// up for a repository named "--branch=-").
// regression: #6896
func TestMissingRepoValue(t *testing.T) {
	noWorkSource(t)
	f := newStore()
	f.seedClaim("at--stream--05", "o1", "claimed", "", 0)
	before := f.claims["at--stream--05"]
	run, _, se := harness(t, f)
	for _, argv := range [][]string{
		{"progress", "at--stream--05", "--repo", "--branch", "-", "--token-file", "tok"},
		{"progress", "at:stream/05", "--repo", "--branch=-", "--token-file", "tok"},
		{"progress", "at--stream--05", "--branch", "-", "--repo"},
		{"progress", "at--stream--05", "--branch", "-", "--repo="},
	} {
		if rc := run(argv...); rc != exitRefused || !strings.Contains(se.String(), "--repo requires a value") {
			t.Errorf("%q: rc = %d, want 5 naming --repo: %s", argv, rc, se.String())
		}
	}
	if f.claims["at--stream--05"] != before {
		t.Fatal("a refused argv changed the claim")
	}
}

// Defect class: every value flag, the performance flags included, refuses a missing value
// rather than recording the next option as one (here `--branch --owner` would acquire a claim
// on branch "--owner").
// regression: #6896
func TestEveryFlagNeedsValue(t *testing.T) {
	noWorkSource(t)
	f := newStore()
	run, _, se := harness(t, f)
	for _, flag := range []string{"--owner", "--branch", "--reason", "--token-file", "--perf-role"} {
		argv := []string{"acquire", "at--stream--06", "--repo", "medici-finance/assay", flag, "--owner", "o1"}
		if flag == "--owner" {
			argv[len(argv)-2] = "--branch"
		}
		if rc := run(argv...); rc != exitRefused || !strings.Contains(se.String(), flag+" requires a value") {
			t.Errorf("%q: rc = %d, want 5 naming %s: %s", argv, rc, flag, se.String())
		}
	}
	// The trailing form used to succeed silently, holding a claim on branch "--owner".
	if rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay", "--branch", "--owner"); rc != exitRefused {
		t.Errorf("trailing --branch --owner rc = %d, want 5: %s", rc, se.String())
	}
	if len(f.claims) != 0 {
		t.Fatalf("a refused argv wrote a claim: %v", f.claims)
	}
}

// A value made only of whitespace is a missing value, in the spaced and the inline form alike.
// The repo and credential resolvers trim it and fall back to the origin remote and the ambient
// GH_TOKEN, so a blank --repo or --token-file accepted here would act on the wrong repo or
// under the wrong credential. The performance flags follow the same rule.
// regression: #6896
func TestBlankValueRefused(t *testing.T) {
	t.Setenv("GH_TOKEN", "ambient")
	noWorkSource(t)
	f := newStore()
	run, _, se := harness(t, f)
	const r = "medici-finance/assay"
	for _, tc := range []struct {
		flag string
		argv []string
	}{
		{"--repo", []string{"acquire", "at--stream--09", "--repo", " "}},
		{"--repo", []string{"acquire", "at--stream--09", "--repo= "}},
		{"--repo", []string{"acquire", "at--stream--09", "--repo", "\t"}},
		{"--repo", []string{"acquire", "at:stream/09", "--repo", " "}},
		{"--token-file", []string{"acquire", "at--stream--09", "--repo", r, "--token-file", " "}},
		{"--token-file", []string{"acquire", "at--stream--09", "--repo", r, "--token-file= "}},
		{"--owner", []string{"acquire", "at--stream--09", "--repo", r, "--owner", " "}},
		{"--branch", []string{"acquire", "at--stream--09", "--repo", r, "--branch= "}},
		{"--reason", []string{"acquire", "at--stream--09", "--repo", r, "--reason", " "}},
		{"--perf-role", []string{"acquire", "at--stream--09", "--repo", r, "--perf-role="}},
		{"--perf-role", []string{"acquire", "at--stream--09", "--repo", r, "--perf-role= "}},
		{"--perf-role", []string{"acquire", "at--stream--09", "--repo", r, "--perf-role", " "}},
	} {
		if rc := run(tc.argv...); rc != exitRefused || !strings.Contains(se.String(), tc.flag+" requires a value") {
			t.Errorf("%q: rc = %d, want 5 naming %s: %s", tc.argv, rc, tc.flag, se.String())
		}
		if len(f.claims) != 0 {
			t.Errorf("%q wrote a claim: %v", tc.argv, f.claims)
			f.claims = map[string]fakeClaim{}
		}
	}
}
