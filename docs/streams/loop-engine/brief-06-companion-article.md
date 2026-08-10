---
brief: loop-engine/06
title: Companion article + generic cloneable drain-harness — publish the pattern, not the product
wave: 1
depends: ["loop-engine/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-19 by Fable design session (coordinator direction, folded into the loop-engine design PR)
sources: ["docs/loop-engine-architecture.md (§11 deliverable spec + §9.3 publish-target open question — freshness-checked 2026-07-19)", "docs/brand-guide.md (authoritative for visual/verbal decisions; articles = white background, light theme, blue accent)", "docs/articles/ (the whitepaper/article track's source home; sibling articles set the register)", "open-core strategy (Assay open-core: free tooling as the expertise showcase)", "loop-engine/01 (the engine the article describes; the case-study half)"]
exec-tier: strong
exec-tier-why: public-facing voice demonstrating expertise in both agent tooling and Canton; brand register and honest-bound framing are judgment work, not template fill.
why: >-
  The drain engine's value generalizes: any team running agent loops hits the
  orchestration-in-attention collapse. Publishing (a) a technical article on the mechanism
  and the fix and (b) a generic, cloneable drain-harness others can adopt turns internal
  tooling into an expertise showcase — the open-core strategy's exact move. The Medici
  internals are the case study; the generic harness is the takeaway.
---

# Brief 06 — Companion article + generic cloneable drain-harness

## Context
files: `docs/articles/drain-engine.md` (planned) — article source; filename may adjust to
match sibling naming, record the final path in Evidence — plus a generic harness skeleton
staged under `docs/articles/drain-engine-harness/` (planned) pending human:<name>'s §9.3 publish-target
decision
(repo-neutral layout — the decision gates only the final publish step, not authoring)
facts:
- TWO COUPLED DELIVERABLES (arch doc §11): the article AND the generic harness; the article
  walks the reader through the harness — Medici internals (verify-desk, deskboard, the
  Canton dev/deploy/verify pipeline) appear as the case study, the harness is the takeaway.
- Article required coverage: the attention-capacity insight (orchestration-in-attention →
  single-threaded collapse, told as a general agent-systems finding); moving the scheduler
  out of the model into deterministic code; the item-worker contract
  (select/claim/tier/dispatch/land/idle) and why it stays small; the integrity win WITH the
  honest bound (attribution + structural separation, NOT cryptographic un-forgeability).
- Framing constraints: NOT product-centric — "this is how we build at Medici," an
  engineering-practice piece, never a product pitch. Dual expertise, weighted: (a) AI/agent
  tooling is the PRIMARY theme (how we run agent loops that don't stall); (b) Canton chains
  lighter but concrete — these loops drive our Canton dev/deploy/verify pipeline (DAR ship
  flow, ledger verification, Flux convergence), with at least one real example.
- Generic harness constraints: framework-agnostic, ALL Medici-specific coupling stripped
  (no our package names, no our k8s, no our stream/brief format); self-contained and
  cloneable — an outside team can clone it and run non-stalling agent loops in their own
  stack. It implements the §4 contract shape (typed items, constant-N pool, claim/land/idle
  hooks) against stand-in adapters, not our board tools.
- Publish target (open-core toolkit vs medici-examples vs new standalone repo) is human:<name>'s
  §9.3 decision — do NOT decide it; author repo-neutral and surface the question.
- Brand: docs/brand-guide.md is authoritative — article renders white background, light
  theme, blue accent (house convention for articles/papers; app UI stays dark).
- Honest-bound rule binds the article hardest: it is public — no un-forgeability claims,
  no "measured not self-reported" overclaim (the-desk red-team).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Do not publish anywhere external and do not create any repo — staging in-repo only until
  human:<name> answers §9.3.
- No attribution lines. If anything is unclear or contradicts repo state: report
  NEEDS_CONTEXT, don't guess.

## Task
1. Author the article in `docs/articles/` per Context: required coverage, case-study/
   takeaway structure, dual-expertise weighting, non-product framing, brand register.
2. Build the generic harness skeleton (repo-neutral layout, own README with a quickstart
   that assumes nothing about our stack); article sections reference its files by relative
   path.
3. Strip-check: sweep the harness for Medici coupling (package names, k8s, stream/brief
   vocabulary) — the Verify table makes this executable.
4. Surface §9.3 to human:<name>: label the design PR conversation or file the `needs-decision`
   issue if not already filed; record which in Evidence.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/articles/drain-engine.md; echo $?` | 0 (or the recorded final path from Evidence) |
| 2 | `grep -icE -e 'attention' -e 'scheduler' docs/articles/drain-engine.md` | ≥4 (attention-capacity + scheduler-out-of-model both covered) |
| 3 | `grep -icE -e 'canton' docs/articles/drain-engine.md` | ≥2 (Canton pipeline case-study present) |
| 4 | `grep -icE -e 'unforgeab' -e 'un-forgeab' docs/articles/drain-engine.md` | ≥1 AND every hit is inside the honest-bound disclaimer (reviewer spot-checks context) |
| 5 | `grep -rciE -e 'medici' -e 'deskboard' -e 'statusgen' docs/articles/drain-engine-harness/ \| grep -v ':0' \| wc -l` | 0 in code/config files (README's case-study pointer back to the article is the one allowed mention; record it) |
| 6 | `ls docs/articles/drain-engine-harness/README.md; echo $?` | 0 (self-contained quickstart exists) |
| 7 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at verification time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model. Reviewer confirms (a) the article is practice-framed, not product-centric,
(b) both expertise axes present with the stated weighting (agent tooling primary, Canton
concrete but lighter), (c) the harness is genuinely cloneable — no hidden Medici coupling
beyond the allowed README pointer, (d) the honest bound is stated and no public overclaim
survives, (e) §9.3 was surfaced to human:<name>, not decided, and nothing was published externally,
(f) brand-guide conformance (white/light/blue article convention).
