---
id: DR-desk-tools-48
date: "2026-11-15"
title: "Approve the desk-tools/48 landing-capacity read model: one meter plan shared by the landing gates and a read-only capacity read, with widen advice bounded by that reading; no limit, cap, authorization or width changes"
consequence: major
decided-by: "human:ian"
alternatives:
  - "Option 2 on the decision issue, request corrections and keep the change pending. Not chosen: the driver answered option 1. No rationale was recorded, and none is supplied here."
  - "Option 3 on the decision issue, hold the change. Not chosen: the driver answered option 1. No rationale was recorded, and none is supplied here."
  - "A capacity view with its own table of caps, copied beside the gates. Ruled out by the brief's Task 1 (\"do not hand-copy caps into a new configuration table\"). A copied cap can drift from the enforced one and report room the gate refuses, which is the defect class the delivering pull request closes."
  - "A new limiter, a cap increase, a reservation authority or a universal provider budget. Ruled out by the brief's interface contract: \"A dated read model over current meters, never a new limiter, cap increase, reservation authority or universal GraphQL budget.\""
accepted:
  - "The reading covers this host's audit ledger only. Provider quota and other hosts' writes are reported as unknown, never as spare capacity. An unreadable ledger, or a meter whose check and counted usage disagree, is could-not-check (exit 6) and reports no capacity."
  - "With no `--in-flight` count given, every slot of the loop's resolved width is assumed to hold one item whose landing writes are not yet charged. The reading can under-report capacity while slots sit idle, never over-report it."
  - "Across several watched repos, one blocked or unreadable repo binds the whole answer. Advice can hold the width because of one busy repo even when the next item would land elsewhere."
  - "`--held` and `--continuation` counts are reported only; their landing writes are reserved only through `--in-flight`."
---

**Ruling recorded (2026-11-15): option 1.** The driver ruled on the decision issue for brief
desk-tools/48, [#5837](https://github.com/medici-finance/assay/issues/5837), from the driver's
own login. The
[ruling comment](https://github.com/medici-finance/assay/issues/5837#issuecomment-6899956912),
posted 2026-11-15T19:25:12Z and not edited since, reads `desk-tools/48: 1`. Option 1 on that
issue is "Approve the concrete implementation after independent verification." The record's
`date:` is that comment's posting date (UTC).

This record copies that ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4). It makes no new decision. The human act is the driver's comment
on #5837.

**Why there is no `ruling:` link.** The ruling comment names the brief by its board id. It
does not name this record. The register's corroboration check requires the comment's own text
to name the record's `DR-<slug>` id (`spec/registers-v1.md` §7.5, condition 5), so a link to
that comment would fail as `record-not-named`. The brief's own `ruling:` key does link that
comment: the decision-gate hold (§4.5) needs a comment that names the brief, which this one
does.

**What corroborates `decided-by`, and its state.** With no `ruling:` link, the corroboration
check reads only an APPROVED review by the driver's own login on the pull request that adds this
record, or an explicit approval comment by the driver's own login on that pull request. When this
record was written, neither existed. If the driver instead comments on #5837 naming
`DR-desk-tools-48`, that comment goes in this record as the `ruling:` link before merge. Without
one of those, the stamp is not corroborated, and the record should not merge.

**The decision.** Brief desk-tools/48 asks for a dated, read-only capacity reading derived from
the meters the landing gates already enforce, and for widen advice bounded by it. The design the
driver approved, as delivered in the pull request that adds this record:

1. The rate-limit gates and the new read share one meter plan. Each cap exists once, in that
   plan, and every check function takes its limit as a parameter.
2. The read charges and reserves nothing. It runs each meter's own check and refuses to report
   capacity when that check's verdict and the counted usage disagree.
3. `deskroster capacity` prints that reading per repo, with its date and scope. `deskboard
   throughput` bounds its widen advice by the landing capacity, the actionable depth and the
   roster maximum. A blocked or unread landing budget never yields a widening command.
4. `verifyloop` help points at the capacity read for a deep verify plan. Verify plans are not
   changed.

No limit, cap, authorization or width is changed. The brief and its delivering diff are the
authority; this summary does not amend either.

**The consequence level.** The ruling comment states no level. `consequence: major` is the
transcriber's classification, taken from the brief's risk answers (`customer: yes`, the other
three `no`, including `irreversible: no`) and its `gate-why`. A wrong reading could advise a
width the gates then refuse, or hold a width that had room. Either needs rework, and a route back
exists: the reading changes no limit, and the width is still set by a separate command. The driver
can correct the level before merge.

**What this record does not decide.** The ruling approves the implementation "after independent
verification". That verification is the brief's Verify table, run after merge by a
non-implementer; this record attests none of it, and it is not evidence that the work is correct.
The approval grants no workflow activation and no additional write scope, as the decision issue
states. The `verified` and `done` flips of this `gate: human` brief still need their own human
sign-off on that verification.
