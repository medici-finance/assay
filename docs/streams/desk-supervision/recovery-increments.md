# Clear blocked work through the existing desk loops

Authoring snapshot: 2026-09-20, source main `3db05fb44`. This is a plan; none of the
six new briefs claims implementation or runtime activation. Recheck open work at pickup.

## Existing work to finish or reuse

| Existing work | State at inspection | Relationship to this plan |
|---|---|---|
| [Issue 1309](https://github.com/medici-finance/assay/issues/1309), [PR 1374](https://github.com/medici-finance/assay/pull/1374) | Open, changes requested, conflicting | Immediate verification repair: multi-root coverage, Evidence-only dispatch, stuck-flip classification, row-level lane selection, fail-closed brief lookup, detached worktrees and scoped credentials. Finish this implementation; do not author a duplicate. |
| [desk-supervision/05](brief-05-per-class-caps.md) | Done | Existing reservation settings and planner display. The test deliberately keeps fresh rows eligible: enforcement is missing. Brief 18 supplies it. |
| desk-tools/16 | Done | Equivalent Evidence writes are deduplicated after execution. Brief 16 prevents repeated unchanged execution. |
| Canonical pr-review-desk skill | Existing rule | Three complete verdict/fix/re-review rounds per finding class, then the existing human arbitration lane. Brief 19 persists and applies that rule. |
| graph-execution/03–06 | Existing planned work | Broader execution coverage, receipts and experiments. Reuse their contracts where compatible; current-loop recovery does not require graph executor cutover. |

PR 1374's observed review has a concrete worker-lane bug: a leftover target directory is
misreported as a delivered branch. It also requests fail-first evidence for the seven repairs.
The immediate head is resolving that review plus merging current main, then obtaining current-head
approval and the existing merge authorization. An open PR with passing tests is not a deployed fix.

## New briefs and dependency waves

| Wave | Brief | State | Deliverable |
|---|---|---|---|
| 0 | [20 — first-pass review scope](brief-20-review-scope-and-first-pass.md) | todo, human decision at start | Inventory related claims on first review; bound blocking scope by impact/acceptance rather than proximity. |
| 0 | [16 — verification wake conditions](brief-16-verification-wake-conditions.md) | todo | Unchanged blocked inputs stay visible but do not consume another verifier run. |
| 0 | [19 — review finding continuity](brief-19-review-finding-continuity.md) | todo | Finding IDs, resolution evidence and the existing round count survive replacement agents. |
| 1 | [17 — verification repair obligations](brief-17-verification-repair-obligations.md) | todo | An actionable failed verification becomes durable worker work, retained through independent reverification. Depends on 16. |
| 1 | [21 — external-prerequisite reverification](brief-21-external-prerequisite-reverification.md) | todo, human decision at start | Clear a sole external blocker from fresh evidence without a synthetic commit; update board and ready gate together. Depends on 19. |
| 2 | [18 — repair admission](brief-18-repair-admission.md) | todo | Existing repair reservations are enforced before dispatch can admit fresh work. Depends on 17. |

Critical path: **16 → 17 → 18**. Review continuity and external wakeup form **19 → 21**. Brief 20 is the smaller first review-policy change; it can ship before 19, with the explicit human decision recorded at pickup. Serialize overlapping prompt edits rather than creating avoidable conflicts.
Brief 16 overlaps verifier files in PR 1374; finish or explicitly coordinate that patch first.
Brief 19 can start while that work finishes. This extends the existing plan/dispatch/land loops;
no new standalone scheduler, autonomous runner cutover or fleet-wide migration is prerequisite.

## Unclog now

1. Finish PR 1374, including its concrete review findings and current-main conflicts.
2. Release and pin the resulting tool/skill version through the existing release procedure.
   Verify the version actually used by the desk; source merged is not rollout complete.
3. Use the existing worker rework lane on a bounded cohort of five blocked PRs. Give each one
   an explicit next action and retain its agent responsibility through review. Use current
   reservation advice deliberately; do not claim it is enforced before brief 18 lands.
4. Reclassify verification after installing the repair. Run executable rows; route missing
   acceptance definitions or implementation defects back to a worker. An external prerequisite
   stays visible with its exact required act. Do not rerun unchanged blocked rows just to
   produce a new timestamp; any manual hold must name its wake condition and next check time.

The cohort is a proposed rollout size, not a measured capacity limit. Existing merge, reviewer,
verifier and human-decision authority remains in effect.

## Release each increment independently

Each brief gets its own implementation PR, negative-path tests and independent verification.
After merge: release, pin a pilot desk to the exact version, then observe the same five-item
cohort before widening. Record baseline and post-change counts over equal operating windows;
count distinct items and relevant input revisions, not repeated evidence rows.

- 16: unchanged failed inputs produce zero repeat dispatches; changing a declared prerequisite
  wakes the affected rows. Unknown reads remain visibly unknown.
- 17: each actionable failed verification has exactly one recoverable repair obligation.
  Restart preserves it; a merge alone does not resolve it.
- 19: existing finding IDs and round counts survive head and agent changes; a worker cannot
  resolve its own blocking finding; the existing cap produces one arbitration packet.
- 18: with fresh and repair work both runnable, repair capacity remains available under direct
  and concurrent dispatch. Exercise one complete fail → repair → review → merge → independent
  pass sequence, with restarts at each boundary.

Judge recovery by independently completed items, age of the oldest actionable repair,
review rounds per completed item, and unchanged verifier redispatches. A smaller visible queue
alone proves nothing: every held item must retain an owner/action/wake condition.

Rollback uses the previous released binary/skill pin and disables the opt-in admission policy.
Additive records remain readable or safely ignored by the older reader; never erase repair
obligations to make rollback look clean. Confirm this compatibility in each implementation PR.

## Issue 1387: why finding persistence alone is insufficient

[Issue 1387](https://github.com/medici-finance/assay/issues/1387) identifies first-pass search,
blocking proportionality and unchanged-head external prerequisites as distinct defects. Brief 20
addresses search/scope; 19 preserves class identity and counts; 21 fixes the external-condition
exception across actual tool gates. The proposed two-round cap in the issue remains a proposal;
this plan preserves the current three-round policy.

Do not adopt a blanket "untouched file cannot block" rule: an omitted explicit deliverable or
concrete safety consequence still matters. Do not treat every search hit as a blocker either.
A first-pass inventory separates both, so a worker receives one bounded repair request.
