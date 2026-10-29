package deskkit

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siblingTwoSource = "89abcdef0123456789abcdef0123456789abcdef"

// twoSiblingRoster allows a second (public) and third (private) sibling beside
// example-org/one; a private tracking repository may name every sibling.
func twoSiblingRoster(t *testing.T) { twoSiblingVisibility(t, "private") }

func twoSiblingVisibility(t *testing.T, tracking string) {
	t.Helper()
	cfgMu.Lock()
	previous, once := cfgValue, cfgOnce
	cfgValue = parseConfig(ClassWrite, "offline fixture", parseDotenv(strings.ReplaceAll(raisedByFixtureRoster, "example-desk-app", "example-desk")+
		"\nASSAY_ALLOWED_REPOS=example-org/one:ci:public,example-org/two:ci:public,example-org/three:ci:private,example-org/tracking:ci:"+tracking+"\n"))
	cfgOnce = true
	cfgMu.Unlock()
	t.Cleanup(func() { cfgMu.Lock(); cfgValue, cfgOnce = previous, once; cfgMu.Unlock() })
}

const twoSiblingFiles = "files:\n- `../one/a.go`\n- `../two/b.go`"

// Each declared sibling is pinned to its own source, and coverage stays per
// path: without sibling two's key, ../two/b.go is refused although one's
// source covers ../one/a.go.
// regression: #4876
func TestTwoSiblingRecord(t *testing.T) {
	twoSiblingRoster(t)
	in := siblingTestInput()
	in.SiblingSources = map[string]string{"example-org/two": siblingTwoSource}
	r, err := siblingTestBuild(t, twoSiblingFiles, in)
	if err != nil {
		t.Fatalf("two-sibling record refused: %v", err)
	}
	one, two := InputKeyRepoSourcePrefix+"example-org/one", InputKeyRepoSourcePrefix+"example-org/two"
	if r.Inputs[one] != siblingTestSource || r.Inputs[two] != siblingTwoSource {
		t.Fatalf("siblings not pinned to their own sources: %v", r.Inputs)
	}
	decls := []string{"../one/a.go", "../two/b.go"}
	if err := CheckSiblingCoverage(decls, r.Inputs); err != nil {
		t.Fatalf("complete coverage refused: %v", err)
	}
	for _, drop := range []string{one, two} {
		cut := maps.Clone(r.Inputs)
		delete(cut, drop)
		err := CheckSiblingCoverage(decls, cut)
		want := map[string]string{one: "../one/a.go", two: "../two/b.go"}[drop]
		if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), want) {
			t.Errorf("without %s: %v; want a refusal naming %s", drop, err, want)
		}
	}
}

// A declared sibling whose source the run did not bind is still refused, and
// a sibling source never stands in for a different or undeclared repository.
// regression: #4876
func TestSiblingSourceMissing(t *testing.T) {
	twoSiblingRoster(t)
	for name, siblings := range map[string]map[string]string{
		"none":            nil,
		"undeclared":      {"example-org/three": siblingTwoSource},
		"short commit":    {"example-org/two": "89abcdef"},
		"empty commit":    {"example-org/two": ""},
		"repeats source":  {"example-org/one": siblingTwoSource},
		"tracking repo":   {"example-org/tracking": siblingTwoSource},
		"outside the set": {"fork-org/two": siblingTwoSource},
	} {
		in := siblingTestInput()
		in.SiblingSources = siblings
		if _, err := siblingTestBuild(t, twoSiblingFiles, in); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%s: got %v; want explicit refusal", name, err)
		}
	}
	same := wakeTestInput()
	same.SiblingSources = map[string]string{"example-org/two": siblingTwoSource}
	if _, err := siblingTestBuild(t, "files:\n- `../two/b.go`", same); ExitCodeOf(err) != ExitRefused {
		t.Errorf("same-repository run bound a sibling: %v", err)
	}
}

