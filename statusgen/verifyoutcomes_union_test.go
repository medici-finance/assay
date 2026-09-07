package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitIn runs git in dir with a hermetic identity/config (no user global/system config leaks in),
// failing the test on error. Local operations only — init/commit/merge never touch a network.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func voWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// TestVerifyOutcomesUnionMergeInEitherOrder is the #588 acceptance: two branches each appending a
// distinct row to a merge=union JSON-lines log merge CLEANLY in EITHER direction — both appends
// survive, no conflict markers. This is what stops concurrent verifier Evidence PRs from
// re-conflicting on the append-only log. It runs in a scratch repo so it asserts git's union
// driver behaviour directly, independent of the real tree.
func TestVerifyOutcomesUnionMergeInEitherOrder(t *testing.T) {
	for _, order := range []struct{ first, second string }{{"A", "B"}, {"B", "A"}} {
		t.Run(order.first+"-then-"+order.second, func(t *testing.T) {
			dir := t.TempDir()
			gitIn(t, dir, "init", "-q", "-b", "main")
			// The rule under test, mirroring the repo's .gitattributes line for the log.
			voWriteFile(t, filepath.Join(dir, ".gitattributes"), "log.jsonl merge=union\n")
			voWriteFile(t, filepath.Join(dir, "log.jsonl"), `{"brief":"base/00"}`+"\n")
			gitIn(t, dir, "add", ".")
			gitIn(t, dir, "commit", "-qm", "base")

			// Branch A appends row A; branch B (off the same base) appends a DIFFERENT row.
			gitIn(t, dir, "checkout", "-qb", "branchA")
			appendLine(t, filepath.Join(dir, "log.jsonl"), `{"brief":"a/01"}`)
			gitIn(t, dir, "commit", "-qam", "A")
			gitIn(t, dir, "checkout", "-q", "main")
			gitIn(t, dir, "checkout", "-qb", "branchB")
			appendLine(t, filepath.Join(dir, "log.jsonl"), `{"brief":"b/02"}`)
			gitIn(t, dir, "commit", "-qam", "B")

			// Merge the second branch into the first — no --edit, so a conflict would exit non-zero
			// and gitIn would fail the test.
			gitIn(t, dir, "checkout", "-q", "branch"+order.first)
			gitIn(t, dir, "merge", "--no-edit", "branch"+order.second)

			body, err := os.ReadFile(filepath.Join(dir, "log.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			s := string(body)
			if strings.Contains(s, "<<<<<<<") || strings.Contains(s, ">>>>>>>") {
				t.Fatalf("union merge left conflict markers:\n%s", s)
			}
			for _, want := range []string{"base/00", "a/01", "b/02"} {
				if !strings.Contains(s, want) {
					t.Fatalf("row %q missing after the %s→%s union merge:\n%s", want, order.second, order.first, s)
				}
			}
		})
	}
}

// TestRepoMarksVerifyOutcomesUnion asserts the repo's OWN .gitattributes gives the append-only
// evidence log the union merge driver (#588), so the acceptance above applies to the real file.
// It skips when the test tree is not a git checkout (e.g. a vendored source copy), because the
// scratch-repo test above is the driver-behaviour guarantee; this one guards the wiring.
func TestRepoMarksVerifyOutcomesUnion(t *testing.T) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not a git checkout; the scratch-repo test covers the driver behaviour")
	}
	out := gitIn(t, strings.TrimSpace(string(root)), "check-attr", "merge", "--", "docs/streams/verify-outcomes.jsonl")
	if !strings.Contains(out, "merge: union") {
		t.Fatalf("docs/streams/verify-outcomes.jsonl must be merge=union in .gitattributes; got: %s", out)
	}
}
