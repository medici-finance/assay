---
brief: assay:assay:forge-neutral:51
title: Store-injected mirror read-through coordinator and shared refresh gate port
why: "A missing or stale shared answer should trigger one admitted refresh, while equivalent readers wait for that result. A reusable coordinator makes failures and cooldown visible without making every client poll the forge or waiting for write execution features."
wave: 3
depends: ["forge-neutral/46"]
unblocks: ["forge-neutral/53", "forge-neutral/54"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Read-through orchestration controls when private forge observations may be refreshed and returned. Accept the demonstrated admission, shared-gate and bounded-demand contract; fake storage proves neither durable coordination nor permission to expose a service."
decision-trigger: spec
decision-issue: 5399
issues: []
schema: brief-v2
version: 11
outcome: none
design: DR-forge-library
authored: 2026-11-03 by statusgen newbrief
sources:
  - "authoritative contract source: commit a1ffbc0e47708dcea430b59ccb89c466c71e7aee; this authoring adoption carries no runtime implementation or acceptance claim"
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/streams/forge-neutral/brief-50-join-admitted-incremental-reads-to-mirror-observations.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/library-first.md
  - docs/go-engineering.md
  - docs/performance-attribution-spec.md
  - "freshness-checked 2026-11-03 @ bb18a372b2b498e679922f5b83f6736d4b87fd01 (main); no refresh coordinator or canonical workflow store exists"
  - "read-through requirements amendment: medici-finance/assay#5394 at 46f46b17c8372d688a0bc666b806d6b7e66cc120; reconcile the current published amendment before dispatch"
exec-tier: strong
exec-tier-why: "Concurrency, cancellation, admission epochs and a partial nil-error source result cross independent boundaries; a process-local coalescer can appear correct while separate coordinators stampede."
domain: complicated
consumers:
  - "forgeread/refresh/: fixed-here (coordinator and typed gate port only)"
  - "forgeread/mirror/query.go, forgeread/mirror/kernel_test.go: fixed-here (canonical guarded snapshot acquisition and explicit-time evaluation; no transport or custody)"
  - "forgeread/README.md, docs/contracts.md, changelog/mirror-read-through-coordinator.md: fixed-here"
  - "tools/forge-import-check/, tools/desk/internal/regression/mirrorimports_test.go, tools/desk/internal/deskkit/citrigger_test.go: fixed-here (closed-target checker reuse and existing CI reader)"
  - "workflow/store/: follow-up forge-neutral/54 (canonical R2 refresh-gate adapter; no implementation here)"
  - "tools/desk/internal/deskkit/: follow-up forge-neutral/54 (durable joined fixture; source-only mapper remains50/53)"
  - "forgeread/service/, forgeread/client/: follow-up forge-neutral/39 (wire contract stays separate)"
  - "tools/desk/internal/forgeservice/: follow-up forge-neutral/40 (authentication, remote serving and executor custody stay separate)"
  - "statusgen/: out-of-scope (no subpackage import, evaluator change or caller cutover)"
---

# Brief 51 — Store-injected mirror read-through coordinator and shared refresh gate port

## Context

files:
- `forgeread/mirror/query.go`
- `forgeread/mirror/kernel_test.go`
- `forgeread/refresh/types.go`
- `forgeread/refresh/gate.go`
- `forgeread/refresh/coordinator.go`
- `forgeread/refresh/coordinator_test.go`
- `forgeread/refresh/boundary_test.go`
- `forgeread/README.md`
- `docs/contracts.md`
- `changelog/mirror-read-through-coordinator.md`
- `tools/forge-import-check/check_env_imports.go`
- `tools/forge-import-check/check_env_imports_test.go`
- `tools/desk/internal/regression/mirrorimports_test.go`
- `tools/desk/internal/deskkit/citrigger_test.go`

Boundary reuse edits the listed existing checker and CI reader; no workflow edit.

Preserve QueryScope's explicit-time evaluation contract. Factor its existing admission/store
acquisition into ReadScopeSnapshot, which returns an owned raw snapshot or an explicit
unavailable reason without evaluating freshness. The coordinator samples its trusted clock
AFTER every acquired query/baseline snapshot and before evaluation or PlanRefresh. A
concurrent committed observation is not future merely because I/O began earlier; timestamps
truly beyond the post-read clock still refuse. Preserve current admission/caller/deadline
checks at release and the independently captured source-observation time; do not retimestamp
coverage, add a clock callback to the kernel, or change Gate/store semantics.

facts:
- At the pinned main, `forgeread/mirror` exports `QueryScope`, `PlanRefresh`, `Source.Scan`, `PlanScan` and `Store.Read/Apply`. `PlanRefresh.NotBefore` is advisory; it neither schedules nor provides cross-process ownership. The reference store is synthetic conformance support, not runtime persistence.
- `/47` and `/48` supply admitted incremental reads and the native decoded response bound. Their provider-to-kernel mapper is a separate contribution. Consume that port; do not write another provider walker or move credentials here.
- `workflow/` and a shared refresh gate do not yet exist. R2 owns one PostgreSQL database and migration set; this brief supplies no alternative store, in-memory production gate or process-local failure cache.
- This is a bounded early contribution. Full `/37` retains every original Task and all 23 Verify rows, especially persisted joined row 23. Real PostgreSQL gate/store conformance, chosen retention, scheduler, served caller authentication, reader cutover and activation remain unproved.

layering: `forgeread/refresh` is a stdlib-only sibling of the pure mirror package. It orchestrates injected mirror Source/Store, explicit time and trusted policy through a typed shared-gate port. No HTTP, database, credential, OS/environment, process, daemon or wire adapter enters this package; no kernel algorithm is copied. Rows 1–8 exercise that boundary.

design-fit:
  owner: mirror semantics remain forgeread/mirror; orchestration is forgeread/refresh; durable refresh ownership/accounting remains the canonical R2 store
  contract: "S-forge-mirror in docs/contracts.md; extend its composition boundary with one read-refresh gate port, preserving execution claims and effect authority"
  retires: []
  weight: "verbs 0; flags 0; production stores 0; production credentials 0; one coordinator package and typed port"
  why-add: "The kernel intentionally does not schedule or coalesce. A single store-injected coordinator supplies that missing composition without copying its planners, adding client pollers or waiting for write/execution delivery."

single-point-of-failure: the shared gate's acquire/join/completion and retry-eligibility state. The independently admitted Source and Store still reject foreign/revoked scope, and the Store independently refuses incomplete watermark advancement. Gate ownership never grants data permission. Missing gate state holds the fill instead of falling back locally.

## Read first and architecture pickup

Read current `/37`, `/46`–`/48`, DR-forge-library, library-first and Go engineering guidance, the current read-through amendment, and the actual mapper/foundation child contracts. Record their revisions, actual APIs, source availability, and continue/adapt/hold before implementation. Mapper50 is a compatibility reference and later real-source integration owner, not an offline start dependency: Source.Scan, ScanResult and Store already exist in46. The coordinator treats Observation as opaque and compiles without50's additional presence fields. Its authoring commit is retained in genuine merge history, without claiming implementation delivery. Actual50+51+PostgreSQL qualification remains the joined integration owner; do not invent another mapper. No graph-execution admission, claim, outbox or effect prerequisite is added to this offline orchestration unit.

The real critical-path head for durable read-through is the canonical R2 foundation and shared-gate adapter, absent at the inspected main. Independent coordinator implementation can proceed with the explicit substitutes below; it cannot claim that head delivered.

## Human decision

Accept or hold the demonstrated read-refresh coordination boundary. The contribution uses only synthetic observations, an injected reference store and a shared stateful test gate. It introduces no production data copy, credential, listener or deployment. Acceptance would cover current authorization checks, isolated shared refreshes, bounded caller demand and truthful unavailable outcomes. It would not certify a durable gate, choose private-content retention or activate remote access.

Options:
1. Accept the demonstrated bounded contribution, retaining separate durable integration and activation gates.
2. Hold for a named correction to its scope, resource limits or failure behavior.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic evidence preparation may proceed. The executor prepares the concrete evidence decision at the spec trigger; it does not claim the human has accepted it.

## Ground rules

- Own isolated worktree and draft PR; no main write, merge, ready flip, release, workflow trigger or live infrastructure contact. Export `KUBECONFIG=/dev/null`.
- Test fixtures only. No real provider, ambient DSN, private data, credential discovery, daemon, wire DTO, command, client fallback or new storage backend.
- Only the canonical R2 adapter may implement production refresh ownership/accounting and failure cooldown. A missing/unreachable gate yields held/unavailable with zero Source calls; never substitute a local mutex, singleflight map, cache file or per-process retry allowance.
- Preserve `/37` row 23 and all existing acceptance/gates. Keep `/39`–`/40` custody and exposure held; this package is no authentication or service rollout.
- Permission loss reports a retention-decision signal and no content. Do not implement retain/purge actions before the existing human retention ruling. A truncated or oversized 404 classification alone never authorizes purge.
- Stop at implemented. Independent verification and human acceptance remain separate. Unknown exact model identity supplies no attestation.

## Task

1. Implement one typed read-through entry point over injected `mirror.Store`, `mirror.Source`, current `mirror.Admission`, explicit clock and finite trusted policy. On a sufficient fresh result return the actual `QueryScope` result without a source attempt. Otherwise perform the real sequence `QueryScope → PlanRefresh → gate acquire/join → Source.Scan → PlanScan → Store.Apply → QueryScope`. Re-read after joining another owner and before returning; a successful source call alone is never fulfilment. The supported query is the existing kernel scope/freshness query, with an optional requirement that every returned change has complete existing Reviews coverage. A sufficient result is current-admission, fresh Complete scope coverage and every requested existing population complete; a fresh genuinely empty complete scope remains a positive result. Refuse unsupported depth/fields explicitly rather than inventing an unfillable query or a policy boolean. Reuse kernel planners and admission checks, not parallel implementations. Preserve mapper records, explicit unknown fields and nil-versus-empty collections; never infer required body/review coverage from an empty value.
2. Define a narrow refresh-gate port with acquire-or-join, bounded wait/observation, current owner generation, completion and retry eligibility. This is read-refresh ownership/accounting only: no work claim, action admission, executor capability or effect intent. The base-fill key binds actual source population/operation, provider/host/canonical repository/kind, composition-derived admission, access/configuration epochs and trusted policy class/epoch. Projection differences do not split a compatible underlying fill. Per-caller accounting additionally binds the trusted caller identity; request text cannot choose a principal, epoch, policy or limit. Validate requested freshness/depth against that policy before gate access; client input cannot mint unlimited force-refresh demand.
3. Require the canonical R2 adapter to atomically bind a successful `Store.Apply`, its current refresh generation and gate completion/cooldown in the same store transaction. An old owner cannot publish after takeover even while holding the still-current expected snapshot version. Failed-fill completion records failure/retry eligibility and cannot promote coverage or advance LastComplete or LastInventory. A partial update may retain observed facts but never deletes prior rows or claims completeness; expected snapshot version alone is insufficient. Preserve the explicit Store.Apply seam through a typed gate-bound adapter; no generic transaction callback or arbitrary SQL port. This brief defines/qualifies the port with fixtures only. The production PostgreSQL implementation and independent bypass tests remain `/37` integration obligations.
4. Equivalent misses in two separately constructed coordinators sharing only their injected gate/store cause one source walk. Failure eligibility and cooldown live in that shared gate; no local map owns them. A failed attempt, partial nil-error scan, unfulfilled not-modified result or incomplete requested review population remains explicitly partial/unavailable and consumes the bounded failure interval. Preserve prior rows and LastComplete. Only a result sufficient for the actual query clears the relevant failure state. Authoritative observed absence is distinct from transport/permission failure. At cooldown expiry allow one gate-owned bounded retry in the synthetic profile, not one retry per caller. Current ScanResult/native receipts and Completion carry no qualified provider retry floor; fixture cooldown/RetryAt is not HTTP Retry-After proof. Real source/gate integration stays held until the joined owner carries typed qualified retry timing or an explicit unknown-retry hold. No body/header stash or blind fixed-interval production substitute is permitted.
5. Enforce explicit finite trusted limits for active fills, queued callers, waiter time, source deadline/Scan-attempts and retained failure entries/expiry, scoped per caller and admission. Missing/invalid policy holds. Saturating caller A cannot starve an eligible caller B under the configured fairness rule; A's quota refusal neither poisons B's cache/gate result nor creates a global failure entry. Reject excess demand visibly and count it separately. Capacity pressure cannot evict an unexpired failure entry into a fresh allowance; hold new demand or use the gate's existing bounded admission rule until expiry. Unknown shared usage holds rather than resetting per coordinator. Do not invent universal production thresholds: fixtures state their finite values; production policy is injected and remains unactivated. Count coordinator Scan attempts separately from the source owner's page, byte and provider-retry budgets; never claim one Scan means one native request or install a second retry policy.
6. Separate waiter cancellation from shared owner lifetime. Cancelling one waiter releases its queue/budget accounting and cannot cancel another admitted caller's fill. The fill itself has a bounded coordinator-owned lifetime and release/completion path. No detached unlimited goroutine survives all demand or a deadline. Revalidate current admission and epochs before Source access, Store.Apply and the final query. Changed/revoked access refuses without returning stale private rows.
7. Add stateful fake-gate and reference-store fixtures only in test files. Two independently constructed coordinators must share no private coalescer state. Exercise stale generation, takeover, crash/unknown completion, gate outage, cross-admission and caller-quota negatives, partial nil-error results, useful complete-empty and nonempty positives. Mutate actual coordinator decisions, not only test doubles; restore every plant and rerun named tests. Record which lower controls are fixture contract evidence and which require future real PostgreSQL proof.
8. Preserve root/offline and mirror boundaries, current operation inventory and no-dependency module manifest. Update README and the existing semantic-owner register; do not add a competing meaning/register. Record own source attempts, coalesced waits, bytes and cache outcomes only through an injected bounded observation seam compatible with perf-v3; unknown instrumentation is unknown, never measured zero or a copied waiter total.

## Interface contract and substitution limits

The shared-gate composition admits only `Limits.Refresh.FailureBackoff == 0`.
Reject any nonzero value, including negative values, before store reads, gate acquisition,
quota debit or source work; do not silently derive a different policy. The canonical R2 gate
is the sole shared retry-eligibility authority here, including provider-qualified retry floors
when joined later. Its trusted configuration/profile validation must enforce the same zero
kernel-backoff policy. Old incompatible profiles remain held until explicitly replaced.
The standalone mirror planner retains its general nonzero advisory-backoff support unchanged.
A kernel NotBefore non-attempt is never an unqualified source failure or permission to slide
shared cooldown. Test positive zero and explicit 10-second, 3-minute and negative refusals
with zero I/O, alongside existing actual cooldown-expiry and review-only partial controls.

Inputs are a currently admitted scope and query, trusted caller/policy/epoch facts, explicit time, source/store and a shared gate. Outputs preserve `mirror.Result` plus explicit fulfilment/held/unavailable, typed reason and retry eligibility; they grant no authority. Gate references and owner generations are opaque, scoped and never credentials. A gate refusal is not a successful empty query.

Only a typed stale-generation error from Gate.Wait may trigger a guarded current-snapshot
requery, without another Acquire, source attempt or retry loop. Return success only if that
snapshot satisfies this caller's complete observation/freshness requirements under current
caller/admission/deadline checks, including after Leave I/O. If recovery is insufficient,
future-dated or revoked, retain an explicit hold/error and no released rows. Other gate errors
and multi-error holds are not recovered this way. The old ticket's Leave outcome remains
UNCHECKED even when a current snapshot is sufficient; it cannot clear/install failures for
a rotated generation. Exact old-ticket accounting release remains the canonical adapter's
obligation. Do not pin completed generations while old weak readers remain: stronger reads
must still acquire/complete a needed refresh under concurrent weak traffic. Qualify these
cases through the existing gate-fence/shared-read tests and compiling removal controls.

Demand carries one bounded observation requirement class: whether existing Reviews must be complete. It does not add arbitrary freshness, projection or request identifiers to the base fill key. Compatible demands retain one underlying fill; source-failure eligibility and query-qualified failure eligibility are separate shared records. After its actual requery, each waiter supplies a typed Leave outcome bound to its ticket, refresh generation and exact queried snapshot version. A current authorized unfulfilled outcome records the relevant bounded shared cooldown; a fulfilled outcome clears only the relevant requirement failure. Cancellation, expired deadline, admission/epoch denial and refusal are unchecked outcomes that cannot install shared failure. A late unfulfilled outcome from an older generation or snapshot version must not poison a later qualified recovery, including when another waiter in the same original fill is delayed. Canonical R2 persists this outcome/accounting atomically; the fixture supplies no production backend. Recheck current caller authority, repository admission and deadline after Leave I/O before releasing any rows.

Use the canonical pure Completion.ReviewOnlyPartial classification before normalizing an owner's query: a matching partial update caused by RequireReviews, with no admission refusal or explicit source Failure. Persist that class only with a qualified completed generation and its applied snapshot version. A joined weaker base waiter must not install a Requirements=false cooldown solely because that stronger owner conservatively left the scope Partial. Its next admitted base read may perform the existing one bounded refetch. The stronger requirement still receives its shared failure; actual incomplete source populations retain explicit Failure and source cooldown. This does not change Task4's LastComplete preservation or turn unknown reviews into complete coverage. Test both owner/waiter orderings and remove the classification guard to prove the reverse-order fixture fails.

Expose a narrow pure snapshot evaluator in the existing mirror semantic owner, reusing its validators and freshness/permission result computation. QueryScope delegates to that evaluator after its existing admission and store-read checks; all refusal behavior remains. The evaluator accepts only an already-owned typed snapshot and explicit Query/time, performs no I/O and grants no authorization. After the last observer or Leave callback, refresh performs current caller/admission/deadline checks, then uses the evaluator with trusted current time and no later callback or I/O before return. Source completion time is captured immediately when Scan returns, before diagnostics; scan-call counts are emitted only after that actual invocation, with no observer between final authorization and Scan. Do not copy time/coverage rules or add a Store wrapper. Test actual observer and Leave callbacks that revoke access or advance time beyond freshness, truthful scan counts, plus pure evaluator validation and detached copies.

| Boundary | Real in this contribution | Substitute or absent | Evidence limit / replacement owner |
|---|---|---|---|
| Query/planning/application/requery | Imported production mirror kernel and production coordinator | Test reference Store | No durability; `/37` runs the same cases with canonical PostgreSQL Store |
| Source | Existing Source port and mapper contract | Scripted synthetic Source | No provider/custody qualification; mapper child owns actual mapping and `/37` joins its admitted implementation |
| Refresh ownership/failure eligibility | Production port and coordinator use | One shared stateful fake gate across two independent coordinators | No cross-process or restart claim; canonical R2 adapter and `/37` real PG tests must replace it |
| Caller scope/policy | Current production admission checks | Mutable synthetic capability and finite trusted policy | No authenticated remote caller; `/39`–`/40` retain their gates |
| Effects/execution/activation | None | Absent | No claim, executor, listener, scheduler or activation promise |

Provider retry timing is absent at the inspected admitted port; configured fixture cooldown is only shared-failure suppression evidence. Real provider retry eligibility remains unqualified and held for the joined integration owner.

Tests set up fresh reference store and fake gate per case, seed only synthetic records, explicitly drive time and cancellation, and reset/close all owned work at teardown. Missing gate, store or policy cannot turn into another implementation. Future real-adapter qualification must retain these cases and add independent-process/restart and direct-SQL bypass controls; replacing a fixture never inherits its evidence automatically.

## Verify

Named tests exercise the delivered coordinator and kernel through explicit synthetic Source/gate/reference-store fixtures. No PostgreSQL, provider, custody or activation pass follows. Missing packages/tests or required fixtures fail; no skip-to-PASS. The existing cross-module trigger/staleness instruments currently skip on their absent legacy tools.yml fixture; that gap is recorded on issue5390, never credited as passing coverage.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshJoinedFlow$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshJoinedFlow ' "$out")` | named PASS; actual kernel/coordinator sequence fills cold scope and reuses fresh nonempty/complete-empty answers with no extra source call. Mutate coordinator to return source success without applying/requerying; the test fails. |
| 2 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshSharedGate$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshSharedGate ' "$out")` | named PASS; two separate coordinator instances, 50 equivalent callers synchronized at the source boundary and one shared fake gate/store produce one scripted source inventory; compatible projections join it. Mutate production gate use to a per-coordinator coalescer; observed source count exceeds one. Fixture contract evidence only, no cross-process claim. |
| 3 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshFailureBackoff$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshFailureBackoff ' "$out")` | named PASS; transport failure and partial nil-error leave prior watermark/rows and explicit unfulfilled coverage; a second coordinator makes zero attempts inside shared cooldown, then one retry at expiry. Mixed base/required-review waiters retain one fill and shared requirement-qualified cooldown, without sliding expiry or poisoning weaker demand. A sufficient recovery alone clears relevant failure. Mutate nil error into fulfilled or keep cooldown locally; the named test fails. |
| 4 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshScopeEpochs$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshScopeEpochs ' "$out")` | named PASS; separate admissions/access/config/policy epochs never share unauthorized rows or failures. Compatible projections still share base fill. Revocation during a fill refuses publication/return. Mutate the production key or skip final authorization; the canary/isolation assertion fails. |
| 5 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshFairBudgets$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshFairBudgets ' "$out")` | named PASS; finite fixture limits bound queue/fills/wait/attempts/failure retention; saturated A is rejected, eligible B completes under the stated fairness rule, and A's quota refusal never poisons B or creates a source-failure entry. Mutate per-caller accounting to one global denial; B's positive control fails. |
| 6 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshWaiterCancel$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshWaiterCancel ' "$out")` | named PASS; cancel one waiter while another receives the shared result; waiter accounting releases and all work terminates by its finite owner deadline. Mutate shared lifetime to the first waiter's context; the surviving caller fails. |
| 7 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshGateFence$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshGateFence ' "$out")` | named PASS; missing/unreachable gate causes held/unavailable and zero Source calls, with no local fallback. Fixture takeover rejects stale owner's Apply/completion even with unchanged snapshot version; unknown completion re-queries rather than replaying blindly. Mutate production bypass of gate ownership or fallback; named test fails. Late same-fill waiters cannot poison recovered generation/version; independent generation and actual snapshot-version controls refuse stale outcomes. Real SQL atomicity is explicitly not credited. |
| 8 | check:ci +dereference +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshBoundary$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshBoundary ' "$out" && GOWORK=off go test -count=1 ./...)` | named PASS and module suite PASS; actual production dependency closure has no HTTP, SQL, process, credential or external-module access, and row12 independently reuses the shared environment-import checker for direct decoded os/syscall imports; root imports no subpackage and pure mirror never imports refresh. A planted forbidden import is rejected; useful injected gate/source controls remain. Rows 2 and 7 independently reject a process-local production gate/fallback. |
| 9 | check:ci | `cd forgeread && GOWORK=off go test -race -count=1 ./refresh/...` | exit 0; all named coordinator cases pass under race; no owned work/blocked waiter leaks at teardown. |
| 10 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/51 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Pre-merge on the implementing branch: exit 0; actual fixed-here edits corroborate, deferred routing proves no producer use, and unchecked remains unchecked. Before post-merge verification, bind this row to the independently established immutable mainline delivery and its first parent with --delivery and --base. A branch-relative check on merged main is never a pass. Authoring claims no implementation delivery. |
| 11 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshReleaseAuthority$' ./refresh/... > "$out" && grep -F -- '--- PASS: TestRefreshReleaseAuthority ' "$out")` | named PASS; deterministic Store.Read revokes caller, repository admission or access epoch after the initial check. Fresh, joined and completion returns expose zero rows. Gate Leave then independently revokes admission/caller/epoch or expires the deadline; final release still returns zero rows. Observer/Leave callbacks that advance trusted time past MaxAge cannot remain fulfilled; canonical pure evaluation validates/ages detached snapshots without I/O or authorization. Mutate post-I/O release checks independently and the named controls fail. |
| 12 | check:ci +dereference +mutation | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/refresh` | exit 0; output is 0; the existing closed-target checker scans refresh. Existing mirror/events and arbitrary-target refusals remain; compiler-valid raw/escaped forbidden imports planted independently in refresh are rejected by the actual always-on checker fixture suite. No arbitrary target or environment access is admitted. |

## Evidence

<!-- Independent verifier records command, exit, named result, exact subject/environment and date. No prospective runtime PASS is authored here. -->

## Review

Gate: human. Review actual port ownership, current admission, bounded demand, independent-coordinator evidence and substitution limits. Formal review never grants durable integration, remote custody or activation. Preserve all full `/37` criteria and existing gates.
