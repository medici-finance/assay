---
brief: assay:assay:forge-neutral:50
title: Join admitted incremental reads to mirror observations
why: "The delivered incremental readers and mirror kernel have no connecting source. A bounded mapper lets read-only ingestion qualify independently of the PostgreSQL store, execution and serving, while preserving incomplete data as unknown."
wave: 4
depends: ["forge-neutral/46", "forge-neutral/47", "forge-neutral/48"]
unblocks: ["forge-neutral/37"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "This adapter can carry private forge observations. Human acceptance covers the demonstrated admission, uncertainty and native-bound flow; no private ingestion, retention action, service or activation is authorized."
decision-trigger: spec
decision-issue: 5398
issues: []
schema: brief-v2
version: 3
id: 1a152bcf-dd69-451a-b02c-438e7ecc8e63
outcome: none
design: DR-forge-library
authored: 2026-11-03 by statusgen newbrief
sources:
  - "freshness-checked 2026-11-03 @ 438310c23194b20a5cb1ceba998fb6b42b30bf0f; F-mapper-owner routing repair from PR #5397 and implementation PR #5406"
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/streams/forge-neutral/brief-46-pure-read-only-mirror-kernel-and-conformance.md
  - docs/streams/forge-neutral/brief-47-admitted-incremental-forge-readers-for-mirror-ingestion.md
  - docs/streams/forge-neutral/brief-48-bound-native-mirror-response-allocation-before-decoding.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-03 against App-fetched main 863c43dbcd84d6f05ac86f9197c586e14a3a3828"
exec-tier: strong
exec-tier-why: "Two-provider provenance, optional-field presence and incomplete watermarks must remain correct through independent admission and actual composition."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/mirroringest.go and mirroringest_test.go: fixed-here"
  - "tools/desk/internal/deskkit/mirrorsource.go and tests: out-of-scope (reuse the existing enumerated reads unchanged)"
  - "forgeread/mirror/types.go, planner.go, scan.go and presence_test.go: fixed-here (one observation owner and synthetic joined proof)"
  - "forgeread/mirror/mirrortest/: out-of-scope (reuse the existing synthetic reference adapter unchanged)"
  - "docs/contracts.md and forgeread/README.md: fixed-here"
  - "workflow/store/ and persisted joined-watermark proof: follow-up forge-neutral/37"
  - "forgeclient/ composition relocation: out-of-scope (no forgeclient implementation exists here; desktools-v2/75 retains its later relocation contract)"
  - "statusgen/ and desk command callers: out-of-scope (no production consumer cutover or new online path)"
---

# Brief 50 — Join admitted incremental reads to mirror observations

## Context

files:
- `tools/desk/internal/deskkit/mirroringest.go` (planned)
- `tools/desk/internal/deskkit/mirroringest_test.go` (planned)
- `tools/desk/internal/deskkit/mirrorsource.go`
- `tools/desk/internal/deskkit/mirrorsource_test.go`
- `forgeread/mirror/types.go`
- `forgeread/mirror/planner.go`
- `forgeread/mirror/scan.go`
- `forgeread/mirror/presence_test.go` (planned)
- `forgeread/mirror/mirrortest/`
- `docs/contracts.md`
- `forgeread/README.md`
- `changelog/mirror-observation-source.md` (planned)

facts:
- Inspected main 863c43db contains /46, /47 and /48; no workflow module, mirror.Source implementation or populated production mirror caller exists.
- MirrorReadSource.Read already binds admitted native backends to two enumerated operations, validates provenance and retains endpoint coverage; reuse it without another page walker or credential resolver.
- adapters.IssueUpdate has BodyKnown and nil-versus-empty collections. mirror.Observation lacks BodyKnown and cloneRecords currently collapses known-empty slices to nil. Preserve presence at the canonical observation boundary before claiming mapped completeness.
- mirror.PlanRefresh, PlanScan and QueryScope own planning/query semantics. This adapter supplies observations, never scheduling or ownership.

layering: trusted deskkit composition implements the existing mirror.Source interface over MirrorReadSource. The pure mirror package owns only additive observation/presence semantics and detached copies; no transport, clock discovery, credential or scheduler moves into it. Tests call the real native adapter, mapper, planner and reference Store.

design-fit:
  owner: forgeread/mirror observation semantics; trusted deskkit source composition
  contract: S-forge-mirror
  retires: []
  weight: "verbs 0; flags 0; stores 0; one adapter implementing the existing Source port"
  why-add: "Existing source and observation owners are disconnected. Reuse both; a second walker, store or source kernel would duplicate their meaning."

## Read first and architecture pickup

Read the full current /37 Task and Verify, /46–48, DR-forge-library, docs/library-first.md and docs/go-engineering.md. Follow implementation-pickup; bind inspected source and contracts on the PR, recording continue/adapt/hold. These are legacy brief-v2 references because negotiated milestone-reader support is unqualified. No new milestone schema or acceptance authority is introduced.

## Human decision

Accept the demonstrated read-only observation adapter, or hold for a named correction. Evidence covers independent scope admission, private-data isolation, incomplete/unknown observations and both native providers. Acceptance grants no credential, private-data ingestion, retention action, database, service exposure or deployment.

Default if no answer: none — blocks until answered

This gate covers acceptance and activation. Offline implementation and synthetic fixture preparation may proceed before acceptance; no private population is ingested or served.

## Ground rules

- Existing architecture/import and cross-module CI registries must include actual new inputs when required; do not weaken their assertions.

- Own isolated worktree and draft PR. Synthetic transports/data only; no live infrastructure, default DSN, credential discovery, new endpoint, command, flag, daemon, serving, polling loop or client fallback.
- Preserve all parent /37's 23 original Verify Class/Command pairs, remaining Tasks, retention/human gates and persisted row 23. This source-only proof cannot qualify PostgreSQL, shared coalescing or full /37 acceptance.
- Reuse /47 paging and /48 native bounds. Preserve existing root forgeread API/envelope and pure-kernel dependency boundary. An additive observation field is versioned under its existing owning contract; do not expand the frozen root surface.
- Stop at implemented; never edit historical Evidence, lifecycle or verifier markers.
- Missing mandatory context holds only affected work; no substitute store, permission inference or authority may fill a gap.

## Task

1. Add ScanPlan.PriorObservedUpTo, populated only from the selected Snapshot by canonical PlanRefresh. PlanScan independently requires that baseline to equal its own snapshot watermark and still checks exact scope/version, temporal regressions, admission and completeness. This retained maximum is not fresh coverage or permission. Source composition rejects a future baseline and only a qualified complete walk may report max(prior baseline, actual returned update maximum); incomplete, truncated and failed scans report zero. Do not infer the prior bound from Since plus an assumed overlap. Implement mirror.Source.Scan in trusted deskkit composition, constructed from the existing admitted MirrorReadSource, a trusted bound source-admission ID and an explicit trusted observation clock. Derive that ID alongside the source from the canonical credential owner, never from a request. Match Scope.AdmissionID and the current Admission.ID to that bound ID before and after the read; recheck current authorization before observations escape. This checks consistency of the supplied binding, not independent proof that composition selected the right credential installation; the real credential-owner/source/store join remains /37. Each call independently validates current Admission, exact admission id/provider/host/repository/kind and ScanPlan scope/window before transport. Do not accept caller-selected endpoints, credentials or a source route different from the constructor's bound identity. Keep native lower admission as a separate check.
2. Delegate to exactly the existing issues/changes operation and its bounded paging. Preserve record ids, provider-specific change kind, state/reason, title/body presence, author, labels, requested individual reviewers, head revision and timestamps. GitHub repository case is canonicalized by trusted composition; GitLab project identity is not silently case-folded. Unqualified case aliases refuse. Reviews/checks/comments/teams not fetched remain unknown.
3. Add BodyKnown to the canonical mirror observation and preserve non-nil empty versus nil slices through mapping, planner and cloned snapshots, with detached mutable collections. Unknown bodies cannot become known-empty; valid empty bodies must remain known-empty. Update S-forge-mirror and README, retaining root API/envelope compatibility.
4. Map only complete qualified endpoint walks to Complete. Incomplete/limited results have zero ObservedUpTo and no conditional validator/304 claim; filtered-empty complete reads cannot fabricate or regress a source stamp. Preserve original errors and typed limit/status causes; failed scans expose no successful population. Status-only 403/404, including oversized or truncated bodies, never becomes confirmed permission loss, authoritative absence or retention authority. Return conservative unavailable/unknown without selecting or running purge policy.
5. Exercise both providers through real native reads → MirrorReadSource → mapper → PlanScan → reference Store → QueryScope with synthetic data and explicit time. Include full then overlapping incremental scans, newest-record removal leaving older nonempty rows, complete-empty and filtered-empty inventories, unknown fields, duplicate/foreign records, partial pagination, native and aggregate limits, transport/refusal/cancellation and later success. Incomplete or failed scans preserve prior LastComplete/inventory and make no closure/purge claim. This fixture uses a reference Store, never claims persistence or global fill ownership.
6. Run independent source-layer and lower native scope bypass controls and actual production-path mutations for delegation, source binding, field presence and incomplete-watermark handling. Include exact restored passing evidence and the existing native-bound/read-only/pure-kernel suites. Record implementation/consumer routing and a fragment. Preserve the explicit /37 persisted joined proof.

## Interface contract

Use mirror.Source.Scan unchanged. ScanPlan carries the trusted prior watermark with its existing expected version and admission-bound scope; PlanScan rejects a forged or regressed baseline against the selected snapshot before any update. Composition holds the already-admitted source and its trusted bound admission ID. A different otherwise-valid Admission with a matching relabelled Scope cannot reuse that source; current admission and bound identity are checked before and after the native read. These consistency checks do not qualify actual credential-installation selection, which remains the joined /37 composition obligation. ObservedAt comes from the trusted explicit clock, not request data. ScanResult completeness describes the endpoint inventory only, not unrequested fields. No new public network/query/wire API or refresh gate is introduced.

## Verify

All rows use offline synthetic fixtures. Named PASS output is mandatory; no test selector match or skipped test is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorSourceFlow$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorSourceFlow ' "$o")` | Both providers use actual native reads, mapper, planner, reference Store and query. Positive full/incremental/complete-empty controls and request counts pass; removing real delegation fails. No PostgreSQL or production caller claim. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorSourceScope$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorSourceScope ' "$o")` | Wrong admission/provider/host/repository/kind/plan and canceled input refuse before zero requests; a second otherwise-valid Admission plus a matching relabelled scope also refuses. In-flight revocation or admission-identity change drops all observations without Permission failure/retention authority; valid source passes. With mapper checks bypassed, native A-only scope independently refuses B. Removing either actual boundary fails its own control. GitLab unqualified case aliases refuse. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorSourceUncertainty$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorSourceUncertainty ' "$o")` | Partial/malformed/limited/canceled/failed responses cannot advance complete watermark or yield complete-empty; LastComplete survives. Status-only/oversized/truncated 403/404 preserves cause and private canary, without permission-loss/purge authority. Native8 MiB and aggregate8 MiB limits remain effective; actual incomplete-as-complete mutation fails. |
| 4 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorFieldPresence$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorFieldPresence ' "$o" && GOWORK=off go test -count=1 ./...)` | Unknown versus known-empty body/collections survive canonical observation inputs, PlanScan and detached Store/query clones; unrequested review populations remain unknown. Dropping BodyKnown or normalizing empty collections to nil fails. Frozen root and pure-kernel suites pass. |
| 5 | check:ci +flow | `(cd tools/desk && GOWORK=off go test -count=1 -run '^(TestMirrorNativeResponseBounds\|TestMirrorResponseBoundIsolation\|TestReadOnlyForgeRefusesEveryWriteMethod\|TestForgeSurfaceUnchangedByDeskread\|TestForgeGitlabCoverage\|TestAmbientTokenGuardCoversForgeread)$' ./internal/deskkit/)` | Existing native, neighboring operation, write refusal, frozen surface, inventory and ambient-token guard controls still run/pass; no new Forge method or credential surface. |
| 6 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/50 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Pre-merge implementing-branch routing corroborates actual edits; unchecked exclusions remain reviewer obligations. Before post-merge verification bind independently established mainline delivery and first parent with --delivery/--base; routing alone is not producer identity or acceptance. |
| 7 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorPlanBaseline$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorPlanBaseline ' "$o")` | Canonical plan carries the exact prior observed bound with scope/version. Future, forged, cleared or regressed baseline and stale expected version refuse independently before an update; existing raw scan-watermark regression controls stay unchanged. Actual plan-baseline check removal fails its mutation. Row 1 joins a legitimate full source inventory whose former newest record disappeared: complete coverage advances, the retained observed maximum never regresses and unqualified absence never becomes closed. Incomplete scans still have zero source ObservedUpTo and preserve prior LastComplete. |

## Pre-mortem and detection

- Source and planner tests pass without joined native mapping: row 1's actual request/result oracle and delegation mutation.
- An omitted body or collection becomes authoritatively empty: rows 1 and 4, presence mutations.
- Partial or error status advances coverage or purges a private canary: row 3 failure flow and actual complete-flag mutation.
- Outer admission masks a bypassable native boundary: row 2 independent lower-scope control.
- Fixture evidence is called persistent/shared ingestion: explicit review-only limitation; /37's full joined persisted row remains owed.

## Evidence

<!-- Independent verifier records exact source/brief and actual fail-first controls. Authoring supplies no implementation or acceptance evidence. -->

## Review

Gate human. Review actual scope and field-presence flow; maintain the original parent acceptance and activation gates. The pure mapper may be qualified before any SQL, execution or serving capability exists.
