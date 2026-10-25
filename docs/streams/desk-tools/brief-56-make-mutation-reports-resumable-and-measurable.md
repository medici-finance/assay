---
brief: assay:assay:desk-tools:56
title: Make mutation reports resumable and measurable
why: "Long mutation sweeps currently expose results only at normal completion, so an interruption loses both progress and the timing data needed for optimisation. An optional run-bound state file will make retries resumable and measurements auditable without reusing stale verdicts."
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
  - tools/desk/cmd/muhar/README.md
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
exec-tier: strong
exec-tier-why: "Resume correctness depends on source identity, interruption cleanup and durable-state invariants that a green final report alone cannot prove."
domain: complicated
consumers:
  - "tools/desk/cmd/muhar/README.md: follow-up desk-tools/56 (this brief; flips to fixed-here when the state contract is documented)"
---

# Brief 56 — Make mutation reports resumable and measurable

## Context
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/harness.go, tools/desk/cmd/muhar/runner.go, tools/desk/cmd/muhar/state.go, tools/desk/cmd/muhar/state_test.go, tools/desk/cmd/muhar/README.md, changelog/muhar-resume-state.md
facts:
- The existing runner prints its final report at completion; an interrupted sweep has no resumable result record.
- A healthy harness can exit zero with NOT_CAUGHT or COULD_NOT_MUTATE, so resume must preserve verdicts verbatim rather than infer health from exit status.
- The existing per-run GOCACHE quarantine and process-group cleanup remain mandatory.
- Callers, not the manifest, own state paths. A fixed shared state path would race independent runs and is forbidden.

layering: Keep state as a small append-only adapter around the existing Harness. Verdict production stays in the runner; the state layer persists observations and refuses incompatible reuse, with no second mutation engine or daemon.
design-fit:
  owner: muhar run reporting and Harness execution
  contract: none — this adds a local file contract inside the existing tool owner
  retires: [end-only reporting, repeated completed work after an interrupted retry]
  weight: "One optional -state flag and its invalid-identity/concurrent-writer refusals; no new verb."
  why-add: "Resume needs an explicit caller-owned durable path; environment-global or fixed manifest paths were rejected because they create races and stale reuse."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never cache infrastructure errors, an incomplete record, or a result from a different source/test/toolchain identity.

## Task
1. Add optional `-state PATH`. When supplied, write a versioned JSONL header and one atomically appended record per completed mutation with its stable spec index, verdict, build duration, test duration, timeout classification, and selection description, alongside the pristine-baseline and positive-control durations so optimisation sees phase timing, not only per-mutation timing. Keep ordinary stdout/stderr report behaviour unchanged.
2. Bind the header to a canonical identity covering the spec bytes, the relevant source and test files (the package set the spec's test command compiles), dependency metadata, Go toolchain, build/test flags, the run mode — `-select` on/off and the `-shard i/n` selection, since a narrowed run can report CAUGHT where the full command survives, so verdicts do not transfer across modes — and runner schema version. `-j` needs no binding: it changes scheduling, not verdicts. A resume with any identity mismatch refuses before planting a mutation.
3. On a same-identity retry, rerun the pristine baseline and positive control, then skip only mutations with complete definitive records. Do not reuse HARNESS BROKEN, interrupted, infrastructure-error, or partially written records. The final report must include resumed and freshly executed results in the original spec order.
4. Serialize writers to one state file with the existing cross-platform lock owner (`deskkit.TryLockExclusive`/`UnlockFile` in `tools/desk/internal/deskkit`; muhar is in the same `tools/desk` module, so the import adds no release dependency). Flush each record before reporting the mutation complete. On SIGINT/SIGTERM, kill child process groups (Unix process groups as today; on Windows the existing top-process-only kill remains, as `process_windows.go` documents), restore the source tree, and leave a syntactically valid state file that a later invocation can resume.
5. Add `TestStateResume`, `TestStateRejectsNewTree`, `TestStateSkipsNoErrors`, and `TestStateEndToEnd`. `TestStateResume` covers interruption after a completed record and corrupt trailing data (truncate or refuse on a partial trailing record, never resume past it); `TestStateRejectsNewTree` covers changed source/tests/toolchain identity and changed run mode (`-select` off→on, `-shard i/n` changed or removed); `TestStateSkipsNoErrors` covers non-reuse of HARNESS BROKEN, interrupted and infrastructure-error records; `TestStateEndToEnd` covers concurrent writers (the lock holder writes, the second writer refuses without writing) and exact final-report parity with an uninterrupted run.
6. Document the state schema, identity, cleanup, and operator-owned path contract. Change the consumer entry to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | exit 0; all four named PASS lines; resume, identity refusal, error non-reuse and final parity are exercised | check:ci +mutation +flow |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | exit 0; existing manifests remain valid | check:ci +neighbour |
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the documented state contract is in the delivering diff and corroborated | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Failure map
- A changed tree reuses an old verdict: TestStateRejectsNewTree fails.
- An interrupted or failed run becomes a completed result: TestStateSkipsNoErrors fails.
- Resume changes verdict order or content: TestStateEndToEnd's uninterrupted comparison fails.
- Two callers race one state file: the concurrent-writer case in row 1's `TestStateEndToEnd` refuses — the lock holder writes, the second writer refuses without writing.
- A changed run mode reuses another mode's verdicts: `TestStateRejectsNewTree`'s `-select`/`-shard` case fails.
- The state option is documented without its delivery: row 3 fails.

## Review
Gate: model. Focus on stale-result refusal, crash consistency and exact report equivalence, not JSON shape alone.
