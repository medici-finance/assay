---
id: DR-milestone-support-06
date: "2026-11-08"
title: "Approve the bounded milestone acceptance receipt adapter with trusted sources and authorized revocation"
consequence: major
decided-by: "human:ian"
ruling: "https://github.com/medici-finance/assay/issues/5705#issuecomment-6715755830"
alternatives:
  - "Request corrections: keep implementation held for a revised subject, authority, revocation or consumer rule. The driver accepted the corrected bounded proposal."
  - "Hold: defer implementation pending further evidence or ownership resolution. The driver instead authorized the reviewed design."
accepted:
  - "Implement the bounded design in PR #5711 at ffaaf82ff0ab3e09e779ca7fa34ef00ac91804ea."
  - "Contract and authority policy source commits must be reachable from the declaring root's protected default branch; unavailable observations remain unknown."
  - "Revocation and supersession use the same bound authority policy as acceptance; unauthorized comments are quarantined rather than counted as conflicts."
  - "Keep historical acceptance separate from current applicability, without introducing effect admission or a new approval store."
---

The driver accepted milestone-support/06 option 1 in an unedited human comment
naming this record, the proposal and its exact head.
Ruling: https://github.com/medici-finance/assay/issues/5705#issuecomment-6715755830

This record transcribes that human decision; it does not create an approval.
The receipt adapter binds the canonical milestone/version, contract source, exact
subject/profile, contributor evidence, prerequisites, criterion coverage and bound
acceptance authority to a content-addressed manifest and explicit human disposition.
It reuses existing evidence, identity and ruling mechanisms, preserves duty
separation, and keeps accepted history addressable after revocation or loss of
current applicability. Authentication, disposition, exact-subject matching and
coverage remain separate requirements; a corroboration pass alone is not acceptance.

The protected-source and revocation rules in the accepted proposal are required
implementation cases. Producer-selected unmerged authority/contract commits cannot
set policy. A successor or revocation must pass authentication and authorization
before it counts; unauthorized comments cannot force an accepted receipt to unknown.

Worker-desk owns implementation under `assay--milestone-support--06`, after refreshing
source, overlapping work and this ruling. Coordinator attestation and independent
review remain required. The pure adapter grants no execution, merge, release or
deployment authority. This design approval accepts no milestone outcome and supplies
no implementation, Verify/Evidence or lifecycle-completion credit. /07 remains the
readiness-integration owner and /10 the dispatch/pickup-admission owner.
