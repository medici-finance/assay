---
id: DR-desk-tools-39
date: "2026-11-03"
title: "Keep shared review observations and candidate-priority planning in the existing pure internal reviewflow owner"
consequence: major
decided-by: "human:<name>"
ruling: https://github.com/medici-finance/assay/issues/4521#issuecomment-6615602021
alternatives:
  - "Copy classification or priority policy into every command: the chosen design rejects duplicate semantic owners and a second ready predicate."
  - "Introduce a cache or service before eliminating repeated observation reads: the chosen design avoids a new validity/custody boundary without evidence it addresses the residual bottleneck."
  - "Treat a tools/desk internal package as cross-module importable: the Go boundary does not permit this; public serving extraction remains forge-neutral/39."
accepted:
  - "Option A keeps BuildObservation, PlanObservation and PlanCandidates in tools/desk/internal/reviewflow, with effects and credentialed collection in existing adapters."
  - "Observations are untrusted advisory interval data, not authority. Exact provider-host-repository/head/lane, requested/admitted population, completeness/freshness and applicability stay explicit; partial/stale/malformed inputs hold rather than becoming empty work."
  - "Current policy, evaluator revision, time and capacity are separate pure inputs; observation bytes cannot approve their own policy. Preserve domain holds/watch rows, unmapped candidates, uncapped coverage and legacy command semantics."
  - "Collection/CLI handoff remains desk-tools/40, preparation desk-tools/41, caller adoption desk-tools/43 and managed all-role binding graph-execution/27. The merged extraction and selection amendment do not establish their runtime adoption or fleet-wide savings."
---

**Ratified — 2026-11-03.** The new unedited human comment approves
DR-desk-tools-39 for desk-tools/39 as proposed on the existing decision issue:
https://github.com/medici-finance/assay/issues/4521#issuecomment-6615602021

The record and brief cite the same comment. Its author, unedited timestamps,
record/brief names and issue marker require native corroboration; the public
`decided-by` placeholder supplies no independent identity proof. Implementation
verification and final lifecycle acceptance remain pending, as the ruling states.

The original reviewflow extraction shipped in #3593. The candidate-plus-priority
amendment shipped in #4235 after planning #4218. This record retains the approved
bounded internal pure library and its existing semantic owners. It grants no
collection, caller adoption, dispatch/ready authority or operational activation.

After this metadata lands, the normal native statusgen reconcile path can assess
the implemented transition against the merged deliveries. Independent Verify,
consumer qualification and final human acceptance retain their existing gates.
