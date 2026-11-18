---
brief: assay:assay:statusgen:44
title: 'Hydration budget: profile a real tree and remove the dominant cost'
why: "Every statusgen query that needs brief frontmatter pays about eleven seconds on this repository's own tree, while the same load without that step takes a fifth of a second. Desk tools run those queries once per configured root and several times per act, so a board read that should be instant takes a minute or more. Nobody has profiled which part of the step costs the time, and the existing benchmark corpus does not reproduce it. This brief finds the cost, removes it, and adds a budget that fails when it comes back."
wave: 1
depends: []
unblocks: ["statusgen/39"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-11-15 by statusgen newbrief
sources:
  - "Measured on this repository's tree with the released v1.0.84 binary, 14 November 2026 (see Context facts): the gap between the bare load and the hydrated load"
  - "statusgen/bench/README.md and statusgen/bench/workload-results.md: the existing synthetic workload and its recorded timings"
  - "statusgen/26 (phase measurements through the shared performance recorder): the coarse discover/parse/evaluate phases this brief refines for one step"
  - "tools/desk/cmd/deskboard/roots.go lines 3 to 13: the recorded cost of running one statusgen subprocess per root"
  - freshness-checked 2026-11-14 @ 876ca4bca (origin/main)
exec-tier: strong
exec-tier-why: "The fix is not specified: it depends on what the profile shows, and the implementer must choose it without changing any verdict."
domain: complicated
outcome: none
---

# Brief 44 — Hydration budget: profile a real tree and remove the dominant cost

## Context

files: `statusgen/brieffile.go`, `statusgen/load.go`, `statusgen/hydrate_budget_test.go` (planned), `statusgen/testdata/hydrate-plant/` (planned), `statusgen/bench/README.md`, `statusgen/bench/workload-results.md`, `statusgen/bench/hydration-profile.md` (planned), `statusgen/bench/perfworkload.sh`, `changelog/statusgen-hydration-budget.md` (planned).

facts (876ca4bca, 14 November 2026):
- Timings below are wall time of the released v1.0.84 binary on this repository's tree: 38
  streams plus 2 archived, 614 brief files of about 23 MB, 711 verify-outcome records.
- Bare load: `--graph` 0.17 s, `brief <key> --json` 0.02 s. Both call `loadStreams` (`load.go:195`).
- Hydrated load: `--gate-scores` 10.8 s and 10.9 s (two separate runs), `--eligibility --json`
  10.6 s, `--launch` 11.6 s, `--next-up` 18.4 s (it also reads git and remote branches). All call
  `loadHydratedStreams` (`load.go:366`), whose extra work is `attachPlaceholders` and
  `checkBriefFiles(streams, streams) // for its hydration side effect only` (`load.go:371-372`).
- Which part of `checkBriefFiles` (`brieffile.go:1613`) costs the time is NOT known. No profile
  exists. The hydrated modes do not run the full lint, and file reads and brief parses are
  already memoized (`readFileMemo`, `briefParseMemo`), so repeated reads are an unlikely cause.
- The synthetic benchmark does not show the cost: its large corpus (240 briefs) records
  `--gate-scores` at 27 to 37 ms (`statusgen/bench/workload-results.md`). The real tree has 2.6
  times the briefs and about 300 times the wall time.
- Callers multiply it. `deskboard` runs `--gate-scores` and `--next-up` once per configured root;
  `tools/desk/cmd/deskboard/roots.go:10-13` records 82.82 s for one `throughput` run over four roots.
- Candidates seen while reading, none proven: a regular expression compiled per call at
  `brieffile.go:455`; `loadGraphRepos` and `loadOutcomeRegister` called per root inside
  `checkBriefFiles`; the 711 verify-outcome files read into `awaitOutcomeCache`.

layering: no new component. The work stays inside the existing hydration step; the profile
harness and the budget are tests and benchmark files.

design-fit:
  owner: statusgen
  contract: none — no meaning changes; every existing output stays byte-identical
  retires: []
  weight: "verbs 0, flags 0, refusals 0, rule-text lines 0 (one test-only environment variable names the tree to measure)"
  why-add: n/a

## Read first

- `statusgen/load.go` lines 195 to 375 and `statusgen/brieffile.go` lines 1613 to 2152.
- `statusgen/bench/README.md`: how the workload is generated and what it deliberately leaves out.
- `statusgen/readmemo.go` and `brieffile.go` lines 510 to 610: the two existing memos.
- `docs/three-state-instrument-rule.md`: a faster answer that drops a could-not-check is wrong.

## Ground rules

- Isolated branch and draft PR. No push to main, no merge, no release, no workflow trigger.
- No verdict changes. Every hold, notice and could-not-check the hydrated modes report today is
  reported after the change, in the same order. A faster result that skips required work fails.
- No persistent cache and no new flag. `statusgen/README.md` states there is no persistent
  verdict cache; adding one is a separate decision this brief does not take.
- If the profile shows the dominant cost is work the verdicts need, and it cannot be done once
  instead of repeatedly, STOP and report NEEDS_CONTEXT with the profile attached. Do not weaken
  the budget to fit.
- Stop at `implemented`. A different, non-implementing identity verifies and moves the brief on.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Profile before changing anything.** Add a test harness that runs the bare load and the
   hydrated load on the tree named by `ASSAY_HYDRATE_TREE` and writes a CPU profile under
   `${TMPDIR:-/tmp}`. Run it on this repository's tree. Record in
   `statusgen/bench/hydration-profile.md` (planned): the commit, the host class, both wall times, and the
   ten functions with the largest cumulative time, with a line starting `dominant:` naming the
   function or functions that account for most of the gap.
2. **Remove the dominant cost.** Fix every term that alone accounts for 20% or more of the
   hydrated wall time, in descending order, until step 4's budget holds. Prefer doing work once
   per load over caching across loads.
3. **Make the benchmark reproduce it.** Extend the bench corpus generator (`statusgen/bench/perfworkload.sh`) with a real-shape
   profile that exhibits the cost the profile found (for example long Evidence sections, many
   verify-outcome records, or many cross-references, whichever it was), so the synthetic workload
   would have caught it. Record before and after in `statusgen/bench/workload-results.md`.
4. **Gate a budget.** The budget proposed here is: on one tree, in one process, the hydrated
   load takes at most 10 times the bare load (today about 64 times: 10.8 s against 0.17 s). Its
   basis is the synthetic corpus, where hydration is already cheap. Where the profile identifies
   countable work (for example parses per brief file), guard the count as well, because a count
   does not vary with the host. The timing check takes the best of three runs.
5. **Prove the guard can fail.** Commit a fixture that reintroduces the removed cost behind a
   test hook and assert that the budget check reports it.
6. Add the changelog fragment. State the measured before and after for `--gate-scores` on this
   repository's tree in the PR body.

## Verify

Commands run from this repository root. The named tests are planned deliverables. Each row greps
the exact PASS line, so a selector that matches nothing fails the row.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudget$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudget " "$check_output")` | exit 0; PASS line printed. On the real-shape corpus the hydrated load is at most 10 times the bare load (best of three), and each countable unit the profile named is done at most once per brief |
| 2 | check:ci +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudgetPlant$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudgetPlant " "$check_output")` | exit 0; PASS line printed. With the planted fixture in `testdata/hydrate-plant/` re-enabling the removed cost, the budget check reports a failure naming the ratio or the count |
| 3 | check +flow | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && ASSAY_HYDRATE_TREE="$(cd .. && pwd)" GOWORK=off go test -count=1 -v -run "^TestHydrateRealTree$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateRealTree " "$check_output")` | exit 0; PASS line printed (a skipped test prints no PASS line and fails the row). The same budget holds on this repository's own tree, and the test logs both wall times |
| 4 | check:ci +neighbour | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | exit 0; every existing statusgen test still passes, including `TestFlowQueue` (the `--gate-scores` golden), `TestEvaluationLibraryCLIParity`, `TestRunGateScoresHydratesFrontmatter` and the `TestDispatchView*` tests, so the hydrated modes' output is unchanged |
| 5 | gate:model +dereference | `grep -n -F -e "dominant:" -e "commit:" statusgen/bench/hydration-profile.md` | exit 0; both lines printed. The reviewer then checks that each function named on the `dominant:` line exists in the tree at the recorded commit and that the diff changes it or its caller |
| 6 | check | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-44" .) && "${TMPDIR:-/tmp}/sg-44" --root . --lint` | exit 0; output contains "LINT: PASS" |

