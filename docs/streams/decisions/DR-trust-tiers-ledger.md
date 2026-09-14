---
id: DR-trust-tiers-ledger
date: "2026-09-12"
title: "Four contributor trust tiers, recorded in an operator-side per-repository ledger rather than a file in the public tree"
consequence: critical
decided-by: "human:<name>"
alternatives:
  - "Keep the ledger as a tracked file in the public repository (a register under the docs tree, like findings or decisions) — ruled out: the ledger's rows are trust judgements about named external people, and publishing them turns a review-workflow record into a public reputation list. A demotion row would be a permanent public mark on an individual, which is a consequence far out of proportion to the workflow problem being solved, and a promotion row exposes a relationship the contributor did not ask to have published. The ledger therefore lives with the roster, outside every tree the tools evaluate."
  - "Two states only (trusted / untrusted), as today — ruled out: it is the measured status quo and it is what forces the same stranger-grade assessment on a contributor whose third change is landing. With no intermediate state there is nowhere to record 'this identity has one merged change and nothing adverse', which is exactly the fact a second review wants."
  - "Derive the tier automatically from merge history (N merged pull requests promotes) — ruled out: an automatic promotion is an authorization granted by a counter, and the counter is gameable by exactly the submission pattern this stream exists to handle — a burst of trivial accepted changes would buy workflow auto-approval. Promotion must cost a human act."
  - "Tier the ACCOUNT globally across the fleet rather than per repository — ruled out: trust is contextual. An identity a maintainer knows well in one project is a stranger in another, and a fleet-wide tier would silently export one project's judgement into projects whose maintainers never made it. Per-repository keeps the grant where the granting maintainer's knowledge is."
accepted:
  - "The ledger is operator-side configuration, so it is not reviewable by pull request and not visible in the public tree. Its integrity rests on the same custody the roster already has, and an unreadable or absent ledger must fail closed to `unknown` for every identity rather than failing open."
  - "Every identity not named in the ledger is `unknown`, which is the current bar. Adopting the tiers therefore changes nothing until a human records the first promotion — the model is opt-in by construction."
  - "Four tiers is one more distinction than most repositories will use. `blessed-once` exists specifically so admitting a single item never implies a standing grant, which is the conflation the present per-item bless invites."
  - "A tier is a grant of AUTOMATION, never of merge authority; the highest tier still merges nothing. Two capabilities that look adjacent — 'workflows run without a manual approve' and 'a change may land' — stay separate, and the second remains branch protection's and a human's."
  - "Demotion is available and is a recorded human act with a reason, which means an adverse judgement about a person exists in writing. Keeping it out of the public tree is what makes that acceptable."
---

**PROPOSED — no ruling is recorded.** `decided-by:` is a placeholder until a human rules on
the brief's decision issue; the ruling is recorded into this file in the same motion.

The decision has two halves that must be taken together: the vocabulary, and where it is
written down.

**The vocabulary.** `unknown` is every identity the ledger does not name — the present bar,
and the fail-closed default. `blessed-once` records that a maintainer admitted one specific
item and nothing more; it expires with that item. `contributor` is a standing grant recorded
after a maintainer has seen the identity's work land and judged it sound. `maintainer` is the
existing roster membership and is unchanged by this stream. Each tier unlocks a named,
enumerable set of things: how deep the review lane runs, whether continuous-integration
workflows need a manual approve, whether desk automation may act on the identity's items, and
whether the changelog proxy path applies. The set is small on purpose, and every entry in it
is an automation behaviour.

**Where it is written.** The tempting home is a file in this repository, because every other
register here is one and the review-by-pull-request property is genuinely valuable. It is the
wrong home, for a reason that outranks that property: the rows name external people and record
judgements about them. This tree is public. A register is append-only and tombstoned, so a
demotion recorded here could not later be withdrawn — only annotated. The ledger therefore
sits with the operator's roster configuration, outside the repository, and what the public
tree carries is the *shape* of the ledger and the rules for reading it, never its contents.

**What this record does not decide.** It does not fix the per-tier capability list (the
brief's Task proposes it and the review gate weighs it), it does not decide the continuous-
integration posture that keys on the tiers, and it does not establish that the human recording
a promotion is a different person from the one who benefits — an attributed act, not a checked
identity boundary.
