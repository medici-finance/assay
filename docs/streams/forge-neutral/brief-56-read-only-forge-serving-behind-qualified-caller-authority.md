---
brief: assay:assay:forge-neutral:56
title: Read-only forge serving behind qualified caller authority
why: "The shared mirror can fill its own misses, but clients still lack a qualified read-only serving boundary. A bounded handler can expose that existing read owner without waiting for write executors, while keeping caller authentication, current admission and private-cache release independently enforced."
wave: 8
depends: ["forge-neutral/55", "forge-neutral/39", "forge-neutral/57"]
unblocks: ["forge-neutral/40"]
split-from: forge-neutral/40
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Accept only the demonstrated caller/current-authority binding and private-data release boundary for mirror-query. A missing qualified producer/profile is an implementation blocker; a human acceptance cannot make one exist or activate a service."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
authored: 2026-11-04 by statusgen newbrief
outcome: none
design: DR-forge-library
exec-tier: strong
exec-tier-why: "Authenticated identity, current admission, shared negative caching, wire metadata and final response release must agree across independently enforced boundaries."
domain: complicated
sources:
- docs/streams/forge-neutral/brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md
- docs/streams/forge-neutral/brief-39-typed-forge-service-contract-and-scoped-client-sdk.md
- docs/streams/forge-neutral/brief-40-authenticated-forge-serving-and-role-executor-custody.md
- docs/streams/forge-neutral/brief-55-operator-provisioning-of-known-read-only-refresh-profiles.md
- docs/streams/forge-neutral/brief-57-qualified-local-read-only-caller-authority-and-current-state-producer.md
- docs/streams/cell-api/brief-03-qualify-mounted-cell-api-authorization-and-privacy.md
- spec/operator-incarnation-v1.md
- spec/forge-service-v1.md
- schemas/forge-service-v1.json
- docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
- docs/library-first.md
- docs/go-engineering.md
- "Inspected actual service39 checkpoint 0dffd00706be92a2a6e785f1235c72e35f23035b: API/schema source is available for authoring, but final owner qualification/publication remains owed. It is not an auth producer."
consumers:
- "tools/desk/internal/forgeservice/server/: follow-up forge-neutral/56 (bounded mirror-query handler only)"
- "tools/desk/cmd/forgeserve/: follow-up forge-neutral/56 (existing planned read-only query entrypoint moved forward from full40)"
- "docs/forge-service.md and docs/enforcement-model.md: follow-up forge-neutral/56 (actual launch/custody bounds; no activation)"
- "tools/desk/internal/deskkit/mirror_serving_test.go: follow-up forge-neutral/56 (actual source/coordinator/SQL/client composition)"
- "forgeread/service and forgeread/client: follow-up forge-neutral/56 (consume actual39 DTO/codec/client; no second wire schema or client retry engine)"
- "canonical qualified R3 caller/current-state/custody producer: follow-up forge-neutral/57 (actual earlier producer/profile; absence blocks production pickup)"
- "workflow/store and explicit operator profile: follow-up forge-neutral/55 (consume existing restricted runtime connection, never provision on reads)"
- "remaining executor/custody and full serving qualification: follow-up forge-neutral/40 (consume this read-only owner)"
- "retention, source/query enrichment, tool parity and full mirror acceptance: out-of-scope (already owned by full37; this contribution delivers none of them)"
- "forge effects, role executor, generic apply, scheduler and activation: out-of-scope (not prerequisites for this read-only contribution)"
---

# Brief 56 — Read-only forge serving behind qualified caller authority

## Context

