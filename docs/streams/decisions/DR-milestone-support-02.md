---
id: DR-milestone-support-02
date: "2026-10-30"
title: "Approve the bounded design for negotiating reader capabilities through one shared contract (milestone-support/02), as posted on the decision issue"
consequence: major
decided-by: "human:ian"
alternatives:
  - "Approve with corrections. This was the second option the decision issue offered. The driver chose option 1 instead, and the ruling comment records no correction."
  - "Hold. This was the third option the decision issue offered, and the posted design says implementation stays held until a recorded ruling. The driver chose option 1 instead."
  - "Rely on tolerant YAML parsing, a release number or parser completion to show that a reader supports new semantics. The posted design rules this out: old binaries cannot become safe by adding tolerant keys, and parser completion or a release number alone cannot advertise milestone readiness, dispatch or acceptance."
accepted:
  - "Precise wire key allocation for the capability envelope stays part of the design review, as the posted design states. The record does not fix it."
  - "Fixture success is neither human acceptance nor independent post-merge verification, as the posted design states. Each named production-path test needs a named PASS and a demonstrated wrong-but-plausible mutation."
  - "The follow-up briefs keep their own scopes (the milestone loader, negotiated brief authoring, the graph and evaluation, trailers and claims, exact-subject and human-ruling adaptation, dispatch and pickup eligibility, views, history, reporting, and the qualified release). This delivery must not advertise those features as implemented."
  - "Operational declarations stay held until coordinated readers and rollout are qualified. No gate is weakened, no pin changes, no operational opt-in is enabled, no stream is activated and no release is published."
---

**Ruling recorded (2026-10-30): option 1, approve the bounded design.** The driver ruled on
the decision issue for brief milestone-support/02,
[#4531](https://github.com/medici-finance/assay/issues/4531), from the driver's own login.
The ruling comment
([2026-10-30T00:56:17Z](https://github.com/medici-finance/assay/issues/4531#issuecomment-6512231509),
not edited) reads, verbatim:

> milestone-support/02: 1 — approve the bounded design in issuecomment-6424695649.

The bounded design it approves is the architecture-pickup comment on the same issue
([2026-10-26T18:55:53Z](https://github.com/medici-finance/assay/issues/4531#issuecomment-6424695649),
not edited). That comment's section "Bounded design for maintainer review" is the whole scope
of this record. The record copies the ruling into the register for the lifecycle's
design-approval gate (`spec/lifecycle-v1.md` §4.4). It makes no new decision, adds nothing to
the design and interprets nothing in it. The human act is the driver's comment on #4531.

**What the approval rests on.** The posted design, read from that comment, covers five points:
one deterministic assessment API in the existing statusgen module; one versioned capability
envelope with a matching JSON schema; an additive extension of the existing JSON discovery that
keeps bare version output; routing the existing desk schema-gate adapter through the same
assessment, with enforcement kept at the consuming boundary; and a spec, schema, conformance
fixtures and mutation controls with the three named production-path tests. This record cites
that comment and does not restate it.

**Why there is no `ruling:` link.** The ruling comment names the brief by its board id. It
does not name this record. The register's corroboration check requires the comment's own text
to name the record's `DR-<slug>` id (`spec/registers-v1.md` §7.5, condition 5), so a link to
that comment would fail as `record-not-named`. The brief itself carries the `ruling:` link,
because for the brief the board id is what the comment must name (`spec/lifecycle-v1.md`
§4.5).

**What corroborates `decided-by`.** With no `ruling:` link, the corroboration check reads only
the anchors on the pull request that adds this record: an APPROVED review by the driver's own
login, or an explicit approval comment by the driver's own login on that pull request. The
ruling on #4531 decides the design. It does not by itself corroborate this record's
`decided-by`.

**The tree's present condition (an observation, not a ruling).** The work this design
describes is already on main: pull request #4947 merged on 2026-10-31T22:20:07Z (merge commit
`74d0a8901ccf`), after the ruling, and its change left the brief's board row at `todo`.
A risk-gated brief cannot reach `implemented` without a cited design record and a decision
issue. This record and the brief's `design:`, `decision-issue:` and `ruling:` keys
supply those citations. This record is not evidence that the work is correct. That is the job
of the brief's Verify table, run by an independent verifier on merged main.

**The consequence level.** The ruling comment states no level. `consequence: major` is a desk
default, declared in the pull request that adds this record and open to change by the
maintainer. The brief's `gate-why` says a wrong capability interpretation must not authorize
unsupported work or weaken existing gates. If the design is wrong, a reader could be judged
compatible with semantics it does not implement. That needs rework, and nothing here is
irreversible: no pin change, release, activation or operational opt-in rides on this record.
