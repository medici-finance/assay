---
id: DR-graph-exec-16
date: "2026-11-03"
title: "Retain bounded Cell ownership, cumulative budgets and restore fencing at the existing coordination authority"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Use a restored database generation or lease as current effect authority: excluded by the existing R2 contract because restored state cannot independently fence an old writer."
  - "Use an external secret authority instead of the bounded operator-control volume (1B), manual-only reopen (2B), qualified conservative request bounds (3B), or report-only unsupported providers (4B): alternatives already recorded on decision #5111; the existing ruling selected 1A/2A/3A/4A with pacing rule 5 unchanged. No additional rejection rationale is invented."
accepted:
  - "Transcribe the existing design-only profile: one Cell execution/publisher group; operator-current state outside database backup/restore; role executors cannot read signing material and only the operator writes control state."
  - "A snapshot or backup never restores operator-control state as current. Reconstruction, rollback and transfer rotate identity and key before a match counts. Reopen requires confirmed group stop, old capability invalidation, independent credential restriction/revocation and reconciliation."
  - "Unknown brakes, usage, reservations or effect outcomes hold. Budgets remain separated by provider/account/unit/window and charge preparation, failed, fallback and verification attempts to original work. Keep the existing pacing distinction and reset restrictions."
  - "The delivered library is store-neutral. Real PostgreSQL rollback belongs to graph-execution/19, launch enforcement to graph-execution/26, and concrete custody/provider/credential/consumer qualification and activation remain separate. No exactly-once claim or live authority is added."
---

**PROPOSED RECORD BINDING — no record-specific human ruling is claimed.**
The `decided-by` placeholder is required register shape, not a new approval stamp.
This formalization preserves the already recorded bounded design direction; it does
not ask for another choice of architecture or expand the implementation scope.

The existing design-only ruling and its two custody restrictions remain authoritative:
https://github.com/medici-finance/assay/issues/5111#issuecomment-6561259515

That comment names graph-execution/16 and its selected dimensions, but does not name
DR-graph-exec-16. Under registers-v1 section 7.5 it therefore cannot be placed in this
record's `ruling:` field as corroboration. A new unedited mapped-human comment on
that decision issue must bind this record and brief. Only then should a follow-up
add the ruling link, bind `design:` and use the normal lifecycle reconciliation path.
The existing issue marker already names this brief; its existing decision is retained.

The design fits the Cell incarnation/checkpoint and recovery contracts. Merged
schema/test preparation #5113 did not deliver the brief. Subsequent implementation
#5386 added `CellAuthority`, `CellJournal`, reservation/settlement/recovery ports and
separate provider-side checking through the existing drainloop owner.

This record supplies neither an execution result nor final acceptance. Every original
Verify row remains owed to verify-desk; provider/profile qualification, credentials,
activation and production backend work stay with their declared owners.
