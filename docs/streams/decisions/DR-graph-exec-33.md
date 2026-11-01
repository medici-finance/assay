---
id: DR-graph-exec-33
date: "2026-11-01"
title: "Merge is a human decision node in the implementation pattern, and the risk-input mapping is derived, with the merge gate named for elevated and human"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4511#issuecomment-6560687132"
alternatives:
  - "Name the merge step for every class — ruled out: elevated then equals standard, and telling them apart needs a separate lane, which has no ruling."
accepted:
  - "This approves the declaration and the mapping only. Enforcement at dispatch stays pending on graph-execution/14, so until then nothing refuses an automated merge."
  - "The forge ruleset requires a pull request, an approving review and the required checks. It does not limit who presses merge."
  - "Schema, lint and instance validator guard the declared shape. A decision record states that a human act happened and does not prove who did it."
---

**Ruling recorded (2026-11-01): option 1.** The driver (`human:<name>`) ruled on the brief's
decision-gate [issue #4511](https://github.com/medici-finance/assay/issues/4511) under the driver's
own login. The
[first comment](https://github.com/medici-finance/assay/issues/4511#issuecomment-6505759648)
(2026-10-29T19:00:49Z) adopted the derived mapping. The
[recording comment](https://github.com/medici-finance/assay/issues/4511#issuecomment-6560687132)
(2026-11-01T16:29:16Z) reads `graph-execution/33: 1 — DR-graph-exec-33` and repeats that choice
naming this record, because the first comment named no record. This record transcribes what that
issue put for approval. It mints no new decision, and the human act is the driver's comment.

**Order of events.** This record was written after the implementation. The implementation merged
in #4509 on 2026-10-26, before either comment, on the brief's stated default of option 1, which the
driver then chose.

**The decision.** `docs/streams/graph-execution/brief-33-merge-decision-node-and-risk-input.md`:

- **Merge is a person's act.** The `merge` node of the implementation pattern is an effect-free
  `decision` node owned by `human`, for every risk class. No automated role can declare a merge.
- **The mapping.** Low: review. Standard: review, verify. Elevated: review, merge, verify. Human:
  review, merge, verify. Naming merge for elevated and human makes it a gate the admission check
  must see before the integration check runs.
- **Human stays separate from elevated outside the pattern.** Automated agents may only read on
  human-risk work, and its plan carries a human review gate.
