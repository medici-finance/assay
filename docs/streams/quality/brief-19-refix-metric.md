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
- `qualgen/report.go` — a `## Re-fix rate (regression-suite effectiveness)` section in
  `renderReport`.
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

## Review
Gate: model — all four risk answers are no. It is a read-only metric over committed
artifacts, rendered in a CI-written report, and it gates nothing. Reviewer confirms:
(1) the ordering rule is enforced (row 4 is a real mutation); (2) could-not-measure is
never rendered as 0 (row 5); (3) no threshold, budget or alarm was added; (4) no frozen
field was renamed. Record verdict + date in the stream README table.
