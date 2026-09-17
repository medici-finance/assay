---
brief: assay:assay:desk-supervision:12
title: Retire the staged-copy hand-landing once the workflow App PR path is proven
why: >-
  The staged-copy directories exist ONLY because no identity could write .github/workflows/*; once
  the workflow App authors a workflow-only PR, the staging areas are a second, drifting source of
  truth with no reason to exist. Leaving them in place after the App path works re-invites exactly
  the stall and drift the model removes — a future author would stage a copy out of habit and a
  human would forget to land it. This brief removes the workaround, but only after proving the
  replacement carries a real change end to end, so nothing regresses during the cutover.
wave: 2
depends: ["desk-supervision/11"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  Removing the staged-copy path removes the human-in-the-loop copy step that is today's last manual
  control on what lands in CI. The human is confirming the workflow App PR path is proven to carry a
  workflow change end to end BEFORE the fallback is deleted, so the removal never leaves a window
  where a workflow change can land by neither path — and that no currently-stale staged copy (#1187)
  is landed verbatim in a way that reverts an intervening fix.
design: DR-workflow-app-landing
decision-trigger: creation
decision-issue: 1247
issues: [1187, 722]
schema: brief-v2
version: 1
id: c861ae82-1fee-4d34-94f2-ccfc32a126cc
authored: 2026-09-16 by desk-supervision authoring session
exec-tier: strong
exec-tier-why: >-
  (b) correctness is the cross-artifact argument that the replacement path is proven before the
  fallback is removed; (c) it is supply-chain plumbing where removing a control prematurely opens a
  gap a happy-path test would not show.
sources:
  - "[DR-workflow-app-landing](../decisions/DR-workflow-app-landing.md) — the design record; its `accepted:` list makes retirement conditional on the App path being proven end to end."
  - "desk-supervision/11 — the workflow-only-PR contract and verb whose proof is this brief's precondition."
  - "tools/ci-load/activation/README.md and its five staged *.yml — the staging area; #1187 records three of the five stale enough that landing them verbatim would revert #548/#999/#579+#586."
  - "ci/staged-workflows/README.md — the second staging area with its own hand-copy runbook."
  - "#722 — a release consumes and deletes a brief's changelog fragment, reddening --lint on main: the same by-hand workflow/release-automation seam; the retirement must not re-strand fragment handling."
  - "freshness-checked 2026-09-16 @ e9fa19d3"
consumers:
  - "tools/ci-load/activation/ (staging area to remove/reduce): follow-up desk-supervision/12 (this brief; flips to fixed-here when the implementation removes or reduces it to a pointer)"
  - "ci/staged-workflows/ (staging area to remove/reduce): follow-up desk-supervision/12 (this brief; flips to fixed-here when the implementation removes or reduces it to a pointer)"
  - "the PR-discipline / adoption docs that instruct the hand-copy step: follow-up desk-supervision/12 (this brief; flips to fixed-here when the implementation rewrites them to the App path)"
  - "the release/changelog-fragment handling (#722): out-of-scope (the release-automation bug is separately tracked; this brief must not re-strand fragment handling but does not own its fix)"
---

# Brief 12 — Retire the staged-copy hand-landing

## Context

files:
- **remove or reduce to a pointer** `tools/ci-load/activation/` and `ci/staged-workflows/` — the
  two staging areas. If reduced rather than removed, the pointer file is named `POINTER.md` at
  each staging area's root (`tools/ci-load/activation/POINTER.md` (planned),
  `ci/staged-workflows/POINTER.md` (planned)) — pinned here so Verify rows 2/3 name a real deliverable
  rather than an unenumerated "a pointer". Any live-and-current staged delta is first carried
  through the workflow App PR path (desk-supervision/11), never landed verbatim (verbatim landing
  of a stale copy reverts fixes — #1187).
- **edit** the READMEs and `docs/adopting-assay.md` sections that instruct the `cp <staged>
  .github/workflows/` hand-copy step — rewrite them to the workflow-only-PR path.
- **verify (not edit here)** that the release/changelog-fragment handling (#722) is not re-stranded
  by the removal.
- **out of scope, named so it is not silently assumed** — the existing direct-to-default-branch
  promote route (today's human-dispatched job) is NOT retired by this brief; only the staging
  *directories* are. If closing that route is wanted, it is a branch-protection change (who may
  push to the default branch) tracked separately, not implied by this removal (DR
  "What is explicitly NOT decided here").

facts:
- precondition: the workflow App PR path (desk-supervision/11) is proven to carry a workflow change
  end to end. This brief does NOT proceed until that is true — it is why 12 `depends` on 11.
- #1187: three of five staged copies are stale; each must be RECONCILED (carried through the App
  path or discarded), never landed verbatim.
- single-point-of-failure: the ordering — remove the fallback only AFTER the replacement is proven.
  Layer behind it: Verify row 1 is a hard precondition gate (the App path carries a real change)
  that must be green before rows that remove anything; and the brief-11 guard (row 6) still fires,
  so even mid-cutover a mixed PR is refused.
- do-no-harm: `leaksweep-control.yml` / `leaksweep-pattern.yml` and any required status check are
  never in a staging area and are untouched by this brief.

## Human decision
<!-- decision-trigger: creation — options enumerable now; filed as the brief lands. Self-contained. -->
The staged-copy directories were a workaround for a limitation that the workflow App now removes:
no identity could write the CI workflow files, so changes were staged as copies and a human copied
them into place. With the workflow App authoring a workflow-only pull request instead, those
directories become a second, drifting copy of the CI files. A human must confirm the new path is
proven to carry a real change before the old workaround — and its manual copy step — is removed.

What is being decided:
1. Whether to remove (or reduce to a pointer) the staged-copy directories and their hand-copy
   runbooks now that the workflow App path exists.
2. Confirmation that the workflow App path has carried at least one real workflow change end to end
   before the fallback is removed.
3. How to reconcile the currently-stale staged copies: carry each through the App path, or discard
   the ones now superseded — never land a stale copy verbatim.

Options:
1. **Retire now** — the App path is proven; remove the staging areas and rewrite the runbooks.
   Reconcile the stale copies through the App path. (Recommended once desk-supervision/11 is green.)
2. **Reduce to pointers, keep as history** — replace each staging area with a two-line pointer to
   the contract, keeping git history but removing the live copies and the hand-copy instruction.
3. **Keep** — leave the staging areas in place. (Not recommended: re-invites the drift and stall.)

Recommendation: **option 1** (or **2** if history-in-place is preferred), only after
desk-supervision/11 is verified.

Default if no answer: none — blocks until answered. This brief's README row is `blocked`
(`lifecycle-v1.md` §2.0) via its own `depends: [desk-supervision/11]` chain back to 10's
unruled DR — it stays `blocked` until 11 reaches `done`.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. The deliverable is a draft PR
  opened by the desk verbs.
- Stop at `implemented` — you do not set verified/done.
- Do NOT remove any staging area until Verify row 1 (App path proven) is green; a stale staged copy
  is reconciled through the App path, never landed verbatim.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Confirm and record that the workflow App path (desk-supervision/11) has carried a real workflow
   change end to end (Verify row 1).
2. Remove or reduce to pointers `tools/ci-load/activation/` and `ci/staged-workflows/`.
3. Rewrite the READMEs and `docs/adopting-assay.md` sections that instruct the hand-copy step.
4. Reconcile the stale staged copies (#1187) through the App path or discard the superseded ones.
5. Confirm the release/changelog-fragment handling (#722) still works after the change.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | PRECONDITION + FLOW: `tools/workflowpr --dry-run` on a real prepared workflow change opens a workflow-only PR that carries the change end to end | exit 0; a workflow-only diff (proves the replacement path carries a change before anything is removed) | gate:human +flow +dereference |
| 2 | `test ! -d tools/ci-load/activation -o -f tools/ci-load/activation/POINTER.md` | exit 0 (removed, or reduced to a pointer) | check |
| 3 | `test ! -d ci/staged-workflows -o -f ci/staged-workflows/POINTER.md` | exit 0 | check |
| 4 | `grep -rEn -e 'cp tools/ci-load/activation' -e 'cp ci/staged-workflows' docs/ ci/ tools/ 2>/dev/null \| wc -l` | 0 (no hand-copy runbook remains) | check +dereference |
| 5 | `git diff $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- .github/workflows/leaksweep-control.yml .github/workflows/leaksweep-pattern.yml` | empty (security/required workflows untouched) — base spelled `refs/remotes/origin/main` in full: git resolves `refs/heads/` before `refs/remotes/`, so a checkout that ever acquired a local branch literally named `origin/main` would silently compare against the stale one | check +neighbour |
| 6 | `tools/workflowpr --check` on a fixture mixing a workflow file with a source file (the brief-11 guard) | exit non-zero (the guard still fires mid/post cutover) | check:ci +mutation |
| 7 | `statusgen --consumers --root . --brief desk-supervision/12` | exit 0 (consumers routing corroborated against the diff) | check:ci |
| 8 | `statusgen --root . --lint` | exit 0 (only pre-existing PROBLEMs — covers the #722 lint-red class) | check:ci |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Row 1 is the precondition proof;
     row 6 is the guard-still-fires proof. -->

## Review
Gate: human (from frontmatter — sensitive-data is yes). Human gate is MANDATORY: the reviewer
confirms the App path was proven (row 1) BEFORE any staging area was removed, that no stale copy
was landed verbatim, and that no security/required workflow was touched (row 5).
Reviewer records verdict + date in the stream README table.
