package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/reviewhome"
)

func finishedReview(t *testing.T) (string, reviewhome.Receipt, string) {
	t.Helper()
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "review-attempt")
	head := mustGit(t, target, "rev-parse", "HEAD")
	mustGit(t, target, "checkout", "--detach", head)
	key := "tracker--pr-42"
	ref := key + "@20261121T090000Z.0123456789ab"
	runKeyed(t, target, key)
	mustGit(t, target, "config", "--worktree", "assay.dispatchRef", ref)
	mustGit(t, target, "config", "--worktree", "assay.reviewAttempt", ref)
	r := reviewhome.Receipt{Home: resolvePath(target), Repo: "example-org/tracker", PR: 42, Key: key, Attempt: ref, Head: head, Review: "review-17"}
	if err := r.Confirm(); err != nil {
		t.Fatal(err)
	}
	admin, _, err := r.Identity()
	if err != nil {
		t.Fatal(err)
	}
	return work, r, admin
}

func TestReviewRetirementWaitsForExitThenRetriesUnmerged(t *testing.T) {
	work, r, admin := finishedReview(t)
	old := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(filepath.Join(admin, reviewhome.ReceiptFile), old, old); err != nil {
		t.Fatal(err)
	}
	plan := pruneOK(t, "--dry-run")
	if !strings.Contains(plan, "review_oldest=3h") {
		t.Fatal(plan)
	}
	if !strings.Contains(plan, "awaiting reviewer exit") || !strings.Contains(plan, "review_backlog=1") {
		t.Fatal(plan)
	}
	if code, _, _ := runCapBoth(t, []string{"remove", r.Home}); code == 0 {
		t.Fatal("explicit removal bypassed pending review")
	}
	if code, _, _ := runCapBoth(t, []string{"review-exited", r.Home, r.Key + "@20261121T090000Z.000000000000"}); code == 0 {
		t.Fatal("accepted another attempt's exit")
	}
	if _, err := os.Stat(filepath.Join(admin, reviewhome.ExitFile)); !os.IsNotExist(err) {
		t.Fatal("wrong attempt acknowledged")
	}
	// Save local ignored and untracked evidence, then finish while a reader still owns a lock.
	writeFile(t, filepath.Join(r.Home, "notes"), "review evidence")
	writeFile(t, filepath.Join(work, ".git", "info", "exclude"), "ignored-note\n")
	writeFile(t, filepath.Join(r.Home, "ignored-note"), "ignored evidence")
	bareLock(t, work, r.Home)
	rc, out, diag := runCapBoth(t, []string{"review-exited", r.Home, r.Attempt})
	if rc != 0 || !strings.Contains(out, "queued") {
		t.Fatalf("%d: %s %s", rc, out, diag)
	}
	plan = pruneOK(t, "--reclaim-stale-locks", "--lock-ttl", "1ns")
	assertLocked(t, work, r.Home, true)
	plan = pruneOK(t, "--reap-dead-sessions", "--no-singleton")
	assertLocked(t, work, r.Home, true)
	if !strings.Contains(plan, "review_backlog=1") || strings.Contains(plan, "review_backlog_bytes=0 ") {
		t.Fatal(plan)
	}
	mustGit(t, work, "worktree", "unlock", r.Home)
	for _, name := range []string{"notes", "ignored-note"} {
		if plan := pruneOK(t, "--no-singleton"); !strings.Contains(plan, "files require handoff") {
			t.Fatal(plan)
		}
		if err := os.Remove(filepath.Join(r.Home, name)); err != nil {
			t.Fatal(err)
		}
	}
	// A queued tree can become active again. The ordinary sweep applies its idle gate.
	if plan := pruneOK(t, "--no-singleton", "--min-idle", "1h"); !strings.Contains(plan, "inside the 1h") {
		t.Fatal(plan)
	}
	ageActivity(t, work, r.Home, 2*time.Hour)
	plan = pruneOK(t, "--min-idle", "1h", "--dry-run")
	if !strings.Contains(plan, "review_retired=1 review_backlog=0") {
		t.Fatal(plan)
	}
	assertExists(t, r.Home)
	plan = pruneOK(t, "--no-singleton", "--min-idle", "1h")
	if !strings.Contains(plan, "review_retired=1 review_backlog=0") {
		t.Fatal(plan)
	}
	gone(t, r.Home, plan)
	if _, err := os.Stat(admin); !os.IsNotExist(err) {
		t.Fatalf("registration survived: %v", err)
	}
}

func TestReviewRetirementRechecksAttemptHeadAndClaim(t *testing.T) {
	work, r, admin := finishedReview(t)
	if err := r.Acknowledge(admin); err != nil {
		t.Fatal(err)
	}
	holdClaim(t, work, r.Key)
	held := pruneOK(t, "--dry-run")
	if !strings.Contains(held, "cannot prove release") {
		t.Fatal(held)
	}
	mustGit(t, r.Home, "checkout", "--detach", "origin/main")
	held = pruneOK(t, "--reap-dead-sessions", "--no-singleton")
	if !strings.Contains(held, "HEAD changed") {
		t.Fatal(held)
	}
	assertExists(t, r.Home)
	mustGit(t, r.Home, "config", "--worktree", "assay.dispatchRef", r.Key+"@20261121T090000Z.000000000000")
	if err := r.Confirm(); err == nil {
		t.Fatal("reused path accepted old finish")
	}
}

func TestReviewExitedRetiresImmediatelyWhenEligible(t *testing.T) {
	work, r, _ := finishedReview(t)
	// A review fetch may populate only FETCH_HEAD, with no tracking ref for the PR.
	ref := mustGit(t, work, "for-each-ref", "--format=%(refname)", "--points-at", r.Head, "refs/remotes/origin/")
	mustGit(t, work, "update-ref", "-d", ref)
	mustGit(t, originBare(t, work), "update-ref", "refs/pull/42/head", r.Head)
	rc, out, diag := runCapBoth(t, []string{"review-exited", r.Home, r.Attempt})
	if rc != 0 || !strings.Contains(out, "review checkout retired") {
		t.Fatalf("%d %s %s", rc, out, diag)
	}
	gone(t, r.Home, diag)
}

func TestReviewRetirementRetriesFailedGitRemoval(t *testing.T) {
	_, r, admin := finishedReview(t)
	original := execCommand
	t.Cleanup(func() { execCommand = original })
	attempts := 0
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 4 && slices.Equal(args[len(args)-4:], []string{"worktree", "remove", "--", r.Home}) {
			attempts++
			if attempts == 1 {
				return exec.Command("git", "not-a-command")
			}
		}
		return original(name, args...)
	}
	rc, out, diag := runCapBoth(t, []string{"review-exited", r.Home, r.Attempt})
	if rc != 0 || !strings.Contains(out, "queued") || attempts != 1 {
		t.Fatalf("failed removal: %d %s %s", rc, out, diag)
	}
	if _, err := os.Stat(filepath.Join(admin, reviewhome.ExitFile)); err != nil {
		t.Fatal(err)
	}
	plan := pruneOK(t, "--no-singleton")
	if attempts != 2 || !strings.Contains(plan, "review_retired=1 review_backlog=0") {
		t.Fatal(plan)
	}
	gone(t, r.Home, plan)
	rc, out, diag = runCapBoth(t, []string{"review-exited", r.Home, r.Attempt})
	if rc != 0 || !strings.Contains(out, "already absent") {
		t.Fatalf("duplicate exit after removal: %d %s %s", rc, out, diag)
	}
}