files:
- `tools/desk/internal/forgeservice/server/` (planned)
- `tools/desk/internal/forgeservice/server/testdata/read_mutations.py` (planned)
- `tools/desk/cmd/forgeserve/` (planned)
- `docs/forge-service.md` (planned)
- `docs/enforcement-model.md`
- `tools/desk/internal/deskkit/mirror_serving_test.go` (planned)
- `tools/desk/internal/deskkit/citrigger_test.go`
- `.github/workflows/assay-workflow.yml` (planned)
- `forgeread/service/` (planned)
- `forgeread/client/` (planned)
- `forgeread/README.md`
- `spec/forge-service-v1.md` (planned)
- `schemas/forge-service-v1.json` (planned)
- `docs/contracts.md`
- `changelog/mirror-read-only-serving.md` (planned)

facts:
- The actual39 wire supports MirrorQuery with Kind, MaxAgeMS and a scope-bound optional Minimum token. Scope fields are claims; the client takes an already admitted transport and explicit endpoint. Neither supplies authentication or credentials.
- Metadata.RetryAt is nullable. Cached is nullable CachedObservation containing original Coverage, EvaluatedAt and nullable AgeMS, with no rows. Permission responses carry neither retry advice nor cache metadata. The selected39 checkpoint is not yet a qualified delivery; implementation must consume its completed owner source.
- Full40 already names the planned read-only query entrypoint `tools/desk/cmd/forgeserve`; this contribution owns that launch path while the parent retains its separate executor entrypoint. It uses the existing CLI framework and adds no default listener or probe.
- The canonical forge-neutral/54 R2 Gate and existing refresh coordinator own source fills, native budgets, failures and retry eligibility. Gate-owned shared failure and retry state reaches only callers currently authorized for that exact source, admission and repository scope. Clients make no provider fallback, automatic cooldown sleep or second refresh loop. The runtime connection cannot initialize the operator-owned profile from /55.
- At the inspected source, the forge auth/server implementations are absent. The current-incarnation contract explicitly has no live authority source or key provisioning. Passive records, matching scope strings, SQL generations and celllaunch.Permit do not authenticate a remote caller.
- The comms signed-envelope/mTLS primitive does not by itself bind all required forge repository/source/admission/access/configuration/policy/current-incarnation claims or qualify their custody. The canonical earlier57 producer must distinguish retained21 external machine/light-client requirements from22's local launcher capability and R7 remote login scope, qualifying actual local claims/custody rather than assuming those caller classes contradict.
- Source50 does not provide full review enrichment or all facts required by current desk-read/review projections. This contribution serves the existing mirror-query only; it is not tool parity or full45 cutover.

layering: the handler consumes the actual39 contract, the canonical read-through owner and an independently qualified R3 caller/current-authority producer. It owns no issuer, key store, source walker, local cache backend or quota policy. Runtime serving uses /55's restricted connection; it holds no operator initialization credential.

design-fit:
  owner: bounded read-only forgeservice/server adapter
  contract: actual39 forge-service-v1 and existing R3/admission/coordinator/R2 owners
  retires: []
  weight: "One already-planned read-only query entrypoint moves forward from full40; no extra executor, issuer, provider walker, cache backend or quota owner. Record actual implementation counts."
  why-add: "The existing wire and shared read owner need one bounded consumer; current auth/custody must come from its actual qualified owner rather than a new handler issuer."

single-point-of-failure: a decoded request or cached scope can be mistaken for a current caller grant. Recheck actual authority at final release and retain independent Store admission/native source checks; authentication absence or uncertainty releases no rows, counts, tokens or cached coverage.

## Read first and actual-source prerequisite

Read39,40,51–55,57, cell-api/03, the current-incarnation/custody contract and the actual selected R3 producer/profile before runtime pickup. Record immutable source, actual API, signed/transport claims, caller mapping, current-state source, key/control custody, expiry and revocation bounds. Consume57's concrete local mechanism/profile compatibility with retained21/22; any unresolved choice belongs at that canonical producer, not in this handler.

**Production/authenticated pickup is HELD while that qualified producer/profile is absent.** Report NEEDS_CONTEXT, name the missing actual source/profile from earlier57, the canonical R3 producer owner. Human approval cannot substitute for missing implementation. Do not infer a port, issuer, signature algorithm, role mapping, universal token or TTL from a passive record or another transport. This is a source prerequisite, not a dependency on full40,41 or workflow execution.

