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
		"\nASSAY_ALLOWED_REPOS=example-org/one:ci:public,example-org/two:ci:public,example-org/three:ci:private,mirror-org/two:ci:public,example-org/tracking:ci:"+tracking+"\n"))
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
// Every guard case also binds a valid example-org/two, so coverage holds and
// the guard the case names is the only refusal left.
// regression: #4876
func TestSiblingSourceMissing(t *testing.T) {
	twoSiblingRoster(t)
	withTracking := twoSiblingFiles + "\n- `../tracking/t.go`"
	for _, tc := range []struct {
		name, files string
		siblings    map[string]string
	}{
		{"none", twoSiblingFiles, nil},
		{"undeclared", twoSiblingFiles, map[string]string{"example-org/two": siblingTwoSource, "example-org/three": siblingTwoSource}},
		{"short commit", twoSiblingFiles, map[string]string{"example-org/two": "89abcdef"}},
		{"empty commit", twoSiblingFiles, map[string]string{"example-org/two": ""}},
		{"repeats source", twoSiblingFiles, map[string]string{"example-org/two": siblingTwoSource, "example-org/one": siblingTwoSource}},
		{"tracking repo", withTracking, map[string]string{"example-org/two": siblingTwoSource, "example-org/tracking": siblingTwoSource}},
		{"outside the set", twoSiblingFiles, map[string]string{"fork-org/two": siblingTwoSource}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := siblingTestInput()
			in.SiblingSources = tc.siblings
			if _, err := siblingTestBuild(t, tc.files, in); ExitCodeOf(err) != ExitRefused {
				t.Errorf("got %v; want explicit refusal", err)
			}
		})
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

// Prepare binds a further declared sibling only at its one configured root of
// that name, whose origin is that repository, at the fetched origin/main, and
// only where the tracking repository may name it; every sibling it leaves
// unbound gets one NOTICE saying why.
// regression: #4876
func TestPrepareBindsSibling(t *testing.T) {
	const brief = "docs/streams/sample/brief-01-check.md"
	body := "# Plan\n\n## Context\n\nfiles:\n- `../one/source.txt`\n- `../two/b.go`\n- `../three/c.go`\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f source.txt` | exit 0 |\n\n## Evidence\n\nPending.\n"
	onlyThree := func(_, three string) string { return "example-org/three@" + three }
	for _, tc := range []struct {
		name, tracking string
		moved          bool
		// roots maps the fixtures to the configured roots.
		roots  func(two, three, mirror string) string
		want   func(two, three string) string
		notice string
	}{
		{"fetched", "private", false, nil, func(two, three string) string { return "example-org/three@" + three + ",example-org/two@" + two }, ""},
		{"unfetched", "private", true, nil, onlyThree, "example-org/two left unbound: its HEAD is not the fetched origin/main"},
		// A public tracking repository never names a private sibling.
		{"public tracking", "public", false, nil, func(two, _ string) string { return "example-org/two@" + two }, "example-org/three left unbound: the tracking repository cannot publish it"},
		// Two roots named two under different owners: neither is chosen.
		{"ambiguous", "private", false, func(two, three, mirror string) string {
			return "example-org/two=" + two + ",example-org/three=" + three + ",mirror-org/two=" + mirror
		}, onlyThree, "../two/ left unbound: 2 configured roots"},
		// A root keyed example-org/two whose origin is another repository.
		{"misidentified", "private", false, func(_, three, mirror string) string {
			return "example-org/two=" + mirror + ",example-org/three=" + three
		}, onlyThree, "example-org/two left unbound: its configured root is not a checkout whose origin is example-org/two"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			twoSiblingVisibility(t, tc.tracking)
			var notices []string
			previous := verifierSiblingNotice
			verifierSiblingNotice = func(msg string) { notices = append(notices, msg) }
			t.Cleanup(func() { verifierSiblingNotice = previous })
			home := crossRepoGitFixture(t, "example-org/one", "source.txt", "observed\n")
			two := crossRepoGitFixture(t, "example-org/two", "b.go", "package two\n")
			three := crossRepoGitFixture(t, "example-org/three", "c.go", "package three\n")
			mirror := crossRepoGitFixture(t, "mirror-org/two", "b.go", "package two\n")
			if tc.moved {
				attestGit(t, two, "commit", "--allow-empty", "-m", "not yet fetched")
			}
			tracking := crossRepoGitFixture(t, "example-org/tracking", brief, body)
			roots := "example-org/two=" + two + ",example-org/three=" + three
			if tc.roots != nil {
				roots = tc.roots(two, three, mirror)
			}
			t.Setenv(RootsEnv, roots)
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
			if tc.notice == "" && len(notices) != 0 || tc.notice != "" && (len(notices) != 1 || !strings.Contains(notices[0], tc.notice)) {
				t.Errorf("notices %q, want one naming %q", notices, tc.notice)
			}
			if err := verifierLocalCheck(home, r.Binding.Brief, r, verifierExecution); err != nil {
				t.Fatalf("bound run no longer checks: %v", err)
			}
		})
	}
}

// A run that bound a private sibling under a private tracking repository is
// refused at check time once the tracking repository becomes public, although
// the deliverable itself stays publishable there.
// regression: #4876
func TestSiblingVisibilityCheck(t *testing.T) {
	twoSiblingRoster(t)
	previous := verifierSiblingNotice
	verifierSiblingNotice = func(string) {}
	t.Cleanup(func() { verifierSiblingNotice = previous })
	const brief = "brief.md"
	home := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
	three := crossRepoGitFixture(t, "example-org/three", "c.go", "package three\n")
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Plan\n\n## Context\n\nfiles:\n- `../one/source.txt`\n- `../three/c.go`\n\n## Evidence\n\nPending.\n")
	t.Setenv(RootsEnv, "example-org/three="+three)
	if err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, brief), "gpt-6-astra", "strong"); err != nil {
		t.Fatal(err)
	}
	r, err := verifierLoad(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(r.TrackingHome)) })
	if !strings.HasPrefix(r.Binding.SiblingSources, "example-org/three@") {
		t.Fatalf("private sibling not bound: %q", r.Binding.SiblingSources)
	}
	f := &verifierForge{actor: "example-desk[bot]"}
	if _, err := IssueVerifierAttestation(home, f); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierAttestationWithForge(home, brief, f); err != nil {
		t.Fatalf("untampered run refused: %v", err)
	}
	twoSiblingVisibility(t, "public")
	if _, err := CheckVerifierAttestationWithForge(home, brief, f); err == nil || !strings.Contains(err.Error(), "cannot publish the deliverable repository binding") {
		t.Fatalf("private sibling under a public tracking repository: %v", err)
	}
}
