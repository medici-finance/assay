---
id: DR-milestone-support-08
date: "2026-11-08"
title: "Approve bounded trailer and claim normalization with strict subject-token and spelling safeguards"
consequence: major
decided-by: "human:ian"
alternatives:
  - "Request corrections before approval: keep implementation held for a revised design. The driver instead accepted the reviewed proposal with two explicit safeguards."
  - "Hold: defer implementation pending further evidence or ownership resolution. The driver instead authorized the bounded design."
accepted:
  - "Implement the bounded design in PR #5712 at 5e8f7424a78027d9988801182e9ac8a82ba4977f."
  - "Reject duplicate or note-injected subject tokens."
  - "Spellings refused by the strict resolver must never be admitted through legacy normalization."
---

The driver accepted milestone-support/08 option 1 in an unedited human comment.
Ruling: https://github.com/medici-finance/assay/issues/5704#issuecomment-6715665207

This record transcribes that comment; it is not a bot-issued ruling. It carries no
frontmatter `ruling:` because the comment names the brief and reviewed proposal,
not this DR id. As in DR-iso-9001-08, the `decided-by` stamp uses the existing other
corroboration anchors; the brief's frontmatter `ruling:` binds its decision gate.
No record-naming requirement is relaxed.

The approved design keeps canonical logical identity, the existing physical claim
key and exact selected subject separate. It reuses workref and operation-scoped
capability assessment, preserves link kinds and complete-file classification, and
retains holder admission, CAS, custody, revocation and effect-admission checks.
Subject metadata remains in the same claim payload; no sidecar store or new
credential owner is authorized. The two explicit safeguards above are required
implementation acceptance cases, not optional review suggestions.

Worker-desk owns the implementation handoff under claim key
`assay--milestone-support--08`. It must refresh source, overlapping work and the
recorded ruling before dispatch. Coordinator author-model attestation and the
independent review gates remain required. This record grants no merge, release,
deployment, lifecycle completion or milestone acceptance authority. Original Verify
rows remain unexecuted until implementation; no Evidence is created here.
