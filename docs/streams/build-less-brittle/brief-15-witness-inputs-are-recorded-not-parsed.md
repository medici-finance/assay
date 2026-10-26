---
brief: assay:assay:build-less-brittle:15
title: Witness inputs are recorded, not parsed
why: >-
  A verified brief reaches done only when its Verify witnesses still speak for the current tree.
  Today a witness speaks for almost every file in the repository, so an unrelated merge (a plugin
  index, a surfaces list) holds every gate:model brief at verified, and nothing auto-flips. The
  earlier fix guessed each row's inputs from its command text, which covers a few shell readers and
  none of the go, git or statusgen rows. Recording what a row really read, and comparing only that,
  releases a witness exactly when its real inputs are unchanged.
wave: 0
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  It narrows a verification control. Today a witness is judged against every path outside the
  board's bookkeeping; after this brief it is judged only against the paths its run recorded. An
  under-recorded read set lets a stale witness pass, which is the failure the coverage gate exists
  to prevent. The driver confirms the narrowing, the floor behind it, and the one-time re-witness.
decision-trigger: creation
issues: [2026]
regression-of: medici-finance/assay#2304
schema: brief-v2
version: 1
authored: 2026-10-26 by a worker-desk authoring session (design proposal; the driver rules on it before implementation is dispatched)
sources:
  - "medici-finance/assay#2026 (the mechanism's issue; its strike-two design note names options a-c)"
  - "medici-finance/assay#3105 (source issue: gate:model briefs held at verified)"
  - "medici-finance/assay#2018 (no gate or board line reports that zero briefs are released)"
  - "medici-finance/assay#2304 (prior fix: command-text input parser and witness landing check)"
  - "docs/contracts.md S-evidence-coverage"
  - "freshness-checked 2026-10-26 @ 5ddf5a317 (origin/main)"
exec-tier: strong
exec-tier-why: "(a) the recording mechanism and its completeness rule are not pre-specified; (b) correctness spans verifyrun, the coverage library and three callers; (c) an error here lets a stale witness release a brief to done; (d) raised by an error-class trigger (strike two on the mechanism's issue)."
domain: complicated
outcome: none
consumers:
  - "statusgen/verifyrun.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/coverage.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/autoflip.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/reconcilefold.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/evaluation/coverage.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/evaluation/coverage_api.go: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/contracts.md: follow-up build-less-brittle/15 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/evaluation/coverage_inputs.go: out-of-scope (the command-text parser stays until the measurement in Task step 9 shows it releases nothing on its own; its retirement is a later brief)"
---

# Brief 15 — Witness inputs are recorded, not parsed

Related: medici-finance/assay#2026 (this brief's issue), medici-finance/assay#3105,
medici-finance/assay#2018.

This is a design proposal. The coordinator relayed a go to author it; the scope below is a
recommendation, not a ruling. The driver rules on the brief before implementation is dispatched.

## Context
files:
- `statusgen/verifyrun.go` — records each row's read set beside the witness (new recorder, Linux only)
- `statusgen/coverage.go` — the adapter: one read-set source (`coverageOptionsFor`) and the go compile-input floor, both handed to the library as observations
- `statusgen/autoflip.go`, `statusgen/reconcilefold.go` — call `coverageOptionsFor` instead of passing `coverageOptions{}`
- `statusgen/evaluation/coverage.go`, `statusgen/evaluation/coverage_api.go` — judge a witness against its recorded read set
- `docs/contracts.md` — the S-evidence-coverage row names the new independent layer
- tests: `statusgen/evaluation/readset_test.go` (planned), `statusgen/readset_test.go` (planned)

facts:
- Current shape. With no dependency manifest, a witness speaks for every path outside
  `docs/streams/**` and `STATUS.md` (`witnessScope.describe`, `statusgen/evaluation/coverage.go`).
  All three callers pass an empty manifest: `statusgen/autoflip.go:1068` and
  `statusgen/reconcilefold.go:138` pass `coverageOptions{}`, and `--coverage`
  (`statusgen/coverage.go:323`) passes `coverageOptions{Patterns: patterns}`. The
  `Dependencies` field exists and nothing fills it.
