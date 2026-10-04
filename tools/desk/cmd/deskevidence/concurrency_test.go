package main

// concurrency_test.go — Verify row 6: the #882 mergeability proxy.
//
// The brief's own facts (2026-09-27): for each of 19 real CONFLICTING verifier-App PRs,
// `git merge-tree --write-tree` against main with attributes disabled is a faithful local proxy
// for the forge's server-side merge verdict (clean exit 0 with attributes, conflict exit 1
// without — the forge applies no .gitattributes merge driver at all). This test exercises that
// SAME proxy directly, in a scratch repository, to prove the NEW per-file record layout never
// needs a merge driver in the first place — unlike the shared log it replaces, which the
// negative control below shows DOES conflict under the identical proxy with NO driver present.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitversion"
)

// gitEmptyTreeSHA is git's well-known empty-tree object id — passed to `--attr-source=` so
// merge-tree reads NO .gitattributes from any tree, the same attribute-free proxy the brief's
// facts verified against the forge's own verdict.
const gitEmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// mergeTreeConflictExit is the exit code `git merge-tree --write-tree` returns when the merge
// has conflicts (0 is clean). Any OTHER non-zero code — 129 for a usage error such as an
// unknown `--attr-source` on git older than 2.40, 128 for a fatal error — means the proxy did
// not run, so it is never read as "conflict".
const mergeTreeConflictExit = 1

// requireAttrSource skips the proxy tests, naming why, on a git older than 2.40: the
// attribute-free proxy needs the top-level `--attr-source` option, and an older git rejects
// it as a usage error (exit 129) before merge-tree runs at all.
func requireAttrSource(t *testing.T) {
	t.Helper()
	gitversion.RequireGit(t, 2, 40, "merge-tree --attr-source")
}