Offline handler/fixture preparation may consume only actual existing ports and must be labelled UNQUALIFIED. Mocks, fake caller authorities, synthetic scope strings and a local listener provide no authentication or serving acceptance credit. The mandatory auth/served-flow rows remain unrun until they exercise the actual independently qualified producer. Do not ask or file an accept-versus-hold fork before that concrete source/profile and evidence packet exist.

## Human decision

After the actual producer/profile and served negative controls are concrete, accept or hold the demonstrated read-only boundary. The packet must show exact caller mapping/claims, current state, custody, expiry/revocation and private-data release behavior. This decision does not choose a speculative issuer or grant operational exposure.

Options:
1. Accept the demonstrated mirror-query contribution, retaining the qualified profile's limits and all separate deployment, retention and remaining parent gates.
2. Hold for a named authentication, admission, privacy or compatibility correction.

Default if no answer: none — blocks until answered

This gates acceptance. Qualified source availability remains independently required. No operational listener launch, credential, profile, client cutover or activation is authorized by authoring, source tests or this future acceptance.

## Ground rules

- Owned isolated branch/draft only; no main push/merge, release, workflow dispatch, production endpoint or cluster contact. Only explicitly owned loopback listeners and the verified disposable PostgreSQL container may run in tests.
- No login scheme, signer/key custody, current-authority source or default entitlement lease is added here. Unknown/unreadable/stale identity or policy holds; /55 does not authenticate a caller.
- No write executor, effect route, source-query endpoint, generic proxy, admin verb, role switch or production initializer. Query composition holds no signer or publisher key and uses only its already qualified admitted read access. In particular FileAt/Diff/MergeBase, effects, projections and review operations remain refused by this slice.
- Preserve the actual39 schema/codec and protocol caps. No truncation disguised as complete, inferred missing facts, global progress token or SQL/provider ordering from opaque token text.
- Preserve full40's complete Tasks, seven Verify Class/Command pairs, gate and Evidence; its typed dependency on this contribution orders later reuse, not acceptance. Full37 and full45 remain owed.

## Task

