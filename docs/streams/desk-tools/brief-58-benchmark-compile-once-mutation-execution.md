---
brief: assay:assay:desk-tools:58
title: Benchmark compile-once mutation execution
why: "Selection removes test work, but some mutation suites remain compile-bound. A controlled comparison of direct test-binary execution and the existing schemata implementation will show which builds are safe to reuse before any workflow pays for or adopts a new execution path."
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
delivery:
  - {in: "assay#4494", covers: partial, note: "authoring only: Verify-row rewrite and changelog, none of the deliverable"}
schema: brief-v2
version: 1
authored: 2026-10-24 by maintainer-directed mutation review
sources:
  - medici-finance/assay#4359
  - tools/desk/cmd/muhar/schemata.go
  - https://stryker-mutator.io/blog/announcing-stryker-4-mutation-switching/
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
exec-tier: strong
exec-tier-why: "Build overlays, test-binary reuse and schemata can change observable mutation semantics; adoption needs parity and resource measurements across modes."
domain: complex
---

# Brief 58 — Benchmark compile-once mutation execution

## Context
sequencing: desk-tools/56 (brief merged in medici-finance/assay#4364) lists the same `main.go`, `harness.go`, `runner.go` and `README.md`; if its implementation is still in flight, it lands first and this one merges main before editing the shared files — a soft ordering, not a dependency.
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/harness.go, tools/desk/cmd/muhar/runner.go, tools/desk/cmd/muhar/schemata.go, tools/desk/cmd/muhar/buildmode_test.go, tools/desk/cmd/muhar/classification_test.go, tools/desk/cmd/muhar/schemata_e2e_test.go, tools/desk/cmd/muhar/schemata_test.go, tools/desk/cmd/muhar/README.md, changelog/muhar-build-modes.md
facts:
- Schemata already compiles eligible mutants once behind runtime selection and reruns green schemata results classically; this brief must not build a second schemata implementation.
- Selection-narrowed mutants deliberately run classically, so schemata is not a substitute for test selection.
- On 2026-10-24 the untouched base's schemata tests failed wherever the temp root resolves through a symlink — the default macOS per-user temp dir under `/var` does — with overlay module admission and source-position mismatches: paths track the symlink target, not the caller's spelling. The same tests pass with a fully resolved TMPDIR, so the trigger is path resolution, not the OS. Verify row 1 builds and forces the symlinked condition itself, so it fails on unfixed code on every runner class.
- Go test-result caching remains disabled for verdict runs. Reusing a byte-identical test binary must never reuse a previous mutant's test result.

layering: Keep execution modes behind the existing Harness command boundary. Direct-binary support reuses the current plain-Go command parser and fallback; no compiler fork, persistent test process, or second mutation engine.
design-fit:
  owner: muhar build and test execution
  contract: none — this optimises existing execution internals and documents measured behaviour
  retires: [duplicate go-command setup where direct execution is proven equivalent, host-specific schemata failures]
  weight: "No new user-facing flag: unsupported commands retain the classic path; schemata keeps its existing opt-in."
  why-add: "Internal direct execution is justified only where parity and a measured gain hold; an always-on new mode without fallback was rejected."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not enable schemata or direct execution for the prune gate in this brief. A later adoption decision needs the selected workload's own parity evidence.

## Task
1. Diagnose and fix the existing host-side schemata failures, including overlay path identity and classic-versus-schema source positions. Preserve the existing fallback and line-mapping contract; do not weaken assertions to make the benchmark runnable.
2. Add an internal direct execution path for supported plain `go test` commands: compile the package test binary once per mutant with `go test -c`, then execute selected test groups directly with fresh processes and `-test.count=1`. Unsupported commands, TestMain semantics, subprocess overlays, and source-reading tests retain the classic path.
3. Add `TestDirectParity` and `TestDirectUnsupported`. Cover caught, survived, timeout, compile-broken, selected-test, TestMain, and source-reading cases. Never turn a build or infrastructure error into CAUGHT.
4. Add `BenchmarkMutationBuildModes` over one committed, representative compile-heavy fixture. Measure classic, direct, and schemata wall time, CPU time and peak memory where the host exposes it, emitting the three modes as sub-benchmarks named `BenchmarkMutationBuildModes/classic`, `/direct` and `/schemata`, plus one `mode-eligibility: <mode>=<eligible-or-reason>` line per mode in the benchmark output, so Verify row 3's exit status asserts every mode was measured with its eligibility. Record the toolchain, host class, workload and mode eligibility in `tools/desk/cmd/muhar/README.md`.
5. Keep direct execution off unless classic-versus-direct per-mutant parity is exact and the benchmark shows at least 20% lower whole-sweep wall time without more timeout or infrastructure outcomes. If the criterion is not met, retain the classic default and record the negative result rather than claiming a speedup. Either way, pin the outcome in exactly one `tools/desk/cmd/muhar/README.md` line — `build-mode default: classic, decided on <date> against the recorded measurements` (or `direct`, with its parity and gain evidence) — which Verify row 5 asserts.
6. Document remaining limitations and the separate adoption evidence required for real manifests. Do not change `prune-mutations.json`, `-select`, timeout policy, or the classic CI lane.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && X=$(mktemp -d "${TMPDIR:-/tmp}/dt58.XXXXXX") && mkdir "$X/real" && ln -s "$X/real" "$X/link" && L=$(mktemp "${TMPDIR:-/tmp}/dt58-schema.XXXXXX") && TMPDIR="$X/link/" go test ./cmd/muhar -run '^TestSchema' -count=1 -v > "$L" 2>&1 && grep -F -- '--- PASS: TestSchemataOneBuildPerWorker ' "$L" && grep -F -- '--- PASS: TestSchemaPositionsMatchClassic ' "$L"` | exit 0 with the temp root forced through a symlink (the failure trigger, on any runner class); named PASS lines; one-build and source-position contracts hold under an unresolved path | check:ci +mutation |
| 2 | `cd tools/desk && d=$(mktemp -d "${TMPDIR:-/tmp}/dt58.XXXXXX") && go test ./cmd/muhar -run '^TestDirect' -count=1 -v > "$d/direct.log" 2>&1 && grep -F -- '--- PASS: TestDirectParity ' "$d/direct.log" && grep -F -- '--- PASS: TestDirectUnsupported ' "$d/direct.log" && rm -rf "$d"` | exit 0; both named PASS lines; supported parity and loud classic fallback are exercised | check:ci +mutation +flow |
| 3 | `cd tools/desk && L=$(mktemp "${TMPDIR:-/tmp}/dt58-bench.XXXXXX") && go test ./cmd/muhar -run '^$' -bench '^BenchmarkMutationBuildModes$' -benchtime 1x -timeout 30m > "$L" 2>&1 && grep -F 'BenchmarkMutationBuildModes/classic' "$L" && grep -F 'BenchmarkMutationBuildModes/direct' "$L" && grep -F 'BenchmarkMutationBuildModes/schemata' "$L" && test "$(grep -c -F 'mode-eligibility: ' "$L")" -eq 3` | exit 0; all three mode sub-benchmarks and three mode-eligibility lines present, so only a benchmark that measures every mode and states its eligibility passes | check:ci +dereference |
| 4 | `cd tools/desk && d=$(mktemp -d "${TMPDIR:-/tmp}/dt58.XXXXXX") && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "$d/anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "$d/anchors.log" && rm -rf "$d"` | exit 0; existing manifests and their execution defaults remain valid | check:ci +neighbour |
| 5 | `cd tools/desk && test "$(grep -c -e 'build-mode default: classic, decided on' -e 'build-mode default: direct, decided on' cmd/muhar/README.md)" -eq 1` | exit 0; exactly the shipped default is recorded against the measured decision, not left implicit | check:ci |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map
- Schemata is benchmarked while fundamentally broken on the host: row 1 fails before adoption.
- Direct execution changes a verdict or drops TestMain/source-reading semantics: row 2 fails.
- A benchmark reports only the favourable mode or omits eligibility: row 3 fails on the missing sub-benchmark or eligibility line.
- A marginal result becomes an always-on default: the 20% and parity criteria in Task block it.
- The prune gate changes without workload-specific parity: this brief's file boundary and review reject it.

## Review
Gate: model. Review the parity matrix and benchmark conditions, including negative results; compile-time savings alone do not justify adoption.
