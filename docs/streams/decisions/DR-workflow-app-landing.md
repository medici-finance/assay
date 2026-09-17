---
id: DR-workflow-app-landing
date: "2026-09-16"
title: "Land a workflow change as a single workflow-only PR the workflow App writes, instead of a human hand-copying a staged file into .github/workflows/"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Grant every desk App `workflows: write` directly — ruled out: an identity that can rewrite `.github/workflows/*` is a supply-chain surface, and there are several desk Apps (one per role). Spreading that grant across all of them multiplies the attack surface by the number of roles and makes the one power that can rewrite CI un-auditable — you can no longer point at a single actor and say `only this identity ever touches CI`. Least privilege wants that power concentrated in ONE narrowly-scoped identity, not diffused across the fleet. The concentration is the control; diffusing it destroys the control."
  - "Status quo — keep hand-landing: an author who lacks `workflows: write` stages a post-change copy of the workflow under a staging directory (`ci/staged-workflows/`, `tools/ci-load/activation/`), and a human copies it into `.github/workflows/` in a separate maintainer-credentialled commit — ruled out: the staged copy and the live file drift the moment either moves. A staged copy authored against one base goes stale when the live file changes underneath it (landing it verbatim silently REVERTS intervening fixes — #1187), and the copy step blocks a brief's `implemented -> verified` transition indefinitely when no human performs it (#1175 sat 9+ days BLOCKED-ON-HUMAN; #1185 is `help wanted - human workflows-copy still not landed`). The workaround's whole cost IS the drift and the stall."
  - "Let a human's ambient credential push the workflow file directly to the default branch (no PR) — ruled out: a workflow change then lands with no reviewable diff and no separate review gate, under a broad ambient identity rather than a narrowly-scoped one. It removes the stall but keeps the supply-chain exposure (a broad credential rewriting CI) and adds a new one (an unreviewed direct-to-main CI write). The reviewable artifact — a PR whose diff a reviewer reads before it lands — is exactly what this record refuses to give up."
accepted:
  - "The fleet keeps exactly ONE identity holding `workflows: write` — the workflow App. Its granted permission set is `contents: write` + `workflows: write` + `pull_requests: write` + `metadata: read`, subscribing to zero webhook events; `pull_requests: write` is required because the App's duty (below) is to OPEN — and, only if the merge-authority sub-decision selects identity-merges, to merge — the workflow-only PR, and that act is governed by the forge's `pull_requests: write` permission, not by `contents: write` or `workflows: write`. The sole-holder invariant binds `workflows: write` alone: every other desk App stays without it. The App explicitly WITHHOLDS `administration`, `actions`, `checks`/`statuses: write`, `members`, and repository secrets/variables — none of those is granted, so 'narrowly scoped' is a checkable list, not an adjective. The one power that can rewrite CI is concentrated in one auditable actor."
  - "A workflow change travels as a SINGLE workflow-only PR: its diff touches `.github/workflows/**` (and its own changelog fragment) and NOTHING else. The workflow App authors that PR; because the diff is workflow-only, no non-workflow file is coupled to the landing, so a workflow change can never again go stale waiting on an unrelated file, and a brief that PRESCRIBES a workflow change routes it to the workflow App's PR rather than staging a copy it cannot land."
  - "No ambient human credential authors CI. The workflow App is the author of record on the workflow-only PR; a human's role is the review/merge decision on that PR, not the act of writing `.github/workflows/*`. This preserves the reviewable-diff property the staged-copy model had while removing the human-copy step that stalls it."
  - "The staged-copy directories and their hand-copy runbooks are retired once the App path is proven end to end — a workflow change is authored directly against `.github/workflows/**` on the workflow App's branch, not staged elsewhere first. Until then they remain the fallback, so nothing regresses during the cutover."
  - "The mixed-PR guard (desk-supervision/11) is enforced OUTSIDE the workflow App's own write surface: it is registered as a REQUIRED status check in branch protection / a repository ruleset — repo settings, not a workflow file — with the workflow App absent from any bypass list. A workflow-only PR that edits the CI job invoking the guard's `--check` mode cannot silently disarm it, because the check's required-ness and the bypass list live in a surface the App cannot write (no `administration` grant, per the first entry above). Without this, the App's own write surface is the one adversary the two-layer design (construction + inspection) does not hold against."
  - "The credentialled path (opening, and — if selected — merging, the workflow-only PR) runs ONLY under a human-initiated invocation: an operator/desk run or an explicit `workflow_dispatch`, never a trigger an outside contributor can fire on a public repository (no `pull_request`, `pull_request_target`, `issue_comment`, or `workflow_run` trigger carries the workflow App's credential). The verb's `--check` classification mode is credential-free and makes no forge call — it is the only mode of the same binary reachable from ordinary CI on an arbitrary PR. Today's use (a human-dispatched promote job with no credential reachable from a PR-triggered run) is the property this record must not lose, not merely a description of current practice."
  - "Open sub-decision, NOT settled by this record: whether the workflow App also MERGES the workflow-only PR (landing it under `pull_requests: write`) or whether merge stays a human act as it is for every other PR. Both are compatible with the record; the merge-authority choice is deferred to the wiring brief's human gate. Choosing identity-merges is WIDER than today's status quo (today's promote route needs a maintainer credential — `ci/staged-workflows/README.md:1-8,16-19`): it is admissible only once the branch-protection placement above (bypass-list absence) is verified AND paired with a rule that any diff touching the guard's own CI job still requires a human merge, so the one identity able to rewrite the guard can never also be the one that waives review of that rewrite."
