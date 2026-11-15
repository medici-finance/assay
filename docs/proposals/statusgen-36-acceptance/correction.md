# Statusgen/36 correction and remaining hold

Issue #5684. The version-3 draft changes only the Command cells of Verify rows
12 and 23. Every original row, Expect, class and historical Evidence entry
survives. The row-12 worktree/sample path separation and row-15 fixture-scoped
lint correction from #5689 are retained; row 12 is amended again here. Row 14's
Command remains unchanged. This amendment
supplies no approval, independent witness or lifecycle change. Row 23 remains
unresolved.

## Architecture pickup

Pickup source: `f85ceb1bd1a9d052ea19a8889f8a283a6f9470ae`. Owners remain
S-evidence-coverage in `docs/contracts.md`, the existing historical consumer
checker and the protected acceptance planner. Pickup inspected those owners,
`docs/protected-paths.md`, `docs/consumer-delivery-checks.md`,
`docs/verify-row-strength.md`, `docs/go-engineering.md` and
`plugins/assay/references/implementation-pickup.md` at that source.

The former chain-support prerequisite is now implemented in
`statusgen/acceptancetransition.go` under DR-transition-chain. Its independent
acceptance is separate: statusgen/40's retained Evidence reports an admitted
environment failure. Source presence does not declare that brief verified.

Disposition: adapt the definitions within the original obligations; retain the
consumer-proof hold. Producer implementation, shared consumer routing semantics,
forge authority and security controls are unchanged.

## Row 12: qualify the binary being timed

The old command is green on the original delivery base. Timing coverage over a
corpus without executing v2 records does not establish that the new reader is
present. Comparing the base to itself remains green; prefixing row 11 instead
borrows another test's absence. Neither is used here.

The command builds the candidate from the tree under test and the reference
from the same v1.0.52 release. The harness runs from immutable helper commit
`b12abbe425d9c031e9094bddbe7ab20d4abb75e3`, with explicit binary paths and the
absolute corpus root. The identical harness therefore tests head and historical
base; a missing newly added test cannot manufacture the base failure.

The helper reuses the existing real-Git v2 fixture, reader and CLI ports. Both
binaries must parse the same nonempty fixture. The reference refuses its v2
claims; the candidate releases every claim and reports the expected v2 schema
records. Five plain-output candidate executions on that fixture are timed;
every sample must print the exact successful claim count. An empty, held or
schema-blind fast path fails qualification.

Fixture timings are separate. The original complete-corpus budget still runs
in plain `--coverage` mode: five successful samples per binary, both medians
printed and new median at most 1.10 times the fixed reference median. No fixture
replaces that corpus, no failed process becomes a timing sample and no threshold
is relaxed. The overall test deadline also bounds each child measurement. The
`witnessperf` build tag separates this acceptance measurement from ordinary
unit-suite runs; the Verify command explicitly selects it, and missing binary
or corpus inputs fail. The harness captures the invocation environment before
TestMain installs fixture authority and uses that snapshot for corpus children;
only the isolated v2 fixture uses fixture authority. A real-child boundary test
checks both arms. Removing the explicit corpus environment is caught by a
planted mutation: the child prints the fixture marker instead of the invocation
marker.

The actual delivery-base binary from
`c288fd881e7fe5532614ad25a4610be7c6acf8a3`, passed to this same harness, fails
on an actual v2 claim held at `witness=[w2]`. The fixed release fails the same
candidate qualification. These are functional refusals, not missing-file or
missing-test failures. The head's first full-corpus run hit its 180-second
execution limit; that is an incomplete measurement, never a performance pass.
The final bounded result and hashes belong in the implementation PR's current
workpad. These reproductions supply no independent verification Evidence.

## Row 23: expose the immutable delivery's real result

The command now names producer delivery #4409 and its exact first parent:

```sh
statusgen --root . --consumers --brief statusgen/36 \
  --delivery 3b2235af1af6dd42118cbf975eb4ac2b8e8ab409 \
  --base c288fd881e7fe5532614ad25a4610be7c6acf8a3
```

At the pickup source the actual report is:

```text
summary: 7 corroborated, 1 disproved, 1 unchecked, 0 brief(s) claiming nothing
exit=1
```

This fails the retained exit-0 Expect. The command neither accepts that failing
summary nor invents successful future counts. Exact counts, including unchecked
dispositions, remain a review obligation under `docs/consumer-delivery-checks.md`.
A future passing report must still exit 0 with no disproved route. This draft
cannot support final acceptance while the report remains red.

The disproved declaration names `statusgen/autoflip.go` and
`statusgen/reconcilefold.go` as `fixed-here`. Neither changes in the delivery.
Both already call `evaluateCoverage` at that snapshot; the delivery changes its
adapter in `statusgen/coverage.go` and its shared library. That demonstrates an
indirect behavioral dependency, not the changed-path evidence promised by
`fixed-here`.

Every existing disposition was considered. `fixed-here` requires a changed path.
`follow-up` promises deferred work to a real brief with a back-reference, rather
than proving existing adoption. `out-of-scope` with a substantive reason is an
unchecked reviewer judgment. Historical mode reads delivered declarations;
changing today's frontmatter cannot correct this historical assertion.

The remaining repair needs an explicitly reviewed contract: retain the original
delivery and parent, identify the exact incorrect declaration, prove the existing
callers reach the changed owner and observe its v2 result, and retain every
unchecked review obligation. Negative controls must include an unchanged caller
still reaching the old evaluator, an absent caller and a wrong producer delivery.
Each must fail; path presence or prose alone is insufficient. A later source
change cannot provide historical proof. The checker continues reporting this
contradiction until such a disposition has authority. This draft adds no generic
indirect-call exception.

## Exact approval order

The original #5689 approval remains link 1:

```text
Verification-Table: delivery=3b2235af1af6dd42118cbf975eb4ac2b8e8ab409 base=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 original=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1
Acceptance-Transition: link=1 amendment=37872cfb9961db6ad8825d0b628e6a2793fb3d21 approval=https://github.com/medici-finance/assay/issues/5684#issuecomment-6714773562
```

This protected-table baseline is the divergence baseline. The consumer check
separately requires the delivering merge's first parent; they are different
contracts and cannot be substituted for each other.

This PR must be merged with a two-parent merge commit, never squashed or rebased:
the helper pin is a branch commit and must remain reachable in a fresh mainline
clone. This is a requirement on the driver's merge act, not merge authority for
the worker. After review and human merge, verify-desk identifies the actual immutable
first-parent amendment and drafts link 2 for the driver. Its predecessor is
`37872cfb9961db6ad8825d0b628e6a2793fb3d21`; changed rows in table order are
`12:Command,23:Command`. The driver posts the exact unedited v2 payload on the
marked #5684 issue per `docs/protected-paths.md`. The branch head is not a
substitute for that landed amendment. No approval is requested for an unknown
future SHA, and the old approval gains no authority over new cells.

The coordinate-only descriptor then gains the actual link-2 amendment and human
approval URL. Independent verify-desk re-corroborates both approvals with
`--acceptance-forge` and runs every original obligation afresh after the final
amendment, retaining full Evidence history. Row 23's contradiction must be
discharged before any PASS or lifecycle transition. Readiness belongs to
review-desk; merge belongs to the driver.
