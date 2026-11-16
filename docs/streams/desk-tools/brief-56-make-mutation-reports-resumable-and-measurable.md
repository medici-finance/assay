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
  - "tools/desk/cmd/muhar/README.md: fixed-here (documents the state schema, identity, cleanup and operator-owned path contract)"
---

# Brief 56 — Make mutation reports resumable and measurable

## Context
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/harness.go, tools/desk/cmd/muhar/runner.go, tools/desk/cmd/muhar/state.go, tools/desk/cmd/muhar/state_test.go, tools/desk/cmd/muhar/README.md, docs/contracts.md, changelog/muhar-resume-state.md
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
6. Document the state schema, identity, cleanup, and operator-owned path contract. Record its identity/writer/reuse invariant in the existing `docs/contracts.md` rule-register census; this documents the same optional state contract and adds no verb. Change the consumer entry to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | exit 0; all four named PASS lines; resume, identity refusal, error non-reuse and final parity are exercised | check:ci +mutation +flow |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | exit 0; existing manifests remain valid | check:ci +neighbour |
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the documented state contract is in the delivering diff and corroborated | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->
### Non-implementer verifier run — VERIFY: PASS — 2026-11-16 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main b7c7b20e214d4cf5fa423ab0a590741ddccffe35; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.85, verify-runner v1.0.85 index digest sha256:271d9b171d01 (linux/arm64 manifest run). Admission: deskdispatch check-verifier rc=0 before the rows and again before hand-back.

Rows 1 and 2 (check:ci) ran inside one closed container-netns admission bracket: docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2. The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:edc16502aaad | 2026-11-16 | assay-verifier-app[bot] @ b7c7b20e214d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:ac37bd9cc2b7 | 2026-11-16 | assay-verifier-app[bot] @ b7c7b20e214d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:c269d82a105cedc28fcc6ef082a387f24a4ecb883af793c9029f6c530822d4b8
Container-Image: sha256:271d9b171d01592693dc055717f2aa22661bae2ed2e6d59d6e50ecfc0fa5943a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6622 run=5d584484d4a845771023b86a0a5af448a9dfb9a8462071352770b2476a005acd source=b7c7b20e214d4cf5fa423ab0a590741ddccffe35 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=5d584484d4a845771023b86a0a5af448a9dfb9a8462071352770b2476a005acd source=b7c7b20e214d4cf5fa423ab0a590741ddccffe35 tree=sha256:c269d82a105cedc28fcc6ef082a387f24a4ecb883af793c9029f6c530822d4b8 witness=sha256:4200415bbd3c2cc62d3542cbc4434a643d90d21853dad17667a46993691725a9 opened=2026-11-16T16:46:49Z closed=2026-11-16T16:48:10Z before=receipt after=check-verifier

Row 3 (class check, it needs git history) ran on the host through statusgen verifyrun with --rows 3 and --dry-run, with DELIVERY_HEAD = b7c7b20e214d4cf5fa423ab0a590741ddccffe35 (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-16 | assay-verifier-app[bot] @ b7c7b20e214d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6622 run=5d584484d4a845771023b86a0a5af448a9dfb9a8462071352770b2476a005acd source=b7c7b20e214d4cf5fa423ab0a590741ddccffe35 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0; four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:edc16502aaad; the and-chain requires every grep to match, so all four named PASS lines were present.
- Row 2 (check:ci +neighbour): expected exit 0; anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:ac37bd9cc2b7.
- Row 3 (check +flow +dereference): expected exit 0; consumers corroborated; observed pass exit=0 on host, output sha256:6378c2f1c28a; matched the line summary: 1 corroborated, 0 disproved, 0 unchecked.

Supplemental observations (no acceptance credit, outside every attempt). In the same image under the same network-none flags, go test ./cmd/muhar -run '^(TestState|TestSpecAnchorsLand$)' -count=1 -v in tools/desk exited 0 with zero SKIP lines. It listed PASS for the state-resume test and its identity-interrupt subtest; the new-tree test with source, tests, select, shard-change, shard-remove, runner-build, command-words, descriptor-type, descriptor-error, unsafe-state and Go-inputs; the skips-no-errors test with external-signal, fault-signals and restore-error; the end-to-end test with shard-indices, selection-timing and schemata-timing; and the anchors test. On the host, statusgen --consumers for desk-tools/56 with base 7d119c295ffd printed CORROBORATED for the muhar README fixed-here entry. A source read confirmed each Task obligation: the torn trailing record is refused with "incomplete trailing record" and runs no command; the second writer refuses on "writer lock" and leaves the bytes unchanged; HARNESS BROKEN, interrupted, incomplete, infrastructure-error and field-missing records are re-executed; resume, -j and uninterrupted reports are byte-identical; the writer lock uses the deskkit TryLockExclusive call in the muhar state source at line 102; the R-muhar-state row was added to the docs/contracts.md register.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag (statusgen riskBearingRowRe), so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources). The trigger does not fire: irreversible is no, all risk fields are present and no, and no changed path matches .assay-surfaces or the protected set. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; permission mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse needs LimitNS > 0; BuildNS >= 0 and TestNS >= 0, with key presence enforced at line 150) at tools/desk/cmd/muhar/state.go:167-169. All of these are reversible with an edit and a redeploy, because the state file is a local, caller-owned file.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — this is the first version of a new on-disk contract, and it is hashed into the header identity (state.go:323), so any later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input (muhar README), so a file other users can write could plant reusable CAUGHT verdicts. Refusing the group and world write bits (0o020|0o002) is exactly that exclusion, and it pairs with the owner-only 0o600 create mode at state.go:94.

**VERIFY: PASS** — 3/3 rows pass (rows 1-2 in a closed container-netns bracket, row 3 on the host); no defect found.

## Failure map
- A changed tree reuses an old verdict: TestStateRejectsNewTree fails.
- An interrupted or failed run becomes a completed result: TestStateSkipsNoErrors fails.
- Resume changes verdict order or content: TestStateEndToEnd's uninterrupted comparison fails.
- Two callers race one state file: the concurrent-writer case in row 1's `TestStateEndToEnd` refuses — the lock holder writes, the second writer refuses without writing.
- A changed run mode reuses another mode's verdicts: `TestStateRejectsNewTree`'s `-select`/`-shard` case fails.
- The state option is documented without its delivery: row 3 fails.

## Review
Gate: model. Focus on stale-result refusal, crash consistency and exact report equivalence, not JSON shape alone.
