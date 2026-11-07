---
brief: assay:assay:forge-neutral:54
title: Canonical PostgreSQL refresh gate and admitted read-through integration
why: "Read-through needs one durable refresh owner across processes, with native attempts bounded before they happen and observations committed only by the current generation. Joining the existing source, coordinator and PostgreSQL Store closes this gap without waiting for workflow execution."
wave: 6
depends: ["forge-neutral/53", "forge-neutral/52", "forge-neutral/51"]
unblocks: ["forge-neutral/37", "forge-neutral/55"]
effort: L
gate: human
decision-issue: 5411
ruling: "https://github.com/medici-finance/assay/issues/5411#issuecomment-6672310072"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The adapter controls durable private-observation refreshes and shared work budgets. Accept only the demonstrated synthetic PostgreSQL/native integration; retention, real policy initialization, credentials, remote exposure and activation remain separately gated."
issues: []
schema: brief-v2
version: 9
authored: 2026-11-03 by statusgen newbrief
outcome: none
design: DR-forge-library
decision-trigger: spec
split-from: forge-neutral/37
exec-tier: strong
domain: complicated
exec-tier-why: "Cross-process generation fencing, SQL budget debits, atomic Store completion and query-qualified backoff must survive bypass, cancellation and restart."
sources:
- "authoritative contract source: commit cb63c1f814ac55b0f01d0789a346fd2f884f16bf; this authoring adoption carries no runtime implementation or acceptance claim"
- docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
- docs/streams/forge-neutral/brief-51-store-injected-mirror-read-through-coordinator-and-shared-refresh-gate-port.md
- docs/streams/forge-neutral/brief-52-canonical-postgresql-mirror-store-without-execution-dependencies.md
- docs/streams/forge-neutral/brief-53-qualified-native-mirror-attempt-accounting-and-retry-metadata.md
- spec/coordination-store-v1.md
- docs/library-first.md
- docs/go-engineering.md
- "freshness-checked 2026-11-03 main 214b4c8308b3cc57beed91feccf45e6a03f2eecd; foundation 4d028e05913d66067ad7ee8518422b3390790c8f, Store52 e7738328b614d8616cef35c8a91e822b16844d64, mapper 2677323c9bdf81aad1a1e3b4ad4407e0703ee25c; final repaired51 and actual53 must be consumed at pickup"
consumers:
- "workflow/store and its canonical numbered Goose migrations: follow-up forge-neutral/54 (implementation owner) (Gate, AttemptAuthority and transaction-bound Store adapter)"
- "workflow/internal/pgtest and workflow/pgtestfixture: follow-up forge-neutral/54 (implementation owner) (single canonical harness; thin test-support export only)"
- "tools/desk/internal/deskkit admitted source/coordinator integration fixture: follow-up forge-neutral/54 (implementation owner)"
- "forgeread/refresh Gate/Requirements/Outcome and mirror semantics: follow-up forge-neutral/51 (consume its repaired API; no competing kernel)"
- "spec/coordination-store-v1.md, docs/contracts.md, workflow/README.md and actual CI registry/workflows: follow-up forge-neutral/54 (implementation owner)"
- "authenticated read-only serving: follow-up forge-neutral/56 (consume canonical Gate, no second refresh owner)"
- "remaining authenticated executor/effect serving: follow-up forge-neutral/40 (consume56 read-only handler and57 producer)"
- "retention, scheduler, caller cutover and full joined acceptance: follow-up forge-neutral/37"
- "workflow execution/admission/claims/outbox and global reconstruction: out-of-scope (remain graph-execution/19)"
---

# Brief 54 — Canonical PostgreSQL refresh gate and admitted read-through integration

## Context

