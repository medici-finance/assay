# Backlog replay before implementation

Candidate: [fixed-delivery verification proposal](README.md). Retrospective design
assessment on 2026-10-26; no brief, Evidence, status, tool implementation or live
infrastructure changed. [Per-target counts](replay.csv) contain all 201 evaluator
comparisons. The deliberately unsuccessful cases are part of the result.

## Frozen population and instruments

Source/board cutoff: `ae2f92ee89ef16f6c215616abcbe50575cbd4987` in public Assay.
Select the existing awaiting queue: **152 briefs, 117 implemented and 35 verified**.
This is a selected backlog, not a sample of all deliveries. All 152 are held by current
coverage; that is not an overall failure rate or evidence that every hold is wrong.
Later main changes and subsequent queue movement are outside this census.
A final freshness read at `e25479a69d69ad678c15253309fcac6b7b0c86b0` found
PR 4571's attribution/backfill predicate fix; the coverage/evaluation code used by this
experiment is unchanged. The proposal routes reuse to that shared owner. No newer queue
counts are inferred from the pinned replay.

Read existing `statusgen --lint --json`, `--coverage --json`, and `--scores --json`
outputs. Lint passed overall; inherited consumer-range problems are notices while new
closure can still be held. Installed statusgen/desk-tools v1.0.63 came from the published
release, source `125611dfbdfb950cf09c003e941c48902803fb1f`; installed/archive binary equality
was checked. The experiment's pure evaluator and adapter are source at the cutoff, not
an assumption that every installed command already supports fixed D.

A temporary, unshipped Go test in an isolated worktree calls the existing
`parseBriefFile`, `observeBrief`, `newMemoRepo(newCoverageRepo(root))` and
`evaluation.EvaluateCoverage(CoverageInput{Revision: target, Briefs: observation})`.
No evaluator, manifest, criteria, witness, protected audit or observation is changed.
No forge approval opt-in is supplied. This deliberately retains current policy holds.

For each queued brief, evaluate its unchanged committed observation against current H.
For the 45 observations with exactly one clean, at-least-12-character recorded execution
token, also evaluate against that W as a **diagnostic**. This selection excludes mixed,
dirty, missing and protected-held witness sets; it is not a general reuse entitlement.
For four manually reviewed delivery candidates, separately evaluate against D. Three
are the original incidents; the fourth is a substantive negative control. D is never
chosen by maximizing pass count. These are replay inputs, not newly issued authoritative
bindings. The current binding writer is not implemented.

