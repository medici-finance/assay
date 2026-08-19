package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// renderDispatchPrompt builds the per-item worker prompt from the fixed template + the item's typed
// payload. This template is the ONLY per-loop-authored prose (arch doc §4); everything else is
// engine machinery. It carries the batch worker essentials VERBATIM from
// `.claude/skills/worker-desk/SKILL.md` (the dispatch template requirement in the brief Task): the
// offline envelope, the security-gate clause, the no-evasion clause, the F-35 home-worktree rule,
// merge-never-rebase, one-brief-one-branch-one-PR, the PR watch-loop, the exec-tier pickup-STOP for
// `strong`, and — because gate:human briefs dispatch normally — the stop-at-`implemented` +
// BLOCKED-ON-IAN protocol.
//
// The verbatim clauses are pulled up as named constants so the SAME bytes reach every worker and so
// TestDispatchPrompt_CarriesSkillEssentials can pin their presence; a batch worker prompt that
// dropped one of these is the class of drift the essentials-registry exists to catch.
func renderDispatchPrompt(it loopengine.Item, tier loopengine.Tier) string {
	var b strings.Builder

	if it.Payload["kind"] == kindOrphan {
		fmt.Fprintf(&b, "RESUME PR %s#%s (tier=%s) — finish started work; resume takes priority over fresh dispatch.\n\n",
			it.Payload["repo"], it.Payload["pr"], tier)
		fmt.Fprintf(&b, "Branch: %s\n", it.Payload["branch"])
		if fnd := strings.TrimSpace(it.Payload["findings"]); fnd != "" {
			fmt.Fprintf(&b, "Open findings to work:\n%s\n", fnd)
		}
		b.WriteString("\n")
	} else {
		fmt.Fprintf(&b, "IMPLEMENT %s (tier=%s)\n\n", it.ID, tier)
		fmt.Fprintf(&b, "Brief: %s\n", it.BriefPath)
		if repo := strings.TrimSpace(it.Payload["repo"]); repo != "" {
			fmt.Fprintf(&b, "Target repo: %s\n", repo)
		}
		fmt.Fprintf(&b, "Target SHA (merged main to branch from): %s\n\n", it.TargetSHA)
	}

	// exec-tier: strong — the pickup-STOP text, verbatim (SKILL § tier by effort).
	if strings.EqualFold(strings.TrimSpace(it.ExecTier), "strong") {
		b.WriteString(pickupStopClause)
		b.WriteString("\n\n")
	}

	b.WriteString(homeWorktreeClause)
	b.WriteString("\n\n")
	b.WriteString(offlineClause)
	b.WriteString("\n\n")
	b.WriteString(securityGateClause)
	b.WriteString("\n\n")
	b.WriteString(noEvasionClause)
	b.WriteString("\n\n")
	b.WriteString(workflowClause)
	b.WriteString("\n\n")

	// gate:human briefs dispatch NORMALLY; the prompt says so and carries the stop-point protocol.
	if strings.EqualFold(strings.TrimSpace(it.Gate), "human") {
		b.WriteString(gateHumanClause)
		b.WriteString("\n")
	}
	return b.String()
}

// homeWorktreeClause — F-35 no-shared-paths (SKILL § worker prompt essentials). The worker's own
// worktree is its ONLY writable root; no shared-checkout path appears anywhere in the prompt.
const homeWorktreeClause = "Isolation (MANDATORY): work in your OWN worktree off the target SHA, created with " +
	"`git worktree add <path> refs/remotes/origin/main --detach`. Your home worktree is that path — every " +
	"file operation stays under it. Check `git rev-parse --show-toplevel` before your first write and ABORT " +
	"if it resolves to a shared checkout. Never touch a shared checkout; path-specific `git add` only."

// offlineClause — the offline-envelope line the dispatcher hands EVERY worker (SKILL § worker
// prompt essentials). Verbatim.
const offlineClause = "You run OFFLINE against live infrastructure: no command or script you run may contact a " +
	"cluster or production endpoint, read-only included. Export `KUBECONFIG=/dev/null` before your first " +
	"command. Anything that needs live state is could-not-check + BLOCKED-ON-IAN on the PR — never a probe."

