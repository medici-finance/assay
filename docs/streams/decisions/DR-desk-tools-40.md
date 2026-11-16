---
id: DR-desk-tools-40
date: "2026-11-16"
title: "Collect the review queue once and plan offline from that one observation through the shared pure planner"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4522#issuecomment-6923695940"
alternatives:
  - "Keep the separate actions and prs collections plus another independently collected planner input: it repeats the expensive reads and can join observations taken at different cutoffs."
  - "Let the snapshot adapter reconstruct its own classifier: it forks semantic policy away from the one resolved review classifier."
  - "Fall back to the legacy in-cycle collection after a partial snapshot: it obscures the failure and repeats the expensive read."
accepted:
  - "Option A, as proposed on the decision issue: deskboard review-queue collects once through the existing typed adapters and resolved classifier facts, preserving source coverage, exact identities, independent lanes and the complete population, and writes the observation with an explicit cutoff and generation."
  - "reviewloop plan --snapshot decodes and validates that observation, takes policy, scope, time and capacity as an explicit selection invocation, and calls the shared pure planner with no provider access. Consequential dispatch and publication still re-read at their existing effect owner."
  - "This is source-qualified collection and planning, not permission to activate callers or a claim of a live fleet saving. An observation can be stale immediately; unknown schema, changed scope or policy, and an incomplete population remain explicit holds."
  - "Independent implementation verification and final lifecycle acceptance remain pending, exactly as the ruling states. This design approval supplies neither."
---

**Ratified — 2026-11-16.** The new unedited human comment approves
DR-desk-tools-40 for desk-tools/40 as proposed on the existing decision issue:
https://github.com/medici-finance/assay/issues/4522#issuecomment-6923695940

The record and brief cite the same comment. Its author, unedited timestamps,
record/brief names and issue marker require native corroboration; the public
`decided-by` placeholder supplies no independent identity proof. Implementation
verification and final lifecycle acceptance remain pending, as the ruling states.

The approved basis is the design, alternatives and limits stated on the decision
issue, which inspected the delivered collection and snapshot planner from PR #4248.
`major` is the proposal's author classification, kept here as a reversible desk
default: wrong queue or admission behaviour can suppress required work or accept
stale state, while a bounded source correction and a fresh run remain possible. The
ruling states no severity, so this field is not attributed to the human.

This record grants no caller activation, dispatch or ready authority, deployment,
release, pin, credential or live infrastructure access.
