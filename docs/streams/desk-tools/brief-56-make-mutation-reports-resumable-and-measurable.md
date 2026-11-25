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
Verdict correction for the run above (same run, same witness rows, nothing re-run). The verdict line above recorded the three rows' exit codes, not the run's closure verdict. The outcome record this run wrote (`docs/streams/verify-outcomes/desk-tools/56-20261116T181937Z-a347e9581b5e.json`) is blocked: rows 1 and 2 are credited; row 3 met its Expect, but its consumer command pins `--base` without `--delivery`, which the closure lint does not admit for closure (check-definition, deferred to #6659). No closure is claimed, and the status stays implemented. The witness rows above are unchanged.

**VERIFY: BLOCKED** — check-definition: rows 1-2 credited, row 3 deferred to #6659; status stays implemented.

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-11-18 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main 9c845402c8c00d7c14bda75cd5222fff907fd05d; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.87, verify-runner v1.0.87 index digest sha256:c67baa7a3c8b (linux/arm64). Re-run because declared inputs changed after the 2026-11-16 blocked record (muhar runner.go and README, docs/contracts.md). Admission: deskdispatch check-verifier rc=0 before the rows; deskpathguard rederive listed 3 original rows, 0 author-added, all run as delivered.

Rows 1 and 2 (check:ci) ran inside one closed container-netns admission bracket: docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2. The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:9d005734e57b | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:c7d3b30ab991 | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6963 run=bd3b76a8c6b54be84bd3539d3a2e0207e6551483db40227bb472bccf290a3adb source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=bd3b76a8c6b54be84bd3539d3a2e0207e6551483db40227bb472bccf290a3adb source=9c845402c8c00d7c14bda75cd5222fff907fd05d tree=sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715 witness=sha256:1fb000255709616ca7c0c58b8f1bc3a54728faaa0e59bca74622cc4bcb12b63f opened=2026-11-18T17:27:16Z closed=2026-11-18T17:28:52Z before=receipt after=check-verifier

Row 3 (class check, it needs git history) ran on the host through statusgen verifyrun with --rows 3 and --dry-run, with DELIVERY_HEAD = 9c845402c8c00d7c14bda75cd5222fff907fd05d (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6963 run=bd3b76a8c6b54be84bd3539d3a2e0207e6551483db40227bb472bccf290a3adb source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0 and four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:9d005734e57b; the and-chain requires every grep to match.
- Row 2 (check:ci +neighbour): expected exit 0 and the anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:c7d3b30ab991.
- Row 3 (check +flow +dereference): expected exit 0 and consumers corroborated; observed pass exit=0 on the host, output sha256:6378c2f1c28a. Held for closure: the command is unchanged at this source and pins --base without --delivery, and statusgen v1.0.87 lint still reports Verify row 3 [consumer-delivery-range] for this brief. Same check-definition blocker as the 2026-11-16 run (medici-finance/assay#6659).

Supplemental observations (no acceptance credit, outside every attempt). In the same image under the same network-none flags, go test ./cmd/muhar -run '^(TestState|TestSpecAnchorsLand$)' -count=1 -v in tools/desk exited 0 with zero SKIP lines; it listed PASS for TestStateResume (identity-interrupt), TestStateRejectsNewTree (source, tests, select, shard-change, shard-remove, runner-build, command-words, descriptor-type, descriptor-error, unsafe-state, Go-inputs), TestStateSkipsNoErrors (external-signal, fault-signals, restore-error), TestStateEndToEnd (shard-indices, selection-timing, schemata-timing) and TestSpecAnchorsLand. On the host, statusgen --consumers for desk-tools/56 with base 7d119c295ffd printed CORROBORATED for the muhar README fixed-here entry.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources; the state source is byte-identical at this HEAD). The trigger does not fire: irreversible is no and all risk fields are present and no. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; group/world-write mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse only for CAUGHT with LimitNS > 0; BuildNS >= 0, TestNS >= 0, Index >= 0) at tools/desk/cmd/muhar/state.go:167-169; interrupt exit 130 in the harness. All are reversible by an edit and a redeploy, because the state file is local and caller-owned.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — first version of a new on-disk contract, hashed into the header identity (state.go:323), so a later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input, so a file other users can write could plant reusable CAUGHT verdicts; refusing the group and world write bits is exactly that exclusion, paired with the owner-only 0o600 create mode at state.go:94.

**VERIFY: BLOCKED** — check-definition: rows 1-2 passed in a matching closed bracket (credit was could-not-check before landing), row 3 met its Expect but is held on consumer-delivery-range (medici-finance/assay#6659); status stays implemented.

### Non-implementer verifier run — 2026-11-19 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main 08834a1a6e1226100404e8479e6473d607629260; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.88. Re-run because a declared input changed after the 2026-11-18 record (docs/contracts.md); the muhar sources are byte-identical to that run. Admission: deskdispatch check-verifier rc=0 before any row.

Row 3 (class check, it needs git history) ran first, on the host, through statusgen verifyrun with --rows 3, with DELIVERY_HEAD = 08834a1a6e1226100404e8479e6473d607629260 (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-19 | assay-verifier-app[bot] @ 08834a1a6e12 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7224 run=d76015f0cf4270cbee7c4d084405b3c19f39ecc5cb891de71e9b0f4c68b37d63 source=08834a1a6e1226100404e8479e6473d607629260 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Rows 1 and 2 (check:ci) ran last, inside one closed container-netns admission bracket: verify-runner v1.0.88 index digest sha256:dd758003b77f (linux/arm64), docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2. The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:95123e112c5d | 2026-11-19 | assay-verifier-app[bot] @ 08834a1a6e12 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:c94a03fd7302 | 2026-11-19 | assay-verifier-app[bot] @ 08834a1a6e12 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:d9f8f2e4dd8764322dbc45d950a8b2e30cb0903e1a3d41cdced4efb81cc79aae
Container-Image: sha256:dd758003b77fc718ec5b9b3e3a34529a422eb87316a605207bdad2560ca169bc
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7224 run=d76015f0cf4270cbee7c4d084405b3c19f39ecc5cb891de71e9b0f4c68b37d63 source=08834a1a6e1226100404e8479e6473d607629260 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d76015f0cf4270cbee7c4d084405b3c19f39ecc5cb891de71e9b0f4c68b37d63 source=08834a1a6e1226100404e8479e6473d607629260 tree=sha256:d9f8f2e4dd8764322dbc45d950a8b2e30cb0903e1a3d41cdced4efb81cc79aae witness=sha256:13bf218347df343e2152088faf0c3dd4495ad71da9c9ce4a3c9a2b5c9404748b opened=2026-11-19T16:44:10Z closed=2026-11-19T16:45:40Z before=receipt after=check-verifier

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0 and four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:95123e112c5d; the and-chain requires every grep to match.
- Row 2 (check:ci +neighbour): expected exit 0 and the anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:c94a03fd7302.
- Row 3 (check +flow +dereference): expected exit 0 and consumers corroborated; observed pass exit=0 on the host, output sha256:6378c2f1c28a. Held for closure: the command is unchanged at this source and pins --base without --delivery. statusgen v1.0.88 lint reports Verify row 3 [consumer-delivery-range] for this brief as a NOTICE, and the rule is in promotedRowRules (statusgen strength.go), so it becomes a PROBLEM on the closing transition. Same check-definition blocker as the 2026-11-16 and 2026-11-18 runs (medici-finance/assay#6659, still open).

Supplemental observation (no acceptance credit, outside every attempt): on the host, statusgen --consumers for desk-tools/56 with base 7d119c295ffd printed CORROBORATED for the muhar README fixed-here entry and summary 1 corroborated, 0 disproved, 0 unchecked.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources; the state source is byte-identical at this HEAD, and the docs/contracts.md change since the last run touches no muhar line). The trigger does not fire: irreversible is no and all risk fields are present and no. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; group/world-write mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse only for CAUGHT with LimitNS > 0; BuildNS >= 0, TestNS >= 0, Index >= 0) at tools/desk/cmd/muhar/state.go:167-169. All are reversible by an edit and a redeploy, because the state file is local and caller-owned.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — first version of a new on-disk contract, hashed into the header identity (state.go:323), so a later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input, so a file other users can write could plant reusable CAUGHT verdicts; refusing the group and world write bits is exactly that exclusion, paired with the owner-only 0o600 create mode at state.go:94.

**VERIFY: BLOCKED** — check-definition: rows 1-2 passed in a matching closed bracket, row 3 met its Expect but is held on consumer-delivery-range (medici-finance/assay#6659); status stays implemented.

### Non-implementer verifier run — 2026-11-21 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main 43638ce59f736b6fd30ec32c318adef43b084ae8; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.90. Re-run because a declared input changed after the 2026-11-19 record (docs/contracts.md gained one S-cell-lifecycle register row, no muhar line); the muhar state source is byte-identical to the delivering merge. Admission: deskdispatch check-verifier rc=0 before any row.

Row 3 (class check, it needs git history) ran first, on the host, through statusgen verifyrun with --rows 3, with DELIVERY_HEAD = 43638ce59f736b6fd30ec32c318adef43b084ae8 (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-21 | assay-verifier-app[bot] @ 43638ce59f73 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7341 run=05ac2e5727bbcfc0fbde162f4a04c7db4b8ffef3ced9e4383a2abddc70fe192f source=43638ce59f736b6fd30ec32c318adef43b084ae8 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Rows 1 and 2 (check:ci) ran last, inside one closed container-netns admission bracket: verify-runner v1.0.90 index digest sha256:4e8e3c80a075 (the tools/verify-runner image.lock pin, linux/arm64 manifest run), docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2. The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:906e378464fc | 2026-11-21 | assay-verifier-app[bot] @ 43638ce59f73 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:77ea78900a36 | 2026-11-21 | assay-verifier-app[bot] @ 43638ce59f73 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:081a093f80c2809ef621159860b5394515915d50beea6fbe0f9b8fae24ca87ac
Container-Image: sha256:4e8e3c80a075c552de0966c886784c6020153c27e624f8a1ce988aa3b0590217
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7341 run=05ac2e5727bbcfc0fbde162f4a04c7db4b8ffef3ced9e4383a2abddc70fe192f source=43638ce59f736b6fd30ec32c318adef43b084ae8 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=05ac2e5727bbcfc0fbde162f4a04c7db4b8ffef3ced9e4383a2abddc70fe192f source=43638ce59f736b6fd30ec32c318adef43b084ae8 tree=sha256:081a093f80c2809ef621159860b5394515915d50beea6fbe0f9b8fae24ca87ac witness=sha256:efb300fc1b432498f6f8a5cb59b9f63a67a2e9c16c399bcddffd290f3061bced opened=2026-11-21T01:52:41Z closed=2026-11-21T01:53:26Z before=receipt after=check-verifier

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0 and four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:906e378464fc; the and-chain requires every grep to match.
- Row 2 (check:ci +neighbour): expected exit 0 and the anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:77ea78900a36.
- Row 3 (check +flow +dereference): expected exit 0 and consumers corroborated; observed pass exit=0 on the host, output sha256:6378c2f1c28a. Held for closure: the command is unchanged at this source and pins --base without --delivery. statusgen v1.0.90 lint reports Verify row 3 [consumer-delivery-range] for this brief as a NOTICE, and the rule is in promotedRowRules (statusgen strength.go), so it becomes a PROBLEM on the closing transition. Same check-definition blocker as the 2026-11-16, 2026-11-18 and 2026-11-19 runs (medici-finance/assay#6659, still open).

statusgen verifyrun --check for this brief printed: 3 pass, 0 fail, 0 could-not-run/missing (of 3 Verify rows); 0 author-added with no acceptance credit. That tally counts row 3's witness; the closure count here is 2 of 3, because row 3 is held on #6659.

Supplemental observation (no acceptance credit, outside every attempt): on the host, statusgen --root . --lint at this source exited 1 with three PROBLEMs, each naming another brief (desk-tools/37, desktools-v2/65, forge-neutral/54, all protected-table acceptance transitions that need --acceptance-forge); none names this brief. Its two NOTICEs for this brief are the row 3 consumer-delivery-range finding above and outcome-absent.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources; the state source is byte-identical between the delivering merge and this HEAD, and the docs/contracts.md change since the last run adds one S-cell-lifecycle row with no muhar line). The trigger does not fire: irreversible is no and all risk fields are present and no. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; group/world-write mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse only for CAUGHT with LimitNS > 0; BuildNS >= 0, TestNS >= 0, Index >= 0) at tools/desk/cmd/muhar/state.go:167-169. All are reversible by an edit and a redeploy, because the state file is local and caller-owned.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — first version of a new on-disk contract, hashed into the header identity (state.go:323), so a later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input, so a file other users can write could plant reusable CAUGHT verdicts; refusing the group and world write bits is exactly that exclusion, paired with the owner-only 0o600 create mode at state.go:94.

**VERIFY: BLOCKED** — check-definition: rows 1-2 passed in a matching closed bracket, row 3 met its Expect but is held on consumer-delivery-range (medici-finance/assay#6659); status stays implemented.

### Non-implementer verifier run — 2026-11-21 (second) — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main 2aafcfe485c74626db401b8209155f215015cc11; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.90. Re-run because a declared input changed after the earlier 2026-11-21 record: docs/contracts.md changed one S-context-handoff register row (a milestone-projection sentence, no muhar line). Between that record's source 43638ce59f73 and this HEAD no other declared input changed (tools/desk/cmd/muhar and changelog/muhar-resume-state.md are byte-identical). Admission: deskdispatch check-verifier rc=0 before any row, binding run=86315b6d3bad source=2aafcfe485c7 model=claude-opus-5-5 tier=strong (medici-finance/assay#7360).

Row 3 (class check, it needs git history) ran first, on the host, through statusgen verifyrun with --rows 3, with DELIVERY_HEAD = 2aafcfe485c74626db401b8209155f215015cc11 (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-21 | assay-verifier-app[bot] @ 2aafcfe485c7 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7360 run=86315b6d3bad66927c400e2b9bb4fb44049bd89e50f3b1457b67cdbda764eee3 source=2aafcfe485c74626db401b8209155f215015cc11 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Rows 1 and 2 (check:ci) ran last, inside one closed container-netns admission bracket: verify-runner v1.0.90 index digest sha256:4e8e3c80a075 (the tools/verify-runner image.lock pin, linux/arm64 manifest run), docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2. The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:43c474649b17 | 2026-11-21 | assay-verifier-app[bot] @ 2aafcfe485c7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:4c2b5729c0e9 | 2026-11-21 | assay-verifier-app[bot] @ 2aafcfe485c7 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:eafba232abf1813c3b52c59d03d137e994c98a5b2597311e9e05bf806f34d0d6
Container-Image: sha256:4e8e3c80a075c552de0966c886784c6020153c27e624f8a1ce988aa3b0590217
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7360 run=86315b6d3bad66927c400e2b9bb4fb44049bd89e50f3b1457b67cdbda764eee3 source=2aafcfe485c74626db401b8209155f215015cc11 brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=86315b6d3bad66927c400e2b9bb4fb44049bd89e50f3b1457b67cdbda764eee3 source=2aafcfe485c74626db401b8209155f215015cc11 tree=sha256:eafba232abf1813c3b52c59d03d137e994c98a5b2597311e9e05bf806f34d0d6 witness=sha256:3fe732d8dca4e027b472ac2cf2a481baeaa2abee70419c6808770d5a9b188521 opened=2026-11-21T16:57:20Z closed=2026-11-21T16:58:42Z before=receipt after=check-verifier

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0 and four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:43c474649b17; the and-chain requires every grep to match.
- Row 2 (check:ci +neighbour): expected exit 0 and the anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:4c2b5729c0e9.
- Row 3 (check +flow +dereference): expected exit 0 and consumers corroborated; observed pass exit=0 on the host, output sha256:6378c2f1c28a. Held for closure: the command is unchanged at this source and pins --base without --delivery. statusgen v1.0.90 lint reports Verify row 3 [consumer-delivery-range] for this brief as a NOTICE, and the rule (ruleConsumerRange) is in promotedRowRules (statusgen strength.go), so it becomes a PROBLEM on the closing transition. Same check-definition blocker as the 2026-11-16, 2026-11-18, 2026-11-19 and earlier 2026-11-21 runs (medici-finance/assay#6659, still open at this run).

statusgen verifyrun --check for this brief printed: 3 pass, 0 fail, 0 could-not-run/missing (of 3 Verify rows); 0 author-added with no acceptance credit. That tally counts row 3's witness; the closure count here is 2 of 3, because row 3 is held on #6659. statusgen verifyrun --bracket-digest over this brief file reported verdict=match for this run's bracket (witness sha256:3fe732d8dca4), exit 0.

Supplemental observation (no acceptance credit, outside every attempt): on the host, statusgen --root . --lint at this source exited 1 with three PROBLEMs, each naming another brief (desk-tools/37, desktools-v2/65, forge-neutral/54, all protected-table acceptance transitions that need --acceptance-forge); none names this brief. Its two NOTICEs for this brief are the row 3 consumer-delivery-range finding above and outcome-absent.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources; state.go is byte-identical between the delivering merge and this HEAD, and the docs/contracts.md change since the last run edits one S-context-handoff row with no muhar line). The trigger does not fire: irreversible is no and all risk fields are present and no. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; group/world-write mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse only for CAUGHT with LimitNS > 0; BuildNS >= 0, TestNS >= 0, Index >= 0) at tools/desk/cmd/muhar/state.go:167-169. All are reversible by an edit and a redeploy, because the state file is local and caller-owned.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — first version of a new on-disk contract, hashed into the header identity (state.go:323) and compared on resume (state.go:136), so a later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input, so a file other users can write could plant reusable CAUGHT verdicts; refusing the group and world write bits is exactly that exclusion, paired with the owner-only 0o600 create mode at state.go:94.

**VERIFY: BLOCKED** — check-definition: rows 1-2 passed in a matching closed bracket, row 3 met its Expect but is held on consumer-delivery-range (medici-finance/assay#6659); status stays implemented.

### Non-implementer verifier run — 2026-11-25 — claude-opus-5-5 (strong) as assay-verifier-app[bot]

Merged main 77ff0f2222a28df926c38ad30fea4a368173712e; delivering change medici-finance/assay#5917 (merge 1e55fa42a2f4, first parent 7d119c295ffd). statusgen and desk-tools v1.0.94. Re-run because declared inputs changed after the second 2026-11-21 record: docs/contracts.md gained register rows (S-loopadmin-process-facts, S-verify-syntax, R-temp-root-in-repo) and edited two others, none of them a muhar line, and tools/desk/internal/deskkit, which the muhar package compiles, changed. tools/desk/cmd/muhar and changelog/muhar-resume-state.md are byte-identical between the earlier record's source 2aafcfe485c7 and this HEAD. Admission: deskdispatch check-verifier rc=0 before any row, binding run=d82b89a72b9c source=77ff0f2222a2 model=claude-opus-5-5 tier=strong (medici-finance/assay#7679).

Row 3 (class check, it needs git history) ran first, on the host, through statusgen verifyrun with --rows 3, with DELIVERY_HEAD = 77ff0f2222a28df926c38ad30fea4a368173712e (the verified HEAD, which the row requires) and DELIVERY_BASE = 7d119c295ffd26e069c4fedee55171437ec9d2f3 (the first parent of the delivering merge 1e55fa42a2f4).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/56 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7679 run=d82b89a72b9c8f98b8f67329ccb429c45dd4b49b26c761cd2dd31551bb99d15f source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong

Rows 1 and 2 (check:ci) ran last, inside one closed container-netns admission bracket: verify-runner v1.0.94 index digest sha256:79e23ec185b1 (the tools/verify-runner image.lock pin, linux/arm64 manifest run, launched by digest only), docker --network none, read-only root and source mount, module cache downloaded by the open step and mounted read-only, GOPROXY=off, GOTOOLCHAIN=local, selected with --rows 1,2.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestState' -count=1 -v > "${TMPDIR:-/tmp}/dt56-state.log" 2>&1 && grep -F -- '--- PASS: TestStateResume ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateRejectsNewTree ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateSkipsNoErrors ' "${TMPDIR:-/tmp}/dt56-state.log" && grep -F -- '--- PASS: TestStateEndToEnd ' "${TMPDIR:-/tmp}/dt56-state.log"` | pass exit=0 sandbox=container-netns | sha256:c5fec71176ed | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt56-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt56-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:3afe0e503073 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:0fbdf881b52b282252f5dde3f303f7386f3a61b054f48b98e2fab9cf25b0fd49
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7679 run=d82b89a72b9c8f98b8f67329ccb429c45dd4b49b26c761cd2dd31551bb99d15f source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desk-tools/brief-56-make-mutation-reports-resumable-and-measurable.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d82b89a72b9c8f98b8f67329ccb429c45dd4b49b26c761cd2dd31551bb99d15f source=77ff0f2222a28df926c38ad30fea4a368173712e tree=sha256:0fbdf881b52b282252f5dde3f303f7386f3a61b054f48b98e2fab9cf25b0fd49 witness=sha256:90f9e757df5568e86c2b0acf9155b72d83e9b211ff42c5a56f6e5aa56044e662 opened=2026-11-25T04:06:26Z closed=2026-11-25T04:07:40Z before=receipt after=check-verifier

Per-row summary (these rows earn no extra credit; the witness tables above are the run records):

- Row 1 (check:ci +mutation +flow): expected exit 0 and four named PASS lines; observed pass exit=0 sandbox=container-netns, output sha256:c5fec71176ed; the and-chain requires every grep to match.
- Row 2 (check:ci +neighbour): expected exit 0 and the anchors PASS line; observed pass exit=0 sandbox=container-netns, output sha256:3afe0e503073.
- Row 3 (check +flow +dereference): expected exit 0 and consumers corroborated; observed pass exit=0 on the host, output sha256:6378c2f1c28a. Held for closure: the command is unchanged at this source and pins --base without --delivery. statusgen v1.0.94 lint still reports Verify row 3 [consumer-delivery-range] for this brief as a NOTICE, which becomes a PROBLEM on the closing transition. Same check-definition blocker as every earlier run (medici-finance/assay#6659, read open at this run).

The close step re-admitted the run, and bracket-digest reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1 (witness sha256:90f9e757df55).

Supplemental observations (no acceptance credit, outside every attempt). In the same image under the same network-none flags, after the bracket closed, go test ./cmd/muhar -run '^(TestState|TestSpecAnchorsLand$)' -count=1 -v in tools/desk exited 0 with zero SKIP lines; it listed PASS for TestStateResume (identity-interrupt), TestStateRejectsNewTree (source, tests, select, shard-change, shard-remove, runner-build, command-words, descriptor-type, descriptor-error, unsafe-state, Go-inputs with goenv-flags and external-flags), TestStateSkipsNoErrors (external-signal, fault-signals, restore-error), TestStateEndToEnd (shard-indices, selection-timing, schemata-timing) and TestSpecAnchorsLand. On the host, statusgen --root . --lint at this source exited 1; its two NOTICEs for this brief are the row 3 consumer-delivery-range finding above and outcome-absent, and no PROBLEM names this brief.

Fail-first: not required. Every risk answer in the frontmatter is "no", and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row is risk-bearing.

Risk-bearing value enumeration (delivering diff 7d119c295ffd..1e55fa42a2f4, muhar sources; state.go is byte-identical between the delivering merge and this HEAD, the muhar package is byte-identical to the previous record's source, and the docs/contracts.md changes since then add or edit rows with no muhar line). The trigger does not fire: irreversible is no and all risk fields are present and no. Enumerated anyway: stateSchema = 1 at tools/desk/cmd/muhar/state.go:25; group/world-write mask 0o022 at tools/desk/cmd/muhar/state.go:81; create mode 0o600 at tools/desk/cmd/muhar/state.go:94; definitive-record bounds (test-timeout reuse only for CAUGHT with LimitNS > 0; BuildNS >= 0, TestNS >= 0, Index >= 0) at tools/desk/cmd/muhar/state.go:167-169. All are reversible by an edit and a redeploy, because the state file is local and caller-owned.
RISK-VALUE: DERIVED — stateSchema = 1 @ tools/desk/cmd/muhar/state.go:25 — first version of a new on-disk contract, hashed into the header identity (state.go:323) and compared on resume (state.go:136), so a later schema change refuses an older file rather than misreading it.
RISK-VALUE: DERIVED — group/world-writable mask = 0o022 @ tools/desk/cmd/muhar/state.go:81 — the state file is trusted verdict input, so a file other users can write could plant reusable CAUGHT verdicts; refusing the group and world write bits is exactly that exclusion, paired with the owner-only 0o600 create mode at state.go:94.

**VERIFY: BLOCKED** — check-definition: rows 1-2 passed in a matching closed bracket, row 3 met its Expect but is held on consumer-delivery-range (medici-finance/assay#6659); status stays implemented.

## Failure map
- A changed tree reuses an old verdict: TestStateRejectsNewTree fails.
- An interrupted or failed run becomes a completed result: TestStateSkipsNoErrors fails.
- Resume changes verdict order or content: TestStateEndToEnd's uninterrupted comparison fails.
- Two callers race one state file: the concurrent-writer case in row 1's `TestStateEndToEnd` refuses — the lock holder writes, the second writer refuses without writing.
- A changed run mode reuses another mode's verdicts: `TestStateRejectsNewTree`'s `-select`/`-shard` case fails.
- The state option is documented without its delivery: row 3 fails.

## Review
Gate: model. Focus on stale-result refusal, crash consistency and exact report equivalence, not JSON shape alone.
