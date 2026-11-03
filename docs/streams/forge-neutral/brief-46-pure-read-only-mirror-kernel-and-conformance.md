---
brief: assay:assay:forge-neutral:46
title: Pure read-only mirror kernel and conformance
why: "Repeated readers need one reusable answer whose scope, age and completeness cannot be mistaken for a fresh empty queue. A pure kernel establishes these semantics before the durable store exists, without choosing private-data retention or adding another poller."
wave: 2
depends: ["forge-neutral/36"]
unblocks: ["forge-neutral/37", "forge-neutral/51"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The kernel's admission and completeness rules will govern private forge observations. The human accepts the demonstrated pure boundary and its limitations; synthetic fixtures do not choose retention, qualify a durable store or authorize exposure."
decision-trigger: spec
decision-issue: 5365
issues: []
schema: brief-v2
version: 10
id: cf6f3530-04b7-4c11-b611-e4c02c24a0f4
outcome: none
design: DR-forge-library
authored: 2026-11-03 by statusgen newbrief
sources:
  - docs/consumer-delivery-checks.md
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-02 @ public main 7c62923293d0b640ec90afbf318cae2497908f28; read-only mirror acceleration"
exec-tier: strong
exec-tier-why: "Admission mismatches, partial scans and stale watermarks can silently become complete-empty facts; independent lower-layer controls and exact source identity are required."
domain: complicated
consumers:
  - "forgeread/mirror/: follow-up forge-neutral/46"
  - "forgeread/mirror/mirrortest/: follow-up forge-neutral/46"
  - "forgeread/README.md: follow-up forge-neutral/46"
  - "docs/contracts.md: follow-up forge-neutral/46"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up forge-neutral/46 (exact mutation harness test classification only)"
  - "changelog/forge-mirror-kernel.md: follow-up forge-neutral/46"
  - "workflow/store/, trusted mirror ingestion, coalescing and populated reader flow: follow-up forge-neutral/37"
  - "statusgen/: out-of-scope (no import, transport or eligibility change in this contribution)"
---

# Brief 46 — Pure read-only mirror kernel and conformance

## Context

files:
- `forgeread/mirror/`
- `forgeread/README.md`
- `docs/contracts.md`
- `docs/streams/desktools-v2/cli-migration.json`
- `changelog/forge-mirror-kernel.md`

Reuse the existing source-only checker in tools/forge-import-check without copying it.

facts:
- At source `7c62923293d0b640ec90afbf318cae2497908f28`, forgeread/36's module, root offline reader and admitted adapters exist; `forgeread/mirror` and `workflow/store` do not.
- The kernel needs an injected Store and authorizer, not PostgreSQL or graph-domain types. The reference Store is synthetic conformance support, not production persistence or another deployment authority.
- This is a declared contribution split from /37. Every full /37 criterion remains, including real PostgreSQL transactions/restart, authenticated source composition, scoped coalescing, review populations, retention selection, domain mapping and reader cutover.
- The 25 October DR-forge-library selects a future shared credential-owning forgeclient library. This contribution creates no credential-bearing client and does not reinstate the superseded global ban on statusgen's approved library migration.

layering: Stdlib-only mirror semantics take typed observations and explicit time through narrow Store/Admission ports. They perform no HTTP, database, OS/environment, filesystem or process operations. The forgeread root imports no subpackage; no package here imports statusgen or workflow. Rows 1–6 prove the split.

design-fit:
  owner: forgeread/mirror, extracted early from the existing scoped mirror plan
  contract: "scoped mirror identity, coverage and query relation; declare the one semantic owner in docs/contracts.md without taking ownership of credentials, domain eligibility or retention policy"
  retires: []
  weight: "verbs 0; flags 0; production stores 0; one pure package and reusable conformance helper"
  why-add: "The full persisted mirror currently holds these independently useful semantics behind missing durable-store work; this supplies that capability once for the later integration owner."

## Read first and architecture pickup

Read /37's entire current Task/Verify/retention decision, `docs/contracts.md`, `docs/library-first.md`, `docs/go-engineering.md`, DR-forge-library and the existing forgeread code. Apply the public implementation-pickup procedure before effects and on resume. Record source/guideline/contract revisions and continue/adapt/hold in the PR. The observed /36 source capability does not silently upgrade its lifecycle status; dispatcher eligibility remains independently enforced.

## Human decision

At implementation design, prepare the demonstrated query/admission and completeness boundary with positive and bypassed-layer evidence. Accepting this pure contribution grants no private-data retention policy, persistent storage, network listener, credential or operational activation. The existing separate retention choice remains unanswered. Options are acceptance of the demonstrated bounded contribution or hold for a named correction; absence of acceptance permits no activation. The executor files the concrete evidence decision at the spec trigger, before acceptance is claimed.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic evidence preparation may proceed; no private repository data is ingested or served.

## Ground rules

- Own isolated worktree and draft PR; no main push, merge, release, workflow trigger or live infrastructure contact. Use synthetic records and disposable offline fixtures; export `KUBECONFIG=/dev/null`.
- Stop at implemented. Tests are implementation evidence; an independent verifier and the existing human gate own acceptance.
- Preserve every /37 obligation and historical Evidence. Do not select retain-history/purge or invent a production default. Permission loss must expose unavailable coverage and a required retention-decision signal.
- No new service, scheduler, wire DTO layer, credential discovery, production Source/Store, cross-process claim, dependency or statusgen link. Missing context holds affected work rather than creating a second authority.

## Task

1. Implement the single mirror-owned identity and observation model. A repository is provider/host/full path, including nested GitLab namespace; a Scope also binds record kind and opaque composition-derived admission ID. Records distinguish issue/pull-request/merge-request, exact head and provider-specific review/approval completeness. Coverage includes never-read/complete/partial/unavailable reason, attempted/complete/observed-up-to/retained-from stamps and revision. RetainedFrom remains optional and unknown here; /37’s durable Store must derive it from actual retained coverage under the selected policy, never from the oldest visible row. Trusted composition must canonicalize GitHub owner/repository case before constructing a Scope; the pure kernel does not resolve provider aliases. Validate foreign records, invalid identity and duplicate records before applying. Copy mutable input/output collections.
2. Define an Admission capability supplied by trusted composition, with ID and current `Authorize(ctx, RepositoryID)`; never deserialize it from client input. `QueryScope` requires a nonempty matching admission ID and current coverage before Store access. Store.Read/Apply independently enforce that same relation and return typed permission refusal, not filtered complete-empty. Revalidate returned scope. A genuinely admitted empty complete snapshot alone is known-empty; stale/partial/failed data never imply current emptiness. Explicit Now/MaxAge govern freshness.
3. Implement deterministic `PlanRefresh` and `PlanScan` with supplied overlap/full-inventory/backoff policy and time. Initial/full scans and overlapping incremental scans preserve uncertainty. A permission-held baseline forces the next recovery plan to a full inventory, independent of the periodic repair interval. Track the last full inventory independently from the last complete incremental scan so successful incremental reads cannot postpone periodic repair forever. One ScanUpdate binds expected version, upserts, coverage and complete marker. Partial/failing scans retain prior records and LastComplete; absence from incremental/partial observations proves nothing. Only complete full inventory can mark not-in-inventory, distinct from closed. Validate temporal regressions. Synthetic not-modified input cannot promote a partial baseline; conditional transport remains /37.
4. Publish narrow Store/Source contract types and reusable conformance assertions against a synthetic reference Store. The source port is a future composition seam, not an adapter added here. Direct Store calls with the library bypassed reject nonmatching/revoked admission and incomplete watermark advance; a permissive Store double proves the library also refuses independently. This is not a claim about PostgreSQL or durable compare-and-swap.
5. On permission loss produce unavailable:permission plus retention-decision-required. Keep this permission-loss hold across subsequent transport, rate-limited, unknown or truncated/non-complete refreshes; only a successful complete full inventory applied under the current matching admission may clear it, with LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. An incremental scan, stale/replayed complete update or conditional not-modified result cannot clear the hold. Store adapters independently refuse any other clearing transition, including with the planner bypassed. Do not apply retain/purge actions. /37 still owns the selected retention behavior, real SQL constraints, real restart/rebuild, coalesced ingestion, source mapping, graph completeness mapping and populated readers. Document how the kernel is consumed there rather than reimplemented.
6. Add an external imported-package fixture flow that seeds a synthetic scan through production planning/application and queries it through the production API. Keep forgeread root/wire behavior unchanged and no module require/replace. Record lower-layer negative evidence by mutating actual production admission, watermark and partial-absence logic, restoring it and rerunning the named test; a duplicate test implementation is not an oracle.

## Interface contract

The concrete small API uses `RepositoryID`, `Scope`, `Admission`, `Observation`, `Coverage`, `Snapshot`, `Result`, `Query`, `RefreshPolicy`, `ScanPlan`, `ScanResult`, `ScanUpdate`, `Source` and `Store`, with `PlanRefresh`, `PlanScan` and `QueryScope`. Freeze exported signatures in the implementation PR before another contribution consumes them. /47 owns provider transport DTOs and imports no mirror package; /37 later maps those DTOs into this model. No generic URL/query/exec/SQL/token accessor is provided. Retained history is distinct from freshness and remains policy-neutral.

## Verify

Named tests below are deliverables, not claimed existing results. Exact PASS output is required; absent/skipped tests cannot pass. All fixtures are offline and synthetic.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelAdmission$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelAdmission ' "$o")` | Allowed A reads its canary; missing, invalid, covered-but-different B and revoked A refuse with no rows/counts. Library refuses before zero Store reads, direct Store bypass refuses independently, and wrong returned scope refuses. Removing either production admission check fails its bypass control. |
| 2 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelCoverage$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelCoverage ' "$o")` | Complete-empty differs from never-read/partial/unavailable/stale. Failed/partial page observations keep prior rows and LastComplete; foreign records and temporal regression refuse. Permission loss signals required retention decision without choosing an action; each later non-complete failure keeps rows withheld until a fresh complete full inventory under the current matching admission, with LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. The recovery plan forces full inventory; incremental and not-modified results cannot clear the unavailable baseline. Removing the planner hold (`permission-hold-planner`), marking partial complete, advancing watermark or deleting unseen records fails the oracle. |
| 3 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelScanPlanning$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelScanPlanning ' "$o")` | Explicit time/policy yields first/full and overlapping incremental plans, periodic repair and bounded backoff. Complete inventory absence means not-in-inventory, never closed; partial/incremental absence does nothing. Not-modified never promotes partial coverage. Remove overlap or infer closure from absence and the test fails. |
| 4 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorStoreConformance$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorStoreConformance ' "$o")` | Shared conformance exercises reference Store with caller checks bypassed: wrong admission and incomplete watermark advance refuse, authorized complete update succeeds, expected-version mismatch preserves old snapshot; direct Store updates cannot clear a permission-loss hold without a fresh complete full inventory under the current matching admission and LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. Crafted CompleteScan=true with non-complete coverage, incremental recovery and stale/replayed completeness refuse with the planner bypassed. Removing the independent Store hold (`permission-hold-store`) or making the reference Store permissive fails its bypass control. No SQL/restart acceptance is claimed. |
| 5 | check:ci +flow | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelFlow$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelFlow ' "$o" && GOWORK=off go test -count=1 ./...)` | External package consumer applies synthetic observation then queries exact admitted scope through production API; canary and complete-empty controls pass, unknown/stale remain explicit, caller mutation cannot alter stored data. Existing root/OpenIssues suites pass. |
| 6 | check:ci +mutation +dereference | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/mirror && (cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelBoundaries$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelBoundaries ' "$o")` | Exit 0; source checker prints 0 and the test emits its named PASS; actual source-only checker rejects os/syscall imports including raw/escaped/build-tagged plants. Implement TestMirrorKernelBoundaries in the module suite to reject HTTP/process/database dependencies, new module requirements and root imports of mirror. Missing package/check execution fails. |
| 7 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/46 --delivery a07f6cc85ad145a5a5b634f2f2a046974f44cf44 --base 2a796ec43b59f4fa4bf97c5ebfedac869beffb00` | The literal pins name the actual /46 mainline delivery and its first parent, fetched from main. Exit 0 corroborates the consumer declarations recorded at that immutable snapshot; unchecked exclusions remain reviewer obligations. This proves path routing, not newer Task/Verify semantics, independent acceptance or activation. |

## Evidence

<!-- Independent verifier records exact source/brief, commands, named PASS and production negative controls. No implementation or acceptance evidence is asserted at authoring. -->

## Review

Gate human, inherited from /37. Review the actual lower-layer refusal, typed unknown/partial states, dependency closure and declared limits. The full persisted mirror, retention choice, service and deployment remain unqualified by this contribution.