files:
- `workflow/store/` (planned)
- `workflow/store/migrations/` (planned)
- `workflow/internal/pgtest/` (planned)
- `workflow/internal/pgtest/fixture_error_test.go` (planned)
- `workflow/internal/refreshcodec/` (planned)
- `workflow/pgtestfixture/fixture.go` (planned)
- `workflow/pgtestfixture/fixture_test.go` (planned)
- `workflow/README.md` (planned)
- `workflow/go.mod` (planned)
- `workflow/go.sum` (planned)
- `tools/desk/go.mod`
- `tools/desk/go.sum`
- `tools/desk/internal/deskkit/mirror_joined_test.go` (planned)
- `tools/desk/internal/deskkit/citrigger_test.go`
- `spec/coordination-store-v1.md` (planned)
- `docs/contracts.md`
- `.github/workflows/assay-workflow.yml` (planned)
- `changelog/mirror-postgres-refresh-gate.md` (planned)

single-point-of-failure: the canonical SQL transaction binding the current refresh generation, Store update and completion — independent SQL version/coverage constraints reject malformed observations, while the native pre-request authority/closed-operation guard bounds provider work even when coordination planning is bypassed. No gate ownership grants data permission.

facts:
- The canonical disposable fixture uses a finite 30-second Close-only cleanup budget so a forced PostgreSQL checkpoint can finish after strict ownership validation. Startup remains bounded to 10 seconds; Reset retains its explicit caller context. Exact generated name, database owner, marker and forced DROP checks remain unchanged, with no retry, fallback or ignored cleanup failure. Safe SQLSTATE/context diagnostics disclose no raw PostgreSQL message or connection value. This is test-harness completion time, not an R2/auth/lease policy.
- Store52 uses the canonical private DB pool, one Goose ledger and migration00002. Its public Apply owns its transaction and explicitly supplies no generation fence. Refactor private helpers in that owner; do not wrap naked Apply in an unrelated gate transaction.
- Required51 pickup supplies bounded observation Requirements on Demand and a checked post-requery Leave Outcome bound to actual snapshot version. Base FillKey remains projection-independent. Owner-only completion cannot classify a stronger joined query. Consume the actual published repair before implementation; no current working API is assumed delivered.
- Source53 supplies scoped grant/BeforeAttempt/ObserveAttempt and typed receipt semantics. Existing page/byte counts or post-hoc HTTP totals cannot prevent an overspend. The canonical authority must validate/debit before every native base RoundTrip, including SDK retries.
- Actual mapper50 is internal to tools/desk/internal/deskkit; Go forbids importing it from workflow tests. Keep the joined fixture there and reuse the existing canonical PostgreSQL test harness through a thin test-support export, not a copied fixture.
- Full37 retains all23 Verify pairs and its full joined row23; fullGE19 retains13. This bounded contribution provides concrete persisted read-through evidence, not complete retention, conditional/review/event/scheduler/service or parent acceptance.

layering: Extend only workflow/store and its existing migration ledger. Implement refresh.Gate and53 AttemptAuthority using that same PostgreSQL owner. Consume the production kernel/coordinator/mapper. Keep the real joined fixture inside deskkit; the pure forgeread root and refresh/kernel retain no SQL or credential implementation. No local fallback, second quota owner, provider walker or execution subsystem.

design-fit:
  owner: canonical R2 workflow/store for refresh ownership, work-budget debits and persistence
  contract: S-coordination-store and S-forge-mirror; implemented refresh tables replace only the explicitly planned refresh portion
  retires: []
  weight: "verbs0; flags0; backends0; ledgers0; drivers0; one canonical shared Gate/attempt adapter"
  why-add: "The existing typed port and SQL Store have no common transaction yet. Bind them once in R2 instead of adding per-client locks, retries or a mirror database."

## Read first and pickup

Read actual36/50/51/52/53 source, this schema contract,37, DR-forge-library, library-first and Go engineering. Record immutable revisions and continue/adapt/hold. Canonical dispatch requires the published53 contract and final repaired51 source. SQL runtime pickup may then genuinely consume a coherent immutable tested53 shared-port/coordinator-propagation commit supplied by its actual owner while native implementation continues. It must compile and pass the declared port/propagation controls; authoring-only declarations or copied types do not qualify. Final joined evidence and delivery/publication must consume the completed qualified native53 source. Genuine tested feature merges may supply canonical36/52/53 and published mapper/coordinator source before mainline delivery; a stack grants no acceptance. A missing module, schema, fixture or port is NEEDS_CONTEXT, never permission for another store or guessed source API. Authoring and SQL preparation can proceed in parallel with53; runtime pickup follows its actual tested port source, without any execution/claim/outbox dependency.