1. Establish and record the actual-source prerequisite above. Bind the handler's trusted construction to the independently qualified producer/profile, its current CallerAuthority/admission and preinitialized restricted R2 connection. Derive stable caller, represented actor, cell/audience, repository/source/admission, access/configuration/policy epochs and expiry server-side; compare request claims against that current binding. Never turn a role, Scope, token, TLS peer label or profile string supplied by the caller into authority. Missing producer/profile is NEEDS_CONTEXT; fixture-only prep remains unqualified.
2. Add a bounded handler for only service.MirrorQuery using the actual39 strict request/response codec and existing client transport contract. Reject unsupported operations, malformed/oversized/duplicate/foreign payloads and unauthorized scope before lower reads. Reuse existing protocol limits, explicit timeout/profile and canonical read-only composition. Implement the already planned `cmd/forgeserve` query entrypoint using the existing CLI framework, consuming the actual qualified producer and restricted runtime configuration. No configuration, malformed/unknown profile or missing authority must refuse before database contact or listener creation. Start only an explicit operator-selected private channel; print the nonsecret target before contact and never probe an ambient endpoint. No remote ingress/login, default listener, signer/publisher key or executor launch is added. The command is built/tested here; operational startup remains a separate operator act.
3. Compose the existing source/coordinator/R2 Gate/Store, with /55's explicit operator provisioning performed separately. Cold, missing, expired or insufficient cache data invokes the shared server-side fill; warm permitted reads avoid provider work. Failure suppression and eligibility come from the Gate, shared across independently constructed handlers/coordinators, not a handler map or client retry. Preserve source-versus-caller denial, native attempt ceilings, unknown holds and current generation fencing.
4. Recheck current caller/profile/admission/epochs/deadline after all read, observer and coordination I/O, and immediately before releasing the detached encoded response. Sample trusted time at the correct post-I/O boundary. Independent Store/native checks remain even with upper ingress checks bypassed. Permission/uncertain authorization responses contain no rows, totals, progress/source metadata, RetryAt or Cached coverage; no error or diagnostic path leaks provider bodies or credential/header material.
5. Map only actual qualified mirror results to the existing39 states, closed reasons and nullable metadata. Retain original coverage and data time for authorized unfulfilled cached observations; descriptive age is not a freshness grant. Gate retry advice is service-owned; the client neither sleeps nor retries the source. Preserve null/unknown versus measured zero, scope/repository/kind-bound minimum-token equality and protocol bounds. Define and qualify an honest mirror-observation provenance mapping for successful39 metadata: actual canonical snapshot owner/version and original observation time may identify a mirror revision only under an explicit immutable profile. Source50 does not supply a provider revision; never fabricate one, copy an empty Coverage.Revision into required metadata, restamp data time or treat the revision as SQL/provider ordering or permission. If the actual owner cannot supply credible required provenance, hold Unavailable. The current mirror query has no minimum-progress classifier, and a SQL scope version may advance on failure/coverage changes. A token validator or numeric/lexicographic scope version is not progress evidence. If the qualified observation owner supplies no exact progress/provenance comparison, a minimum-bearing query must hold Unavailable; bounded no-minimum reads may still proceed with otherwise credible required provenance. Qualify both outcomes independently. Do not invent a scopewide/global SQL progress token.
6. Build an actual client → handler → qualified authority → coordinator → admitted native source → canonical SQL fixture, using the production operator API and restricted runtime role. Use the actual qualified R3 producer's synthetic profile, not a local allow-all auth replacement. Independently exercise cold/warm/stale/partial/empty/failure/cooldown recovery, two handlers sharing negative state and current revocation. No browser or production provider is contacted. Source review/fact gaps remain explicit; no desk command or projection consumer is cut over.
7. Plant production faults for trusting request claims, skipping final authorization/deadline/time checks, bypassing lower admission, leaking Cached/RetryAt on Permission, client fallback and unshared failure suppression. Each must compile and fail an independent runtime expectation with its baseline passing. Keep a bounded rerunnable control manifest/script and exact source restoration. Distinguish qualified auth/served flow from unqualified fixture/codec preparation.
8. Register the read-only semantic owner and update docs/changelog. Extend the actual dedicated PostgreSQL CI/reader registry so all cross-module inputs trigger and real served tests require explicit DSN and exact named PASS. Preserve current foundation, Store, Gate and provisioned-join qualification and every publication/acceptance gate. Full40 later consumes this owner and adds its remaining executor/custody/effect obligations; it must not duplicate the handler or query launch entrypoint.

## Interface contract

A trusted constructor receives actual qualified authority/current-state and transport/profile composition, an existing read coordinator and the restricted R2 runtime connection. The existing planned forgeserve command composes those owners only after explicit configuration validation; it creates no default endpoint or authority. The request carries only existing39 claims/query data. The handler emits only a bounded validated mirror-query response or explicit refusal; it grants no credential, role, source operation, provisioning or retry authority. Missing qualified producer/profile means NEEDS_CONTEXT, not an implicit mock-backed server.

## Verify

