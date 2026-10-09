---
id: DR-forge-neutral-36
date: "2026-10-08"
title: "Library-first for offline and frozen inputs only: statusgen may link a narrow module's offline and frozen packages; every online forge read stays on the read verb, `deskkit` stays internal, and no credential moves into a process that has none today"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2395#issuecomment-6066969289"
alternatives:
  - "Option 2, verb-only stays (the 2026-09-14 direction unchanged) — not chosen; the ruling selected option 1. No reason was given with the ruling. As put to the driver, its consequence was: forge-neutral/36 and the library-first amendments to forge-neutral/18, /35 and desktools-v2/08 are withdrawn; statusgen/15 is not a forge read and stays."
  - "Option 3, library-first including in-process online reads — not chosen; the ruling selected option 1. No reason was given with the ruling. As put to the driver, its consequence was: the plan does not prescribe this and would be reworked; forge-neutral/18's gate is re-derived, and retiring the read-verb bridge needs its own human-gated brief."
accepted:
  - "statusgen may link a narrow module's offline and frozen packages."
  - "Every online forge read stays on the read verb."
  - "`deskkit` stays internal."
  - "No credential moves into a process that has none today."
  - "The 2026-09-14 direction (statusgen reaches the forge seam only by running the read verb, never by linking a package) is amended in that part only."
  - "forge-neutral/36 becomes eligible for pick-up and keeps its own human gate."
---

**Ruling recorded (2026-10-08): option 1, library-first for offline and frozen inputs only.**
The question was put on [#2395](https://github.com/medici-finance/assay/issues/2395) in the
desk's restatement of 2026-10-08T18:50:53Z, which offered three numbered options with no
default and asked for the reply shape `<1|2|3> — DR-forge-neutral-36`. It replaced the
lettered options of an earlier ask on the same issue. The driver's
[ruling comment](https://github.com/medici-finance/assay/issues/2395#issuecomment-6066969289),
posted under the driver's own login, never a role App, at 2026-10-08T18:59:56Z and not edited since,
reads `1 — DR-forge-neutral-36`. No rationale was given with the answer, and this record
supplies none. It transcribes the ruling and the option text the issue put in front of the
driver into the register, for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4). It does not mint a new decision: the human act is the driver's
comment on #2395, not this file.

**Option 1, as written on the decision issue.**

> **Library-first for offline and frozen inputs only** (desk recommendation, not a ruling).
> statusgen may link a narrow module's offline and frozen packages. Every online forge read
> stays on the read verb, `deskkit` stays internal, and no credential moves into a process
> that has none today. This is what #2396 prescribes if adopted. Consequence: the 2026-09-14
> direction is amended in that part only; forge-neutral/36 becomes eligible for pick-up and
> keeps its own human gate.

**The decision.** `docs/library-first.md` and the desktools-v2 spec (§2 Principle 2,
commitment 3) record library-first reuse within the limits above, and
`docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md`
cites this record with its `design:` key.

**The consequence level.** `consequence: major` is not in the ruling comment, which states no
level. It is a desk default declared in the pull request that adds this record (its
`## Desk-decided` section), amendable by the maintainer; it is not part of what the driver
ruled.

**What this record does not decide.** It does not sign off forge-neutral/36: that brief keeps
`gate: human` and its own `## Human decision`, and its implementation still needs that human's
answer. It does not retire the read-verb bridge for any online read, and it does not let
statusgen make an online forge read in-process. It does not attest the design is correct: it
records that the alternatives were weighed and names a human approver; whether an
implementation holds is the review gate's judgement and then each brief's Verify table.
