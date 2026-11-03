package deskkit

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siblingTestSource = "0123456789abcdef0123456789abcdef01234567"

// A cross-repository receipt: the brief is tracked in example-org/tracking and
// its deliverable is in example-org/one, a sibling checkout named "one".
func siblingTestInput() OutcomeReceiptInput {
	in := wakeTestInput()
	in.Repo, in.SourceRepo, in.Source = "example-org/tracking", "example-org/one", siblingTestSource
	return in
}

func siblingTestBuild(t *testing.T, files string, in OutcomeReceiptInput) (WakeReceipt, error) {
	t.Helper()
	tree := wakeTestTree()
	tree.files[wakeTestBriefPath] = "# Brief\n\n## Context\n\n" + files + "\n\nfacts:\n- none\n"
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
	var r WakeReceipt
	if err == nil {
		if jerr := json.Unmarshal(raw, &r); jerr != nil {
			t.Fatal(jerr)
		}
	}
	return r, err
}

// regression: #3483
func TestSiblingOutcomeRecord(t *testing.T) {
	crossRepoRoster(t)
	for _, files := range []string{
		"files:\n- `../one/statusgen/emit.go`",
		"files:\n- `tools/a.go` (edit)\n- `../one/statusgen/emit.go` (edit)\n- `../one/tools/{x,y}.go`\n- `../one/docs/`",
		"files: `../one/tools/*.go`, `tools/pkg/` and tests",
	} {
		t.Run(files, func(t *testing.T) {
			r, err := siblingTestBuild(t, files, siblingTestInput())
			if err != nil {
				t.Fatalf("cross-repository record refused: %v", err)
			}
			if got := r.Inputs[InputKeyRepoSourcePrefix+"example-org/one"]; got != siblingTestSource {
				t.Fatalf("sibling deliverable not pinned to the source commit: %q", got)
			}
			for k := range r.Inputs {
				if strings.Contains(k, "..") {
					t.Fatalf("sibling path became a local input key %q", k)
				}
			}
			if strings.Contains(files, "tools/a.go") && r.Inputs["file:tools/a.go"] == "" {
				t.Fatal("local deliverable lost beside a sibling one")
			}
		})
	}
}

// Every path that is not exactly ../<the source repository>/<canonical path>
// stays refused, cross-repository or not.
// regression: #3483
func TestSiblingEscapeRefused(t *testing.T) {
	crossRepoRoster(t)
	cross := siblingTestInput()
	same := wakeTestInput()
	for _, tc := range []struct {
		decl string
		in   OutcomeReceiptInput
	}{
		{"../other/x.go", cross},        // a sibling the run never bound
		{"../../one/x.go", cross},       // climbs past the sibling parent
		{"../one/../other/x.go", cross}, // re-enters a different sibling
		{"../one/a/../../other/x", cross},
		{"../one/.git/config", cross},
		{"../one", cross}, // a sibling with no path in it
		{"../one/", cross},
		{"../.git/x", cross},
		{"../../x", cross},
		{"../*/x.go", cross},           // a glob may not choose the sibling
		{"../{one,other}/x.go", cross}, // nor may a brace
		{"{../one/x.go,tools/a.go}", cross},
		{"tools/../one/x.go", cross},
		{"./../one/x.go", cross},
		{"..//one/x.go", cross},
		{"../one//x.go", cross},
		{"../one/x.go", same}, // a same-repository run binds no sibling
	} {
		if _, err := siblingTestBuild(t, "files:\n- `"+tc.decl+"`", tc.in); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q (source %q): got %v; want explicit refusal", tc.decl, tc.in.SourceRepo, err)
		}
	}
}

// A sibling declaration never resolves against the local tree, whoever calls.
// regression: #3483
func TestSiblingNeverResolvedLocal(t *testing.T) {
	tree := wakeTestTree()
	tree.files["one/x.go"] = "package one\n"
	if _, err := ResolveDeclaredScope([]string{"../one/x.go"}, tree); ExitCodeOf(err) != ExitRefused {
		t.Fatalf("sibling declaration resolved against the local tree: %v", err)
	}
	rr := NewRootRevisionReader(t.TempDir(), "v1")
	for _, key := range []string{"scope:../one/x.go", "file:../one/x.go"} {
		if _, ok := rr.Revision(key); ok {
			t.Errorf("%s was read outside the root", key)
		}
	}
}

// The planner holds a sibling declaration only while the receipt pins that
// sibling's source commit; otherwise it is could-not-check, never unchanged.
// regression: #3483
func TestSiblingPlannerScope(t *testing.T) {
	crossRepoRoster(t)
	source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
	const brief = "docs/streams/sample/brief-01-plan.md"
	for _, tc := range []struct {
		files string
		key   string
		want  InputsState
	}{
		{"files:\n- `../one/source.txt`", "example-org/one", InputsUnchanged},
		{"files:\n- `../one/source.txt`", "", InputsCouldNotCheck},
		{"files:\n- `../other/source.txt`", "example-org/one", InputsCouldNotCheck},
	} {
		body := "# Plan\n\n## Context\n\n" + tc.files + "\n"
		tracking := crossRepoGitFixture(t, "example-org/tracking", brief, body)
		sha, _ := verifierGit(source, "rev-parse", "HEAD")
		t.Setenv(RootsEnv, "example-org/one="+source)
		landed, err := os.ReadFile(filepath.Join(tracking, brief))
		if err != nil {
			t.Fatal(err)
		}
		input := map[string]string{"file:" + brief: Sha256Hex(landed), "tool": "test-tool"}
		if tc.key != "" {
			input[InputKeyRepoSourcePrefix+tc.key] = sha
		}
		r := NewWakeReceipt("fixture", "example-org/tracking", "sample/01", "verifier", "verify-fail", sha, nil, input, "test-tool", "implementation", "#1", WakeRelevantInputChanged, "", "", "2026-10-13T20:00:00Z")
		if state, why := Unchanged(r, NewRootRevisionReader(tracking, "test-tool")); state != tc.want {
			t.Errorf("%q with source key %q: %v %s, want %v", tc.files, tc.key, state, why, tc.want)
		}
	}
}

