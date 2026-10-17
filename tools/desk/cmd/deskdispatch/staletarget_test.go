package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2746
//
// The target-directory refusal names the stale directory; the hint must hand the operator
// the exact command for THAT directory, on every lane that can hit it, not a `<path>`
// placeholder to reconstruct.
func TestHintNamesTheExactReclaimCommand(t *testing.T) {
	stale := filepath.Join(t.TempDir(), "stale-worktree-dir")
	said := "refused: target already exists (never clobbered): " + stale
	want := "`deskwt remove " + stale + "`"
	for _, kit := range []string{"worker", "verifier"} {
		got := worktreeCreateHint(kit, "feat/item-1", said)
		if !strings.Contains(got, want) {
			t.Errorf("%s hint does not carry the exact command %s:\n%s", kit, want, got)
		}
		if strings.Contains(got, "<path>") {
			t.Errorf("%s hint still leaves a <path> placeholder:\n%s", kit, got)
		}
	}
	// A message that carries no absolute path is never guessed at.
	if got := worktreeCreateHint("worker", "feat/item-1", "refused: target already exists (never clobbered)"); !strings.Contains(got, "`deskwt remove <path>`") {
		t.Errorf("a pathless refusal must keep the placeholder, not a guess:\n%s", got)
	}
}

// regression: #2746
//
// A post-claim abort leaves the worktree this run created in place. The abort report must
// name it and the exact command that reclaims it, so the same-key re-dispatch is not a
// "target exists" dead end the operator has to diagnose by hand.
func TestAbortNamesTheWorktreeItLeft(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	home := t.TempDir()
	s.replies = happyReplies(home)
	writeStateHooks(t, "before_run: exit 1\n")

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("before_run failure rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if want := "`deskwt remove " + home + "`"; !strings.Contains(stderr, want) {
		t.Errorf("the abort report does not carry %s:\n%s", want, stderr)
	}
}

// A refused worktree-create created nothing, so the report must not point at a worktree.
func TestCreateRefusalNamesNoWorktree(t *testing.T) {
	h := &heldClaim{phrase: "released"}
	err := h.settle(deskkit.Refused("step worktree-create: refused"))
	if err == nil || strings.Contains(err.Error(), "deskwt remove") {
		t.Fatalf("a run that created no worktree named one to remove: %v", err)
	}
}
