---
brief: assay:assay:forge-neutral:47
title: Admitted incremental forge readers for mirror ingestion
why: "The mirror cannot avoid repeated full sweeps while its provider adapters accept no update bound. Two bounded incremental reads through existing admitted transports supply reusable ingestion without a new credential owner, database or service prerequisite."
wave: 2
depends: ["forge-neutral/36"]
unblocks: ["forge-neutral/37", "forge-neutral/48", "forge-neutral/50"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new reads handle private issue/change data. The human accepts demonstrated pagination, uncertainty and lower-layer repository scope; this grants no new identity, retention, service exposure or activation."
decision-trigger: spec
decision-issue: 5364
ruling: "https://github.com/medici-finance/assay/issues/5364#issuecomment-6923603705"
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

### Independent verification — 2026-11-18, merged main 1a259b85f7b5

Non-implementer verification at merged main `1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c` with statusgen v1.0.87. The brief's risk is `sensitive-data: yes`, so every row is risk-bearing and owes a fail-first witness. Delivery: #5367, merge commit `16ead8da012b57e0865f354e55447f55ca7c5600`, first parent `192433c1578f4417137ab48670534405f4de25ca` (read from main's first-parent history). The merge commit is an ancestor of HEAD, and the brief exists at its first parent.

Governing table: the current Verify table, brief version 10. The delivery held version 7. Later authoring revisions tightened row 1's Expect and replaced row 6's derived merge-base with literal pins; #5377 records that independent re-derivation is owed. `deskpathguard rederive` reports 8 rows, 0 author-added, all present at the base. The delivery itself did not edit the table.

Rows 1–5, 7 and 8 ran in one container-netns admission bracket: `--network none`, read-only root, and a module cache provisioned at bracket open for the admitted tree, run with `--fail-first --delivering-commit 16ead8da012b`. The closed witness follows verbatim. `statusgen verifyrun --bracket-digest` reports `verdict=match`.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalPagination$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestIncrementalPagination ' "$o")` | pass exit=0 sandbox=container-netns | sha256:a96367a32915 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 2 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalUncertainty$' ./adapters/... > "$o" && grep -F -- '--- PASS: TestIncrementalUncertainty ' "$o" && GOWORK=off go test -count=1 ./...)` | pass exit=0 sandbox=container-netns | sha256:a6f9f627d06b | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 3 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalReadComposition$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestIncrementalReadComposition ' "$o")` | pass exit=0 sandbox=container-netns | sha256:13c69b6685de | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalReadLowerScope$' ./internal/deskkit/... > "$o" && grep -F -- '--- PASS: TestIncrementalReadLowerScope ' "$o")` | pass exit=0 sandbox=container-netns | sha256:1f7217c98059 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 5 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeSurfaceUnchangedByDeskread$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeSurfaceUnchangedByDeskread ' "$o" && GOWORK=off go test -count=1 -v -run '^TestForgeGitlabCoverage$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestForgeGitlabCoverage ' "$o" && GOWORK=off go test -count=1 -v -run '^TestAmbientTokenGuardCoversForgeread$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestAmbientTokenGuardCoversForgeread ' "$o")` | pass exit=0 sandbox=container-netns | sha256:032d3142d043 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 7 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReadOnlyForgeRefusesEveryWriteMethod$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestReadOnlyForgeRefusesEveryWriteMethod ' "$o" && GOWORK=off go test -count=1 -v -run '^TestCITransportRefusesWriteMethods$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestCITransportRefusesWriteMethods ' "$o" && GOWORK=off go test -count=1 -v -run '^TestOutboundForgeWrapsEveryWriteMethod$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestOutboundForgeWrapsEveryWriteMethod ' "$o")` | pass exit=0 sandbox=container-netns | sha256:11de78a247b5 | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f unproven · head=1a259b85f7b5 |
| 8 | `(cd statusgen && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off GOPROXY=off GOSUMDB=off go test -count=1 -v -run '^TestReaderSDKExternalConsumer$/^a_consumer_builds_against_the_published_module_with_no_workspace$' . > "$o" && grep -F -- '--- PASS: TestReaderSDKExternalConsumer/a_consumer_builds_against_the_published_module_with_no_workspace ' "$o" && GOWORK=off GOPROXY=off GOSUMDB=off go test -count=1 -v -run '^TestReaderSDKExternalConsumer$/^omitting_the_module_publication_fails_the_build$' . > "$o" && grep -F -- '--- PASS: TestReaderSDKExternalConsumer/omitting_the_module_publication_fails_the_build ' "$o")` | pass exit=0 sandbox=container-netns | sha256:ca521d04c64e | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=192433c1578f green rc=0 · head=1a259b85f7b5 · delivering=16ead8da012b output=sha256:2d5b0d009b5c981f4041d1edfae46f85af4918c3bc2628e02408e0639835c43c log=row-000001.log |
Tree-Digest: sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6925 run=68657a9d2e826363be5eb558f9720d9248d2835213c549d70d2397977e89a593 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c brief=docs/streams/forge-neutral/brief-47-admitted-incremental-forge-readers-for-mirror-ingestion.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=68657a9d2e826363be5eb558f9720d9248d2835213c549d70d2397977e89a593 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c tree=sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453 witness=sha256:e65b1fa3e2f87f0cdc10a8e5fe5d8d4340d9e0553d5c597a814edc253d3598a2 opened=2026-11-18T16:18:48Z closed=2026-11-18T16:25:03Z before=receipt after=check-verifier

