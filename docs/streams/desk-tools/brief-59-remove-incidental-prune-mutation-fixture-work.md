---
brief: assay:assay:desk-tools:59
title: Remove incidental prune mutation fixture work
why: "The prune mutation sweep repeats expensive performance fixtures for every mutation, including mutations they cannot detect. Move the correctness invariants into focused tests so verification measures the guards rather than fixture construction."
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
  - tools/desk/cmd/deskwt/pruneperf_test.go
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
domain: complicated
consumers:
  - "tools/desk/cmd/deskwt/prune-mutations.json: follow-up desk-tools/59 (this brief; flips to fixed-here when the implementation edits the manifest)"
---

# Brief 54 — Remove incidental prune mutation fixture work

## Context
files: tools/desk/cmd/deskwt/prune-mutations.json, tools/desk/cmd/deskwt/pruneperf_test.go, tools/desk/cmd/deskwt/prune_test.go, tools/desk/cmd/deskwt/prunesingleton_test.go, changelog/muhar-prune-fixtures.md
facts:
- On 2026-10-24 the manifest selected 151 top-level tests for the baseline, positive control and each of 55 mutations.
- TestPruneSweepScalesAtN and TestConcurrentPrunesCostOne each construct a 5,000-commit history and 50 worktrees by default; only the ancestor-walk mutation needs the scaling test's one-walk invariant.
- The singleton interval test sleeps 300 ms and can perform six sweeps; runPruneLoop already sweeps once before selecting on stop.
- Keep real Git fixtures and the existing performance tests in their ordinary verification rows. This brief removes incidental work from the mutation lane, not performance coverage.

design-fit:
  owner: deskwt prune tests and the prune mutation manifest
  contract: none — this rearranges existing test coverage without changing the production contract
  retires: [performance fixtures in unrelated mutation runs, the multi-sweep interval sleep]
  weight: "No new verbs, flags, refusals or rule text; one focused test replaces indirect performance-only coverage."
  why-add: "A tiny correctness fixture is required because the 5,000-commit performance fixture is the wrong owner for a mutation-detection invariant."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not weaken or delete the existing scale, concurrency, or real-Git assertions.

## Task
1. Add `TestPruneWalksMainOnce` using a small real-Git fixture. It must prove that one sweep performs exactly one mainline ancestor walk through the existing walk seam, including the shape exercised by the ancestor-walk mutation.
2. Keep `TestPruneSweepScalesAtN` and `TestConcurrentPrunesCostOne` unchanged in their ordinary performance rows. Add `-skip '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$'` to the prune mutation manifest so neither runs for unrelated mutations.
3. Rework the singleton interval test as `TestPruneIntervalLockRelease`: use one immediate tick and a pre-closed stop channel, then prove the singleton can be reacquired. Remove the 300 ms sleep and extra sweeps without removing the lock-release assertion.
4. Preserve every existing mutation anchor and the exact 55-caught gate. Run the complete manifest to prove the small walk test catches the ancestor-walk mutation and that no other mutation survives.
5. Change this brief's consumer entry from `follow-up desk-tools/59` to `fixed-here` in the implementation PR. Do not leave a deferred routing claim after the manifest edit lands.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-walk.log" 2>&1 && grep -F -- '--- PASS: TestPruneWalksMainOnce ' "${TMPDIR:-/tmp}/dt59-walk.log"` | exit 0; named PASS; the test reports exactly one mainline walk | check:ci +mutation |
| 2 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneIntervalLockRelease$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-interval.log" 2>&1 && grep -F -- '--- PASS: TestPruneIntervalLockRelease ' "${TMPDIR:-/tmp}/dt59-interval.log"` | exit 0; named PASS with one tick and no interval sleep | check:ci |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt59-anchors.log"` | exit 0; every manifest anchor still lands | check:ci |
| 4 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | exit 0; exact 55 caught, zero survivors and zero unapplied mutations | check +mutation +flow |
| 5 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/59 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the implementation diff contains the manifest and its routing is corroborated | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map
- The small walk test misses the mutation: row 4 remains red with a surviving ancestor-walk mutation.
- Performance coverage is accidentally removed: the performance files and their ordinary rows remain unchanged; review must reject their deletion.
- The interval test becomes a timing-only check: row 2 must prove lock reacquisition after a real tick.
- The manifest changes but its consumer claim stays deferred: row 5 fails.

## Review
Gate: model. Check that mutation coverage moved rather than disappeared and that the full 55-mutation result is mechanical.
