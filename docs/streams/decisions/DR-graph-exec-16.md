---
id: DR-graph-exec-16
date: "2026-11-03"
title: "Retain bounded Cell ownership, cumulative budgets and restore fencing at the existing coordination authority"
consequence: critical
decided-by: "human:<name>"
ruling: https://github.com/medici-finance/assay/issues/5111#issuecomment-6612666889
alternatives:
  - "Use a restored database generation or lease as current effect authority: excluded by the existing R2 contract because restored state cannot independently fence an old writer."
  - "Use an external secret authority instead of the bounded operator-control volume (1B), manual-only reopen (2B), qualified conservative request bounds (3B), or report-only unsupported providers (4B): alternatives already recorded on decision #5111; the existing ruling selected 1A/2A/3A/4A with pacing rule 5 unchanged. No additional rejection rationale is invented."
accepted:
  - "Transcribe the existing design-only profile: one Cell execution/publisher group; operator-current state outside database backup/restore; role executors cannot read signing material and only the operator writes control state."
  - "A snapshot or backup never restores operator-control state as current. Reconstruction, rollback and transfer rotate identity and key before a match counts. Reopen requires confirmed group stop, old capability invalidation, independent credential restriction/revocation and reconciliation."
  - "Unknown brakes, usage, reservations or effect outcomes hold. Budgets remain separated by provider/account/unit/window and charge preparation, failed, fallback and verification attempts to original work. Keep the existing pacing distinction and reset restrictions."
  - "The delivered library is store-neutral. Real PostgreSQL rollback belongs to graph-execution/19, launch enforcement to graph-execution/26, and concrete custody/provider/credential/consumer qualification and activation remain separate. No exactly-once claim or live authority is added."
---

**Record binding ratified — 2026-11-03.** The new unedited human comment names
DR-graph-exec-16 and graph-execution/16 and preserves the existing bounded
1A/2A/3A/4A profile, pacing rule 5 and both 1A custody restrictions:
https://github.com/medici-finance/assay/issues/5111#issuecomment-6612666889

The original design-only ruling remains the provenance for those policy choices:
https://github.com/medici-finance/assay/issues/5111#issuecomment-6561259515

The new `ruling:` binds this formal record; the earlier comment did not name a DR
identifier. The public `decided-by` placeholder does not replace corroboration of
the actual mapped-human author, unedited timestamps and issue marker. This binding
supplies no independent verification, final acceptance, credentials or activation.
The brief now cites this record so the existing lifecycle tools can assess the
merged implementation under the unchanged gates.

The design fits the Cell incarnation/checkpoint and recovery contracts. Merged
schema/test preparation #5113 did not deliver the brief. Subsequent implementation
#5386 added `CellAuthority`, `CellJournal`, reservation/settlement/recovery ports and
separate provider-side checking through the existing drainloop owner.

This record supplies neither an execution result nor final acceptance. Every original
Verify row remains owed to verify-desk; provider/profile qualification, credentials,
activation and production backend work stay with their declared owners.
