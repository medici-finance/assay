# Review workflow qualification

desk-tools/37 freezes a local tool-only baseline for the
[approved efficiency specification](desk-workflow-efficiency-spec.md). It supplies
the semantic and workload inputs for desk-tools/38–44; it does not establish a
production performance improvement or activate a new caller.

## Run

From `tools/desk`, with the repository's Go toolchain and dependencies available:

```sh
GOWORK=off go test -count=1 -v -run '^TestWorkflowOracle$' ./internal/reviewtest
ASSAY_REVIEW_REPORT=/tmp/review-cost.json GOWORK=off \
  go test -count=1 -v -run '^TestCostCohort$' ./internal/reviewtest
```

Both named PASS lines must be present. The optional report path is an explicit
local test artifact, not telemetry. The tests never need credentials: subprocess
environments omit inherited tokens, use disposable configuration homes and disable
telemetry. HTTP is answered by an in-memory transport below the production GitHub
adapter; no socket, hostname resolution or live provider is involved. Git uses
disposable local repositories, with hooks, signing and background maintenance off.
The package uses the shared `gitquiet.Run` test runner and retains its explicit
child Git settings; binary cleanup still runs after the suite.
Command test binaries compile once per qualification process, outside measured
intervals. Fixture setup and compilation are excluded and declared in the manifest.

## Semantic oracle

`internal/reviewtest/testdata/oracle.json` contains separately authored literal
expectations. Test adapters project values from production code; they never
derive expectations with another classifier. The four packages remain independent
`package main` commands. Their bridges exist only in `_test.go`; policy extraction
belongs to desk-tools/39.

| Production boundary | What is frozen |
|---|---|
| deskboard `sweepActionsRepo` + existing head read | NEEDS-REVIEW, approved-at-head, degraded per-item reads, failed enumeration; full heads obtained through the second read required today |
| reviewloop `cmdPlan`, `ReadBoard`, `Coalesce`, `Idle` | Actual command stdout: ordered visible rows, printed depths, outward proposals and final idle; internal observations additionally retain full heads, duplicate sweeps and suppressed unresolved heads |
| deskdispatch `prepareReviewRound`, `writeReviewRound` | Actual rendered round/scope text, correctness/security lane independence, duplicate verdicts at one head, required-check hold and failed-read behavior before any claim; a hold emits no assignment |
| issueboard `run` with production GitHub issue enumeration | Open versus empty versus access failure; absent data never becomes a successful empty scan |

The oracle also requires named passes from the existing body-edit command flow,
per-item degradation, latest-check applicability, path-filtered CI, never-ran CI,
superseded-review, declared-head, lane and hold regressions. Missing named tests
fail qualification. Historic unsafe behavior is not a golden contract: missing
heads suppress outward proposals, overflow/failed reads cannot prove idle, and
documented body-only re-verification must retain its current narrow admission.

Literal output expectations check the printed row sequence after normalizing only
column padding, plus printed disposition counts, outward proposals and idle text.
These assertions read captured stdout, independently of the adapter's internal
observations. Full-head and duplicate-sweep assertions remain separate because the
human-facing output abbreviates heads and renders a single sweep.

Negative controls change a head, remove a WAIT row, reorder rows and change depth,
both in internal observations and in captured output. Further controls remove a
printed proposal and change the printed idle verdict. Production Go overlays
replace `cmdPlan`'s row rendering with a placeholder and change the dispatcher's
DELTA scope text. Each overlay builds an actual command test binary and must fail
the same literal output comparator, while internal observations remain intact.
The overlays never edit the checkout. Do not
regenerate expectations from actual output to make a change pass. A changed
expectation requires an explanation against the approved semantic contract.

## Workloads and matching

`testdata/cohorts.json` fixes small cold, repeated steady, concentrated growth,
concurrent, partially unreadable and failed cohorts. Seed 37 identifies the fixed
deterministic fixture; there is no random input generator. Steady cycles reuse
per-slot configuration homes and Git objects; command processes restart. Concurrent
slots have separate homes. Growth reaches the current 100-PR population cap, so
its final idle answer must remain unverifiable even when its visible rows are approved.

Each cycle collects the production board actions and heads, feeds those values
to the planner, prepares a dispatch for every review candidate, runs a fixed local
Git-diff reviewer and scans issues. The board, dispatch and reviewer endpoints
use the same full Git head; all selected candidates contribute their work. The
cost cohorts reject missing printed planner rows and missing dispatch diff
endpoints. No model agent, launcher, claim acquisition or consequential write runs. These local
reviewer costs must never be represented as model-review costs.

For future comparisons run the same oracle, cohorts and supervision on both source
revisions. Match fixture hash, sizes, concurrency, cache treatment, host/toolchain
and binary identity; preserve result and coverage differences. The manifest records
Git revision/dirty state, per-tool binary hashes, fixture hash, Go/host details,
scope-bearing observations, cache state and invocation results. A dirty revision
is diagnostic only; retain the exact source diff as well as binary hashes.

Only the tool-only arm exists here. No plugin executes, and its version field says
so. Caller-only and combined arms must record the actual installed plugin/launcher
versions and qualified supervision policy when desk-tools/43 adopts them. The
proposal in #3325 is not assumed qualified. This fixture does not replace the
generic experiment planning proposed in #3305.

## Cost vector and coverage

| Component | Recorded meaning and limit |
|---|---|
| Own CPU | OS user + system CPU for each direct subprocess, plus parent self CPU where supported. Sum these disjoint own-process measurements once. |
| Descendant/inclusive CPU | Explicit `null`; a direct-child sample is not a complete process tree. desk-tools/38 owns closing this attribution gap. |
| Wall | Cycle start/end envelope and individual intervals. Use interval union for overlapping child coverage, never summed parallel durations. |
| HTTP attempts | Every actual transport attempt from production issue enumeration, including method, path, query, body hash and response status. Typed deskboard/dispatch and issue-policy fakes bypass HTTP, so whole-workflow coverage is partial. |
| Retries / 304s | Preserve every attempt and status 304. Retry lineage is `null` without a client signal; repeated endpoint reads do not prove retries. A transport control checks 200/304/503 preservation. |
| Quota points | Explicit `null`: a fake provider cannot establish actual or estimated billable REST/GraphQL points. Never substitute typed call counts or zero. |
| Invocations and results | Every board, planner, dispatch, Git reviewer and issue invocation, including failed/partial observations. Missing work is not a saving. |

The cost report is intentionally incomplete as a production estimate. It makes
those gaps visible before optimization. No scalar score, currency conversion or
new numerical SLO is defined. A claimed improvement must preserve semantic output,
scope, failure behavior and coverage, and discuss each measured component and any
tradeoff separately. An unknown component prevents a claim about that component.

No production caches, token memoization, audit-tail behavior, read pools or
conditional-read implementations change here. Their production effect is not
qualified by typed fakes. Existing caps, breakers, trust and audit controls remain
owned by their existing paths. Exact production HTTP/child coverage belongs to /38,
shared semantics to /39, combined observations/preparation to /40–42, caller
migration to /43 and qualification/retirement to /44.
