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
// Change state is not something a git-ref view can answer, so the decay reads it
// from the FORGE — and which forge that is decides the reader:
//
//   - GitHub (and an unclassifiable remote, where GitHub is the historical
//     fallback): `gh pr list`, the same dependency the `--issues` metrics path
//     already shells out to (ghIssueMetricLister).
//   - GitLab: the project's merge requests over REST v4 (claimdecay_gitlab.go).
//     `gh` does not exist there and a GitLab project has no pull requests to
//     list, so routing the read by forge is the only way the pass can run at all
//     — before this, a GitLab-hosted adopter's claims never decayed and the board
//     silently held their briefs forever.
//
// Both readers are injected as package vars so the whole decay is exercised
// offline in tests.
//
// When neither reader can look — no `gh`, no token, an API error — the pass is
// LOUD and says so in its own words: `could-not-check: claims not decayed`, with
// the reason, carried out of the decay as a string so `--lint` and the emitted
// board can wear it too. A stderr-only NOTICE while the artifact still reads
// clean is a TWO-state instrument (docs/three-state-instrument-rule.md), and that
// is precisely what hid the GitLab gap.

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

// decayReader picks the change-state reader for the forge behind root's `origin`
// remote, and names it for the could-not-check message.
//
// forgeUnknown (a self-hosted host naming neither forge, or no remote at all)
// keeps the historical `gh` attempt: "could not tell" is not "confirmed not
// GitHub", and a GitHub Enterprise host that names neither word still answers
// `gh`. Its failure then reports as an ordinary could-not-check with the reason,
// which is the honest answer for a forge we could not identify.
func decayReader(root string) (read func(string) (map[string]bool, error), what string) {
	if detectForge(root) == forgeGitLab {
		return listMergedClosedBranchesGitLab, "merge-request state over the GitLab REST v4 API"
	}
	return listMergedClosedBranches, "PR state through `gh pr list`"
}

// decayDeadClaims removes, from an open-branch list, the branches whose PR or
// merge request has already merged or closed — the corpses that
// `git ls-remote --heads` still reports and that would otherwise keep consuming
// their stream's dispatch cap.
//
// It returns the surviving branches AND the could-not-check reason: empty when
// the decay actually ran, and otherwise the reason the change-state read failed.
// The caller carries that reason onto ClaimSource so `--lint` and the emitted
// board wear it (claims.go). A stderr NOTICE alone left the artifact reading
// clean — a two-state instrument — and that is how a forge on which the pass
// could NEVER run went six days unnoticed.
//
// Fail direction (load-bearing, unchanged): the decay can only SHRINK the claim
// set, so a failed read falls back to the full open-branch set — exactly the
// pre-decay behaviour. That is the safe direction: the worst case is the old
// over-holding (a corpse still counts), never a NEW under-holding that drops a
// live open-change claim and lets two sessions pick the same brief.
func decayDeadClaims(root string, branches []string) ([]string, string) {
	if len(branches) == 0 {
		return branches, ""
	}
	read, what := decayReader(root)
	dead, err := read(root)
	if err != nil {
		reason := fmt.Sprintf("%s could not be read: %v", what, err)
		fmt.Fprintf(os.Stderr, "could-not-check: claims not decayed — %s. "+
			"Open branches of already merged/closed changes are still counted as claims, so they may be consuming their "+
			"stream's dispatch cap and silently holding its briefs back; regenerate once the forge read is available to release them.\n",
			reason)
		return branches, reason
	}
	if len(dead) == 0 {
		return branches, ""
	}
	live := make([]string, 0, len(branches))
	for _, b := range branches {
		if dead[b] {
			continue
		}
		live = append(live, b)
	}
	return live, ""
}
