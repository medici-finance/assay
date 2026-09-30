package main

// Fail-first wiring tests for the publish-identity gate in deskevidence (issue #1490 lane B).
//
// deskevidence commits Evidence AS the verifier App via the Contents API, but derives the
// witness Runner from the verifier WORKTREE's identity — the value #1490 saw come out wrong.
// This gate refuses, before any network call, when the worktree the landing is authored from
// carries commits ahead of the target branch that are not the verifier's.
//
// setupFake stubs the gate no-op for the general suite (its roots are not git repos); these
// tests build a real git root and restore the real gate.
//
// FAIL-FIRST. Before the gate was wired, deskevidence proceeded to WriteFile regardless of
// the worktree's local commits. The refusal subtest proves the wiring stops the landing
// (rc 5, ZERO WriteFile); dropping the `publishIdentityGate(...)` call from cmdEvidence
// turns it green-would-write.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifierBotEmail is the fixture roster's verifier bot; issueLoopBotEmail is another role's
// bot — the cross-role misattribution #1490 observed in a verifier worktree.
const (
	verifierBotEmail  = "300000005+assay-verifier-app[bot]@users.noreply.github.com"
	issueLoopBotEmail = "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"
)

const evidenceBrief = "docs/streams/x/brief.md"

// gitRoot builds a real git checkout: a brief file committed on main and recorded as
// origin/main. When aheadEmail is non-empty it adds ONE commit ahead of origin/main authored
// and committed under that identity — the range the gate inspects.
func gitRoot(t *testing.T, aheadName, aheadEmail string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-b", "main")
	run("config", "commit.gpgsign", "false")
	run("config", "user.email", "seed@example.org")
	run("config", "user.name", "Seed")
	write(evidenceBrief, "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	run("add", ".")
	run("commit", "-m", "init")
	run("update-ref", "refs/remotes/origin/main", "HEAD")

	if aheadEmail != "" {
		run("config", "user.email", aheadEmail)
		run("config", "user.name", aheadName)
		write("unrelated.txt", "ahead\n")
		run("add", "unrelated.txt")
		run("commit", "-m", "ahead of main")
	}
	return root
}

func useRealPublishIdentityGate(t *testing.T) {
	t.Helper()
	stub := publishIdentityGateFn
	publishIdentityGateFn = productionPublishIdentityGateFn
	t.Cleanup(func() { publishIdentityGateFn = stub })
}

func TestPublishIdentityGateWiredRefusesForeignWorktree(t *testing.T) {
	f, _ := setupFake(t)
	f.setFile(evidenceBrief, "# Brief\n\n## Evidence\n")
	useRealPublishIdentityGate(t)
	// The verifier worktree carries a commit authored as ANOTHER role's bot — #1490.
	root := gitRoot(t, "Assay Issue Loop", issueLoopBotEmail)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidenceBrief, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("landing from a foreign-identity worktree rc = %d, want 5 (refused)", code)
	}
	if f.putCalls != 0 {
		t.Fatalf("the gate refused but deskevidence still wrote %d time(s) — Evidence landed from an untrusted worktree", f.putCalls)
	}
}

func TestPublishIdentityGateWiredCleanWorktreePasses(t *testing.T) {
	f, _ := setupFake(t)
	f.setFile(evidenceBrief, "# Brief\n\n## Evidence\n")
	useRealPublishIdentityGate(t)
	// The sanctioned post-merge flow: the worktree sits at origin/main, nothing ahead.
	root := gitRoot(t, "", "")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidenceBrief, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("landing from a clean (at-origin/main) worktree rc = %d, want 0 (ok)", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("a clean worktree must let the landing through: WriteFile calls = %d, want 1", f.putCalls)
	}
}

// TestPublishIdentityGateSeamIsRealInProduction — the seam setupFake stubs must, in a fresh
// binary, be the real deskkit.PublishIdentityMatchesRole; otherwise the wiring tests above
// are vacuous.
func TestPublishIdentityGateSeamIsRealInProduction(t *testing.T) {
	if productionPublishIdentityGateFn == nil {
		t.Fatal("productionPublishIdentityGateFn is nil — the seam has no recorded production binding")
	}
	if err := productionPublishIdentityGateFn(deskkit.PublishIdentityInput{
		Role: "verifier",
		Commits: func(string, string) ([]deskkit.PublishCommit, error) {
			return nil, deskkit.Unverifiable("seam probe", nil)
		},
	}); err == nil {
		t.Fatal("productionPublishIdentityGateFn is not the real gate — a real gate reports the probe error, a stub returns nil")
	}
}
