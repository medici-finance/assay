# Proposed statusgen/36 acceptance correction (held)

Issue #5684. This is a held proposal for a later version 3 of the statusgen/36
brief: Command-only corrections to Verify rows 12 and 23. It does not edit the
brief. The brief's Verify table, `version:`, Evidence and lifecycle records stay
as they are on main. Two support obligations must be met first, and they are
described below. Nothing here supplies acceptance credit, an approval payload, a
local helper or an alternate binary.

## Why the brief is not edited here

The brief has one usable acceptance transition today: amendment #5689
(`37872cfb9961db6ad8825d0b628e6a2793fb3d21`, rows 12 and 15), which a human
approved on issue #5684. The resolver,
`statusgen/acceptancetransition.go:acceptanceTransition`, checks the current
brief against that amendment. It refuses with `current Verify differs from the
approved amendment` and `current acceptance version differs from amendment` when
they differ. It also refuses a second amendment, because #5689 already changed
the table after delivery (`acceptance changed between delivery and amendment`).

So landing a version-3 edit now would be harmful. #5689's transition would stop
working, row 15's approved correction would lose its credit path, and the
version-3 edit could not take its place. It would also be the unapproved
intermediate acceptance edit that `docs/protected-paths.md` refuses and the
successor model below forbids. The edit lands only after successor-chain support
and its own approval exist.

## Row 12: performance command

Proposed Command: the existing row 12 command, unchanged, with this prefix:

```text
(cd statusgen && out=$(GOWORK=off go test -count=1 -timeout 120s -v -run '^TestCoverageGitCallsBounded$' . 2>&1) && printf '%s\n' "$out" | grep -F -- '--- PASS: TestCoverageGitCallsBounded ') || exit 1;
```

Everything else in the existing comparison stays the same: fixed release
v1.0.52, five successful coverage runs per binary, both medians printed, exit 3
on a failed coverage run, and the 1.10 multiplier. The prefix requires the
named PASS of the batching test. A missing test then fails the row, even when Go
reports success with `[no tests to run]`.

**Limit of this fail-first (open for the approver).** The prefix runs the same
test and the same named-PASS check as row 11. Row 12 would go red at the
delivery's first parent `c288fd881e7f` only because that test does not exist
there; it is added by the delivery `3b2235af1af6`. The timing comparison itself
still runs against a fixed release tag, and verify-desk reported it green at the
delivery base on #5684. The prefix does not make that comparison discriminating.
Before an approval, the approver should pick one of the directions recorded on
#5684:

1. build the comparison binary from the delivery's first parent instead of a
   fixed tag, so the timing check can tell the two binaries apart; or
2. reclassify row 12 as a regression guard (no fail-first of its own), and say
   that its fail-first is carried by row 11.

This proposal does not choose between them.

## Row 23: immutable consumer result, still failing

The producer delivery is #4409: `3b2235af1af6dd42118cbf975eb4ac2b8e8ab409`. Its
first parent is `c288fd881e7fe5532614ad25a4610be7c6acf8a3`. A corrected row 23
should name both full pins with `--delivery` and `--base` instead of the moving
`git merge-base refs/remotes/origin/main HEAD` range in the current row.

**Observation only, not a proposed assertion.** With those pins, the shared
checker currently prints this summary and exits 1:

```text
summary: 7 corroborated, 1 disproved, 1 unchecked, 0 brief(s) claiming nothing
```

Exit 1 is correct here. `statusgen/consumers.go` returns 1 whenever any claim is
disproved, and `docs/consumer-delivery-checks.md` says "Exit 0 means no
disproved claim". This proposal therefore does not propose a row 23 Command that
asserts this summary. Such a command can never pass, contradicts the row's
unchanged Expect, and would encode a disproved route as the accepting outcome if
the checker's exit rule ever changed.

The disproved claim is `statusgen/autoflip.go and statusgen/reconcilefold.go
...: fixed-here`. Neither file changes between the pins. Both already call
`evaluateCoverage`, and the delivery changes that shared implementation in
`statusgen/coverage.go`. That is source evidence of indirect adoption. It is not
a changed path, and it is not a successful consumer routing proof. The
reader-census prose entry is unchecked. The report also leaves producer identity
and implementation completeness as could-not-check; those remain review
obligations.

An earlier base-only report of 7 corroborated, 0 disproved, 2 unchecked read the
latest checkout's routes, not the delivered snapshot, so it cannot set the
pinned row's counts. Editing today's `consumers:` cannot repair the historical
declaration. Widening the base, choosing a later delivery or accepting exit 1
would each drop an original obligation.

**Before row 23 can be authored**, the consumer-proof owner must provide a
reviewed disposition for this indirectly updated consumer at the original
delivery. That disposition must keep the immutable producer identity and the
exit-0 requirement, and it must be proved against clean and negative controls.
Only then can the row's exact summary be derived from a successful run. That
summary must report 0 disproved, and the command must require exit 0.

## Successor-chain support needed in the resolver

The current protocol has one transition descriptor and no predecessor-approval
chain. A version-3 correction needs a designed and independently reviewed
successor model in the same shared owner. Its minimum invariants:

- keep the original delivery, baseline and original-table coordinates;
- authenticate each predecessor's exact immutable amendment and its existing
  human approval afresh;
- permit no unapproved intermediate acceptance edit;
- preserve each row's effective prior obligations, so row 15's approved #5689
  correction keeps its credit;
- allow only one corrected column per approved row;
- require exact version increments, first-parent ordering and complete
  historical Evidence preservation;
- require fresh independent witnesses after the final approved amendment.

Original rows cannot disappear or become supplemental. Every authoritative
consumer must use the same validated plan. This describes the design that is
needed. It is not that design's approval or an exception to the resolver.

## Order of work

1. Successor-chain support in the shared resolver, reviewed.
2. Consumer-proof disposition for the indirectly updated callers, reviewed.
3. Row 12 direction chosen; row 23 summary derived from a successful run.
4. The version-3 brief edit, in its own change.
5. A new human approval naming that edit's immutable amendment and its rows.

The existing approval of #5689 and the recorded statusgen/36 design ruling stay
valid for their original scope. Neither is requested again, and neither carries
over to version 3. Independent verification, Evidence and status transitions
remain with verify-desk; readiness remains with review-desk.
