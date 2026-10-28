---
brief: assay:assay:quality:19
title: qualgen re-fix metric — SZZ-traced fixes that repeat a defect an earlier fix already addressed
why: >-
  A regression suite, class guards and stub-coverage rules are only worth their cost if fewer
  defects come back. Nothing measures that today. This brief adds the trend line: how often
  a fix repairs something an earlier fix had already repaired (a re-fix). It is linked
  either explicitly through `regression-of:` or through a shared defect class. The line is
  rendered in the quality report and gates nothing until it has been measured.
wave: 3
depends: ["quality/07"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v2
version: 1
authored: 2026-09-23 by assay-worker session (issue #1581, part 4 of 4)
sources:
  - "issue #1581 part 4 — qualgen reports RE-FIXES: a fix whose SZZ-traced inducing change repeats a pattern an earlier fix addressed, the earlier fix linked via `regression-of:` (#1580) or the same fix-linkage class; the trend tells whether the suite works; measured before it gates anything. Done criterion: renders in the quality report"
  - "issue #1580 — defines `regression-of:` (the prior fix's issue or commit) on fix/bug briefs; pairing recorded here, not as a depends: edge (#1580 is issue-only)"
  - "docs/streams/quality/spec.md §9.6 and §13 item 4 (Thresholds) — budgets/thresholds only after ≥ 2 windows of measurement"
  - "docs/streams/quality/brief-06-m2-fix-identification.md and brief-07-m2-szz-trace-metrics.md — the DefectFix / DefectTrace contracts this brief reads"
  - "freshness-checked 2026-09-23 @ 5ee5ccb39 — no re-fix metric in qualgen; `renderReport` (qualgen/report.go) renders M2 as a `not measured` placeholder; `ClassifyFix` / `TraceDefects` have no non-test caller (M2 is library-only, not yet wired into `mine`)"
exec-tier: strong
exec-tier-why: >-
  (a) the linkage adapter's shape and the "earlier" ordering are design calls the facts
  bound but do not fully fix; (b) the metric joins three records (DefectFix, DefectTrace,
  the regression link) across two briefs' frozen contracts.
consumers:
  - "qualgen/report.go: follow-up quality/19 (this brief; adds the re-fix section to the rendered QUALITY.md view)"
  - "docs/quality/QUALITY.md: out-of-scope (CI-written single-writer view; it picks up the new section on the next main-push regen, never hand-edited in a PR)"
---

# Brief 19 — qualgen re-fix metric

## Context

files:
- NEW `qualgen/regressionlink.go` (planned) + `regressionlink_test.go` (planned) — the
  pluggable `RegressionLinkage` seam and its reference adapter.
- NEW `qualgen/refix.go` (planned) + `refix_test.go` (planned) — the re-fix join and metric.
- `qualgen/report.go` — a `## Re-fix rate (regression-suite effectiveness)` section in `renderReport`.
- NEW `qualgen/testdata/refix/` (planned) — fixture DefectFix / DefectTrace records and
  brief files.
- `changelog/<branch-slug>.md` — the per-PR fragment.

facts:
- **Records (frozen contracts — do not rename fields):** `DefectFix` in
  `qualgen/fixlinkage.go` (`FixCommitSHA`, `FixPRNumber`, `ClosedIssue *IssueRef`, `Tier`,
  `Identified`); `DefectTrace` in `qualgen/szz.go` (`FixCommit`, `FixPR`,
  `InducingCommits`, `TraceState`). Both live in the `defects` table
  (`Store.ReadDefects` / `Store.ReadTraces` in `qualgen/store.go`).
- **Existing seam convention:** `LinkageAdapter` (`qualgen/fixlinkage.go`) is the
  pattern: an interface plus a reference adapter, with an adapter error mapped to
  could-not-measure and never to a silent "no". `adapters.IssueLabelSource`
  (`qualgen/adapters/githublabels.go`) already reads an issue's labels.
- **`regression-of:` (from #1580):** a fix/bug brief names the prior fix's issue or commit.
  Pickup precondition: `grep -n 'regression-of' plugins/assay/skills/author-brief/SKILL.md`
  prints at least one line. Read the exact value format from there, and accept both an
  issue ref (`#N`, `owner/repo#N`) and a commit SHA. If it prints nothing, #1580 has not
  landed: report NEEDS_CONTEXT. Do not invent the format.
- **Re-fix definition:** a fix F is a re-fix when its trace is `traced` and there is a fix E
  that meets BOTH conditions:
  - E is linked to F by (a) F's `regression-of:` naming E (E's closed issue, fix commit or
    fix PR), or (b) F's closed issue and E's closed issue carrying the SAME defect-class
    key;
  - E's fix commit time is before the time of F's EARLIEST inducing commit. The defect was
    fixed, then reintroduced. A later or concurrent E is not a re-fix.
- **Defect-class key:** read from the closed issue's labels by a CONFIGURED label prefix
  (e.g. `class:`). There is no built-in default. Unconfigured, class linkage is
  `could-not-measure` and never "no class".
- **Metric:** per window, `refix_count` and `refix_rate` = re-fixes / traced fixes, each
  a three-state `Measure`. Beside them: `linkage_coverage` = the share of traced fixes
  whose regression linkage resolved (either path), and the evidence-tier composition of
  the counted fixes (spec §10 honest-claims). It is REPORT-ONLY: no threshold, budget or
  alarm. Thresholds follow ≥ 2 windows (spec §13 item 4, Thresholds; and §9.6) in a later brief.
- **What renders on this repo today:** the `defects` table is not populated here (M2 is
  library-only), so the section renders `not measured` with the reason. That is correct
  three-state output, not a failure. Numbers appear on any target whose defects table is
  populated. Wiring M2 into `mine` is out of scope.
- single-point-of-failure: the linkage is the one control. A re-fix whose link is missing is
  counted as nothing. The layer behind it is `linkage_coverage`, published beside the rate,
  so an under-linked corpus reads as "low coverage", never as "few re-fixes". The two
  linkage paths (explicit `regression-of:` and the class key) come from different sources
  (the brief file versus the issue tracker) and fail independently.

layering: adds one adapter seam, `RegressionLinkage` (planned), in the existing
`LinkageAdapter` pattern. The current reason is that the link comes from two sources that
differ per target (brief files in git; issue labels in a tracker). The re-fix join and
metric are pure over DefectFix / DefectTrace / link records and are tested with fixture
records (no git, no tracker). The reference adapter holds the effects (Task 1–2;
Verify 2–5 pure, 6–7 render).

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Report-only: no gating, budget or alarm on the re-fix rate. Never hand-edit
  `docs/quality/QUALITY.md` (CI is its single writer).
- Do not change `DefectFix` / `DefectTrace` field names. Add, never rename.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `regressionlink.go`: `type RegressionLinkage interface { RegressionOf(f DefectFix)
   (refs []RegressionRef, ok bool, err error); DefectClass(ref IssueRef) (class string,
   ok bool, err error) }`. Reference adapter: `RegressionOf` reads `regression-of:` from
   the brief(s) tied to the fix — the brief named by the fix commit message's `Brief:`
   trailer, or any brief file the fix commit touches — at the fix commit's tree.
   `DefectClass` reads the configured label prefix through `adapters.IssueLabelSource`.
   Adapter errors → could-not-measure.
2. `refix.go`: the join and the ordering rule above, then the metric records per window,
   written through the existing `Store` (a new metric name in the metrics table, following
   the `MetricRecord` pattern).
3. `report.go`: render the section: rate, count, linkage coverage, tier composition, one
   row per window (the trend line), and `not measured` with its reason when the inputs are
   absent. Never render a 0 that was not measured.
4. Fixtures + tests, one per Verify row 2–6.
5. Fail-first: paste the ordering test (row 4) failing with the ordering check removed into
   the PR body under `## Fail-first`.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector (`^Name$`), writes the output to a file, and
asserts that test's `--- PASS:` line. A test that is missing or misnamed then fails the row.
A bare `-run` would print `no tests to run` and exit 0 (the vacuous pass statusgen/14 lints for).

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 | check:ci |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | exit 0; a traced fix whose brief's `regression-of:` names an earlier fix is counted as a re-fix | check:ci |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | exit 0; with a class prefix configured, two fixes whose closed issues share a class key yield one re-fix | check:ci |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | exit 0; negative path: a linked fix E whose fix time is AFTER F's earliest inducing commit is NOT a re-fix | check:ci +mutation |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | exit 0; with no `regression-of:` anywhere and no class prefix, rate and coverage are could-not-measure, never measured-zero | check:ci |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | exit 0; over a fixture store holding one planted re-fix, the rendered report's re-fix section names that fix's PR number and a rate of the expected value. This dereferences the rendered number back to the planted record, not only the heading | check:ci +dereference |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | exit 0; over this repo's real tracking root the section renders (as `not measured` with its reason until the defects table is populated) | check +flow |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
### Non-implementer verifier run — VERIFY: PASS on behaviour, held — 1/7 witness-clear, rows 1–6 held (#1800) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`, cross-checked), host darwin/arm64, go1.27.1, `KUBECONFIG=/dev/null`. Implementing merge: #1658 (136f670eb). `gate: model`, all four risk answers `no`. Status stays `implemented`: rows 1–6 are `check:ci` and their network-off witness cannot run on a darwin host (#1800). All direct runs below are non-hermetic and supporting.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 | HELD (#1800) — direct run exit 0, no output | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | Verify row 2 command verbatim (`-run` anchored on the regression-of-link counted test) | exit 0 + named PASS line | HELD (#1800) — direct run exit 0; `--- PASS` for Test Refix_RegressionOfLink_Counted | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | Verify row 3 command verbatim (same-defect-class counted test) | exit 0 + named PASS line | HELD (#1800) — direct run exit 0; `--- PASS` for Test Refix_SameDefectClass_Counted | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | Verify row 4 command verbatim (earlier-fix-after-inducer not counted) + mutation | exit 0; mutant red | HELD (#1800) — direct run exit 0, `--- PASS`; verifier's own mutation (`et.Before(earliestInducing)` at qualgen/refix.go:371 forced true) → exit 1, `refix_test.go:154 expected refix_count measured-zero … got {State:measured Value:1}` (mutant killed) | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | Verify row 5 command verbatim (no linkage configured → could-not-measure) | exit 0 + named PASS line | HELD (#1800) — direct run exit 0; `--- PASS` for Test Refix_NoLinkageConfigured_CouldNotMeasure | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | Verify row 6 command verbatim (report refix section renders) | exit 0 + named PASS line | HELD (#1800) — direct run exit 0; `--- PASS`; asserts "PR #4242", "PR #4100" and "0.25" in the rendered view | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | `cd qualgen && go build -o $TMPDIR/qualgen19 . && $TMPDIR/qualgen19 report --out .. > $TMPDIR/quality19.md && grep -F 'Re-fix rate' $TMPDIR/quality19.md` | exit 0; section present | PASS — exit 0; `## Re-fix rate (regression-suite effectiveness)`, next line `not measured — populated once the defects table is traced … Never rendered as 0.` | 2026-09-30 | claude-opus-5-5-verifier |

Ground checks: `regression-of` present in author-brief SKILL.md (lines 102, 469, 493, 503); the implementing diff touches neither qualgen/fixlinkage.go nor qualgen/szz.go (frozen fields intact); no threshold, budget or alarm added; `MetricRefix = "refix"` unique; an equal-time fix is not counted (strict `Before`).

RISK-VALUE (trigger does not fire — risk all `no`, not irreversible, no risk-classed path; enumerated anyway):

- RISK-VALUE: DERIVED — `shaPattern` min length 7 @ qualgen/regressionlink.go:83 — git's default minimum abbreviation; matched only against the identified-fix set, so a false prefix match is ~4e-5 at 10^4 fixes, and bounded by the ordering rule. Reversible (report-only metric).
- RISK-VALUE: N/A — `MetricRefix`, the path/trailer regexes and the empty default class prefix are identifiers or format bindings; the only comparator (strict `Before`, refix.go:371) matches the brief's "a later or concurrent E is not a re-fix".

Findings: F1 filed as #1844 — with every traced fix issue-less, no `regression-of:` and no class prefix, the window renders measured-zero where the brief says could-not-measure (outside every Verify row). F2 (minor): the planned `qualgen/testdata/refix/` fixtures were written inline in the tests instead. F3 (minor): row 6 checks a planted pre-computed rate is echoed, not derived through `ComputeRefix`.

### 2026-10-02 desk dispatch — re-verify at e1d99484ffd9: all 7 rows pass by hand, witness 1/7 (check:ci rows 1–6 held on #1800), inputs unchanged since the 2026-09-30 record

The runner is not the implementer. It used an isolated detached worktree at merged main `e1d99484ffd9`, darwin/arm64, go1.27.1, with a throwaway HOME, `KUBECONFIG=/dev/null`, `GOPROXY=off` and no forge token in the environment. The implementing merge is #1658. The brief is `gate: model` and all four risk answers are `no`. Every Verify command was run verbatim from the repo root. Each input the 2026-09-30 outcome record hashed is byte-identical at this sha: the brief, the regression-link and re-fix sources and tests, and the report renderer. Since that record, no commit has touched the qualgen re-fix code.

| # | Command | Exit | Observed | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | `cd qualgen && go build ./... && go vet ./...` | 0 | No output. Witness: could-not-run, because a darwin host has no network-off sandbox (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 command verbatim (anchored run of the regression-of-link counted test, output captured, PASS line grepped) | 0 | `--- PASS` line for the regression-of-link counted test (0.00s). Witness: could-not-run on darwin (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 command verbatim (anchored run of the same-defect-class counted test) | 0 | `--- PASS` line for the same-defect-class counted test (0.00s). Witness: could-not-run on darwin (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 command verbatim (anchored run of the earlier-fix-after-inducer not-counted test) | 0 | `--- PASS` line for the earlier-fix-after-inducer not-counted test (0.00s). Witness: could-not-run on darwin (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 command verbatim (anchored run of the no-linkage-configured could-not-measure test) | 0 | `--- PASS` line for the no-linkage-configured could-not-measure test (0.00s). Witness: could-not-run on darwin (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6 command verbatim (anchored run of the report re-fix section renders test) | 0 | `--- PASS` line for the report re-fix section renders test (0.02s). Witness: could-not-run on darwin (#1800) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | Verify row 7 command verbatim: build qualgen into the temp dir, run `report --out ..`, then grep `Re-fix rate` | 0 | `## Re-fix rate (regression-suite effectiveness)`, then `not measured — populated once the defects table is traced (quality/06–07) and a RegressionLinkage adapter is configured. Never rendered as 0.` Witness: pass | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun --dry-run` (v1.0.31) exited 2 with 1 of 7 rows proven. Row 7 passed. Rows 1–6 are `check:ci`, and their network-off re-execution needs `unshare --net`, which exists only on Linux (#1800, open). These rows clear on a Linux runner.

Vacuity check, run in a scratch clone and never in the verify worktree:

- **Row 1 is non-discriminating.** At the parent of the implementing commit, row 1 also exits 0.
- **Rows 2, 6 and 7 fail at that parent:** exit 1, 1 and 1. The tests and the heading do not exist there. Rows 3–5 fail the same way, since their tests also first appear in #1658.
- **Mutations against the current code:**
  - Row 2: disabling the regression-of match makes the test fail at line 79.
  - Row 3: forcing the class path to "no class" makes the test fail.
  - Row 4: forcing the ordering comparison true makes the test fail at line 154.
  - Row 5: removing the could-not-measure guard makes the test fail at line 196 (`measured-zero`).
  - Row 6 checks a planted, pre-computed rate. A mutation of the rate formula in the re-fix join survives row 6 but is caught by row 2 (line 117).
- **Surviving mutant.** Changing the strict "before" comparison at the ordering rule to "at or before" passes rows 2–6 and every Refix, Report and Regression test. No test pins the brief's rule that a concurrent earlier fix is not a re-fix. The code (a strict comparison) is correct as written; only the test coverage of that boundary is missing.

Open finding #1844 still reproduces at this sha. A scratch probe ran a window where the only traced fix is issue-less, with no `regression-of:` and an unconfigured class prefix. It returned `measured-zero` for both rate and coverage, where the brief's facts say could-not-measure. This lies outside every Verify row.

- RISK-VALUE: DERIVED — `shaPattern` = `^[0-9a-fA-F]{7,40}$` @ qualgen/regressionlink.go:83 — 7 is git's default minimum abbreviation and 40 is a full SHA-1 hex id. A match is only resolved against the identified-fix set, and the ordering rule bounds it. The metric is report-only and reversible by an edit. A SHA-256 object-format repo's 64-hex ids would not classify; that is not reachable on SHA-1 repos.
- RISK-VALUE: N/A for the rest — the enumeration covered the implementing diff of the re-fix join, the regression link, the report renderer and the artifact table. It found no threshold, budget, alarm or timeout. The remaining literals are zero-length guards, the metric name, and path, trailer and ref regexes. The 4-significant-figure rate formatting predates this brief.

The brief is `gate: model` with all risk answers `no`, so a model sign-off is permitted. It is held only on the environment witness.

VERIFY: BLOCKED — all 7 rows pass by hand at e1d99484ffd9; witness 1/7, check:ci rows 1–6 need the Linux network-off witness (#1800); finding #1844 still open; new coverage gap: concurrent-fix boundary not pinned by any test

### 2026-10-11 desk dispatch — re-verify at 7808e0f17daa: all 7 rows pass by hand (twice), witness 1/7 (check:ci rows 1–6 held on #1800)

The runner is not the implementer. It used an isolated, locked, detached worktree at merged main `7808e0f17daae18a2b38891c1d2d18147fba9459`, on darwin/arm64 with go1.27.1, git 2.56.0 and `KUBECONFIG=/dev/null`. The implementing merge is #1658 (136f670eb). The brief is `gate: model` and all four risk answers are `no`. Admission came from `deskdispatch --check-verifier`, which exited 0 and bound run `e827b1d3f2c5`, source `7808e0f17daa`, model claude-opus-5-5, tier any (attestation issue #2608).

Every Verify command was run verbatim from the repo root, with `TMPDIR` pointed at a scratch directory outside the worktree. Each row was then rerun as a diagnostic with `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1`. Both runs are recorded below. Since the 2026-10-02 record at e1d99484ffd9, the only commit under the qualgen directory is a11ac4167, which touches only the riskscore package. The re-fix join, regression link, report renderer and their tests are byte-identical. Row 1's first run downloaded module dependencies from the Go proxy into the module cache. That proxy is not cluster infrastructure.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 | As written: exit 0, no stdout (stderr held only `go: downloading` lines). Diagnostic rerun with global and system git config off: exit 0, no output | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 command verbatim (anchored run of the regression-of-link counted test, output captured, PASS line grepped) | exit 0 + named PASS line | As written: exit 0, `--- PASS: TestRefix_RegressionOfLink_Counted (0.00s)`. Diagnostic rerun: exit 0, same PASS line | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 command verbatim (anchored run of the same-defect-class counted test) | exit 0 + named PASS line | As written: exit 0, `--- PASS: TestRefix_SameDefectClass_Counted (0.00s)`. Diagnostic rerun: exit 0, same PASS line | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command verbatim (anchored run of the earlier-fix-after-inducer not-counted test), plus a mutation | exit 0 + named PASS line; mutant red | As written: exit 0, `--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted (0.00s)`. Diagnostic rerun: exit 0, same PASS line. Mutation in a scratch export (ordering check at refix.go line 371 forced true): exit 1, `refix_test.go:154: expected refix_count measured-zero (linked but wrong order), got {State:measured Value:1 Reason:}`, so the mutant is killed | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 command verbatim (anchored run of the no-linkage-configured could-not-measure test) | exit 0 + named PASS line | As written: exit 0, `--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure (0.00s)`. Diagnostic rerun: exit 0, same PASS line | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 command verbatim (anchored run of the report re-fix section renders test) | exit 0 + named PASS line | As written: exit 0, `--- PASS: TestReport_RefixSection_Renders (0.00s)`. Diagnostic rerun: exit 0, same PASS line | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 command verbatim: build qualgen into the temp dir, run `report --out ..`, then grep `Re-fix rate` | exit 0; section present | As written: exit 0, `## Re-fix rate (regression-suite effectiveness)`, followed by `not measured — populated once the defects table is traced (quality/06–07) and a RegressionLinkage adapter is configured. Never rendered as 0.` Diagnostic rerun: exit 0, and the rendered report is byte-identical to the first run | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness: `statusgen verifyrun` (v1.0.39) was run from the worktree root, and its table is below. Row 7 passes. Rows 1–6 are `check:ci`, and the witness re-executes them network-off through `unshare --net`. That is a Linux facility, so on this darwin host the witness records them as could-not-run (#1800, open). They clear on a Linux runner.

Further checks, run in a scratch export of the qualgen tree and never in the verify worktree:

- **Concurrent-boundary mutant still survives.** Changing the strict "before" comparison at refix.go line 371 to "at or before" passes every Refix, Report and Regression test (exit 0). No test pins the brief's rule that a concurrent earlier fix is not a re-fix. The code (a strict comparison) is correct as written; only the test coverage of that boundary is missing. No issue for this gap was found on the tracker.
- **Open finding #1844 still reproduces at this sha.** The probe was a window whose only traced fix has no closed issue, with no `regression-of:` and an unconfigured class prefix. It returned `measured-zero` for rate, count and coverage, each with an empty reason. The brief's facts say could-not-measure. This lies outside every Verify row.
- **Frozen fields intact.** `regression-of` is present in the author-brief skill at lines 102, 467, 491 and 501. The implementing diff touches neither the fix-linkage nor the SZZ source.

- RISK-VALUE: DERIVED — `shaPattern` = `^[0-9a-fA-F]{7,40}$` @ qualgen/regressionlink.go:83 — 7 is git's default minimum abbreviation and 40 is a full SHA-1 hex id. A match is resolved only against the identified-fix set, and the ordering rule bounds it. The metric is report-only and reversible by an edit. A SHA-256 object-format repo's 64-hex ids would not classify; that is not reachable on SHA-1 repos.
- RISK-VALUE: N/A for the rest — the enumeration covered the implementing diff of the re-fix join, the regression link, the report renderer and the artifact table. It found no threshold, budget, alarm or timeout. The remaining literals are zero-length guards, the metric name (`MetricRefix` = "refix" @ qualgen/refix.go:31), and the path, trailer and ref regexes. The only comparator is the strict `Before` @ qualgen/refix.go:371, which matches "a later or concurrent E is not a re-fix". The trigger does not fire: all four risk answers are `no`, the item is not irreversible, and no risk-classed path is touched.

The brief is `gate: model` with all risk answers `no`, so a model sign-off is permitted. The status flip is held only on the environment witness for check:ci rows 1–6 (#1800).

**VERIFY: PASS** — all 7 rows pass by hand at 7808e0f17daa, as written and under the git-config-isolated diagnostic; witness 1/7, check:ci rows 1–6 need the Linux network-off witness (#1800); finding #1844 still open; concurrent-fix boundary not pinned by any test (unfiled)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 | sha256:65f316141cb7 | 2026-10-11 | assay-verifier-app[bot] @ 7808e0f17daa+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2608 run=e827b1d3f2c583e3af7f86118865caeaacfc3cafeba633b50212c6b600028ce2 source=7808e0f17daae18a2b38891c1d2d18147fba9459 brief=docs/streams/quality/brief-19-refix-metric.md model=claude-opus-5-5 tier=any
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at c690023a864b (all 7 rows pass by hand, twice; witness 1/7, check:ci rows 1–6 held on #1800)

Verification-Attestation: medici-finance/assay#2932 run=976772318bf610ce2fc0528bb8a7143623b32e95a5964b442b825e707ebaf215 source=c690023a864b1c1cfdd55c0a28e19d22e572048d brief=docs/streams/quality/brief-19-refix-metric.md model=claude-opus-5-5 tier=strong

The runner is not the implementer. It used an isolated, detached worktree at merged main `c690023a864b1c1cfdd55c0a28e19d22e572048d`, on darwin/arm64 with go1.27.2, git 2.56.0 and `KUBECONFIG=/dev/null`. Admission came from `deskdispatch --check-verifier` (v1.0.42), which exited 0 and bound run 976772318bf6 to this source, brief, model and tier. The implementing merge is #1658 (136f670eb). The brief is `gate: model` and all four risk answers are `no`. The author-brief skill still carries `regression-of` (lines 120, 485, 509, 519), so the pickup precondition holds. No commit has touched the qualgen directory since the 2026-10-11 record at 7808e0f17daa. Every Verify command ran from the repo root with `TMPDIR` pointed at a scratch directory outside the worktree. The commands ran twice: once with the test names filled in from the table, and once with each Command cell extracted character for character from the brief and passed to `bash -c`. Both passes gave the same exit codes and PASS lines. The two row 7 outputs were byte-identical (sha256 5980ff3afcd9). The worktree was clean (`git status --porcelain --ignored` empty) after every pass.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | pass exit=0 | No output apart from module download lines on stderr. Matched on the literal rerun | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | pass exit=0 | `--- PASS` line for Test Refix Regression Of Link Counted (0.00s). Matched on the literal rerun | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | pass exit=0 | `--- PASS` line for Test Refix Same Defect Class Counted (0.00s). Matched on the literal rerun | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | pass exit=0 | `--- PASS` line for Test Refix Earlier Fix After Inducer Not Counted (0.00s). Matched on the literal rerun. Mutation in a scratch export, with the ordering check at refix.go line 371 forced true: exit 1, `refix_test.go:154: expected refix_count measured-zero (linked but wrong order), got {State:measured Value:1 Reason:}`. The mutant was killed | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | pass exit=0 | `--- PASS` line for Test Refix No Linkage Configured Could Not Measure (0.00s). Matched on the literal rerun | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | pass exit=0 | `--- PASS` line for Test Report Refix Section Renders (0.01s, then 0.00s on the literal rerun) | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 | `## Re-fix rate (regression-suite effectiveness)`, then the report-only note, then `not measured — populated once the defects table is traced (quality/06–07) and a RegressionLinkage adapter is configured. Never rendered as 0.` The worktree stayed clean afterwards | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |

Execution witness: `statusgen verifyrun --dry-run` (v1.0.42) exited 2 with 1 of 7 rows proven. Row 7 passed (sha256 65f316141cb7). Rows 1–6 are `check:ci`. The witness re-runs them network-off through `unshare --net`, which only Linux provides, so on this darwin host they are could-not-check (#1800, open). They clear on a Linux runner. The dry-run wrote nothing to the brief.

Further checks, run in a scratch export of the qualgen tree and never in the verify worktree:

- **The concurrent-boundary mutant still survives.** Changing the strict "before" comparison at refix.go line 371 to "at or before" (`!et.After(...)`) still passes every Refix, Report and Regression test (exit 0). No test pins the brief's rule that a concurrent earlier fix is not a re-fix. The strict comparison in the code is correct. Only the test coverage of that boundary is missing. A read-only issue search found no issue filed for this gap.
- **Finding #1844 is still open.** No qualgen file has changed since the 2026-10-11 record, which reproduced it. This run did not probe it again.
- **Frozen fields are intact.** The implementing diff touches neither the fix-linkage source nor the SZZ source. No threshold, budget or alarm was added; the section is labelled report-only.

RISK-VALUE: The trigger does not fire. All four risk answers are `no`, the item is not irreversible, and the diff touches no risk-classed path. The enumeration below was done anyway, over the implementing diff #1658 (refix.go, regressionlink.go, report.go, artifacts.go).

- RISK-VALUE: DERIVED — `shaPattern` = `^[0-9a-fA-F]{7,40}$` @ qualgen/regressionlink.go:83. 7 is git's default minimum abbreviation, and 40 is the length of a full SHA-1 hex id. A match is resolved only against the identified-fix set, and the ordering rule bounds it. The metric is report-only, and an edit plus a redeploy reverses it. A SHA-256 object-format repo's 64-hex ids would not classify, but that case cannot arise on SHA-1.
- RISK-VALUE: N/A for the rest. The other literals are format bindings, identifiers or zero-guards, not thresholds:
  - `MetricRefix` = "refix" @ qualgen/refix.go:31
  - `issueRefPattern` / `repoIssueRefPattern` @ qualgen/regressionlink.go:81–82
  - `briefTrailerPattern` @ qualgen/regressionlink.go:66
  - the `!= 0` PR-number guards

  The only comparator is the strict `et.Before(earliestInducing)` @ qualgen/refix.go:371. It matches the brief's rule that "a later or concurrent E is not a re-fix". The enumeration found no threshold, budget, alarm or timeout.

The brief is `gate: model` with all risk answers `no`, so a model sign-off is permitted. The status flip is held only on the environment witness for check:ci rows 1–6 (#1800).

**VERIFY: PASS** — all 7 rows pass by hand at c690023a864b, both as filled in and as literally extracted. Witness 1/7: check:ci rows 1–6 need the Linux network-off witness (#1800). Finding #1844 is still open. The concurrent-fix boundary is still not pinned by any test (no issue filed).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 | sha256:65f316141cb7 | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3069 run=741cbf112072046859778387d223f19b69dfffe1293c022ea3388cdb5210488b source=a39ad90497c10cdc16413695c3ce9b74c2b098d6 brief=docs/streams/quality/brief-19-refix-metric.md model=claude-opus-5-5 tier=any

### Non-implementer verifier run — 2026-10-15 claude-opus-5-5 verifier at a39ad90497c1 (all 7 rows pass by hand under a network-denied sandbox; witness 1/7, check:ci rows 1–6 held on #1800)

The runner is not the implementer. It used an isolated, detached worktree at merged main `a39ad90497c10cdc16413695c3ce9b74c2b098d6`, on darwin/arm64 with go1.27.2 (`GOTOOLCHAIN=local`), git 2.56.0 and `KUBECONFIG=/dev/null`. Admission came from `deskdispatch --check-verifier` (v1.0.44), which exited 0 and bound run 741cbf112072 to this source, brief, model and tier (attestation issue #3069). The implementing merge is #1658 (136f670eb). The brief is `gate: model` and all four risk answers are `no`. The author-brief skill still carries `regression-of` (lines 120, 485, 509, 519), so the pickup precondition holds. Since the 2026-10-14 record at c690023a864b, the only commit under the qualgen directory is 715796ed2 (#2707), which changes only go.mod and go.sum (Go floor 1.25.0 to 1.27.0, go-git dependency bumps). The re-fix join, regression link, report renderer and their tests are byte-identical.

`statusgen verifyrun` (v1.0.44) ran first, on the clean tree, and appended the witness table above. It exited 2 with 1 of 7 rows proven. Row 7 passed (sha256 65f316141cb7). Rows 1–6 are `check:ci`, and the witness re-runs them through `unshare --net`, which only Linux provides, so on this darwin host they are could-not-check (#1800, open).

Every Verify row was then run by hand. Each Command cell was extracted character for character from the brief at the verified commit and passed to `bash -c` from the worktree root. Each ran under macOS `sandbox-exec` with the profile `(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))`. The runs used a throwaway HOME, the real Go build and module caches, and `TMPDIR` set to a scratch directory outside the worktree. The deny was proved inside the sandbox before the runs. A control fetch of the Go module proxy outside the sandbox returned HTTP 200. Inside the sandbox, the same fetch failed with curl exit 6 (could not resolve host), and a fetch by IP literal failed with curl exit 7 (could not connect). A loopback fetch from a local listener returned HTTP 200. Every module came from the local cache, so no row reached the network.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | pass exit=0 | No stdout and no stderr, under the network-denied sandbox | 2026-10-15 | claude-opus-5-5 verifier |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | pass exit=0 | `--- PASS: TestRefix_RegressionOfLink_Counted (0.00s)`; no FAIL line in the captured output | 2026-10-15 | claude-opus-5-5 verifier |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | pass exit=0 | `--- PASS: TestRefix_SameDefectClass_Counted (0.00s)`; no FAIL line in the captured output | 2026-10-15 | claude-opus-5-5 verifier |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | pass exit=0 | `--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted (0.00s)`. Mutation in a scratch export outside the worktree, with the ordering check at refix.go line 371 forced true: the same row command, sandboxed, exited 1 with `refix_test.go:154: expected refix_count measured-zero (linked but wrong order), got {State:measured Value:1 Reason:}` and `--- FAIL`, so the mutant was killed. After the file was restored byte-identical to the commit, the row exited 0 with the PASS line again | 2026-10-15 | claude-opus-5-5 verifier |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | pass exit=0 | `--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure (0.00s)`; no FAIL line in the captured output | 2026-10-15 | claude-opus-5-5 verifier |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | pass exit=0 | `--- PASS: TestReport_RefixSection_Renders (0.00s)`; no FAIL line in the captured output | 2026-10-15 | claude-opus-5-5 verifier |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 | `## Re-fix rate (regression-suite effectiveness)`, then the report-only note, then `not measured — populated once the defects table is traced (quality/06–07) and a RegressionLinkage adapter is configured. Never rendered as 0.` The rendered report hashes to sha256 5980ff3afcd9, the same as the 2026-10-14 record | 2026-10-15 | claude-opus-5-5 verifier |

Further checks, run in a scratch export of the qualgen tree and never in the verify worktree:

- **The concurrent-boundary mutant still survives.** Changing the strict "before" comparison at refix.go line 371 to "at or before" (`!et.After(...)`) still passes every Refix, Report and Regression test under the sandbox (`ok`). No test pins the brief's rule that a concurrent earlier fix is not a re-fix. The strict comparison in the code is correct. Only the test coverage of that boundary is missing. A read-only issue search again found no issue filed for this gap.
- **Finding #1844 is still open.** The re-fix code is byte-identical to the 2026-10-11 record that reproduced it. This run did not probe it again.
- **Frozen fields are intact.** The implementing diff touches neither the fix-linkage source nor the SZZ source. No threshold, budget or alarm was added, and the section is labelled report-only.
- **The worktree's code is untouched.** `git diff` over the qualgen directory in the worktree is empty, and the only changed file is this brief.

RISK-VALUE: The trigger does not fire. All four risk answers are `no`, the item is not irreversible, and the diff touches no risk-classed path. The enumeration below was done anyway, over the implementing diff #1658 (refix.go, regressionlink.go, report.go, artifacts.go) and the one later qualgen commit, the go.mod bump in #2707.

- RISK-VALUE: DERIVED — `shaPattern` = `^[0-9a-fA-F]{7,40}$` @ qualgen/regressionlink.go:83. 7 is git's default minimum abbreviation, and 40 is the length of a full SHA-1 hex id. A match is resolved only against the identified-fix set, and the ordering rule bounds it. The metric is report-only, and an edit plus a redeploy reverses it. A SHA-256 object-format repo's 64-hex ids would not classify, but that case cannot arise on SHA-1.
- RISK-VALUE: N/A for the rest. The other literals are format bindings, identifiers or zero-guards, not thresholds:
  - `MetricRefix` = "refix" @ qualgen/refix.go:31
  - `issueRefPattern` / `repoIssueRefPattern` @ qualgen/regressionlink.go:81–82
  - `briefPathPattern` @ qualgen/regressionlink.go:60, `briefTrailerPattern` @ qualgen/regressionlink.go:66, `regressionOfPattern` @ qualgen/regressionlink.go:75
  - the `!= 0` PR-number guards
  - the Go floor `go 1.27.0` in the qualgen go.mod (#2707), a toolchain pin rather than a value the metric relies on

  The only comparator is the strict `et.Before(earliestInducing)` @ qualgen/refix.go:371. It matches the brief's rule that "a later or concurrent E is not a re-fix". The enumeration found no threshold, budget, alarm or timeout.

The brief is `gate: model` with all risk answers `no`, so a model sign-off is permitted. All six `check:ci` rows passed by hand under a network-denied sandbox. The verifyrun witness for them is still held on the Linux network-off facility (#1800).

**VERIFY: PASS** — all 7 rows pass by hand at a39ad90497c1, each Command cell run literally under a macOS sandbox with outbound network denied (deny proved). Witness 1/7: check:ci rows 1–6 need the Linux network-off witness (#1800). Finding #1844 is still open. The concurrent-fix boundary is still not pinned by any test (no issue filed).

### Non-implementer verifier run — 2026-10-19 claude-opus-5-5 verifier at 3add8a9ae315 (Linux network-off container witness, 7/7)

The runner is not the implementer. The witness below is a mechanical re-run of every Verify row by `statusgen verifyrun --sandbox container-netns`, not dry. It ran at merged main `3add8a9ae31542f02623379063049f3dcd5fdebc` in a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), `--network none`, sandbox=container-netns, the engine's default seccomp profile, `GOPROXY=off`, `GOTOOLCHAIN=local` and `KUBECONFIG=/dev/null`. It used a throwaway clone of the verify worktree at that commit, and statusgen and the desk tools were built inside the container from the same tree. Admission came from `deskdispatch --check-verifier`. It exited 0 before the run and again after it, binding run eb01579337a1 to this source, brief, model and tier (attestation issue #3507). The witness is offered under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Those rulings close a `gate: model` brief with no `risk: yes` when every Verify row passes on a working runner, here with 0 fail. qualgen has changed since the a39ad90497c1 record, in #3152 (Cobra/Viper migration), #3226 (offline defect tracing and measured reports, touching the report renderer) and #3294 (a test pinning the concurrent re-fix boundary). This witness at the current source is what covers those changes. An earlier container attempt today was discarded and never recorded: its module pre-download step listed no modules, so qualgen could not load under `GOPROXY=off`. That runner defect was fixed and the module cache warmed before this run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | pass exit=0 sandbox=container-netns | sha256:0d8963f66123 | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | pass exit=0 sandbox=container-netns | sha256:1ef19648e219 | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | pass exit=0 sandbox=container-netns | sha256:e21bfa879efb | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | pass exit=0 sandbox=container-netns | sha256:c4dbc2aaf6dc | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | pass exit=0 sandbox=container-netns | sha256:caa5ce191da3 | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 sandbox=container-netns | sha256:65f316141cb7 | 2026-10-19 | assay-verifier-app[bot] @ 3add8a9ae315 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3507 run=eb01579337a1559a9e186d8ccafcc3deab2c83e4249f6093debfe018e45bd672 source=3add8a9ae31542f02623379063049f3dcd5fdebc brief=docs/streams/quality/brief-19-refix-metric.md model=claude-opus-5-5 tier=strong

The substance is the by-hand Evidence block of 2026-10-15 at a39ad90497c1 (`a39ad90497c10cdc16413695c3ce9b74c2b098d6`), where all 7 rows passed by hand with 0 fail and the row 4 mutant was killed. This witness re-runs the same 7 Command cells network-off at the current source. Row 7's rendered output hashes to sha256 65f316141cb7, the same as the earlier darwin witnesses.

**VERIFY: PASS** — all 7 Verify rows pass with 0 fail in the Linux network-off container witness (container-netns) at 3add8a9ae315, which covers the qualgen changes since the 2026-10-15 by-hand record; the #1800 environment hold is cleared by this witness.

### Non-implementer verifier run — 2026-10-20 claude-opus-5-5 verifier at b73d90d774fc (receipt-bearing re-pass, Linux network-off container witness, 7/7)

The runner is not the implementer. This is a receipt-bearing re-pass at the current merged main `b73d90d774fca8aec8974ef410da63ebcb10eece`. The latest prior Evidence on main is the 2026-10-19 container witness at 3add8a9ae315 above, landed by medici-finance/assay#3513. The witness below is a mechanical re-run of every Verify row by `statusgen verifyrun --sandbox container-netns`, not dry. It ran in a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), `--network none`, sandbox=container-netns, the engine's default seccomp profile, `GOPROXY=off`, `GOTOOLCHAIN=local` and `KUBECONFIG=/dev/null`. It used a throwaway clone of the verify worktree at that commit, and statusgen and the desk tools were built inside the container from the same tree. Admission came from `deskdispatch --check-verifier`. It exited 0 before the run and again after it, binding run 34aaf7807110 to this source, brief, model and tier (attestation issue medici-finance/assay#3640). The witness is offered under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). No commit under the qualgen directory lands between 3add8a9ae315 and this source, and the author-brief skill still carries `regression-of`, so the pickup precondition holds.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_RegressionOfLink_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_RegressionOfLink_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_RegressionOfLink_Counted.out"` | pass exit=0 sandbox=container-netns | sha256:0d8963f66123 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_SameDefectClass_Counted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_SameDefectClass_Counted' "${TMPDIR:-/tmp}/q19-TestRefix_SameDefectClass_Counted.out"` | pass exit=0 sandbox=container-netns | sha256:1ef19648e219 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_EarlierFixAfterInducer_NotCounted$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out" 2>&1 && grep -F -e '--- PASS: TestRefix_EarlierFixAfterInducer_NotCounted' "${TMPDIR:-/tmp}/q19-TestRefix_EarlierFixAfterInducer_NotCounted.out"` | pass exit=0 sandbox=container-netns | sha256:e21bfa879efb | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestRefix_NoLinkageConfigured_CouldNotMeasure$' -v ./... > "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out" 2>&1 && grep -F -e '--- PASS: TestRefix_NoLinkageConfigured_CouldNotMeasure' "${TMPDIR:-/tmp}/q19-TestRefix_NoLinkageConfigured_CouldNotMeasure.out"` | pass exit=0 sandbox=container-netns | sha256:c4dbc2aaf6dc | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd qualgen && go test -count=1 -timeout 300s -run '^TestReport_RefixSection_Renders$' -v ./... > "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out" 2>&1 && grep -F -e '--- PASS: TestReport_RefixSection_Renders' "${TMPDIR:-/tmp}/q19-TestReport_RefixSection_Renders.out"` | pass exit=0 sandbox=container-netns | sha256:d6a3e04e9b69 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd qualgen && go build -o "${TMPDIR:-/tmp}/qualgen19" . && "${TMPDIR:-/tmp}/qualgen19" report --out .. > "${TMPDIR:-/tmp}/quality19.md" && grep -F 'Re-fix rate' "${TMPDIR:-/tmp}/quality19.md"` | pass exit=0 sandbox=container-netns | sha256:65f316141cb7 | 2026-10-20 | assay-verifier-app[bot] @ b73d90d774fc (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3640 run=34aaf7807110de95ad851cecfdd68ee2d44d654dfe1afe57909576856cde175c source=b73d90d774fca8aec8974ef410da63ebcb10eece brief=docs/streams/quality/brief-19-refix-metric.md model=claude-opus-5-5 tier=strong

The substance is the by-hand Evidence block of 2026-10-15 at a39ad90497c1 (`a39ad90497c10cdc16413695c3ce9b74c2b098d6`), where all 7 rows passed by hand with 0 fail and the row 4 mutant was killed. This witness re-runs the same 7 Command cells network-off at the current source. Rows 1–5 and 7 reproduce the 2026-10-19 output hashes exactly. Row 6's captured output hash differs (d6a3e04e9b69 against caa5ce191da3), but the row's own PASS-line assertion held with exit 0 and its test source is unchanged since that record.

**VERIFY: PASS** — all 7 Verify rows pass with 0 fail in the Linux network-off container witness (container-netns) at b73d90d774fc, and `statusgen verifyrun --check` reports 7 pass, 0 fail, 0 could-not-run.

## Review
Gate: model — all four risk answers are no. It is a read-only metric over committed
artifacts, rendered in a CI-written report, and it gates nothing. Reviewer confirms:
(1) the ordering rule is enforced (row 4 is a real mutation); (2) could-not-measure is
never rendered as 0 (row 5); (3) no threshold, budget or alarm was added; (4) no frozen
field was renamed. Record verdict + date in the stream README table.
