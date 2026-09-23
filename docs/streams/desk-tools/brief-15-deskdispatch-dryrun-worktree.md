---
brief: assay:assay:desk-tools:15
title: "`deskdispatch --dry-run --worktree <path>` — render the prompt against an operator-supplied home"
why: >-
  A dry-run dispatch prints the prompt with the agent's home worktree shown as a not-yet-known
  placeholder — deliberately, so no predicted path is ever pasted into a real dispatch. But a
  dry run is also how a verify desk previews a batch of verifier prompts, and each previewed
  prompt then has the placeholder substituted by hand before it is handed to an agent. A
  24-hour sweep of fifteen desk-role and worker session transcripts found one operator
  running a one-liner substitution over every dry-run prompt file per batch, two occurrences
  per file. A path the OPERATOR states for a worktree that already exists is not a prediction;
  letting the verb render it — after checking it is a real worktree of the item's repo under a
  sanctioned prefix — retires the substitution without weakening the rule that the verb never
  guesses.
wave: 1
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskdispatch/prompt.go` § homeUnknown renders the placeholder on `--dry-run` (used at the home line and again in the recreate-worktree command, two sites); `dispatch.go` § the dry-run branch calls `assemblePrompt(o, plan, \"\")`; `deskdispatch_test.go` pins that a dry run must not print a PREDICTED path. No `--worktree` flag exists."
  - "The path rules an operator-supplied home must satisfy, already implemented: `tools/desk/cmd/deskwt/deskwt.go` § pathGuard (resolves under the sanctioned prefixes; the shared checkout refused by identity) and § currentRepo (the worktree belongs to the item's repo)."
  - "The verifier prompt this is previewed for: `tools/desk/cmd/deskdispatch/references/verifier-prompt.md` and `tools/desk/cmd/verifyloop/dispatch.go` § assertNoSharedCheckout."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
version: 1
id: 1c3c43be-f164-4605-a8d7-239d1c542dd0
---

# Brief 15 — `deskdispatch --dry-run --worktree <path>`: render the prompt against an operator-supplied home

## Dependencies
None.

## Context

files:
- `tools/desk/cmd/deskdispatch/dispatch.go` (flag; the dry-run branch passes the validated path)
- `tools/desk/cmd/deskdispatch/main.go` (usage)
- `tools/desk/cmd/deskdispatch/deskdispatch_test.go`
- `tools/desk/README.md` (one paragraph under `deskdispatch`)

facts:
- `--worktree` is accepted ONLY together with `--dry-run`; on a real dispatch it is refused
  (exit 5) — the real dispatch names the path `deskwt add` printed and nothing else, and that
  rule does not move.
- validation before rendering, all three fail-closed (exit 5 naming the failed check):
  1. the path resolves (symlinks followed) under a sanctioned worktree prefix — reuse
     `deskwt`'s `pathGuard.check` logic (import or duplicate the two-line rule, per the module
     layout; do not loosen it);
  2. the path IS a registered git worktree (`git -C <path> rev-parse --show-toplevel` equals
     the resolved path, and `git -C <path> rev-parse --git-common-dir` is the item repo's
     common dir) — so a typo, an unrelated directory, or a clone of another repo is refused;
  3. it is not the shared checkout (the common dir's main worktree) — the isolation floor.
- rendering: `assemblePrompt(o, plan, home)` with the validated path; both placeholder sites
  become the path; the `deskdispatch: PLAN (dry run …)` banner gains `worktree=<path>
  (operator-supplied, verified)` so a transcript shows the path was checked, not predicted.
- the existing test that a dry run without `--worktree` prints the placeholder is unchanged.
- tests use a temp repo with a real `git worktree add` under a temp dir that the test points
  the sanctioned-prefix rule at (the `deskwt` tests already do this).

## Ground rules
- Never let `--worktree` reach a non-dry-run dispatch.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Flag + guard**: `--worktree <path>`, refused unless `--dry-run`.
2. **Validation** per the facts, in one function with one named reason per failure.
3. **Render** and banner line.
4. **Tests**: valid worktree → prompt carries the path at both sites and no placeholder; a
   path outside the prefixes → 5; an existing directory that is not a worktree → 5; a worktree
   of a DIFFERENT repo → 5; the shared checkout itself → 5; `--worktree` without `--dry-run`
   → 5 and no child process ran; dry run without `--worktree` → placeholder, unchanged.
5. **README + usage** text.
6. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1` | exit 0 — the path appears at both sites, the placeholder at none, the banner says `operator-supplied, verified` |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1` | exit 0 — the four NEGATIVE cases (outside prefix, not a worktree, other repo, shared checkout) each exit 5 with their own reason and print no prompt |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1` | exit 0 — exit 5, zero child processes recorded |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | exit 0 — including the existing dry-run placeholder test, unchanged |
| 6 | check:ci | `gofmt -l tools/desk/cmd/deskdispatch > /tmp/dd-fmt.out; test ! -s /tmp/dd-fmt.out` | exit 0 |
| 7 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| Only the first placeholder site is substituted | row 2 |
| A clone of another repo passes validation | row 3 |
| The flag leaks into a real dispatch and overrides deskwt's path | row 4 |
| Validation loosened to "directory exists" | row 3 (not-a-worktree case) |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->
| # | Command | Expect | Observed | Date / Runner |
|---|---------|--------|----------|---------------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | exit 0, no output | 2026-09-15 sonnet-5-verifier |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1` | exit 0 — path at both sites, no placeholder, banner says "operator-supplied, verified" | exit 0 — `--- PASS: TestDryRunWorktreeRendersVerifiedPath (0.21s)` | 2026-09-15 sonnet-5-verifier |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1` | exit 0 — four negative cases each exit 5 with own reason, no prompt printed | exit 0 — all four subtests PASS: outside-prefix, not-a-worktree, other-repo, shared-checkout | 2026-09-15 sonnet-5-verifier |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1` | exit 0 — exit 5, zero child processes recorded | exit 0 — `--- PASS: TestWorktreeFlagRefusedOnRealDispatch (0.19s)` | 2026-09-15 sonnet-5-verifier |
| 5 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | exit 0, including unchanged placeholder test | exit 0 — `ok github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 10.627s` | 2026-09-15 sonnet-5-verifier |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch > /tmp/dd-fmt.out; test ! -s /tmp/dd-fmt.out` | exit 0 | **exit 1** — `gofmt -l` lists `phantom_test.go` only; this brief's own touched files (dispatch.go, main.go, worktree.go, worktreedryrun_test.go) are independently confirmed gofmt-clean. The drift predates this brief's merge by two days (introduced by an unrelated commit, confirmed by checking the parent commit's copy of the file) and trips this package-wide row for any brief touching this package. Already filed and open: medici-finance/assay#1119. Not a change-failure of this deliverable. | 2026-09-15 sonnet-5-verifier |
| 7 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | exit 0 — `LINT: PASS` (repo-wide NOTICEs present, none fatal) | 2026-09-15 sonnet-5-verifier |

RISK-VALUE: DERIVED — worktreeTmpBase = "/private/tmp" @ tools/desk/cmd/deskdispatch/worktree.go:30 — matches the canonical sanctioned-prefix value already pinned in tools/desk/cmd/deskwt/deskwt.go:26 (`tmpBaseDir = "/private/tmp"`), which this brief's own Deliverables required duplicating "no looser" from deskwt's pathGuard rule. Confirmed identical, not a fabricated or independently-chosen value.
RISK-VALUE: DERIVED — the "tracker-" worktree-name prefix check @ tools/desk/cmd/deskdispatch/worktree.go:100 and the `.claude/worktrees` sanctioned-suffix path @ worktree.go:103 both match deskwt.go's own literals (`"tracker-"` prefix test and `filepath.Join(root, ".claude", "worktrees")`) byte-for-byte — the duplicated isolation-floor allowlist is faithful to its source, not loosened.

VERIFY: HELD — deliverable sound; stays `implemented`. Rows 1-5 and 7 PASS. Row 6 fails on an EXTERNAL, pre-existing, unrelated cause (medici-finance/assay#1119) — not a defect in this brief's diff. Not advanced to `verified` per the row-6 fail; no new bug filed since #1119 already covers it.
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PARTIAL** — HELD at `implemented`

Runner ≠ implementer. Deliverable commit `622400754` confirmed merged and an ancestor of `origin/main`. The brief's own file already carried a prior verdict (dated 2026-09-15, attributed `sonnet-5-verifier` — a distinct, non-implementer identity from the implementing `assay-worker-app[bot]` commit; not self-reported). Every row independently re-run from scratch regardless, not trusted from that prior record.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go vet ./...` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 2 | `TestDryRunWorktreeRendersVerifiedPath` | exit 0, path at both sites, no placeholder, banner present | exit 0 — independently read the test body: asserts count ≥2, no placeholder string, banner substring present | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestDryRunWorktreeRefusesUnverifiablePaths` | exit 0, 4 negative subtests each exit 5 | exit 0 — all 4 subtests PASS (outside-prefix, not-a-worktree, other-repo, shared-checkout) | 2026-09-17 | sonnet-5-verifier |
| 4 | `TestWorktreeFlagRefusedOnRealDispatch` | exit 0, exit 5, zero child processes | exit 0 — read test body: asserts exit 5 and zero recorded calls | 2026-09-17 | sonnet-5-verifier |
| 5 | whole-package test | exit 0 incl. unchanged placeholder test | exit 0 — pre-existing placeholder assertions still run and pass | 2026-09-17 | sonnet-5-verifier |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch` | exit 0 | **exit 1** — only `phantom_test.go` listed, an UNRELATED file this brief does not touch. Independently confirmed: this brief's 4 touched files are gofmt-clean on their own; the drift is a comment-alignment issue from a separate commit `91a7f9208` (2026-09-06), predating this brief's merge (2026-09-08). Already tracked at `medici-finance/assay#1119` — not re-filed | 2026-09-17 | sonnet-5-verifier |
| 7 | `statusgen --lint` | exit 0 | exit 0, LINT: PASS | 2026-09-17 | sonnet-5-verifier |

**RISK-VALUE: DERIVED** — `worktreeTmpBase = "/private/tmp"` (`worktree.go:30`), cross-checked byte-for-byte against `deskwt.go:26`'s identical constant — not independently invented or loosened. The `"tracker-"` prefix test and `.claude/worktrees` sanctioned-suffix path (`worktree.go:100,103`) cross-checked against `deskwt.go`'s own `pathGuard.allowed` — same rule, duplicated verbatim. Check ordering (shared-checkout identity check before prefix check) matches deskwt's own refusal order. Exit code `deskkit.ExitRefused = 5` used throughout, matching the brief's requirement. No self-chosen or unexplained literal found — both named values are derived (duplicated from deskwt's existing pinned constants), not fabricated.

**VERIFY: PARTIAL** — rows 1-5, 7 PASS. Row 6 fails AS WRITTEN (the command scans the whole `cmd/deskdispatch` directory, not just this brief's 4 files) but for a confirmed, pre-existing, unrelated cause tracked at `assay#1119` — not a defect in this brief's own diff. Per the discipline that a Verify row's literal command is what's judged, this stays a PARTIAL, not a rounded-up PASS: held at `implemented`, consistent with how this house has treated similarly externally-caused literal-row failures elsewhere in this drain (e.g. harness-portability/04, sdlc/15).
### Non-implementer verifier re-run — VERIFY: PARTIAL (row 6 pre-existing unrelated gofmt drift, tracked) — sonnet-5-verifier (verify-desk dispatch), @ merged main `951ca784d100a7d201a28a34033da6709ec2ec8f`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted. Deliverable commit `622400754` confirmed a merged ancestor of origin/main.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `go build ./... && go vet ./...` | exit 0 | exit 0, silent | 2026-09-18 | sonnet-5-verifier |
| 2 | `TestDryRunWorktreeRendersVerifiedPath` | exit 0, path+banner | PASS | 2026-09-18 | sonnet-5-verifier |
| 3 | `TestDryRunWorktreeRefusesUnverifiablePaths` | exit 0, 4 negative subtests exit 5 | PASS, all 4 | 2026-09-18 | sonnet-5-verifier |
| 4 | `TestWorktreeFlagRefusedOnRealDispatch` | exit 0, exit 5, zero children | PASS | 2026-09-18 | sonnet-5-verifier |
| 5 | whole-package test | exit 0 incl. placeholder tests | exit 0 — ok (8.879s) | 2026-09-18 | sonnet-5-verifier |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch` | exit 0 | **exit 1** — lists only `phantom_test.go`, confirmed NOT this brief's diff (last touched by unrelated commit 91a7f9208, predates this brief's merge 622400754). Already tracked medici-finance/assay#1119, not re-filed | 2026-09-18 | sonnet-5-verifier |
| 7 | `statusgen --root .. --lint` | 0 | exit 0 — LINT: PASS | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all 7 rows map 1:1 to Verify rows. Diff scope confirmed matching brief's file list exactly (dispatch.go, main.go, worktree.go, worktreedryrun_test.go, README.md) — no unrelated changes.

RISK-VALUE: DERIVED — `worktreeTmpBase = "/private/tmp"` @ worktree.go:30, cross-checked identical to deskwt.go:26.
RISK-VALUE: DERIVED — `"tracker-"` prefix literal @ worktree.go:100, cross-checked against deskwt.go:166 (same literal, same test shape).
RISK-VALUE: DERIVED — `.claude/worktrees` sanctioned-suffix path @ worktree.go:103, cross-checked against deskwt.go:118 (same shape).
RISK-VALUE: DERIVED — exit code `deskkit.ExitRefused = 5` @ exitcodes.go:27, used throughout the three refusal paths, matching the brief's stated requirement.

VERIFY: PARTIAL — held at implemented. Rows 1-5,7 checked-clean; row 6 fails exactly as written, but the cause (phantom_test.go, drifted by a separate commit two days before this brief's merge) is confirmed pre-existing and unrelated, tracked at assay#1119. Matches both prior recorded verdicts (2026-09-15 implementer, 2026-09-17 non-implementer) — this third pass reaches the same result independently. No new issue filed.
### Non-implementer verifier run — VERIFY: PARTIAL (row 6 only: pre-existing, unrelated gofmt finding, tracked #1119; rows 1-5, 7 checked-clean; 3rd independent pass, same result) — verify-desk-dispatch-20260920T0246Z (verify-desk dispatch), @ merged main `e4109205`, 2026-09-20

Own detached worktree off origin/main; deliverable commit 622400754 (PR-landed 2026-09-08) confirmed ancestor. Offline envelope, non-implementer, read-only.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | cd tools/desk && go build ./... && go vet ./... | exit 0 | exit 0 — build OK, vet OK | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 2 | go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1 | exit 0 — path at both sites, no placeholder, banner "operator-supplied, verified" | exit 0 — PASS; test asserts the path at both sites, no homeUnknown placeholder, banner substring present | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 3 | go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1 | exit 0 — 4 negative cases each exit 5 with own reason | exit 0 — all 4 subtests PASS: outside-prefix, not-a-worktree, other-repo, shared-checkout; each names its reason, no prompt printed | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 4 | go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1 | exit 0 — exit 5, zero child processes | exit 0 — PASS; asserts rc==5 and no child process ran | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 5 | go test ./cmd/deskdispatch/ -count=1 | exit 0 | exit 0 — ok 9.075s (whole package) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 6 | gofmt -l tools/desk/cmd/deskdispatch; test ! -s <listing> | exit 0 | **exit 1** — gofmt lists tools/desk/cmd/deskdispatch/phantom_test.go. Freshly attributed at this SHA: last touched by unrelated commit 91a7f9208 (2026-09-06), two days BEFORE deliverable 622400754 (2026-09-08); that commit's stat does not include the file. Pre-existing, unrelated to this brief's diff, tracked at #1119 — not re-filed | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 7 | cd statusgen && go run . --root .. --lint | exit 0 | exit 0 — LINT: PASS (repo-wide NOTICEs only, none fatal) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |

RISK-VALUE: DERIVED — worktreeTmpBase = /private/tmp @ tools/desk/cmd/deskdispatch/worktree.go:30 — byte-identical to the pinned isolation-floor constant tmpBaseDir @ tools/desk/cmd/deskwt/deskwt.go:26; the brief required duplicating deskwt's pathGuard rule "no looser", and it is duplicated, not independently chosen or loosened.
RISK-VALUE: DERIVED — "tracker-" worktree-name prefix @ worktree.go:100 and the .claude/worktrees sanctioned-suffix join @ :103 — identical literal and shape to deskwt.go:166 / deskwt.go:118 (strict child-prefix test with separator in both).
RISK-VALUE: DERIVED — refusal exit code ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:27 (ExitUnverifiable = 6 @ :31) — matches the brief's requirement and the verb family convention.
Ranking note: all four gate a DRY-RUN-ONLY render (dispatch.go:455 refuses --worktree on any real dispatch; row 4 pins zero child processes) — wrongness breaks an operator preview, reversible by edit + rebuild.

VERIFY: PARTIAL — row 6 exactly as written: gofmt -l over cmd/deskdispatch lists phantom_test.go (test exit 1). Cause confirmed pre-existing and unrelated to this brief's diff (91a7f9208, 2026-09-06, before the 2026-09-08 merge; tracked #1119). Rows 1-5 and 7 checked-clean. Item does NOT advance; stays implemented.

## Review

Gate: model (all four risk answers no). The reviewer confirms row 4 is present and that the
validation is the three checks named, not a subset.
