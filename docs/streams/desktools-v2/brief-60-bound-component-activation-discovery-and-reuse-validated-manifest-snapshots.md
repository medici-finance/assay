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

### Superseded runs, 2026-10-21 (diagnostic history, no acceptance credit)

The two earlier verifier runs on this brief are kept below as history; later runs append and never overwrite. Neither earlier run is current acceptance. Two changes were made to their text, and nothing else: each run's closing verdict line was rewritten in plain words so that only the newest run carries a bold verdict, and the second run's verdict line no longer counts rows 1-4 as passed acceptance, because no admitted execution witness existed for them (they were direct test observations, with acceptance unproven). The first run's row 5 witness entry records exit 0, but its output read 0 corroborated, 0 disproved and 11 unchecked, so it is non-accepting. The 2026-10-25 row 5 witness below supersedes it. The original wording of both runs is in this pull request's commit history.

#### First run, 2026-10-21, at merged source 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c

Verification-Table: delivery=74f50d985cc1224c252a348c65d3639fe2a08aa2 base=08d807836774857e8b735e1e4ef0563f6d553d01 original=08d807836774857e8b735e1e4ef0563f6d553d01

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotMatchesFreshDiscovery$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotMatchesFreshDiscovery ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotIsNotAuthority$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotIsNotAuthority ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotCorruptionAndBounds$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotCorruptionAndBounds ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCompiledStartupDiscoveryWorkload$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCompiledStartupDiscoveryWorkload ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `statusgen --root . --consumers --brief desktools-v2/60` | pass exit=0 | sha256:ddedb228a8f2 | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3845 run=74b38ec930152fc8e963d53ee4fc341554f35573863d93fe4b889d77e7e49d4c source=3cea6aa621ec0364fb7a6cbd1196141829ba6c4c brief=docs/streams/desktools-v2/brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md model=claude-opus-5-5 tier=strong

