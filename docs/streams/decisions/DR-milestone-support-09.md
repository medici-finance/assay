---
id: DR-milestone-support-09
date: "2026-11-23"
title: "Approve the bounded alias and ruling-subject bridge"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/7088#issuecomment-7103065020"
alternatives:
  - "Request corrections: hold implementation for a revised adapter, condition, preserved rule or consumer, such as binding legacy rulings to a revision or persisting the revision in handoff records. The driver accepted the reviewed bridge."
  - "Hold: leave /09 at its lifecycle state pending further evidence or ownership resolution. The driver authorized implementation."
accepted:
  - "Implement the revised bounded bridge in PR #7091 at 209ea9a125c747497a15be6a7d3daaeb91cd9cf1."
  - "Add one pure Subject value and SameSubject comparison to statusgen/workref, with handoff and verification edge adapters that leave the strict grammar and every record digest unchanged."
  - "Bind revision-bound human rulings through an explicit Ruling-Subject line and card digest in both statusgen and an in-process deskkit gate; keep pre-cutover unrevisioned rulings under the bounded compatibility rule."
---

The unedited ruling by the configured human driver says:

> milestone-support/09: 1 — accept DR-milestone-support-09 and the revised bounded bridge in PR #7091 at 209ea9a125c747497a15be6a7d3daaeb91cd9cf1.
> Ruling-Subject: assay:assay:milestone-support:b-09@1 card=none

The ruling's creation and update timestamps are both `2026-11-23T13:48:48Z`.
This transcription uses the implementing worker's pickup observation of the
ruling and the reviewed design. It creates no new approval.

Consequences: `statusgen/workref` owns the one subject value. The
context-handoff and verification-domain adapters pass the existing numeric
`fullId` to the unchanged legacy readers and keep the subject beside it; no
record field, digest or grammar changes. The statusgen decision-gate hold and
ruling-link lane, and the deskkit gate that `deskevidence` calls in-process,
each check the human comment's subject line and card digest independently
against the brief's resolved key and explicit version. A ruling with no subject
line keeps its historical logical-key meaning only when the forge reports it
was created before the fixed cutover; an unknown time holds.

The subject check grants no permission, claim, dispatch, status move or
acceptance. It can only refuse. Register gates (`R-N`) are unchanged, no
reader capability is advertised, and dispatch admission stays with /10. This
record supplies no independent verification or lifecycle completion.
