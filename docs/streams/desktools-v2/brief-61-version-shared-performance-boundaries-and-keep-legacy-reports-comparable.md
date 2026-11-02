---
brief: assay:assay:desktools-v2:61
title: "Version shared performance boundaries and keep legacy reports comparable"
why: "Current desk wall time excludes entrypoint setup while CPU includes process lifetime, and readers cannot distinguish several command modes. A shared versioned measurement boundary makes the local records usable for comparing optimizations across desk tools and statusgen without treating missing measurements as zero."
wave: 1
depends: []
unblocks: ["desktools-v2/63", "desktools-v2/62", "statusgen/26", "performance-collection/02"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 2
id: c01a7fef-514d-47ac-b39e-ede3f4e9d63a
authored: 2026-10-16 by performance brief authoring
sources:
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
consumers:
  - "perfrec/: follow-up desktools-v2/61"
  - "tools/desk/internal/deskkit/perf.go and perfread.go: follow-up desktools-v2/61"
  - "tools/desk/cmd/deskperf/: follow-up desktools-v2/61"
  - "tools/desk/cmd/cellctl/perf.go and existing boot/launch step readers: follow-up desktools-v2/61 (compatibility, no duplicate invocation totals)"
  - "tools/desk/cmd/deskboard, deskdispatch and desktoken: follow-up desktools-v2/62"
  - "statusgen/perfrecord.go and perfrecord_test.go: follow-up statusgen/26"
  - "qualgen and standalone tools: out-of-scope (document the credential-free perfrec API and owner adoption checklist; no speculative wrappers)"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "docs/telemetry.md and tools/desk/README.md: follow-up desktools-v2/61"
---

# Brief 61 — Version shared performance boundaries and keep legacy reports comparable

## Context

files:
- `perfrec/perfrec.go`
- `perfrec/perfrec_test.go`
- `perfrec/cpu_unix.go`
- `perfrec/cpu_windows.go`
- `perfrec/deps_test.go`
- `perfrec/testdata/` (planned)
- `tools/desk/internal/deskkit/perf.go`
- `tools/desk/internal/deskkit/perfread.go`
- `tools/desk/internal/deskkit/perfread_test.go`
- `tools/desk/cmd/deskperf/`
- `docs/telemetry.md`
- `tools/desk/README.md`
- `changelog/performance-boundaries.md` (planned)

facts (source and date in frontmatter):
- Fresh main eba941721 already has the stdlib-only perfrec module. deskkit aliases its record/writer, and statusgen/perfrecord.go records through it from exitProcess. This brief extends that owner; it does not re-extract logging.
- Desk PerfRun starts wall timing inside run's wrapper after config echo and activation; CPUMillis currently returns process-lifetime RUSAGE_SELF. A compiled --help benchmark recorded wallMs=0 despite 92–101 ms external elapsed startup in checkouts.
- perfrec.Schema explicitly requires a version bump when fields change. The current reader filters perf-v1 and step records by a verb prefix; preserve existing files and legacy step views.
- performance-collection/01 owns the separate optional correlated event/spool contract, not local perf-v1 semantics. This work needs no perfdata, database, network or runtime activation.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: perfrec and existing domain producers
  contract: versioned local performance record
  retires: [incompatible timing boundaries and unclassified measured paths]
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

Extend perfrec with a documented invocation/session measurement API and a deliberately new
local schema for changed boundaries or fields. Capture wall and CPU baselines at the same
declared Go entrypoint boundary, before configuration/activation, and finish before best-effort
append. State excluded runtime startup and writer/flush costs; benchmark executable elapsed time
separately. Typed fixed-roster phases/counters have explicit availability; no argv/path/repo,
credential, raw error or new cross-run identifier. Reader APIs retain legacy perf-v1 semantics,
surface schema/coverage differences and never combine inclusive child/phase times into additive
elapsed time. perfrec remains credential-, network-, subprocess- and database-free.

## Task

1. Specify the new local schema and timing/counter meanings in perfrec and docs/telemetry.md. Keep old fixtures readable; do not reinterpret historical wallMs or CPU fields. Distinguish unsupported counter coverage from measured zero, failed/missing CPU reads and timed phases that overlap.
2. Deliver a narrow shared begin/phase/finish API with injected clocks and CPU samples for deterministic tests. Fixed tool-owned command/phase rosters determine labels after successful parsing; unknown input maps to constants, never substring matches over arbitrary argv. Bound per-invocation phases/counters and disabled-path overhead.
3. Adapt deskkit recorder and deskperf readers to both versions, with explicit legacy/new groups or clearly stated incompatible coverage. Keep boot and cellctl launch step records out of invocation totals; do not count a parent's inclusive elapsed time plus its children as recoverable minutes.
4. Provide low-cardinality counters for subprocess, direct HTTP, cache lookup/hit/miss/refresh and coalesced waiter outcomes with measured/unavailable coverage. Measurement adapters observe existing effects and cannot initiate credential or provider work. Provider hooks and command phase placement land in 62; statusgen uses this same API in statusgen/26.
5. Prove off creates nothing and cannot change stdout, stderr or exit code; failed recording is best effort. Keep the dependency-closure privacy tests, restrictive local storage and seven-day retention. Document module build/consumer resolution with GOWORK=off and a synthetic external consumer; no source reimplementation in another module.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | exit 0; named PASS; injected setup delay and CPU deltas share one boundary; absent CPU is unavailable and a planted late start fails |
| 2 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | exit 0; named PASS; old and new fixtures retain meanings, hostile labels never serialize, disabled mode writes nothing and sink failure preserves result; silent schema reuse fails |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | exit 0; named PASS; mixed perf-v1/new records and boot/launch steps report coverage and distributions without double-counting inclusive durations; naive summed totals fail |
| 4 | check:ci +flow +mutation | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | exit 0; named PASS; synthetic external GOWORK=off consumer uses the actual library; network, exec, credential and desk imports are planted negative controls |
| 5 | check +dereference | `o=$(statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact `summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` — the command itself exits non-zero on any other summary or any statusgen exit but 0. Every entry here is routed follow-up or out-of-scope, so statusgen settles each follow-up from the stream tables (the target brief exists and refers back), not from the diff: the delivery range (`2013b8dc` against its first parent `613cce8e`) is pinned because the `consumer-delivery-range` lint requires it, and the counts do not depend on it. Only 3 of the 8 CORROBORATED entries are back-reference corroboration: those routed to desktools-v2/62, statusgen/26 and performance-collection/02. The other 5 are routed `follow-up desktools-v2/61`, this brief itself, so the back-reference check reads this brief's own file and passes by self-reference: they carry no corroboration, and this row gives no evidence about those delivered paths; whatever evidence they have comes from rows 1–4 and review. Record the result that way — 3 back-reference corroborations, 5 self-routed passes — never as diff corroboration and never as 8 corroborated routing claims. The 8 / 0 / 1 pin guards only against a change in the three summary counts: a change in the per-entry lines that keeps those counts (a different entry becoming the UNCHECKED one, an entry's text changing) still passes, so read the printed entries, not just the summary. Report that summary line as printed, all three counts; exit 0 alone does not show corroboration, it only says nothing was disproved. Out-of-scope sufficiency remains review: the one UNCHECKED entry (the out-of-scope qualgen and standalone tools entry) is not a pass, and the reviewer weighs its stated reason. No unavailable check is recorded as a pass: missing history or an unreadable delivery makes statusgen exit 2 (could-not-check), which this command turns into a non-zero exit, never a pass. |

## Pre-mortem and detection

- Row 1: injected setup delay and CPU deltas share one boundary; absent CPU is unavailable and a planted late start fails.
- Row 2: old and new fixtures retain meanings, hostile labels never serialize, disabled mode writes nothing and sink failure preserves result; silent schema reuse fails.
- Row 3: mixed perf-v1/new records and boot/launch steps report coverage and distributions without double-counting inclusive durations; naive summed totals fail.
- Row 4: synthetic external GOWORK=off consumer uses the actual library; network, exec, credential and desk imports are planted negative controls.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

Subject revision: merged main d8a4d0a941a1c312ebfbce16ecda782a5111eadb; delivering change 2013b8dc6128 (squash of #3431, parent 613cce8e72cb).
Environment: darwin/arm64, go1.27.2, git 2.56.0, statusgen v1.0.49; detached verifier worktree, offline, KUBECONFIG=/dev/null.
Verification-Attestation: medici-finance/assay#3437 run=cc223cdc405e65397a31e82411801221b181777f309bf65eaae99279ebccc2a1 source=d8a4d0a941a1c312ebfbce16ecda782a5111eadb brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | Verify row 1 as written (perfrec, GOWORK=off go test -count=1 -v -run ^TestInvocationBoundaryAndCPUParity$, grep named PASS) | exit 0; named PASS | exit 0; --- PASS: TestInvocationBoundaryAndCPUParity (0.03s). Test body holds the planted late-start and planted process-lifetime CPU controls (both must fail the parity oracle) and the unsupported/failed/backwards/missing CPU cases (no cpuMs, never zero). | 2026-10-18 claude-opus-5-5 (dispatched verifier) |
| 2 | Verify row 2 as written (perfrec, -run ^TestPerformanceSchemaAndPrivacyCompatibility$) | exit 0; named PASS | exit 0; --- PASS: TestPerformanceSchemaAndPrivacyCompatibility (0.01s). Test body covers perf-v1/perf-v2 fixture byte round-trip, schema-reuse refusal in both directions, hostile labels mapped to constants, switch-off writes nothing, sink failure leaves exit/result unchanged, 0700/0600 modes. | 2026-10-18 claude-opus-5-5 (dispatched verifier) |
| 3 | Verify row 3 as written (tools/desk, -run ^TestMixedSchemaReportBoundaries$ on the deskperf command package) | exit 0; named PASS | exit 0; --- PASS: TestMixedSchemaReportBoundaries (0.00s). Test body asserts the legacy total 2100, perf-v2 sum 3600 and boot 700 each differ from every naive summed total (all walls, v2 plus inclusive child/phase time, v1+v2 folded). | 2026-10-18 claude-opus-5-5 (dispatched verifier) |
| 4 | Verify row 4 as written (perfrec, -run ^TestPublicRecorderConsumerBoundary$) | exit 0; named PASS | exit 0; --- PASS: TestPublicRecorderConsumerBoundary (0.87s); subtests planted_network, planted_exec, planted_credential, planted_desk ran. | 2026-10-18 claude-opus-5-5 (dispatched verifier) |
| 5 | statusgen --root . --consumers --brief desktools-v2/61 | exit 0; implementation diff corroborates routing; unchecked never a pass | FAIL as written: exit 2, COULD-NOT-CHECK. Output: the brief is not in the diff against d8a4d0a941a1, so this run carries no evidence about its claims; nothing corroborated, nothing disproved. Expect (exit 0) is not met. The row has no pinned delivery range (check-definition class), so on merged main it has no diff to read. | 2026-10-18 claude-opus-5-5 (dispatched verifier) |

Supplementary (outside the table): GOWORK=off go test -count=1 over the whole perfrec module: ok. The deskperf and deskkit packages under a Perf/Mixed/Report/Boot selector: ok.

Supplementary diagnostic for row 5, which is NOT the row result: two ad-hoc runs, both exit 0, both with summary 8 corroborated, 0 disproved, 1 unchecked. The unchecked entry is the qualgen/standalone-tools out-of-scope claim, which stays a review judgment.
- statusgen --root . --consumers --brief desktools-v2/61 --base aa42282c9e36. This window is 52 commits, wider than the delivery.
- The pinned delivery form statusgen --root . --consumers --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899, which evaluates the immutable delivered snapshot.

Pin the author lane needs for row 5: delivery 2013b8dc612842467d08330e298b5401ce855e5f with first parent 613cce8e72cb95ec21d3f63120be68585fc91899, in the form --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899.

Risk-bearing values: the trigger FIRES. Risk metadata is present and all no, and irreversible is no. But the declared path tools/desk/internal/deskkit/perf.go matches this repo's security-path trigger tools/desk/internal/deskkit/ (statusgen --lint NOTICE risk-files-crossread), so the diff touches a risk-classed path. The enumeration below was run over the delivering diff's non-test perfrec, deskkit and deskperf sources:
- SchemaV2 = "perf-v2" @ perfrec/schema.go:26. Rank 1: reusing the tag would make readers apply perf-v1 meanings to perf-v2 fields. Reversible by an edit, but records already written keep whatever tag they got.
- labelPattern = ^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$ and the 64-byte cap @ perfrec/schema.go:100 and :105. Rank 2: the privacy bound on serialized labels.
- MaxPhases = 16 @ perfrec/schema.go:94. Reversible operational bound, ranked last.
- RetentionDays = 7 @ perfrec/perfrec.go:50 and the 0o700/0o600 modes. Unchanged by this diff; pre-existing.

RISK-VALUE: DERIVED — SchemaV2 = "perf-v2" @ perfrec/schema.go:26 — perfrec.Schema says any field change needs a version bump. perf-v2 adds boundary, cpu, phases, phaseOverlap, counters and unavailable and drops childProcs, childWallMs and forgeCalls, so a distinct tag is required. The reader then routes by exact tag, and the test refuses each cross-schema mix.
RISK-VALUE: DERIVED — labelPattern @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105 — the character class cannot match slash, dot, underscore, equals, space or uppercase. Paths, owner/repo slugs, token prefixes such as ghp_, flag=value argv and free text therefore all fail the pattern and map to "other". The test drives each of those hostile shapes through tool, verb, role, phase and counter. The 64-byte cap only tightens the pattern's 83-byte maximum.

**VERIFY: BLOCKED** — class check-definition, rows [5]. Rows 1–4 PASS. Row 5 as written is could-not-check (exit 2) on merged main, which is not an observed implementation failure. No status advance; the author lane needs to pin row 5's delivery range as named above.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 5 | `o=$(statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:71c2c0ad9b2d | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3454 run=a2f5d22fbcc6066f158ddbcc003f601b51aa8d44ac3b47eda0d44a6812b77564 source=e4d474af8bca036b11291259cf6130d98abe5feb brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong

Re-verify, 2026-10-19. Subject revision: merged main e4d474af8bca036b11291259cf6130d98abe5feb (row 5 re-authored by medici-finance/assay#3449, commit daa487841); delivering change 2013b8dc6128 with first parent 613cce8e72cb.
Environment: darwin/arm64, go1.27.2, git 2.56.0, statusgen v1.0.50; detached verifier worktree, offline, KUBECONFIG=/dev/null, GOFLAGS=-count=1.
Verification-Attestation: medici-finance/assay#3454 run=a2f5d22fbcc6066f158ddbcc003f601b51aa8d44ac3b47eda0d44a6812b77564 source=e4d474af8bca036b11291259cf6130d98abe5feb brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong

The witness table above is the execution witness: rows 1–4 could-not-run there because this darwin host has no unshare --net; row 5 passed. The rows below for 1–4 are by-hand runs of each command exactly as written, under a macOS sandbox-exec profile that denies all network except local unix sockets (an outbound TCP connect to 1.1.1.1:443 failed inside it), with GOPROXY=off. They are by-hand evidence, distinct from the witness.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | Verify row 1 as written, under the deny-network sandbox-exec profile | exit 0; named PASS | exit 0; --- PASS: TestInvocationBoundaryAndCPUParity (0.05s). The test body holds the planted late start and planted process-lifetime CPU controls, which must fail the boundary oracle. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 as written, under the deny-network sandbox-exec profile | exit 0; named PASS | exit 0; --- PASS: TestPerformanceSchemaAndPrivacyCompatibility (0.02s). The test body refuses schema reuse in every direction and asserts SchemaV2 differs from Schema. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 as written, under the deny-network sandbox-exec profile | exit 0; named PASS | exit 0; --- PASS: TestMixedSchemaReportBoundaries (0.00s). The test body checks that every naive summed total (all walls, v2 plus inclusive, v1 plus v2) differs from each reported total. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 as written, under the deny-network sandbox-exec profile | exit 0; named PASS | exit 0; --- PASS: TestPublicRecorderConsumerBoundary (0.59s). Planted network, exec, credential and desk imports are the negative controls. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 as written (statusgen consumers, pinned brief, delivery 2013b8dc against base 613cce8e) | exit 0; exact summary 8 / 0 / 1 / 0 | exit 0; printed "summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing" over the immutable delivered snapshot. Per the row's own Expect: 3 are back-reference corroborations (follow-ups routed to desktools-v2/62, statusgen/26 and performance-collection/02; each target brief exists and lists desktools-v2/61 in its depends). The 5 entries routed to desktools-v2/61 itself (perfrec, deskkit perf and perfread, deskperf, cellctl perf and step readers, telemetry and desk README docs) are self-routed passes and corroborate nothing. The 1 UNCHECKED entry is the out-of-scope qualgen and standalone tools entry; it is a review judgment, not a pass. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Row 5 note: the row is still self-referential for 5 of its 8 corroborated entries. Its Expect says so and asserts only the three summary counts, so the row passes on its own terms but gives no evidence about the delivered paths. That evidence comes from rows 1–4 and review. The check-definition defect from the previous record (no pinned delivery range, exit 2 on merged main, tracked under medici-finance/assay#1927) no longer applies: the pinned row runs and exits 0.

Risk-bearing values: the trigger fires because the declared path tools/desk/internal/deskkit/perf.go matches the security-path trigger (statusgen lint NOTICE risk-files-crossread). The enumeration covered the delivering diff's non-test Go sources (perfrec, deskkit perf and perfread, deskperf main and report):
- SchemaV2 = "perf-v2" @ perfrec/schema.go:26. Rank 1: if the tag were reused, readers would apply perf-v1 meanings to perf-v2 fields, and records already written keep their tag.
- labelPattern = ^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$ @ perfrec/schema.go:100, with the cap len(s) <= 64 @ perfrec/schema.go:105. Rank 2: the privacy shape gate on serialized labels.
- MaxPhases = 16 @ perfrec/schema.go:94; the nearest-rank percentiles 50 and 90 and the default of 10 slowest in deskperf report. Reversible operational bounds, ranked last.
- RetentionDays = 7 @ perfrec/perfrec.go:50; modes 0o700 @ perfrec/perfrec.go:340 and 0o600 @ perfrec/perfrec.go:355. The delivering diff did not change these; they were already there.

RISK-VALUE: DERIVED — SchemaV2 = "perf-v2" @ perfrec/schema.go:26 — Schema = "perf-v1" @ perfrec/perfrec.go:44 is documented as frozen, so any changed field needs a new tag. perf-v2 adds boundary, cpu, phases, phaseOverlap, counters and unavailable and drops the legacy child and forge counters, so a distinct tag is required.
RISK-VALUE: DERIVED — labelPattern @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105 — the character class cannot match slash, dot, underscore, equals, space or uppercase, so paths, owner/repo slugs, underscored token prefixes, flag=value argv and spaced free text map to "other". The 64-byte cap only tightens the pattern's 83-byte maximum. Limit: the shape does admit a lowercase alphanumeric string of up to 40 characters that starts with a letter (for example a hex digest starting a–f). So the source comment's claim that "a token ... cannot pass it" holds only for tokens with an underscore or uppercase letters. The real gate is roster membership at the callers (deskkit's compiled rosters; perfrec Begin keeps only roster entries that pass Label).

**VERIFY: PASS** — 5/5 rows. Rows 1–4 are by-hand under a deny-network sandbox (the witness could-not-run them on darwin) and row 5 passes in the witness. Row 5 gives 3 back-reference corroborations and 5 self-routed passes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:c198ed529693 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | pass exit=0 sandbox=container-netns | sha256:317e34e516cb | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | pass exit=0 sandbox=container-netns | sha256:346457f3f5a2 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:5743fbce55a3 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 5 | `o=$(statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 sandbox=container-netns | sha256:9845e86678a2 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |

Re-wake verify, 2026-10-19. Subject revision: merged main 0e661b6e15de107efcf05ed7d8d0d21983eb3afa; delivering change 2013b8dc6128 with first parent 613cce8e72cb. The witness table above is the execution witness, a mechanical re-run of every Verify row exactly as written by statusgen verifyrun with sandbox=container-netns. Runner class: local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), started with --network none, the engine's default seccomp profile, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-count=1, KUBECONFIG=/dev/null; statusgen and the desk tools were built inside the container from the tree at the subject revision. This record follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun exited 0 and its check reported 5 pass, 0 fail, 0 could-not-run of 5 Verify rows.
Verification-Attestation: medici-finance/assay#3520 run=997aa4e942d1e62cf8c7ac41d30a0759d628785c4fd9374750bbba7753e39d01 source=0e661b6e15de107efcf05ed7d8d0d21983eb3afa brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong

The prior Evidence verdict was VERIFY: PASS on 2026-10-19 at e4d474af8bca. Since then the perfrec module, the deskkit perf and perfread sources and the deskperf command are unchanged; what changed is this brief's frontmatter (desktools-v2/63 added to unblocks, version 1 to 2), additive perf-v3 planning text in the telemetry doc and the desk README, and statusgen changes, which row 5 exercises with the tree-built statusgen.

No row failed in the witness. Row 5 was also run by hand on the darwin host with the pinned statusgen v1.0.51: exit 0, printed summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing. Per the row's own Expect, that is 3 back-reference corroborations (follow-ups routed to desktools-v2/62, statusgen/26 and performance-collection/02) and 5 self-routed passes, which corroborate nothing about the delivered paths; the 1 UNCHECKED entry is the out-of-scope qualgen and standalone tools entry, a review judgment and not a pass.

Risk-bearing values: the trigger fires because the declared path tools/desk/internal/deskkit/perf.go matches the security-path trigger. No code under perfrec, deskkit perf or perfread, or deskperf changed between the delivery 2013b8dc6128 and the subject revision, and every literal named in the prior record still sits at its line at the subject revision: SchemaV2 = "perf-v2" @ perfrec/schema.go:26; MaxPhases = 16 @ perfrec/schema.go:94 (reversible, ranked last); labelPattern @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105; Schema = "perf-v1" @ perfrec/perfrec.go:44; RetentionDays = 7 @ perfrec/perfrec.go:50; modes 0o700 @ perfrec/perfrec.go:340 and 0o600 @ perfrec/perfrec.go:355 (pre-existing, not changed by the delivery).
RISK-VALUE: DERIVED — SchemaV2 = "perf-v2" @ perfrec/schema.go:26 — Schema = "perf-v1" @ perfrec/perfrec.go:44 is frozen, and perf-v2 adds and drops fields, so a distinct tag is required; readers route by exact tag.
RISK-VALUE: DERIVED — labelPattern = ^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$ @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105 — the character class cannot match slash, dot, underscore, equals, space or uppercase, so paths, owner/repo slugs, underscored token prefixes, flag=value argv and spaced free text map to "other"; the shape still admits a lowercase alphanumeric string of up to 40 characters, so the real gate stays roster membership at the callers, as the prior record noted.

**VERIFY: PASS** — 5/5 rows pass with 0 fail in the network-off container witness at 0e661b6e15de, and no deliverable code changed since the prior PASS.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:c198ed529693 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | pass exit=0 sandbox=container-netns | sha256:317e34e516cb | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | pass exit=0 sandbox=container-netns | sha256:346457f3f5a2 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:5b8410d3d93c | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |
| 5 | `o=$(statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 sandbox=container-netns | sha256:9845e86678a2 | 2026-10-20 | assay-verifier-app[bot] @ 742839555dcd (on-behalf-of human:ian) (forge-identity) |

Receipt-bearing re-pass, 2026-10-20. Subject revision: merged main 742839555dcd1731139b15a295cf78cdbad9dc91; delivering change 2013b8dc6128 with first parent 613cce8e72cb. The witness table above is the execution witness, a mechanical re-run of every Verify row exactly as written by statusgen verifyrun with sandbox=container-netns. Runner class: local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), started with --network none, the engine's default seccomp profile, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-count=1, KUBECONFIG=/dev/null; statusgen and the desk tools were built inside the container from the tree at the subject revision. This record follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun exited 0 and its check reported 5 pass, 0 fail, 0 could-not-run of 5 Verify rows. The latest prior Evidence on main is the 2026-10-19 container-witness record at 0e661b6e15de, landed by medici-finance/assay#3522; this pass exists to carry the outcome receipt that record could not derive.
Verification-Attestation: medici-finance/assay#3646 run=23b293fe01cf4b7a5a7df711b4840db527ebc481a91d1ba4d16f0f6a8c058904 source=742839555dcd1731139b15a295cf78cdbad9dc91 brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong

The substance is the by-hand Evidence block of 2026-10-19 at e4d474af8bca, in which every Verify row passed with 0 fail. Since the prior record at 0e661b6e15de, the perf-v3 work of desktools-v2/63 changed code in this brief's declared paths (perfrec gained v3 sources, and perfrec.go, schema.go, session.go, deskkit perf.go and perfread.go and the deskperf main and report sources were extended additively). Rows 1–4 re-ran green against that code at the subject revision, and rows 1, 2, 3 and 5 printed output byte-identical (same hash) to the prior witness; row 4's hash differs, which the elapsed-time figure in its PASS line can account for, since that test runs a sub-second external build. Row 5's hash equals the prior witness's, so it printed the same 3 back-reference corroborations (desktools-v2/62, statusgen/26, performance-collection/02), 5 self-routed passes that corroborate nothing, and 1 UNCHECKED out-of-scope qualgen and standalone tools entry, which stays a review judgment.

Risk-bearing values: the trigger fires because the declared path tools/desk/internal/deskkit/perf.go matches the security-path trigger. Enumerated over this brief's literals at the subject revision and over the changes to its declared paths since 0e661b6e15de: SchemaV2 = "perf-v2" @ perfrec/schema.go:26; Schema = "perf-v1" @ perfrec/perfrec.go:44; MaxPhases = 16 @ perfrec/schema.go:94 (reversible, ranked last); labelPattern @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105; RetentionDays = 7 @ perfrec/perfrec.go:50; modes 0o700 @ perfrec/perfrec.go:351 and 0o600 @ perfrec/perfrec.go:387 (pre-existing; lines moved by the v3 insertions, values unchanged). The one new literal in those changes, SchemaV3 = "perf-v3" @ perfrec/v3.go:9, is a desktools-v2/63 deliverable and is derived by that brief's verification, not here.
RISK-VALUE: DERIVED — SchemaV2 = "perf-v2" @ perfrec/schema.go:26 — Schema = "perf-v1" @ perfrec/perfrec.go:44 is frozen and perf-v2 adds and drops fields, so a distinct tag is required; at the subject revision the marshaller still refuses v3 data on a perf-v1 or perf-v2 record, so the tags stay disjoint.
RISK-VALUE: DERIVED — labelPattern = ^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$ @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105 — unchanged since the prior derivation: the character class rejects slash, dot, underscore, equals, space and uppercase, so paths, slugs, underscored token prefixes, flag=value argv and spaced free text map to "other"; it still admits a lowercase alphanumeric string up to 40 characters, so roster membership at the callers stays the real gate.

**VERIFY: PASS** — 5/5 rows pass with 0 fail in the network-off container witness at 742839555dcd, on top of the by-hand substance of 2026-10-19.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationBoundaryAndCPUParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationBoundaryAndCPUParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:c198ed529693 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceSchemaAndPrivacyCompatibility$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceSchemaAndPrivacyCompatibility ' "$out")` | pass exit=0 sandbox=container-netns | sha256:317e34e516cb | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestMixedSchemaReportBoundaries$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestMixedSchemaReportBoundaries ' "$out")` | pass exit=0 sandbox=container-netns | sha256:346457f3f5a2 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd perfrec && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPublicRecorderConsumerBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPublicRecorderConsumerBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:5a11240c23f9 | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
| 5 | `o=$(statusgen --consumers --root . --brief assay:assay:desktools-v2:61 --delivery 2013b8dc612842467d08330e298b5401ce855e5f --base 613cce8e72cb95ec21d3f63120be68585fc91899); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 sandbox=container-netns | sha256:cbe002affc1f | 2026-11-02 | assay-verifier-app[bot] @ 2c6c21336a60 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c
Container-Image: sha256:097db7d3e20415bc7a2c524250e2ad89158ed93a31a7d8cd861e6cd38fc3b3b4
Verification-Attestation: medici-finance/assay#5261 run=0bbc632da42e3b22a0c166a431263c5d5f5df4e9b02e98ec32205a36baf547a0 source=2c6c21336a606e64df057294f21177f16db7b9f8 brief=docs/streams/desktools-v2/brief-61-version-shared-performance-boundaries-and-keep-legacy-reports-comparable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=0bbc632da42e3b22a0c166a431263c5d5f5df4e9b02e98ec32205a36baf547a0 source=2c6c21336a606e64df057294f21177f16db7b9f8 tree=sha256:ddf14768e4b78f02e94f6fae4c310c08dc06e0a8db50a2fb68244111759cd68c witness=sha256:156e1b87075c4a023bc521f6e1cf8dc87b744d57165b0d5ac6fdab4bd9dd0ede opened=2026-11-02T01:36:36Z closed=2026-11-02T01:36:53Z before=receipt after=check-verifier

Batch re-witness, 2026-11-02. Subject revision: merged main 2c6c21336a606e64df057294f21177f16db7b9f8, the single pinned commit of the driver's one-time batch re-witness (medici-finance/assay#2026); delivering change 2013b8dc6128 with first parent 613cce8e72cb. The witness table above is the execution witness: one closed admission bracket (host open-bracket, container run, host close-bracket) over every Verify row exactly as written, by statusgen verifyrun with sandbox=container-netns. Runner class: local docker container, image ghcr.io/medici-finance/verify-runner (linux/arm64, native, digest on the Container-Image line, statusgen v1.0.66 inside), started with --network none as non-root uid 501, with the home and the git common directory mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null. Host: darwin/arm64, statusgen v1.0.69, git 2.56.0. The bracket closed and bracket-digest reported verdict=match; the container run reported 5 pass, 0 fail, 0 could-not-run of 5 Verify rows.

Rows 1, 2 and 3 printed output byte-identical (same hash) to the 2026-10-20 witness at 742839555dcd. Row 4's hash differs; its PASS line carries the elapsed time of a sub-second external build. Row 5's hash also differs from that witness, which used a statusgen built from the tree at that revision. A supplemental by-hand run of row 5's statusgen call in the same image (no acceptance credit) exited 0 and printed summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing. Per the row's own Expect: 3 are back-reference corroborations (follow-ups routed to desktools-v2/62, statusgen/26 and performance-collection/02); 5 are self-routed to desktools-v2/61 (perfrec, deskkit perf and perfread, deskperf, cellctl perf and step readers, telemetry and desk README docs) and corroborate nothing about the delivered paths; the 1 UNCHECKED entry is the out-of-scope qualgen and standalone tools entry, a review judgment and not a pass.

Risk-bearing values: the trigger fires because the declared path tools/desk/internal/deskkit/perf.go matches the security-path trigger. Enumerated over this brief's literals at the subject revision and over the changes to its declared paths since 742839555dcd (the perf-v4 readers and shared reduction, build-stamp fallback, histogram and lifecycle sources, deskkit perf and perfread, deskperf main, report and v4): SchemaV2 = "perf-v2" @ perfrec/schema.go:26; Schema = "perf-v1" @ perfrec/perfrec.go:44; MaxPhases = 16 @ perfrec/schema.go:94 (reversible, ranked last); labelPattern @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105; RetentionDays = 7 @ perfrec/perfrec.go:50; modes 0o700 @ perfrec/perfrec.go:372 and 0o600 @ perfrec/perfrec.go:388 and :408 (pre-existing; lines moved, values unchanged). None of those lines changed since 742839555dcd. The new literals in that range, among them SchemaV4 = "perf-v4" @ perfrec/v4.go:17 and RevisionLen = 12 @ perfrec/buildstamp.go:33, are deliverables of later briefs (the perf-v4 envelope is desktools-v2/67) and are derived by those briefs' verification, not here.
RISK-VALUE: DERIVED — SchemaV2 = "perf-v2" @ perfrec/schema.go:26 — Schema = "perf-v1" @ perfrec/perfrec.go:44 is frozen and perf-v2 adds and drops fields, so a distinct tag is required; the later tags perf-v3 @ perfrec/v3.go:9 and perf-v4 @ perfrec/v4.go:17 are distinct strings, so the four tags stay disjoint and readers route by exact tag.
RISK-VALUE: DERIVED — labelPattern = ^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$ @ perfrec/schema.go:100 with len(s) <= 64 @ perfrec/schema.go:105 — unchanged: the character class rejects slash, dot, underscore, equals, space and uppercase, so paths, slugs, underscored secret prefixes, flag=value argv and spaced free text map to "other"; it still admits a lowercase alphanumeric string up to 40 characters, so roster membership at the callers stays the real gate.

**VERIFY: PASS** — 5/5 rows pass with 0 fail in the closed container-bracket witness at 2c6c21336a60 (bracket-digest verdict=match).

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
