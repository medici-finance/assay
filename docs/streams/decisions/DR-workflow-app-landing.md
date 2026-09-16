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
  - "The fleet keeps exactly ONE identity holding `workflows: write` — the workflow App — scoped to `contents: write` + `workflows: write` + `metadata: read` and nothing else, subscribing to zero webhook events. Every other desk App stays without workflow scope. The one power that can rewrite CI is concentrated in one auditable actor."
  - "A workflow change travels as a SINGLE workflow-only PR: its diff touches `.github/workflows/**` (and its own changelog fragment) and NOTHING else. The workflow App authors that PR; because the diff is workflow-only, no non-workflow file is coupled to the landing, so a workflow change can never again go stale waiting on an unrelated file, and a brief that PRESCRIBES a workflow change routes it to the workflow App's PR rather than staging a copy it cannot land."
  - "No ambient human credential authors CI. The workflow App is the author of record on the workflow-only PR; a human's role is the review/merge decision on that PR, not the act of writing `.github/workflows/*`. This preserves the reviewable-diff property the staged-copy model had while removing the human-copy step that stalls it."
  - "The staged-copy directories and their hand-copy runbooks are retired once the App path is proven end to end — a workflow change is authored directly against `.github/workflows/**` on the workflow App's branch, not staged elsewhere first. Until then they remain the fallback, so nothing regresses during the cutover."
  - "Open sub-decision, NOT settled by this record: whether the workflow App also MERGES the workflow-only PR (landing it under `contents: write`) or whether merge stays a human act as it is for every other PR. Both are compatible with the record; the merge-authority choice is deferred to the wiring brief's human gate."
---

**Status:** proposed

**This record is NOT approved.** No design-approval ruling has been recorded. It is proposed
for the driver (`human:<name>`) to weigh; when a ruling is made, the decision-issue reference
and the outcome are recorded here, and `decided-by` above is confirmed. Until then, the briefs
that cite this record (`design: DR-workflow-app-landing`) remain `todo` and MUST NOT advance to
`in-progress` — the design-approval gate (`lifecycle-v1.md` §4.4) is the control that holds them.

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
   It is scoped to `contents: write` + `workflows: write` + `metadata: read`, zero events.
2. **A workflow change is a single workflow-only PR.** Its diff touches `.github/workflows/**`
   (plus its own changelog fragment) and nothing else. The workflow App authors it.
3. **No ambient human credential authors CI.** The human's role is the review/merge decision on
   the workflow-only PR, not the act of writing the workflow file.

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

## Open question — if the workflow App turns out unwired or missing

The methodology assumes ONE narrowly-scoped `workflows: write` identity exists. If, on
inspection, this project has no such App installed, or the installed App lacks the PR-authoring
capability (it can only push a promote job to the default branch, say, not open a PR), then the
model cannot be wired without a human provisioning step: create/adjust the App, set its
permissions to exactly `contents: write` + `workflows: write` + `metadata: read`, install it on
the repo, and record it in the App inventory. That provisioning ask is raised to the driver by
the wiring brief (`desk-supervision/10`); it is a human-only act and is called out there as a
`could-not-check` until performed.