- Prior fix (medici-finance/assay#2304). `deriveRowInputs` (`statusgen/evaluation/coverage_inputs.go`)
  reads the command text with a closed shell grammar. A row it cannot parse keeps the whole-tree
  scope. Its header states why `go` is outside the grammar: the modules most rows test read
  outside themselves.
- Before measurement 1, relayed by the coordinator, not re-run by the author: the model
  auto-flip job of 2026-10-26 14:59Z refused 35 briefs at item revision 9ab9fa76ab4d.
- Before measurement 2, taken by the author: on 5ddf5a317 (main, 2026-10-26), a statusgen built
  from that tree (`cd statusgen && go build -o <tmp>/sg .`) ran
  `<tmp>/sg --root . --auto-flip-model --dry-run`. Exit 0, 36 briefs
  `REFUSED (stays verified) — coverage is not released`, 0 flips, 301 refused Verify rows.
  - First differing path the witness spoke for: `.claude-plugin/marketplace.json` on 186 rows,
    `.assay-surfaces` on 101, `.gitattributes` on 7, any other path on at most 1 row each.
  - Why the parser kept the whole-tree scope: 269 rows on 36 briefs it cannot parse. Of these,
    164 have a shell metacharacter, 76 run a command outside the set (`go` 53, `git` 10,
    `statusgen` 6, other 7), 21 have a double-quote expansion, 7 an assignment, and 1 a
    `grep -r`. A further 59 rows on 7 briefs have `+dirty` witnesses, and 4 rows on 4 briefs
    were edited after their witness ran.
  - A `--coverage --json` pass over all 568 briefs at the same revision released 0 briefs.
    42 claims pass only because the parser derived their inputs. 133 briefs are fully witnessed
    and held only by `wrong-revision`.
- The desk's proposed derived set for go rows (`cd <module> && go test|build|vet`, inputs =
  the module's files plus `go list -deps -test`) does not release enough to justify itself, and
  it is unsound:
  - Yield. 25 of the 301 refused rows have that clean shape. Every one of the 36 briefs has some
    other row shape or a `+dirty` or edited witness. So 0 of 36 would release.
  - Soundness. build-less-brittle/10 row 1 (`cd tools/desk && go test ./internal/arch/`) runs
    `tools/desk/internal/arch/arch_test.go`. That test opens `docs/contracts.md` and stats
    `statusgen/evaluation` at the repository root. Neither path is in the `tools/desk` module or
    its `go list -deps -test` graph. In `tools/desk`, 117 test lines climb three directories
    with `../`, and 6 test files ask git for the toplevel. A derived set misses these reads,
    which is an under-derivation: a stale witness passes.
- What `go list -deps -test` can and cannot establish:
  - It CAN list each in-graph package's Go, cgo, C/assembly, ignored, test and `//go:embed` files.
  - It CANNOT see:
    - files a test opens at run time: fixtures, `testdata/` that is not embedded, `../` climbs,
      the repository root;
    - headers a cgo `#include` pulls from outside the package directory;
    - files whose generator `go test` does not run;
    - a `go.work` or `GOFLAGS` the row's environment supplies;
    - the module cache and the toolchain (environment, outside the tree);
    - anything a test reads by running `git`.
  - Build tags change which files compile, but go/build still reads every Go file's header, so
    the listed files are read whatever the tags.
  - Conclusion: the list is a sound LOWER bound on what a go row reads and an unsound upper bound.
    This brief uses it only as the floor below.
- History reads. A row that runs `git` (42 refused rows by command text), `statusgen` (86, most
  of which read trailers), or a go test that asks git for the toplevel reads `.git`. The witness
  revision's own `HEAD` differs from the item revision's, so a history-reading row can be judged
  only at its exact revision. 28 of the 36 refused briefs have at least one such row by command
  text. Count go tests that shell out and the number is higher. A recorded read set does not
  release those briefs. They keep today's route.
- single-point-of-failure: the recorder's completeness. Coverage releases a witness for any path
  outside the read set verifyrun recorded, so a read the recorder misses is a stale pass. Two
  layers stand behind it:
  - The go compile-input floor. For a go row, the statusgen adapter computes `go list -deps -test`
    at the witness revision, and any in-tree file it lists that the recorded set lacks is refused.
    That is a static dependency graph computed by a different component at evaluation time, not
    a run-time trace.
  - Fail-closed recording. The recorder writes no read set for a read it cannot attribute:
    `io_uring`, a connection to a local socket or daemon, or a host that is not Linux.
  For a non-go row the floor is NONE: no static derivation exists for an arbitrary command. The
  recorder's fail-closed rule is the only guard there, which is why that rule is strict.

design-fit:
  owner: statusgen coverage library (`statusgen/evaluation/coverage_api.go`, `EvaluateCoverage`), fed by the statusgen adapter (`statusgen/coverage.go`)
  contract: S-evidence-coverage
  retires: []   # the command-text parser and its .gitattributes-ancestor rule retire in a later brief, gated on Task step 9's measurement
  weight: verbs 0, flags 0, refusals +3 (unreadable read set, floor not covered, history read at a different revision), rule-text lines +0 (no skill or kit text changes)
  why-add: each refusal is the fail-closed branch of a narrowing that does not exist today; the alternative is the parser, which narrows only for shell readers and is retired once this measures it redundant

## Human decision
Should a verification witness be judged only against the files its run actually read? Today any
later change anywhere in the repository makes every earlier witness stale, so no verified item
reaches done without being run again. The proposal narrows that check, and narrowing a
verification control is a human call.

Options:
1. **Recorded read set with a static floor (recommended).**
   - The verify runner records every file each row reads (Linux only). An item stays released
     while none of those files changed.
   - For go test rows, the evaluator also requires the record to include every file the go
     toolchain lists as compiled. A recorder that missed one is caught by a different signal.
   - Anything the recorder cannot attribute keeps today's whole-tree check. So does any row
     that reads repository history; those items stay on today's manual route.
   - The verify desk re-runs about 133 already-witnessed items once, at one main revision.
   - The command-text guesser stays until a measurement shows it adds nothing.
2. **The same, plus a derived input set for existing go rows with no re-run.** This releases
   none of the 36 items held today: each has other row shapes or edited or dirty witnesses. It
   also misses files that tests open at run time, so a stale result can pass.
3. **No new mechanism.** The verify desk re-runs the rows at the head of main before each flip.
   The check is not narrowed; every unrelated merge still costs a re-run.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not run the batch re-witness (Task step 8 names it; the verify desk runs it after merge).
  Do not edit any brief's Verify table, Evidence or witness rows.
- Do not remove or weaken `deriveRowInputs`, the command-text guard, the Expect guard, the
  landing check, or the `+dirty` / `+unknown` conservative scope.
- No flag, environment variable or override may switch the floor or the fail-closed recording off.
- Pure verification: every new check is a closed-form comparison over recorded sets and trees.
  Add no runtime assertions to the code under test.

## Task
1. **Freshness.** On fresh `origin/main` re-read the files in Context and record the commit
   read. If a merged change already fills `Dependencies` from any source, stop and report
   NEEDS_CONTEXT.
2. **Recorder (verifyrun, Linux).** Run each row under a tracer that follows every child
   process (ptrace-class). `LD_PRELOAD` is not acceptable: Go binaries make raw system calls.
   Record:
   - every in-tree path opened for reading or mapped;
   - every directory listed (the names of its direct children are an input);
   - every in-tree path looked up and found absent (its continued absence is an input);
   - every path stat-ed (its existence and mode are an input).
   Collapse a directory to one entry only when every tracked file beneath it was read.
   Out-of-tree reads (toolchain, module cache, `/etc`) are environment and stay outside the
   coverage model. Write no read set, and add the note `read set not recorded: <reason>`, when:
   - the host is not Linux;
   - the row submits I/O through `io_uring`;
   - a process connects to a local socket;
   - a recorded in-tree path is untracked in the witness tree;
   - the tracer loses a process.
3. **Storage.** Write the read set as a sorted, newline-separated file under
   `docs/streams/<stream>/reads/`, named by its sha256. Bind it to the witness with one line
   after the Evidence table:
   `Read-Set: row=<n> command=sha256:<12 hex of the row's Command text> tree=<12 hex> reads=sha256:<64 hex>`.
   A line whose command digest or tree does not match its witness row is ignored. The witness
   then has no read set.
4. **One source, three callers.** Add `coverageOptionsFor(root string, patterns []string) coverageOptions`
   in `statusgen/coverage.go`. It reads the `Read-Set` lines and their files, and computes the go
   floor (step 6). `autoflip.go`, `reconcilefold.go` and `runCoverage` call it. No caller builds
   `coverageOptions{}` itself.
5. **Judging (library).** A witness with a readable read set speaks for exactly the recorded
   paths. It is stale when any of the following holds:
   - a recorded file's blob or mode differs between the witness tree and the item tree;
   - a listed directory's direct-children names differ;
   - a path recorded absent now exists;
   - the result of `git check-attr` for `text`, `eol`, `filter`, `working-tree-encoding` and
     `ident` on a recorded path differs.
   So a `.gitattributes` change that alters no recorded path's checkout releases. Exact refusal
   strings, which the tests assert:
   - `the witness's recorded read set cannot be read or does not match its digest`: a
     `Read-Set` line whose file is absent, unreadable or of another digest. The claim is
     `could-not-check`.
   - `the row read repository history (.git), so only a witness at the item's exact revision speaks for it`:
     any recorded path under the git directory. Exact revision only.
   Unchanged:
   - A witness with no `Read-Set` line keeps today's scope and reason.
   - A `+dirty` or `+unknown` witness keeps the conservative scope, read set or not.
   - The command-text and Expect guards run before any of this.
6. **The go floor (adapter, lower layer).** For a row shaped `cd <dir> && go test|build|vet …`,
   the adapter exports the witness tree to a scratch directory. It runs
   `go list -deps -test -json` there with `GOFLAGS=-mod=readonly GOPROXY=off GOWORK=off`.
   It collects every in-tree file listed: Go, cgo, C, H, S, ignored, test, x-test and embed
   files, plus each in-tree module's `go.mod` and `go.sum`. It hands the result to the library
   as an observation. The library refuses the claim with
   `the recorded read set omits a file the go toolchain compiles for this row` when the
   recorded set lacks any of them. If `go list` fails, the claim is `could-not-check` (for
   example, the module cache misses a dependency). The floor never widens a release. It only
   refuses.
7. **Contract.** In `docs/contracts.md`, rewrite the S-evidence-coverage row's
   independent-layer column to name the compile-input floor, its signal (a static dependency
   graph computed at evaluation time) and its gap (rows that are not go rows).
8. **Name the batch re-witness. Do not run it.** Add a section `## Batch re-witness` to this
   brief's stream README that states:
   - Who runs it: the verify desk, a non-implementing identity, with writing-form `verifyrun`
     under the recorder on a Linux host (a container qualifies).
   - Which revision: the first main commit that contains this brief's merge, the same one for
     every row.
   - Its bound: the briefs at `verified` whose only hold is `wrong-revision`, listed by
     `statusgen --coverage --json` at that revision before the run starts (133 at 5ddf5a317).
     Nothing is added mid-batch.
   - Failures: a row that fails takes the ordinary red path and is never re-run to green.
   - Exclusions: history-reading rows gain nothing from it, and the section says so.
9. **Parser measurement (report only).** On the implementation branch, count the claims that
   pass only through `deriveRowInputs` (42 at 5ddf5a317) and put that number in the PR body. The
   parser's retirement is a later brief, opened only once that count is 0 after the batch
   re-witness.

## Verify (executable — no prose-only DoD items)
Baselines: rows 1–14 and 16–17 are fail-first. Each names a test or check this brief adds, so
on the base commit each prints 0 or exits 1. Row 15 compares against before-measurement 2
(author-run, 5ddf5a317), re-taken at the implementation's merge. Before-measurement 1 (35 at
9ab9fa76ab4d, relayed) is context only, and no row asserts against it.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetReleasesUnreadPathChange$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetReleasesUnreadPathChange \('` | output is 1 — the test asserts a witness whose read set omits `.claude-plugin/marketplace.json`, `.assay-surfaces` and `.gitattributes` is released (claim `pass`) when the item differs from the witness only in those three paths | check:ci |
| 2 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetRefusesChangedDependency$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetRefusesChangedDependency \('` | output is 1 — the test asserts that, with the row-1 unrelated-path release in place, a one-line change to a recorded file of a depended-on package leaves the claim `wrong-revision` with a reason containing `differs between the witness's tree and the item's and the witness speaks for it` and naming that file | check:ci +mutation |
| 3 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetFloorCatchesUnderRecordedGoInput$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetFloorCatchesUnderRecordedGoInput \('` | output is 1 — LOWER layer with the UPPER bypassed: the read set deliberately omits a depended-on Go file and only that file changed, so the tree-wise comparison alone would release; the test asserts the claim is refused with the reason `the recorded read set omits a file the go toolchain compiles for this row` | check:ci +mutation |
| 4 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetMissingFailsClosed$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetMissingFailsClosed \('` | output is 1 — the test asserts a `Read-Set` line whose file is absent, and one whose file has a different digest, each give claim `could-not-check` with the reason `the witness's recorded read set cannot be read or does not match its digest`, and never `pass` | check:ci +mutation |
| 5 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetDirtyWitnessStillRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetDirtyWitnessStillRefuses \('` | output is 1 — the test asserts a `+dirty` witness carrying a valid read set, with only an unrecorded path changed, stays `wrong-revision` with a reason containing `uncommitted edits (+dirty)` | check:ci |
| 6 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetEditedRowStillRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetEditedRowStillRefuses \('` | output is 1 — the test asserts a witness with a valid read set whose row Command was edited after the run is refused with a reason containing `the witness records a different command than the Verify row now carries` | check:ci |
| 7 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetHistoryReadNeedsExactRevision$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetHistoryReadNeedsExactRevision \('` | output is 1 — the test asserts a read set containing a path under the git directory is refused at any other revision with the reason `the row read repository history (.git), so only a witness at the item's exact revision speaks for it`, and released at the exact revision | check:ci +mutation |
| 8 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetNegativeInputsRefuse$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetNegativeInputsRefuse \('` | output is 1 — the test asserts two refusals: a file added to a directory the row listed, and a file appearing at a path the row looked up and found absent; each leaves the claim `wrong-revision` | check:ci +mutation |
| 9 | `cd statusgen && go test ./evaluation/ -run '^TestReadSetContentAttributeChangeRefuses$' -count=1 -v \| grep -c -E '^--- PASS: TestReadSetContentAttributeChangeRefuses \('` | output is 1 — the test asserts a `.gitattributes` change that sets `eol` on a recorded path refuses the claim, while one that touches no recorded path releases | check:ci +mutation |
| 10 | `cd statusgen && go test ./evaluation/ -run '^TestLegacyWitnessKeepsConservativeScope$' -count=1 -v \| grep -c -E '^--- PASS: TestLegacyWitnessKeepsConservativeScope \('` | output is 1 — the test asserts a witness with no `Read-Set` line whose row is `cd tools/x && go test ./...` stays `wrong-revision` when only `.claude-plugin/marketplace.json` changed (no derived go input set exists) | check:ci |
| 11 | `cd statusgen && go test . -run '^TestCoverageCallersAgreeOnReadSet$' -count=1 -v \| grep -c -E '^--- PASS: TestCoverageCallersAgreeOnReadSet \('` | output is 1 — over one fixture repository the test asserts the auto-flip dry run flips the brief, the reconcile fold folds it, and `--coverage` reports it released, all for the same recorded witness | check:ci +flow |
| 12 | `! grep -n -F 'coverageOptions{}' statusgen/autoflip.go statusgen/reconcilefold.go statusgen/coverage.go` | exit 0 — no caller builds an empty manifest; all three go through `coverageOptionsFor` | check |
| 13 | `cd statusgen && go test . -run '^TestGoFloorFromWitnessTree$' -count=1 -v \| grep -c -E '^--- PASS: TestGoFloorFromWitnessTree \('` | output is 1 — over a fixture module with a cgo file, an embedded file, a build-tag-excluded file and an x-test file, the test asserts the floor lists all four and `go.mod`; with a dependency missing from the module cache it asserts `could-not-check` | check:ci |
| 14 | `cd statusgen && go test . -run '^TestVerifyrunRecordsReadSet$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunRecordsReadSet \('` | output is 1 on Linux — the test runs a row that reads a fixture file through a child process, lists a directory and probes an absent path, and asserts all three appear in the written read set and the `Read-Set` line's digest matches the file | check:ci +dereference |
| 15 | `statusgen --root . --auto-flip-model --dry-run \| grep -c 'REFUSED (stays verified)'` | the printed count equals the count printed by a statusgen built from the implementation merge's first parent, run on the same checkout; both are recorded in Evidence beside before-measurement 2 (36 at 5ddf5a317). The implementation alone releases no existing witness, because none yet carries a read set | gate:model |
| 16 | `cd statusgen && go test . -run '^TestVerifyrunUntraceableReadOmitsReadSet$' -count=1 -v \| grep -c -E '^--- PASS: TestVerifyrunUntraceableReadOmitsReadSet \('` | output is 1 — the test asserts a row that connects to a local socket, and a run on a non-Linux host (simulated), each write no `Read-Set` line and a note starting `read set not recorded:` | check:ci +mutation |
| 17 | `grep -F '\| S-evidence-coverage \|' docs/contracts.md \| grep -c -F 'compile-input floor'` | output is 1 — the contract row names the new independent layer | check |
| 18 | `statusgen --root . --lint` | exit 0; no PROBLEM line names `build-less-brittle/15` | check |

<!--
Pre-mortem → detection map (author-brief B5)
| Failure mode of the work | Caught by |
|---|---|
| Recorder misses a read on a go row, stale witness released | row 3 (floor, upper bypassed); row 14 (recorder sees child-process reads) |
| Recorder misses a read on a non-go row | row 16 for the named untraceable channels; otherwise review-only — no static floor exists, stated as the SPOF |
| Narrowing also releases a real change | rows 2, 8, 9 |
| Existing witnesses quietly released before the re-witness | rows 10, 15 |
| +dirty / edited / history rows loosened | rows 5, 6, 7 |
| One caller left on the empty manifest | rows 11, 12 |
| Read set file tampered or lost | row 4 |
| Batch re-witness run by the implementer or unbounded | Ground rules; README section (step 8) is review-checked |
-->

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter `gate: human`, the narrowing of a verification control). The
reviewer answers both design-fit questions in the verdict: (1) the single control between a
missed read and a stale release is the recorder's completeness; is the go floor plus fail-closed
recording an acceptable guard for go rows, and is the absence of a floor for other rows
acceptable? (2) row 3 proves the lower layer (the floor) refuses with the upper layer (the
tree-wise comparison) bypassed; confirm it plants the omission rather than walking the happy path.
