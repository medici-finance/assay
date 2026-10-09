package main

// preflight_test.go — pins deskevidence's write-boundary PRE-FLIGHT end to end: the
// statusgen PROBLEM-diff gate runs against the would-be state BEFORE the write, and a
// landing that would add a PROBLEM is refused with nothing written.
//
// TestLintDiffIntroducedProblemRefused (deskevidence_test.go) stubs the whole lintDiffFn
// seam, and lintdiff_test.go covers lintDiffAt alone. Neither proves the two are composed:
// a refactor that stopped calling the real lintDiffAt from cmdEvidence, or that staged the
// wrong bytes, would keep both green. This test runs cmdEvidence with the PRODUCTION
// lintDiffFn and stubs only the single statusgen shell beneath it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestRefuseIntroducedProblem: a landing whose staged content makes statusgen report a NEW
// PROBLEM is refused (exit 5) before any write; a PROBLEM already standing in the tree is not
// this landing's and never blocks it. Either way the landing worktree comes back unchanged.
func TestRefuseIntroducedProblem(t *testing.T) {
	const (
		evidencePath = "docs/streams/x/brief.md"
		preexisting  = "PROBLEM: docs/streams/other/brief.md: unrelated pre-existing red"
		introduced   = "PROBLEM: docs/streams/x/brief.md: backticked path \"../sibling/x\" does not exist — " +
			"for a sibling-repo file, prefix it ../<repo>/../sibling/x"
		localContent = "content\n"
	)
	for _, tc := range []struct {
		name      string
		after     []string
		wantCode  int
		wantPuts  int
		wantAudit string
	}{
		{"new problem refused", []string{preexisting, introduced}, deskkit.ExitRefused, 0, introduced},
		{"standing problem passes", []string{preexisting}, deskkit.ExitOK, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			lintDiffFn = lintDiffAt // the production composition; setupFake restores it on cleanup

			root := rootWithFile(t, evidencePath, localContent)
			abs := filepath.Join(root, evidencePath)
			var staged []byte
			calls := 0
			old := statusgenLintFn
			statusgenLintFn = func(r string) ([]string, error) {
				calls++
				if r != root {
					t.Fatalf("statusgen ran against %q, not the landing worktree %q", r, root)
				}
				if calls == 1 {
					return []string{preexisting}, nil
				}
				staged, _ = os.ReadFile(abs) // what the after-run saw: the would-be state
				return tc.after, nil
			}
			t.Cleanup(func() { statusgenLintFn = old })
			f.setFile(evidencePath, "old\n")

			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d", code, tc.wantCode)
			}
			if calls != 2 {
				t.Fatalf("statusgen ran %d time(s); the pre-flight runs it exactly twice (before, staged)", calls)
			}
			if len(staged) == 0 {
				t.Fatal("the staged run saw an empty file — the pre-flight did not lint the would-be state")
			}
			if f.putCalls != tc.wantPuts {
				t.Fatalf("writes = %d, want %d", f.putCalls, tc.wantPuts)
			}
			if b, _ := os.ReadFile(abs); string(b) != localContent {
				t.Fatalf("the landing worktree was left changed by the pre-flight: %q", b)
			}
			if tc.wantAudit != "" && !strings.Contains(lastAudit(t).Detail, tc.wantAudit) {
				t.Fatalf("the refusal does not carry the introduced PROBLEM verbatim: %q", lastAudit(t).Detail)
			}
		})
	}
}
