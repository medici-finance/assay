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

### Non-implementer verify, 2026-11-18: claude-opus-5-5, strong

Admitted source is merged main 283b024f33318f1d811b8e0f42df6f7f63608fa8, dispatcher attestation [#6893](https://github.com/medici-finance/assay/issues/6893). Delivery is [PR #6575](https://github.com/medici-finance/assay/pull/6575), merged by the two-parent merge 05995daf1da1c307c39b23af41bbe77ffa1611b2 (first parent 0c41250f0aae, second parent 6e9735953a25). The delivered content is commit a45142e6fcbd5eb546b0515361fec6dac6eb09a6; the branch after it carries only two merges of main, and the delivered files are unchanged from a45142e6fcbd to the admitted source. The runner is not the implementer (the delivery was authored by the worker App). `deskpathguard rederive` reports 6 rows, all from base 283b024f3331, 0 author-added: the delivery did not change this Verify table. Every frontmatter risk answer is `no`, so no row carries a fail-first obligation.

#### Container execution witness (closed bracket)

These are the closed witness file's bytes as the bracket steps wrote them. Host open at 14:32:22Z, container rows 1, 2 and 4 from 14:32:52Z to 14:36:08Z, host close at 14:36:53Z. `statusgen verifyrun --bracket-digest` on the closed file reported `verdict=match` with `span-lines=tree-digest:1,container-image:1,attestation:1`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudget$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudget " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:cb2f7f70554c | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestHydrateBudgetPlant$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateBudgetPlant " "$check_output")` | pass exit=0 sandbox=container-netns | sha256:df81414c6b6f | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && GOWORK=off go test -count=1 ./...)` | fail exit=1 sandbox=container-netns | sha256:051bcca18646 | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:1efe6c181cc7f3cd68aeef2ed8ec2ef27c832d9b0ca1ecad663a759e8ba6f410
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6893 run=b4fbf0c76467162fd387d480392da0f005f5f0af2a7338668aef47a05ab85af4 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 brief=docs/streams/statusgen/brief-44-hydration-budget-profile-a-real-tree-and-remove-the-dominant-cost.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=b4fbf0c76467162fd387d480392da0f005f5f0af2a7338668aef47a05ab85af4 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 tree=sha256:1efe6c181cc7f3cd68aeef2ed8ec2ef27c832d9b0ca1ecad663a759e8ba6f410 witness=sha256:71cd8315bc4f7f879729cd2b917a632299223fd1387843834c0df8484b538bfe opened=2026-11-18T14:32:22Z closed=2026-11-18T14:36:53Z before=receipt after=check-verifier

#### Host rows 3, 5 and 6 (verifyrun, after the bracket closed)

Rows 3, 5 and 6 are not `check:ci`, so they ran on the darwin host from 14:45:54Z to 14:46:38Z. The two selections are disjoint and together cover all six rows.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && ASSAY_HYDRATE_TREE="$(cd .. && pwd)" GOWORK=off go test -count=1 -v -run "^TestHydrateRealTree$" . > "$check_output" && grep -F -- "--- PASS: TestHydrateRealTree " "$check_output")` | pass exit=0 | sha256:75fd3ce7275a | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -n -F -e "dominant:" -e "commit:" statusgen/bench/hydration-profile.md` | pass exit=0 | sha256:f83e1f0906e1 | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg-44" .) && "${TMPDIR:-/tmp}/sg-44" --root . --lint` | fail exit=5 | sha256:0dd1335ca96d | 2026-11-18 | assay-verifier-app[bot] @ 283b024f3331 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6893 run=b4fbf0c76467162fd387d480392da0f005f5f0af2a7338668aef47a05ab85af4 source=283b024f33318f1d811b8e0f42df6f7f63608fa8 brief=docs/streams/statusgen/brief-44-hydration-budget-profile-a-real-tree-and-remove-the-dominant-cost.md model=claude-opus-5-5 tier=strong

#### Execution envelope

