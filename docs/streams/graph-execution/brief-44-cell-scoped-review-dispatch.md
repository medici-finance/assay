---
brief: assay:assay:graph-execution:44
title: Run one cell's review dispatch in a container
why: A cell should be able to notice a new PR head and dispatch an independent Assay review while its interactive review window is closed.
schema: brief-v2
version: 1
id: 64e66f47-f61c-4057-a66e-99506d844614
wave: 1
depends: []
unblocks: []
effort: M
gate: human
gate-why: The reviewer App can post a consequential verdict. The operator owns the choice of sole review launcher and promotion of its qualified image.
risk: {regulatory: 'no', customer: 'no', irreversible: 'no', sensitive-data: 'yes'}
issues: []
authored: 2026-11-28 by worker-desk
sources:
  - "freshness-checked 2026-11-28 @ 6bb9303da8 on main"
  - "docs/streams/graph-execution/brief-27-standing-desk-loop-admin.md"
  - "tools/desk/cmd/reviewloop/snapshot.go and plugins/assay/skills/pr-review-desk/SKILL.md"
exec-tier: strong
exec-tier-why: Process interruption, repository scope and reviewer identity affect whether a review can run twice or under the wrong authority.
domain: complicated
outcome: none
consumers:
  - "containers/pr-review-desk/Dockerfile: fixed-here (ships the optional service command)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here (bounded handoff for the selected subject)"
  - "cell workload and enrollment: out-of-scope for this repository (the adopter owns its pilot manifest)"
---

# Run one cell's review dispatch in a container

## Context

The unit of deployment is one review role in one cell. The first adopter enrolls one repository. The collector and planner already accept the whole cell population. The missing handoff is from a selected review proposal to a bounded desk session with the trusted checkout, observation, head and cell-owned credentials. A missed webhook is recoverable by polling. The worker desk retains ownership of code changes after findings.

files: `containers/pr-review-desk/Dockerfile`, `containers/pr-review-desk/cell-review.py`, `containers/pr-review-desk/test_cell_review.py`, `containers/entrypoint.sh`, `plugins/assay/skills/pr-review-desk/SKILL.md`, `changelog/cell-scoped-review-service.md`

single-point-of-failure: the retained launch ledger fences duplicate model dispatch after an interruption. The reviewer claim and head checks independently govern verdicts.

The adopter owns the workload, image pin, repository map and persistent volume. Graph-execution/27 owns the wider transfer of standing desk custody to loop-admin. This pilot exercises one role before that transfer.

## Ground rules

- One active owner for the cell's review role. A cutover drains the previous launcher first. A single retained volume carries the service lock, outstanding attempts and desk state. An empty replacement volume requires operator reconciliation.
- Existing role identities, claims, model attestation, lane requirements and exact-head readbacks govern every consequential action. The pilot owns only review dispatch. The human owns activation and merge.
- Exercise the implementation offline. Record a live independent verdict only when a qualified adopter supplies it.

## Task

1. Bind a reviewed cell profile and each permitted GitHub repository to a trusted checkout. Require exact agreement with the desk roster before collecting the queue.
2. Reconcile the complete observation with the existing selection planner. On selected review work, start one bounded Assay desk pass, hand it the observation and subject, and let the desk allocate independent reviewers. A waiting or ready-only queue consumes zero model calls.
3. Persist launch intent before spawning. On restart, hold an unresolved attempt at the same repository, PR and head. Admit a selected fix head after fresh collection. Provide an explicit recovery for a reconciled ambiguous launch.
4. Stage an adopter workload at zero replicas with an immutable image, a cell-local model pin, retained single-writer storage and isolated role credentials. Confirm the prior owner is drained before activation.

## Verify

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check +flow | host:desk-tools | `python3 containers/pr-review-desk/test_cell_review.py -v` | Real roster and planner route one first head and one corrected head. Restart after each starts no duplicate. Approval, waiting and an empty queue start no model. |
| 2 | check +flow | host:desk-tools | `python3 containers/pr-review-desk/test_cell_review.py ReviewServiceTest.test_two_repositories_with_same_pr_and_head -v` | Two enrolled repositories with identical PR number and SHA each receive one dispatch. |
| 3 | check +flow | host:desk-tools | `python3 containers/pr-review-desk/test_cell_review.py ReviewServiceTest.test_misbound_roster_busy_owner_or_failed_read ReviewServiceTest.test_uncertain_exit_and_reconciled_retry -v` | Roster mismatch stops before collection. A second owner, failed read or outstanding attempt starts no reviewer. Only the reconciled target retries. |
| 4 | check +dereference | tree | `statusgen --root . --consumers --brief graph-execution/44 --delivery "$ASSAY_DELIVERY_SHA" --base "$ASSAY_DELIVERY_BASE_SHA"` | After landing, corroborate each fixed-here consumer on the delivery and its first parent. Require zero disproved and zero unchecked. |

## Evidence

Independent verification pending.

## Review

At the live trial, record both full heads, independent reviewer identity and verdicts, the fix owner and the dispatch count across a restart. The operator accepts pilot custody and rollback after checking the image, cell role, retained state and independent review readbacks.