## Completion clock and consumer binding

`DB.Refresh` requires an explicit trusted `refresh.Clock`, shared in composition
with the native source and coordinator. It validates decoded-data bounds and
supplies `ApplyCompletion` query time in that application clock domain, sampled
after SQL I/O and rechecked before commit. Nil, typed-nil and zero-time clocks
refuse before SQL activity; invalid later samples cannot commit. No default clock,
skew tolerance, policy duration or serialized profile field is introduced.
PostgreSQL remains the only authority for lease expiry, windows, generation
publication and retry floors. Original observation timestamps are never restamped
at database/report time, and missing or future bounds still refuse promotion.

Actual API consumers are the workflow Store/process/schema tests,
`workflow/pgtestfixture/fixture_test.go` and the deskkit persisted native join.
The operator-provisioned runtime join owned by forge-neutral/55 must genuinely
consume this constructor change and supply the same explicit data clock; its
known-accounting profile codec remains unchanged. Planned forge-neutral/56 serving
must bind its admitted source, coordinator and canonical Gate to that trusted data
clock. These are composition requirements, not production clock/identity selection
or authorization. A held/done completion locks and fences only the current scope
identity/version without decoding observations or altering coverage.

`NativeLimits.Reservations` is a lifetime retained-ledger cap per source identity,
including observed entries. It is independent of per-window charges and concurrent
in-flight capacity. Reaching it holds further reservations across windows/restarts;
there is no runtime pruning, reset or automatic repair. Any future bounded archival
or reconstruction needs its own explicit canonical accounting contract.

## Human decision

Accept or hold the demonstrated synthetic persistent read-through integration. One PostgreSQL owner coordinates refresh generations, bounded request attempts and query failures, and commits observations with completion. It uses only disposable local containers and synthetic admitted transports. Acceptance does not initialize production budgets, authorize private copying or deletion, expose a service, select credentials or activate execution.

Options:
1. Accept the bounded contribution, retaining separate production policy, retention, custody, exposure and activation gates.
2. Hold for a named correction to fencing, authorization, accounting, fairness or persistence behavior.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic fixture qualification may proceed.

## Ground rules

- Own isolated branch/draft; no main push/merge, ready flip, release, workflow dispatch, live provider/cluster/operational database contact. Explicit verified disposable Docker PostgreSQL only; KUBECONFIG=/dev/null.
- Reuse selected pgx/Goose, private canonical pool, numbered forward-only migrations and fixture ownership checks. No second driver, ledger, provider walker, quota policy, local coalescer, default DSN or backend fallback.
- No retention action, scheduler, HTTP/wire service, new credential/identity, client fallback, execution claim/admission/outbox or activation API. Known synthetic fixture initialization is not production policy activation.
- Preserve every parent Task,23/13 Verify pair and gate. Do not mark full37 or fullGE19 implemented. Stop at implemented; independent verification owns Evidence.

## Task