Independent verifier pass, 2026-10-21, at merged source 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (darwin/arm64, go1.27.2, statusgen v1.0.54). Admission (deskdispatch --check-verifier) returned exit 0 before any row ran. The delivery is merge 74f50d985cc1224c252a348c65d3639fe2a08aa2 (medici-finance/assay#3811) on the first-parent chain. Its base is the unique merge-base 08d807836774857e8b735e1e4ef0563f6d553d01, and the parsed Verify table there is byte-identical to the one at the source and at main's first parent. deskpathguard rederive reported 5 rows from base and 0 added by the author.

| # | Result at the mandatory witness | Supplemental diagnostic (no acceptance credit) | Date | Runner |
|---|---|---|---|---|
| 1 | could-not-run: the check:ci row needs the unshare network-off sandbox, which is Linux-only, and this host is darwin | host run of the exact command: exit 0, named PASS line for TestActivationSnapshotMatchesFreshDiscovery (0.04s); mutants "membership shortcut" (HEAD-only equivalent) and "content bytes not hashed" both killed | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | could-not-run: the check:ci row needs the unshare network-off sandbox, which is Linux-only, and this host is darwin | host run of the exact command: exit 0, named PASS line for TestActivationSnapshotIsNotAuthority (0.01s); mutant "cached acceptance" (ComputeActivation with nil ext and trust forced true) killed | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | could-not-run: the check:ci row needs the unshare network-off sandbox, which is Linux-only, and this host is darwin | host run of the exact command: exit 0, named PASS line for TestActivationSnapshotCorruptionAndBounds (0.04s); mutant "corrupt index accepted" (checksum compare forced true) killed | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | could-not-run: the check:ci row needs the unshare network-off sandbox, which is Linux-only, and this host is darwin | host run of the exact command: exit 0, named PASS line for TestCompiledStartupDiscoveryWorkload (12.36s); mutant "pre-parser startup scan" (planted in deskboard main) killed | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | pass exit=0, but the captured output read 0 corroborated, 0 disproved, 11 unchecked: the Evidence-only working-tree edit made the brief selectable and supplied no implementation diff. Before that edit the same command exited 2 with COULD-NOT-CHECK (delivery not derivable) | with the explicit delivery 74f50d985cc1 and its first parent 8121208a6a4f: exit 0, 9 corroborated, 0 disproved, 2 unchecked (tools/desk/cmd/ and statusgen/ out-of-scope, both review judgments) | 2026-10-21 | assay-verifier-app[bot] @ 3cea6aa621ec0364fb7a6cbd1196141829ba6c4c (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

The mutation diagnostic is the brief's own production-owner mutation script (testdata activation_mutations.py). I ran it on an extracted copy of the source outside the admitted home: exit 0. All 10 discriminators passed their baseline on unmodified source, and all 12 planted mutants were killed. Log sha256: b7648b935d2429231430cfb4c72c52736805569a1559396745e0a46415c1b6c2. These diagnostics ran on the host, not inside the required network-off sandbox. They discharge no original row.

Out-of-scope review for row 5: the delivery diff changes no path under tools/desk/cmd/ or statusgen/. The only refusal call site that changed is VerbActivationRefusal, which now routes through the shared owner. The independent full membership and content walk still runs on every reused return.

Risk-value enumeration covers the delivery diff in activationindex.go and activation.go: two runtime literals were introduced. Both are reversible, process-local values with no persistent state.

RISK-VALUE: DERIVED — manifestLimit = 1 << 20 @ tools/desk/internal/deskkit/activationindex.go:18 — this is the brief's declared bounded fallback ("one slot and 1 MiB source/payload limits"). Exceeding it marks the source invalid, refuses validation, or skips retention, and every one of those paths returns the reference DiscoverManifests result. Correctness therefore never depends on the value; it caps memory at one 1 MiB slot.
RISK-VALUE: DERIVED — manifestRevision = "activation-discovery/1" @ tools/desk/internal/deskkit/activationindex.go:17 — this is the derivation-revision tag that the snapshot contract requires. It is hashed into the source digest and checked on decode, so a mismatch falls back to fresh discovery. The slot is in-process only, so no stale cross-binary snapshot can carry over.

Lint notices observed at this source: (a) the changed deskkit paths match the repo's security-path trigger, while the brief answers all four risk questions "no". The literal enumeration above found no credential, grant or refusal change: activation is still recomputed from the current configuration on every call, and the "cached acceptance" mutant was killed. (b) Row 5's command pins no --delivery or --base, so after merge it cannot corroborate on its own. That is a check-definition gap, routed for a reviewed amendment. It is not an implementation failure.

Verdict recorded by this run, in plain words: blocked — rows 1-4 (check:ci) are could-not-run, because the mandatory unshare network-off witness is unavailable on this darwin host. Row 5's routing obligation is met only through the supplemental explicit-delivery probe and the review above, not by the command as written. No acceptance obligation was observed to fail. Next actor: a Linux runner (or an approved container-netns runner) re-runs statusgen verifyrun under this admission.

#### Second run, 2026-10-21, at merged source 116a2f1494fca6c97209f10883861467f729216d

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

Verdict recorded by this run, in plain words: blocked — rows 1-4 had successful direct test observations network-off in an unpinned local container (not the qualified runner), with acceptance unproven because the execution witness refused and produced no rows; row 5 (statusgen --consumers as written) exited 2 COULD-NOT-CHECK because the delivery cannot be derived without a Verification-Table marker or an explicit --delivery. No obligation was observed to fail.

### Third run, 2026-10-25, at merged source 61eb62a4a61bce0f2e0c69b98f1906de80400316

Independent verifier pass at merged main 61eb62a4a61bce0f2e0c69b98f1906de80400316. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran, with the attestation named on the witness below. The delivery is still merge 74f50d985cc1224c252a348c65d3639fe2a08aa2 (#3811). The Verify section and the activation sources are unchanged since the second run's source, except for a three-line change to the startup test file. Host: darwin/arm64, statusgen v1.0.59, git 2.56.0, KUBECONFIG=/dev/null.

Rows 1-4 are check:ci rows. This host has no unshare network sandbox, so they went to the container admission bracket. The host open step succeeded and recorded tree sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9. The container step ran in a --network none container with the engine's default seccomp: image golang@sha256:55395706e9703db746cc507abfc4eb2aea75918f8a8024e4848e2cb81004f5ad (linux/amd64, go1.27.2, git 2.39.5), with the released statusgen v1.0.59 linux/amd64 binary, its sha256 checked against the release checksums. Only loopback and an inert tunl0 interface were present. The container step refused before any row ran, exit 2:

"statusgen verifyrun: container admission refused; no Verify rows executed: the mounted root [the admitted home] digests to sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, not the sha256:e24b2cf936ec7be8af97daf9fb0cc0cc29fc93e9f3d69d4d4954ed564d950de9 the host recorded at the open step. The container is not running the admitted home"

The home is a linked worktree, and git inside the container reported "fatal: not a git repository" for it. The tree digest came out empty (the sha256 of no input), so admission refused. This is the documented limit for linked worktrees in container mode. The refusal was not retried in another form, and no close step ran, because no witness existed to close. Rows 1-4 therefore have no admitted witness in this run.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotMatchesFreshDiscovery$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotMatchesFreshDiscovery ' "$out")` | exit 0; named PASS | could-not-run: the container admission step refused before any row executed (the mounted linked worktree digested empty), so no network-off witness exists | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotIsNotAuthority$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotIsNotAuthority ' "$out")` | exit 0; named PASS | could-not-run: the container admission step refused before any row executed (the mounted linked worktree digested empty), so no network-off witness exists | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestActivationSnapshotCorruptionAndBounds$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestActivationSnapshotCorruptionAndBounds ' "$out")` | exit 0; named PASS | could-not-run: the container admission step refused before any row executed (the mounted linked worktree digested empty), so no network-off witness exists | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCompiledStartupDiscoveryWorkload$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestCompiledStartupDiscoveryWorkload ' "$out")` | exit 0; named PASS | could-not-run: the container admission step refused before any row executed (the mounted linked worktree digested empty), so no network-off witness exists | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row 5 is a host row (check, not check:ci). Its witness is below, written by statusgen verifyrun --rows 5 on the unedited home.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `statusgen --root . --consumers --brief desktools-v2/60` | fail exit=2 | sha256:a22a2dbd0e7f | 2026-10-25 | assay-verifier-app[bot] @ 61eb62a4a61b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4377 run=91a2a72c0bf68f61bb7eb987dcf4772fd15f86fbf4057c4d9d767ee92d668e5b source=61eb62a4a61bce0f2e0c69b98f1906de80400316 brief=docs/streams/desktools-v2/brief-60-bound-component-activation-discovery-and-reuse-validated-manifest-snapshots.md model=claude-opus-5-5 tier=strong

Row 5 output (its sha256 prefix a22a2dbd0e7f matches the witness): "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desktools-v2:60 is not in the diff against 61eb62a4a61bce0f2e0c69b98f1906de80400316, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." The exit 2 is the instrument reporting that it could not check, not a disproved routing claim. Row 5 as written stays could-not-check until its command names the delivery, which is the check-definition class already tracked in #1927.

Supplemental diagnostics, no acceptance credit for any row. They ran in a separate detached checkout of the same source, outside the admitted home.
- Rows 1-4: each exact command on the plain darwin host, without network isolation, exited 0 with its named PASS line (0.31s, 0.10s, 0.16s and 46.67s). These are successful test observations with acceptance unproven. They are not witnesses and do not discharge a check:ci row.
- Row 5: the explicit-delivery form, statusgen --root . --consumers --brief desktools-v2/60 --delivery 74f50d985cc1224c252a348c65d3639fe2a08aa2 --base (its first parent), exited 0 with "summary: 9 corroborated, 0 disproved, 2 unchecked". The two unchecked entries (tools/desk/cmd/ and statusgen/, both out of scope) are review judgments. This different invocation gives row 5 no credit.

Risk-bearing values. The enumeration covers the delivery diff in activationindex.go and activation.go and the brief's Deliverables. It found these literals, and none of them changed since the earlier runs:
- manifestRevision = "activation-discovery/1" @ tools/desk/internal/deskkit/activationindex.go:17
- manifestLimit = 1 << 20 @ tools/desk/internal/deskkit/activationindex.go:18
- the one-slot bound, slot *manifestSnapshot (a single field) @ tools/desk/internal/deskkit/activationindex.go:127

All three are reversible process-local knobs; the item is marked irreversible: no. Ranked by what breaks if wrong, manifestLimit comes first. Each value only decides whether a parsed snapshot is retained or reused. Every refusal path returns the reference DiscoverManifests result, and activation is recomputed from current configuration on every call.

RISK-VALUE: DERIVED — manifestLimit = 1 << 20 @ tools/desk/internal/deskkit/activationindex.go:18 — the brief's declared fallback fixes "one slot and 1 MiB source/payload limits". Going over the bound marks the source invalid, refuses the payload or skips retention, and each of those falls back to fresh discovery. So the value bounds memory to one 1 MiB slot and cannot change an activation result.
RISK-VALUE: DERIVED — manifestRevision = "activation-discovery/1" @ tools/desk/internal/deskkit/activationindex.go:17 — this is the derivation-revision tag the snapshot contract requires. It is hashed into the source digest and checked on decode, so a mismatch falls back to fresh discovery. The slot is in-process only, so no snapshot can carry over across binaries.

**VERIFY: BLOCKED** — 0 of 5 rows have admitted acceptance evidence. Rows 1-4 (check:ci) are could-not-run because the container admission step refused (linked-worktree tree digest empty), and no host unshare sandbox exists. Row 5 as written is could-not-check (exit 2, delivery not derivable). No acceptance obligation was observed to fail. Next actor: a verifier run whose home is not a linked worktree (or whose git directory resolves inside the container), on a network-off runner, re-runs rows 1-4 under fresh admission; row 5 needs its reviewed check-definition repair (#1927).

Routing note, 2026-10-25 (appended after review; the verdict line above is unchanged). This branch restores the first run's `Verification-Table:` marker above. Once it merges, row 5 has a table to read: in a simulated merge with current main, row 5 as written exits 0 with 9 corroborated, 0 disproved and 2 unchecked. So row 5's next action is an as-written re-run on merged main. The check-definition repair (#1927) named in the third run's next-actor sentence does not gate row 5. Two corrections to the third run's prose: (1) the delivery diff also changed `tools/desk/internal/deskkit/rosterfixture_test.go` (+2/-1), alongside `activationstartup_test.go`; (2) the same empty-tree digest (sha256:e3b0c442…) also appears when the container bind mount does not expose the home. The next container run should confirm the home's files are visible inside the container before it opens the bracket.

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