Commands executed offline used `KUBECONFIG=/dev/null`, `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, a writable temporary Go cache and temporary build directory. No production
endpoint or cluster was contacted. No lifecycle writer was invoked. Existing recorded
outcomes were inspected with knowledge of their reported results: this is not a blinded
prospective experiment and cannot establish a prevention rate.

## Population results

| Comparison | Brief-target evaluations | Coverage released | Claim counts: pass / fail / missing / error / could-not-check / wrong-revision |
|---|---:|---:|---|
| Current H, unchanged production observation | 152 | 0 | 41 / 77 / 212 / 59 / 190 / 895 |
| Recorded W, diagnostic only | 45 | 13 | 271 / 24 / 4 / 48 / 79 / 10 |
| Independently reviewed D candidates, unchanged policy | 4 | 0 | 0 / 2 / 9 / 0 / 2 / 18 |

In the 45 paired W diagnostics, **251 claims change from wrong-revision to pass**, while
24 failures, 79 could-not-checks, 46 errors and four missing claims remain in those same
states. Two wrong-revision claims become errors when historical criteria can be checked;
10 remain wrong-revision. Changing the subject does not repair those definitions,
conditions or inputs. The 13 released coverage sets comprise 10 implemented and three
verified briefs; they are **not 13 eligible closures**.

The warning is concrete: desktools-v2/22 and /23 become 10/10 and 9/9 coverage-pass at W,
but their original bare consumer commands checked an empty diff. A green observation
against that command is not proof of the intended delivered consumer coverage. Therefore
“freeze the last passing revision” is rejected. Bind the actual delivery and retain the
consumer obligation, not merely its old exit code.

Current consumer-range census: **53 implemented briefs** and another two verified briefs
have a range notice. The 53 implemented rows group as 21 named unpinned, 19 unscoped, and
13 explicit/computed ranges outside the proposed bounded automatic compatibility class.
These are command-shape candidates, not eligibility counts; original-table equivalence
and authority have not been established for all 40 named/unscoped cases. Their existing
queue buckets include environment and human holds as well as worker/desk repair.

At least one broad fallback wrong-revision claim occurs for **84 of 117 implemented**
and **35 of 35 verified** briefs. Go tests and shell wrappers generally get whole
non-bookkeeping repository scope because no production dependency manifest is supplied.
Common first differences include marketplace and harness-management files. This supports
the moving-subject diagnosis, but direct package non-overlap cannot prove a test did not
read those files. Preserve that conservative rule when judging W against D.

## Original incidents and boundary cases

| Case | Observed evidence / experiment | Effect of the proposed model | Work that remains |
|---|---|---|---|
| PR 4559, desktools-v2/23 | Eight original core rows reported pass at W; bare consumer row exit 0 with empty diff. Current coverage: nine wrong-revision. At actual D, current evaluator still refuses reuse of all nine because broad input scope differs | Fixed D gives acceptance a stable subject. Fresh checks on D pass; delivered consumer check has 7 corroborated, 0 disproved, 0 unchecked | Independently admit D, authorize historical subject treatment, obtain adequate evidence on D for the remaining obligations. The W diagnostic is not closure |
| PR 4561, apps-installer/10 | Five functional passes; named consumer checker now obtains 4/0/0 from the committed marker. Current protected observer rejects latest BLOCKED and exposes six missing credited rows. Fresh avatar suite on D passes | Preserve the five observations; distinguish protected eligibility and scope parsing from functional failure. Stable D stops later H churn | Historical attribution, approved compatibility, scope grammar/outcome writing, applicable row evidence and independent final closure |
| PR 4562, desk-tools/38 | Three core rows reported pass in a network-none runner. Changed row 3 is in amendment PR 4414; exact approval remains open on issue 4563. Current observer exposes three missing credited rows after BLOCKED | Fixed D does not approve the command amendment. Preserve observations without claiming approved acceptance | Exact authority, criteria/source separation in the transition audit, then required independent acceptance; do not silently use a source commit newer than D |
| desk-supervision/13 | Immutable delivered snapshot: 2 corroborated, **2 disproved**, 1 unchecked. The claimed deskroster/ackbeacon edits are absent from that delivery's diff | Stays held. Context repair must reveal the substantive gap, not clear it | Deliver the missing promise or approve the appropriate contract correction through existing paths |
| contributor-trust/09 | Fourteen of sixteen rows reported pass. Row 8 expects bare `3`, but original `git grep -n -c` prints a path prefix plus `3`; supplemental `-h -c` prints `3`. Consumer row lacks the needed marker | Keeps the 14 observations; cannot relabel the changed command as the original passing row | Reviewed command/expectation treatment, meaningful consumer result, applicable D evidence and disclosed execution conditions |
| desk-containers/02 | A context/scope repair caused a seven-row repeat although Verify and the three declared implementation files were unchanged. Current evaluator already reuses three simple-command passes, while container/host evidence has distinct environment outcomes | Positive control for reuse already present; no new digest system needed for those three rows | Resolve actual environment/human obligations. Unchanged implementation files do not by themselves validate all seven rows |
| desktools-v2/22 | Recorded 10/10 PASS at W; current ten wrong-revision; diagnostic at W releases all ten, including empty-diff consumer check | Demonstrates why the design cannot pick W or blindly preserve every reported PASS | Bind true D and execute consumer coverage meaningfully |
| agent-execution/01 | Diagnostic recovers three functional passes; the fourth row's wrong explicit range remains fail | Keeps unaffected facts without automatic range reinterpretation | Existing protected correction route, not historical unpinned compatibility |
| forge-neutral/17 | Diagnostic recovers five more passes, but four failed claims remain; actual holds mix runner/tool availability and assertion interpretation | Context/test distinction reports the causes without declaring product correctness | Fix the affected definition/environment/semantic obligations; no blanket meta waiver |

The four explicit delivery-evaluator comparisons include /23, apps-installer/10,
desk-tools/38 and desk-supervision/13. The other table rows are historical record
inspection, existing-output replay and specifically identified command probes; do not
read the table as a fresh full Verify run for every case. `missing` in protected-held
observations means no admissible row was exposed, not that the historical execution was
erased or never occurred. A recorded `fail` can be environment or matcher failure; the
77 failures in the full population are not 77 established product defects.

## Actual fixed-source executions and consumer checks

Own, locked, detached worktrees were made at these delivery commits. They contain their
historical product/test sources; no main-tip source was overlaid.

| Brief | D | First parent |
|---|---|---|
| desktools-v2/23 | `3f15da841717fa597b7473eb8c0efbbd46e594c3` | `90da708cb3a192d0fd32b7b85185c0c3fead5392` |
| apps-installer/10 | `f52610f4e036de6b8e01e16c6f7aadde168b5b25` | `59246f0b7a95c5ac600ef209d6731fd378125e6e` |
| desk-tools/38 | `eded7ba6e4461a3afac436ff7196987d83268bb3` | `da7a4be21e61acf2f8a2c09749f8cec7d60468f1` |
| desk-supervision/13 | `d8552d943f3c0a64d035abd6c80d9b53e93b8d27` | `f4cb66a3444e9bc8519990847e3e634ecd446b6e` |

From deskfleet D's `tools/desk`, this command passed all three named tests:

```sh
go test -count=1 -v ./cmd/deskfleet \
  -run '^(TestCLIHelpOffline|TestCLIConfigFlow|TestCLILegacyForms)$'
