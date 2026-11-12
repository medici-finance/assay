# Zero-CI observation reuse

Issue: [#6255](https://github.com/medici-finance/assay/issues/6255).
Baseline: `0665bfb771788b07473c281b8875fc1806dab36d`.

The existing `ChecksAtHead` adapter reads both check runs and commit statuses.
The zero-rollup probe previously called it through two wrappers, discarding half
of each result. The probe now uses one fresh typed observation. Path-filtered
workflows also supply their successful, complete file observation to the remaining
classification in that invocation. Failed or partial reads are not memoized.

CI path filters still read files fresh, even at an unchanged PR key: a base branch
can move outside the persistent risk-file cache key (#2722). Qualified review
collection also checks the observed metadata head against the frozen queue head.
The standalone zero-CI reader retains its direct file read.

## Matched fixture results

`TestActionsZeroCIReadsOnce` runs the production actions collector and GitHub HTTP
adapter against one repository and one PR. It counts actual HTTP attempts,
including conditional requests. The same test compiles on the baseline and fails
its request-count assertions there. All nine normalized action rows are identical
between baseline and candidate; only `openAge` and `approvedAge` are normalized.

| Scenario | Cold baseline → candidate | Unchanged baseline → candidate | Changed baseline → candidate |
|---|---:|---:|---:|
| No workflows or CI evidence | 11 → 9 | 9 → 7 | 8 → 6 |
| External commit status exists | 10 → 8 | 8 → 6 | 8 → 6 |
| Workflow uses a path filter | 14 → 10 | 12 → 10 | 12 → 10 |

The changed phase adds an external status, or changes the file set to match the
workflow filter, while keeping the PR head, update timestamp and queue rollup
unchanged. The former remains `unverified`; the latter becomes
`checks-never-ran`. No mutable CI verdict is cached between sweeps. Each phase
still reads both CI channels. Existing tests cover read failures, truncated file
and check-run lists, skipped checks, unknown workflows, scope, and rate floors.

These figures describe this frozen workload. They are neither a live fleet
wall-time benchmark nor GitHub rate-limit points. Default-branch health, queue
collection, and the conditional HTTP cache remain in the measured sweep.

## Fit with existing work

This extends #4885's bounded read reuse. It implements a local part of the ranked
zero-CI reduction in `docs/desk-workflow-efficiency-spec.md`, using the existing
semantic and typed provider owners. It serves both actions and the shared
review-queue collector (desk-tools/39–40), without changing its observation schema
or activating a new planner consumer. The shared ingestion and exact-source
cache work in forge-neutral/37–43 and the standing selection work in
graph-execution/27 keep their existing scope and approval obligations. Pagination
and retry work (#3096, #4160, #4287, #6041) is separate.

## Validation

From `tools/desk`, with `GOWORK=off`:

- `go test -count=1 ./cmd/deskboard`: PASS.
- `go test -race -count=1 ./cmd/deskboard ./cmd/reviewloop ./internal/reviewflow`: PASS.
- `go vet ./cmd/deskboard`: PASS.
- `go run ./cmd/muhar -spec cmd/deskboard/zeroci-readcost-mutations.json`: healthy
  baseline and caught control; 3 caught, 0 not caught, 0 could-not-mutate.

The mutation controls repeat the typed CI operation, feed persistent risk files
into the CI path filter, and discard the within-classification file observation.
