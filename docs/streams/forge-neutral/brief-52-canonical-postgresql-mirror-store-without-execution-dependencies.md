---
brief: assay:assay:forge-neutral:52
title: Canonical PostgreSQL mirror store without execution dependencies
why: "The shared mirror needs durable scoped rows and honest coverage before execution features exist. Extend the canonical PostgreSQL owner instead of introducing a mirror database or waiting for workflow admission and outboxes."
wave: 3
depends: ["graph-execution/36", "forge-neutral/46"]
unblocks: ["forge-neutral/54"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The shared mirror needs durable scoped rows and honest coverage before execution features exist. Extend the canonical PostgreSQL owner instead of introducing a mirror database or waiting for workflow admission and outboxes."
split-from: forge-neutral/37
decision-trigger: spec
decision-issue: 5401
ruling: "https://github.com/medici-finance/assay/issues/5401#issuecomment-6621965962"
issues: []
schema: brief-v2
version: 3
outcome: none
id: d356d914-92e7-43b0-ae37-17016f7993bc
authored: 2026-11-03 by statusgen newbrief
sources:
- docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
- docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md
- docs/library-first.md
- docs/go-engineering.md
- "freshness-checked 2026-11-03 against App-fetched main bb18a372; foundation contract revision 7646ec0da read before implementation pickup"
exec-tier: strong
exec-tier-why: "Direct-store authorization, independent SQL transition checks and concurrent real-PostgreSQL commits must remain consistent when planners and processes are bypassed."
domain: complicated
consumers:
- "workflow/store mirror adapter and canonical migration set: follow-up forge-neutral/52 (implementation owner)"
- "workflow/internal/pgtest and .github/workflows/assay-workflow.yml: follow-up forge-neutral/52 (reuse graph-execution/36 fixtures and extend actual trigger inputs)"
- "forgeread/mirror Store contract: follow-up forge-neutral/52 (consume unchanged port, no kernel SQL dependency)"
- "shared refresh gate and admitted source join: follow-up forge-neutral/54 (canonical implementation owner)"
- "full mirror acceptance: follow-up forge-neutral/37 (consume canonical Store/Gate/join; persisted parent qualification remains)"
- "workflow execution, admission, claims and outbox: out-of-scope (remain graph-execution/19)"
---

# Brief 52 — Canonical PostgreSQL mirror store without execution dependencies

## Context

files:
- `workflow/go.mod`
- `workflow/go.sum`
- `workflow/store/`
- `workflow/store/migrations/`
- `workflow/internal/pgtest/`
- `workflow/README.md`
- `spec/coordination-store-v1.md`
- `.github/workflows/assay-workflow.yml`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `docs/contracts.md`
- `changelog/mirror-postgres-store.md`

facts:
- The R2 foundation child graph-execution/36 supplies the only PostgreSQL driver, Goose migration ledger and explicitly provisioned container fixture. The Store genuinely merged its tested source before publication and now consumes its mainline delivery through main `077f19e6afb71169f79295486a223896cab94913`. The admitted mapper and pure kernel are also on that actual main revision. Missing canonical migration/harness input still requires stopping; source pickup supplies no independent verification or full parent acceptance credit.
- `forgeread/mirror` already exports Admission, Scope, Snapshot, ScanUpdate and Store.Read/Apply. `mirrortest.Check` exercises the Store directly without QueryScope/PlanScan. ReferenceStore is synthetic, not persistence or a retention policy.
- Full /37 retains 23 Verify Class/Command pairs and its persisted joined row23. Full graph-execution/19 retains its 13 rows and execution/reconstruction obligations. This contribution satisfies neither complete parent.
- The operator selected purge-to-tombstones, but the separate human issue record and full retention implementation remain /37 obligations. This contribution uses synthetic observations only and implements no purge/retain action or private-data exposure.

layering: Keep the SQL adapter and its migrations in the existing `workflow/store` owner, using /36's driver, ledger and harness. Import mirror semantics and conformance support; never copy the planner, create a SQL dependency in forgeread or add a second backend/ledger. Tests exercise the real library→driver→PostgreSQL flow.

design-fit:
  owner: R2 workflow/store; mirror semantics remain forgeread/mirror
  contract: "coordination-store-v1 gains implemented mirror tables; future workflow and shared-refresh-gate schemas remain explicitly planned"
  retires: []
  weight: "verbs 0; flags 0; backends 0; no new module, driver or migration ledger; one planned Store adapter"
  why-add: "Use the already selected R2 store. A mirror-only database or temporary local cache would duplicate persistence, migrations and permission handling."

single-point-of-failure: the per-scope SQL compare-and-set transaction. Independent SQL transition checks still refuse incomplete coverage advancement when the Go planner or validator is bypassed; neither guard grants caller authorization.

## Read first and pickup

Read the actual /36 delivery, /37 and /46 contracts, `mirror.Store`, `mirrortest.Check`, library-first and the selected dependency catalogue. Record exact source revisions and continue/adapt/hold before code. A tested immutable /36 feature revision may be genuinely merged into an owned stacked implementation branch before mainline delivery; the supported dispatcher must still admit pickup. An authoring-only contract is insufficient. A missing canonical migration/harness is NEEDS_CONTEXT, never permission for a second store. Reuse /36's approved pinned dependencies and explicit container DSN; no ambient PostgreSQL variables, Docker context or cluster discovery.

## Human decision

Accept or hold the demonstrated synthetic-data PostgreSQL Store contribution, including independent authorization, atomic rows/coverage, restart durability and explicit unavailable outcomes.

1. Accept the bounded contribution, retaining full /37 retention, shared-refresh integration, service and activation gates.
2. Hold for a named correction to admission, persistence or coverage behavior.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic fixture preparation may proceed. No real private-data ingestion, deletion or service exposure is authorized.

## Ground rules

- Own isolated branch/draft PR. No merge, ready flip, release, workflow dispatch, deployment or live infrastructure contact. Container provisioning is explicit and local; tests receive a verified disposable fixture DSN.
- One canonical Goose migration manifest/ledger and one PostgreSQL backend from /36. No SQLite, cache file, new database service or fail-open fallback.
- Implement only `mirror.Store` and its schema. No scheduler, source walker, credential, wire endpoint, refresh gate, execution claim, incarnation activation, action admission, outbox or effect delivery.
- Keep every parent Task, dependency, Verify Class/Command pair and human gate. Do not credit this contribution as full /19, full /37 or parent row23.
- Policy-neutral permission holds only. Never purge copied fields on an unqualified403/404 or select a retention policy here. No real private content enters the fixtures.
- Stop at implemented; independent verification owns Evidence and lifecycle advancement.

## Task

1. Consume the genuine /36 module, typed connection configuration, supported Goose checksum policy and fixture lifecycle. Add mirror tables through that same ordered forward-only migration set. Document implemented mirror columns, keys, constraints and their schema floor in coordination-store-v1; preserve planned workflow/refresh schemas as planned. Missing/unreachable/unsupported input refuses without another backend.
2. Implement the existing Store port with independent current admission checks for both Read and Apply. Bind rows, coverage and versions to provider/host/canonical repository/kind/admission. A supplied admission string alone grants nothing. Wrong, missing or revoked capability refuses before rows are disclosed or changed, including direct Store calls with QueryScope bypassed. Initial valid scope reads never-read explicitly, not complete-empty. Store configuration never resolves a credential or repository alias.
3. Atomically compare ExpectedVersion, upsert observed facts, mark only qualified full-inventory absences and write coverage/version in one PostgreSQL transaction. Concurrent first writes and competing updates from separate clients have exactly one winner; stale updates change nothing. Partial scans may preserve observed facts but cannot delete absent rows or advance complete/inventory watermarks. Refuse stamp regression, malformed/foreign IDs, incomplete complete coverage and invalid permission-hold recovery independently of the planner. Add SQL-enforced transition predicates so the specific completeness/inventory/monotonicity guards still hold under a raw invalid SQL control, without pretending SQL can reauthenticate an external caller capability.
4. Preserve the persisted permission hold and require the existing complete fresh full-inventory recovery proof before it clears. A failed/partial/status-only result cannot clear it or fabricate successful empty coverage. Preserve unknown fields and nil-versus-known-empty collection presence supported by the current kernel; if /50's additive presence delivery is available, test exact round trips rather than infer unknown values. No retain/purge policy is implemented. Keep RetainedFrom unknown unless independently qualified by the existing canonical history owner; this bounded delivery creates no qualified retention authority or positive history claim.
5. Reuse `mirrortest.Check` against real PostgreSQL, then add independent-client/process CAS, atomic rollback and reopen/restart cases. Reset only owned disposable fixture databases. A malformed SQL transition must be refused when Go validators are bypassed. Drop/recreate only owned mirror tables as a synthetic rebuild control, returning never-read without reviving old completeness. Plant actual production mutations for authorization, CAS, transaction/constraint guards and hold recovery; a compile failure does not count as a killed mutation.
6. Update workflow README, the existing semantic-owner register, schema conformance and workflow CI trigger coverage for every actual out-of-module input (`forgeread/mirror/**`, conformance helpers and the store contract). Use the actual current cross-module registry or document its honest opt-out; do not weaken any existing check. Record resource ownership, close/rollback and explicit fixture setup/teardown. Include the changelog fragment.
7. Record substitution limits: this real Store is not yet a coordinator-safe shared refresh adapter. /51's production gate must later bind current refresh generation, Store.Apply and completion/cooldown in the same SQL transaction. Until that joined adapter is delivered and independently qualified, no production coordinator may use naked Apply to claim stale-owner fencing. Actual admitted mapper + coordinator + PostgreSQL joined evidence remains full /37 row23.

## Verify

All named tests below are prospective. Missing test/package or fixture fails; a skipped test is not PASS. Use only the explicit container fixture supplied by /36.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorConformance$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorConformance ' "$out")` | named PASS; Run actual mirrortest.Check against the SQL Store with direct admission, conflict, completeness and permission-hold controls. Independently remove Store authorization and hold recovery refusal; each mutation fails while allowed complete/empty controls pass. |
| 2 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorAtomicCoverage$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorAtomicCoverage ' "$out")` | named PASS; Actual rows, version and coverage commit atomically. Inject a failure between row work and coverage commit; reopening sees the old entire snapshot. Direct invalid SQL cannot advance LastComplete/LastInventory or regress stamps. Bypass Go validation as the negative control; removing the production transaction/SQL guard fails. |
| 3 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorConcurrentCAS$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorConcurrentCAS ' "$out")` | named PASS; Two independent processes/connections race initial insertion and same-version updates through deterministic barriers; exactly one winner and an intact losing snapshot. Remove the production SQL version predicate and the named test fails; an in-process mutex is insufficient. |
| 4 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorRestartRebuild$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorRestartRebuild ' "$out")` | named PASS; Restart the Store client in an independent process; committed rows/coverage survive through the actual database. Rebuild owned mirror tables with the canonical migrations and every scope is never-read. Restore old completeness after rebuild and the test fails. A client-process restart is not a PostgreSQL-server restart claim; the explicit container provisioning protocol owns server-restart qualification separately. |
| 5 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorScopeIsolation$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorScopeIsolation ' "$out")` | named PASS; Two admissions with the same repository and number have separate rows/versions. Missing, foreign and revoked admission direct reads/applies refuse without canary disclosure or mutation. Provider/host/kind scope also isolates. Remove the admission SQL partition or independent authorization and the test fails. |
| 6 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorPresenceHistory$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorPresenceHistory ' "$out")` | named PASS; Unknown scalar/collection presence survives SQL round trip; known-empty stays distinct when supported by the actual kernel revision. Prior visible old rows alone leave RetainedFrom unknown. A status-only permission hold performs no retention deletion and cannot return complete-empty; mutate derived history to oldest row or clear the hold and the test fails. |
| 7 | check +dereference +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPGMirrorSchemaBoundary$' ./store/... > "$out" && grep -F -- '--- PASS: TestPGMirrorSchemaBoundary ' "$out")` | named PASS; Resolve the implemented mirror contract and actual migration schema in both directions. Missing/extra table or column, second ledger/backend or production forgeread SQL dependency fails. Check actual cross-module CI inputs, not just package presence; full workflow/refresh schemas remain planned. |
| 8 | check | `cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -race -count=1 ./...` | exit0 against the explicit disposable container; no skipped database evidence credited. |
| 9 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/52 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Pre-merge actual delivery corroborates implemented paths; downstream routing proves no implementation. Post-merge use independently established immutable delivery/base bounds. |

## Pre-mortem and detection

- Leaked cross-admission rows: rows1/5 direct-store bypass/canary controls.
- Rows acknowledged without coverage or conflicting writers both commit: rows2/3 real SQL and independent-process mutations.
- Restart/rebuild resurrects completeness: row4 real fixture restart and rebuild negative.
- Presence/retention guessed from empty/old rows: row6 unknown/empty and oldest-row mutations.
- A second store or fixture quietly replaces canonical PostgreSQL: row7 schema/boundary plus explicit absence refusal from /36.
- Store acceptance mistaken for shared-gate fencing or full parent completion: independent review of Task7 and unchanged parent acceptance; no fixture result supplies joined row23.

## Evidence

<!-- Independent verifier only. No prospective runtime PASS or parent acceptance is authored here. -->

## Review

Gate: human. Judge direct-store authorization, SQL atomicity, actual real-PostgreSQL evidence and explicit substitution limits. Acceptance never activates private-data copying, remote serving or execution.
