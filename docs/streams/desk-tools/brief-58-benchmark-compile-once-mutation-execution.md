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
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/harness.go, tools/desk/cmd/muhar/runner.go, tools/desk/cmd/muhar/schemata.go, tools/desk/cmd/muhar/buildmode_test.go, tools/desk/cmd/muhar/classification_test.go, tools/desk/cmd/muhar/schemata_e2e_test.go, tools/desk/cmd/muhar/schemata_test.go, tools/desk/cmd/muhar/README.md, changelog/muhar-build-modes.md
facts:
- Schemata already compiles eligible mutants once behind runtime selection and reruns green schemata results classically; this brief must not build a second schemata implementation.
- Selection-narrowed mutants deliberately run classically, so schemata is not a substitute for test selection.
- On 2026-10-24 the untouched base's schemata tests failed on macOS, including overlay module admission and source-position mismatches. Benchmark or adopt only after those tests pass on the target host.
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
4. Add `BenchmarkMutationBuildModes` over one committed, representative compile-heavy fixture. Measure classic, direct, and schemata wall time, CPU time and peak memory where the host exposes it. Record the toolchain, host class, workload and mode eligibility in `tools/desk/cmd/muhar/README.md`.
5. Keep direct execution off unless classic-versus-direct per-mutant parity is exact and the benchmark shows at least 20% lower whole-sweep wall time without more timeout or infrastructure outcomes. If the criterion is not met, retain the classic default and record the negative result rather than claiming a speedup.
6. Document remaining limitations and the separate adoption evidence required for real manifests. Do not change `prune-mutations.json`, `-select`, timeout policy, or the classic CI lane.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestSchema' -count=1 -v > "${TMPDIR:-/tmp}/dt58-schema.log" 2>&1 && grep -F -- '--- PASS: TestSchemataOneBuildPerWorker ' "${TMPDIR:-/tmp}/dt58-schema.log" && grep -F -- '--- PASS: TestSchemaPositionsMatchClassic ' "${TMPDIR:-/tmp}/dt58-schema.log"` | exit 0; named PASS lines; one-build and source-position contracts hold on the host | check:ci +mutation |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestDirect' -count=1 -v > "${TMPDIR:-/tmp}/dt58-direct.log" 2>&1 && grep -F -- '--- PASS: TestDirectParity ' "${TMPDIR:-/tmp}/dt58-direct.log" && grep -F -- '--- PASS: TestDirectUnsupported ' "${TMPDIR:-/tmp}/dt58-direct.log"` | exit 0; both named PASS lines; supported parity and loud classic fallback are exercised | check:ci +mutation +flow |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^$' -bench '^BenchmarkMutationBuildModes$' -benchtime 1x > "${TMPDIR:-/tmp}/dt58-bench.log" 2>&1 && grep -F 'BenchmarkMutationBuildModes' "${TMPDIR:-/tmp}/dt58-bench.log"` | exit 0; output contains comparable classic, direct and schemata measurements with their eligibility | check:ci +dereference |
| 4 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt58-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt58-anchors.log"` | exit 0; existing manifests and their execution defaults remain valid | check:ci +neighbour |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map
- Schemata is benchmarked while fundamentally broken on the host: row 1 fails before adoption.
- Direct execution changes a verdict or drops TestMain/source-reading semantics: row 2 fails.
- A benchmark reports only the favourable mode or omits eligibility: row 3 is incomplete and review rejects it.
- A marginal result becomes an always-on default: the 20% and parity criteria in Task block it.
- The prune gate changes without workload-specific parity: this brief's file boundary and review reject it.

## Review
Gate: model. Review the parity matrix and benchmark conditions, including negative results; compile-time savings alone do not justify adoption.