Darwin has no host network-namespace lane, so rows 1, 2 and 4 ran in the installed v1.0.87 runner image ghcr.io/medici-finance/verify-runner:v1.0.87@sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1 (linux/arm64, Go 1.27.4, already present, not pulled). The container ran with `--network none --read-only --cap-drop ALL`, a writable tmpfs at /tmp, the admitted home mounted at /work, the receipt mounted read-only, and the flags the open step printed: the module cache built by `--open-bracket --module-cache` mounted read-only at /go/pkg/mod, `GOPROXY=off`, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`. The host rows used the declared verifier Git environment (every `GIT_*` variable unset, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_COUNT=0`, `GIT_TERMINAL_PROMPT=0`), `KUBECONFIG=/dev/null`, no forge token, and statusgen v1.0.87 as installed. No locally built statusgen ran an admitted row. Rows 1 to 3 send verbose output to a temporary file and print only the grep'd PASS line, so their recorded output hashes cover only that line.

The first host `--close-bracket`, run from outside the home, refused with the misleading message "the receipt admits brief docs/streams/statusgen/brief-44-….md, but this run's brief is docs/streams/statusgen/brief-44-….md" (the same path on both sides). Run again from the home, it closed with no edit to the home in between. This is the cwd-relative brief-path refusal seen in earlier bracket passes, not a property of this delivery.

#### Observed per row

- Row 1, FAIL on its Expect. The witness passed (exit 0, PASS line printed), and the count clause holds: history queries 0, parses 32 for 32 briefs. The Expect also requires "the hydrated load is at most 10 times the bare load (best of three)". The test does not assert that. It asserts `hydrateRatioBudget = 20.0` (statusgen/hydrate_budget_test.go:30) over `hydrateSynthRuns = 9` runs (statusgen/hydrate_budget_test.go:33). The observed ratio on the real-shape corpus is 13.6x (supplemental probe below). The implementer's own PR record gives 10.3x to 12.6x across four runs. The 10x clause is not met, and the test as delivered cannot fail on it.
- Row 2, PASS. Exit 0 with the PASS line. `TestHydrateBudgetPlant` turns on the test hook `briefHistoryNoticesInHydration` (statusgen/brieffile.go:1621; set only at statusgen/hydrate_budget_test.go:191 and reset at :192) on the committed fixture under statusgen/testdata/hydrate-plant/. It requires the reported failure to contain the `count: `, `git history queries` and `ratio: ` parts, so the budget names both the count and the ratio.
- Row 3, FAIL on its Expect. The witness passed (exit 0, PASS line printed; the test ran, not skipped), and the test logs both wall times. The Expect requires "the same budget holds on this repository's own tree", that is, the 10x bound. The test asserts `hydrateRealTreeBudget = 100.0` (statusgen/hydrate_budget_test.go:31). The observed ratio on this tree is 32.1x (642 briefs, history queries 0, parses 642; supplemental probe below). The implementer's own profile records 30x after the fix (statusgen/bench/hydration-profile.md). The 10x bound does not hold on this tree.
- Row 4, could-not-run (environment). The container run exited 1. A diagnostic re-run of the same command in the same image, mounts and flags (14:37:24Z to 14:45:06Z, no acceptance credit) shows the leaf failures:
  - TestPushcredHeaderInstrument/global, TestPushcredHeaderCleared/bash/global, TestPushcredHeaderCleared/sh/global and TestReconcileJobIgnoresRunnerResidue fail with `fatal: gitfile does not point to a valid repository: /work/.git`. The admitted home is a linked worktree, and its git directory is not mounted in the container.
  - statusgen/internal/gitlaunch TestGitCeilingOnlyDown fails with `could-not-check prior ratchet revision: exit status 128`, the same missing git directory.
  - TestWitnessSkewOldReader and TestWitnessV1Unchanged fail with `SG_OLD_BIN is unset: point SG_OLD_BIN at the previous release binary (statusgen/fetch-baseline.sh fetches it; this test never skips)`. The previous release binary is a network fetch, and the lane is network-off.

  None of these tests lives in a file the delivery touched. TestFlowQueue, TestEvaluationLibraryCLIParity, TestRunGateScoresHydratesFrontmatter and the TestDispatchView* tests are not among the failures, and every other statusgen package reported `ok`. Row 4's obligation is therefore unobserved, not failed. The missing capability is an admitted lane that mounts the home's git directory and supplies `SG_OLD_BIN` offline.