1. Add canonical refresh ownership, waiter, requirement-failure and accounting tables through the existing Goose manifest. Document exact implemented columns/keys/checks/floors; leave execution and reconstruction schemas planned. Separate compatible base-fill identity from bounded Requirements failure state, per-caller accounting and source/native-attempt budget. Persist full scope/source/admission/access/configuration/policy bindings; a hash, request key or epoch string is no authority.
2. Implement the actual repaired refresh.Gate. Acquire atomically joins an equivalent current base fill or creates one admitted owner generation within finite trusted active/window/wait/failure bounds. No process map owns coalescing or cooldown. A missing/unreadable/uninitialized shared usage or policy record holds; never infer zero from a cache miss, new client, epoch change or restart. An explicit known synthetic seed is allowed only in the owned fixture. Production initialization/repair remains operator-owned and unactivated. Do not create a reconstruction or blanket reset API. Shared profiles require `Limits.Refresh.FailureBackoff == 0`: the Gate is the sole shared retry authority. Reject nonzero values before I/O; do not normalize them or change the standalone mirror planner.
3. Implement53 AttemptAuthority in the same R2 owner. Each opaque grant binds the exact admitted source operation/repository/kind and current owner generation/policy. Before every native base RoundTrip, atomically check current generation, expiry, trusted caller/admission/source budget/window and finite per-fill ceiling, then debit/reserve the attempt and return a unique canonical per-attempt identity. ObserveAttempt records only that reservation's safe typed evidence after the existing bounded body read and before SDK return, and publishes retry floor/unknown hold atomically. Until publication it remains finitely in-flight; consumed-prefix bytes and full-response knowledge stay distinct. There is no byte quota or two-stage report API. BeforeAttempt reads that durable floor/hold; later authorizations refuse, while already-authorized requests remain counted in-flight within explicit finite trusted concurrency/deadline bounds. No mandatory one-inflight domain policy or production threshold is invented. Source constructor and native closed-operation checks independently bind actual request before this call. Unknown/failed/uncertain authorization refuses the base call and preserves the conservative debit/hold; no guessed0 or refund. Distinguish authorized budget slots from measured own HTTP attempts and provider points. Post-hoc counts alone never grant an allowance. No new global quota policy or production threshold is selected; use explicit finite trusted profiles.
4. Factor52's private Store helpers so a private transaction-bound mirror.Store can run the existing typed ApplyCompletion seam. Gate.Complete must reject expired/stale owner generation before any Store.Apply even when ExpectedVersion still matches, verify the supplied Store is its actual bound canonical backend, independently revalidate current scope/caller epochs/admission, and commit rows, coverage, version, ownership, failure and completion in ONE SQL transaction. No generic transaction callback or caller SQL handle is exposed. Failure/caller denial cannot promote coverage. A rollback/unknown acknowledgement is not permission to replay. Retain52 direct admission and independent SQL coverage/CAS guards.
5. Persist individual actual retry observations from53 before relying on a final Completion receipt. Unknown/missing/uncertain reports remain conservative in-flight/held; no final aggregate can invent missing observations. Per-attempt native evidence is distinct from post-hoc fill metrics. Persist actual retry metadata from53. Failure eligibility uses the maximum trusted cooldown and qualified provider floor. Known no-observed-constraint and a fully qualified controlled no-response transport failure retain their distinct meanings; lost/invalid/unknown response timing holds, not a blind fixed retry. Any same-invocation repeated native entry must honor its newly observed floor before BeforeAttempt; cross-fill visibility comes from the actual ObserveAttempt transaction, not leaf-local state alone. A prior owner with unknown native completion cannot regain an allowance through lease expiry, process restart or failure-entry eviction. Reconcile only from canonical qualified evidence; do not fabricate it. Source failures and stronger-query failures are separate from caller quota/admission refusal.
6. Implement bounded wait/leave and51's typed post-requery Outcome. A checked unfulfilled Requirements outcome records shared query-qualified backoff only for the actual ticket/current generation/current snapshot version; a later sufficient result clears only its relevant failure. Compatible projections keep one base fill. Cancellation, deadline, revoked epoch/admission and unchecked outcomes only release accounting, never poison another eligible caller. Late old-generation/version outcomes cannot poison a later recovery. Preserve final authorization/deadline checking after Leave I/O. Unknown completion retains an explicit held state.
7. Enforce configured fair finite caller/admission active fills, waiters, attempt windows and retained failure entries. CallerA saturation cannot starve eligibleB under the declared fixture/policy rule or createB's failure. Never evict an unexpired failure into a fresh allowance. Waiter cancellation releases its own durable debit without canceling other demand; owner lifetime/deadline and expired-owner recovery are bounded and fenced. A disappeared process leaks no permanent active/queue allowance, but unknown source/budget completion is held rather than reset. Distinguish expiry with proved no issued attempts from an unknown in-flight attempt.
8. Build a real joined fixture inside deskkit using the completed qualified53 production admitted native SDK reads → mapper50 → coordinator51/53 → SQL Gate/Store → QueryScope, with synthetic transports and the explicit disposable PostgreSQL container. Reuse the canonical pgtest implementation through a thin explicitly test-support export. The deskkit-to-workflow first-party edge needs its risk assessment before any manifest change. No production consumer/cutover entrypoint or new server is added. Test ownership/teardown keeps one harness and only owned databases/resources. Ordinary required desk shards have no PostgreSQL service: when the explicit test DSN is absent, only the outer deskkit joined test skips before invoking the canonical fixture and labels that result unqualified. The declared dedicated assay-workflow PostgreSQL job must set a nonempty DSN, invoke the joined selector with verbose output and require its exact named PASS. Missing, malformed, unreachable or skipped qualifying fixture/test is failure. The canonical pgtest harness itself never skips, and no ci.yml/shard change or build tag is required by this mechanism.
9. Run the real cross-process/restart, direct-SQL bypass, rollback, missing-schema/unknown-usage and joined request-count cases. Plant actual production mutations for generation predicate, common transaction, lower attempt authority, shared budget, retry floor/unknown hold and requirement failure fencing; restore exact bytes. Compile/setup failures do not count. Extend schema bidirectional checks, README, semantic owner and actual CI registry/workflow inputs for the kernel/coordinator/source/fixture dependencies. Workflow publication inherits the existing human-only permission hold; no narrowed retry or identity/scope change.

