---
brief: assay:assay:desktools-v2:58
title: "Extract the role credential provider and retire internal desktoken callouts"
why: "Every Go caller starts another command to obtain a credential even when the credential is already cached. One internal provider lets the CLI and existing role-owned Go processes share the same custody, cache and mint implementation without repeating command startup or creating a second credential authority."
wave: 1
depends: []
unblocks: ["desktools-v2/59", "desktools-v2/62", "desktools-v2/33", "forge-neutral/40", "desktools-v2/75", "desktools-v2/77"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
outcome: none
version: 3
id: 8c796b94-21a4-4f83-94a0-e1954671972b
authored: 2026-10-16 by performance brief authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
gate-why: "The extraction changes how existing authorized processes obtain credentials. Human acceptance confirms role and installation binding, guard/audit parity, restrictive custody and GitLab rotation semantics survive direct calls; no new process receives credentials."
decision-trigger: spec
consumers:
  - "tools/desk/internal/credential/: follow-up desktools-v2/58"
  - "tools/desk/cmd/desktoken/: follow-up desktools-v2/58"
  - "tools/desk/internal/deskkit/roletoken.go and forgeresolve.go: follow-up desktools-v2/58"
  - "tools/desk/cmd/deskboard, deskdispatch, deskread, deskflip, deskpost, deskpr, deskreply, deskfile, deskclose, deskdisposition, deskevidence, deskroster, issueboard and scanloop: follow-up desktools-v2/58 (inherit direct provider through the shared resolver; inventory actual callsites at pickup)"
  - "tools/desk/internal/deskkit/preflight.go: out-of-scope (coldMintProbe deliberately proves a fresh executable and cold environment; retain its subprocess)"
  - "tools/desk/cmd/deskrelease/: out-of-scope (release minter selection is a separate deployment boundary; inventory and retain its explicit executable contract)"
  - "tools/desk/cmd/cellctl and shell/non-Go consumers: out-of-scope (retain CLI compatibility and their credential/process boundaries)"
  - "statusgen/: out-of-scope (credential-free process; online reads stay on deskread and benefit inside that existing role-owned child)"
  - "tools/desk/internal/forgeexecutor/: follow-up forge-neutral/40"
  - "docs/library-first.md, docs/contracts.md and tools/desk/README.md: follow-up desktools-v2/58"
---

# Brief 58 — Extract the role credential provider and retire internal desktoken callouts

## Context

files:
- `tools/desk/internal/credential/` (planned)
- `tools/desk/internal/deskkit/roletoken.go`
- `tools/desk/internal/deskkit/forgeresolve.go`
- `tools/desk/internal/deskkit/appconfig.go`
- `tools/desk/internal/deskkit/confighome.go`
- `tools/desk/internal/deskkit/custodylink.go`
- `tools/desk/cmd/desktoken/`
- `tools/desk/internal/forgeban/allowlist.go` — and its existing package tests
- `tools/desk/README.md`
- `docs/library-first.md`
- `docs/contracts.md`
- `changelog/credential-provider.md` (planned)

facts (source and date in frontmatter):
- At eba941721, RoleTokenForOwner calls an exec.Command("desktoken", ...) seam, then reads the named file. cmd/desktoken owns the actual GitHub cache/mint and GitLab rotation implementations.
- The 2026-10-16 investigation of installed source e70f08e4 observed 8,256 token invocations and 81 real mints in its bounded local window. This aggregate is motivation, not a fleet baseline or a promise of recoverable wall time.
- Current Go callers already possess their role custody. statusgen is a separate module with no credential authority; docs/library-first.md keeps every online statusgen read behind deskread.
- The provider cannot import deskkit if deskkit imports it. Split typed credential/cache primitives from config, guard, audit and CLI adapters; do not export deskkit or move it into a public SDK.
- L is deliberate: moving the two provider-specific paths with the CLI and shared-resolver compatibility proof is one reviewable retirement. The concurrency policy and instrumented workload are separate briefs 59 and 62.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: internal credential provider
  contract: internal provider/custody parity
  retires: [internal minter subprocess and duplicate CLI credential implementation]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

## Read first

Read the existing implementations listed in Context, the source-pinned performance priority
section of the desktools-v2 README, and docs/library-first.md. Refresh current claims and the
source revision before implementation; dated measurements do not certify current deployment.

## Human decision

Confirm the provider and CLI preserve the same role, installation, host, repository scope, file protection, refresh, stop and audit behavior. Review direct-call evidence and GitLab read-versus-rotation results; no additional credential holder or universal broker is allowed.

Options:
1. **Accept the demonstrated implementation** — accept the bounded change and its stated
   compatibility; grant no new credential, role, endpoint or deployment authority.
2. **Hold or narrow the change** — identify the unsupported boundary and revise it before acceptance.

Default if no answer: none — blocks until answered. The implementer presents the concrete
compatibility and negative-path results before this decision; authoring and offline fixtures
do not activate the change.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure
contact. Use isolated temporary homes, fake providers and synthetic repositories. Preserve
existing custody, stop, trust, audit, budget and unavailable-state semantics. Optional
measurement cannot change the operation result. Stop at implemented; independent verification
owns Evidence. All new paths and named tests below are implementation deliverables.

## Interface contract

The internal provider takes an explicit resolved custody/configuration snapshot, provider host,
role/App binding, account/installation and granted repository scope; it returns credential material
only in memory to the already-authorized caller, its custody path, actual expiry and typed outcome.
It accepts an injected transport/clock and narrow guard/audit adapters. No ambient credential
fallback, process launch, generic URL/token-request service or serializable secret result.
A missing required custody/guard/audit adapter refuses rather than silently bypassing it.
The CLI remains a path-only adapter with compatible flags, exit classes, coverage, TTL and fresh
behavior. Library entrypoints retain lower-layer binding and file-mode checks with CLI bypassed.

## Task

1. Inventory production desktoken invocations and credential readers; record each as migrated, intentionally separate or explicitly deferred, with its owning test. Keep cold preflight, release and shell boundaries when their executable contract is intentional.
2. Extract GitHub cache/install-ID resolution/JWT exchange and GitLab custody/rotation semantics into the internal provider. Supply config, guard, audit and typed errors through acyclic adapters. Retire copied implementations in cmd/desktoken; preserve the existing cache as the single on-disk credential cache and existing permission/owner sidecars.
3. Route desktoken and RoleTokenForOwner through that provider. Existing ForgeFor/GitHubRoleToken consumers inherit direct lookup: with desktoken absent from PATH, a synthetic-cache deskboard/deskdispatch/deskread read still obtains its correct credential and starts zero minter subprocesses.
4. Preserve a live guard check for each externally initiated credential operation; replacing the child must not drop DISABLED, loop/run STOP, heartbeat, custody or audit checks. A cache reuse writes no mint audit row; real mint/rotation/refusal behavior retains its record. Do not replace provider refusals with fallback identities.
5. Return actual expiry from mint metadata; on legacy caches whose expiry cannot be established use the existing conservative reuse bound, never invent a fresh lifetime. Keep --fresh invalidation and granted-permission refresh exact. Rotation remains explicit and serialized on GitLab; read-only lookup never rotates.
6. Update the existing semantic-owner and library-first tables, public tool docs and fragment. Preserve executable packaging and GOWORK=off builds; no independent module or service is justified by this internal consumer set. Preserve the forge-ban assertions: removing an allowed minter subprocess retires its permit, not the ban.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProviderCLICacheParity$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestProviderCLICacheParity ' "$out")` | exit 0; named PASS; CLI and direct API share cache, expiry, sidecars, fresh/TTL and error classes; planted copied-cache behavior or token-on-stdout fails |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestDirectProviderHasNoMinterChild$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestDirectProviderHasNoMinterChild ' "$out")` | exit 0; named PASS; real shared resolver plus fake HTTP with desktoken absent succeeds for its admitted role and rejects another identity; planted exec fallback fails |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProviderLowerCustodyBoundary$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestProviderLowerCustodyBoundary ' "$out")` | exit 0; named PASS; bypass the CLI and reject wrong host/role/installation/scope, missing or loose custody and symlinks while the allowed case succeeds |
| 4 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProviderGuardAuditParity$' ./cmd/desktoken/ > "$out" && grep -F -- '--- PASS: TestProviderGuardAuditParity ' "$out")` | exit 0; named PASS; stop and expiry fail through both adapters, reuse is not a mint row and a real fake-provider mint is recorded once; planted guard/audit bypass fails |
| 5 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestProviderGitLabReadRotationParity$' ./internal/credential/... > "$out" && grep -F -- '--- PASS: TestProviderGitLabReadRotationParity ' "$out")` | exit 0; named PASS; read-only performs zero rotations; explicit concurrent rotation preserves the existing per-role lock and new-token self-check, with wrong-role custody refused |
| 6 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/58` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: CLI and direct API share cache, expiry, sidecars, fresh/TTL and error classes; planted copied-cache behavior or token-on-stdout fails.
- Row 2: real shared resolver plus fake HTTP with desktoken absent succeeds for its admitted role and rejects another identity; planted exec fallback fails.
- Row 3: bypass the CLI and reject wrong host/role/installation/scope, missing or loose custody and symlinks while the allowed case succeeds.
- Row 4: stop and expiry fail through both adapters, reuse is not a mint row and a real fake-provider mint is recorded once; planted guard/audit bypass fails.
- Row 5: read-only performs zero rotations; explicit concurrent rotation preserves the existing per-role lock and new-token self-check, with wrong-role custody refused.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Perf-v3 consumer routing — 19 October 2026

[The attributed performance contract](../../performance-attribution-spec.md) governs future
local measurement. Preserve this brief's existing behavior, tests and legacy v1/v2 privacy;
new qualified work/agent linkage requires the selected v3 workflow profile. /63 owns the
schema/resolver/readers, /64 the shared CLI/env and launch propagation, /65 own-request
cost/timing, /62 desk producer qualification and statusgen/26 standalone phase integration.
Reuse those seams when available and record missing coverage otherwise; no duplicate
writer/parser, speculative ID inference or additional measurement request. Existing credential,
cache/discovery and CLI work remains independently deliverable and is not held for v3.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human; review the demonstrated custody/expiry contract and the lower-layer bypass proof. Approval grants no new identity or deployment.