These are prospective delivery tests. Every auth/served row requires the actual qualified R3 producer/profile and completed39/55 source. A fake authority, missing test, skip or synthetic-only preparation does not pass them. All SQL/provider/listener traffic is confined to explicit disposable fixtures.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeReadOnlyAuthority$' ./internal/forgeservice/server > "$o" && grep -F -- '--- PASS: TestForgeReadOnlyAuthority ' "$o")` | Named PASS using the actual independently qualified producer/profile. Forged/expired/revoked/foreign-cell/audience/source/repository/operation and swapped epochs refuse before data release. Trusting request Scope or replacing the producer with allow-all fails; missing actual producer is unqualified, not PASS. |
| 2 | check +flow +mutation +neighbour | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeReadOnlyRelease$' ./internal/forgeservice/server > "$o" && grep -F -- '--- PASS: TestForgeReadOnlyRelease ' "$o")` | Named PASS; revocation/epoch change/cancellation/ageing during actual lower I/O, observer and final release yields zero protected rows/counts/tokens/cache advice. Bypassed ingress still meets independent Store/native checks. Removing each final guard causes a runtime failure. |
| 3 | check +flow +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -race -v -run '^TestMirrorReadServingFlow$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorReadServingFlow ' "$o")` | Named PASS through actual client, handler, qualified auth producer, operator provisioning, restricted role, coordinator, admitted SDK and SQL. Cold/stale data fills in the shared owner; warm data uses zero provider work; changed and complete-empty inventories are honest. No client source fallback or unqualified SeedRefresh/auth substitute. |
| 4 | check +flow +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorReadServingFailures$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestMirrorReadServingFailures ' "$o")` | Named PASS; two independent handlers/coordinators share fixed negative-cache eligibility, suppress retries before cooldown/provider floor and permit one later recovery. Caller refusal never poisons another eligible caller; unknown usage/profile/report holds. Remove canonical sharing or invent known-zero and the control fails. |
| 5 | check +dereference +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeReadOnlyWire$' ./internal/forgeservice/server > "$o" && grep -F -- '--- PASS: TestForgeReadOnlyWire ' "$o")` | Named PASS; independently specified39 schema/codec witnesses preserve nullable RetryAt/Cached, original coverage/time and scope-bound tokens. Permission leaks, unknown-as-zero, truncation-as-complete and unsupported operations fail. Actual minimum-progress absence holds Unavailable without blocking otherwise qualified no-minimum reads; a coverage/failure-only SQL version advance cannot satisfy the token. This wire row alone supplies no actual auth/SQL qualification. |
| 6 | check +mutation | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 internal/forgeservice/server/testdata/read_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: read-serving production controls killed; exact bytes restored' "$o")` | In an owned clean linked feature worktree, each named production guard fault compiles and fails its independent baseline-passing witness; fixture/setup failures are invalid. Exact bytes restore. Mocks cannot qualify actual producer controls. |
| 7 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestWorkflowReadServingCI$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestWorkflowReadServingCI ' "$o")` | Named PASS; actual CI reaches every wire/auth/SQL input, requires explicit DSN/qualified producer fixture and exact served-test PASS. No ordinary no-DSN skip, fake auth replacement or absent package is credited. |
| 8 | check +dereference | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/56 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Actual producer/codec/coordinator/R2 edges are corroborated or explicitly held. Before post-merge verification bind immutable actual delivery/base; routing, primitive presence and offline prep are not authentication, deployment, parent acceptance or tool parity. Any forge lookup uses the verifier's explicit managed identity. |
| 9 | check +flow +mutation +neighbour | `(cd tools/desk && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeReadOnlyLaunch$' ./cmd/forgeserve > "$o" && grep -F -- '--- PASS: TestForgeReadOnlyLaunch ' "$o")` | Named PASS through the actual planned query command and existing CLI framework. Missing/malformed configuration or unavailable authority opens no listener/database/provider contact; a qualified explicit owned private-channel fixture serves only mirror-query. Ambient/default endpoint, signer/publisher key custody and executor/effect route regressions fail. No operational startup is performed. |

## Evidence
<!-- A NON-implementer supplies independent evidence. Authoring and unqualified prep pass no served/auth row. -->

## Review

Gate: human. Actual source/profile qualification, independent verification and acceptance precede any separately authorized deployment or client cutover. Missing producer remains a work blocker, not an approval-shaped substitute.
