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
  - "This is source-qualified collection and planning, not permission to activate callers or a claim of a live fleet saving. An observation can be stale immediately; unknown schema, changed scope and an incomplete population remain explicit holds. Changed policy is a hold at the planner step only through three mitigations, per the accepted policy-binding limit below: the planner's policy comparison checks the artifact against itself."
  - "Accepted limit, custody and sensitive content (decision issue limits amendment, https://github.com/medici-finance/assay/issues/4522#issuecomment-6422804401): the observation is an unsigned file whose provenance describes a read, not authentication, so file custody is its only integrity boundary and anyone able to replace a snapshot can fabricate internally consistent facts. The planner detects inconsistency, not fabrication. The file stores every configured repository's PR titles and full bodies. Artifacts live in caller-controlled private directories and are never accepted from untrusted authors; the second layer is the effect owners re-reading live state before acting, and no Verify row yet shows that re-read refusing a fabricated but consistent observation."
  - "Accepted limit, externally triggerable whole-cycle hold (same amendment): collection refuses the entire cycle if any configured repository holds a truncated or quarantined PR, and later cycles then fall to the legacy actions/prs recipe until the quarantine clears. An outside author on a public repository can therefore hold every qualified cycle and push the desk onto the legacy path. This is accepted as a stated cost and is a cross-cycle downgrade, distinct from the in-cycle fallback rejected above."
  - "Accepted limit, caller-supplied planner-step policy check (same amendment): scope is bound to the launcher's repository list, but the shipped caller passes the artifact's own inputs.policy as the observation policy, so the planner's policy comparison checks the artifact against itself and cannot fail; the offline planner cannot compute the current digest. Policy is held today only by the five-minute observation cutoff, deskboard throughput recomputing the current policy, and deskdispatch --observation treating the digest as untrusted and reading live state."
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
