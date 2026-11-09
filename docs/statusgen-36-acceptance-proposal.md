# Proposed statusgen/36 acceptance correction

Issue #5684. This draft prepares version 3 of the brief with Command-only
corrections to rows 12 and 23. Every original row ID, Class, Expect, shell,
order and obligation is retained. Evidence and lifecycle records are unchanged.
This is a held proposal: the current acceptance resolver cannot admit a second
amendment, and the immutable consumer report exposes a genuine routing-proof
failure. Neither this draft nor a merge supplies acceptance credit.

## Performance command

Row 12 keeps its exact existing performance comparison: fixed release v1.0.52,
five successful coverage runs per binary, both printed medians, exit 3 for a
failed coverage run and the 1.10 multiplier. Before timing, it executes the
existing coverage batching behavior test and requires that test's named PASS.
The test releases all 200 fixture claims while bounding git calls independently
of row count. Requiring its actual PASS makes a missing test refuse, even when
Go reports success with no matching tests. This ties the performance row to the
delivered reader while preserving the independent wall-clock regression guard.
It grants no exemption from fail-first or the performance budget.

## Immutable consumer result: still fails

The producer delivery remains #4409:
`3b2235af1af6dd42118cbf975eb4ac2b8e8ab409`, with first parent
`c288fd881e7fe5532614ad25a4610be7c6acf8a3`. Row 23 names both full pins
and reports the checker's stdout even when it fails. It retains the original
exit-0 obligation and asserts the actually observed exact summary, including
unchecked and disproved counts. It consequently remains red; the assertion is
a diagnostic bound, not a proposal to accept a disproved consumer.

The current shared checker reports:

```text
summary: 7 corroborated, 1 disproved, 1 unchecked, 0 brief(s) claiming nothing
```

The disproved delivered claim is `statusgen/autoflip.go and
statusgen/reconcilefold.go ...: fixed-here`. Neither file changes between those
pins. Both callers already use `evaluateCoverage`; the delivery changes its
shared implementation in `statusgen/coverage.go`, including observation of v2
inputs and records. That is source evidence of indirect adoption, not a changed
path and not a successful consumer routing proof. The reader-census prose entry
is unchecked. The follow-up to statusgen/37 proves a target/back-reference only.
The report also expressly leaves producer identity and implementation completeness
could-not-check; those remain independent review obligations.

The earlier supplemental base-only report, 7 corroborated / 0 disproved /
2 unchecked, reads the latest checkout's routes rather than the immutable
delivered snapshot. It cannot establish the pinned row's expected counts.
Editing today's `consumers:` cannot repair the historical declaration. Widening
the base, choosing a later delivery or accepting checker exit 1 would discard an
original obligation. This draft does none of those.

Before row 23 can be an accepting command, the consumer-proof owner must provide
a reviewed, contract-preserving disposition for this indirectly updated consumer
at the original delivery. Any needed shared-owner change must preserve immutable
producer identity and the original exit-0 requirement, prove clean and negative
controls, and then derive its exact successful counts afresh. That work and its
acceptance authorization are separate from silently changing this command to
accept the current disproof. No local helper or alternate binary is proposed.

## Protected amendment gate

The existing human approval on issue #5684 for amendment #5689,
`37872cfb9961db6ad8825d0b628e6a2793fb3d21`, rows 12 and 15, remains valid
for that amendment. The design ruling for statusgen/36 is also already recorded.
Neither act is requested again or inherited by version 3.

`statusgen/acceptancetransition.go:acceptanceTransition` requires the complete
pre-amendment Verify section to equal the original delivered section. #5689
already changed rows 12 and 15, so a later version-3 amendment is rejected as
`acceptance changed between delivery and amendment`. The changed-command rule
also retains the complete original delivered definition. The current protocol
has one transition descriptor and no supported predecessor-approval chain.

Required separate work is a designed and independently reviewed successor model
in that same shared owner. Its minimum invariants are: keep the original delivery,
baseline and original-table coordinates; authenticate each predecessor's exact
immutable amendment and its existing human approval afresh; permit no unapproved
intermediate acceptance edit; preserve each row's effective prior obligations;
allow only one corrected column per approved row; require exact version increments,
first-parent ordering and complete historical Evidence preservation; and require
fresh independent witnesses after the final approved amendment. Original rows
cannot disappear or become supplemental. Every authoritative consumer must use
the same validated plan. This is a proposal for that design, not its approval or
an implementation exception.

After that support and the consumer-proof disposition exist, the exact final
landed correction still owes its own externally mapped human approval naming its
actual immutable amendment and corrected rows. The currently supported v1
payload cannot honestly describe a successor here, so this draft supplies no
misleading runnable approval payload. Independent verification, Evidence and
status transitions remain with verify-desk; readiness remains with review-desk.

## Architecture pickup

Source: aae4fbf71bca3419f1ac7ece8e83e8e5a651983e. The dispatched packet and
home worktree match that source. `AGENTS.md`, the existing statusgen/36 design
record, `docs/protected-paths.md`, `docs/consumer-delivery-checks.md`,
`docs/contracts.md` (S-evidence-coverage and R-original-table-closure),
`docs/library-first.md` and `docs/go-engineering.md` retain their current owners.
The shared acceptance resolver remains authoritative for lint, coverage, closure,
reconciliation and status consumers. The historical consumer path remains
`runDeliveryConsumers`; no dependency, module, CLI integration boundary or
production evaluator changes here. Continue the bounded Command proposal; hold
acceptance/closure on the two concrete support obligations above.
