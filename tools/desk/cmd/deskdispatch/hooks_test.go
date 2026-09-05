package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeStateHooks plants a hooks.yaml in the desk-tools state dir (<HOME>/.config/assay),
// the ONE place hooks are read from. install() sets HOME to a temp dir, so this is that
// session's state dir.
func writeStateHooks(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestBeforeRunFailureAbortsAndReleases (Verify row 5): a before_run hook that fails aborts
// the dispatch with exit 6, emits NO prompt, and RELEASES the durable claim so a corrected
// re-run is not wedged behind a dispatcher that never dispatched. This is the one lifecycle
// point after the claim, so the release is the property that keeps the wedged-item class
// closed for it.
func TestBeforeRunFailureAbortsAndReleases(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)

	// The worktree home must be a real directory: before_run runs with cwd = the worktree.
	home := t.TempDir()
	s.replies = happyReplies(home)

	// A before_run hook that fails.
	writeStateHooks(t, "before_run: exit 1\n")

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc := run([]string{"example-stream--07", "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile})

	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("before_run failure rc = %d, want %d (unverifiable)", rc, deskkit.ExitUnverifiable)
	}
	// No prompt may be emitted — the agent's "run" never begins.
	if _, err := os.Stat(promptFile); err == nil {
		t.Error("a prompt was emitted after before_run failed — the attempt must abort with no prompt")
	}
	// The claim must be released via the consumer claim script's own release verb.
	if !s.ran("dispatch-claim.sh release example-stream--07") {
		t.Errorf("the claim was NOT released after before_run failed — the item is wedged. calls: %v", s.calls)
	}
	// And the worktree WAS created (before_run runs after worktree-create), so this is a
	// genuine post-claim abort, not a pre-claim refusal.
	if !s.ran("deskwt add") {
		t.Error("before_run should run AFTER the worktree is created")
	}
}

// TestBeforeRunSuccessEmitsPrompt is the positive control: with a before_run hook that
// succeeds, the dispatch completes and a prompt is emitted — the hook is a gate, not a wall.
func TestBeforeRunSuccessEmitsPrompt(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	home := t.TempDir()
	s.replies = happyReplies(home)

	writeStateHooks(t, "before_run: exit 0\n")

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc := run([]string{"example-stream--07", "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("before_run success rc = %d, want 0", rc)
	}
	if _, err := os.Stat(promptFile); err != nil {
		t.Errorf("no prompt was emitted though before_run succeeded: %v", err)
	}
	if s.ran("dispatch-claim.sh release") {
		t.Error("the claim was released though before_run succeeded — release is a failure path only")
	}
}
