---
id: DR-desk-tools-41
date: "2026-11-16"
title: "Prepare one exact-source review context and share it between the review round and its packet"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4523#issuecomment-6923696525"
alternatives:
  - "Reuse all mutable review state as if head identity alone proved freshness: body edits, new reviews and check reruns invalidate that assumption."
  - "Read separate round and packet contexts: it keeps the duplicate work and lets the two disagree."
  - "Add a persistent global cache or a new service: it creates freshness and custody obligations beyond this bounded extraction."
accepted:
  - "Option A, as proposed on the decision issue: the existing deskdispatch preparation adapter over the shared review context and exact Git objects, keyed by provider-qualified repository, full head, used base, path and adapter/content version; the round and the packet consume one prepared context."
  - "--observation is advisory attribution, used only through live complete preparation. Fresh body, reviews, checks, permissions, claims and ready/publication inputs keep their own existing consequential admission checks."
  - "The prepared context grants no dispatch or ready authority. Reuse of immutable Git objects is scoped to the exact provider, head, base and interpretation; no unsupported cross-module import is promised, and budget and failure behaviour stay with their owners."
  - "Independent implementation verification and final lifecycle acceptance remain pending, exactly as the ruling states. This design approval supplies neither."
---

**Ratified — 2026-11-16.** The new unedited human comment approves
DR-desk-tools-41 for desk-tools/41 as proposed on the existing decision issue:
https://github.com/medici-finance/assay/issues/4523#issuecomment-6923696525

The record and brief cite the same comment. Its author, unedited timestamps,
record/brief names and issue marker require native corroboration; the public
`decided-by` placeholder supplies no independent identity proof. Implementation
verification and final lifecycle acceptance remain pending, as the ruling states.

The approved basis is the design, alternatives and limits stated on the decision
issue, which inspected the delivered preparation adapter from PR #4259. `major` is
the proposal's author classification, kept here as a reversible desk default: wrong
queue or admission behaviour can suppress required work or accept stale state, while
a bounded source correction and a fresh run remain possible. The ruling states no
severity, so this field is not attributed to the human.

This record grants no dispatch or ready authority, deployment, release, pin,
credential or live infrastructure access.