| Failure mode of the work | Caught by |
|---|---|
| The fix is a guess and the real cost stays | rows 1 and 3: the budget fails on the corpus and on the real tree |
| Speed is bought by skipping a check, so a hold or a could-not-check disappears | row 4: the goldens and parity tests |
| The budget passes because the corpus never had the cost | row 2: the planted fixture must fail it |
| The profile file names a function that was never the cost | row 5: review against the tree |
| The timing check flakes on a slow host | review-only: the count guard is the primary assertion where one exists; the timing check takes the best of three |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudget$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudget " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:236bed00c70b | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudgetPlant$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudgetPlant " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:e1f8e0126577 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | fail exit=1 sandbox=container-netns | sha256:80bd9ba6b275 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:7fc8216b67e2dc1d9eac1305a2b70a311ca2d70f94c13ea341c3d73e24f2c5b3
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6830 run=91c9aa90348a8b1b62073fa9c1a64dcaaba1e82a333fee75393b2c7cec2c59f8 source=185b7ee7ebdfefbcd5866c8dc2389a2ab82989e9 brief=docs/streams/statusgen/brief-44-hydration-budget-profile-a-real-tree-and-remove-the-dominant-cost.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=91c9aa90348a8b1b62073fa9c1a64dcaaba1e82a333fee75393b2c7cec2c59f8 source=185b7ee7ebdfefbcd5866c8dc2389a2ab82989e9 tree=sha256:7fc8216b67e2dc1d9eac1305a2b70a311ca2d70f94c13ea341c3d73e24f2c5b3 witness=sha256:8089c77860c419e84f95b94f6efd4cc29db6285034f94a05227db4a3d7cb8ca0 opened=2026-11-18T01:55:22Z closed=2026-11-18T01:59:41Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && ASSAY_HYDRATE_TREE="$(cd .. && pwd)" GOWORK=off go test -count=1 -v -run "^TestHydrateRealTree$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateRealTree " "$check_output")` | pass exit=0 | sha256:1614ddfc4955 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -n -F -e "dominant:" -e "commit:" statusgen/bench/hydration-profile.md` | pass exit=0 | sha256:f83e1f0906e1 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-44" .) && "${TMPDIR:-/tmp}/sg-44" --root . --lint` | fail exit=5 | sha256:573ea0f7ed74 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6830 run=91c9aa90348a8b1b62073fa9c1a64dcaaba1e82a333fee75393b2c7cec2c59f8 source=185b7ee7ebdfefbcd5866c8dc2389a2ab82989e9 brief=docs/streams/statusgen/brief-44-hydration-budget-profile-a-real-tree-and-remove-the-dominant-cost.md model=claude-opus-5-5 tier=strong

