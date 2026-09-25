---
brief: assay:assay:graph-execution:03
title: Evidence coverage rule and the observe evidence kind
why: >-
  A brief can be marked verified while a mandatory check was never run, errored, or ran
  against a different revision of the work: the tools record what happened but no rule says
  what MUST have happened before the next step is allowed. A deterministic coverage rule
  turns "the evidence looks fine" into "every mandatory claim has a passing result at this
  exact revision, or nothing downstream moves" — and adds the one evidence kind the fleet
  lacks, a signal watched over a window after a change lands.
wave: 1
depends: ["graph-execution/02"]
unblocks: ["graph-execution/05"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/admission-assurance-spec.md — 2026-09-18 integration amendment"
  - "freshness-checked 2026-09-18 @ 951ca784d100a7d201a28a34033da6709ec2ec8f"
  - "docs/streams/graph-execution/spec.md §2.3 (verification governs transitions; the coverage rule; the observe kind; integration check at the join) and §3 (evidenceactor advisory, autoflip does not re-check the stamp)"
  - "graph-execution/02 — the node contract's `evidence: [{kind, claim, mandatory}]` and the `join:` node this rule reads"
  - "statusgen/verifyrun.go (the execution witness: pass / fail / could-not-run, tree SHA recorded per row, treeSHALen=12); statusgen/lifecycle.go (WitnessInfo.Version stale-witness demotion); statusgen/witnessgate.go"
  - "statusgen/evidenceactor.go (Evidence attribution by git blame — a NOTICE, not a gate); statusgen/autoflip.go (header: the flip does NOT re-check the `verified` stamp it promotes)"
  - "docs/three-state-instrument-rule.md (could-not-check is never pass)"
  - "Baz — planner/verifier split https://youtu.be/aWrGSM5vVyc?t=566 ; per-requirement dispatch https://youtu.be/aWrGSM5vVyc?t=706 ; pre-change grounding https://youtu.be/aWrGSM5vVyc?t=872"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn -- '--coverage\\|wrong-revision\\|observe:' statusgen/*.go` returns no hits; verifyrun records a tree SHA per witness row but nothing compares it to the brief's merged revision as a release condition — not already satisfied"
exec-tier: strong
exec-tier-why: "(b) the rule joins four artifacts — Verify rows, verifyrun witnesses, reviewer approvals and the pattern's evidence declarations — and a mismatch in revision semantics between any two lets a stale pass release work; (c) an aggregation that treats could-not-check as pass is the exact fault and passes every happy-path test"
domain: complicated
consumers:
  - "statusgen/autoflip.go (the verified→done flip gains a coverage precondition): fixed-here"
  - "statusgen/lifecycle.go (the `verified` witness precedence reads coverage, not only WitnessInfo.Passed): fixed-here"
  - "spec/lifecycle-v1.md (the verified state gains the coverage condition): fixed-here"
  - "spec/workflow-pattern-v1.md (the `observe` kind's fields): fixed-here"
  - ".github/workflows/verify-gate-open.yml and verify-gate-close.yml (the human sign-off pair): out-of-scope (unchanged — a human sign-off is a `decision` node; coverage governs the model lane's transitions and reports for the human lane)"
  - "statusgen/evidenceactor.go (attribution stays advisory): out-of-scope (this brief adds no identity gate; attribution and coverage are different questions and stay separate rules)"
version: 2
id: d94aa822-65e3-44ad-ba9a-479ddcb47fdc
---

# Brief 03 — Evidence coverage rule and the observe evidence kind

## Context
files: `statusgen/coverage.go` (planned), `statusgen/coverage_test.go` (planned), `statusgen/testdata/coverage/` (planned), `statusgen/autoflip.go`, `statusgen/lifecycle.go`, `statusgen/main.go` (the `--coverage` flag), `spec/lifecycle-v1.md`, `spec/workflow-pattern-v1.md` (planned) — graph-execution/02 creates it, `schemas/workflow-pattern-v1.json` (planned) — graph-execution/02 creates it, `docs/lifecycle.md`, `docs/enforcement-model.md`, `changelog/graph-execution-03-coverage.md` (planned)
facts:
- `statusgen verifyrun --brief <path>` writes an Evidence row per Verify row with a three-state Result (`pass` / `fail` / `could-not-run`) and the tree it ran against (`Runner` cell, 12-char SHA, `+dirty` when modified) — `statusgen/verifyrun.go`. `--check` reports per row whether a witness exists, still describes the row, and passed. Nothing compares the witness SHA to the brief's merged revision as a condition for anything. (Read 2026-09-16 @ d96fd3ba.)
- `statusgen/lifecycle.go` folds witnesses by precedence: `WitnessInfo{Passed, Version}` is the `verified` witness and a Version mismatch against the brief demotes it (stale-Verify). `ApprovalInfo{Approved, AtHead}` is the `done` witness for `gate: model`.
- `statusgen/autoflip.go` (header comment): the verified→done flip's precondition is exactly two facts — README row at `verified`, and an APPROVED reviewer-App review at the merged head; it does NOT re-check the `verified` stamp it promotes. `statusgen/evidenceactor.go` attributes Evidence lines by `git blame` and emits NOTICEs.
- Vocabulary (from graph-execution/02 and the README): evidence kinds `command | review | witness | observe`; results `pass | fail | missing | error | could-not-check | wrong-revision`. Mapping from today's witnesses: verifyrun `pass`→`pass`, `fail`→`fail`, `could-not-run`→`could-not-check`, no witness→`missing`, witness at a different SHA than the item's revision→`wrong-revision`, witness present but unparseable→`error`.
- **Decision recorded here:** `autoflip.go` DOES gain a coverage precondition — the flip refuses (reported, never silent) when coverage for the brief is not `released`. This is the one place the stamp it promotes is re-checked; the header comment's trade is thereby closed for the model lane. The human lane (`verify-gate-*.yml`) is not changed by this brief.
- Single-point-of-failure note: the ONE control is the coverage verdict. Second layer, independent: verifyrun's `could-not-run` contains `not-run`, which `unrunMarkerRe` reads at the board level, so a row nobody ran stays UNRUN on the board even if coverage were miscomputed (a different reader, a different signal). Third: the reviewer App's approval-at-head in `autoflip.go` remains a separate precondition; coverage adds to it, it does not replace it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: `example-org/*` placeholders; fixtures name no adopter.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Integration amendment — 2026-09-18

Retain this brief as the sole coverage implementation. Applicable evidence must bind the exact subject and acceptance-definition digest; a model assessment is never an execution witness. Treat unavailable identity/definition corroboration as could-not-check. The later instance contract (09) supplies bindings through an adapter; this core rule remains testable without a provider. Control-profile population/period export belongs to 15, not this brief. Keep protected acceptance/executor identity as independent controls; coverage alone does not establish them.

## Task
1. **The rule** (`statusgen/coverage.go` (planned)): `func evaluateCoverage(root string, streams []*Stream, patterns map[string]Pattern) map[string]Coverage`. For each brief, the set of mandatory claims is the union of (a) its Verify rows and (b) the `mandatory: true` evidence entries of the pattern node the brief is at (when a pattern applies; a brief with no pattern uses (a) alone). Each claim resolves to `{claim, kind, result, revision, released: bool, reason}`. The brief is `released` only when every mandatory claim is `pass` at the item's revision — the merged SHA for a merged brief, the PR head for an open one. Any `missing | error | could-not-check | wrong-revision` holds with the reason naming the claim. `fail` holds and is reported distinctly from could-not-check.
2. **The join's integration check.** When the pattern's `join:` node applies, coverage requires at least one Verify row classed `+flow` (rowclass.go's obligation token); absent → a `missing` claim `integration-check` — individually passing rows do not release the join.
3. **The `observe` kind.** Extend `spec/workflow-pattern-v1.md` (planned) and `schemas/workflow-pattern-v1.json` (planned) with `evidence: [{kind: observe, signal, band, window, source, mandatory}]`. Coverage treats it as a claim filled by the verifier from the named `source`; an unreadable source → `could-not-check` → held. The schema doc states: declare `observe` only where a deploy exists; where none exists the kind is **omitted**, never recorded as "n/a". The two patterns 02 ships carry no `observe` entry; the doc shows the shape under an `example-org` deploy.
4. **Consumers.** `autoflip.go`: before flipping, call coverage; not `released` → refuse with the reasons in the result (dry-run prints them). `lifecycle.go`: the `verified` witness precedence reads `released` in addition to `WitnessInfo.Passed`, so a stale or wrong-revision witness demotes exactly as a version mismatch does today.
5. **CLI.** `statusgen --coverage [--json] --root <root>`: one line per brief `<id> released|held <n-claims> <first-reason>`; `--json` emits every claim. Exit 0; exit 2 on an unreadable tree.
6. **Docs.** `spec/lifecycle-v1.md`: `verified` gains the coverage condition; `docs/lifecycle.md` and `docs/enforcement-model.md` gain the rule and the row. `changelog/graph-execution-03-coverage.md` (planned).

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go test -run TestCoverage ./...` | exit 0 |
| 2 | check:ci +mutation | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | exit 0; a fixture whose witness SHA differs from the brief's merged revision is `wrong-revision` and `released: false` |
| 3 | check:ci +mutation | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | exit 0; a `could-not-run` witness yields `could-not-check` and holds — never `pass` |
| 4 | check:ci +mutation | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | exit 0; a `verified` row with an approval at head but a `missing` mandatory claim is NOT flipped, and the dry-run output names the claim |
| 5 | check:ci +flow | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | exit 0; a brief at the pattern's join with only site rows is held on `integration-check`; adding one `+flow` row releases it |
| 6 | check | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | the printed set is a subset of `['could-not-check', 'error', 'fail', 'missing', 'pass', 'wrong-revision']` |
| 7 | check +dereference | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | each count >= 1 |
| 8 | check:ci +neighbour | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | exit 0 — the no-override pin and the advisory attribution check are untouched |
| 9 | check | `statusgen --root . --lint; echo rc=$?` | `rc=0` |
| 10 | check | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | `rc=0` on the authoring branch (every entry routes follow-up or out-of-scope); exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass |
| 11 | check:ci +mutation | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | exit 0; named test PASS; high-confidence advice does not satisfy missing mandatory evidence |
| 12 | check:ci +mutation | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | exit 0; named test PASS; changed acceptance invalidates the affected witness |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestCoverage ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test -run TestCoverageWrongRevisionHolds ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestCoverageCouldNotCheckIsNotPass ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestAutoFlipRefusesUnreleasedCoverage ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && go test -run TestCoverageJoinRequiresIntegrationRow ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --coverage --json --root . > /tmp/ge03.json; python3 -c 'import json;d=json.load(open("/tmp/ge03.json"));print(sorted({c["result"] for b in d for c in b["claims"]}))'` | pass exit=0 | sha256:0d5489d2325b | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'observe' spec/workflow-pattern-v1.md schemas/workflow-pattern-v1.json spec/lifecycle-v1.md` | pass exit=0 | sha256:38af6536faaa | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestAutoFlipNoOverride ./... && go test -run TestEvidenceActor ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:70dd3e1cc640 | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/03 --root .; echo rc=$?` | fail exit=0 | sha256:d7e294d75f26 | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd statusgen && go test -count=1 -v -run TestCoverageAdviceCannotSupplyWitness ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |
| 12 | `cd statusgen && go test -count=1 -v -run TestCoverageAcceptanceDigestChanged ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-25 | assay-worker-app[bot] @ 8b88e97f1252 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-09-25, round 3** (the third CHANGES_REQUESTED review at `5980c2acb`: F2 round 3,
F6 / security S5, A7): `statusgen verifyrun --brief` re-executed every row at `8b88e97f1252`
(a CLEAN tree — the round-3 `coverage.go`, its tests, the spec paragraph and the changelog
committed at `83280a436`; then main merged, bringing in #1691, and the `af/brief-08`/`-09`
autoflip fixtures rewritten to the witnessed Evidence form per the merge-order note), using
the `statusgen` binary built from that same commit. The table above
replaces the round-2 set recorded at `e2c2b350c87c`: those witnesses read `wrong-revision`
by design, since `statusgen/coverage.go` and `spec/lifecycle-v1.md` changed after them.
Rows 1–5, 8, 11, 12 again record `could-not-run` (no `unshare --net` on this darwin host);
`cd statusgen && go build ./... && go vet ./... && go test ./... -count=1` was also run
directly on the identical tree and passed — corroborating detail, not a substitute witness.

**Re-run 2026-09-25, round 2** (the second CHANGES_REQUESTED review at `e43403dfd`: F2, F3,
security S1–S3): `statusgen verifyrun --brief` re-executed every row at `e2c2b350c87c` (a
CLEAN tree — the reworked `coverage.go`, its tests, and the spec paragraph committed first,
so the witnesses carry no `+dirty` suffix), using the `statusgen` binary built from that
same commit. The table above replaces the round-1 set recorded at `8737498f7fa5+dirty` —
those witnesses read `wrong-revision` under the round-2 rule by design, since declared
`files:` paths (`statusgen/coverage.go`, `spec/lifecycle-v1.md`) changed after them. Rows
1–5, 8, 11, 12 still record `could-not-run` — this darwin host lacks `unshare --net` for
`check:ci`'s hermetic network-off re-run — but `cd statusgen && go build ./... && go vet
./... && go test ./... -count=1` was ALSO run directly at `e2c2b350c87c` (no hermetic
wrapper; no test in the package uses the network) and passed, which includes every test
those rows name — corroborating detail, not a substitute witness; a Linux `check:ci`
runner still owes the mechanical rows.

**Fail-first, round 1** (findings F1/F2/F3 at `6c0403006`; tests not named in the frozen
Verify table, exercised by `go test ./...`): `TestCoverageNoTargetRevisionIsCouldNotCheck`,
`TestCoverageMissingTreeTokenIsCouldNotCheck`, `TestCoverageShortTreeTokenIsCouldNotCheck`,
`TestCoverageAncestorWitnessReleases`, `TestCoverageAncestorWitnessOtherPathChangedMismatch`
and `TestCoverageExpectChangedSinceWitnessRan` were RED against the ORIGINAL `coverage.go`
(`6c0403006ca3521b74a6421fba2b64c77cb2bbd3`) and GREEN after the round-1 rework.

**Fail-first, round 2** (F2, F3, security S1/S2 at `e43403dfd`): the five new tests below were
run against `e43403dfd`'s `coverage.go` (new tests in place, no other file touched) and were
RED, each reproducing one of the review's probes, then GREEN at `e2c2b350c87c`:

| Test | Probe | RED at `e43403dfd` |
|------|-------|--------------------|
| `TestCoverageDirtyWitnessExpectChangedIsError` | PROBE-F / S1 | `released: true`, `Result: pass` for a `+dirty` witness whose Expect was tightened |
| `TestCoverageBriefAbsentAtWitnessTreeIsCouldNotCheck` | PROBE-G / S2 | `released: true`, `Result: pass` with the brief absent at the witness tree |
| `TestCoverageRowAbsentAtWitnessTreeIsCouldNotCheck` | F3(b) | `released: true` with row 2 absent at the witness tree |
| `TestCoverageSiblingEvidenceAndStatusRegenStillRelease` | PROBE-D/E | `wrong-revision` after sibling Evidence and a `STATUS.md` regen |
| `TestCoverageDeclaredFilesScopeTheWitness` | F2 | `wrong-revision` for a change to an UNDECLARED path |

**Mutation proof** (the four pre-existing `+mutation` rows 2, 3, 11, 12, and the round-2
guards). Each mutation was applied to a scratch COPY of `statusgen/` (never the worktree),
the named test was run, and the copy was restored. Every mutant was killed (RED):

| Row / guard | Mutation (in `coverage.go`) | Test | Result |
|---|---|---|---|
| row 2 | `classifyRevision`'s final `return revisionMismatch` → `revisionMatch` | `TestCoverageWrongRevisionHolds` | RED: want `wrong-revision`, got `could-not-check` |
| row 3 | the could-not-run branch returns `covPass` | `TestCoverageCouldNotCheckIsNotPass` | RED: `released: true` |
| row 11 | `if isWitnessRow(er.Text)` → `if true` | `TestCoverageAdviceCannotSupplyWitness` | RED: want `missing`, got `error` |
| row 12 | the Command guard → `if false` | `TestCoverageAcceptanceDigestChanged` | RED: want `error`, got `could-not-check` |
| F3(a) | `base := witnessBaseRevision(wrev)` → `base := wrev` | `TestCoverageDirtyWitnessExpectChangedIsError` | RED |
| F3(b) | an unreadable historical row returns `covPass` | `…BriefAbsentAtWitnessTree…`, `…RowAbsentAtWitnessTree…` | RED (both) |
| F2 bookkeeping | `isBoardBookkeepingPath` → `false` | `TestCoverageSiblingEvidenceAndStatusRegenStillRelease` | RED |
| F2 declared | the declared-scope branch disabled | `TestCoverageDeclaredFilesScopeTheWitness` | RED |
| F2 directory | a declared directory no longer covers paths under it | `TestCoverageDeclaredFilesScopeTheWitness` | RED |
| F2 other path | the scope check in `ancestorNoOtherChanges` disabled | `TestCoverageAncestorWitnessOtherPathChangedMismatch` | RED: `released: true` |

(The round-1 note that this mutation proof was blocked by a tool-use classifier is superseded:
it ran this round.)

**Fail-first, round 3** (F2 round 3, F6 / security S5 at `5980c2acb`): the new tests were run
against `5980c2acb`'s `coverage.go` (new test file in place, no other file touched). These 14
subtests were RED there, each reproducing one of the reviewers' probes, and are GREEN at
`83280a43620f`:

| Test / subtest | Probe | RED at `5980c2acb` |
|------|-------|--------------------|
| `TestCoverageDeclaredScopeExemptsVerifyBookkeeping/R1…`, `/R2…`, `/whole_docs/streams…` | F2 R1, R2 | `wrong-revision` after the brief's own README row flip, a `STATUS.md` regen, sibling Evidence |
| `TestCoverageWitnessScopeNeverFailsOpen/S5a…`, `/S5a'…`, `/S5b…` | S5a, S5a', S5b | `pass` after a declared file was renamed / moved into `docs/streams/` |
| `TestCoverageWitnessScopeNeverFailsOpen/S5c…`, `/S5d…`, `/S5e…`, `/X1…`, `/X2…`, `/X3…` | S5c–e, X1, X2 | `pass` with a brace, `.`, `n/a`, bare-sibling or `**` declaration |
| `TestCoverageWitnessScopeNeverFailsOpen/S5f…`, `/S5f'…` | S5f | `pass` after `files:` was narrowed / added in the Evidence commit |

The two "still holds" subtests of `TestCoverageDeclaredScopeExemptsVerifyBookkeeping`,
`TestCoverageResolvableDeclarationsStillScope` and `TestCoverageBriefOwnFileNeverInvalidates`
(A7) were already GREEN at `5980c2acb`. They pin behaviour that must not regress, and the
mutations below prove each one is load-bearing. `TestCoverageDeclaredFilesScopeTheWitness`'s
fixture now writes the file and the directory its `files:` line declares, because an entry that
names nothing now selects the conservative scope.

**Mutation proof, round 3.** Each mutation was applied to `coverage.go`, `go test -run
TestCoverage` was run, and the file was restored byte-for-byte. Every mutant was killed:

| Guard | Mutation (in `coverage.go`) | Killed by |
|---|---|---|
| F6 renames | drop `--no-renames` from the diff | `…NeverFailsOpen/S5a`, `/S5a'`, `/S5b` |
| F6 base declaration | skip the `files:` read at the witness's base commit | `…NeverFailsOpen/S5f`, `/S5f'` |
| F6 unresolved entry | an entry that resolves to nothing no longer widens | `…NeverFailsOpen/S5c`, `/S5d`, `/S5e`, `/X1` |
| F6 unsupported syntax | `declaredEntrySupported` check off | `…NeverFailsOpen/X3` |
| trailing `/**` | the `/**` → directory rewrite removed | `…ResolvableDeclarationsStillScope/trailing_/**…` |
| F2 exemption | the `isVerifyWrittenPath` exemption off | `…ExemptsVerifyBookkeeping/R1`, `/R2`, `/whole_docs/streams…` |
| F2 over-exemption | every `docs/streams/` path exempt in the declared branch | `…ExemptsVerifyBookkeeping/…artifact_changed_still_holds` (both) |
| A7 | the brief-own-file exemption removed (`p == sc.briefRel`) | `TestCoverageBriefOwnFileNeverInvalidates` (both subtests) |

**Task item 1's revision wording.** Task item 1 says the item's revision is "the merged SHA
for a merged brief, the PR head for an open one". The implementation resolves the item's
revision offline as the checked-out `HEAD` — the PR head on the branch, but the main TIP
(not the merge SHA) on the model-lane flip job — and credits a witness at an ancestor when
no path it speaks for changed since (the brief's `files:` now and at the witness's commit,
else — or when a declared entry names no real path — everything outside `docs/streams/**`
and `STATUS.md`; never the files verify and regeneration write; renames count against their
old path). That deviation from the Task's literal wording is
deliberate (the literal form needs a live merge lookup the ground rules forbid, and the
round-1 whole-tree rule halted the model lane); `spec/lifecycle-v1.md` and the
`coverage.go` header state the implemented rule. The `+dirty` tolerance is a declared
residual (header "THE +dirty TOLERANCE"), pinned by `TestCoverageDirtyWitnessToleranceIsDeclared`.

**Row 10** mechanically records `fail exit=0`, but the row's own Expect cell states TWO
acceptable outcomes (`rc=0` on the authoring branch; `rc=2` on a fully merged main) —
`parseExpect`'s exit-code reader lifts the FIRST unquoted digit after `exit`, which here
is the "expected 2" fragment naming the merged-main case, not the authoring-branch case
this run is actually in. The command's REAL exit code on this authoring branch is `0`
(confirmed directly above, and by hand: `statusgen --consumers --brief graph-execution/03
--root .` prints `rc=0` with every consumers: entry CORROBORATED or UNCHECKED, none
DISPROVED). This is a parseExpect limitation on a two-case Expect cell, not a defect in
`--consumers` or in coverage.go; recorded here rather than silently edited into a false
`pass`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question: name one input state in which coverage reports `released` while a
mandatory claim has no passing result at the item's revision. If one exists, the rule is
not deterministic and the brief is bounced.
