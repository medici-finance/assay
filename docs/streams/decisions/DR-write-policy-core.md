---
id: DR-write-policy-core
date: "2026-10-22"
title: "Accept bounded shared write-policy extraction and preserve hook behavior"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/3752#issuecomment-6352197356"
alternatives:
  - "Narrow or hold the extraction, retaining current behavior until missing boundary or compatibility evidence is supplied: the second offered option; the human selected bounded extraction instead."
accepted:
  - "Extract the reusable write-isolation evaluator into the existing pure deskcore module, with command hooks and managed adapters supplying explicit observations."
  - "Preserve existing legacy denial and exemption conditions and keep the legacy no-decision failure path explicit."
  - "Qualify the separate managed profile before activation; managed admission must not inherit the legacy fallback."
---

The human explicitly approved this named record on 2026-10-22:

https://github.com/medici-finance/assay/issues/3752#issuecomment-6352197356

This record transcribes the human's earlier option 1 decision for desktools-v2/66.
The original decision issue and accepted option define the bounded scope:

https://github.com/medici-finance/assay/issues/3752#issuecomment-6299695731

The issue offered approval of the bounded extraction, preserving legacy behavior
and qualifying the separate managed profile before activation, or narrowing/holding
until missing boundary or compatibility evidence was supplied. The human selected
`desktools-v2/66: 1`. This record adds the structured design reference that the
brief omitted; it records the human acts above and makes no independent
verification claim.

The design and compatibility contract are documented in
[write policy](../../write-policy.md) and its
[consumer handoff](../../write-policy-consumers.md). A wrong shared evaluator could
weaken write-isolation decisions across consumers, hence `consequence: major`.
Independent filesystem confinement, consumer activation, release qualification,
and merged-main verification retain their own owners and evidence obligations.