// The planner reads each sibling's key against that sibling's own root: both
// unmoved hold, a moved second sibling wakes, a missing key is could-not-check.
// regression: #4876
func TestTwoSiblingPlanner(t *testing.T) {
	twoSiblingRoster(t)
	one := crossRepoGitFixture(t, "example-org/one", "a.go", "package one\n")
	two := crossRepoGitFixture(t, "example-org/two", "b.go", "package two\n")
	const brief = "docs/streams/sample/brief-01-plan.md"
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Plan\n\n## Context\n\n"+twoSiblingFiles+"\n")
	t.Setenv(RootsEnv, "example-org/one="+one+",example-org/two="+two)
	landed, err := os.ReadFile(filepath.Join(tracking, brief))
	if err != nil {
		t.Fatal(err)
	}
	oneSHA, _ := verifierGit(one, "rev-parse", "HEAD")
	twoSHA, _ := verifierGit(two, "rev-parse", "HEAD")
	full := map[string]string{"file:" + brief: Sha256Hex(landed), "tool": "test-tool",
		InputKeyRepoSourcePrefix + "example-org/one": oneSHA, InputKeyRepoSourcePrefix + "example-org/two": twoSHA}
	state := func(inputs map[string]string) InputsState {
		r := NewWakeReceipt("fixture", "example-org/tracking", "sample/01", "verifier", "verify-fail", oneSHA, nil, inputs, "test-tool", "implementation", "#1", WakeRelevantInputChanged, "", "", "2026-10-29T00:00:00Z")
		s, _ := Unchanged(r, NewRootRevisionReader(tracking, "test-tool"))
		return s
	}
	if got := state(full); got != InputsUnchanged {
		t.Fatalf("unmoved siblings: %v", got)
	}
	cut := maps.Clone(full)
	delete(cut, InputKeyRepoSourcePrefix+"example-org/two")
	if got := state(cut); got != InputsCouldNotCheck {
		t.Fatalf("second sibling unbound: %v, want could-not-check", got)
	}
	attestGit(t, two, "commit", "--allow-empty", "-m", "moved")
	if got := state(full); got != InputsChanged {
		t.Fatalf("moved second sibling: %v, want changed", got)
	}
}

// Prepare binds a further declared sibling only at its configured root's
// fetched origin/main, and only where the tracking repository may name it.
// regression: #4876
func TestPrepareBindsSibling(t *testing.T) {
	const brief = "docs/streams/sample/brief-01-check.md"
	body := "# Plan\n\n## Context\n\nfiles:\n- `../one/source.txt`\n- `../two/b.go`\n- `../three/c.go`\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f source.txt` | exit 0 |\n\n## Evidence\n\nPending.\n"
	for _, tc := range []struct {
		name, tracking string
		moved          bool
		want           func(two, three string) string
	}{
		{"fetched", "private", false, func(two, three string) string { return "example-org/three@" + three + ",example-org/two@" + two }},
		{"unfetched", "private", true, func(_, three string) string { return "example-org/three@" + three }},
		// A public tracking repository never names a private sibling.
		{"public tracking", "public", false, func(two, _ string) string { return "example-org/two@" + two }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			twoSiblingVisibility(t, tc.tracking)
			home := crossRepoGitFixture(t, "example-org/one", "source.txt", "observed\n")
			two := crossRepoGitFixture(t, "example-org/two", "b.go", "package two\n")
			three := crossRepoGitFixture(t, "example-org/three", "c.go", "package three\n")
			if tc.moved {
				attestGit(t, two, "commit", "--allow-empty", "-m", "not yet fetched")
			}
			tracking := crossRepoGitFixture(t, "example-org/tracking", brief, body)
			t.Setenv(RootsEnv, "example-org/two="+two+",example-org/three="+three)
			if err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, brief), "gpt-6-astra", "strong"); err != nil {
				t.Fatal(err)
			}
			r, err := verifierLoad(home)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(r.TrackingHome)) })
			twoHead, _ := verifierGit(two, "rev-parse", "refs/remotes/origin/main")
			threeHead, _ := verifierGit(three, "rev-parse", "HEAD")
			want := tc.want(twoHead, threeHead)
			if r.Binding.SiblingSources != want {
				t.Fatalf("sibling sources %q, want %q", r.Binding.SiblingSources, want)
			}
			if err := verifierLocalCheck(home, r.Binding.Brief, r, verifierExecution); err != nil {
				t.Fatalf("bound run no longer checks: %v", err)
			}
		})
	}
}
