---
brief: assay:assay:forge-neutral:57
title: Qualified local read-only caller authority and current-state producer
why: "The read-only handler needs an actual protected local caller grant and current-state producer. Passive incarnation records, request claims and same-UID socket access cannot supply that boundary; assigning it explicitly avoids putting read serving behind its downstream write executor."
wave: 4
depends: ["forge-neutral/39"]
unblocks: ["forge-neutral/56"]
split-from: forge-neutral/40
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Approve the concrete Task1 local producer/current-state/custody design and bounded profile before implementing that newly selected mechanism. Actual authentication and custody remain unqualified until the existing implementation, negative-control and independent-verification obligations are met. Approving the library mechanism assigns no operational issuer identity, credential, entitlement, lease value or deployment authority."
decision-trigger: spec
issues: []
schema: brief-v2
version: 3
authored: 2026-11-04 by statusgen newbrief
outcome: none
design: DR-forge-library
exec-tier: strong
exec-tier-why: "Claim binding, current authority, revocation and process/key custody must remain correct independently of request parsing and lower Store admission."
domain: complicated
sources:
- docs/proposals/non-ci-architecture/21-cell-operator-surface.md
- docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
- docs/streams/cell-api/brief-03-qualify-mounted-cell-api-authorization-and-privacy.md
- docs/streams/forge-neutral/brief-39-typed-forge-service-contract-and-scoped-client-sdk.md
- docs/streams/forge-neutral/brief-40-authenticated-forge-serving-and-role-executor-custody.md
- spec/operator-incarnation-v1.md
- spec/forge-service-v1.md
- docs/library-first.md
- docs/go-engineering.md
- "Immutable public main b4badb920569f5f3d83504ab21181a6e803d42cd: no forge-service auth producer or query server exists; the inspected incarnation/capability seams are passive or injected. Existing comms signing/transport primitives require a concrete compatibility and custody assessment, not automatic adoption."
consumers:
- "tools/desk/internal/forgeservice/auth/: follow-up forge-neutral/57 (canonical local grant verifier/producer and current-state composition)"
- "tools/desk/internal/celllaunch/: follow-up forge-neutral/57 (bounded existing launcher integration; no lifecycle execution)"
- "tools/desk/internal/cellcontainer/: follow-up forge-neutral/57 (closed producer/query/caller profile extension and actual named-volume isolation qualification; no arbitrary mount builder)"
- "spec/forge-local-authority-v1.md, docs/forge-local-authority.md and docs/contracts.md: follow-up forge-neutral/57 (concrete claims/custody/current-state qualification)"
- "tools/desk/internal/forgeservice/server/: follow-up forge-neutral/56 (consume the actual qualified producer; no handler issuer)"
- "full executor/custody acceptance: follow-up forge-neutral/40 (reuse this local read-only boundary; write authority remains separate)"
- "remote OIDC, workload ingress and mounted API authentication: out-of-scope (R7 retains its distinct caller class and acceptance)"
- "operator policy/quota provisioning, effects, reconstruction and activation: out-of-scope (no new identity, policy value or runtime entitlement)"
---

# Brief 57 — Qualified local read-only caller authority and current-state producer

## Context

files:
- `tools/desk/internal/forgeservice/auth/` (planned)
- `tools/desk/internal/celllaunch/`
- `tools/desk/internal/cellcontainer/`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `.github/workflows/ci.yml`
- `spec/forge-local-authority-v1.md` (planned)
- `docs/forge-local-authority.md` (planned)
- `docs/contracts.md`
- `changelog/forge-local-authority.md` (planned)

single-point-of-failure: the local producer could mistake possession of a passive cell record or socket for current caller authority — actual protected producer/current-state checks remain independent of handler request validation, with lower repository/admission and native-scope checks still required by the consumer.

facts:
- Amendment21 retains external machine mTLS/light-client signed-token requirements and does not choose a concrete implementation. Amendment22 separately specifies a trusted local launcher capability over a protected per-cell channel; R7 owns remote login integration. Distinguish those caller classes before claiming inconsistency or compatibility.
- The local capability must bind stable authenticated caller/represented actor, cell/audience, exact provider/host/repository and admitted source/composition, read-only operation, current incarnation/generation, policy/configuration/access epochs and finite expiry. Scope strings and role/environment headers supply none of that authority.
- The current-incarnation contract explicitly excludes live authority/key provisioning. Passive CellOwnership, celllaunch.Permit and injected capability/current-state interfaces are not a qualified producer. Existing comms signing and mTLS primitives are candidates for reuse, not proof of forge claim binding or role custody.
- A same-UID socket ACL is not role separation. Restored provenance is never current authority; operational restoration must rotate/requalify under its existing owner.

