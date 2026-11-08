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
decision-issue: 5428
ruling: https://github.com/medici-finance/assay/issues/5428#issuecomment-6631318014
issues: []
schema: brief-v2
version: 13
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
- "forgeread/refresh/types.go: follow-up forge-neutral/57 (additive Limits.Validate delegates the existing canonical policy predicate; auth construction reuses it before key/listener contact)"
- "tools/desk/internal/forgeban/allowlist.go: follow-up forge-neutral/57 (exact existing AuthorityRuntime.command unresolved-argv ledger entry; no forge permit or scanner relaxation)"
- "tools/desk/internal/commstransport/: follow-up forge-neutral/57 (canonical checked Unix listener and additive cancellable Unix dial; no new authority or remote transport)"
- "docs/workspace-callers.md: follow-up forge-neutral/57 (exact non-cell CI qualification recipe census route; no allocator or cleanup authority)"
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
- `tools/desk/internal/forgeservice/auth/testdata/local_authority_mutations.py` (planned)
- `tools/desk/internal/forgeservice/auth/testdata/qualify_fixture.py` (planned)
- `tools/desk/internal/celllaunch/`
- `tools/desk/internal/cellcontainer/`
- `forgeread/refresh/types.go`
- `tools/desk/internal/arch/markers.txt`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `tools/desk/internal/forgeban/allowlist.go`
- `tools/desk/internal/commstransport/`
- `docs/workspace-callers.md`
- `.github/workflows/ci.yml`
- `.github/workflows/assay-workflow.yml`
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

## Review-driven protocol qualification

The current-proof signature must have its own adversarial transport witness: a
valid, revoked grant with an otherwise matching fresh proof signed by another
key remains held. A compiling production control removes only current-proof
signature verification, preserving grant verification. This changes no issuer,
custody, channel or lease mechanism.

Registration retains every entry until explicit Rotate. An existing non-revoked
entry, including an expired one, requires explicit Revoke before registration;
ApplyControl returns ErrRegistrationNeedsRevoke rather than a version conflict.
MaxGrants is a per-generation lifetime registration budget, including revoked
and expired entries; exhaustion returns ErrCapacity. Neither refusal changes the
version, drops history or revives authority. The control wire keeps its uniform
held response. These bounded error/documentation repairs preserve all original
Task, Verify, Evidence and human design obligations.

## Retained-channel qualification correction

The current hosted failure demonstrates socket inode reuse after successful
rebinding: identical device/inode, new owner-only mode/timestamps and a live
connection. Inode inequality therefore cannot prove a new listener. The positive
restart fixture instead requires an actual canonical framed current request and
typed held reply from the fresh, uninitialized controller within the original
absolute one-second deadline. A closed retained socket cannot produce that reply.
No deadline is renewed and a connection alone is insufficient. All production
serve/auth code and the negative root, gate, ownership, active-owner, duplicate,
regular-file and symlink assertions remain unchanged. Compiling rebind/reply
controls qualify this replacement witness; no authority or custody gate changes.

## Shared fixture startup qualification

The host protocol fixture must wait for successful framed responses on all three
actual channels before returning: canonical protected control readback, grant
delivery and a verified signed current proof. Socket presence or connection
acceptance alone does not establish that the server has passed its live gate.
Reuse RequestControl, ReadGrant and Verifier.Authorize with one absolute finite
startup deadline; retries are confined to these fixture reads. Keep the existing
cleanup assertion that every started server stops without an error. Production
startup cancellation remains conservatively held, and no production serve,
authorization, ownership, deadline or custody predicate changes. Preserve all
original Task, Verify, Evidence and human decision obligations.

## Actual-source and mechanism prerequisite

The concrete Task1 proposal is [forge-local-authority-v1](../../../spec/forge-local-authority-v1.md). It is a draft assessment, not a selected operational profile or accepted custody mechanism. It names the actual cellcontainer source/consumer seam before any runtime edit. All original Task and Verify obligations remain.

Before runtime pickup, inspect the immutable current launcher, comms and incarnation owners and configured code roots. Produce a concrete compatibility packet: selected existing primitive/source, exact local caller class, all bound claims, independently current-state source, key/control custody, protected process/channel profile, revocation/expiry semantics and operator inputs. Explain compliance with retained21 external requirements and22's distinct local mechanism. Do not copy a primitive's default TTL, issuer or audience as policy.