## Verify

Qualifying named tests require the explicit verified disposable container. Missing test/fixture or a qualifying skip fails. The ordinary no-DSN desk-shard outer skip is explicitly unqualified; it supplies no Verify evidence. All provider traffic is synthetic; a client-process restart is not a server-restart claim.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateAtomicCompletion$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateAtomicCompletion ' "$out")` | Named PASS; current generation+transaction-bound Store+completion commit once. Stale/taken-over owner with still-matching ExpectedVersion cannot change rows/coverage; foreign Store refuses. Inject error after row staging; reopen sees whole old snapshot. Remove production generation predicate/common transaction and named control fails. |
| 2 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateProcesses$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateProcesses ' "$out")` | Named PASS; independent processes share only SQL, acquire one owner, join others, persist failure and survive client restart. Proved0-issued-attempt expiry can recover safely; unknown native completion holds. No per-process allowance or stale generation publish; removing durable fence fails. |
| 3 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshAttemptAuthority$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshAttemptAuthority ' "$out")` | Named PASS; actual SQL authorizes/debits unique attempts before native calls, under simultaneous caller/admission/source/window/concurrency ceilings. ObserveAttempt publishes floor/unknown atomically: later authorization refuses, earlier in-flight reservations stay truthfully accounted. Individual duplicate/foreign/out-of-order/missing/uncertain reports cannot close another slot or be invented by aggregate completion. GrantA cannot spend onB/repo/kind/operation; expired/taken-over/unknown/full budget refuses. Post-hoc totals cannot overspend; bypass upper planner and remove lower SQL debit/predicate to fail. No refund/reset on uncertainty. Individual observation identities cannot be replaced by a final aggregate Completion; duplicate, foreign or reordered reports cannot close/refund another slot or weaken a later floor. A qualified late report cannot complete a stale refresh or clear later state. |
| 4 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRequirementBackoff$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshRequirementBackoff ' "$out")` | Named PASS; base-only owner and stronger joined Reviews demand share one base fill; stronger immediate repeat is held. Checked Outcome binds generation+snapshot version; late outcome cannot poison recovery. Cancel/deadline/current refusal are unchecked and do not poison. Known provider floor wins; unknown/lost retry timing holds. Removing requirement/floor/late-outcome guards fails. |
| 5 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshCapacityCancellation$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshCapacityCancellation ' "$out")` | Named PASS; explicit finite fair caller/admission/window/failure limits; eligibleB progresses whileA saturates its own quota. Cancellation releases own waiter; owner stays bounded. Unexpired failure cannot be evicted into allowance; absent/unreadable/unknown accounting holds. Restart/epoch change does not reset usage. |
| 6 | check +flow +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorPersistedReadThroughJoin$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorPersistedReadThroughJoin ' "$out")` | Named PASS; production native SDK+mapper+coordinator+SQL Gate/Store flow.50 compatible readers share one paginated inventory; exact own base RoundTrips/pagination counted once under actual pinned no-hidden-retry SDK configuration, warm query makes0. Repeated native entries remain guarded; stale HTTP-cache replay cannot fabricate fresh completeness. Changed/empty/partial/oversize/transport/known-rate-floor/unknown-retry/caller-denial cases preserve prior watermarks, presence and truthful unavailable. Independent admissionB and stronger requirements do not share authorization/failure incorrectly. Removing source/gate/store join or lower request guard fails. This concrete evidence does not satisfy every full37 row23 condition. |
| 7 | check +dereference +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshSchemaBoundary$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshSchemaBoundary ' "$out")` | Named PASS; actual schema/manifest/ledger matches implemented contract both ways. Invalid raw SQL bypassing Go cannot create an unfenced completion or fake a fresh budget. Missing/extra column/table, competing ledger/backend, public generic transaction API or missing actual CI input fails; no guard weakening. |
| 8 | check | `cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./store` | Exit0 against explicit container; inherited foundation/Store controls remain live; no database skips count. |
| 9 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/54 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Actual delivery corroborates implemented paths; out-of-scope/remaining-parent obligations stay unchecked, not PASS. Post-merge use independent immutable delivery bounds. |

