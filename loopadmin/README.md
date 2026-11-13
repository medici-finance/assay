# loopadmin

The shared runner contract for model-backed agent launches: one checked protocol that standing desks and workflow stages both use, so provider integrations, crash recovery and cost accounting are implemented once. The protocol is specified in [`spec/loop-admin-runner-v1.md`](../spec/loop-admin-runner-v1.md) and its launch request has a machine-readable shape in [`schemas/loop-admin-runner-v1.json`](../schemas/loop-admin-runner-v1.json). The `execution-profile/v1` extension, which pins a qualified harness, model and backend and states what each lifecycle operation preserves, is specified in [`spec/loop-admin-execution-profile-v1.md`](../spec/loop-admin-execution-profile-v1.md) with its shapes in [`schemas/loop-admin-execution-profile-v1.json`](../schemas/loop-admin-execution-profile-v1.json).

This unit ships the contract, an in-memory fake adapter, an offline conformance kit, the one shared process runner (`process/`) and the execution journal (`journal/`). There is no real provider client and no provider call.

## Layout

| Path | What it is |
|---|---|
| `runner/` | The contract: request and result types, strict decoding, the caller `Client`, the `Adapter` and `Fence` interfaces, usage and capability declarations, credential exclusion. `executionprofile.go` holds the `execution-profile/v1` extension: qualified profiles, requirements, admission before Start and the execution report checked at acceptance. |
| `runner/fake/` | An in-memory adapter with scripted faults: lost launch acknowledgment, definite launch rejection, ignored cancel, a different model than pinned. `Qualify` makes it declare qualified execution profiles and run its own admission; `SubstituteBackend` and `ObserveRestored` script a run on another backend and a partial restoration. |
| `runner/conformance/` | The conformance kit (`RunAll` and one exported case per behaviour), request builders, a reference caller claim check and the fence-bypass fixture; for the execution-profile extension, profile builders, the `ForgedProfiles` client-bypass fixture and the reference `ProfileCaller` check. |
| `testdata/runner/` | Request fixtures: valid standing-desk, valid workflow-stage, a workflow request missing a reference, an unknown field and a future version. |
| `testdata/executionprofile/` | Execution-profile fixtures: a qualified profile, a standing-desk and a workflow-stage requirement, and malformed requirements and profiles (unknown field, null field, bad digest, undefined support value, drifted content). |
| `process/` | The one process runner, moved here from `tools/desk/internal/cellprocess` (graph-execution/26). It runs one harness invocation in its own process group (Unix) or job object (Windows), reaps the whole tree on exit or cancellation, and reports an uncertain cleanup instead of hiding it. A `Runner` exists only through `New(custody)`: every launch is admitted by the injected `Custody` first and its `Admission` is finished with `clean=false` after an uncertain cleanup. A runner with no custodian refuses to launch. `tools/desk/internal/cellrunner` binds the managed Go-cache custodian and is the only `tools/desk` package that imports this one. |
| `journal/` | The execution journal (graph-execution/26, R2 amendment): process facts (invocations keyed by caller namespace, invocation and owner generation; receipts; lifecycle, cancellation, limit, usage and checkpoint events) in the cell's PostgreSQL coordination database, in its own `loopadmin` schema with forward-only embedded migrations under `journal/migrations/` and the `loopadmin.journal_schema` version table. `Intend` commits the launch intent and its receipt in one transaction before a child may start and reconciles by stable request ID; a receipt the journal has lost holds as `unknown` instead of relaunching. Every launch and receipt delivery first compares the current operator incarnation (`spec/operator-incarnation-v1.md`) with the passive activation record: a missing or mismatched record holds the journal paused until the operator path `Activate` records a rotated incarnation with a confirmed execution-group stop. `Open` never migrates; `Migrate` is the operator step. There is no embedded engine and no local fallback. |
| `testdata/mutations.json`, `mutate/` | The mutation gate. |

## Module

`loopadmin` is an independent Go module (`github.com/medici-finance/assay/loopadmin`) with no dependency on the workflow module or the instance store, and the repository root has no `go.work`. Its package closure is the local `perfrec` module, the pinned, unreplaced `golang.org/x/sys` and `golang.org/x/term` the process runner needs, and, for the journal, the pinned PostgreSQL driver `github.com/jackc/pgx/v5` and migration runner `github.com/pressly/goose/v3` with their own dependencies (all at the versions `tools/desk` already pins; risk summary #5929). `TestNoGraphDependency` refuses any other module that provides a package, any replaced module other than `perfrec`, and any other local assay module; `TestNoEmbeddedJournalEngine` refuses any SQLite or other embedded database engine in the closure. A consumer links it by `require` plus a relative `replace`, the way `tools/loopresolve/go.mod` links `cellconfig`.

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
ASSAY_TEST_POSTGRES_DSN=postgresql://... GOWORK=off go test -count=1 ./journal   # needs a disposable PostgreSQL
GOWORK=off go run ./mutate -map testdata/mutations.json
```

The mutation gate copies the module to a scratch directory, applies one textual replacement from the map (the old text must match exactly once), and requires the named test to fail. Every guard the contract tests name has a mutation (236 in the map: the five Verify tests and the three execution-profile tests, plus one or more per review finding, including the source-level class guard `TestNoPayloadInErrors`, the gate's own file confinement, and the runner's custody and route guards `TestRunnerCustodyBoundary` and `TestCacheObservedRoutes`); a test that stays green under its mutation does not guard what it names. A mutation must fail a test, not the build: a mutation that does not compile is reported as an error, not a kill. The journal's PostgreSQL tests fail, never skip, when `ASSAY_TEST_POSTGRES_DSN` is unset; the CI that supplies a disposable PostgreSQL for them is a maintainer change still to land. The module's CI job is staged at `ci/staged-workflows/loopadmin.yml` until a maintainer promotes it; the repository's `ci.yml` build-test job already builds and vets every module it finds.

## What is not here

The supervisor, launch budget brake, control surface and `loop-admin` command that graph-execution/26 also names are not here yet; this module currently carries the runner contract, the moved process runner and the journal the supervisor will write through. Queue selection, graph advancement, role credentials, result approval and operator administration belong to the consumers of this contract. This module only defines and checks the boundary between them and an adapter.
