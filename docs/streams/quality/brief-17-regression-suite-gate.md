---
brief: assay:assay:quality:17
title: regression suite — TestRegression_ naming convention + count-can't-drop / vacuous-selector CI gate
why: >-
  A fixed bug leaves no durable, countable trace today: nothing names the test that pins it,
  so that test can be deleted or renamed away in an unrelated PR, and a `go test -run`
  selector that matches nothing exits 0 and reads green. This brief makes regression tests
  countable and hard to lose — one naming convention, and one CI job that goes red when a
  regression test fails, when the count drops against the base, or when the selector runs
  nothing.
wave: 0
depends: []
unblocks: ["quality/18"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1581]
schema: brief-v2
version: 1
authored: 2026-09-23 by assay-worker session (issue #1581, parts 1–2 of 4)
sources:
  - "issue #1581 — regression suite: parts 1 (naming convention) and 2 (CI job: fail on a failing regression test, a count DROP relative to main, or a selector matching zero tests in a module that previously had some); Done criterion: live in CI on this repo with fail-first evidence (a planted deletion and a planted vacuous selector each go red)"
  - "issue #1580 — the paired skills change (fix the defect CLASS: class-guard Verify row + `regression-of:`); it makes regression tests get written, this brief makes them countable and un-deletable. #1580 is delivered issue-only (no brief), so the pairing is recorded here, not as a depends: edge"
  - "issue #1573 / PR #1577 — the worked example: a token-path defect fixed at one call site re-entered through another, hidden by a shared test stub; its unstubbed test is the suite's seed member"
  - "docs/test-policy.md §Regression floor — the invariant this job makes mechanical: a merged change may not reduce the set of asserted behaviours"
  - "freshness-checked 2026-09-23 @ 5ee5ccb39 — `git grep -n TestRegression -- '*.go'` returns nothing; no regression-suite job in .github/workflows/; quality stream carries briefs 01–16, 17 free; no open PR touches docs/streams/quality/"
exec-tier: strong
exec-tier-why: >-
  (b) correctness spans three artifacts that must agree — the documented prefix, the tool's
  constant and the workflow that runs it; (c) the defect this gate exists to stop (a vacuous
  green) is exactly the one a subtly wrong implementation passes its own tests with.
consumers:
  - "docs/test-policy.md: follow-up quality/17 (this brief; flips to fixed-here when the implementation adds the convention section)"
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up quality/17 (this brief; the fix-brief class-guard Verify row gains a pointer to the convention)"
  - ".github/workflows/regression-suite.yml: follow-up quality/17 (this brief; new workflow)"
  - "tools/desk/cmd/deskwt/roleinitgitlabcred_test.go: follow-up quality/17 (this brief; the seed rename)"
---

# Brief 17 — regression suite: `TestRegression_` naming convention + count-can't-drop / vacuous-selector CI gate

## Context

files:
- NEW `tools/regsuite/` (planned) — its own Go module (`go.mod`, module path
  `github.com/medici-finance/assay/tools/regsuite`), like every other tool under `tools/`.
  `main.go` (planned) with a `gate` subcommand; `gate.go` (planned) + `gate_test.go` (planned).
- NEW `tools/regsuite/testdata/*.txtar` (planned) — fixture Go modules stored as txtar
  archives and materialized into `t.TempDir()` at test time (see the fixture fact below).
- NEW `.github/workflows/regression-suite.yml` (planned) — the `regression-gate` job.
- `docs/test-policy.md` — new `## Regression suite` section under `## Regression floor`.
- `plugins/assay/skills/author-brief/SKILL.md` — a one-line pointer from the fix-brief
  class-guard Verify-row paragraph (added by #1580) to the new test-policy section.
- `tools/desk/cmd/deskwt/roleinitgitlabcred_test.go` — rename one existing test (seed).
- `changelog/<branch-slug>.md` — the per-PR fragment this repo enforces.

facts:
- **Convention (issue #1581 part 1):** every closed bug leaves a named regression test
  `TestRegression_<repo>_<issue>[_<Desc>]` — preferably the class guard #1580 asks for, at
  minimum the failing case. `<repo>` is the repository short name lowercased with every
  character outside `[a-z0-9]` removed (`example-service` → `exampleservice`), because a Go
  identifier cannot carry `-`; `<issue>` is the issue number, digits only. Shape regex:
  `^TestRegression_[a-z0-9]+_[0-9]+(_[A-Za-z0-9_]+)?$`.
- **Module discovery (checked 2026-09-23 @ 5ee5ccb39):** `git ls-files '*go.mod'` lists 18
  modules; the `build-test` job in `.github/workflows/ci.yml` iterates exactly that list.
  None sits under a `testdata/` directory today.
- **Why CI does not already run tests:** `ci.yml` `build-test` runs `go build` + `go vet` per
  module (full `go test` only for `tools/desk`), because some tests are structural guards
  that cannot pass in the published tree. The regression job runs ONLY `^TestRegression_`,
  so those guards never run under it — and so every regression test must be hermetic
  (unit tier in `docs/test-policy.md` §Tiers: no network, no clock, no filesystem beyond
  the checkout).
- **Go facts the gate relies on:** `go test -list '<regex>' ./...` prints matching
  top-level test names without running them. `go test -run '<regex>'` that matches
  nothing prints `testing: warning: no tests to run` and exits 0 — the vacuous pass.
  `go test -json` emits one event per test action (`run` / `pass` / `fail` / `skip`) with
  a `Test` field; a subtest's name contains `/`.
- **Zero regression tests exist today** (`git grep -n TestRegression -- '*.go'` → no output,
  2026-09-23 @ 5ee5ccb39). Seed member: `TestRoleInitGitLabReadsCustodyNeverGitHubMinter`
  in `tools/desk/cmd/deskwt/roleinitgitlabcred_test.go`, the unstubbed guard #1577 landed
  for #1573; it already runs green under `ci.yml`'s `tools/desk` `go test ./...`.
- **Fixture hazard:** a committed fixture `go.mod` would be picked up by `ci.yml`'s
  `git ls-files '*go.mod'` loop AND by this gate's own discovery, so a deliberately failing
  fixture test would redden the real job. Fixtures therefore live as txtar archives
  (`golang.org/x/tools/txtar`, or the equivalent few lines of parsing) and are written out
  to a temp dir per test; discovery additionally skips any path with a `testdata`
  segment.
- **Pairing with #1580:** #1580 adds the class-guard Verify row and `regression-of:` to the
  author-brief skill. Pickup precondition: `grep -n 'regression-of' plugins/assay/skills/author-brief/SKILL.md`
  prints at least one line. If it prints nothing, #1580 has not landed: do Task steps 1–5
  and report NEEDS_CONTEXT for step 6. Do not write #1580's content here.
- single-point-of-failure: the base-vs-head count is the one control against a quiet
  deletion. Two independent layers back it. (1) The vacuity check compares the static
  `-list` set against the RUNTIME `-json` events: a different signal from a different
  `go test` mode, so a selector or build-tag fault that hides tests from execution is
  caught even when the listed count holds. (2) The gate prints the NAMES that
  disappeared, and review reads them. A deletion is always a visible diff. The gate names
  it so a reviewer cannot miss it. Removing the job itself is a `.github/workflows/` diff,
  and making it a required check is a repository-settings act (see Ground rules).

layering: flat tool — bounded orchestration of `go test` subprocesses, no domain-core
extraction warranted. The one rule (the clean / count-drop / vacuous / could-not-check
verdict) is a pure function over per-module listed/executed counts, unit-tested without
running `go`. The subprocess layer is tested end to end on txtar fixtures (Task 2–3;
Verify 2–7), and on the real tree by Verify 8.

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- The workflow runs PR code, so it triggers on `pull_request` (never `pull_request_target`),
  declares `permissions: contents: read`, uses no secrets, and reuses the runner label and
  hand-installed Go toolchain steps of `ci.yml`'s `build-test` job verbatim.
- Making `regression-gate` a REQUIRED check is a repository-settings act outside this diff —
  a human's. Do not attempt it; say in the PR body that it is owed.
- A change under `.github/workflows/` needs a push credential allowed to write workflow
  files. If the push is refused for that reason, STOP and report it verbatim. Do not route
  around it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Scaffold `tools/regsuite` as its own module. The prefix `TestRegression_` (planned) is a Go
   constant. No flag, env var or workflow input can change what is COUNTED.
2. `regsuite gate --head <dir> --base <dir> [--module <relpath>]... [--selector <regex>]`:
   - discover modules per the fact above (skip `testdata` segments); `--module` narrows
     the set, which is for bounded local runs;
   - per module, in head AND base: `listed` = top-level names from
     `go test -list '^TestRegression_' ./...`;
   - per module, in head only: execute with `go test -count=1 -json -run <selector> ./...`
     (selector defaults to `^TestRegression_`; the override exists so the vacuous case can
     be planted) and collect top-level `pass`/`fail`/`skip` events for prefixed names;
   - verdicts, each printed with its module and names:
     - **fail (exit 1)** — any regression test `fail`;
     - **fail (exit 1), count drop** — total head `listed` < total base `listed`. Print
       both totals and the base names absent from head. The comparison is on COUNT, so a
       rename that keeps the prefix passes;
     - **fail (exit 1), vacuous** — a module whose head or base `listed` > 0 executes zero
       tests (`pass`+`fail` = 0), or whose executed set (`pass`+`fail`+`skip`) is a strict
       subset of its head `listed`;
     - **could-not-check (exit 2)** — a module fails to build or list, a `--base`/`--head`
       dir is unreadable, or the toolchain errors. Never exit 0;
     - **clean (exit 0)** — a per-module table: listed / executed / skipped. Zero tests
       everywhere in both trees is reported as measured-zero, not vacuous;
   - a name that fails the shape regex prints a NOTICE and does not change the exit code.
3. Tests + txtar fixtures, one per Verify row 2–7.
4. `.github/workflows/regression-suite.yml` (planned), job `regression-gate`: on `pull_request` the base
   is the PR base SHA; on `push` to `main` the base is the first parent. Check out with
   enough depth, `git worktree add "$RUNNER_TEMP/base" <sha>`, build `tools/regsuite`, run
   `regsuite gate --head . --base "$RUNNER_TEMP/base"`. The workflow passes no selector.
5. Seed: rename `TestRoleInitGitLabReadsCustodyNeverGitHubMinter` →
   `TestRegression_assay_1573_RoleInitGitLabReadsCustodyNeverGitHubMinter` (planned; body unchanged).
6. Docs: `docs/test-policy.md` `## Regression suite` — the convention and shape regex, the
   hermetic requirement, what the job fails on (the three causes + could-not-check), that a
   rename must keep the prefix, and that a genuinely correct deletion is stated in the PR
   and needs a human-reviewed exception. Add a one-line pointer to that section in the
   author-brief skill's class-guard Verify-row paragraph (precondition above).
7. Fail-first in CI: push one commit that removes the prefix from the seed test (planted
   deletion), let `regression-gate` go red, then revert it in the next commit. Quote both
   check-run results in the PR body under `## Fail-first`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/regsuite && go build ./... && go vet ./...` | exit 0 | check:ci |
| 2 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_PlantedDeletion_Red' -v ./...` | exit 0; the test asserts gate exit 1 on a base with 2 regression tests and a head with 1, and that the output names the missing test | check:ci +mutation |
| 3 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_PlantedVacuousSelector_Red' -v ./...` | exit 0; the test asserts gate exit 1 with `vacuous` in the output when `--selector` matches nothing in a module whose listed count is > 0 | check:ci +mutation |
| 4 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_FailingRegressionTest_Red' -v ./...` | exit 0; a fixture regression test that fails makes the gate exit 1 | check:ci +mutation |
| 5 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_RenameKeepsPrefix_Clean' -v ./...` | exit 0; negative control: base and head differ only by a prefixed rename, and the gate exits 0 (proves the count is compared, not the name set) | check:ci |
| 6 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_BrokenModule_CouldNotCheck' -v ./...` | exit 0; a head module that does not compile yields gate exit 2 with `could-not-check` in the output, never exit 0 | check:ci |
| 7 | `cd tools/regsuite && go test -count=1 -timeout 300s -run 'TestGate_TestdataModulesExcluded' -v ./...` | exit 0; a module under a `testdata/` path is not discovered | check:ci |
| 8 | `cd tools/regsuite && go build -o "${TMPDIR:-/tmp}/regsuite17" . && cd ../.. && "${TMPDIR:-/tmp}/regsuite17" gate --head . --base . --module tools/desk > "${TMPDIR:-/tmp}/regsuite17.out" && grep -F 'TestRegression_assay_1573_RoleInitGitLabReadsCustodyNeverGitHubMinter' "${TMPDIR:-/tmp}/regsuite17.out"` | exit 0; the real gate over this repo's `tools/desk` module exits 0 and names the seeded test among those EXECUTED (it was found and run, not only listed) | check +flow |
| 9 | `cd tools/regsuite && go test -count=1 -timeout 120s -run 'TestConvention_DocMatchesConstant' -v ./...` | exit 0; the test reads `../../docs/test-policy.md` and asserts it states the tool's prefix constant and shape regex byte-for-byte (drift guard between doc and code) | check:ci +dereference |
| 10 | `grep -n 'Regression suite' plugins/assay/skills/author-brief/SKILL.md` | exit 0; the class-guard Verify-row paragraph points at the test-policy section | check:ci |
| 11 | Read the PR's commit trail and the `regression-gate` check runs on it | the planted-deletion commit's `regression-gate` run is red with the seed test's name in its log, and the revert commit's run is green. This is the in-CI fail-first evidence the issue requires | gate:model |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model — all four risk answers are no. The job can block a merge only once a human
makes it a required check, and it is reversible either way (revert the workflow, or drop
it from the required set). It reads no secret, touches no customer data and writes
nothing but its own log.
The brief touches `.github/workflows/`, a security-path trigger. The answers stay no
because the addition is a NEW workflow with `permissions: contents: read`, triggered on
`pull_request` (never `pull_request_target`), with no secret and no write. It weakens no
existing control. The reviewer checks exactly that (item 4 below).
Reviewer confirms: (1) the prefix is a constant nothing
downstream can override, and the workflow passes no selector; (2) could-not-check never
exits 0 (row 6); (3) rows 2–4 are real mutations whose planted fault the test proves red;
(4) the workflow's trigger, permissions and runner posture match Ground rules. Record
verdict + date in the stream README table.