- Row 5, PASS. Both lines printed: `commit: 1db3085ddbdc8314ab46cdcd8a0ac4e256580d68` and the `dominant:` line naming newestFailFix, waitingBriefNotice, checkBriefFiles and loadHydratedStreams. The dereference below holds.
- Row 6, could-not-run (instrument configuration). The witness exited 5. The same command with no operator roster configured exits 5 and prints `LINT: REFUSED configuration; trust-dependent checks could-not-check`. With a roster configured (diagnostic, no acceptance credit), it exits 1 with `LINT: FAIL 3 problem(s)`, all `protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required`, for desk-tools/37, desktools-v2/65 and forge-neutral/54. No PROBLEM names this brief or a delivered file. The row command supplies neither the configuration nor the forge authority that a whole-tree lint at this source needs, so its obligation is unobserved here, not failed by this delivery.

#### Supplemental probe (no acceptance credit)

Date 2026-11-18; Runner assay-verifier-app[bot] @ 283b024f3331. This ran on the host after the admitted runs, from 14:50:50Z to 14:51:00Z, with host load average about 42. The command was the two row selectors together with `-v` and the full log kept: `ASSAY_HYDRATE_TREE=<this repository's root> GOWORK=off go test -count=1 -v -run '^(TestHydrateBudget|TestHydrateRealTree)$' .`, exit 0. The logged lines:

- `hydrate_budget_test.go:177: 32 briefs: bare 965.833µs, hydrated 13.145875ms (13.6x), history queries 0, parses 32`
- `hydrate_budget_test.go:219: … 642 briefs, bare 15.808958ms, hydrated 508.208375ms (32.1x), history queries 0, parses 642`

Both ratios exceed the Verify rows' 10x bound. They agree with the implementer's measurements recorded in PR #6575: 10.3x to 12.6x synthetic, and 27x to 30x on the real tree.

#### Row 5 dereference (review)

At the recorded commit 1db3085ddbdc8314ab46cdcd8a0ac4e256580d68, which is an ancestor of the delivery merge:

- `func newestFailFix` is at statusgen/stalefail.go:79.
- `func waitingBriefNotice` is at statusgen/stalefail.go:221, called from statusgen/brieffile.go:2024.
- `func checkBriefFiles` is at statusgen/brieffile.go:1613.
- `func loadHydratedStreams` is at statusgen/load.go:366 and calls `checkBriefFiles(streams, streams)` at statusgen/load.go:372.

The delivered diff changes each of them or its caller:

- loadHydratedStreams now calls `checkBriefFilesMode(streams, streams, briefHistoryNoticesInHydration)` (statusgen/load.go:377).
- checkBriefFiles becomes `return checkBriefFilesMode(streams, allStreams, true)` (statusgen/brieffile.go:1614).
- The waitingBriefNotice call is gated by `if historyNotices` (statusgen/brieffile.go:2046).
- newestFailFix gains `staleFailHistoryQueries.Add(1)` (statusgen/stalefail.go:80).

The only production caller passing false is hydration. The lint path keeps the notice.

#### Risk-bearing values: enumerate, rank, derive

Every frontmatter risk answer is `no`. The enumeration is over the delivered diff a45142e6fcbd against its parent 1db3085ddbdc (production and test literals):

