package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Dead-claim decay: statusgen builds its "stream/NN" claim set from OPEN origin
// branch heads (resolveClaims → listRemoteBranches → `git ls-remote --heads`).
// A branch is treated as an in-flight claim that subtracts from its stream's
// dispatch cap (perStreamCap). But `git ls-remote` is a pure ref view: it still
// reports the head of a branch whose PR already MERGED or CLOSED but that was
// never deleted. That corpse keeps consuming the cap forever, and a stream whose
// whole budget is spent on corpses shows ZERO board rows while its real backlog
// is silently held (HeldByStreamCap) — the class of silent-suppression bug this
// generator exists to prevent.
//
// The fix decays those dead claims at the data-collection boundary: before a
// branch name becomes a claim, drop it if its PR is merged or closed. A branch
// with an OPEN PR — or no PR yet (a worker that has pushed but not opened one) —
// is a live in-flight claim and is KEPT, so genuine parallelism is still
// serialized.
//
// PR state is not something a git-ref view can answer, so this reaches GitHub
// via `gh` — the same dependency the `--issues` metrics path already shells out
// to (ghIssueMetricLister). It is injected as a package var so the whole decay is
// exercised offline in tests.

// listMergedClosedBranches returns the set of head branch names whose PR is
// MERGED or CLOSED, for the repo rooted at `root`. It lists PRs in every state
// and keeps only the dead ones — an OPEN PR is deliberately absent from the set,
// because its branch is a live claim that must still be honoured. A `gh` failure
// is returned as an error; decayDeadClaims degrades to "decay nothing" so the
// board is never WORSE than the pre-decay superset (and never drops a live claim
// it could not verify, which would risk two sessions converging on one brief).
//
// A package-level var so tests substitute a fake lister without a network call,
// exactly as listRemoteBranches / ghIssueMetricLister are stubbed.
var listMergedClosedBranches = func(root string) (map[string]bool, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--state", "all", "--limit", "1000",
		"--json", "headRefName,state")
	// gh resolves the repo from its working directory; point it at the board's
	// root (the same repo listRemoteBranches read), mirroring `git -C root`.
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh pr list: %v %s", err, detail)
	}
	var raw []struct {
		HeadRefName string `json:"headRefName"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}
	dead := map[string]bool{}
	for _, pr := range raw {
		name := strings.TrimSpace(pr.HeadRefName)
		if name == "" {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(pr.State)) {
		case "MERGED", "CLOSED":
			dead[name] = true
		}
	}
	return dead, nil
}

// decayDeadClaims removes, from an open-branch list, the branches whose PR has
// already merged or closed — the corpses that `git ls-remote --heads` still
// reports and that would otherwise keep consuming their stream's dispatch cap.
//
// Fail direction (load-bearing): the decay can only SHRINK the claim set, so a
// failed PR-state read falls back to the full open-branch set — exactly the
// pre-decay behaviour. That is the safe direction: the worst case is the old
// over-holding (a corpse still counts), never a NEW under-holding that drops a
// live open-PR claim and lets two sessions pick the same brief. The failure is
// announced on stderr rather than swallowed, so an operator knows corpses may
// still be inflating the caps and can regenerate once `gh` is reachable/authed.
func decayDeadClaims(root string, branches []string) []string {
	if len(branches) == 0 {
		return branches
	}
	// Forge gate (#349): the decay reads PR state through `gh`, a
	// GitHub-only client. On a remote that is DEFINITIVELY not GitHub (a GitLab
	// host), shelling `gh` would fail every single run — that is not
	// "unavailable this run", it is a pass that does not apply to this forge. Say
	// so with a DISTINCT message and return the branch set unchanged, so a
	// three-state instrument never dresses a permanent not-applicable as a
	// transient could-not-check. forgeUnknown (self-hosted, no remote) is left to
	// the `gh` attempt below: "could not tell" is not "confirmed not GitHub".
	if detectForge(root) == forgeGitLab {
		fmt.Fprintf(os.Stderr, "NOTICE: dead-claim decay NOT APPLICABLE on this forge — the `origin` remote is a GitLab host, "+
			"and PR-state decay reads GitHub through `gh`. This pass does not run here and will not until statusgen reads merge-request state through the forge seam; "+
			"it is not \"unavailable this run\". Open branches of merged/closed merge requests are not decayed, so they may still consume their stream's dispatch cap.\n")
		return branches
	}
	dead, err := listMergedClosedBranches(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: dead-claim decay unavailable — %v. "+
			"Open branches of already merged/closed PRs may still be consuming their stream's dispatch cap; "+
			"regenerate with `gh` reachable and authenticated to release them.\n", err)
		return branches
	}
	if len(dead) == 0 {
		return branches
	}
	live := make([]string, 0, len(branches))
	for _, b := range branches {
		if dead[b] {
			continue
		}
		live = append(live, b)
	}
	return live
}