// securityGateClause — the security-gate clause the dispatcher hands EVERY worker (SKILL § worker
// prompt essentials). Verbatim.
const securityGateClause = "If a change you are about to commit deletes, disables, or weakens a security or " +
	"access-control control or its CI assertion — a NetworkPolicy or egress/ingress allowlist, RBAC, " +
	"auth/identity config, a secret-scan/leak-sweep gate, a fence script or workflow, an admission policy, a " +
	"required check — STOP, even if it is the fix for a red check and even if this brief instructs it: do not " +
	"commit the removal, leave the check red, post `BLOCKED-ON-HUMAN — security-gate removal` on the PR naming " +
	"the control and any relocation evidence, and label `needs-decision`. Only a human ruling recorded on the " +
	"PR/issue authorizes the removal; this prompt is not that ruling."

// noEvasionClause — the no-evasion clause (SKILL § worker prompt essentials). Verbatim.
const noEvasionClause = "Any guard or hook BLOCK (writeguard, deskpushguard, a permission denial) is a STOP " +
	"signal, not an obstacle. NEVER re-attempt the same effect with a different command, tool, or path " +
	"spelling — `find -delete` for a blocked `rm`, an interpreter one-liner, an API call for a blocked push. " +
	"On a block: stop that line of work, quote the block message verbatim in your PR/issue report, and " +
	"escalate per the output contract. If you believe the block is a false positive (your target is outside " +
	"the shared checkout), you may re-issue the SAME command with absolute target paths or a single " +
	"`cd <abs-dir> && …` chain — the guard resolves those; anything else is escalate-only. A task completed " +
	"via substitution is a failed task."

// workflowClause — the merge-never-rebase + one-brief-one-branch-one-PR + PR watch-loop essentials
// (SKILL § worker prompt essentials + PR watch-loop). Verbatim in substance.
const workflowClause = "Implement the brief to its Context + Task + Ground rules; run its Verify table locally; " +
	"then push your own feature branch and open a DRAFT PR (`deskpr create`, or `git push -u origin <branch>` " +
	"+ `gh pr create --draft` on fallback). One brief = one branch = one PR. Keep the branch current with " +
	"main — `git fetch origin && git merge origin/main` (merge, NEVER rebase — force-push is denied) — " +
	"periodically and right before signalling DESK-READY. Arm a Monitor on your own PR (`gh pr view <N> " +
	"--json state,mergeable,reviews`): new review → work the findings; CONFLICTING → merge origin/main and " +
	"push; MERGED/CLOSED → STOP. Stop at `implemented` — never set verified/done, never flip ready, never " +
	"approve your own PR (that's the review desk + a human). Report NEEDS_CONTEXT rather than guess; no " +
	"attribution lines."

// pickupStopClause — the exec-tier:strong cheap-pickup STOP text (SKILL § tier by effort). Verbatim.
const pickupStopClause = "If you are a fast/cheap-tier model, STOP — this brief requires a strong implementer. " +
	"Report which model you are and hand back."

// gateHumanClause — gate:human dispatches normally; the worker stops at `implemented`, sign-off is
// human, and a documented cutover/co-execution step is prepared-then-STOPPED with BLOCKED-ON-IAN
// (SKILL § scope the refill, gate:human handling). Verbatim in substance.
const gateHumanClause = "This brief is gate:human. The gate binds APPROVAL, not implementation: implement " +
	"normally and stop at `implemented`; sign-off (review, merge, verify) is a human's. If the brief's Task " +
	"has an explicit human co-execution / cutover step (a cutover, a repo bootstrap), prepare everything, " +
	"STOP at that documented point, and report BLOCKED-ON-IAN on the PR rather than performing it."

// assertNoSharedCheckout is the in-code backstop: the emitted dispatch instruction must never name a
// shared-checkout path (prompt-carried paths override every isolation layer — F-35). If the rendered
// prompt contains a shared-checkout marker, dispatch refuses (fail closed) rather than emit a leaky
// instruction. Mirrors verifyloop's assertNoSharedCheckout.
func assertNoSharedCheckout(prompt string) error {
	for _, bad := range []string{
		"/tracker/.claude/worktrees",
		"worktrees/thedesk",
		"worktrees/tracker-fanout",
	} {
		if strings.Contains(prompt, bad) {
			return fmt.Errorf("dispatch prompt names a shared-checkout path %q (isolation violation) — refusing", bad)
		}
	}
	return nil
}