// Exactly one non-empty source key binds a sibling; malformed or ambiguous
// coverage is refused.
// regression: #3483
func TestSiblingCoverageExact(t *testing.T) {
	one := InputKeyRepoSourcePrefix + "example-org/one"
	for _, tc := range []struct {
		decl   string
		inputs map[string]string
		ok     bool
	}{
		{"../one/x.go", map[string]string{one: siblingTestSource}, true},
		{"../ONE/x.go", map[string]string{one: siblingTestSource}, true},
		{"../one/x.go", map[string]string{one: ""}, false},
		{"../one/x.go", map[string]string{one: siblingTestSource, InputKeyRepoSourcePrefix + "fork-org/one": siblingTestSource}, false},
		{"../one/x.go", map[string]string{InputKeyRepoSourcePrefix + "example-org/other": siblingTestSource}, false},
		{"../one/x.go", map[string]string{"file:one/x.go": siblingTestSource}, false},
		{"../one/../x.go", map[string]string{one: siblingTestSource}, false},
		{"one/x.go", map[string]string{one: siblingTestSource}, false},
		{"../one/x.go", map[string]string{InputKeyRepoSourcePrefix + "one": siblingTestSource}, false},
	} {
		err := CheckSiblingCoverage([]string{tc.decl}, tc.inputs)
		if (err == nil) != tc.ok || (err != nil && ExitCodeOf(err) != ExitRefused) {
			t.Errorf("%q %v: got %v, want ok=%v", tc.decl, tc.inputs, err, tc.ok)
		}
	}
}

// The completeness check covers local declarations by their scope: keys,
// in either key form, and sibling ones by their repo-source: commit; neither
// half stands in for the other.
// regression: #3483
func TestSiblingMixedScopeCheck(t *testing.T) {
	crossRepoRoster(t)
	tree := scopeKeyFixture()
	brief := "## Context\nfiles:\n- `tools/pkg/`\n- `../one/x.go`\n"
	tree.files[wakeTestBriefPath] = brief
	in := siblingTestInput()
	in.LandedBrief = []byte(brief)
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	r.Brief = "demo/01"
	const dirKey, sourceKey = "scope:tools/pkg/", InputKeyRepoSourcePrefix + "example-org/one"
	if r.Inputs[dirKey] == "" || r.Inputs[sourceKey] != siblingTestSource {
		t.Fatalf("mixed record lacks a key: %v", r.Inputs)
	}
	rr := NewRootRevisionReader(scopeKeyRoot(t, tree.files), in.ToolVersion)
	if err := rr.CheckReceiptScope(r); err != nil {
		t.Fatalf("complete mixed receipt refused: %v", err)
	}
	legacy := maps.Clone(r.Inputs)
	legacy["scope:tools/pkg"] = legacy[dirKey]
	delete(legacy, dirKey)
	if err := rr.CheckReceiptScope(WakeReceipt{Brief: r.Brief, Inputs: legacy}); err != nil {
		t.Fatalf("legacy-keyed mixed receipt refused: %v", err)
	}
	for _, drop := range []string{dirKey, sourceKey} {
		cut := maps.Clone(r.Inputs)
		delete(cut, drop)
		if err := rr.CheckReceiptScope(WakeReceipt{Brief: r.Brief, Inputs: cut}); err == nil {
			t.Errorf("receipt without %s passed completeness", drop)
		}
	}
}

// The repository-name rule alone keeps ".", "..", ".git" and glob or space
// names from being a sibling, even where a repo-source: key spells the same
// name and the rest of the path is canonical.
// regression: #3483
func TestSiblingNameIsolated(t *testing.T) {
	if _, _, ok := SiblingDeclaration("../one/x.go"); !ok {
		t.Fatal("control: ../one/x.go is not a sibling declaration")
	}
	for _, name := range []string{".", "..", ".git", "o*e", "o?e", "o[n]e", "o{n}e"} {
		decl := "../" + name + "/x.go"
		if _, _, ok := SiblingDeclaration(decl); ok {
			t.Errorf("%q accepted as a sibling declaration", decl)
		}
		inputs := map[string]string{InputKeyRepoSourcePrefix + "example-org/" + name: siblingTestSource}
		if err := CheckSiblingCoverage([]string{decl}, inputs); ExitCodeOf(err) != ExitRefused {
			t.Errorf("%q bound by a key of the same name: %v", decl, err)
		}
	}
}