## Substitution limits

This replaces51's fake Gate and52's naked-Apply substitution only for the demonstrated bounded read-refresh flow. It is not the retention implementation, all full37 joined row23 conditions, fullR2 execution/reconstruction or a production quota/policy activation. The actual admitted transport is exercised only with synthetic provider responses. No live credential, scope or endpoint was accessed or granted. Service contracts, authenticated remote serving, full review/conditional/event/poll support and tool cutover retain their existing owners and gates.

## Evidence

<!-- Independent verifier only. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateAtomicCompletion$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateAtomicCompletion ' "$out")` | pass exit=0 | sha256:2ca5ba6244a2 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateProcesses$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateProcesses ' "$out")` | pass exit=0 | sha256:d43270459c03 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshAttemptAuthority$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshAttemptAuthority ' "$out")` | pass exit=0 | sha256:62b16ef712e9 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRequirementBackoff$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshRequirementBackoff ' "$out")` | pass exit=0 | sha256:7a22013f95a8 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshCapacityCancellation$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshCapacityCancellation ' "$out")` | pass exit=0 | sha256:f48140b82cd6 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorPersistedReadThroughJoin$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorPersistedReadThroughJoin ' "$out")` | pass exit=0 | sha256:eac04426be67 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshSchemaBoundary$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshSchemaBoundary ' "$out")` | pass exit=0 | sha256:f69d363526b3 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./store` | pass exit=0 | sha256:7e5023a6a860 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief forge-neutral/54 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:6bf71ae6ad2e | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5693 run=65024386b4d48ce1c60469b4f7c48504a5b7e77bb395e524227dab93c673368d source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateAtomicCompletion$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateAtomicCompletion ' "$out")` | pass exit=0 | sha256:05c58f925cc3 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshGateProcesses$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshGateProcesses ' "$out")` | pass exit=0 | sha256:d43270459c03 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshAttemptAuthority$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshAttemptAuthority ' "$out")` | pass exit=0 | sha256:efd8fd6ea745 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshRequirementBackoff$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshRequirementBackoff ' "$out")` | pass exit=0 | sha256:c4f40262fcaa | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshCapacityCancellation$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshCapacityCancellation ' "$out")` | pass exit=0 | sha256:d5dd2f160d5c | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 6 | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorPersistedReadThroughJoin$' ./internal/deskkit > "$out" && grep -F -- '--- PASS: TestMirrorPersistedReadThroughJoin ' "$out")` | pass exit=0 | sha256:fdb6aefe9d79 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGRefreshSchemaBoundary$' ./store > "$out" && grep -F -- '--- PASS: TestPGRefreshSchemaBoundary ' "$out")` | pass exit=0 | sha256:cd8c3696b34d | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=1 · head=5142af8c66f4 |
| 8 | `cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./store` | pass exit=0 | sha256:da6e908e5845 | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 green rc=0 · head=5142af8c66f4 |
| 9 | `statusgen --root . --consumers --brief forge-neutral/54 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pass exit=0 | sha256:b96c1bc017ab | 2026-11-07 | assay-verifier-app[bot] @ 5142af8c66f4 (on-behalf-of human:ian) (forge-identity) | base=33b78e4273a2 red rc=2 · head=5142af8c66f4 |
Verification-Attestation: medici-finance/assay#5693 run=65024386b4d48ce1c60469b4f7c48504a5b7e77bb395e524227dab93c673368d source=5142af8c66f4671d7cb3c575d5b65d5eadd6275b brief=docs/streams/forge-neutral/brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md model=claude-opus-5-5 tier=strong