layering: one canonical R3 local producer/current-state adapter supplies the read-only consumer. Reuse supported primitives and the existing launcher; no handler issuer, remote login platform, second credential broker, workflow execution or new policy owner.

design-fit:
  owner: canonical R3 local caller/current-state and custody producer
  contract: forge-local-authority-v1, aligned with retained21/22 and operator-incarnation-v1
  retires: []
  weight: "No new operator CLI verb, default listener or executor. Prefer existing signing/transport primitives; record actual packages/refusals after the concrete profile is selected under the retained requirements."
  why-add: "The required producer is absent; naming its source and qualification obligation removes a hidden semantic dependency on downstream full40."

## Actual-source and mechanism prerequisite

The concrete Task1 proposal is [forge-local-authority-v1](../../../spec/forge-local-authority-v1.md). It is a draft assessment, not a selected operational profile or accepted custody mechanism. It names the actual cellcontainer source/consumer seam before any runtime edit. All original Task and Verify obligations remain.

Before runtime pickup, inspect the immutable current launcher, comms and incarnation owners and configured code roots. Produce a concrete compatibility packet: selected existing primitive/source, exact local caller class, all bound claims, independently current-state source, key/control custody, protected process/channel profile, revocation/expiry semantics and operator inputs. Explain compliance with retained21 external requirements and22's distinct local mechanism. Do not copy a primitive's default TTL, issuer or audience as policy.

If an actual requirement remains unresolved, record NEEDS_CONTEXT with the exact incompatible requirements and bounded supported alternatives at the canonical R3 owner. A human cannot approve an absent producer into existence. No speculative accept-versus-hold issue is filed before this packet is reviewable. Offline contract/primitive investigation may continue, but a port, mock authority or signature round-trip provides no qualified caller/custody credit.

## Acceptance preparation

The decision-trigger is spec. After Task1 supplies the supported mechanism/profile, claims, current-source/custody/revocation design and actual prerequisite evidence/limits, the executor authors a self-contained Human decision asking approval of that design for bounded offline implementation or a named design correction. Primitive/channel evidence is not demonstrated caller authentication. This single spec-stage ruling does not satisfy Tasks2-7, any Verify/Evidence row, implemented authentication/custody proof or production provisioning/activation. Existing56 acceptance and operational credential/profile/deployment gates remain unchanged. Report DECISION-BLOCK READY only with that concrete self-contained design choice so the canonical helper can file it; until then keep the spec decision deferred. No generic accept-versus-hold issue, implied operational issuer/credential or copied TTL default is authorized.

## Ground rules

- Work only in an owned branch/draft. No cluster/production probe, main push/merge, live key read, grant, role change, deployment or activation. Fixture keys are ephemeral synthetic material in owned temporary resources; never load operational signing material to qualify tests.
- No dependency on full40/41, workflow claims/execution/outbox or live reconstruction. Full40 later reuses this local read-only boundary and retains every write/executor/custody obligation.
- No remote OIDC/mTLS platform replacement, generic token minting endpoint or default entitlement/profile/TTL. Missing, unreadable, stale or uncertain current state refuses before protected data release.
- Socket permissions, request claims, passive records, saved checkpoints and authenticated transport peers alone cannot substitute for the exact current caller grant. Preserve independent lower admission/native checks in the later handler.

## Task

