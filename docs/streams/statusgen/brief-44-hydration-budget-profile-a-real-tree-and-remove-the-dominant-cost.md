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

files: `statusgen/brieffile.go`, `statusgen/load.go`, `statusgen/hydrate_budget_test.go` (planned), `statusgen/testdata/hydrate-plant/` (planned), `statusgen/bench/README.md`, `statusgen/bench/workload-results.md`, `statusgen/bench/hydration-profile.md` (planned), `statusgen/bench/` corpus generator, `changelog/statusgen-hydration-budget.md` (planned).

facts (876ca4bca, 14 November 2026; timings are wall time of the released v1.0.84 binary on this
repository's tree: 38 streams plus 2 archived, 614 brief files of about 23 MB, 711 verify-outcome
records):
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
3. **Make the benchmark reproduce it.** Extend the bench corpus generator with a real-shape
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

## Review
Gate: model (from frontmatter — all four risk answers no). Not a core-system surface: nothing
here writes, authenticates or reaches a network. Reviewer records verdict + date in the stream
README table.
