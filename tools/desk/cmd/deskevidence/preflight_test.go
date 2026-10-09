package main

// preflight_test.go — pins deskevidence's write-boundary PRE-FLIGHT for the --brief-path
// landing shape: the statusgen PROBLEM-diff gate lints the would-be state BEFORE the write,
// and a landing that would add a PROBLEM is refused with nothing written.
//
// TestLintDiffIntroducedProblemRefused (deskevidence_test.go) stubs the whole lintDiffFn
// seam, and lintdiff_test.go covers lintDiffAt alone. Neither proves the two are composed:
// a refactor that stopped calling the real lintDiffAt from cmdEvidence, or that staged the
// wrong bytes, would keep both green. This test runs cmdEvidence with the PRODUCTION
// lintDiffFn and stubs only the single statusgen shell beneath it — and that stub is a
// function of the TREE it is pointed at (it reads the brief from disk), never of the call
// order, so the verdict follows the bytes actually staged.
//
// Shapes covered. Only --brief-path: there the bytes to commit (the remote brief merged with
// the fragment) differ from the brief in the landing worktree, so the before-run and the
// after-run lint different trees and the gate can refuse. NOT covered: the direct shape (an
// --evidence-file under --root, no --brief-path). There the bytes to commit ARE the local
// file, already sitting at the target path, so both runs lint identical trees and the gate
// cannot refuse anything — a gap in the gate itself, not in this test, tracked as #2460.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRefuseIntroducedProblem: a --brief-path landing whose merged brief makes statusgen
// report a NEW PROBLEM is refused (exit 5) before any write; a PROBLEM already standing in
// the tree is not this landing's and never blocks it. The before-run sees the worktree's own
// brief, the after-run sees exactly the bytes that would be committed, and the worktree comes
// back unchanged either way.
func TestRefuseIntroducedProblem(t *testing.T) {
	const (
		briefPath   = "docs/streams/x/brief.md"
		remoteBrief = "# Brief\n\n## Evidence\n| 1 | a | b |\n"
		marker      = "../sibling/x"
		preexisting = "PROBLEM: docs/streams/other/brief.md: unrelated pre-existing red"
		introduced  = "PROBLEM: docs/streams/x/brief.md: backticked path \"../sibling/x\" does not exist — " +
			"for a sibling-repo file, prefix it ../<repo>/../sibling/x"
	)
	for _, tc := range []struct {
		name      string
		fragment  string
		wantCode  int
		wantPuts  int
		wantAudit string
	}{
		{"new problem refused", "| 2 | `" + marker + "` | d |\n", deskkit.ExitRefused, 0, introduced},
		{"standing problem passes", "| 2 | c | d |\n", deskkit.ExitOK, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			lintDiffFn = lintDiffAt // the production composition; setupFake restores it on cleanup

			root := rootWithFile(t, briefPath, remoteBrief)
			abs := filepath.Join(root, briefPath)
			var seen []string
			old := statusgenLintFn
			statusgenLintFn = func(r string) ([]string, error) {
				if r != root {
					t.Fatalf("statusgen ran against %q, not the landing worktree %q", r, root)
				}
				b, err := os.ReadFile(abs)
				if err != nil {
					t.Fatalf("statusgen stub could not read the brief in the tree: %v", err)
				}
				seen = append(seen, string(b))
				if strings.Contains(string(b), marker) {
					return []string{preexisting, introduced}, nil
				}
				return []string{preexisting}, nil
			}
			t.Cleanup(func() { statusgenLintFn = old })
			f.setFile(briefPath, remoteBrief)
			fragment := writeRepoFile(t, "row.md", tc.fragment)
			want, _ := mergeEvidenceContent([]byte(remoteBrief), []byte(tc.fragment))

			code := run([]string{"example-org/tracker", "main",
				"--evidence-file", fragment, "--brief-path", briefPath, "--root", root})
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d", code, tc.wantCode)
			}
			if len(seen) != 2 {
				t.Fatalf("statusgen ran %d time(s); the pre-flight runs it exactly twice (before, staged)", len(seen))
			}
			if seen[0] != remoteBrief {
				t.Fatalf("the before-run did not lint the worktree as it stands; it saw:\n%s", seen[0])
			}
			if seen[1] != string(want) {
				t.Fatalf("the after-run did not lint the bytes to be committed; it saw:\n%s\nwant:\n%s", seen[1], want)
			}
			if seen[0] == seen[1] {
				t.Fatal("before and after runs saw the same bytes — the fixture cannot tell a staged lint from an unstaged one")
			}
			if f.putCalls != tc.wantPuts {
				t.Fatalf("writes = %d, want %d", f.putCalls, tc.wantPuts)
			}
			if tc.wantPuts == 1 && f.putContent != string(want) {
				t.Fatalf("the committed bytes differ from what the after-run linted:\n%s", f.putContent)
			}
			if b, _ := os.ReadFile(abs); string(b) != remoteBrief {
				t.Fatalf("the landing worktree was left changed by the pre-flight: %q", b)
			}
			if tc.wantAudit != "" && !strings.Contains(lastAudit(t).Detail, tc.wantAudit) {
				t.Fatalf("the refusal does not carry the introduced PROBLEM verbatim: %q", lastAudit(t).Detail)
			}
		})
	}
}