- `hydrateRatioBudget = 20.0` @ statusgen/hydrate_budget_test.go:30
- `hydrateRealTreeBudget = 100.0` @ statusgen/hydrate_budget_test.go:31
- `hydrateRuns = 3` @ statusgen/hydrate_budget_test.go:32
- `hydrateSynthRuns = 9` @ statusgen/hydrate_budget_test.go:33
- history-query bound `m.historyQueries != 0` @ statusgen/hydrate_budget_test.go:159
- parse bound `m.parses > int64(m.briefs)` @ statusgen/hydrate_budget_test.go:162
- `hydrateCorpus = hydrateShape{streams: 4, briefsPerStream: 8, merges: 24}` @ statusgen/hydrate_budget_test.go:171
- `"real-shape":{"streams":4,"briefsPerStream":8,"merges":24}` @ statusgen/testdata/perfworkload/sizes.json:1
- `checkBriefFilesMode(streams, allStreams, true)` @ statusgen/brieffile.go:1614
- `var briefHistoryNoticesInHydration bool` (zero value false) @ statusgen/brieffile.go:1621

Ranking: nothing here is irreversible. Every value is a test bound, a corpus shape or a mode argument, fixable by edit and release. The two ratio budgets rank first, because they are the delivered substitute for the brief's pinned 10x bound. The production mode arguments rank next, because a wrong value would either restore the cost (true in hydration) or drop the lint notice (false in checkBriefFiles). The run counts and corpus shape are reversible knobs.

- RISK-VALUE: NAMED, NOT DERIVED — hydrateRatioBudget = 20.0 @ statusgen/hydrate_budget_test.go:30 — the brief's Verify row 1 pins 10x and its ground rules say "Do not weaken the budget to fit". 20.0 is derived only from the fixed code's own measured floor (10.3x to 12.6x), which is fitting the bound to the fix. No recorded human acceptance of a changed bound exists to derive it from.
- RISK-VALUE: NAMED, NOT DERIVED — hydrateRealTreeBudget = 100.0 @ statusgen/hydrate_budget_test.go:31 — Verify row 3 requires "the same budget", 10x. 100.0 sits above the observed 30x to 32x. It is chosen to stay under the roughly 730x the removed cost produced, not derived from the brief's bound.
- RISK-VALUE: DERIVED — briefHistoryNoticesInHydration = false (zero value) @ statusgen/brieffile.go:1621, with `checkBriefFilesMode(streams, allStreams, true)` @ statusgen/brieffile.go:1614 — hydration discards every problem and notice that checkBriefFiles returns. It is called only for the frontmatter side effect, so the history-backed notice has no reader there and false is correct. Lint and STATUS keep true, so no notice disappears from a reported surface (the ground rule "No verdict changes"). The only write of true is the plant test, at statusgen/hydrate_budget_test.go:191.
- RISK-VALUE: DERIVED — history-query bound 0 @ statusgen/hydrate_budget_test.go:159 and parse bound one per brief @ statusgen/hydrate_budget_test.go:162 — once hydration requests no notices, any history query during hydration is the removed cost returning. The brief asks that countable work be guarded at most once per brief.

#### Attempts

There was one attempt in this dispatch, 14:31:42Z to 14:46:38Z, 6 rows run: the container bracket for rows 1, 2 and 4, then the host run for rows 3, 5 and 6. The row 4 and row 6 diagnostics and the supplemental probe re-ran no admitted row for credit.

**VERIFY: FAIL** — rows 1 and 3 fail their Expect. The delivered budget is 20x (synthetic) and 100x (real tree), where the rows pin 10x, and the observed ratios are 13.6x and 32.1x. Rows 2 and 5 pass. Rows 4 and 6 could not run (environment: no git directory or `SG_OLD_BIN` in the container lane; lint without operator configuration or forge authority). The deviation was declared in the PR's Desk-decided block. A merge is not a recorded acceptance transition for this Verify table, so closing needs either a reviewed Verify-row amendment with recorded human acceptance of the 20x/100x bounds, or further work that brings hydration under 10x.


## Review
Gate: model (from frontmatter — all four risk answers no). Not a core-system surface: nothing
here writes, authenticates or reaches a network. Reviewer records verdict + date in the stream
README table.
