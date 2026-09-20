package main

// verifierdetached_test.go — FAIL-FIRST coverage for the verifier kit's detached worktree and
// the worker kit's refusal-text split (#1309 item 6).
//
// THE DEFECT. `deskdispatch --kit verifier` cut its worktree on the brief's own `feat/<id>`
// branch, so a delivered brief whose feature branch still sat in a stale worker worktree
// refused the verifier with "already delivered or in progress" — 8 of 12 dispatchable rows on
// one root were blocked that way by three-day-old worktrees. A verifier runs against merged
// origin/main and needs no feature branch. And the worker-kit refusal said the same sentence
// for two different facts deskwt distinguishes: the branch CHECKED OUT in a live worktree vs
// the branch merely existing (delivered).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The verifier's worktree-create step is `deskwt add verify-<item>-<sess> --detach --base
// refs/remotes/origin/main`: its own name, no --branch, never the brief's feat branch.
func TestVerifierKitCutsDetachedWorktreeUnderItsOwnName(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/verifier-home")

	rc := run([]string{"example-stream/07", "--root", root, "--kit", "verifier",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("verifier dispatch rc = %d, want 0", rc)
	}
	if !s.ran("deskwt add verify-example-stream-07-deskdispatch-test --detach --base refs/remotes/origin/main") {
		t.Errorf("the verifier worktree was not cut detached under its own name; deskwt calls: %v", s.deskwtCalls())
	}
	for _, c := range s.deskwtCalls() {
		if strings.Contains(c, "--branch") || strings.Contains(c, "feat/") {
			t.Errorf("a verifier dispatch named a feature branch: %s", c)
		}
	}
}

// The worker kit is byte-for-byte unchanged: tracking branch `feat/<item>`, item-named dir.
func TestWorkerKitStillCutsTrackingBranchWorktree(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	rc := run([]string{"example-stream/07", "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("worker dispatch rc = %d, want 0", rc)
	}
	if !s.ran("deskwt add example-stream-07-deskdispatch-test --branch feat/example-stream-07 --base refs/remotes/origin/main") {
		t.Errorf("the worker worktree step changed; deskwt calls: %v", s.deskwtCalls())
	}
}

// --branch contradicts the verifier shape and is refused pre-claim (zero processes run).
func TestVerifierKitRefusesExplicitBranch(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	rc, stderr := runCapturingStderr(t, []string{"example-stream/07", "--root", root, "--repo", allowedRepo,
		"--kit", "verifier", "--branch", "feat/example-stream-07", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "--branch is not accepted with --kit verifier") {
		t.Fatalf("rc = %d (want 5), stderr:\n%s", rc, stderr)
	}
	if len(s.calls) != 0 {
		t.Fatalf("a pre-claim refusal must run no process; ran %v", s.calls)
	}
}

// A stale worker worktree holding feat/<id> cannot refuse the verifier: the dry-run plan for a
// verifier names no branch at all.
func TestVerifierDryRunPlanNamesNoBranch(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	promptFile := filepath.Join(t.TempDir(), "p.md")
	out := captureStdoutOf(t, func() {
		if rc := run([]string{"example-stream/07", "--root", root, "--repo", allowedRepo, "--kit", "verifier",
			"--dry-run", "--prompt-file", promptFile}); rc != deskkit.ExitOK {
			t.Errorf("dry run rc = %d", rc)
		}
	})
	if !strings.Contains(out, "branch=(detached off origin/main — verifier touches no branch)") {
		t.Fatalf("verifier dry-run plan still names a branch:\n%s", out)
	}
	body, _ := os.ReadFile(promptFile)
	if strings.Contains(string(body), "feat/example-stream-07") {
		t.Fatalf("verifier prompt names the brief's feature branch:\n%s", body)
	}
}

// The worker-kit refusal text is split on what deskwt said: CHECKED OUT (active — find the
// worktree, an agent may be on it) vs merely exists (delivered — look for the PR). The old
// single sentence said "already delivered or in progress" for both.
func TestWorkerHintSplitsCheckedOutFromDelivered(t *testing.T) {
	active := worktreeCreateHint("worker", "feat/item-1",
		"refused: branch feat/item-1 already exists and is CHECKED OUT in the worktree /private/tmp/tracker-item-1 — that worktree owns it")
	if !strings.Contains(active, "CHECKED OUT and ACTIVE") || !strings.Contains(active, "do not look for a PR first") ||
		strings.Contains(active, "already delivered or in progress") {
		t.Errorf("checked-out hint does not say the branch is active in another worktree:\n%s", active)
	}
	delivered := worktreeCreateHint("worker", "feat/item-1",
		"refused: branch feat/item-1 already exists, is checked out in NO worktree, and carries 2 commit(s) not in origin/main")
	if !strings.Contains(delivered, "DELIVERED") || !strings.Contains(delivered, "merged or open PR") ||
		strings.Contains(delivered, "ACTIVE") {
		t.Errorf("exists-only hint does not say the brief is delivered:\n%s", delivered)
	}
	verifier := worktreeCreateHint("verifier", "", "refused: target already exists (never clobbered): /private/tmp/tracker-verify-item-1")
	if !strings.Contains(verifier, "not a branch collision") || strings.Contains(verifier, "merged or open PR") {
		t.Errorf("verifier hint sends a verifier to look for a PR:\n%s", verifier)
	}
	if active == delivered {
		t.Error("the two worker refusals render identically — the split is the fix")
	}
}

// The live step wires the split: a CHECKED OUT refusal from deskwt reaches the worker's step
// report as the active-worktree hint.
func TestWorktreeCreateReportCarriesTheCheckedOutHint(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: deskwtStderr, code: deskkit.ExitRefused},
	}
	_, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	report := stepReport(t, stderr)
	if !strings.Contains(report, "CHECKED OUT and ACTIVE in another worktree") {
		t.Errorf("the step report does not carry the checked-out hint:\n%s", report)
	}
	if strings.Contains(report, "already delivered or in progress") {
		t.Errorf("the step report still carries the undifferentiated hint:\n%s", report)
	}
}

func (s *stub) deskwtCalls() []string {
	var out []string
	for _, c := range s.calls {
		if len(c) > 0 && filepath.Base(c[0]) == "deskwt" {
			out = append(out, strings.Join(c, " "))
		}
	}
	return out
}

func captureStdoutOf(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, rerr := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if rerr != nil {
			break
		}
	}
	return string(buf)
}