If an actual requirement remains unresolved, record NEEDS_CONTEXT with the exact incompatible requirements and bounded supported alternatives at the canonical R3 owner. A human cannot approve an absent producer into existence. No speculative accept-versus-hold issue is filed before this packet is reviewable. Offline contract/primitive investigation may continue, but a port, mock authority or signature round-trip provides no qualified caller/custody credit.

## Acceptance preparation

The decision-trigger is spec. After Task1 supplies the supported mechanism/profile, claims, current-source/custody/revocation design and actual prerequisite evidence/limits, the executor authors a self-contained Human decision asking approval of that design for bounded offline implementation or a named design correction. Primitive/channel evidence is not demonstrated caller authentication. This single spec-stage ruling does not satisfy Tasks2-7, any Verify/Evidence row, implemented authentication/custody proof or production provisioning/activation. Existing56 acceptance and operational credential/profile/deployment gates remain unchanged. Report DECISION-BLOCK READY only with that concrete self-contained design choice so the canonical helper can file it; until then keep the spec decision deferred. No generic accept-versus-hold issue, implied operational issuer/credential or copied TTL default is authorized.

## Human decision

Question: Approve the concrete local caller-authority design for bounded offline implementation and fixture qualification now, or implement and qualify the independently non-restored anchor first as an additional prerequisite?

The decision is the library mechanism/profile, not acceptance of an implemented authentication boundary. The producer/current registry does not exist yet. Existing primitive evidence establishes only that an explicitly owned Linux engine can run the source-bound helper at UID501 with separate named-volume views: producer private/channel write and socket creation, client private exclusion plus read-only-directory write refusal and successful Unix-socket roundtrip, and sibling exclusion. All owned resources were removed. It proves no signing, caller authentication, live current writer, production credential custody or Ubuntu CI.

The proposed design adds one canonical R3 operator-only controller/current registry and verifier under the existing launcher. A dedicated producer container holds the signing key and private control writer, outside query and sibling containers on the same explicitly selected Linux engine. Closed per-channel named volumes provide grant delivery and read-only current-state requests; neither consumer receives the key, writer or engine socket. Existing signing and lock primitives are reused, while legacy container refusals remain unchanged. No generic mint endpoint or arbitrary mount/command builder is introduced.

Grants bind issuer/audience, stable caller/represented actor, cell/role, canonical repository and issue/change kind, exact source/composition/admission, mirror-query only, incarnation/generation, policy/config/access epochs, grant/key reference and finite expiry. A pinned verifier bootstrap checks fresh challenge-bound signed current replies. The live operator gate and exact producer identity serialize updates/current reads; an old token, socket or restored record cannot establish currentness. Request-start and grant leases use private process-local monotonic elapsed time, with explicit wall expiry and final release checks; a wall-only clock cannot renew them. All lease/skew/resource limits remain required explicit operator inputs, with no adopted production values.

Restart invalidates old grants, rotates identity/key and requires trusted re-registration. Whole-host/VM-memory rollback cannot be detected by a local lock or operator boolean/file. Production restore and whole-VM rollback remain UNQUALIFIED and HELD until the existing requirement for a trustworthy anchor outside the restored volume and actual identity-and-key rotation is met. No snapshot/backup restore may establish currentness. Both options retain that custody requirement; neither approves an absent anchor, a supported production restore profile or an exception to rotation. No operational issuer identity, key, credential, policy/entitlement value, endpoint, network target or deployment is selected.

The credential-free/network-none query-verifier is a 57 fixture. Actual56 serving must separately compose its admitted native read credential, SQL/provider connectivity and combined isolation, without the R3 signer/control key. No new source proxy, full40 write executor dependency or client source fallback follows. Actual producer/current-state/custody negative controls, independent verification, existing56 acceptance and operational/workflow publication gates remain owed.

### Options

1. (Recommended) Approve this bounded same-engine local design for OFFLINE implementation and fixture qualification now. Production restore and whole-VM rollback remain UNQUALIFIED and HELD until the required independent anchor and real rotation are qualified; every later proof and operational gate remains.
2. Implement and qualify the independently non-restored anchor FIRST as an additional critical-path prerequisite. Return Task1 for its concrete source, rotation design and qualification before producer implementation. This changes implementation order, does not approve an absent anchor and authorizes no live probe.

Default if no answer: none — blocks until answered

A ruling must name forge-neutral/57 and the selected design option explicitly. No answer or primitive pass authorizes producer-mechanism selection.

## Ground rules

