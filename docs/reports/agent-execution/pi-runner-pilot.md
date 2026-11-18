# Pi Durable runner pilot (agent-execution/10)

Verdict: **UNKNOWN**

This verdict comes from `loopadmin/testdata/pirunner/results.json`. `TestPiPilotDecisionEvidence` recomputes it from that file, the source manifest, the hashes of the subject files in this tree and the subtests the test sources declare. If the published verdict or any of its reasons disagree with that recomputation, the test fails.

The pilot is an experiment behind the `pirunner` build tag. It is not a supported adapter, it activates no profile or runtime, and it gives no approval for managed use.

## Why UNKNOWN

The behavioural corpus passes on the real Pi path. The conformance kit, restart, effects, authority, stop/observe and accounting groups all pass, and all five negative controls are killed. The verdict is still UNKNOWN, for the reasons below.

- `tuple-not-vetted`: no maintainer has vetted the exact Node, Pi and compiler tuple yet. That vetting is requested on medici-finance/assay#6208. `source-manifest.json` records the tuple as pending.
- `environment-not-approved`: every result here came from a darwin-arm64 workstation. The brief's approved environment is network-off Linux, and no run there has happened yet. The workstation run did use an offline KUBECONFIG, fixture-only state directories, no credentials and no provider call, but network egress was not denied at the host.
- `measurement-coverage-partial`: Pi Durable 1.4.0 does not expose a commit count, so tool-result entries stand in for it. Bridge RSS is sampled once after each run, which is not a continuous peak.

Other evidence limits are recorded but do not block the verdict:

- Pi's usage model has no "unknown cost" value. The adapter therefore never reads Pi usage as spend; see the Pi findings below.
- The adapter declares no `Resume`, `CancelAck`, `Snapshot` or `runner.ProfileDeclarer`. The conformance kit's profile-admission cases therefore pass without exercising the adapter. The `NoProfileDecl` subtest of `TestPiPilotContract` pins that fact.
- The npm 1.4.0 tarballs carry no `gitHead`. They correspond to the inspected Pi revision `8c03fef4d3b7ecf7a9e8cc5b1312ff60ec2be149` only by declared version and the 1.4.0 changelog entry; no byte comparison was made.

A GO needs all three reason codes cleared on a re-run: the tuple vetted, rows 2-9 rerun in the network-off Linux environment, and a measurement that covers commits and peak memory. After that re-run, `results.json` must be regenerated. `cases.json` proves the derivation rejects a fabricated GO for each of these gaps and for every adverse fixture.

## Architecture pickup: adapt