---

**Status:** proposed

**This record is NOT approved.** No design-approval ruling has been recorded. It is proposed
for the driver (`human:<name>`) to weigh; when a ruling is made, the decision-issue reference
and the outcome are recorded here, and `decided-by` above is confirmed.

**How an unruled record is distinguishable from a ruled one, mechanically, not just by this
sentence:** the design-approval gate (`lifecycle-v1.md` §4.4) only fires on the `todo →
in-progress` transition — it does not exclude an unapproved brief from the Next-up batch, so
citing this record alone does not hold the citing briefs. What holds them is that each of the
three briefs' README rows carries status `blocked` (`lifecycle-v1.md` §2.0 — "MUST NOT be
offered" in Next-up), not `todo`. That is the reader-checkable signal: a `todo` row citing this
record would be a defect; a `blocked` row is correct while this record is proposed. When a
ruling is recorded here and `decided-by` is confirmed, the driver flips the rows back to `todo`
(brief 10; briefs 11/12 stay `blocked` on their own `depends:` chain until 10 is `done`) — that
flip, not a sentence, is what "ruled" means to any consumer reading the board.

## The problem — seam D

Workflow files under `.github/workflows/**` are a supply-chain surface: an identity that can
write them can rewrite what every check asserts. So the desk Apps that do the day-to-day work
are deliberately NOT granted `workflows: write` — and GitHub hard-rejects any App push that
creates or updates a `.github/workflows/*` file without it. The consequence is that today a
workflow change cannot ride the same PR as the brief that needs it. Instead it is delivered as
a **staged copy** — a post-change copy of the workflow under a staging directory
(`ci/staged-workflows/`, `tools/ci-load/activation/`) — and a human copies that file into place
in a separate maintainer-credentialled commit.

That workaround has one failure mode, and it fires two ways:

- **It stalls.** The copy step is a human act with no owner and no deadline. #1175 sat 9+ days
  `BLOCKED-ON-HUMAN` waiting for the schedule/reconcile half of a workflow to be pushed; #1185
  is a standing `help wanted - human workflows-copy still not landed`, and it blocks the citing
  brief's `implemented -> verified` transition until someone performs the copy.
- **It drifts.** A staged copy is authored against one base; when the live file moves underneath
  it, the copy goes stale. #1187 records three of five staged copies stale enough that landing
  them verbatim would silently REVERT intervening fixes. The staged file and the live file are
  two truths that only a human remembers to reconcile.

The related release-automation edge (#722 — a release consumes and deletes a brief's changelog
fragment, reddening `--lint` on the default branch) is the same shape: workflow/release
automation that a narrowly-scoped, PR-authoring identity could carry cleanly is instead carried
by hand.

## The proposed rule change

1. **One identity holds `workflows: write`** — the **workflow App** — and only that identity.
   It is scoped to `contents: write` + `workflows: write` + `pull_requests: write` +
   `metadata: read`, zero events, with `administration`, `actions`,
   `checks`/`statuses: write`, `members`, and secrets/variables all withheld.
2. **A workflow change is a single workflow-only PR.** Its diff touches `.github/workflows/**`
   (plus its own changelog fragment) and nothing else. The workflow App authors it.
3. **No ambient human credential authors CI.** The human's role is the review/merge decision on
   the workflow-only PR, not the act of writing the workflow file.
4. **The mixed-PR guard lives outside the App's write surface** — a required status check in
   branch protection / a ruleset, with the App absent from any bypass list — and the
   credentialled path runs only under a human-initiated invocation, never a trigger an outside
   contributor can fire.

Because the PR is workflow-only, there is no other-file coupling: the workflow change can no
longer go stale waiting on an unrelated file, and the staged-copy directories can be retired
once the path is proven.

## What is explicitly NOT decided here

- **Merge authority** — whether the workflow App merges its own workflow-only PR or a human
  merges it (see the final `accepted:` entry). Deferred to the wiring brief's human gate.
- **Whether the workflow App is currently installed and wired** on this project with the
  PR-authoring capability. This record proposes the model; confirming or provisioning the App is
  the first brief's job, and if the App turns out unwired or missing, that brief ends in a
  provisioning ask to the driver (installing an App and setting its permissions are acts GitHub
  reserves for a signed-in human). See the open question below.
- **Whether the existing direct-to-default-branch promote route (today's human-dispatched job,
  `ci/staged-workflows/README.md`) is closed once this path is proven.** This record does not
  retire it and desk-supervision/12 retires only the staging directories, not that route. Its
  closure, if wanted, is a branch-protection change (who may push to the default branch) — the
  same surface the mixed-PR guard's required-check placement lives on above — and is left to the
  wiring brief's human gate to decide, not silently assumed.

## Open question — if the workflow App turns out unwired or missing

The methodology assumes ONE narrowly-scoped `workflows: write` identity exists. If, on
inspection, this project has no such App installed, or the installed App lacks the PR-authoring
capability (it can only push a promote job to the default branch, say, not open a PR), then the
model cannot be wired without a human provisioning step: create/adjust the App, set its
permissions to exactly `contents: write` + `workflows: write` + `pull_requests: write` +
`metadata: read`, install it on the repo, and record it in the App inventory. That provisioning ask is raised to the driver by
the wiring brief (`desk-supervision/10`); it is a human-only act and is called out there as a
`could-not-check` until performed.