Verifier observations (claude-opus-5-5 non-implementer, merged main 185b7ee7ebdf; delivery PR #6575, merge 05995daf1da1). Rows 1, 2 and 4 ran in one closed container-netns admission bracket above (bracket-digest verdict=match); rows 3, 5 and 6 ran on the host. Supplemental rows below carry NO acceptance credit; they read the logged values the witness hashes do not show.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| S1 (row 1, no credit) | row 1 test run with -v in the same verify-runner image, network none | hydrated at most 10x bare on the real-shape corpus; one parse per brief | exit 0; log line: 32 briefs, bare 121.793us, hydrated 1.55072ms (12.7x), history queries 0, parses 32. The ratio exceeds the 10x Expect; the test passes only because its bound is 20x | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |
| S2 (row 2, no credit) | row 2 test run with -v in the same image | plant reported by ratio or count | exit 0; planted: 32 git history queries during hydration (32 briefs), want 0; ratio 320.7x bare, budget 20x | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |
| S3 (row 3, no credit) | row 3 test run with -v on the host, tree = the home | same 10x budget holds on this repository's tree; both wall times logged | exit 0; log line: 642 briefs, bare 6.940583ms, hydrated 215.354458ms (31.0x), history queries 0, parses 642. Both wall times are logged; the ratio is about 3x over the 10x budget; the test passes only because its real-tree bound is 100x | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |
| S4 (row 4, no credit) | row 4 command rerun in the same image with output captured | exit 0 | exit 1. Six failures, all environmental. Three git-config subprocess tests (pushcred instrument and cleared, reconcile-job runner residue) fail with "gitfile does not point to a valid repository" because the home is a linked worktree whose gitdir lies outside the mount. The git-ceiling ratchet test cannot read the prior revision (no history in the container). Both witness-skew tests need SG_OLD_BIN, which CI supplies through fetch-baseline.sh; v1.0.52 publishes no linux-arm64 asset (HTTP 404), so the arm64 image has no baseline. Every other package is ok, and no failure touches the hydration code | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |
| S5 (row 5, reviewer dereference) | read hydration-profile at recorded commit 1db3085ddbdc | named functions exist at the recorded commit; diff changes them or their caller | newestFailFix exists in stalefail.go, checkBriefFiles in brieffile.go and loadHydratedStreams in load.go at 1db3085ddbdc. The diff routes the load.go caller through checkBriefFilesMode, gating waitingBriefNotice, and adds the history-query counter in newestFailFix. Minor defect: the dominant line attributes waitingBriefNotice to brieffile.go, but it is defined in stalefail.go (called from brieffile.go) | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |
| S6 (row 6, no credit) | row 6 command rerun on the host with output captured | LINT: PASS | exit 1 (witness exit 5): LINT: FAIL 3 problem(s), all on other briefs (desk-tools/37, desktools-v2/65, forge-neutral/54): "protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required". The row's command takes no --acceptance-forge (CI's lint step does), so offline it cannot pass on a tree that holds protected transitions. No problem names statusgen/44 | 2026-11-18 | assay-verifier-app[bot] @ 185b7ee7ebdf (model verifier, supplemental) (on-behalf-of human:ian) |

