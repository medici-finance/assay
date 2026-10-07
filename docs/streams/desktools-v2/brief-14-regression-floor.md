---
brief: assay:assay:desktools-v2:14
title: regression floor — the behavior the desk tools pass today, pinned as tests seeded from resolved issues, which desktools-v2 must keep green
why: >-
  desktools-v2 rewrites the forge access layer underneath every verb. The bugs it must not
  reintroduce are already known — they are the resolved issues: deskwt hardcoding /private/tmp
  (#656), deskclaim-ref silently dialing gitlab.com when go-git cannot read origin (#727),
  deskdispatch rejecting a Windows worktree home (#757), the GitLab backend stubbing
  ListOpenIssues (#1033), rotate-on-mint with no lock revoking the live PAT (#1034), the
  review board missing GitLab <slug> authors (#1056), compare needing both base and head
  (#1067), deskfile stamping labels on the merge request that shares the new issue's number
  (#1086), and the token-scoping fixes of #1145/#1146/#1223. Today those fixes exist only as
  diffs; nothing fails if v2 undoes one. This brief pins each resolved behavior as a test that
  passes on main TODAY and rides the existing go test ./... in PR CI, so every v2 change has
  to keep it green — the floor v1 already stands on, made load-bearing.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1836, 322, 643, 656, 687, 697, 708, 719, 727, 757, 772, 773, 786, 999, 1007, 1033, 1034, 1056, 1067, 1086, 1145, 1146, 1223]
schema: brief-v2
outcome: none
authored: 2026-09-29 by the desk, at the driver's direction alongside the #1836 scoping
sources:
  - "the driver's direction of 2026-09-29: a regression test the desk tools pass today and desktools-v2 must be able to pass, with the already-resolved issues integrated"
  - "closed-issue harvest of 2026-09-29 (gh issue list --state closed --search windows / --search gitlab on this repo): the starter set in deliverable 1; all 22 confirmed closed on 2026-09-30. The same query, with an explicit --limit, is re-run at implementation to catch anything resolved since"
  - "#1836 §2 — #1145, #1146 and #1223 confirmed closed; the token-scoping behavior they fixed is part of the floor (assay:assay:desktools-v2:13 drops them from brief 06's basis and routes that behavior here)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the package-by-package map of forge-access sites; used to place each test in its owning package. It does not map seed issues — that mapping is this brief's MANIFEST"
exec-tier: strong
exec-tier-why: >-
  (a) each regression test must pin the FIX, not a neighbor — a test that passes both before
  and after the fixing commit is decoration, and telling the two apart is the judgment this
  brief is made of; (b) several seeds are concurrency or transport behaviors (rotate-on-mint
  locking, the gitlab.com dial) where a weak test passes vacuously; (c) the red-at-parent
  confirmation is per-test evidence work a cheap tier cuts corners on exactly where it matters.
domain: complicated
consumers:
  - "tools/desk (NEW internal/regression/ — planned: MANIFEST.md + TestRegressionManifest + the desk-half TestReg<N> tests): follow-up desktools-v2/14 (this brief)"
  - "statusgen (the statusgen-half TestReg<N> tests for #999 and #1007): follow-up desktools-v2/14 (this brief)"
  - "docs/streams/desktools-v2/README.md (the `## Regression floor` section): follow-up desktools-v2/14 (this brief)"
  - "every other desktools-v2 brief — the floor their PRs must keep green through the existing go test ./... in PR CI: out-of-scope (no edit to those briefs; the rule lives in the stream README where their implementers read it)"
  - "desktools-v2/12 (the platform compatibility suite): out-of-scope (sibling brief — 14 pins today's resolved behavior, 12 extends coverage to Windows/GitLab semantics; the two suites share no cases)"
version: 4
id: 4f2e1b7a-9c3d-4e5f-8a6b-1d2c3b4a5967
---

# Brief 14 — regression floor

## Context

files:
- NEW `tools/desk/internal/regression/` (planned) — `MANIFEST.md`, the manifest test, its `testdata/` fixture, and desk-half regression tests.
- `statusgen/` — NEW statusgen-half regression tests (tests only).
- `docs/streams/desktools-v2/README.md` — the `## Regression floor` section.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

v2's premise is that the flows have solidified. What has solidified is recorded in the
resolved-issue register: every closed Windows/GitLab/desk-tools bug is a behavior someone
already fixed once. A rewrite that silently un-fixes one is the most expensive defect class
there is — the report comes back as "this worked before". The desk tools pass every one of
these behaviors today; the suite this brief lands proves it and makes any v2 PR that breaks
one go red.

**Scope discipline.** Seeds are issues whose resolution was a CODE fix. Decision issues,
verify-gate trackers and docs-only closes are not seeds. Where a seed's fix is already covered
by a test that genuinely pins it (would fail at the fixing commit's parent), the manifest
points at that test instead of adding a duplicate.

**Two modules.** The seeds span both Go modules in this repo: most are desk-tools fixes
(`tools/desk/...`), while #999 and #1007 are statusgen behaviors (`statusgen/...`, its own
module). The manifest names the owning package per row, and the Verify rows run each half in
its own module. #786 is a shell-hardening fix in the fleet-provisioning script; its row pins
the behavior through a Go test that runs the script against a fake, or it is dropped with that
reason.

## Deliverables

1. **Harvest and manifest.** Re-run the closed-issue harvest (`gh issue list --state closed
   --search windows --limit 500` and the same with `--search gitlab`, on this repo, plus the
   token-scoping closes named in #1836), filter to code fixes, and land
   `tools/desk/internal/regression/MANIFEST.md` (planned) with two tables:
   - the seed table, one row per pinned issue:
     `| #<N> | behavior pinned | owning package | test | fixing commit |` — the package column
     is a repo-relative path beginning `tools/desk/` or `statusgen/`, and the test column names
     either a new `TestReg<N><Behavior>` (for example TestReg1034RotateLock; at most 31
     characters, so no name trips the secret scanners' long-identifier heuristics) or an existing
     test that genuinely pins the fix;
   - a `## Dropped` section, one row per starter issue that is not a seed:
     `| #<N> | <reason> |`, the reason never empty.

   Starter set from the 2026-09-29 harvest: #322, #643, #656, #687, #697, #708, #719, #727,
   #757, #772, #773, #786, #999, #1007, #1033, #1034, #1056, #1067, #1086, #1145, #1146,
   #1223. Known drops: **#719** (docs-only close, not a code fix) and **#322** (a cross-compile
   break — its class is held by desktools-v2/13's T1 cross-compile gate, not by a behavior
   test). That leaves 20 expected seed rows.
2. **The manifest test.** `TestRegressionManifest` (planned) in `tools/desk/internal/regression` parses
   MANIFEST.md and fails unless: every starter issue is either a seed row or a `## Dropped` row
   with a non-empty reason; no issue appears in both; every seed row's fixing commit is a
   7–40 character hex sha; every new test name matches `^TestReg[0-9]+[A-Z][A-Za-z0-9]*$` and is
   at most 31 characters; and every named test function exists as `func <Name>(` in a
   `*_test.go` file under the row's package directory (resolved from the repo root, so both
   modules are checked). It carries a positive-control fixture under `testdata/` — a manifest
   with one starter issue missing and one seed naming a non-existent test — and asserts the
   parser rejects it with both faults named, so the manifest test cannot pass vacuously.
3. **The tests.** One `TestReg<N><Behavior>` (planned) per seed row that does not point
   at an existing test, in the row's owning package, as a top-level test (its PASS line is what
   rows 2 and 3 count; subtests are allowed and are not counted), asserting the FIXED behavior against
   fakes/`httptest` — never a live forge, never a network call. Each test's doc comment cites
   the issue and the fixing commit.
4. **Red-at-parent confirmation, recorded.** For every seed row, run its test at the parent of
   the fixing commit and record the failing run in this brief's Evidence table, one row per
   seed in the form `| #<N> | <test> | <fixing sha> | <parent sha> — <failure line> |`. A test
   that passes at the parent pins nothing and is reworked, or moved to `## Dropped` with the
   reason.
5. **The floor rule, where v2 reads it.** A `## Regression floor` section in the desktools-v2
   stream README (prose, outside the generated table): the suite rides the existing
   `go test ./...` in PR CI; a desktools-v2 PR that turns a regression test red may not delete
   or weaken the test — it ports it, and the PR description names the behavior change that
   forced the port.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check +dereference +flow | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestRegressionManifest$' -count=1 -timeout 30s -v ./internal/regression/ > "$d/r1.out" 2>&1 && grep -E -e '^--- PASS: TestRegressionManifest \(' "$d/r1.out"` | prints the top-level PASS line (exit 0) — every starter issue is a seed row or a reasoned drop, every named test exists in its package, and the positive-control fixture is rejected. The `&&` keeps `go test`'s status and the grep is anchored at column 0, so a failing positive-control subtest cannot hide behind a passing one; exits 1 on today's tree. The go tool starts through `floor-go.sh`, the floor's git-environment scrub, like every row here that runs `go` |
| 2 | check | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestReg[0-9]' -count=1 -timeout 10m -v ./internal/regression ./cmd/deskclaim-ref > "$d/r2.out" 2>&1 && grep -q -F -e '--- PASS' "$d/r2.out" && n=$(grep -c -E -e '^--- PASS: TestReg[0-9]' "$d/r2.out") && m=$(grep -c -E -e '^[\|] #[0-9]+ [\|][^\|]*[\|] tools/desk/[^\|]*[\|] TestReg[0-9]' internal/regression/MANIFEST.md) && echo "pass=$n manifest=$m" && test "$n" -ge 1 && test "$n" -eq "$m"` | prints `pass=<n> manifest=<m>` with equal counts, at least 1, and exits 0 — `go test` passed (the `&&` keeps its status) and every `tools/desk/` seed whose test is a new `TestReg<N>` (planned) passes as a top-level test (the count grep is anchored at column 0, so subtest PASS lines are not counted): the desk half of the floor passes TODAY. A failing test, no test run, or a count mismatch exits 1. `go` starts through `floor-go.sh`. The 10m bound sits above the two wrapped shell suites' finite 4m budgets (`shellBudget`), so a hung suite fails on its own deadline while a loaded host still passes |
| 3 | check | `cd statusgen && f=../tools/desk/internal/regression/floor-go.sh && bash "$f" test -run '^TestConsumedFragmentIndexShallowCloneIsCouldNotCheck$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestRelPath_HandlesBothSeparatorStyles$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestScanIssueReadUsesNativeForgeNotGH$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestRunWitnesses_WSLLauncher_BootstrapIsCouldNotRun$' -count=1 -timeout 30s -v .` | exit 0 with four top-level PASS lines — all statusgen seeds reuse existing tests; an anchored run replaces the empty new-test selection, including the additional shell-bootstrap seed from the refreshed harvest. Each `go test` starts through `floor-go.sh`: these are reused tests whose fixtures run git with the environment they inherit, so a caller's exported `GIT_DIR` would otherwise send their commits and identity config to the repository it names |
| 4 | check | `m=$(awk 'BEGIN{FS=sprintf("%c",124)} /^## Dropped/{exit} $2 ~ /^ #[0-9]+ $/ {n++} END{print n+0}' tools/desk/internal/regression/MANIFEST.md); e=$(awk 'BEGIN{FS=sprintf("%c",124)} $2 ~ /^ #[0-9]+ $/ && $3 ~ /^ Test[A-Za-z0-9_]+ $/ && $4 ~ /^ [0-9a-f]+ $/ && $5 ~ /^ [0-9a-f]+ / {n++} END{print n+0}' docs/streams/desktools-v2/brief-14-regression-floor.md); echo "manifest=$m evidence=$e"; test "$m" -eq "$e" && test "$m" -ge 18` | prints equal counts and exits 0 — every seed has its Evidence row; the 18-row minimum remains intact. Field separation uses ASCII 124 to avoid a shell pipe inside a Markdown table cell |
| 5 | check | `grep -q -E -e '^## Regression floor$' docs/streams/desktools-v2/README.md` | exit 0 — the floor rule is where a v2 implementer reads it |
| 6 | check | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |
| 7 | check +flow | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; `seed passes=26` — every named seed passes in its owning module; the runner rejects failed and empty selections |
| 8 | check +mutation | `g=tools/desk/internal/regression/testdata/mutate_guard.py && python3 "$g" manifest > /dev/null && python3 "$g" ci > /dev/null && python3 "$g" directories > /dev/null && python3 "$g" deadline > /dev/null && python3 "$g" gitenv > /dev/null && python3 "$g" execenv > /dev/null && python3 "$g" runnerenv > /dev/null && echo controls=7` | exit 0, printing `controls=7` — each mode breaks one guard in a compiler-valid way (manifest validation, CI registry, descendant CI coverage, shell deadline, `FixtureEnv` passing the caller's `GIT_*` through, the fixture-exec class guard's matcher, the floor runner's `GIT_*` scrub in `floor-go.sh`), requires the inner `go test` to fail with a `--- FAIL:` line, and restores the file. A guard that stays green under its mutation makes the mode, and so the row, end non-zero |
| 9 | check +flow | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestFloor[RG]' -count=1 -timeout 180s -v ./internal/regression/ > "$d/r9.out" 2>&1 && n=$(grep -c -E -e '^--- PASS: TestFloor[RG][A-Za-z]+ [(]' "$d/r9.out") && echo "pass=$n" && test "$n" -eq 2` | prints `pass=2` and exits 0 — the floor runner, run over a planted row while `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE` name a second repository, leaves that repository byte-unchanged, and every floor script starts the go tool only through `floor-go.sh` (planted spawns are named). `-run '^TestFloor[RG]'` selects exactly those two tests without a pipe character in the cell. `go` starts through `floor-go.sh` |

## DoD

- All nine Verify rows pass on Linux/macOS without a network connection to any forge. Every
  row that runs the go tool starts it through `floor-go.sh`, directly or through the runner. The
  whole `go test ./...` suite, floor included, is PR CI's job and is not re-run here.
- Every starter-set issue is either a manifest seed row with a passing test and a
  red-at-parent record, or a `## Dropped` row with its reason (docs-only, decision, held by
  another gate — named, or already pinned by an existing test — named).
- No production code changes except test helpers a seed genuinely requires, each named in the
  PR description.

## Evidence

### Implementer regression receipt — 2026-10-02

Targeted package runs at the implementation base passed all 26 named seed behaviors.
Every parent below is the fixing commit's first parent; only fixture tests and
compatibility adapters were transplanted. No production fix was carried back.
Compilation errors encountered during initial transplantation were discarded and
are not evidence. Fixture paths in the excerpt below are redacted as `[fixture]`.

| Issue | Test | Fixing commit | Parent — observed failing assertion |
|---|---|---|---|
| #643 | TestCommitIdentityGitLabSessionEmail | 0276ce0a5 | f5a969f34 — gitlab entry + configured session email = could-not-check, want clean (the roster binds no App to role worker) |
| #656 | TestAddOnWindowsCreatesUnderPortablePrefix | b31be9266 | 3a25886c9 — expected the worktree under the portable prefix [fixture] stat [fixture] no such file or directory |
| #687 | TestDeskfileFilesOnGitLabThroughBackend | b7a82c025 | dd72e0486 — new on a GitLab-configured repo should FILE (exit 0) now the backend serves GitLab, got 5 |
| #697 | TestForgeGitlabTierErrors | 3040653c4 | 92e81315e — a 404 on the CE-absent project approval route must degrade, not refuse: could-not-check: GET /projects/medici-finance%2Fassay/approvals — not visible (HTTP 404): the object does not exist, or the token cannot see it: forge API GET /projects/medici-finance%2Fassay/approvals returned HTTP 404 |
| #708 | TestDispatchFallsBackToTheGoClaimBinaryWhenNoScriptIsPresent | bf168ab2f | 8bd5ffcc5 — green-field dispatch rc = 6, want 0 — the Go fallback did not carry the claim |
| #727 | TestReg727WorktreeOrigin | 63303b19c | efee10466 — linked worktree origin="", want "https://gitlab.example.invalid/team/repo.git"; never guess a SaaS host |
| #757 | TestDispatchAcceptsDeskwtWindowsWorktreeHome | 03cb769a4 | 8d799c6bc — dispatch with deskwt's Windows worktree home rc = 6, want 0 — the #757 refusal |
| #772 | TestAppTokenGitLabRepoSkipsGitHubMint | 53d6cbe7e | f4693f1df — checkAppToken minted a GitHub App token for a GitLab-resolved repo — the pre-772 bug (medici-finance/assay#772): the GitLab lane must never touch the GitHub App mint |
| #773 | TestReviewKitHeadFetchIsGitLabShapedOnAGitLabRepo | f4693f1df | 9541b2fb5 — the GitLab review Assignment must fetch the MR head at merge-requests/1/head: |
| #786 | TestReg786FleetHardening | 643637114 | e89d0eb02 — FAIL T10 the owner PAT must never reach curl argv (found the sentinel in the argv log); FAIL T11 a transport failure is recorded via record_failure |
| #999 | TestConsumedFragmentIndexShallowCloneIsCouldNotCheck | dc410608c | 64e63388a — shallow clone: got checked=true (exempt=false), want checked=false (could-not-check) — a shallow clone's truncated git log must never be read as a definitive answer |
| #1007 | TestRelPath_HandlesBothSeparatorStyles | 98960cedb | d97abb672 — relPath("C:\\repo\\docs\\streams\\test-stream") = "C:\\repo\\docs\\streams\\test-stream", want "docs/streams/test-stream" |
| #1033 | TestForgeGitlabGolden | 7a7cac919 | 5610bdea1 — golden mismatch for "list_open_issues" |
| #1034 | TestGitLabConcurrentMintsForOneRoleDoNotRevokeTheLivePAT | e428134c6 | 5a03bf5e1 — mint 0 failed: rotate gitlab token for role worker: rotate HTTP 401: {"message":"401 Unauthorized - Token was revoked"} |
| #1056 | TestBoardAcceptsGitLabReviewerUsername | dac811073 | a658a48db — isReviewerBot("gl-reviewer") = false — the rostered GitLab service account was not recognised as the reviewer, so its approvals are invisible to the queue |
| #1067 | TestUnpinnedReviewSHADegradesRowNotSweep | e1742b9f8 | 2c67b34f9 — classifyPR returned compare needs both base and head — one merge request whose verdict the forge could not pin to a head must not fail the whole sweep; every other row in the repo is lost with it and the desk sees an empty board |
| #1086 | TestLabelTargetRoutes | 980ad8cf0 | 7288a6d19 — gitlab: removed [], want [to:desk] |
| #1145 | TestReg1145ShimCredential | f84dde307 | f1e8fa522 — FAIL verb's gh subprocess authenticated (rc=0) |
| #1146 | TestDecisionChildReceivesTheMintedRoleTokenInItsEnvironment | 8467637e4 | eb7520558 — the decision script saw GH_TOKEN="", want the dispatching role's minted token — its forge-CLI calls would have run on the ambient login (argv: ensure spec.md --repo medici-finance/assay --at start) |
| #1223 | TestScanIssueReadUsesNativeForgeNotGH | f19569abc | d9d94a3ec — ghIssueLister(example-org/alpha) errored with a working deskread and no working gh: gh issue list --repo example-org/alpha: exit status 1 gh: stubbed to fail (native-read test) — the read still depends on the gh shell-out |
| #1203 | TestGitLabWorkerDispatchClaimUsesGitLabPATNotTheAppMinter | ccaa5d566 | 6b0038956 — the GitHub App minter was called 1 time(s) on a GitLab-served worker dispatch: [{desk example-org/example-project}] |
| #1411 | TestGitLabPipelineByShaFallbackReachesGate | e67e20f2f | 94ffcd490 — commit.last_pipeline is empty but a GREEN pipeline exists at the head SHA via pipelines?sha=; the flip gate still reports required checks [pipeline] as missing (rollup carries []) — #1411: ChecksAtHead did not fall back to the by-SHA read |
| #1415 | TestGitLabCreateDraftChangeRetriesTransientMissingBranch | 401d545a1 | 54b6da23d — expected the transient missing-branch 400 to be retried to success, got error: could-not-check: POST /projects/medici-finance%2Fassay/merge_requests — HTTP 400: forge API POST /projects/medici-finance%2Fassay/merge_requests returned HTTP 400 |
| #1418 | TestRunWitnesses_WSLLauncher_BootstrapIsCouldNotRun | 5cc0c0d1b | 07c762ad4 — row 1: state "fail", want "could-not-run" — a shell that never bootstrapped is could-not-run, not a product-check failure |
| #1490 | TestVerifierDispatchStampsWorktreeIdentityNotTheSharedCheckouts | 8e838df89 | b8e7403f8 — git config --worktree --get user.email: exit status 1 |
| #1864 | TestNoAPIHostLiteral | 45d4f34a5 | dd9f8b630 — main.go:121:21: string literal restates api.github.com; source it from deskkit.GitHubAPIBase |

Counterfactual compatibility details are recorded in
`tools/desk/internal/regression/PARENT-PROOF.md`. #727 uses the new real linked-worktree
fixture unchanged on both trees, with native-git fallback absent after fixture creation.
The manifest guard was also run against a planted second missing seed and missing test;
its failure named both faults. The committed `testdata/incomplete.md` keeps that control
in every run.

CI scope hold: the desk floor is reached by `.github/workflows/ci.yml`'s existing full-test
case. The statusgen half is build/vet-only until the maintainer applies
`ci/staged-patches/desktools-v2-14-statusgen-tests.patch`; `git apply --check` passed.
The correction is recorded on issue #1836. No live workflow was edited. This receipt is
implementation evidence, not independent verification or a claim that the staged CI
change is live.

### 2026-10-03 desk dispatch — 9/9 Verify rows pass by hand on merged main b3677d6da; execution witness misreads row 8 Expect (exit 1 parsed from prose)

Run against merged main b3677d6da (forge main has since advanced one commit, 8291853224, a STATUS.md regen only; no file under this brief's scope changed). Offline, throwaway HOME, no GIT_DIR or token variables exported. Long test names and shas are abbreviated in this block.

| # | Command | Expected | Observed | Date / Runner |
| --- | --- | --- | --- | --- |
| 1 | Verify row 1 as written (floor-go.sh go test, run TestRegressionManifest in the regression package, anchored PASS grep) | top-level PASS line, exit 0 | exit 0; "--- PASS: TestRegressionManifest (0.70s)" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (floor-go.sh go test, run TestReg[0-9] over the regression package and cmd/deskclaim-ref, count vs MANIFEST) | pass=n manifest=m, equal, at least 1, exit 0 | exit 0; "pass=3 manifest=3" (TestReg786FleetHardening 45.52s, TestReg1145ShimCredential 3.89s, TestReg727WorktreeOrigin 0.09s) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (four anchored statusgen go test runs through floor-go.sh) | exit 0, four top-level PASS lines | exit 0; four PASS lines: the shallow-clone consumed-fragment test (0.15s), the relPath separator-style test (0.00s), the native-forge scan-read test (0.47s), the WSL-launcher bootstrap witness test (0.23s); four "ok" package lines | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (awk seed-row count in MANIFEST vs Evidence-row count in this brief) | equal counts, at least 18, exit 0 | exit 0; "manifest=26 evidence=26" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | grep for the Regression floor heading in the desktools-v2 README | exit 0 | exit 0; heading present, section states the floor-port rule and the statusgen CI enforcement hold | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | statusgen --consumers --root . | exit 0, no routing claim disproved | exit 0; "consumers: no brief files in the diff against b3677d6da ... nothing to corroborate" (vacuous on merged main). Supplemental: --base set to the parent of the #2004 merge, --brief desktools-v2/14: exit 0, "0 corroborated, 0 disproved, 5 unchecked" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | bash check-floor.sh in the regression package | exit 0, seed passes=26 | exit 0; "seed passes=26"; 26 top-level PASS lines, 0 FAIL lines | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (mutate_guard.py in seven modes: manifest, ci, directories, deadline, gitenv, execenv, runnerenv) | prints controls=7, exit 0 | exit 0; "controls=7"; worktree git status clean before and after (every mutated file restored, no backup left) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (floor-go.sh go test, run TestFloor[RG] in the regression package, anchored PASS count) | pass=2, exit 0 | exit 0; "pass=2" (TestFloorRunnerGitIsolation 0.34s, TestFloorGoChokePoint 0.00s) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (statusgen v1.0.31 verifyrun --dry-run, throwaway HOME, at b3677d6da): exit 1. Rows 1-7 and 9 pass (output hashes ab168400e67c, ae6c3e1d0574, 6d350a7fc9d7, d24141988b50, e3b0c44298fc, 2a894b70fdaf, 10966703d678, e79a64120720). Row 8 scored fail: "exit 0, expected 1" (hash 4c85b0ebdf92). Cause: the Expect cell reads "prints controls=7 and exits 0 ... requires go test to exit 1"; the witness exit parser does not match the verb form "exits 0" and takes the later prose "exit 1" (a statement about the mutated inner go test) as the row's required exit. The command's real behavior matches the Expect cell's intent. Same wording class as #1702 and the lint proposal #1906. Fix is a one-word Expect re-author for row 8 (lead with "exit 0", or quote the inner exit as a code span), not a code change.

Red-at-parent spot check (independent of the implementer receipt, 2 of 26 seeds): #1007 (the relPath separator-style test) at d97abb672 with the fixing commit's test file overlaid: exit 1, "relPath of a C: backslash path = the path unchanged, want docs/streams/test-stream" (quote abbreviated); passes at 98960cedb. #727 TestReg727WorktreeOrigin at efee10466 with the new test and the fixture-env helper overlaid: exit 1, "linked worktree origin=, want the fixture GitLab remote; never guess a SaaS host" (URL abbreviated). Both match the implementer's receipt rows.

Risk: gate model; risk metadata present, all four fields no; not irreversible; diff (#2004) is tests, one test-only helper, docs, a changelog fragment and a staged CI patch, with no production code. Enumerated literals: commit-sha length 7 to 40 (manifest_test.go:17), new TestReg name ceiling 31 (manifest_test.go:64), seed-row floor 18 (Verify row 4), shell fixture deadline 60s (shell_test.go:23), floor runner per-test timeout 90s (check-floor.sh:31), git-isolation test deadline 120s (gitenv_test.go:256), mutation control timeouts 60s and 90s (mutate_guard.py:85, :87). All are reversible test knobs; none is irreversible.
RISK-VALUE: N/A — no irreversible literal in the #2004 diff; every literal enumerated above is a reversible test-harness bound (the 31-character name ceiling is derived in the brief from secret-scanner long-identifier heuristics; the 7 to 40 range is short-to-full git sha). Advisory: the 60s shell deadline at shell_test.go:23 is commented as headroom over an observed 23s, but TestReg786FleetHardening took 45.52s in row 2 under load average about 11 (26.92s in row 7), so the margin is thinner than stated.

Open hold, not a Verify row: the statusgen half of the floor is build/vet-only in PR CI until the staged CI patch for statusgen go test is applied by a maintainer (recorded by the implementer on #1836). The desk half rides the existing CI go test.

Tracked at #2077: re-author row 8's Expect cell so the witness reads its required exit as 0.

VERIFY: PASS — all 9 Verify rows pass by hand at b3677d6da; the execution witness scores row 8 fail on an Expect-cell parse (exits 0 not matched, prose exit 1 taken), so a witness-gated flip needs row 8's Expect re-authored first

### 2026-10-03 desk dispatch — 9/9 Verify rows pass on merged main 37d5eb00c; execution witness exit 0, 9/9 pass (row 8 now parses after #2081)

Run against merged main 37d5eb00c (matches forge main at run time). Offline, throwaway HOME, GOPROXY off, no GIT_DIR or token variables exported. Changes since the prior verdict at b3677d6da (range b3677d6da..37d5eb00c): #2046 changed deskkit production code (forge.go, forge_github.go, forge_gitlab.go) with new tests and flipped row 03 in the desktools-v2 README, which is in this brief's files list; #2063 changed deskkit's bodycheck.go and added a deskdispatch test and prompt reference. deskkit and deskdispatch own six floor seeds each in MANIFEST.md; #2075 changed a desk-containers board row; #2076 landed the prior verdict's own Evidence; #2081 (88d998068) re-authored row 8's Expect cell to lead with exit 0. tools/desk/internal/regression/ and statusgen/ are unchanged across the range. The floor stays green across all of these changes, so this run is the first evidence that it held across the #2046 deskkit changes. Separately, row 8's output hash is unchanged from the prior pass, so its fail-to-pass change comes from the Expect wording alone. Long test names and shas are abbreviated in this block.

| # | Command | Expected | Observed | Date / Runner |
| --- | --- | --- | --- | --- |
| 1 | Verify row 1 as written (floor-go.sh go test, run TestRegressionManifest in the regression package, anchored PASS grep) | top-level PASS line, exit 0 | exit 0; "--- PASS: TestRegressionManifest (0.62s)" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (floor-go.sh go test, run TestReg[0-9] over the regression package and cmd/deskclaim-ref, count vs MANIFEST) | pass=n manifest=m, equal, at least 1, exit 0 | exit 0; "pass=3 manifest=3" (TestReg786FleetHardening 28.01s, TestReg1145ShimCredential 4.05s, TestReg727WorktreeOrigin 0.10s) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (four anchored statusgen go test runs through floor-go.sh) | exit 0, four top-level PASS lines | exit 0; four PASS lines: the shallow-clone consumed-fragment test (0.27s), the relPath separator-style test (0.00s), the native-forge scan-read test (0.55s), the WSL-launcher bootstrap witness test (0.27s) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (awk seed-row count in MANIFEST vs Evidence-row count in this brief) | equal counts, at least 18, exit 0 | exit 0; "manifest=26 evidence=26" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | grep for the Regression floor heading in the desktools-v2 README | exit 0 | exit 0; heading present | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | statusgen --consumers --root . | exit 0, no routing claim disproved | exit 0; "consumers: no brief files in the diff against 37d5eb00c ... nothing to corroborate" (vacuous on merged main). Supplemental: --base set to the parent of the #2081 merge, --brief desktools-v2/14: exit 0, "0 corroborated, 0 disproved, 5 unchecked" | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | bash check-floor.sh in the regression package | exit 0, seed passes=26 | exit 0; "seed passes=26"; 26 top-level PASS lines, 0 FAIL lines | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (mutate_guard.py in seven modes: manifest, ci, directories, deadline, gitenv, execenv, runnerenv) | exit 0, printing controls=7 | exit 0; "controls=7"; worktree git status clean before and after (every mutated file restored) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (floor-go.sh go test, run TestFloor[RG] in the regression package, anchored PASS count) | pass=2, exit 0 | exit 0; "pass=2" (TestFloorRunnerGitIsolation 0.72s, TestFloorGoChokePoint 0.00s) | 2026-10-03 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (statusgen v1.0.31 verifyrun --dry-run, throwaway HOME, at 37d5eb00c): exit 0; all 9 rows scored pass exit=0 (output hashes 5638d8de2619, ae6c3e1d0574, 3e67811c8dd1, d24141988b50, e3b0c44298fc, a029e6fdd396, 16dd5af0e4da, 4c85b0ebdf92, e79a64120720). Row 8, scored fail at the prior pass on the Expect-cell parse, now scores pass with the same output hash as before (4c85b0ebdf92), so only the Expect wording changed. Rows 4 and 9 are scored on exit status only (witness note: nothing else in the Expect cell is machine-decidable).

Risk: gate model; risk metadata present, all four fields no; not irreversible. Of the changes since the prior verdict, #2081 changes Expect prose only and introduces no literal; #2046 and #2063 touch no file under tools/desk/internal/regression/ or statusgen/, so the floor's own bounds below are unchanged. Enumerated literals from the #2004 diff, re-confirmed on this main: commit-sha length 7 to 40 (manifest_test.go:17), new TestReg name ceiling 31 (manifest_test.go:64), seed-row floor 18 (Verify row 4), shell fixture deadline 60s (shell_test.go:23), floor runner per-test timeout 90s (check-floor.sh:31), git-isolation test deadline 120s (gitenv_test.go:256), mutation control timeouts 60s and 90s (mutate_guard.py:85, :87). All are reversible test knobs.
RISK-VALUE: N/A — no irreversible literal in the #2004 or #2081 diffs; every literal enumerated above is a reversible test-harness bound (7 to 40 is short-to-full git sha; 31 is the brief's secret-scanner long-identifier ceiling).

Open hold, not a Verify row: the statusgen half of the floor is still build/vet-only in PR CI; the staged CI patch for statusgen go test is present but not applied to the CI workflow (recorded on #1836). #2077 (row 8 Expect re-author) is resolved by #2081 but still reads open.

VERIFY: PASS — all 9 Verify rows pass on merged main 37d5eb00c and the execution witness exits 0 with 9/9 rows pass
2026-10-06 non-implementer re-verification on merged main 11228951d (confirmed by rev-parse and the commits API; delivered by #2004 as b6a5223b). Third pass, the first after #2160 changed the regression package. — VERIFY: PASS, 9/9.

| # | Command | Exit | Observed output | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | Verify row 1 as written (floor-go.sh go test of the manifest test, anchored PASS grep) | 0 | the manifest test PASS (0.75s); package ok | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2 as written (TestReg rows over the regression package and the claim-ref command, count vs MANIFEST) | 0 | pass=3 manifest=3 (the 786, 1145 and 727 regression tests: 29.40s, 3.86s, 0.08s); both packages ok | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3 as written (four anchored statusgen go test runs through floor-go.sh) | 0 | four top-level PASS lines (shallow-clone consumed-fragment, relPath separator style, native-forge scan read, WSL-launcher bootstrap witness); four ok lines | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4 as written (seed-row count in MANIFEST vs Evidence-row count) | 0 | manifest=26 evidence=26 | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5 as written (Regression floor heading in the stream README) | 0 | heading present at README line 171; states the port-not-delete rule | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | statusgen --consumers --root . (v1.0.32) | 0 | "no brief files in the diff against 11228951d0a8 ... nothing to corroborate" (vacuous on merged main). Supplement with --base at the parent of the #2004 merge and --brief desktools-v2/14: exit 0, 0 corroborated, 0 disproved, 5 unchecked | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | bash check-floor.sh in the regression package | 0 | seed passes=26; 26 top-level PASS lines, 0 FAIL | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | Verify row 8 as written (mutate_guard.py in seven modes) | 0 | controls=7; worktree git status empty before and after | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | Verify row 9 as written (TestFloor rows in the regression package, anchored PASS count) | 0 | pass=2 (runner git isolation 0.32s, go choke point 0.00s); package ok | 2026-10-06 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Run conditions: macOS arm64, go 1.27.1, offline, KUBECONFIG=/dev/null, no GIT_* or token variables. The changelog fragment named in Context was folded into the v1.0.32 changelog by cb4105ba (expected release behaviour).

RISK-VALUE: N/A — no irreversible literal in the #2004 diff or the #2160 regression-package change. Every literal enumerated (sha length 7 to 40, name ceiling 31, starter set 22, seed floor 18, test deadlines 60s/30s/120s, WaitDelay 1s, per-test timeout 90s, mutation timeouts 60s/90s, fixture-only git maintenance and gc off) is a reversible test-harness bound.

Open hold, not a Verify row: the statusgen half of the floor still does not run in PR CI (statusgen go test is only in the release workflow; the staged patch is recorded on #1836).

**VERIFY: PASS**
2026-10-06 execution witness for the pass above (statusgen v1.0.32 `verifyrun --brief`, non-dry, at 11228951d0a8, host 1-minute load about 27 to 35): exit 1. Rows 1, 3, 4, 5, 6, 8 and 9 pass; rows 2 and 7 are red.

The row 2 failure was reproduced by hand straight after: the fleet-hardening seed test (#786) was killed at its 60s shell-fixture deadline (`signal: killed`, 60.27s). Every fixture assertion printed before the kill read ok. A hand re-run of row 7 at load about 35 exited 0 (seed passes=26; the #786 test took 43.74s).

This is a load-sensitive deadline, not a behaviour regression. It is the thin-headroom concern flagged on 2026-10-03. The witness is being re-run at lower load. The status does not change until a passing witness lands.
### 2026-10-06 desk dispatch — VERIFY: PASS (re-verify at merged main 437c9ca04727, after #2308)

Re-verify, 2026-10-06, at merged main `437c9ca04727`, by a fresh non-implementer verifier: all 9 Verify rows, run exactly as authored from the worktree root, exit 0 on the first attempt. Row 7 ran at a 1-minute load of 6.58 (11.31 when it finished) on 16 cores and printed `seed passes=26`. Since the last verified head (11228951d0a8), the only commit is #2308 (437c9ca0): it adds the internal/gitquiet package; regression/gitquiet_test.go, whose TestMain calls gitquiet.Run; and changes to regression/shell_test.go. runShellFixture now takes t and passes GIT_TEMPLATE_DIR=gitquiet.TemplateDir(t). There is also a new TestShellFixtureReposQuiet with a 30s deadline. The 60s shell deadline value is unchanged and now sits at line 26. A separate run of `^TestShell` passed (TestShellDeadline, TestShellGitIsolation, TestShellFixtureReposQuiet).

Execution witness, statusgen v1.0.32 `verifyrun --dry-run`, three attempts. Attempts 1 and 2 used the desk's statusgen shim, at 1-minute loads of 12.34 and 11.81. Both exited 1, with rows 2 and 7 failing and the other 7 rows passing. The cause is not load. The shim exports CELLCTL_GH_AMBIENT, and every row inherits it. TestReg1145ShimCredential's fixture (the gen-shims-gh-token suite) then fails 2 assertions deterministically: the ambient-token check and the nested ambient-token check. This reproduces on a direct run with a dummy CELLCTL_GH_AMBIENT and passes when the variable is unset. This is a pre-existing isolation gap: the fixture env drops only GIT_* variables. #2308 did not cause it. Attempt 3 used the same v1.0.32 binary, invoked directly with the cell HOME and CELLCTL_GH_AMBIENT unset, at a 1-minute load of 12.15. It exited 0 with 9 of 9 rows passing. Separate load observation, from a diagnostic run of row 7 (not the Verify attempt): at a 1-minute load of 20 to 26, TestReg786FleetHardening was killed at the 60s shell deadline (60.16s). At loads of 12 to 21 it took 27 to 49s.

Execution witness table (verbatim, attempt 3):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestRegressionManifest$' -count=1 -timeout 30s -v ./internal/regression/ > "$d/r1.out" 2>&1 && grep -E -e '^--- PASS: TestRegressionManifest \(' "$d/r1.out"` | pass exit=0 | sha256:b2ec96ba1a37 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 2 | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestReg[0-9]' -count=1 -timeout 90s -v ./internal/regression ./cmd/deskclaim-ref > "$d/r2.out" 2>&1 && grep -q -F -e '--- PASS' "$d/r2.out" && n=$(grep -c -E -e '^--- PASS: TestReg[0-9]' "$d/r2.out") && m=$(grep -c -E -e '^[\|] #[0-9]+ [\|][^\|]*[\|] tools/desk/[^\|]*[\|] TestReg[0-9]' internal/regression/MANIFEST.md) && echo "pass=$n manifest=$m" && test "$n" -ge 1 && test "$n" -eq "$m"` | pass exit=0 | sha256:ae6c3e1d0574 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && f=../tools/desk/internal/regression/floor-go.sh && bash "$f" test -run '^TestConsumedFragmentIndexShallowCloneIsCouldNotCheck$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestRelPath_HandlesBothSeparatorStyles$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestScanIssueReadUsesNativeForgeNotGH$' -count=1 -timeout 30s -v . && bash "$f" test -run '^TestRunWitnesses_WSLLauncher_BootstrapIsCouldNotRun$' -count=1 -timeout 30s -v .` | pass exit=0 | sha256:24c02095cbbf | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 4 | `m=$(awk 'BEGIN{FS=sprintf("%c",124)} /^## Dropped/{exit} $2 ~ /^ #[0-9]+ $/ {n++} END{print n+0}' tools/desk/internal/regression/MANIFEST.md); e=$(awk 'BEGIN{FS=sprintf("%c",124)} $2 ~ /^ #[0-9]+ $/ && $3 ~ /^ Test[A-Za-z0-9_]+ $/ && $4 ~ /^ [0-9a-f]+ $/ && $5 ~ /^ [0-9a-f]+ / {n++} END{print n+0}' docs/streams/desktools-v2/brief-14-regression-floor.md); echo "manifest=$m evidence=$e"; test "$m" -eq "$e" && test "$m" -ge 18` | pass exit=0 | sha256:d24141988b50 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -q -E -e '^## Regression floor$' docs/streams/desktools-v2/README.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --consumers --root .` | pass exit=0 | sha256:acf9318d9376 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 7 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:b5450c882bf1 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 8 | `g=tools/desk/internal/regression/testdata/mutate_guard.py && python3 "$g" manifest > /dev/null && python3 "$g" ci > /dev/null && python3 "$g" directories > /dev/null && python3 "$g" deadline > /dev/null && python3 "$g" gitenv > /dev/null && python3 "$g" execenv > /dev/null && python3 "$g" runnerenv > /dev/null && echo controls=7` | pass exit=0 | sha256:4c85b0ebdf92 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |
| 9 | `d=$(mktemp -d) && cd tools/desk && bash internal/regression/floor-go.sh test -run '^TestFloor[RG]' -count=1 -timeout 180s -v ./internal/regression/ > "$d/r9.out" 2>&1 && n=$(grep -c -E -e '^--- PASS: TestFloor[RG][A-Za-z]+ [(]' "$d/r9.out") && echo "pass=$n" && test "$n" -eq 2` | pass exit=0 | sha256:e79a64120720 | 2026-10-06 | assay-verifier-app[bot] @ 437c9ca04727 (on-behalf-of human:ian) (forge-identity) |


Enumerated literals, scope = the #2004 floor deliverables plus the #2308 diff into tools/desk/internal/regression and internal/gitquiet, at 437c9ca0: commitRE {7,40} (manifest_test.go:17); the TestReg name ceiling of 31 (manifest_test.go:64); the seed-row floor of 18 (brief Verify row 4, line 129); seed passes=26 (brief Verify row 7, line 132); the shell fixture deadline 60*time.Second (shell_test.go:26); cmd.WaitDelay = time.Second (shell_test.go:60); the TestShellDeadline deadline 20*time.Millisecond (shell_test.go:73); the TestShellGitIsolation deadline 30*time.Second (shell_test.go:100); the TestShellFixtureReposQuiet deadline 30*time.Second (shell_test.go:132, new in #2308); gitquiet.Settings maintenance.auto=false and gc.auto=0 (gitquiet.go:44-45, new in #2308); the FixtureEnv -c maintenance.auto=false and gc.auto=0 (fixtureenv.go:75); and the template modes 0o755, 0o644 and 0o700 (gitquiet.go:63, 67, 70, 125). All are test-harness values that an edit can reverse; none is irreversible. They are ranked by what a wrong value would silently cost. Ranked last: WaitDelay, the 20ms, and the file modes.

RISK-VALUE: DERIVED — commitRE = {7,40} @ tools/desk/internal/regression/manifest_test.go:17 — 7 is git's default short-sha length and 40 is a full SHA-1 in hex, so the range accepts every short-to-full commit id and nothing else
RISK-VALUE: DERIVED — TestReg name ceiling = 31 @ tools/desk/internal/regression/manifest_test.go:64 — this is the brief's Deliverable 1 value (brief line 102), set so that no new seed-test name trips the secret scanners' long-identifier heuristic
RISK-VALUE: DERIVED — seed-row floor = 18 @ docs/streams/desktools-v2/brief-14-regression-floor.md:129 — the 22-issue starter set minus its named drops; it is a lower guard and the observed count is 26 (manifest=26 evidence=26), so it cannot pass vacuously
RISK-VALUE: DERIVED — maintenance.auto = false / gc.auto = 0 @ tools/desk/internal/gitquiet/gitquiet.go:44-45 (and fixtureenv.go:75) — these are git's documented off values for auto-maintenance and auto-gc. They live in each repo's own config via the template, because git drops GIT_CONFIG_COUNT/PARAMETERS across repositories. TestShellFixtureReposQuiet confirmed that init, init --bare and clone all carry both keys
RISK-VALUE: DERIVED — TestShellFixtureReposQuiet deadline = 30*time.Second @ tools/desk/internal/regression/shell_test.go:132 — the planted fixture runs only 3 git create operations and 6 config reads (observed 0.91s), so 30s is more than 30x headroom. It matches the sibling TestShellGitIsolation deadline (shell_test.go:100) and is a test-only knob that an edit can reverse
RISK-VALUE: NAMED, NOT DERIVED — shell fixture deadline = 60*time.Second @ tools/desk/internal/regression/shell_test.go:26 — question carried on #2305 (https://github.com/medici-finance/assay/issues/2305#issuecomment-6014760713); the value is unchanged by #2308 and its only basis is "headroom above an observed 23s". Today TestReg786FleetHardening took 27 to 49s at a 1-minute load of 12 to 21 and was killed at 60.16s at a load of 20 to 26 on 16 cores. Nothing ties 60s to the runner-load envelope; the question is carried on #2305

**VERIFY: PASS** — all 9 Verify rows pass on merged main 437c9ca04727, and the execution witness (statusgen v1.0.32, run without the shim-injected CELLCTL_GH_AMBIENT) exits 0 with 9/9 rows passing. The two shim-run witness attempts that failed are recorded above, with their cause.