- **Continue.** The critical-path head, graph-execution/20's `loopadmin/runner` API and `conformance` kit, is present and unchanged. The pilot consumes `runner.Adapter`, `runner.Client`, `conformance.RunAll`, `conformance.Control` and `conformance.ModelSwitcher` as they are.
- **Adapt.** The brief names `tools/desk/internal/cellprocess`. That package has since moved into `loopadmin/process` (graph-execution/26's first slice). The deletion map below cites the current path. `loopadmin/supervisor`, `loopadmin/adapters/localharness` and the PostgreSQL journal still do not exist, so every deletion claim below concerns code we could avoid writing, not code to remove.
- **Hold:** nothing. No open PR overlapped these paths at pickup.

## What was built

- `loopadmin/experiments/pirunner/`: the Go `runner.Adapter` plus `conformance.Control` (`adapter.go`), the JSON-lines bridge to one Node child (`bridge.go`) and the independent synthetic effect ledger (`ledger.go`). Every Go file carries `//go:build pirunner`.
- `loopadmin/experiments/pirunner/node/`: the bridge (`src/bridge.ts`) over `@earendil-works/pi-durable` 1.4.0 with its built-in SQLite storage and an in-process faux provider. Locked with `package-lock.json`; `ignore-scripts=true` is set in `.npmrc`.
- `loopadmin/go.mod`: `ignore ./experiments/pirunner/node`. This keeps the Node tree, and any Go source an npm package ships, out of every `./...` pattern in the module.
- `loopadmin/testdata/pirunner/`: `source-manifest.json`, `results.json`, `cases.json` and `negative-controls.json`.

Bridge shape:
- **One child process per host directory.** The child holds an `flock` on `owner.lock`, so a second owner is refused (`ErrOwned`).
- **Durable identity mapping.** Each `(caller, invocation, generation)` gets a map record, written with tmp, fsync, rename and directory fsync. A start writes intent first, then the Pi submission, then the admitted record. Recovery settles an intent-only record as abandoned, and a changed payload cannot reuse an identity.
- **Parking.** The faux model parks on a replay-safe `await_result` tool rather than in a model call. Pi serialises every read through one mutation line (`session.readOnLine`), so a model call held in flight would block observation.
- **Usage.** Runner usage travels opaquely in the final answer. Missing fields stay unknown.

## Corpus results (darwin-arm64, 2026-11-11)

Each group ran as `GOWORK=off go test -tags pirunner -count=1 -v -run "^<group>$" ./experiments/pirunner` from `loopadmin/`. Every group exited 0 with no `--- SKIP` line.

| Group | Cases | What the cases show |
|---|---|---|
| `TestPiPilotContract` | Kit, NoProfileDecl, UnsupportedRefused, PinnedModelAndTools | The full exported kit passes through the real `runner.Client` against the Pi adapter. Request scope, Resume, CancelAck and unknown capability requests are refused, with zero Pi submissions. The actual model is reported, and a silent fallback is rejected (`ErrModelFallback`). |
| `TestPiPilotRestart` | KillMidTool, CrashAfterIntent, CrashAfterSubmit, OneOwner, Corpus | SIGKILL during an unsafe tool, then reopening the same SQLite store, gives one submission across two processes. The unsafe tool ran once and settled as one error result. A crash after intent leaves the start abandoned and never relaunched. A crash after submit leaves exactly one launch, and a repeated start dedupes to it. A second owner is refused. |
| `TestPiPilotEffects` | LostReply, StableIdentity, OutcomesSeparate | A lost reply reconciles to one logical effect. The same action under a new tool-call ID applies once. Tool and session outcomes are reported apart from acceptance. |
| `TestPiPilotAuthority` | ReplayRechecks, LedgerAlone, UpperHookBlocks | A safe tool replayed after kill skips `beforeTool` but is refused at execution by the ledger once the owner changes. With the upper hook disabled, the ledger alone refuses a stale owner. The upper hook blocks before intent. |
| `TestPiPilotStopAndObserve` | AbortHoldsForChild, IgnoredCancel, ObserveReadOnly | An abort with a live background child stays running (unconfirmed) until the child is gone. An ignored cancel is never reported as stopped. Observe and reconcile issue zero submissions and zero model calls. |
| `TestPiPilotAccounting` | PartialCostUnknown, RunningUsageUnknown, PauseSurvivesRestart, ContinuityRefused | Missing cost stays unknown and never becomes zero. Running usage is unknown. A pause survives a restart. A files-only restore that requires conversation continuity is refused. |
| `TestPiPilotDecisionEvidence` | ManifestDereferences, CasesMatchSource, VerdictDerived, FabricatedGORejected | The manifest dereferences the lockfile, its integrity and the installed versions. The verdict and reasons are derived, not asserted. Twenty-two adverse, unknown or truthful fixtures are classified correctly against a positive GO control. |
| `TestPiPilotNegativeControls` | Classifier, Mutations | The classifier tells a kill apart from a build failure, timeout, skip, zero-selected run and wrong assertion. All five mutations are killed by their named assertion. |

### Negative controls

`loopadmin/testdata/pirunner/negative-controls.json` holds the mutation map. The shared `loopadmin/mutate` runner runs untagged `go test ./...`, which would select nothing from this package. The test therefore owns its own runner. It copies `loopadmin` and its relative `../perfrec` dependency, links the pinned `node_modules`, applies one exact-once replacement, rebuilds the bridge with the pinned `tsc`, and runs the named test with `-tags pirunner`.

Before any listed mutation, the runner plants the map's positive control, `positive-control-no-dedupe-guard`. It disables the ledger's duplicate guard in `ledger.go` and must be killed by `TestPiPilotEffects`, subtest `StableIdentity`. If it survives, the harness is broken and the test fails before any mutation verdict is recorded. The control also makes the map a well-formed spec for the repository's mutation-anchor guard.

| Mutation | Target | Killed by |
|---|---|---|
| `no-start-dedupe` | `bridge.ts` start reads no prior map record | `TestPiPilotRestart`, subtest `CrashAfterSubmit`, "want exactly one launch" |
| `no-exec-reauth` | `bridge.ts` `safe_probe` skips the execution-time ledger check | `TestPiPilotAuthority`, subtest `ReplayRechecks` |
| `unstable-effect-id` | `ledger.go` effect identity includes the tool-call ID | `TestPiPilotEffects`, subtest `StableIdentity` |
| `no-unknown-stop-hold` | `bridge.ts` aborted run reports stopped despite a live child | `TestPiPilotStopAndObserve`, subtest `AbortHoldsForChild` |
| `unknown-cost-zeroed` | `bridge.ts` missing cost becomes zero | `TestPiPilotAccounting`, subtest `PartialCostUnknown` |

### Measurements

These figures come from the fixed 100 and 1,000 tool-result fixtures, using faux models and `safe_probe` tools. They characterise this experiment only. They make no capacity, provider-cost or throughput claim.

| Tool results | Entries | Input bytes | Store bytes | WAL bytes | Model calls | Run | Reopen | Bridge RSS |
|---|---|---|---|---|---|---|---|---|
| 100 | 203 | 5,881 | 327,680 | 4,169,472 | 101 | 0.60 s | 0.28 s | 58.8 MB |
| 1,000 | 2,003 | 60,781 | 1,912,832 | 4,181,832 | 1,001 | 2.60 s | 0.27 s | 132.2 MB |

Coverage gaps: commit count is not exposed, and RSS is a single sample after the run. The WAL size is bounded by SQLite's checkpointing, so it is not a growth measure.

## Pi Durable 1.4.0 findings

- Every read goes through `session.readOnLine` and serialises with mutations. A long model call therefore blocks `inspect`/`entries` for the whole conversation. The bridge parks on a tool instead.
- `inspect()` lists only queued or placed submissions. A finished submission is found through its answer.
- An interrupted unsafe tool settles as one error result on reopen and does not rerun. An interrupted safe tool reruns, and its replay skips `beforeTool`. That replay behaviour is why execution-time re-authorisation is mandatory, and the `no-exec-reauth` control proves the ledger check is what catches it.
- Tool cancellation needs `awaitWithContext` from `@earendil-works/chord`; an abort request alone stops nothing. Tools execute as `(args, api, context)`.
- The faux provider always overwrites message usage with a text-length estimate. Pi usage has no unknown-cost value, so Pi's own usage cannot carry the runner contract's unknown.
- `harness.close` waits on live tasks. The Go host gives close 2 s and then kills the child.
- Built-in SQLite storage has no cross-process lock. The fixture's `flock` owner is the only thing preventing two writers. This is not a Pi multi-writer claim.

## Deletion map

These are hypotheses with the evidence held so far. Nothing is deleted by this brief.

| Candidate | Inspected path / symbol | Disposition | Added surface beside the saving |
|---|---|---|---|
| Conversation transcript persistence, follow-up/steer inbox, compaction, stream normalisation | Planned under graph-execution/22 (`docs/streams/graph-execution/brief-22-local-harness-adapter.md`); no code present | **omit** for a Pi runner, delegated to Pi Harness entries | The bridge (`node/src/bridge.ts`, ~560 lines), the Go bridge client (`bridge.go`), Node runtime and npm lock upkeep, Pi schema migrations (1.4.0 migrates SQLite to schema v2 with no rollback) |
| Inner tool intent/result checkpointing, safe replay, nested-task ownership | Pi `pi-durable` `dist/harness/tool.js` (hash in the manifest) | **omit** a parallel inner journal | A smaller identity map store in the bridge. The independent effect ledger is kept, not replaced. |
| Observer that wakes a session to display it | `loopadmin/runner/client.go` `Client.Observe` | **omit** the waking path; read-only observe proven with zero starts or model calls | Bridge `observe` op |
| Session wake/continuation and writer fencing | Vercel World (not run) | **hold**: only a World-backed arm could qualify it | n/a |
| Controller admission, ownership/incarnation, outer launch journal, effect ledger, acceptance | graph-execution/26 journal (planned); the pilot's own `ledger.go` stays independent | **keep** | n/a |
| Process containment and standing launch consumers | `loopadmin/process` `Runner` (moved from `tools/desk/internal/cellprocess`), `tools/desk/internal/cellrunner` | **keep** | The bridge child is a second process the host must contain. The pilot does this with SIGKILL plus reap, plus recorded child PIDs, not with `loopadmin/process` custody. |

None of the claimed omissions needs a second authority, store or driver. The cost is a Node runtime and a pinned npm dependency set with upgrade and vetting duties.

## Vercel durable-harness disposition

Not run. Its development Pi tuple is 1.0.4, not 1.4.0, and the brief forbids assuming compatibility. Its private `pi-world` package is not a public import. Its public Claude and Codex adapters need Vercel Sandbox. Its SIGKILL and frozen-owner tests shaped `TestPiPilotRestart`. This omission does not invalidate the Pi result.

## Proof still owed before any managed use

- **agent-execution/01**: a qualified execution profile. The adapter declares none, so profile admission is unexercised.
- **agent-execution/03-04**: readiness enforcement, and integrated recovery/effect qualification against real effects rather than the synthetic ledger.
- **graph-execution/22**: local harness adapter qualification. Pi would be one optional implementation behind it.
- **graph-execution/26**: the durable launch journal and receipts on the shared store. The pilot's map store is a fixture with explicit loss classification, not that journal.
- Staged CI coverage (`ci/staged-workflows/loopadmin.yml`, job `pirunner`) awaits maintainer promotion. Promotion means admitting Node and the pinned npm set to the runner.

## Running it

See `loopadmin/README.md`, section "Pi runner pilot (`pirunner` tag)". Default `go test ./...` selects nothing from the experiment.

## Effort

Pickup to draft took about one engineering day of agent time, well inside the five-day cap. No scope was cut to fit the cap.