- Work only in an owned branch/draft. No cluster/production probe, main push/merge, live key read, grant, role change, deployment or activation. Fixture keys are ephemeral synthetic material in owned temporary resources; never load operational signing material to qualify tests.
- Keep the existing closed authority Docker adapter: legacy Docker(host) uses ambient PATH/HOME/TERM, background timeout, unbounded combined output and interactive foreground, so it cannot replace the required explicit binary/empty configuration/original context/bounded I/O. The existing forgeban unresolved-argv ledger records that exact adapter as could-not-check; it grants no forge permit and does not certify an operator-selected executable as forge-free.
- Host-local endpoints reuse the canonical commstransport owner checks. Its existing Listen supplies owner-only parent and0600; the authority adapter keeps original exclusive-gate/root/inode claims, disables automatic socket unlink and owns exact cleanup. An additive Unix-only cancellable dial preserves the original context and positive finite timeout after existing owner/type checks. Image-owned channel seeds tighten to0700; no runtime chmod/root initializer or legacy transport relaxation. The existing endpoint reader recognizes the actual inline Dialer Unix call and requires its canonical owner check.
- Route the exact disposable GitHub CI qualification checkout recipe as non-cell temporary compatibility under WS-10 in the existing census, with its retirement condition. No workspace allocator/lifecycle API, new checkout effect, broad exemption or scanner change is introduced.
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

### Verification run 2026-11-08 at 2e1ddcbd72c2 (desk tools v1.0.78) — claude-opus-5-5-verifier

Delivering merges on main: #5452 (4680203b0542) and #5524 (7c935394fbc9). Fail-first base: 4682059231e9, the first parent of the #5452 merge, before any of tools/desk/internal/forgeservice/ existed. The brief's risk line answers `sensitive-data: yes`, so every row is risk-bearing and carries a fail-first witness. Rows 1-5, 7 and 8 ran on the host; row 6 (check:ci) ran in the container-netns admission bracket below.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityClaims$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityClaims ' "$o")` | pass exit=0 | sha256:52afd43c052f | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCurrent$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCurrent ' "$o")` | pass exit=0 | sha256:92cbb7c2cea5 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
| 3 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCustody$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCustody ' "$o")` | fail exit=1 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityLaunch$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityLaunch ' "$o")` | pass exit=0 | sha256:412b6161d818 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
| 5 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCompatibility$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCompatibility ' "$o")` | pass exit=0 | sha256:5b31fcabce1a | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
| 7 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 internal/forgeservice/auth/testdata/local_authority_mutations.py > "$o" && cat "$o" && grep -F -- 'PASS: local authority production controls killed; exact bytes restored' "$o")` | fail exit=1 | sha256:c3956c61bb75 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=2 · head=2e1ddcbd72c2 |
| 8 | `GOWORK=off go -C statusgen run . --root .. --consumers --brief forge-neutral/57 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pass exit=0 | sha256:223966c9fc4b | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
Verification-Attestation: medici-finance/assay#5813 run=4659e50fc8aa98cd716a486b435efd8a83a5c5c14ca4f4d28ec9a28651f66851 source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f brief=docs/streams/forge-neutral/brief-57-qualified-local-read-only-caller-authority-and-current-state-producer.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 6 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeLocalAuthorityCI$' ./internal/forgeservice/auth > "$o" && grep -F -- '--- PASS: TestForgeLocalAuthorityCI ' "$o")` | could-not-run exit=1 sandbox=container-netns — image module cache predates go.sum (github.com/mattn/go-isatty v0.0.24 vs v0.0.20): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:1dd2902cb92e | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=4682059231e9 red rc=1 · head=2e1ddcbd72c2 |
Tree-Digest: sha256:1d843a770e690a7bd01d131ad76d41f0ed001b69b9bdc86fdb9bd443b1d0db82
Container-Image: sha256:576cb2e49407447769b79a32af370a23e1c108d62842b8f1772a809d1d3b7617
Verification-Attestation: medici-finance/assay#5813 run=4659e50fc8aa98cd716a486b435efd8a83a5c5c14ca4f4d28ec9a28651f66851 source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f brief=docs/streams/forge-neutral/brief-57-qualified-local-read-only-caller-authority-and-current-state-producer.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=4659e50fc8aa98cd716a486b435efd8a83a5c5c14ca4f4d28ec9a28651f66851 source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f tree=sha256:1d843a770e690a7bd01d131ad76d41f0ed001b69b9bdc86fdb9bd443b1d0db82 witness=sha256:2fe2cf91e057623f1bdad1f623947350a8a247c33cc6c97393151950885bcd22 opened=2026-11-08T07:02:34Z closed=2026-11-08T07:05:49Z before=receipt after=check-verifier

