---
brief: assay:assay:desktools-v2:61
title: "Version shared performance boundaries and keep legacy reports comparable"
why: "Current desk wall time excludes entrypoint setup while CPU includes process lifetime, and readers cannot distinguish several command modes. A shared versioned measurement boundary makes the local records usable for comparing optimizations across desk tools and statusgen without treating missing measurements as zero."
wave: 1
depends: []
unblocks: ["desktools-v2/62", "statusgen/26", "performance-collection/02"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 1
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

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