Verifier notes (independent verifier, model claude-opus-5-5, strong tier; source 5142af8c66f4 on merged main; delivery medici-finance/assay#5485, merge ef8316b8814d, first parent 33b78e4273a2):

- Setup: rows 1-8 ran on the host under `statusgen verifyrun` against a disposable local PostgreSQL container (loopback only, removed after the run) whose DSN was exported as `ASSAY_TEST_POSTGRES_DSN`. No forge write, no live infrastructure and no external endpoint was used; KUBECONFIG pointed at /dev/null. No row is a `check:ci` row, so no container bracket was opened.
- Attempt 1 (first table above, discarded as superseded): rows 1-8 pass exit=0; row 9 exit=2. In a clean clone of the source commit, row 9 prints that `forge-neutral/54` is not in the diff against 5142af8c66f4, because on merged main the merge-base of origin/main and HEAD is HEAD itself, so the three-dot diff is empty.
- Attempt 2 (second table above, credited): `verifyrun --fail-first --base 33b78e4273a2`. All 9 rows pass at head. At base, rows 1-7 are red rc=1 (the refresh gate, attempt authority, requirement backoff, capacity cancellation, schema boundary and read-through join tests do not exist before the delivery) and row 9 is red rc=2. Row 8 is green rc=0 at base: `go vet ./...` plus `go test -race ./store` passes before the delivery as well as after it, so this risk-bearing row does not discriminate the delivery.
- Row 9 at head in attempt 2 passes only because the uncommitted Evidence append puts this brief into the working-tree diff; its output reports 0 corroborated and 9 unchecked rows, so the pass carries no acceptance evidence for the delivery. A supplemental probe pinned to the delivery bounds (head 4d1427fda677, base ae3e2b12) also exited 0 with 0 corroborated and 9 unchecked rows, reporting the brief unchanged since the merge-base. Neither probe is credited.
- `statusgen verifyrun --check`: exit 0, 9 pass, 0 fail, 0 not run/missing of 9 Verify rows; 0 author-added rows with no acceptance credit.
- No deliverable defect was observed: every acceptance test of the delivery passed at head on real PostgreSQL and is absent at base.

Risk-bearing values (enumerated from the delivery's production files, ranked by consequence):

- RISK-VALUE: DERIVED — `workflow/store/migrations/00003_refresh.sql:199` and `:196` round the provider retry delay up to whole microseconds (`ceil(retry_delay_ns/1000)`) and take `greatest(...)` with the reported retry time; PostgreSQL timestamptz resolution is 1 microsecond, so rounding up is the only rounding that never schedules a retry earlier than the provider floor.
- RISK-VALUE: DERIVED — `00003_refresh.sql:191` (`http_count<>1`) and `workflow/store/refresh_attempt.go:121` (`o.HTTP.Value != 1`): one native HTTP attempt per reservation follows from the brief's requirement that budget is debited before every native round trip, SDK retries included.
- RISK-VALUE: DERIVED — `00003_refresh.sql:94` (`status BETWEEN 0 AND 599`) and `refresh_attempt.go:115`/`:128`/`:130` (`> 599`, `>= 100`): the HTTP status code space is 100-599 (RFC 9110 section 15), with 0 reserved for "no response received".
- RISK-VALUE: DERIVED — `00003_refresh.sql:47`/`:50`/`:71`/`:82` (`octet_length(...)=32`) and `workflow/store/refresh.go:101` (`sha256.Sum256`): 32 bytes is the SHA-256 digest length of the fill and scope keys.
- RISK-VALUE: DERIVED — `refresh.go:51` (`[24]byte` from crypto/rand): a 192-bit random generation/owner id, far beyond collision range for per-refresh ownership.
- RISK-VALUE: DERIVED — `refresh.go:59` (`FailureBackoff == 0` rejected): the brief requires requirement-qualified failure to carry a positive backoff, so zero is refused rather than defaulted.
- RISK-VALUE: DERIVED — `workflow/store/mirror.go:69` (SQLSTATE 40001/40P01): these are PostgreSQL's serialization-failure and deadlock-detected codes, the two retryable transaction conflicts.
- RISK-VALUE: NAMED, NOT DERIVED — `workflow/internal/refreshcodec/codec.go:33` (`len(raw) > 64<<10`): the 64 KiB codec cap is stated in code but no derivation of the specific size from a requirement was found.
- RISK-VALUE: N/A — `workflow/internal/pgtest/fixture.go:298`/`:121`/`:193`/`:53`/`:89`/`:91` and `workflow/pgtestfixture/fixture.go:39`/`:69` (30s/10s/5s timeouts, connect_timeout 1..10, MaxOpenConns 4): reversible test-harness knobs with no production effect.

#### Correction — 2026-11-07 rework

- Classification: the verdict above was recorded as BLOCKED. That was wrong. Row 8 is risk-bearing (this brief has `sensitive-data: yes`) and was observed green at base 33b78e4273a2 and at head, so it does not discriminate the delivery. An observed non-discriminating risk-bearing row is a failed acceptance obligation, so the verdict is FAIL. The failure is in the row's check definition, not in the delivery; it is filed as #5697 for row strengthening. Row 9 stays deferred under #4560 (its merge-base base equals HEAD on merged main). No deliverable defect was observed in rows 1-7. The new outcome record is `verify-fail`, blocker kind `check-definition`, blocker #5697, with row 9 `deferred`; it replaces the record this PR first added.
- Supplemental probe: the sentence above about a probe "pinned to the delivery bounds (head 4d1427fda677, base ae3e2b12)" was wrong. Those bounds are refused: at source 5142af8c66f4, `statusgen --root . --consumers --brief forge-neutral/54 --delivery 4d1427fda677abfaa384cf59fc1beb6d7ba5d9b0 --base ae3e2b12ded2c8202264a90df9d666d202842aa7` exits 2 with "base must be the delivery's first parent" (the first parent of 4d1427fda677 is 2a1e26a79da8). The quoted 0 corroborated / 9 unchecked output came from a run without `--delivery`, which compares the working tree, not the delivery. The correct delivery bounds are the merge of #5485 and its first parent. Re-run 2026-11-07T23:25:00Z in a clean detached worktree of 5142af8c66f4 (statusgen v1.0.77, `KUBECONFIG=/dev/null`): `statusgen --root . --consumers --brief forge-neutral/54 --delivery ef8316b8814dc2625ba701089909ddd8a399e96a --base 33b78e4273a2c68395a7cff18d8d831bca9b882f` exits 0 with `summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The one unchecked entry is the out-of-scope `graph-execution/19` boundary, and the run prints its standard notice that producer delivery is not evaluated. This run is supplemental and uncredited; it changes no row result.
- Accounting: the notes above did not record the exit code of the credited `verifyrun --fail-first --base 33b78e4273a2` run (the local run log ends `exit=1`, consistent with row 8 being non-discriminating); fail-first was not re-run for this correction. Row 9's `pass exit=0` cell in the credited table is not acceptance credit: row 9 is the deferred row, so the table's 9 passes are 7 credited rows, 1 failed obligation (row 8) and 1 deferred row (row 9).

**VERIFY: FAIL** — row 8 is risk-bearing and green at base 33b78e4273a2, so it does not discriminate the delivery (check-definition, #5697); row 9 is deferred under #4560; rows 1-7 pass and are red at base; no deliverable defect observed.

## Review

Gate: human. Independently review common SQL transaction, current generation and authority, pre-request budget enforcement, retry knowledge, requirement-qualified failure and durable resource ownership. Explicitly distinguish synthetic native requests, actual PostgreSQL evidence and unactivated production policy.