```

This covers the named tests in original rows 1–3, not mutation, floor, Windows, registry
or all-package rows. It is a new experimental observation on D, not a closure witness
written into the original brief. The package reported 3.520 seconds.

From avatar D's `tools/desk`, `go test -count=1 ./internal/avatar/` passed (5.961 seconds),
matching row 5's suite. Row 1's binary/output flow and rows 2–4's exact output matchers
were not independently re-executed in this experiment. This is evidence that a bounded
fixed-source run is viable for these examples, not that every historical environment
can be reconstructed or that all obligations already pass.

Existing v1.0.63 immutable consumer checks used this supported command shape from the
full-history cutoff checkout, substituting the D and first-parent pins above:

```sh
statusgen --root . --consumers --brief <brief> --delivery <D> --base <D-first-parent>
```

Results: desktools-v2/23 **7/0/0, exit 0**; apps-installer/10 **4/0/0, exit 0**;
desk-supervision/13 **2/2/1, exit 1** (corroborated/disproved/unchecked). These use the
existing snapshot checker, not a new checker and not a forged historical original-row
witness. Only declared routes are visible to that instrument; omissions still require
review.

Seven existing statusgen tests were rerun against unchanged cutoff code and passed:
`TestCoverageAncestorWitnessOtherPathChangedMismatch`,
`TestCoverageExpectChangedSinceWitnessRan`,
`TestCoverageDeclaredScopeExemptsVerifyBookkeeping`,
`TestCoverageReuseDoesNotRetargetPass`, `TestCoverageEnvDependency`,
`TestCoverageManifestScopesReuse`, and `TestCoverageNothingDeclaredStaysConservative`.
They establish existing scope/criteria/environment refusal and bookkeeping-reuse behavior.
They do not establish future trusted-binding admission or the redesigned full lifecycle.

## What the experiment accepts and rejects

Supported: choose an independently bound D, keep it fixed across later recording/main
changes, reuse the current evaluator and immutable consumer checker, retain independently
applicable observations, and separate H health from delivery completion. Real missing
routes, wrong criteria, missing authority and environment gaps continue to hold. Historical
migration must distinguish these classes instead of promising a bulk unblock.

Rejected: freeze a convenient green W; let a new tag alone solve lifecycle acceptance;
auto-credit old empty consumer checks; treat all “meta” holds as irrelevant; narrow test
scope to touched files; claim 40 compatibility candidates or 13 diagnostic releases as
confirmed backlog closures; or claim two headings solve the runtime problem.

**Measured automatic closures: zero.** No proposed policy was activated. The useful result
is a finite route for each case: establish D once, preserve the real observations, repair
only the affected context/criteria, and obtain the remaining proof on D. The experiment
cannot give an honest final migration throughput estimate without completing historical
attribution and applicability assessment across the remaining cases.

Still untested and required in the existing implementation's acceptance: trusted binder
custody and forgery refusal; historical replay using genuinely available provenance;
D/A admission through every production caller; executing amended approved criteria on
older D; E/H advancement without superseding D observations; and a real main regression
remaining visible after D acceptance completes. These are named gaps, not implemented
features or reasons to add a new verification system.

For local audit, raw unmodified evaluator JSON SHA-256 is
`b4ccce2f2dfa2fbf175e5480ab2ad2b1a05959bba70dd08b79504c9c3df1cde6`;
experiment-only Go harness SHA-256 is
`c1d220a2843231d8280455ecb2eb7577f5839bb0f413faf5b3f6d38aabf67d11`.
Raw execution/config logs are retained locally; the CSV contains public result counts
without operator configuration. The experiment harness is not installed, committed as a
test, or a proposed recurring mechanism.
