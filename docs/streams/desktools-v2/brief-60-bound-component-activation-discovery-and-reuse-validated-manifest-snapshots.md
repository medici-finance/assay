---
brief: assay:assay:desktools-v2:60
title: "Bound component activation discovery and reuse validated manifest snapshots"
why: "Each short desk command walks the checkout for component manifests before doing its own work. Reusing a validated discovery snapshot removes repeated startup scans while preserving additions, removals, local changes and inactive-component refusals."
wave: 1
depends: []
unblocks: ["desktools-v2/62"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 2
id: b88b1948-7284-4281-b8a4-e7d128862840
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
consumers:
  - "tools/desk/internal/deskkit/activation.go: fixed-here (reference owner and shared routing)"
  - "tools/desk/internal/deskkit/activationindex.go: fixed-here (bounded validated slot)"
  - "tools/desk/internal/deskkit/activationindex_test.go: fixed-here (fresh reference and lower-boundary negative controls)"
  - "tools/desk/cmd/: out-of-scope (unchanged authority adapters inherit the shared runtime owner; README enumerates all 41 callers and the three named commands have compiled qualification)"
  - "tools/desk/internal/deskkit/activationstartup_test.go: fixed-here (actual compiled board/dispatch/token startup workloads)"
  - "tools/desk/cmd/deskboard help/version entrypoints: follow-up desktools-v2/20"
  - "tools/desk/cmd/deskdispatch help/version entrypoints: follow-up desktools-v2/21"
  - "tools/desk/cmd/desktoken help/version entrypoints: follow-up desktools-v2/33"
  - "statusgen/: out-of-scope (different stream/brief discovery semantics; reuse perfrec and investigate through statusgen/26, not an import of deskkit activation)"
  - "tools/desk/README.md: fixed-here (bounded discovery and measured costs)"
  - "docs/contracts.md: fixed-here (semantic owner)"
---

# Brief 60 — Bound component activation discovery and reuse validated manifest snapshots

## Context

files:
- `tools/desk/internal/deskkit/activation.go`
- `tools/desk/internal/deskkit/activation_test.go`
- `tools/desk/internal/deskkit/activationindex.go` (planned)
- `tools/desk/internal/deskkit/activationindex_test.go` (planned)
- `tools/desk/internal/deskkit/rosterfixture_test.go`
- `tools/desk/internal/deskkit/activationstartup_test.go`
- `tools/desk/internal/deskkit/testdata/activation_mutations.py`
- `tools/desk/internal/arch/arch_test.go`
- `tools/desk/internal/arch/markers.txt`
- `tools/desk/cmd/deskboard/main.go`
- `tools/desk/cmd/deskdispatch/main.go`
- `tools/desk/cmd/desktoken/main.go`
- `tools/desk/README.md`
- `docs/contracts.md`
- `changelog/activation-discovery.md` (planned)

facts (source and date in frontmatter):
- At eba941721, CheckVerbActivation invokes DiscoverManifests, a full WalkDir skipping .git and nested checkouts. It runs before PerfRun in the three named entrypoints.
- Six-repetition installed e70f08e4 synthetic-cache measurements on 2026-10-16 observed median help of 11 ms outside a checkout versus 92–101 ms in two checkouts. Cached lookup medians were 23 ms versus 357–503 ms. These warm host observations include other work and do not isolate all delay to discovery.
- Existing activation semantics include malformed/duplicate manifests, required-provider resolution, deterministic ordering and nested-checkout exclusion. They remain the reference; ignoring an untracked manifest silently would change the contract.
- Statusgen's future context resolver requires independent membership enumeration and cannot accept a previously read file list as proof no consumer was added.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: existing activation owner
  contract: source-bound discovery snapshot
  retires: [unqualified repeated discovery on validated hits]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

Risk derivation: these four no answers are scoped to behavior-preserving discovery or
measurement work. No credential, private payload or new identity is exposed, and no policy,
role grant or refusal may change. Direct reference/exit parity is required. Any proposed
authority or disclosure change is outside this brief and needs a separately derived gate.

## Read first

Read the existing implementations listed in Context, the source-pinned performance priority
section of the desktools-v2 README, and docs/library-first.md. Refresh current claims and the
source revision before implementation; dated measurements do not certify current deployment.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure
contact. Use isolated temporary homes, fake providers and synthetic repositories. Preserve
existing custody, stop, trust, audit, budget and unavailable-state semantics. Optional
measurement cannot change the operation result. Stop at implemented; independent verification
owns Evidence. All new paths and named tests below are implementation deliverables.

## Interface contract

Keep DiscoverManifests as the reference full enumerator and one owner for activation meaning.
A reusable manifest snapshot carries source/root identity, complete membership evidence, content
and derivation revisions. Recompute activation against the current admitted configuration; a
cached discovery result is never a cached grant. Missing/corrupt/unsupported or unverifiable
snapshot inputs fall back to the existing reference behavior. A root mtime, HEAD SHA alone or
list of previously read files cannot validate dirty/untracked membership. No TTL may hide a new
required provider. If a safe persistent fast path cannot be qualified, report that result and
deliver bounded in-process reuse rather than weaken the gate or claim a startup win.

## Task

1. Instrument discovery via a test seam counting visited directories, manifests parsed and full enumerations. Benchmark small and large synthetic trees, cold and warm, through the three actual installed-style entrypoints with isolated state.
2. Build a bounded source-snapshot discovery index at the existing manifest owner. Separate membership/content validation from activation evaluation. Choose and document a validation mechanism that can detect nested untracked additions, removals, ignored manifests that the reference sees, dirty edits, source/root swaps and provider/config changes; preserve exact reference semantics.
3. Reuse snapshots only on proven-valid hits; atomic bounded storage, corruption recovery and concurrent readers must preserve the result. No new service, background watcher requirement, broad repository exclusion or second activation algorithm. Count remaining validation work instead of calling a directory scan a cache hit.
4. Integrate the named entrypoints through the shared owner and measure external elapsed startup including pre-PerfRun work. Runtime inactive/stop/custody refusals stay reachable with upper CLI helpers bypassed. Help/version remain owned by the current Cobra migration briefs; their zero-config/no-scan tests must count this pre-main-operation work too.
5. Document the source-bound snapshot contract, invalidation/fallback and observed cold/warm costs; report p50/p95, input sizes, parse/scan counts and telemetry-off/on overhead. Acceptance requires a stable fixture to avoid repeated full discovery and every changed fixture to match a fresh enumeration; no arbitrary universal milliseconds target.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotMatchesFreshDiscovery$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotMatchesFreshDiscovery ' "$out")` | exit 0; named PASS; warm unchanged fixture avoids a second full enumeration; nested untracked/ignored additions, removals, dirty edits and duplicates match the reference; HEAD-only validation fails |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotIsNotAuthority$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotIsNotAuthority ' "$out")` | exit 0; named PASS; changing config/provider requirements or root identity recomputes activation and inactive refuses through the direct lower API; cached boolean acceptance fails |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotCorruptionAndBounds$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotCorruptionAndBounds ' "$out")` | exit 0; named PASS; corrupt or oversized indexes, concurrent writers and nested checkout changes preserve documented fallback and bounded storage; accepting a corrupt index fails |
| 4 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCompiledStartupDiscoveryWorkload$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCompiledStartupDiscoveryWorkload ' "$out")` | exit 0; named PASS; compiled board/dispatch/token fixture runs retain behavior and report external cold/warm time plus scan counts; planted pre-timer full scan is visible |
| 5 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/60` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: warm unchanged fixture avoids a second full enumeration; nested untracked/ignored additions, removals, dirty edits and duplicates match the reference; HEAD-only validation fails.
- Row 2: changing config/provider requirements or root identity recomputes activation and inactive refuses through the direct lower API; cached boolean acceptance fails.
- Row 3: corrupt or oversized indexes, concurrent writers and nested checkout changes preserve documented fallback and bounded storage; accepting a corrupt index fails.
- Row 4: compiled board/dispatch/token fixture runs retain behavior and report external cold/warm time plus scan counts; planted pre-timer full scan is visible.

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

## Implementation choice within the declared fallback

The bounded fallback is process-local reuse of parsed discovery candidates, with one slot and
1 MiB source/payload limits. Cold discovery retains only the exact parsed observation,
with no second traversal; any failed read prevents candidate retention. Every reused
return, including the first attempted reuse, still requires complete membership and
content-validation traversal; the warm benefit is avoiding YAML parsing and derived
sorting/deduplication, not eliminating filesystem scans. Persistent startup reuse is
not qualified. The three compiled synthetic workloads measure external elapsed
inactive-provider refusals with fresh/reused isolated homes and telemetry off/on;
these do not qualify successful hotpath or recorder-write overhead (/62). Current
board/dispatch help is scan-free; token's pre-parser activation remains owned by /33.
See `tools/desk/README.md` for measured costs and limitations. Existing Verify rows
and independent Evidence ownership are unchanged.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

Verification-Attestation: medici-finance/assay#3925 run=a706b3ea71ce9147314396e7c52cf08d356dd10f084354c18e1b374c8da8f791 source=116a2f1494fca6c97209f10883861467f729216d brief=docs/streams/desktools-v2/brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md model=claude-opus-5-5 tier=strong

Subject: merged main 116a2f1494fca6c97209f10883861467f729216d (delivery merge 74f50d985cc1224c252a348c65d3639fe2a08aa2, PR #3811). Environment: rows 1-4 (check:ci) ran byte-for-byte in a local Linux container started with `docker run --network none` (only loopback and an inert tunl0 present; DNS lookup failed, so the network was down), image assay-verify:go1.27-py (image id sha256:9951b77a257f, linux/arm64, go1.27.2, git 2.47.3; local image with NO pinned repo digest, so this is not the qualified runner). The worktree and Go module cache were mounted read-only, with GOCACHE in container tmp, GOPROXY=off and GOTOOLCHAIN=local. Row 5 ran on the host: darwin/arm64, statusgen v1.0.54, KUBECONFIG=/dev/null. Original table re-derived with deskpathguard rederive: 5 rows, 0 author-added, all from-base.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotMatchesFreshDiscovery$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotMatchesFreshDiscovery ' "$out")` | exit 0; named PASS | exit 0 in the network-none container (assay-verify:go1.27-py, no pinned digest); --- PASS: TestActivationSnapshotMatchesFreshDiscovery (0.01s) | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotIsNotAuthority$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotIsNotAuthority ' "$out")` | exit 0; named PASS | exit 0 in the network-none container (assay-verify:go1.27-py, no pinned digest); --- PASS: TestActivationSnapshotIsNotAuthority (0.00s) | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotCorruptionAndBounds$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotCorruptionAndBounds ' "$out")` | exit 0; named PASS | exit 0 in the network-none container (assay-verify:go1.27-py, no pinned digest); --- PASS: TestActivationSnapshotCorruptionAndBounds (0.05s) | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCompiledStartupDiscoveryWorkload$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCompiledStartupDiscoveryWorkload ' "$out")` | exit 0; named PASS | exit 0 in the network-none container (assay-verify:go1.27-py, no pinned digest); --- PASS: TestCompiledStartupDiscoveryWorkload (18.25s) | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `statusgen --root . --consumers --brief desktools-v2/60` | exit 0; implementation diff corroborates declared routing | exit 2; could-not-check: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desktools-v2:60 is not in the diff against 116a2f149..., and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Supplemental diagnostic, no row credit: rows 1-4 were also run on the plain host (darwin, network not isolated). All four exited 0 with named PASS (0.05s, 0.02s, 0.09s, 26.81s). A host run does not qualify a check:ci row.