Row 6 runs on the host lane, because it dereferences repository objects:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 6 | `statusgen --root . --consumers --brief forge-neutral/47 --delivery 16ead8da012b57e0865f354e55447f55ca7c5600 --base 192433c1578f4417137ab48670534405f4de25ca` | could-not-run: the host `statusgen verifyrun --rows 6 --fail-first --delivering-commit 16ead8da012b` refused before any row ran (exit 2): "--delivering-commit … is refused: it cannot be read through the library read path (native discovery/config/shallow environment profile): GIT_CONFIG_COUNT is set in the environment, which selects a native Git profile the library does not qualify". This is the declared verifier Git environment, and it was not altered to get past the refusal (#6851). | none | 2026-11-18 | assay-verifier-app[bot] (observed the refusal; row not executed) (on-behalf-of human:ian) |

Findings:

- **Rows 1–5 and 7 pass at head.** Each prints its exact named `--- PASS:` lines (grep exit 0). Their base execution is `unproven`. Statusgen reclassified it could-not-run because the module cache was provisioned for the head tree and lacks modules that the base `tools/desk` go.sum requires; the base and head `tools/desk/go.mod` differ. These rows send test output to a file, so the runner could neither confirm nor exclude a stop on module resolution. This gap is in the environment, not the deliverable; it is the class of #4683, a module cache bound to a different tree than the one run. Static read, no credit: the test functions for rows 1–4 do not exist at the base. The tests for rows 5 and 7 do exist there, and the delivery adds the two incremental read methods to their inventories.
- **Row 8 is non-discriminating.** It passes at head and is also green at base: the base log shows both named subtests PASS, base output `sha256:2d5b0d009b5c`. The delivery changed the test's expected API version (1.1.0 → 1.2.0) in the same commit as `forgeread/reader.go`. At the base, the base's own test therefore agrees with the base's code, and the row passes with or without the change. Because the row is risk-bearing, this is a failed acceptance obligation of kind check-definition: the row cannot show that API 1.2.0 was delivered. A repair would assert the 1.2.0 consumer output, or the `APIVersion` literal, independently of the test file the base carries. A definition repair needs fresh dispatch and admission after it lands.
- **Row 6 could not run:** the verifier Git environment and `--delivering-commit` refusal (#6851).

Risk-bearing values in the delivery diff:

- RISK-VALUE: DERIVED — MaxUpdateBytes = 8 << 20 @ forgeread/adapters/updated.go:11 — 8,388,608 bytes, the 8 MiB aggregate budget Task 8 sets. Overflow marks the walk incomplete with reason `byte-limit` (updated.go:96-97), and an incomplete walk exposes no watermark, so a large repository fails safe and never claims a false complete.
- RISK-VALUE: DERIVED — APIVersion = "1.2.0" @ forgeread/reader.go:31 — an additive minor bump from 1.1.0 for the new exported incremental-read surface, per the package's own versioning rule (reader.go:29). EnvelopeSchema is unchanged, because the JSON wire shape did not move.
- RISK-VALUE: DERIVED — "sort": "updated", "direction": "desc" @ forgeread/adapters/github/updated.go:90 — descending update order is what makes early termination at the first record older than `since` sound. Records older than `since` are dropped locally (updated.go:153, 190).
- RISK-VALUE: DERIVED — lower = q.Since.UTC().Add(-time.Second) @ forgeread/adapters/github/updated.go:97 — the server-side `since` is widened by one second to absorb second-granularity truncation. Inclusion is then decided locally against the exact `since` (updated.go:153), so the overlap can only add records that are filtered out, never lose one.
- RISK-VALUE: NAMED, NOT DERIVED — "order_by": "updated_at", "sort": "desc" and updated_after = q.Since RFC3339Nano @ forgeread/adapters/gitlab/updated.go:102,104 — the descending order and local `since` filter (gitlab/updated.go:153, 189) mirror the GitHub adapter. Whether GitLab's `updated_after` bound is inclusive at equal timestamps is provider behaviour, and it could not be checked offline in this run.

**VERIFY: FAIL** — row 8 is non-discriminating: green at base 192433c1578f and passing at head, while risk-bearing (check-definition). Rows 1–5 and 7 pass at head with base unproven (module-cache gap, environment). Row 6 could not run (#6851). Evidence only; the lifecycle status is unchanged.

## Review

Gate human inherited from /37. Inspect actual provider behavior and lower-layer repository refusal, all-state/partial semantics, concrete consumption and the explicit unsupported conditional baseline. Full mirror retention, durable ingestion and serving remain /37's obligations.
