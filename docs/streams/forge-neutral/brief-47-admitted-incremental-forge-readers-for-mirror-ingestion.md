---
brief: assay:assay:forge-neutral:47
title: Admitted incremental forge readers for mirror ingestion
why: "The mirror cannot avoid repeated full sweeps while its provider adapters accept no update bound. Two bounded incremental reads through existing admitted transports supply reusable ingestion without a new credential owner, database or service prerequisite."
wave: 2
depends: ["forge-neutral/36"]
unblocks: ["forge-neutral/37", "forge-neutral/48"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new reads handle private issue/change data. The human accepts demonstrated pagination, uncertainty and lower-layer repository scope; this grants no new identity, retention, service exposure or activation."
decision-trigger: spec
decision-issue: 5364
issues: []
schema: brief-v2
version: 10
id: 91f28e5a-12cc-4e3d-8e9a-846bef0fd93d
outcome: none
design: DR-forge-library
authored: 2026-11-03 by statusgen newbrief
sources:
  - docs/consumer-delivery-checks.md
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-02 @ public main 7c62923293d0b640ec90afbf318cae2497908f28; read-only mirror acceleration"
exec-tier: strong
exec-tier-why: "Truncated pagination, scope bypass or unsupported filtering can silently turn incomplete/private observations into complete answers across two providers."
domain: complicated
consumers:
  - "statusgen/forgeread_sdk_test.go: follow-up forge-neutral/47 (synthetic module version fixture only; production online path unchanged)"
  - "forgeread/reader.go APIVersion and forgeread/README.md: follow-up forge-neutral/47"
  - "tools/desk/internal/deskkit/readonlyforge.go, citransport_recorder_test.go, outbound_test.go: follow-up forge-neutral/47"
  - "forgeread/adapters/updated.go, forgeread/adapters/github/updated.go, forgeread/adapters/gitlab/updated.go and tests: follow-up forge-neutral/47"
  - "tools/desk/internal/deskkit/mirrorsource.go and tests: follow-up forge-neutral/47"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go and frozen surface tests: follow-up forge-neutral/47"
  - "docs/streams/forge-gitlab/inventory.md, forgeread/README.md, changelog/forge-incremental-reads.md: follow-up forge-neutral/47"
  - "forgeread/mirror/, workflow/store/, conditional validator caching and populated mirror reader integration: follow-up forge-neutral/37"
  - "forgeclient/ trusted composition migration: follow-up desktools-v2/75"
  - "statusgen production readers: out-of-scope (no new link or online-read path here; synthetic version fixture named separately)"
---

# Brief 47 — Admitted incremental forge readers for mirror ingestion

## Context

files:
- `statusgen/forgeread_sdk_test.go`
- `forgeread/reader.go`
- `forgeread/adapters/updated.go`
- `forgeread/adapters/github/updated.go`
- `forgeread/adapters/gitlab/updated.go`
- `forgeread/adapters/`
- `tools/desk/internal/deskkit/mirrorsource.go`
- `tools/desk/internal/deskkit/mirrorsource_test.go`
- `tools/desk/internal/deskkit/forge.go`
- `tools/desk/internal/deskkit/forge_context.go`
- `tools/desk/internal/deskkit/forge_github.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- `tools/desk/internal/deskkit/forge_surface_deskread_test.go`
- `tools/desk/internal/deskkit/forge_gitlab_test.go`
- `tools/desk/internal/deskkit/forge_github_golden_test.go`
- `tools/desk/internal/deskkit/readonlyforge.go`
- `tools/desk/internal/deskkit/citransport_recorder_test.go`
- `tools/desk/internal/deskkit/outbound_test.go`
- `tools/desk/internal/deskkit/testdata/`
- `docs/streams/forge-gitlab/inventory.md`
- `forgeread/README.md`
- `changelog/forge-incremental-reads.md`

facts:
- Source `7c62923293d0b640ec90afbf318cae2497908f28` contains forgeread/36's admitted Access, both provider adapters and deskkit's scopedAccess/delegating composition. Those are the only new-delivery prerequisites here.
- Existing ListIssues/ListChanges and ReviewQueueSnapshot are available preservation input. Full /37 retains /33 and desktools-v2/09 dependencies for its all-state inventory/review integration; this contribution adds neither another full inventory nor a review planner.
- /37 Task 3's optional conditional transport, persisted validators, not-modified baselines and final joined flow remain /37. This first implementation explicitly reports conditional unsupported and performs bounded full page reads for its requested update interval.
- DR-forge-library chooses one later shared credential-owning forgeclient module. Current source composition stays at its existing admitted boundary and records /75 as its future consumer; no second minter, raw client or custody service is introduced.

layering: Provider adapter request/paging/reduction code reads solely through already-admitted Access; existing backend composition supplies credentials and host/repository admission. A typed deskkit MirrorReadSource concretely consumes the two new Forge operations. It imports no mirror package and does not decide coverage, retention or scheduling. Rows 1–5 exercise actual adapters and composition.

design-fit:
  owner: existing forgeread provider adapters and admitted desk-tools composition
  contract: "enumerated incremental read surface; preserve existing credential/scope semantic owner and register exactly two new read operations"
  retires: []
  weight: "verbs 0; flags 0; two enumerated read operations; no production store/service/credential owner"
  why-add: "The later mirror requires bounded updated-since reads; placing them in the existing admitted adapter owner avoids a second provider client or unused conditional abstraction."

## Read first and architecture pickup

Read the full /37 Task 3, existing adapters and `sharedread.go`, current Forge interface/inventory and guards, `docs/contracts.md`, `docs/library-first.md`, `docs/go-engineering.md` and DR-forge-library. Apply implementation pickup before effects/on resume, recording source, producer/guideline revisions and continue/adapt/hold. The observed /36 capability does not upgrade its lifecycle; the dispatcher applies existing eligibility rules independently.

## Human decision

At implementation design, prepare the demonstrated two-provider incremental read behavior, request bounds and lower-layer scope evidence. The acceptance decision covers this bounded adapter/composition contribution only and grants no new credentials, retained private-data policy, network listener or activation. Options are accept the demonstrated boundary or hold for correction. The executor files the concrete evidence decision at the spec trigger; no response permits no activation.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic evidence preparation may proceed; no private repository data is ingested or served.

## Ground rules

- Isolated owned worktree and draft PR; no main push, merge, release, workflow trigger or live infrastructure query. Export `KUBECONFIG=/dev/null`; fake provider transports and offline fixtures only.
- No generic query/URL/token accessor, ambient fallback or new credential grant. Preserve root OpenIssues wire/dependency behavior and all provider admission controls.
- Stop at implemented; independent verification and human acceptance remain separate. Preserve /37's complete promise, retention decision and historical Evidence.
- No mirror import, store, scheduler, network mode/default, statusgen link, standalone 304 cache or duplicate provider backend. Missing supported provider capability is explicit unavailable/incomplete, never fabricated success.

## Task

1. Define adapter-owned `UpdateQuery` with explicit Since and reduced typed IssueUpdate/ChangeUpdate/Actor results in `forgeread/adapters/updated.go`. Results carry Incomplete/PageCount/ObservedUpTo and explicit conditional-supported status. Preserve provider-qualified identity, actual state and relevant head/source fields. These are transport observations, not mirror-owned coverage/admission or dispatch/approval facts. Freeze the narrow concrete API in the implementation PR.
2. Add `Adapter.ChangesUpdatedSince(ctx, repo, query)` and `Adapter.IssuesUpdatedSince(ctx, repo, query)` for GitHub and GitLab. Use provider-supported update bounds/order where available and actual pagination signals up to an explicit ceiling, preserving the inclusive boundary. GitHub pulls must request sort=updated and direction=desc. Only with that update-descending order may a nonempty page whose every source timestamp is strictly older than Since terminate the walk as complete; otherwise continue through the provider end or report the ceiling as incomplete. For an endpoint without server-side update filtering, use a bounded correct full read/filter and report the capability/cost honestly or return could-not-check; never claim an unsupported parameter filters results. Never return a truncated/failed population as complete. Preserve correct issue versus PR/MR filtering, update ordering and unknown timestamps.
3. Own the additive module API version bump to 1.2.0 in reader.go; preserve EnvelopeSchema=1 and all root reader/wire behavior. Update only the synthetic published-module version and expected API output in statusgen/forgeread_sdk_test.go; preserve its network-disabled file-proxy behavior and every production statusgen import/online path. The independent kernel contribution does not edit reader.go.
4. Baseline conditional behavior is explicitly unsupported. No page validator, 304 cache or inferred not-modified completeness is added here. Preserve enough exact query/page/result provenance for the later /37 optional conditional adapter to extend without changing these results' meaning. Existing OpenIssues methods and fixtures remain unchanged.
5. Extend the frozen Forge interface with exactly `ChangesUpdatedSince` and `IssuesUpdatedSince`; implement both backend methods by delegating to the shared admitted adapters. Add a concrete typed `MirrorReadSource` in deskkit that consumes these operations for an explicit provider/repository/record-kind/update bound and returns their observations/uncertainty. It does not import the /46 kernel or implement its refresh/coverage policy. Reuse scopedAccess and the current native backend transport rather than adding another HTTP/SDK client. A source request cannot pick a role, endpoint or credential.
6. Prove lower-layer scope refusal with the upper caller check bypassed: A-only transport rejects B before any provider request. Both providers' actual method-to-adapter-to-Access paths are tested, including cancellation, malformed response, page-2 failure and ceiling. Exactly two operation names enter the frozen surface and GitLab inventory with this concrete source consumer. Preserve read guards and the current /75 future migration owner; no claim that /75 custody is implemented.
7. Preserve the read-only Forge wrapper's explicit method allowlist, recorder delegation and outbound read classification for both new methods; retain every existing write refusal and run the existing CI-fence suite. Pass request cancellation through the existing admitted helper without a new client or credential owner.
8. Bound aggregate raw JSON response bytes for each operation to 8 MiB across fetched pages, accounting before page decoding/record retention. Budget overflow and page ceilings yield explicit incomplete coverage with zero ObservedUpTo; never expose a usable watermark from an incomplete walk. This bounds retained multi-page payload, not the existing native single-response io.ReadAll allocation; that transport allocation bound is an explicit pre-activation obligation of /37 Task 8 and Verify row 23 for both existing provider backends, including later shared-library qualification. No new flag, client or credential owner.
9. Keep performance attribution honest: count directly observed provider page attempts, preserving failure/unknown classes; no extra calls for instrumentation and no future waiter multiplication. Reuse current perfrec conventions at available composition points and record missing instrumentation as a gap. Later source-to-kernel mapping, conditional validation, actual coalescing, persistence, review population and reader cutover stay with /37.

## Interface contract

The two provider methods and their DTOs are an admitted read capability, not authority. MirrorReadSource is the concrete consuming composition. /46 and /47 can proceed independently because /46 owns pure mirror identities/planning while this contribution owns provider DTOs; /37 maps them at integration. No new wire DTO layer is introduced. Conditional support reports unsupported until the parent contribution supplies and qualifies it.

## Verify

Named tests are planned deliverables; require exact PASS lines so missing/skipped tests fail. Fake transports exercise actual shipped adapter/backend code. Production-path mutations are restored before the final pass and their failure evidence is retained.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalPagination$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestIncrementalPagination ' "$o")` | Both providers' three-page controls include exact update-boundary records with correct kinds/state/order. Short page with More continues; ceiling/page-2 failure cannot report complete. Nonzero Since reports unsupported server filtering for GitHub pulls and supported bounds on the other three operations. Captured pulls requests assert sort=updated and direction=desc; omitting those parameters or changing to creation/ascending order fails a production-source mutation. A fully older second pulls page still advertising More stops after exactly two requests with complete in-range results under that ordering. Unsupported server filtering uses visible bounded fallback/refusal. Stopping at page 1 or ignoring More fails the oracle. |
| 2 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalUncertainty$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestIncrementalUncertainty ' "$o" && GOWORK=off go test -count=1 ./...)` | Known-empty, incomplete, cancellation, malformed pages, provider refusal and unknown timestamps stay distinct. Conditional capability is explicitly unsupported while full reads occur. Nonempty page-limit or 8 MiB aggregate-budget overflow returns incomplete with zero watermark; byte-limit reason is explicit. Fabricated complete/conditional support fails; old OpenIssues/issue-state/root closure suites pass. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalReadComposition$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestIncrementalReadComposition ' "$o")` | Typed MirrorReadSource consumes actual two Forge methods on both backends, delegating through adapters/Access with expected pages and normalized records. Failed/partial input stays uncertain. Removing delegation or bypassing the real source path fails its request/result oracle. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalReadLowerScope$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestIncrementalReadLowerScope ' "$o")` | With the source caller check bypassed, A-only admitted transport refuses B before zero provider requests; valid A is a positive control for both providers. Removing the production lower scope check sends the canary request and fails. |
| 5 | check:ci +mutation +dereference | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeSurfaceUnchangedByDeskread$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeSurfaceUnchangedByDeskread ' "$o" && GOWORK=off go test -count=1 -v -run '^TestForgeGitlabCoverage$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeGitlabCoverage ' "$o" && GOWORK=off go test -count=1 -v -run '^TestAmbientTokenGuardCoversForgeread$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestAmbientTokenGuardCoversForgeread ' "$o")` | All three named existing tests actually run/PASS. Frozen surface gains exactly the two methods and concrete inventory consumer. Third unregistered operation, missing inventory or adapter ambient-token plant fails. Extend existing guards if new composition files require coverage. |
| 6 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/47 --delivery 16ead8da012b57e0865f354e55447f55ca7c5600 --base 192433c1578f4417137ab48670534405f4de25ca` | The literal pins name the actual /47 mainline delivery and its first parent, fetched from main. Exit 0 corroborates the consumer declarations recorded at that immutable snapshot; unchecked exclusions remain reviewer obligations. This proves path routing, not newer Task/Verify semantics, independent acceptance or activation. |
| 7 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReadOnlyForgeRefusesEveryWriteMethod$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestReadOnlyForgeRefusesEveryWriteMethod ' "$o" && GOWORK=off go test -count=1 -v -run '^TestCITransportRefusesWriteMethods$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCITransportRefusesWriteMethods ' "$o" && GOWORK=off go test -count=1 -v -run '^TestOutboundForgeWrapsEveryWriteMethod$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestOutboundForgeWrapsEveryWriteMethod ' "$o")` | Existing read-only and CI write fences still refuse every write and outbound classification covers the new methods; each named test runs/PASS. Removing a write fence or misclassifying an added method fails its production-path control. |
| 8 | check:ci +flow +mutation | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off GOPROXY=off GOSUMDB=off go test -count=1 -v -run '^TestReaderSDKExternalConsumer$/^a_consumer_builds_against_the_published_module_with_no_workspace$' . > "$o" && grep -F -- '--- PASS: TestReaderSDKExternalConsumer/a_consumer_builds_against_the_published_module_with_no_workspace ' "$o" && GOWORK=off GOPROXY=off GOSUMDB=off go test -count=1 -v -run '^TestReaderSDKExternalConsumer$/^omitting_the_module_publication_fails_the_build$' . > "$o" && grep -F -- '--- PASS: TestReaderSDKExternalConsumer/omitting_the_module_publication_fails_the_build ' "$o")` | The isolated external-consumer fixture publishes current source through its file proxy, builds without a workspace and prints unchanged schema plus API1.2.0. Missing publication fails the build as a negative control; both named subtests run/PASS. No production online-reader/import change. |

## Evidence

<!-- Independent verifier records source/brief, exact commands, named PASS and negative controls. No result is asserted at authoring. -->

## Review

Gate human inherited from /37. Inspect actual provider behavior and lower-layer repository refusal, all-state/partial semantics, concrete consumption and the explicit unsupported conditional baseline. Full mirror retention, durable ingestion and serving remain /37's obligations.
