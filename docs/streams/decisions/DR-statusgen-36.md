---
id: DR-statusgen-36
date: "2026-10-25"
title: "Content-bound witness v2 environment rule: option 1 — a container run, or a host or back-filled run whose row is not marked environment-dependent and names no readable network tool, counts as environment-free; a host row whose command cannot be read needs a re-run"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4394#issuecomment-6399242471"
alternatives:
  - "Option 2 (strictest) — only runs in the pinned, network-less container runner count; host runs and back-filled records are recorded for audit but always reported as needing a re-run. Not chosen by the driver's ruling (option 1). The brief records it as the no-answer default after 2026-11-03 and as the option that closes the residual risk of an unmarked host row whose network use the library cannot see."
  - "Option 3 (widest, not recommended by the brief) — any run not marked environment-dependent by its author counts, including commands the library cannot read. Not chosen by the driver's ruling (option 1)."
  - "Option 4 — option 2, and in addition declared scope does not release yet: a v2 record releases only at whole-tree scope until the observed-read check exists. Not chosen by the driver's ruling (option 1). The brief records its cost: it keeps refused every stuck brief whose cause is whole-tree scope."
accepted:
  - "A host run or a back-filled record can release a v2 witness by digest when its row is neither marked environment-dependent by its author nor names a network tool in a form the library can read. An unmarked host row whose network use the library cannot see remains possible; the brief's pre-mortem names this as the stated residual risk under option 1, which option 2 would have closed."
  - "Under option 1, merging statusgen/36 approves the scope narrowing that each brief's `inputs:` declares, checked by human review of that brief (the brief's Human decision section). A file a row reads transitively but the declaration omits is caught by no mechanical control in this brief; the observed-read check is the named follow-up."
  - "A host row whose command the library cannot read is reported as needing a re-run, never released by digest. A row whose Class carries `+env` is always reported as needing a re-run, under every option."
---

**Ruling recorded (2026-10-25): option 1.** The brief's `## Human decision` asked which runs
count as environment-free for a content-bound (v2) witness. The driver (`human:<name>`) ruled
on the brief's decision-gate [issue 4394](https://github.com/medici-finance/assay/issues/4394)
in a comment posted under the driver's own login, never a role App relay — the
[ruling comment](https://github.com/medici-finance/assay/issues/4394#issuecomment-6399242471),
posted 2026-10-25T15:02:45Z and not edited since, reads `statusgen/36: 1`.

This record transcribes that ruling into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4); it does not mint a new decision. The decision text, the options
and their recorded costs are taken from the brief and the decision issue; nothing here adds
rationale beyond what those two record.

**The decision.** `docs/streams/statusgen/brief-36-content-bound-witness-schema-v2-reader.md`
makes a v2 witness record a digest of the files a Verify row read and the change request that
delivered it, and coverage releases the row when every recorded digest still equals the content
at the item's revision. A digest cannot capture a row that depends on the network, a
code-hosting service or a cluster, so those rows are never released by digest and are reported
as needing a re-run. Option 1 sets which runs count as environment-free: a run in the pinned,
network-less container runner; or a run on a host (or back-filled) whose row is neither marked
environment-dependent by its author nor names a network tool in a form the library can read. A
host row whose command cannot be read is reported as needing a re-run.

**What this record does not decide.** It does not decide the witness writer, which is
statusgen/37. It does not add the observed-read check for transitive reads, which the brief names
as a follow-up. It does not change the v1 witness rule, which the brief keeps unchanged. Whether
the implementation matches the ruling is the review gate's judgement and the brief's Verify
table's; this record attests only that the alternatives were weighed and names the human
approver.
