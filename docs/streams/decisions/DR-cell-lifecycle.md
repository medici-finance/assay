---
id: DR-cell-lifecycle
date: "2026-11-20"
title: "Consolidate the cell role lifecycle judgments (process identity, owner generation, settle outcome, plan class) into one pure model in the existing cellcadence owner, after the independent owner and process controls pass"
consequence: major  # the transcriber's classification, not part of the ruling
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/7066#issuecomment-7054036135"
alternatives:
  - "Keep the current paths and request a narrower change (option 2 on the decision issue). Not chosen: the ruling selected option 1. Under option 2, up, reconcile, down and the interactive-owner settle would each keep their own judgment of whether a recorded owner is the running process and whether a checkpoint is that owner's, which is the duplication the brief's `why` names as the way a sibling path reintroduces the holder-loss class."
accepted:
  - "The lifecycle judgments for a cell role live in one place, `tools/desk/internal/cellcadence` (the owner the brief names); a wrong judgment there is wrong for every caller at once, which the class guard and the model's matrix tests are there to catch."
  - "The decision-only consolidation does not replace the independent controls at their own effect boundaries: the signal-time start re-read in `cellstop`, the stop writer's own owner-generation check under the lease, and the plan re-measure at apply all stay."
  - "The stop writer refuses a stop record for any generation other than the one that owns the unfinished checkpoint (a stale sequence or start, or an unnamed generation). That refusal is fail-closed."
---

This records the design that brief tool-refactoring/06
(`docs/streams/tool-refactoring/brief-06-model-local-cell-lifecycle-transitions-using-the-existing-cell-operation-contract.md`)
was built to, and the human ruling on its decision issue.

## The ruling

The human ruled option 1 on the decision issue, from their own account
([ruling comment](https://github.com/medici-finance/assay/issues/7066#issuecomment-7053556919), 2026-11-20):

> tool-refactoring/06: 1 — approve the decision-only consolidation in #7076, after the independent owner and process controls pass.

The two options put to the human were the brief's own:

1. Approve the decision-only consolidation after the independent owner and process controls pass.
2. Keep the current paths and request a narrower change.

## The decision

Option 1: the decision-only consolidation. The judgments of process identity (a pid and its
start time together, never a pid alone), owner generation, settle outcome and plan class for a
cell role are made once, in a pure model in the existing `cellcadence` owner, and the `cellctl`
adapters read the operating system and call it. The approval is conditional, in the ruling's own
words, on the independent owner and process controls passing.

## Not decided here

- Whether the delivered change meets that condition. That is the review gate and the brief's
  Verify table, run by an independent verifier; this record does not award acceptance.
- Anything the brief's Task leaves to other work: durable launch supervision, standing-client
  migration and new operator commands stay with their own briefs.
- Any change to a public schema, exit contract, gate policy or custody boundary. The brief's
  interface contract rules those out of this refactor.
