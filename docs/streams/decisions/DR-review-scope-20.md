---
id: DR-review-scope-20
date: "2026-09-21"
title: "Bound review scope with a declared first-pass inventory and an impact-based blocking boundary; preserve the three-round cap and the independent security review"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Retain the existing rule — leave review-prompt clause 8's broad prose sweep as the gate, so any occurrence a repository search surfaces can hold the pull request. Ruled out: it is the behaviour this decision exists to bound. An incremental search keeps discovering old, already-present instances after each fix, and a small change acquires unbounded cleanup scope — each newly-noticed sibling sentence becomes a fresh blocker in its own round. The continuing cost is real (several review rounds spent on prose that was in the first tree), which is why the status quo was not kept."
  - "Adopt a two-round review cap instead of narrowing scope — proposed on the linked process issue. Ruled out for THIS decision: the round cap and the scope rule are different controls. Cutting the cap does not stop each search hit from becoming a new blocker; it just ends the fight sooner and can approve a real defect because a counter expired. The three-round cap and its arbitration path are preserved unchanged; this decision narrows what counts as a NEW blocker, not how many rounds a genuine one gets."
  - "Exempt untouched files from review entirely — a blunter narrowing that would drop every occurrence outside the diff. Ruled out: a required operator-state table is a real deliverable a change owes even when the worker omitted it from the diff, and a safety consequence of the change can sit outside the edited lines. Untouched is not automatically irrelevant, so the boundary is impact-based, not diff-membership-based."
accepted:
  - "The first review of a false-claim class must INVENTORY its related occurrences before the verdict — the changed surface, the item's required deliverables, and references to the affected entity — and record the search, its scope, its exclusions and the input revision. An incomplete search is reported incomplete and never certified clean (the three-state instrument rule, applied to discovery)."
  - "A blocking finding must name a concrete failure and one scope basis: changed behaviour, an explicit acceptance obligation, a material PR-body/Verify claim, or a demonstrated safety consequence of the change. Unrelated pre-existing prose routes to a linked follow-up; co-location (same directory or substring) is not a basis."
  - "A late or missed sibling occurrence retains its original claim class and round count — it is review coverage failure, not a fresh class — and a previously non-blocking occurrence cannot become blocking merely because another file was edited, absent recorded changed impact or new evidence. This is the line between genuine changed evidence and a bypass of a standing rejection."
  - "The existing three-round cap on a finding class and the independent security review are UNCHANGED. Repository search remains discovery; it is not narrowed away, only stripped of automatic blocking authority."
---

**Ruling recorded (2026-09-21): APPROVED — option 1.** The driver (`human:<name>`) recorded
the ruling on the brief's decision-gate
[issue 1402](https://github.com/medici-finance/assay/issues/1402) — the
[ruling comment](https://github.com/medici-finance/assay/issues/1402#issuecomment-5762179072)
— approving the first-pass inventory and the impact-based blocking boundary while preserving
the existing three-round policy and the independent security review. This record transcribes
that ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4); it does not mint a new decision — the human act was the driver's
own-login ruling on the issue, not this file.

**The decision.** `docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md`
bounds a small change's review scope at both ends. The first pass INVENTORIES and DECLARES the
related occurrences of a false-claim class (recording search, scope, exclusions and revision),
and an incomplete search is never certified clean. A hit HOLDS the pull request only when it
names a concrete failure and one of four scope bases; unrelated pre-existing prose becomes a
linked follow-up. A late sibling keeps its class and round count, and a previously non-blocking
occurrence is not promoted without recorded changed impact or new evidence.

**The constraint behind it.** The control this narrows is which review findings may hold a pull
request and how a standing rejection can clear — a review-control change, which is why it was
human-gated. The safety direction is preserved: a genuine defect still blocks, including outside
the edited lines; the narrowing removes only the AUTOMATIC promotion of every search hit to a
blocker, and it requires recorded evidence for any promotion of a previously non-blocking
occurrence, so a bypass of a standing rejection is distinguishable from real new evidence. Two
independent controls stand behind the boundary and are explicitly untouched: the three-round cap
(with its arbitration packet) bounds the fix-to-re-review cycle on a genuine class, and the
independent security review is a separate verdict artifact on a separate signal.

**What this record does not decide.** It does not attest the design is correct — that the
alternatives were weighed is recorded here; whether the boundary's shape is right is the review
gate's judgement (the brief's Review section) and the change's own Verify table after it lands.
It does not adopt the two-round cap proposed on the linked process issue, and it does not touch
the round-cap mechanics or the security review.