Attempts: (1) 2026-11-08T07:02:31Z to 07:05:49Z, row 6 inside the container-netns admission bracket above, on the linux/arm64 manifest of the pinned verify-runner image; (2) 2026-11-08T07:06:04Z to 07:06:19Z, rows 1-5, 7 and 8 on the host. Both ran under `statusgen verifyrun --fail-first --base 4682059231e9`.

Fail-first: every row run at base 4682059231e9 is red there (rows 1-5 and 8 rc=1, row 7 rc=2, row 6 rc=1 in the container), because the package under test does not exist at base. Rows 1, 2, 4, 5 and 8 pass at head and fail at base, so each of those discriminates. Rows 3 and 7 are red at both head and base, so they carry no discriminating witness from this run.

Rows 3 and 7 were run but are not credited, and neither result is an observed failure of the code. Row 3's test skips itself with `UNQUALIFIED: explicit owned authority fixture packet required` when no owned custody fixture packet is supplied (`custody_test.go:22-26`), so the row's grep finds no PASS line and the output is empty (sha256:e3b0c44298fc is the digest of empty output). Row 7's mutation script stops with `UNQUALIFIED: explicit source-bound owned custody packet required` (`local_authority_mutations.py:59-61`); its output digest sha256:c3956c61bb75 is exactly that one line. The only producer of that packet is `testdata/qualify_fixture.py`. It builds a source-bound image on a Linux container engine, runs the row 3 and row 7 checks itself, and removes the image when it exits. The CI job `forge-local-authority` runs it on ubuntu-24.04. Nothing in the Verify table hands the packet to `verifyrun`, so these two rows cannot pass for any non-implementer verifier as written. That is a check-definition gap in the table (medici-finance/assay#5817). Running the qualification script by hand would hand-run rows 3 and 7, and this verification did not do that.

Row 6 was not run: the pinned verify-runner image's module cache predates the verified tree's go.sum (github.com/mattn/go-isatty v0.0.24 vs v0.0.20). The row stopped on module resolution before its check ran. This is the known image gap medici-finance/assay#4683.

Risk-bearing values (the brief answers `sensitive-data: yes`; production literals under `tools/desk/internal/forgeservice/` and `tools/desk/internal/cellcontainer/`, ranked):

RISK-VALUE: DERIVED — challenge length = 32 @ tools/desk/internal/forgeservice/auth/controller.go:358 — a fresh 256-bit challenge per current-state read, matched by the verifier's `make([]byte, 32)`; a shorter value makes a replayed proof guessable, and a longer one adds nothing.
RISK-VALUE: DERIVED — grant domain separator = "forge-local-grant-v1\n" @ controller.go:277 (checked at controller.go:365, verifier.go:84) — one ed25519 key signs both grants and current-state proofs, so the separator must differ from "forge-local-current-v1\n" (controller.go:386, verifier.go:95) or a signed current-state proof could be presented as a grant.
RISK-VALUE: DERIVED — file mode = 0600 @ controller.go:106/111/116, controller.go:221, channels.go:160 — the live lock, bootstrap records and channel files are owner-only, so another local user can neither read nor plant them; the lock check refuses any other mode.
RISK-VALUE: DERIVED — consumer container flags = `--read-only`, `--cap-drop ALL`, `--network none`, readonly mounts @ cellcontainer/authority_profile.go:160/167 — consumers read current state but cannot write it, and only the producer receives the private volume.
RISK-VALUE: DERIVED — frame cap = 128*1024 @ codec.go:57/64, channels.go:213 — equal to celllaunch.MaxRecordBytes, which config validation also enforces (types.go:77), so the authority channel accepts no frame larger than the launcher record bound.
RISK-VALUE: NAMED, NOT DERIVED — maxRegistryEntries = 1024, maxChannelConnections = 64 @ types.go:26-27 — operational ceilings on memory and connections; the brief does not derive them.

Verdict basis: five of eight rows are credited, and none shows a failed obligation. Rows 3 and 7 cannot be credited until the Verify table can supply the owned custody packet to `verifyrun`. Row 6 waits on medici-finance/assay#4683.

**VERIFY: BLOCKED** — rows 3 and 7 cannot be credited: the table's custody-packet input has no verifier-runnable producer (medici-finance/assay#5817). Row 6 was not run because of image gap medici-finance/assay#4683. Rows 1, 2, 4, 5 and 8 pass, and each is red at base.

## Review

Gate: human. Review the concrete supported local mechanism/profile, actual current-state source and independent custody/claim controls; operational credentials and activation remain separate.
