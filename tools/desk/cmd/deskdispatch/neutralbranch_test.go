package main

// neutralbranch_test.go — end-to-end coverage of the public-target branch rule (branchname.go),
// driven through run() so it asserts the branch the worktree step is actually handed.
//
// THE DEFECT. The default branch was `feat/<item-key>` whatever the target. An item key of the
// `<alias>--issue-<N>` shape, whose alias names some OTHER (possibly private) repo, dispatched
// into a PUBLIC repo put that alias and issue number in the public branch list — and, through the
// changelog fragment named after the branch, in the public tree.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantRosterWithAllowedRepoPublic re-plants the fixture roster with allowedRepo configured
// `:public` — the only change from the fixture every other test runs against.
func plantRosterWithAllowedRepoPublic(t *testing.T, home string) {
	t.Helper()
	const from, to = allowedRepo + ":ci:private", allowedRepo + ":ci:public"
	if !strings.Contains(fixtureRoster, from) {
		t.Fatalf("fixture roster no longer carries %q — update this helper", from)
	}
	p := filepath.Join(home, ".config", "assay", "roster.env")
	if err := os.WriteFile(p, []byte(strings.Replace(fixtureRoster, from, to, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	if deskkit.RepoVisibility(allowedRepo) != deskkit.VisibilityPublic {
		t.Fatalf("precondition: %s did not reload as public", allowedRepo)
	}
}

// dispatchWorker runs a stubbed worker dispatch of item (plus extra args) and returns the stub.
func dispatchWorker(t *testing.T, public bool, item string, extra ...string) *stub {
	t.Helper()
	s := &stub{}
	home, root := s.install(t)
	if public {
		plantRosterWithAllowedRepoPublic(t, home)
	}
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	args := append([]string{item, "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}, extra...)
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("dispatch %v rc = %d, want 0", args, rc)
	}
	return s
}

// worktreeBranch returns the --branch value the worktree step was handed, or "".
func (s *stub) worktreeBranch() string {
	for _, c := range s.deskwtCalls() {
		f := strings.Fields(c)
		for i := 0; i+1 < len(f); i++ {
			if f[i] == "--branch" {
				return f[i+1]
			}
		}
	}
	return ""
}

// A PUBLIC target with an issue key whose label is another repo's: the branch carries neither
// the label nor the issue number — it is the hashed neutral form.
func TestPublicTargetForeignIssueKeyGetsNeutralBranch(t *testing.T) {
	s := dispatchWorker(t, true, "other-tracker--issue-4242")
	got := s.worktreeBranch()
	if !strings.HasPrefix(got, "feat/item-") {
		t.Fatalf("branch = %q, want feat/item-<hash>; deskwt calls: %v", got, s.deskwtCalls())
	}
	for _, leak := range []string{"other-tracker", "4242"} {
		if strings.Contains(got, leak) {
			t.Errorf("branch %q still carries %q from the foreign item key", got, leak)
		}
	}
	// The CLAIM KEY is unchanged: it is a ref in the claim store, not tree content.
	if !s.ran("dispatch-claim.sh acquire other-tracker--issue-4242") {
		t.Errorf("the claim key changed; calls: %v", s.calls)
	}
}

// A PUBLIC target with a foreign BRIEF claim key: the neutral branch keeps the brief's own
// stream and number, drops the label.
func TestPublicTargetForeignBriefKeyGetsStreamBranch(t *testing.T) {
	s := dispatchWorker(t, true, "other-tracker--example-stream--07")
	if got := s.worktreeBranch(); got != "feat/example-stream-07" {
		t.Fatalf("branch = %q, want feat/example-stream-07", got)
	}
}

// A PRIVATE target keeps the historical derivation byte-for-byte.
func TestPrivateTargetForeignKeyBranchUnchanged(t *testing.T) {
	s := dispatchWorker(t, false, "other-tracker--issue-4242")
	if got := s.worktreeBranch(); got != "feat/other-tracker--issue-4242" {
		t.Fatalf("branch = %q, want feat/other-tracker--issue-4242", got)
	}
}

// A PUBLIC target whose item key carries the target's OWN label is unchanged: there is nothing
// foreign to withhold.
func TestPublicTargetSameRepoKeyBranchUnchanged(t *testing.T) {
	s := dispatchWorker(t, true, "assay--issue-4242")
	if got := s.worktreeBranch(); got != "feat/assay--issue-4242" {
		t.Fatalf("branch = %q, want feat/assay--issue-4242", got)
	}
}

// An explicit --branch wins over the neutral derivation.
func TestPublicTargetExplicitBranchWins(t *testing.T) {
	s := dispatchWorker(t, true, "other-tracker--issue-4242", "--branch", "chosen-name")
	if got := s.worktreeBranch(); got != "chosen-name" {
		t.Fatalf("branch = %q, want chosen-name", got)
	}
}

// The prompt names the neutral branch AND still names the original item key for the worker.
func TestPublicTargetPromptNamesNeutralBranchAndOriginalKey(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantRosterWithAllowedRepoPublic(t, home)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	pf := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"other-tracker--issue-4242", "--root", root, "--repo", allowedRepo,
		"--prompt-file", pf}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d", rc)
	}
	b, err := os.ReadFile(pf)
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(b)
	want := defaultBranch("other-tracker--issue-4242", allowedRepo)
	if !strings.Contains(prompt, "**Branch:** `"+want+"`") {
		t.Errorf("prompt does not name the neutral branch %s", want)
	}
	if strings.Contains(prompt, "feat/other-tracker") {
		t.Errorf("prompt still names the key-derived branch")
	}
	if !strings.Contains(prompt, "other-tracker--issue-4242") {
		t.Errorf("prompt no longer names the original item key")
	}
}
