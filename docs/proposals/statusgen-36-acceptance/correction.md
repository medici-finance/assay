# Statusgen/36 correction and remaining hold

Issue #5684. The version-3 draft changes only the Command cells of Verify rows
5, 6, 12 and 23. Every original row, Expect, class and historical Evidence entry
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
consumer-proof hold. The current adapter also gains the existing bounded Git
reader owner described below. The historical producer snapshot, shared consumer
routing semantics, forge authority and security controls are unchanged.

## Required pre-v2 release selection

The unchanged original Expect requires re-pinning the authoring reference to the
latest release that does not contain this brief's delivery. Retaining v1.0.52
left that original obligation unfulfilled. The selection was made before timing
or comparing the candidate against the new reference.

On 2026-11-15, the complete remote bare `vX.Y.Z` tag list contained 108 releases;
all 108 local refs and tag objects matched it. The highest uncontaminated version
is **v1.0.64**. Its annotated tag object is
`638c088ceb1ddee30f69855d6a7e45ec7a9ab2b0`, peeling to
`0f2b208cebc30af31269e7482b8b9159b602867d`. That commit is an ancestor of delivery
base `c288fd881e7fe5532614ad25a4610be7c6acf8a3` and does not contain delivery
`3b2235af1af6dd42118cbf975eb4ac2b8e8ab409`. Every one of the 20 higher versions
contains that delivery. No version was chosen for its timing result.

The Commands verify both the annotated object and peeled commit before building
from the full immutable commit. Rows 5 and 6 are consumers of the same reference
and contain the same original re-pin obligation, so their Commands are corrected
too. Their Expect, class, shell and order remain unchanged. The existing CI
v1.0.52 compatibility anchor remains separately intact; this acceptance amendment
does not change its independently established contract.

The built v1.0.64 binary reads the nonempty v2 fixture but refuses its real v2
claim at `witness=[w2]` when passed as the candidate to the frozen harness. This
functional negative control ran before the new corpus comparison. The earlier
v1.0.52 measurements and its independent refusal control remain historical
observations; none is relabelled as a v1.0.64 result. The five-sample full-corpus
v1.0.64 comparison is still owed, with the unchanged 1.10 limit.

## Row 12: qualify the binary being timed

The old command is green on the original delivery base. Timing coverage over a
corpus without executing v2 records does not establish that the new reader is
present. Comparing the base to itself remains green; prefixing row 11 instead
borrows another test's absence. Neither is used here.

The command builds the candidate from the tree under test and the reference
from v1.0.64, the latest uncontaminated release required by the original Expect (selection proof below). The harness runs from immutable helper commit
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
is relaxed. The overall test deadline also bounds each child measurement. The Command maps
the immutable helper's timed-child failure diagnostic back to the original exit 3;
budget and non-vacuity failures still fail normally. The
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
The exact-head v1.0.64 comparison at `399f67a7ea4e` reached its 900-second
bound with five successful baseline samples and only one candidate sample; the
next timed child was killed. It is incomplete (Command exit 3), with no candidate
median or budget PASS. The result and hashes remain in the implementation PR's
current workpad. These reproductions supply no independent verification Evidence.

The proposed Command's outer test timeout is 1,800 seconds, an execution-capacity
bound rather than a performance criterion. The five v1.0.64 samples above total
665.363731 seconds; ten samples at an allowed 1.00–1.10 ratio need roughly
1,331–1,397 seconds before fixture/setup margin. The former 900-second harness
bound could reject an otherwise acceptable run on this profile. The original
five successful samples per binary, full corpus and 1.10 ratio remain unchanged;
all old timeouts remain incomplete results. No full local rerun is credited by
this capacity correction. Any separate native qualified-profile ceiling still
applies and must never be bypassed.

## Coverage reader ownership

The coverage CLI and direct coverage consumers bypass `run()`, the existing Git
reader owner. Each typed question therefore reopened and qualified the same Git
repository. `evaluateCoverageObserved` now opens the existing reader only when
no caller owns one, defers its close on every return, and otherwise reuses the
caller's reader. All three product callers (coverage CLI, autoflip and reconcile
fold) pass this common boundary. Cross-repository sources retain separate
root-keyed repositories. No cache survives a standalone evaluation, and no
per-operation authority or history-metadata check is removed.

The valid-v2 lifecycle regression fails on the previous source because its Git
reads have no owner. It passes with the correction and checks repeated-call
separation, caller-owner retention and cleanup. A second test reuses a reader,
confirms a valid release, corrupts shallow metadata and requires a held result,
then restores the metadata and requires release again. Another root with the
same brief IDs opens a separate repository. Native Git parity, cross-source
coverage and the existing invocation-count bound also pass. These are worker
checks; independent review and verification remain separate.

A paired diagnostic on the same frozen dirty corpus completed successfully with
byte-identical output: scoped 187.790895 seconds, previous binary 200.094440
seconds. Each issued 2,623 native Git commands; their summed native-process times
were 48.971494 and 56.715594 seconds. The remainder includes process launch/wait
and Go work and is not isolated by that trace. The serial single-run comparison
under changing host load establishes output parity, not a causal speedup or the
five-sample 1.10 budget. That budget still requires an exact-head run on an
isolated runner; the diagnostic does not credit a performance PASS.

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
The delivered declaration is line 36 of brief blob
`051ddc7b265a8d29681c8552b9721019b8a2afcd`. Base and delivery contain the same
caller blobs: `5b7e7576d8f1ebfa0adf62465eb4ece8d7438fb3` for autoflip (call at
line 1094) and `580cf396ff70323b26071d02aada41bf5bf01ca5` for reconcile fold
(call at line 138). The fixed-here rule is `statusgen/consumers.go:34` at both
the delivery and this correction's parent; the delivered disproved branch is
line 802. The current report resolves the first path of the combined prose
entry; independent blob comparison above establishes that both callers are
unchanged. Neither call-site presence receives behavioral acceptance credit.

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
The proof must exercise the original producer's `autoFlipModel` coverage gate
and `treeFoldInputs`/`foldVerified`, observing both actual v2 release and
changed-input refusal. It needs an external immutable harness, so an absent new
test cannot create a historical failure. Controls must also cover the second
caller still using the old evaluator, a wrong base, altered approval/declaration
and stale or foreign proof. Each must fail; path presence or prose alone is insufficient. A later source
change cannot provide historical proof. The checker continues reporting this
contradiction until such a disposition has authority. This draft adds no generic
indirect-call exception.

The bounded design choice, exact source tuple, both caller proof obligations and
negative controls are specified in the draft [historical correction
specification](historical-consumer-proof.md). It grants no checker activation or
acceptance authority; the driver must rule on that design separately.

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
`5:Command,6:Command,12:Command,23:Command`. The driver posts the exact unedited v2 payload on the
marked #5684 issue per `docs/protected-paths.md`. The branch head is not a
substitute for that landed amendment. No approval is requested for an unknown
future SHA, and the old approval gains no authority over new cells.

The coordinate-only descriptor then gains the actual link-2 amendment and human
approval URL. Independent verify-desk re-corroborates both approvals with
`--acceptance-forge` and runs every original obligation afresh after the final
amendment, retaining full Evidence history. Row 23's contradiction must be
discharged before any PASS or lifecycle transition. Readiness belongs to
review-desk; merge belongs to the driver.