func concurrencyGitIn(t *testing.T, dir string, args ...string) string {
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

func concurrencyWriteAndAdd(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	concurrencyGitIn(t, dir, "add", rel)
}

// mergeTreeExitCode runs the attribute-free merge-tree proxy and returns its exit code: 0 clean,
// 1 conflict. Any other outcome (a usage error, a bad ref, git itself missing) fails the test
// outright, so neither caller can mistake "the proxy did not run" for a verdict.
func mergeTreeExitCode(t *testing.T, dir, base, head string) int {
	t.Helper()
	cmd := exec.Command("git", "-c", "core.attributesFile=/dev/null", "--attr-source="+gitEmptyTreeSHA,
		"merge-tree", "--write-tree", base, head)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == mergeTreeConflictExit {
		return mergeTreeConflictExit
	}
	t.Fatalf("merge-tree %s %s: %v\n%s", base, head, err, out)
	return -1
}

// TestOutcomeRecordsConcurrentLandingsMergeable is Verify row 6.
func TestOutcomeRecordsConcurrentLandingsMergeable(t *testing.T) {
	requireAttrSource(t)
	t.Run("per-file records: B stays mergeable after A lands, with NO merge driver", func(t *testing.T) {
		dir := t.TempDir()
		concurrencyGitIn(t, dir, "init", "-q", "-b", "main")
		// Deliberately NO .gitattributes in this scratch repo — the whole point.
		concurrencyWriteAndAdd(t, dir, "docs/streams/example-stream/brief-01-a.md", "# A\n\n## Evidence\n<!-- appended at verification time -->\n")
		concurrencyWriteAndAdd(t, dir, "docs/streams/example-stream/brief-02-b.md", "# B\n\n## Evidence\n<!-- appended at verification time -->\n")
		concurrencyGitIn(t, dir, "commit", "-qm", "base")

		concurrencyGitIn(t, dir, "checkout", "-qb", "branchA")
		concurrencyWriteAndAdd(t, dir, "docs/streams/verify-outcomes/example-stream/01-20260907T010000Z-aaaaaaaaaaaa.json",
			`{"ts":"2026-09-07T01:00:00Z","brief":"example-stream/01","outcome":"verified"}`+"\n")
		concurrencyWriteAndAdd(t, dir, "docs/streams/example-stream/brief-01-a.md",
			"# A\n\n## Evidence\n<!-- appended at verification time -->\n| 1 | `go test ./...` | 0 | ok |\n")
		concurrencyGitIn(t, dir, "commit", "-qam", "A: brief 01 verified")

		concurrencyGitIn(t, dir, "checkout", "-q", "main")
		concurrencyGitIn(t, dir, "checkout", "-qb", "branchB")
		concurrencyWriteAndAdd(t, dir, "docs/streams/verify-outcomes/example-stream/02-20260907T020000Z-bbbbbbbbbbbb.json",
			`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/02","outcome":"verified"}`+"\n")
		concurrencyWriteAndAdd(t, dir, "docs/streams/example-stream/brief-02-b.md",
			"# B\n\n## Evidence\n<!-- appended at verification time -->\n| 1 | `go test ./...` | 0 | ok |\n")
		concurrencyGitIn(t, dir, "commit", "-qam", "B: brief 02 verified")

		// A lands on main first (the real sequencing: one Evidence PR merges before the sibling).
		concurrencyGitIn(t, dir, "checkout", "-q", "main")
		concurrencyGitIn(t, dir, "merge", "-q", "--no-edit", "branchA")

		if code := mergeTreeExitCode(t, dir, "main", "branchB"); code != 0 {
			t.Fatalf("per-file records: B is CONFLICTING against main after A landed (merge-tree exit %d) — "+
				"the per-file layout must never need a merge driver", code)
		}
	})

	t.Run("negative control: two branches appending to ONE shared log DO conflict with no driver — the proxy can fail", func(t *testing.T) {
		dir := t.TempDir()
		concurrencyGitIn(t, dir, "init", "-q", "-b", "main")
		// Still no .gitattributes: this is the shape #882 retires, reproduced to prove the
		// proxy is not vacuously green-by-construction (row 6's own pre-mortem: "the proxy
		// itself cannot fail" is the failure mode this negative control closes).
		concurrencyWriteAndAdd(t, dir, "docs/streams/verify-outcomes.jsonl",
			`{"ts":"2026-09-07T00:00:00Z","brief":"example-stream/00","outcome":"verified"}`+"\n")
		concurrencyGitIn(t, dir, "commit", "-qm", "base")

		concurrencyGitIn(t, dir, "checkout", "-qb", "branchA")
		appendLineTo(t, filepath.Join(dir, "docs/streams/verify-outcomes.jsonl"),
			`{"ts":"2026-09-07T01:00:00Z","brief":"example-stream/01","outcome":"verified"}`)
		concurrencyGitIn(t, dir, "add", "docs/streams/verify-outcomes.jsonl")
		concurrencyGitIn(t, dir, "commit", "-qm", "A appends a row")

		concurrencyGitIn(t, dir, "checkout", "-q", "main")
		concurrencyGitIn(t, dir, "checkout", "-qb", "branchB")
		appendLineTo(t, filepath.Join(dir, "docs/streams/verify-outcomes.jsonl"),
			`{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/02","outcome":"verified"}`)
		concurrencyGitIn(t, dir, "add", "docs/streams/verify-outcomes.jsonl")
		concurrencyGitIn(t, dir, "commit", "-qm", "B appends a row")

		concurrencyGitIn(t, dir, "checkout", "-q", "main")
		concurrencyGitIn(t, dir, "merge", "-q", "--no-edit", "branchA")

		if code := mergeTreeExitCode(t, dir, "main", "branchB"); code != mergeTreeConflictExit {
			t.Fatalf("negative control: two branches appending to one shared log with NO merge driver "+
				"should CONFLICT (merge-tree exit %d; this is the exact defect class #882 retires) — the proxy "+
				"reported exit %d, which means it cannot distinguish the old shape from the new one",
				mergeTreeConflictExit, code)
		}
	})
}

func appendLineTo(t *testing.T, path, line string) {
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
