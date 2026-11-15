# loopadmin

The shared runner contract for model-backed agent launches: one checked protocol that standing desks and workflow stages both use, so provider integrations, crash recovery and cost accounting are implemented once. The protocol is specified in [`spec/loop-admin-runner-v1.md`](../spec/loop-admin-runner-v1.md) and its launch request has a machine-readable shape in [`schemas/loop-admin-runner-v1.json`](../schemas/loop-admin-runner-v1.json). The `execution-profile/v1` extension, which pins a qualified harness, model and backend and states what each lifecycle operation preserves, is specified in [`spec/loop-admin-execution-profile-v1.md`](../spec/loop-admin-execution-profile-v1.md) with its shapes in [`schemas/loop-admin-execution-profile-v1.json`](../schemas/loop-admin-execution-profile-v1.json).

This unit ships the contract, an in-memory fake adapter, an offline conformance kit and the one shared process runner (`process/`). There is no real provider client and no provider call.

## Layout

| Path | What it is |
|---|---|
| `runner/` | The contract: request and result types, strict decoding, the caller `Client`, the `Adapter` and `Fence` interfaces, usage and capability declarations, credential exclusion. `executionprofile.go` holds the `execution-profile/v1` extension: qualified profiles, requirements, admission before Start and the execution report checked at acceptance. |
| `runner/fake/` | An in-memory adapter with scripted faults: lost launch acknowledgment, definite launch rejection, ignored cancel, a different model than pinned. `Qualify` makes it declare qualified execution profiles and run its own admission; `SubstituteBackend` and `ObserveRestored` script a run on another backend and a partial restoration. |
| `runner/conformance/` | The conformance kit (`RunAll` and one exported case per behaviour), request builders, a reference caller claim check and the fence-bypass fixture; for the execution-profile extension, profile builders, the `ForgedProfiles` client-bypass fixture and the reference `ProfileCaller` check. |
| `testdata/runner/` | Request fixtures: valid standing-desk, valid workflow-stage, a workflow request missing a reference, an unknown field and a future version. |
| `testdata/executionprofile/` | Execution-profile fixtures: a qualified profile, a standing-desk and a workflow-stage requirement, and malformed requirements and profiles (unknown field, null field, bad digest, undefined support value, drifted content). |
| `process/` | The one process runner, moved here from `tools/desk/internal/cellprocess` (graph-execution/26). It runs one harness invocation in its own process group (Unix) or job object (Windows), reaps the whole tree on exit or cancellation, and reports an uncertain cleanup instead of hiding it. A `Runner` exists only through `New(custody)`: every launch is admitted by the injected `Custody` first and its `Admission` is finished with `clean=false` after an uncertain cleanup. A runner with no custodian refuses to launch. `tools/desk/internal/cellrunner` binds the managed Go-cache custodian and is the only `tools/desk` package that imports this one. |
| `testdata/mutations.json`, `mutate/` | The mutation gate. |

## Module

`loopadmin` is an independent Go module (`github.com/medici-finance/assay/loopadmin`) with no dependency on the workflow module or the instance store, and the repository root has no `go.work`. Its closure is exactly the local `perfrec` module and the pinned, unreplaced `golang.org/x/sys` and `golang.org/x/term` the process runner needs (the versions `tools/desk` already pins); `TestNoGraphDependency` refuses anything else. A consumer links it by `require` plus a relative `replace`, the way `tools/loopresolve/go.mod` links `cellconfig`.

## Use

```go
c := runner.NewClient(adapter, fence)       // fence reads the caller's own claim generation
ref, state, err := c.Start(ctx, req)        // errors.Is(err, runner.ErrReconcileRequired): reconcile before anything else
obs, err := c.Reconcile(ctx, ref)
acc, err := c.AcceptResult(ctx, ref)        // the contract's checks, generation fence first
// the caller then rechecks its OWN authority before acting on acc
```

A new adapter runs the kit by returning a fresh `conformance.Subject` (its adapter plus a fault `Control`) from a `Factory`:

```go
func TestMyAdapter(t *testing.T) { conformance.RunAll(t, newSubject) }
```

## Tests

```sh
cd loopadmin
GOWORK=off go test -count=1 ./...
GOWORK=off go test -count=1 ./process   # the moved runner and its custody boundary
GOWORK=off go run ./mutate -map testdata/mutations.json
```

The mutation gate copies the module to a scratch directory, applies one textual replacement from the map (the old text must match exactly once), and requires the named test to fail. Every guard the contract tests name has a mutation (228 in the map: the five Verify tests and the three execution-profile tests, plus one or more per review finding, including the source-level class guard `TestNoPayloadInErrors`, the gate's own file confinement, and the runner's custody and route guards `TestRunnerCustodyBoundary` and `TestCacheObservedRoutes`); a test that stays green under its mutation does not guard what it names. A mutation must fail a test, not the build: a mutation that does not compile is reported as an error, not a kill. The module's CI job is staged at `ci/staged-workflows/loopadmin.yml` until a maintainer promotes it; the repository's `ci.yml` build-test job already builds and vets every module it finds.

## What is not here

The supervisor, journal, control surface and `loop-admin` command that graph-execution/26 also names are not here yet; this module currently carries the runner contract and the moved process runner. Launch journaling, queue selection, graph advancement, role credentials, result approval and operator administration belong to the consumers of this contract. This module only defines and checks the boundary between them and an adapter.