Risk-bearing values: enumerated over the delivery diff (hydration budget test, brieffile/load/stalefail hooks, real-shape sizes). Ranked: the two ratio bounds first, because they decide whether the gate catches a regression. Every literal is a reversible test threshold, and none governs an irreversible act.

RISK-VALUE: NAMED, NOT DERIVED — hydrateRatioBudget = 20.0 @ statusgen/hydrate_budget_test.go:30 — brief Task 4 and row 1 Expect fix the bound at 10x. The code comment justifies 20x as "a 10x bound would flake on the fix itself", which fits the bound to the result; the brief forbids that ("Do not weaken the budget to fit"; STOP and report NEEDS_CONTEXT instead).
RISK-VALUE: NAMED, NOT DERIVED — hydrateRealTreeBudget = 100.0 @ statusgen/hydrate_budget_test.go:31 — row 3 Expect requires "the same budget" (10x) on the real tree; observed 31.0x. No derivation of 100x appears beyond the measured ~27x.
RISK-VALUE: DERIVED — historyQueries want 0 @ statusgen/hydrate_budget_test.go:159 — hydration discards its notices, so any history query is pure waste. Task 4 says to guard countable work, and the plant (S2) shows this bound discriminates.
RISK-VALUE: DERIVED — parses at most briefs @ statusgen/hydrate_budget_test.go:162 — the Expect says "each countable unit at most once per brief"; observed 32/32 and 642/642.

**VERIFY: FAIL** — rows 1 and 3: observed hydrated/bare ratios 12.7x (real-shape corpus) and 31.0x (this repository's tree) exceed the brief's 10x budget. The tests pass only because the bounds were raised to 20x and 100x (statusgen/hydrate_budget_test.go:30-31), against the brief's "Do not weaken the budget to fit". Rows 2 and 5 pass. Rows 4 and 6 are environment-blocked (linked-worktree gitfile and missing linux-arm64 SG_OLD_BIN baseline in the container; lint needs --acceptance-forge on other briefs' protected transitions), with no implementation failure observed.

## Review
Gate: model (from frontmatter — all four risk answers no). Not a core-system surface: nothing
here writes, authenticates or reaches a network. Reviewer records verdict + date in the stream
README table.