Witness could-not-check: `statusgen verifyrun --sandbox container-netns --dry-run --brief docs/streams/desktools-v2/brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md`, run inside the same network-none container with statusgen built from this source, exited 2. The worktree's linked git directory did not resolve inside the container ("fatal: not a git repository"), so no git identity was found ("could-not-attribute — no executing identity is available"), and no witness row was produced.

Supplemental diagnostic, no row credit: `statusgen --root . --consumers --brief desktools-v2/60 --delivery 74f50d985cc1224c252a348c65d3639fe2a08aa2 --base 8121208a6a4fb011ed1b0e7a00bd983a5aa4feeb` exit 0; "summary: 9 corroborated, 0 disproved, 2 unchecked" (the two out-of-scope entries, tools/desk/cmd/ and statusgen/, are reviewer judgement). This suggests row 5 would corroborate once its command names the delivery, but row 5 as admitted did not run to a result.

Risk-bearing values (enumeration over the delivery diff 8121208a6..74f50d985, activationindex.go and activation.go): `manifestLimit = 1 << 20` @ tools/desk/internal/deskkit/activationindex.go:18; `manifestRevision = "activation-discovery/1"` @ tools/desk/internal/deskkit/activationindex.go:17; one-slot bound (single `slot *manifestSnapshot` field in `activationIndex`) @ tools/desk/internal/deskkit/activationindex.go:125-128. All are reversible process-local knobs (the item is irreversible: no). The highest-ranked is the 1 MiB bound.

RISK-VALUE: DERIVED — manifestLimit = 1 << 20 @ tools/desk/internal/deskkit/activationindex.go:18 — the brief's declared fallback fixes "one slot and 1 MiB source/payload limits". The bound only decides whether a candidate is retained or reused: going over it sets source.valid=false or refuses the payload, which falls back to fresh DiscoverManifests. So no value of it can change an activation result, only whether reuse happens, and an edit plus a redeploy undoes it.

**VERIFY: BLOCKED** — 4/5 rows passed (rows 1-4 network-off in an unpinned local container, not the qualified runner); row 5 (statusgen --consumers as written) exited 2 COULD-NOT-CHECK because the delivery cannot be derived without a Verification-Table marker or an explicit --delivery. No obligation was observed to fail.

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
