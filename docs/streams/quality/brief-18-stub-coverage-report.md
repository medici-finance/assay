---
brief: assay:assay:quality:18
title: stub-coverage seam report — report-first list of test seams stubbed everywhere and exercised nowhere in production form
why: >-
  A test seam that every test replaces with a stub hides whatever its real implementation
  does wrong. #1573 escaped exactly that way: a shared test setup stubbed the token resolver
  for every test of the command, so the production path was never run and the bug
  re-entered through it. This brief measures that exposure. It is a report first and a gate
  later, so the rule is calibrated against real numbers before anything blocks a merge.
wave: 1
depends: ["quality/17"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v2
version: 1
authored: 2026-09-23 by assay-worker session (issue #1581, part 3 of 4)
sources:
  - "issue #1581 part 3 — stub-coverage rule: a seam tests replace with a stub (a package-level func var swapped in `_test.go`) must also be exercised through its PRODUCTION implementation by at least one test per forge or variant it branches on; start as a report listing the seams stubbed everywhere and exercised nowhere, gate only after two measurement windows. Done criterion: a planted stub-only seam goes red"
  - "issue #1573 / PR #1577 — the worked example (seam `roleTokenPath`, tools/desk/cmd/deskwt/roleinit.go)"
  - "issue #1580 — paired skills change (class-guard Verify row); pairing recorded here, not as a depends: edge (#1580 is issue-only)"
  - "docs/streams/quality/spec.md §11 item 4 — measure for ≥ 2 windows before any threshold gates anything (the stream's measure-before-threshold rule)"
  - "freshness-checked 2026-09-23 @ 5ee5ccb39 — no stub/seam coverage report exists in the tree; `roleTokenPath` body covered by the `./cmd/deskwt/` test run (coverprofile line `roleinit.go:437.62,440.2 2 1`)"
exec-tier: strong
exec-tier-why: >-
  (a) mapping a seam to its production body is a design call the facts only bound (func
  literal vs named in-package func vs external initializer); (c) a report that silently
  under-lists seams passes its own fixture tests while missing the real ones.
consumers:
  - "tools/regsuite/: follow-up quality/18 (this brief; adds the stubs subcommand beside quality/17's gate)"
  - ".github/workflows/regression-suite.yml: follow-up quality/18 (this brief; adds the report job)"
  - "docs/test-policy.md: follow-up quality/18 (this brief; states the stub-coverage rule and its report-first status)"
---

# Brief 18 — stub-coverage seam report (report-first)

## Context

files:
- NEW `tools/regsuite/stubs.go` (planned) + `stubs_test.go` (planned) — the `stubs`
  subcommand, in the module quality/17 creates (reuse its module discovery and
  `testdata`-skip).
- NEW `tools/regsuite/testdata/stubs-*.txtar` (planned) — fixture packages, materialized to a
  temp dir at test time (same txtar rule as quality/17: never a live committed `go.mod`).
- `.github/workflows/regression-suite.yml` (planned) — created by quality/17; add a `stub-coverage-report` job.
- `docs/test-policy.md` — a `### Stub-coverage rule` subsection under quality/17's
  `## Regression suite`.
- `changelog/<branch-slug>.md` — the per-PR fragment.

facts:
- **Seam (the issue's definition):** a package-level `var` of func type declared in a
  non-`_test.go` file and assigned in at least one `_test.go` file of the same package
  (`x = …`, including the save-then-restore pattern `old := x; x = stub; t.Cleanup(func()
  { x = old })`). Rough size today: `git grep -nE '^var [a-zA-Z_]+ += +func' -- '*.go' ':!*_test.go'`
  finds dozens of func-literal package vars (not all are stubbed). Named initializers
  (`var execCommand = exec.Command`) are seams too.
- **Production body:** for a func-literal initializer, the literal's statement blocks; for
  an initializer naming an in-package func, that func's blocks; for an initializer outside
  the package or module (`exec.Command`, a func in another module), there is no in-package
  body to measure, so the seam is `could-not-measure (external initializer)`.
- **Measurement:** `go test -count=1 -coverprofile` per package. A test that stubs a seam
  never executes its production body, so a covered block in that body means some test ran
  the production form. States per seam: `production-covered` (every block covered),
  `partial` (some blocks uncovered — list their line ranges; this is how the report sees
  an untested forge/variant branch INSIDE the body), `stub-only` (zero blocks covered —
  the #1573 shape), `could-not-measure` (external initializer, or the package's test run
  failed or did not build).
- **Limit, stated and not hidden:** "one test per forge or variant it branches on" is
  visible to coverage only when the branch is inside the seam's body. A variant chosen
  BEFORE the seam is called (a caller picks the forge, then calls the seam) is not visible
  to this report. The report states that limit in its header.
- **Worked example (checked 2026-09-23 @ 5ee5ccb39):** `roleTokenPath` is declared at
  `tools/desk/cmd/deskwt/roleinit.go` as a func literal and stubbed by the shared setup
  in `tools/desk/cmd/deskwt/deskwt_test.go` (`roleTokenPath = func(role, owner string) …`)
  — the #1573 escape. #1577 added `productionRoleTokenPath` (captured at package init) so
  unstubbed tests run the real body; the coverprofile of `./cmd/deskwt/` now shows that
  body covered. So the correct report entry for it today is `production-covered`.
- **Report-first, gate later:** the CI job never fails on findings. `--fail-on-findings`
  (exit 1 when any seam is `stub-only`) exists for this brief's fail-first evidence and for a
  later promotion. Promotion to a gate follows ≥ 2 measurement windows
  (spec §11 item 4) and is a SEPARATE later brief, not this one.
- **Cost:** a coverage run of every module runs every test, so the report job runs on
  `push` to `main` only (not per PR). A module whose tests cannot pass in the published
  tree (see quality/17's CI fact) shows its seams as `could-not-measure`. They are never
  dropped.
- single-point-of-failure: seam detection is the one control (a seam it fails to find is
  never reported). Two things back it: the planted-fixture test proves detection on the
  known shapes (func literal, named func, restore pattern), and the real-tree row
  (Verify 6) proves it finds a known seam in real code, not only in fixtures. Anything past
  that is review's call, by design: this is a report, not a gate.

layering: extends quality/17's flat tool with a second subcommand, no new component. Seam
detection (AST over parsed files) and the coverage join (seam body × coverprofile blocks)
are pure over their inputs and unit-tested on fixtures. Running `go test -coverprofile` is
the only effect (Task 1; Verify 2–5 on fixtures, Verify 6 on the real tree).

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- REPORT-ONLY in CI: the job must not pass `--fail-on-findings` and must not fail on
  findings. Turning it into a gate is out of scope.
- Same workflow posture as quality/17 (`permissions: contents: read`, no secrets, same
  runner and toolchain steps). If a workflow-file push is refused, STOP and report it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `regsuite stubs [--module <relpath>]... [--pkg <pattern>] [--json] [--fail-on-findings]`:
   parse each package's non-test and `_test.go` files with `go/parser` + `go/ast` (no
   type-checker needed for the shapes above). Find the seams, map each one to its production
   body, run `go test -count=1 -coverprofile` for the package, and join the covered blocks to
   the body. Output one line per seam: `<module>/<pkg>.<name>  <state>  <detail>`. Add a
   header naming the module set, the Go version and the variant-visibility limit.
   `--json` emits the same records.
2. Exit codes: 0 always in report mode (findings included); with `--fail-on-findings`, 1 when
   any seam is `stub-only`; 2 on a tool error (unreadable tree, no toolchain). A
   could-not-measure seam is listed, never exit 0 by omission.
3. Fixtures + tests, one per Verify row 2–5.
4. Workflow: job `stub-coverage-report`, `on: push` to `main` only. Build `tools/regsuite`,
   run `regsuite stubs` over every module, append the output to `$GITHUB_STEP_SUMMARY`, and
   upload the JSON as a workflow artifact (one per run; consecutive runs are the
   measurement windows).
5. Docs: the rule as the issue states it (per forge/variant), its report-first status, the
   four states, and the variant-visibility limit.
6. Fail-first: paste `TestStubs_PlantedStubOnlySeam_Flagged` (planned) failing against a stub of
   the classifier that returns `production-covered` for everything into the PR body under
   `## Fail-first`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/regsuite && go build ./... && go vet ./...` | exit 0 | check:ci |
| 2 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestStubs_PlantedStubOnlySeam_Flagged' -v ./...` | exit 0; a fixture seam that every test stubs is reported `stub-only`; in report mode the tool exits 0, and with `--fail-on-findings` it exits 1 (the planted stub-only seam goes red) | check:ci +mutation |
| 3 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestStubs_ProductionExercisedSeam_NotFlagged' -v ./...` | exit 0; negative control: a fixture seam stubbed in one test and run unstubbed in another is `production-covered`, and `--fail-on-findings` exits 0 | check:ci |
| 4 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestStubs_PartialVariant_ListsUncoveredBlock' -v ./...` | exit 0; a fixture seam whose body branches on two variants with only one tested is `partial`, and the uncovered branch's line range is named | check:ci |
| 5 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestStubs_FailedPackageRun_CouldNotMeasure' -v ./...` | exit 0; a fixture package whose tests fail yields `could-not-measure` for its seams, never `stub-only` or `production-covered` | check:ci |
| 6 | `cd tools/regsuite && go build -o "${TMPDIR:-/tmp}/regsuite18" . && cd ../.. && "${TMPDIR:-/tmp}/regsuite18" stubs --module tools/desk --pkg ./cmd/deskwt/ > "${TMPDIR:-/tmp}/stubs18.out" && grep -E 'roleTokenPath +production-covered' "${TMPDIR:-/tmp}/stubs18.out"` | exit 0; the real tree's #1573 seam is found and classified `production-covered` (post-#1577 truth, checked by coverprofile at authoring). A detector that misses the seam, or misreads its coverage, fails this row | check +dereference +flow |
| 7 | `grep -n 'Stub-coverage rule' docs/test-policy.md` | exit 0 | check:ci |
| 8 | Read `.github/workflows/regression-suite.yml` (planned) | `stub-coverage-report` triggers on push to `main` only, never passes `--fail-on-findings`, and publishes to the step summary + a workflow artifact | gate:model |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model — all four risk answers are no. The job is report-only (it cannot block a
merge), runs on `main` pushes with read-only permissions, and reads no secret. It
touches `.github/workflows/` (a security-path trigger) only by adding a job to quality/17's
read-only workflow and weakens no existing control, so the risk answers stay no. Reviewer
confirms: (1) CI never runs `--fail-on-findings` (row 8); (2) row 2 is a real mutation (the
planted stub-only seam flips the exit code); (3) could-not-measure is listed, never folded
into a pass (row 5); (4) the variant-visibility limit is stated in the report header and
the docs. Record verdict + date in the stream README table.
