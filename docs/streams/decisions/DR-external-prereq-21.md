---
id: DR-external-prereq-21
date: "2026-09-21"
title: "Clear a standing rejection whose sole blockers were external prerequisites at an unchanged head, once every prerequisite is independently verified from fresh evidence; preserve the commit-only rule for every other rejection"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Retain the commit-only rule — keep refusing every same-head APPROVE over a standing CHANGES_REQUESTED, so a PR blocked only on an upstream PR or a pending decision needs an unrelated no-op push (or a human dismissing the review by hand) once the prerequisite lands. Ruled out: it is the behaviour this decision exists to bound. A no-op push games the very head-move rule the refusal enforces, and a per-PR hand dismissal does not scale; both spend review rounds re-verifying code that never changed. The refusal stays the default for every OTHER rejection — this decision removes only the one case where the sole blockers were external prerequisites that have since, verifiably, changed."
  - "Infer external-prereq-only from prose — read the CR's English for phrases like 'waiting on upstream' and clear on that. Ruled out: guessing on a GRANT path is exactly how a laundering hole opens. A CR carrying a real code finding and the word 'upstream' would read as external-prereq-only. The exemption is established ONLY by an explicit typed declaration (an External-Prereq-Only line plus a review-finding block whose every blocking finding is an external prerequisite), mirroring the check-only exemption's marker discipline; a mixed CR is ineligible by construction."
  - "Trust the reviewer's citation of the satisfying event — accept the re-approve's stated merge commit / decision as proof. Ruled out: a cached or fabricated citation would then clear the block. The ready gate RE-VERIFIES every prerequisite from fresh, independent evidence at the ready boundary, bound to object, condition, satisfying ref and observation time, and fails closed on a wrong revision, a prerequisite predating the rejection, a later revocation, an unrelated object, unreadable evidence, a standing security failure or any code/content finding. Could-not-check is never rounded up to a clearance."
accepted:
  - "A standing CHANGES_REQUESTED whose ONLY blockers are explicitly recorded external prerequisites may clear at an UNCHANGED head, with no source commit and no empty or comment-only commit, once every named prerequisite is independently verified as changed AFTER the rejection. The exemption removes a block; it never manufactures a verdict — a governing APPROVED at head is still required."
  - "The exemption is DECLARED, not inferred: the CR carries the typed declaration and a review-finding block in which every blocking finding is an external prerequisite, and the clearing APPROVE cites each satisfied prerequisite. Legacy free text cannot qualify, and a single code/content blocking finding makes the CR mixed and ineligible."
  - "The board, the review planner and the ready gate AGREE: a declared external-prerequisite re-review is surfaced distinctly (not as a suspected forgery), and the ready gate is the single place that performs the authoritative independent re-verification and acts on it. Mixed findings, missing evidence, later revocations and unsatisfied security reviews still block, and issue closure alone never establishes the human ruling."
  - "The sibling check-only exemption, the existing three-round finding cap and its arbitration, the independent security review, and the human merge/ready authority are all UNCHANGED. This decision adds one narrow, evidence-gated way to clear a rejection; it removes none of the controls standing behind the flip."
---

**Ruling recorded (2026-09-21): APPROVED — option 1.** The driver (`human:<name>`) recorded the
ruling on the brief's decision-gate
[issue 1403](https://github.com/medici-finance/assay/issues/1403) — the
[ruling comment](https://github.com/medici-finance/assay/issues/1403#issuecomment-5762224251)
— approving the narrow same-head external-prerequisite exception with independent, fresh
re-verification of every prerequisite, revalidated at ready time, while mixed findings,
missing evidence, revocations and unsatisfied security reviews still block. This record
transcribes that ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4); it does not mint a new decision — the human act was the
driver's own-login ruling on the issue.

**The decision.** `docs/streams/desk-supervision/brief-21-external-prerequisite-reverification.md`
adds ONE narrow exemption to the rule that an APPROVED over a standing CHANGES_REQUESTED at an
unchanged head cannot be a re-verification. When the rejection's only blockers were external
prerequisites — an upstream PR that had not merged, a decision that had not been made — and
every one has since been independently verified as satisfied AFTER the rejection, the block
clears with no synthetic push. The exemption is established by an explicit typed declaration
on the CR and a per-prerequisite citation on the re-approve; the ready gate re-verifies each
prerequisite from fresh evidence at flip time.

**The constraint behind it.** The control this narrows is which review findings may hold a
pull request and how a standing rejection can clear — a review-control change, which is why it
was human-gated. The safety direction is preserved in every direction: a code/content finding
still blocks (a mixed CR is ineligible), a wrong revision / stale prerequisite / later
revocation / unrelated object / unreadable evidence all continue to block (could-not-check is
never a clearance), a standing `Security-Review: fail` still blocks, and the human merge and
ready-flip authority are untouched. The exemption removes a block only when the recorded,
verifiable facts say the sole reasons for it are gone.

**What this record does not decide.** It does not attest the design is correct — that the
alternatives were weighed is recorded here; whether the boundary's shape is right is the
review gate's judgement (the brief's Review section) and the change's own Verify table after
it lands. It does not touch the sibling check-only exemption, the three-round cap, the
independent security review, or the human merge gate, and a source merge does not by itself
prove the installed-runtime behaviour.
