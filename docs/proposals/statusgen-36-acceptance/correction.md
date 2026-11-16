# Statusgen/36 correction and remaining approval gates

Issue #5684. Version 3 landed in #6524 at
`b961cebd3eb46a1a73a62e2d491be6e13f175502`, changing only Commands 5, 6, 12
and 23. Its exact human link-2 approval and independent performance validation
remain owed. This follow-up repairs the actual current v2 fold consumer and
proposes version 4, changing only row 23's Command and Expect. Every row, class,
other definition and historical Evidence entry survives; no lifecycle is edited.

## Architecture pickup

Follow-up base: `92304bb0841b86df44899a01d0f2c0bd7670d726`. Owners remain
S-evidence-coverage (`statusgen/evaluation`), the coverage adapter, existing
PASS-run parser, reconcile fold and version reader. No new dependency, credential
channel, retrospective authority or generic consumer-checker exception is added.
The existing protected-table chain supports the ordinary amendment described
below. Original DR-statusgen-36 and its approved policy stay unchanged.

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

## Row 23: repair current behavior and preserve disproved history

The strict historical report still uses original delivery
`3b2235af1af6dd42118cbf975eb4ac2b8e8ab409` and its first parent
`c288fd881e7fe5532614ad25a4610be7c6acf8a3`:

```text
summary: 7 corroborated, 1 disproved, 1 unchecked, 0 brief(s) claiming nothing
exit=1
```

The original combined `fixed-here` declaration names `statusgen/autoflip.go` and
`statusgen/reconcilefold.go`. Both caller blobs are unchanged across that
delivery: `5b7e7576d8f1ebfa0adf62465eb4ece8d7438fb3` and
`580cf396ff70323b26071d02aada41bf5bf01ca5`. The delivered brief blob is
`051ddc7b265a8d29681c8552b9721019b8a2afcd`. The harness checks every object,
the first parent, exact declaration text and both original call edges. The
`fixed-here` diff rule (`statusgen/consumers.go`) is unchanged. Historical mode
continues reading historical declarations; this change never makes the false
claim corroborated or changes unchecked routes into passes.

A real fixture exposed missing implementation beyond that inaccurate proxy:
canonical coverage releases two valid v2 records, and `autoFlipModel` consumes
them, but the original and pre-repair `treeFoldInputs`/`foldVerified` discard the
v2 PASS because the legacy parser requires an execution SHA. The identical
API-only behavioral probe fails on the original source at the actual fold
result, with no new production type or missing helper causing compilation red.
Current repaired behavior supplies new acceptance proof, never historical credit.

The canonical successful v2 judgment now retains its already-validated record
landing in an internal, JSON-excluded observation. Reconcile selects the same
latest strict PASS and matches its exact row/record/runner/date/CR set against
those released observations. It uses the verified landing only for the existing
brief-version lookup; informational.commit retains its old informational meaning.
Every canonical content, identity and landing check, Evidence actor check and
lower Verified-cell write guard remains. V1 parsing and observed-read behavior
are unchanged. WitnessInfo.Run is human-readable only; its sole downstream
consumer appends the diagnostic to BriefCell.Witness, without parsing a SHA.
The v2 diagnostic is explicitly `[w2] landing:<full commit>`.

Row 23 is therefore an ordinary protected Command+Expect amendment: replace the
known-false changed-path proxy as acceptance proof with immutable external
behavioral proof of BOTH current callers. This preserves the original adoption
obligation and exercises actual product decisions rather than treating shared
owner presence as adoption. No mandatory generic planner or closure rule requires
the raw historical consumers report to exit zero independently of this row.
The strict report remains visible inside the harness with its exact failed
summary; its known exit 1 is an audit assertion, not the behavioral acceptance.

The immutable helper and behavioral packet are pinned to
`fdfd791e3de6263381c41704b6db9f4c3889986d`. The subject binary binds the clean
subject HEAD through Go build metadata; the helper prints its SHA-256 and packet
SHA-256. The pre-v2 control is pinned to full source
`0f2b208cebc30af31269e7482b8b9159b602867d` (v1.0.64), not a floating tag.

The subject must first release two actual v2 claims through its CLI. The same
frozen test packet then executes `autoFlipModel` and
`treeFoldInputs`/`foldVerified`/`DeriveLifecycle`, including an actual autoflip
write only in an isolated fixture. Both callers must reject changed implicit
source, module, lock and tool-pin inputs; missing, malformed, foreign or unlanded
records; and an older good witness followed by a bad latest witness. No new v1
Dependencies adapter is invented. Each actual caller is independently rewired
through a test-only overlay to the real immutable pre-v2 binary: it must read
the two nonempty v2 claims, refuse them, and fail at that caller's decision.
Compilation failure is explicitly ineligible as semantic fail-first.

Additional internal-invariant tests keep the landing separate from execution
SHA/stamp, keep it off public JSON, reject mixed row/run provenance and preserve
Evidence actor authority. A fixture advances the current brief to version 2 while
the verified landing retains version 1; the fold must report version 1. The
committed `statusgen/consumerflow-mutations.json` has a caught JSON-leak positive
control and four caught mutants: older-good selection, actor-check bypass, mixed
landing and empty-execution-SHA version lookup. These new-type tests are separate from the frozen
cross-revision packet. The subject tree and HEAD must remain clean and unchanged
throughout the bounded 300-second harness. The row uses the existing local
fixture/reader seam and no forge calls or credentials inside the row.

## Exact approval order

The original #5689 approval remains link 1:

```text
Verification-Table: delivery=3b2235af1af6dd42118cbf975eb4ac2b8e8ab409 base=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1 original=8db7d356334ebd5f59dd1213f4e5c9cf69adbcc1
Acceptance-Transition: link=1 amendment=37872cfb9961db6ad8825d0b628e6a2793fb3d21 approval=https://github.com/medici-finance/assay/issues/5684#issuecomment-6714773562
```

The protected divergence baseline is not the consumer delivery's first parent.
Do not substitute those coordinates. Link 2 is the actual landed version-3
amendment `b961cebd3eb46a1a73a62e2d491be6e13f175502`, predecessor
`37872cfb9961db6ad8825d0b628e6a2793fb3d21`, changed cells in table order
`5:Command,6:Command,12:Command,23:Command`. Its exact unedited v2 approval is
still owed from the existing human approval owner on marked issue #5684.

This follow-up must also use a two-parent merge, preserving the immutable helper
commit in a fresh mainline clone. After that human merge, verify-desk must derive
the actual first-parent version-4 amendment and prepare ordinary link 3,
predecessor `b961cebd3eb46a1a73a62e2d491be6e13f175502`, changed cells
`23:Command,23:Expect`. Its final landed subject, exact prior/current definitions
and digest must be bound by the normal protected-paths payload. No approval is
requested for a branch SHA or an unknown merge, and old approvals gain no new
scope. Both links are required; no combined or skipped-link grammar is invented.

Independent verify-desk corroborates each exact approval with the existing
approval-owner opt-in/permission semantics, then reruns all original obligations
and the amended current-behavior row with complete retained Evidence. The
unchanged row-12 1.10/five-sample/full-corpus budget remains unproven until a valid
exact-head isolated-runner result. Review-desk owns readiness and the driver
owns merge. Neither this proposal nor worker test output supplies authority.

The former special-authority proposal in #6546 is superseded by this ordinary
repair/amendment route. No new design ruling, descriptor input or custody adapter
is requested. Its historical text remains traceable in Git; the replacement
[consumer proof note](historical-consumer-proof.md) records the narrower scope.
