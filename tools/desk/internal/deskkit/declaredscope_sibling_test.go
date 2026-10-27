package deskkit

import (
	"encoding/json"
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