1. Complete the actual-source/mechanism packet above and declare the concrete local profile/claims/current-state contract before implementation. Reuse an existing qualified primitive where it fits; if none fits, present the missing bounded design at its canonical owner before selecting a new crypto/issuer/custody mechanism. Record what remains unqualified and the exact source prerequisite; do not route production authority through downstream full40.
2. Implement the canonical local read-only producer/verifier and current-state adapter under that concrete contract. Derive trusted caller/represented actor/role/cell from protected launcher state, bind exact source/admission/repository/operation and current incarnation/epochs/expiry, and compare every claim independently. A caller cannot select its own role, source grant, current-state record, algorithm or key. Admit mirror-query only; no effect/write/remote-login authority is introduced.
3. Integrate with the existing launcher and explicit operator-selected protected channel/profile. Keep signer/control custody outside query consumers and untrusted roles; verification access must not confer signing or current-state mutation. Missing/malformed/unknown configuration refuses before listener/source contact; no default endpoint, ambient credential or live discovery is added. Do not execute operational launch or provisioning.
4. Qualify the actual current-state source across revocation, epoch/incarnation change, expiry, unavailable state and process restart using independently controlled fixtures. Recheck after current-state I/O and at release; never replay a previously valid grant as current authority. Cancellation/deadline/uncertain state yields a bounded sanitized refusal, no protected claims or key/header material.
5. Demonstrate the actual selected custody profile with independent processes/resources, including a sibling/untrusted role unable to read signing material, mutate authority or mint/upgrade a grant. Same-UID ACL-only fixtures must be explicitly rejected as insufficient. Synthetic qualification must exercise the real producer/profile mechanism with ephemeral owned material; a mock, file-mode assertion or port implementation alone cannot pass custody acceptance. Any unavailable containment prerequisite remains could-not-check, never relaxed to a fake proof.
6. Plant compiling production faults for wrong audience/cell/repository/source/operation, trusting caller role/epoch, stale current-state replay, skipped expiry/revocation/final checks and leaked signer/control access. Each independent baseline-passing witness must fail at runtime. Keep a bounded rerunnable manifest and exact restoration; setup, compile or unavailable-profile failures are not kills.
7. Register the semantic contract, exact source/profile compatibility and operator inputs. Wire the actual CI/reader registry to every cross-module input and qualified fixture entrypoint; absent profile/primitive or skipped containment is explicitly unqualified. Supply the actual immutable producer/API and qualified profile to56; keep its runtime/auth pickup held until this delivery exists and its separate acceptance applies.

## Interface contract

The concrete R3 owner supplies a trusted construction path for local caller/current-state validation and a detached bounded grant/result consumed by56. Request claims remain untrusted. Exact signatures/types are declared from the actual selected primitive and producer in Task1, never guessed by the handler or synthesized from passive records. No arbitrary signer, provider operation, SQL admin connection or role upgrade is exposed to consumers.

## Verify

These are prospective implementation commands. Qualification uses the actual selected producer/profile with ephemeral owned fixture material. A missing test/profile, skip, fake authority or mere signing primitive is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityClaims$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityClaims ' "$o")` | Named PASS; Actual producer binds every declared local claim; forged/foreign cell, audience, repository, source/admission, operation, role and epoch refuse. Removing each binding fails an independent runtime witness. |
| 2 | check +flow +mutation +neighbour | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCurrent$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCurrent ' "$o")` | Named PASS; Actual current-state source refuses revoked, expired, restored/stale incarnation, unavailable and changed epochs, including changes during I/O and final release. Previously valid authority cannot be replayed after restart. |
| 3 | check +dereference +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCustody$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCustody ' "$o")` | Named PASS; Actual independently isolated producer/profile denies sibling-role signing-key reads, current-state writes and grant upgrades; positive in-scope local verification works. Same-UID ACL-only or mock proof is rejected/unqualified. No operational key/role is touched. |
| 4 | check +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityLaunch$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityLaunch ' "$o")` | Named PASS; Existing launcher composes only explicit qualified local profile. Missing/unknown configuration makes zero listener/source/key discovery calls; no default endpoint, remote login or effect grant. |
| 5 | check +dereference +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCompatibility$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCompatibility ' "$o")` | Named PASS; Dereference actual retained21/22 requirements and selected primitive/producer contract against closed witnesses for local versus external caller classes, all claims, custody and revocation. Raw primitive presence/default TTL does not qualify the profile. |
| 6 | check:ci +dereference +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCI$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCI ' "$o")` | Named PASS; Actual workflow reaches every primitive/launcher/current-state/custody input and requires qualified named containment PASS. Missing producer/profile and ordinary skips are explicitly unqualified. |
| 7 | check +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 internal/forgeservice/auth/testdata/local_authority_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: local authority production controls killed; exact bytes restored' "$o")` | Owned clean linked feature worktree; every named production guard fault compiles and fails its independent passing runtime witness. Exact bytes restore; absent containment/setup failures do not count. |
| 8 | check +dereference | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/57 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Actual producer/profile/launcher edges corroborate or remain explicitly held. Bind independently established immutable delivery/base for post-merge verification; mocks and routing are not auth, deployment or parent acceptance. Any forge lookup uses the verifier's explicit managed identity. |

## Evidence
<!-- Independent non-implementer evidence; authoring and primitive-only tests pass no caller/custody row. -->

## Review

Gate: human. Review the concrete supported local mechanism/profile, actual current-state source and independent custody/claim controls; operational credentials and activation remain separate.
