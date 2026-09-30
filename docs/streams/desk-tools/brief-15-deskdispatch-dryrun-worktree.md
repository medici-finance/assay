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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch > /tmp/dd-fmt.out; test ! -s /tmp/dd-fmt.out` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go run . --root .. --lint; echo $?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier notes — 2026-09-27 verify-desk dispatch (opus-5.5 verifier), @ merged main 874d56de38a7

Runner is not the implementer (deliverable commit 622400754, PR #629, merged 2026-09-08, confirmed an ancestor of main). Own detached worktree off origin/main, offline envelope (KUBECONFIG=/dev/null), read-only apart from this Evidence block.

Witness instrument: the table above records all seven check:ci rows as could-not-run — the hermetic network-off sandbox it requires is Linux-only and the verifier host is darwin. That table is an environment limit of the instrument, not a result about the deliverable. So every row was ALSO executed directly on the same host at the same SHA, exact Verify command, with GOPROXY=off and GOTOOLCHAIN=local so no module or toolchain fetch could reach the network (go1.27.1). Real observed output per row:

- Row 1 — build + vet: exit 0, silent.
- Row 2 — exit 0; with -v added the output carries the line "--- PASS: TestDryRunWorktreeRendersVerifiedPath", so the pass is not a vacuous no-tests-to-run. The test asserts the resolved path appears at least twice, the not-yet-known placeholder appears nowhere, and the PLAN banner carries "operator-supplied, verified". Read against the code: both placeholder sites (the home line and the recreate-worktree command in prompt.go) render the one home variable, and the dry-run branch passes plan.home, so a no-placeholder assertion covers the second site.
- Row 3 — exit 0; "--- PASS: TestDryRunWorktreeRefusesUnverifiablePaths" plus all four subtests PASS: outside-prefix, not-a-worktree, other-repo, shared-checkout. Each case asserts rc 5, its own reason string on stderr, and no "# Assignment" prompt on stdout.
- Row 4 — exit 0; "--- PASS: TestWorktreeFlagRefusedOnRealDispatch". Asserts rc 5 and zero recorded child processes; the refusal sits first in validateCallerPreconditions, ahead of any claim or child process.
- Row 5 — exit 0; "ok github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 204.477s" (whole package, including the unchanged placeholder assertions).
- Row 6 — exit 0; gofmt listing empty (0 bytes). The previous row-6 blocker (phantom_test.go comment alignment, medici-finance/assay#1119) is fixed on main by PR #1268 and the issue is closed (2026-09-23). This row no longer holds the item.
- Row 7 — go run . --root .. --lint exit 0, "LINT: PASS". NOTICEs name this brief only for [gotest-run-vacuous] on rows 2-4 (no "--- PASS:" assertion in the row command). This pass closed that gap by observing the "--- PASS:" line for each named test directly, as recorded above. The row wording could be tightened in a follow-up; that is a check-definition polish, not a defect of the deliverable.

Grounding against the brief's facts: the three checks are all present in validateOperatorWorktree (tools/desk/cmd/deskdispatch/worktree.go) — shared checkout refused by identity first, then the sanctioned-prefix rule, then show-toplevel equals the resolved path AND git-common-dir equals the item repo's. Each fails closed through deskkit.Refused (exit 5) with its own message. An unreadable item-repo root returns exit 6 (unverifiable). That is a precondition on the item repo, not one of the three operator-input checks. --worktree without --dry-run is refused at dispatch.go (validateCallerPreconditions) before anything else runs.

Risk-bearing value enumeration (diff scope: every file 622400754 touches — dispatch.go, main.go, worktree.go, the new test file, the tools README and changelog; plus the literals the brief's facts name):

1. worktreeTmpBase = "/private/tmp" @ tools/desk/cmd/deskdispatch/worktree.go:76
2. worktree-name prefix literal "tracker-" @ tools/desk/cmd/deskdispatch/worktree.go:146
3. sanctioned in-repo prefix filepath.Join(sharedCheckout, ".claude", "worktrees") @ tools/desk/cmd/deskdispatch/worktree.go:149
4. ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:27 (used by, not introduced by, this diff; named in the brief's facts)
5. ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:31 (the unreadable-root path)
6. authority binding: --worktree is accepted only when dryRun is true @ tools/desk/cmd/deskdispatch/dispatch.go:647 (a boolean gate, no numeric literal)

Ranking: 1-3 rank highest. A loosened prefix would let a previewed prompt name a home outside the isolation allowlist. An agent handed that prompt could then write in the wrong place, and that is only partly reversible. Everything else only affects a dry-run render and is reversible by an edit and a rebuild. Items 4-6 are exit-code and gating contracts, reversible, and covered by rows 3-4.

RISK-VALUE: DERIVED — worktreeTmpBase = "/private/tmp" @ tools/desk/cmd/deskdispatch/worktree.go:76 — byte-identical to deskwt's tmpBaseDir = "/private/tmp" @ tools/desk/cmd/deskwt/deskwt.go:26. The brief requires duplicating deskwt's pathGuard rule and not loosening it. The var is not wired to any env var or flag, so an operator cannot relocate the allowlist.
RISK-VALUE: DERIVED — "tracker-" prefix @ tools/desk/cmd/deskdispatch/worktree.go:146 and the ".claude"/"worktrees" join @ tools/desk/cmd/deskdispatch/worktree.go:149 — same literals and the same two-branch shape as deskwt's pathGuard.allowed @ tools/desk/cmd/deskwt/deskwt.go:166-169 and its worktreesDir @ tools/desk/cmd/deskwt/deskwt.go:118. The rule is a direct child of the resolved tmp base, or strictly under the resolved worktrees dir with a separator. The order matches too: the shared-checkout identity refusal comes before the prefix test, as in deskwt's check @ tools/desk/cmd/deskwt/deskwt.go:131-135. Not loosened.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:27 — the brief's facts require exit 5 for each refusal, and this is the verb family's refused-by-constraint code. ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:31 is the family's could-not-verify code, correctly kept apart from an operator-input refusal.

VERIFY: PASS — all seven Verify rows checked-clean by direct execution at 874d56de38a7 (row 6 is now green because the unrelated gofmt drift was fixed upstream). The machine witness table above records could-not-run on this darwin host (it has no network-off sandbox). A Linux or CI re-execution of the check:ci rows is what gives the board a passing witness. Gate: model, all four risk answers no.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ e03f4f5c7c41 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main e03f4f5c7c412560a666d95383bee0444fb6d263, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1` | pass exit=0 | sha256:43ba17e809af | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1` | pass exit=0 | sha256:b5f6a78b6330 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1` | pass exit=0 | sha256:4c1a57184d16 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | fail exit=1 | sha256:7e49f1ccbcfe | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch > /tmp/dd-fmt.out; test ! -s /tmp/dd-fmt.out` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go run . --root .. --lint; echo $?` | pass exit=0 | sha256:bf3639f2eb2b | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./...` | exit 0 | Verify row 1: exit 0, no output | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRendersVerifiedPath$' -count=1` | exit 0; path at both sites, no placeholder, banner operator-supplied, verified | Verify row 2: exit 0, ok deskdispatch 1.427s; with -v added the line --- PASS: TestDryRunWorktreeRendersVerifiedPath is present (not a vacuous no-tests-to-run) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 3 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestDryRunWorktreeRefusesUnverifiablePaths$' -count=1` | exit 0; four negative cases each exit 5 with own reason, no prompt | Verify row 3: exit 0, ok deskdispatch 3.060s; with -v the parent and all four subtests PASS: outside-prefix, not-a-worktree, other-repo, shared-checkout | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskdispatch/ -run '^TestWorktreeFlagRefusedOnRealDispatch$' -count=1` | exit 0; exit 5, zero child processes | Verify row 4: exit 0, ok deskdispatch 1.240s; with -v the line --- PASS: TestWorktreeFlagRefusedOnRealDispatch is present | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | exit 0 incl. the unchanged placeholder test | Verify row 5: exit 0, ok deskdispatch 27.236s (clone origin URL set to the same forge URL the dispatched worktree carries). Also exit 0 in Linux docker --network none (ok 1.407s). The machine witness records exit 1 for this row; see Notes | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 6 | `gofmt -l tools/desk/cmd/deskdispatch > /tmp/dd-fmt.out; test ! -s /tmp/dd-fmt.out` | exit 0 | Verify row 6: exit 0, listing 0 bytes | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 7 | `cd statusgen && go run . --root .. --lint; echo $?` | 0 | Verify row 7: printed 0, LINT: PASS; lines naming this brief are three NOTICEs only (gotest-run-vacuous on rows 2-4), no PROBLEM | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — worktreeTmpBase = "/private/tmp" @ tools/desk/cmd/deskdispatch/worktree.go:76 — byte-identical to deskwt's tmpBaseDir = "/private/tmp" @ tools/desk/cmd/deskwt/deskwt.go:26; the brief requires the deskwt pathGuard rule duplicated and not loosened; not wired to any env var or flag.
RISK-VALUE: DERIVED — "tracker-" @ tools/desk/cmd/deskdispatch/worktree.go:146 and the ".claude"/"worktrees" join @ tools/desk/cmd/deskdispatch/worktree.go:149 — same literals and same two-branch shape as deskwt pathGuard.allowed @ tools/desk/cmd/deskwt/deskwt.go:166 and worktreesDir @ tools/desk/cmd/deskwt/deskwt.go:118 (direct tracker-* child of the resolved tmp base, or strictly under the resolved worktrees dir with a separator). One difference noted below: deskdispatch anchors the in-repo prefix at the common dir's main worktree, deskwt at its --root.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 — the brief's facts require exit 5 for every refusal; ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55 is the family's could-not-verify code, kept apart from an operator-input refusal.

Notes:
- BLOCKED (check-definition / instrument), not a product failure. All seven rows pass by hand; the Linux witness passes rows 1-4, 6 and 7 and fails row 5 because the check:ci sandbox's network namespace has loopback down, so other briefs' loopback httptest tests in the named package cannot pass there. `statusgen brief --check-verified` with a hypothetical flip exits 1 on row 5 alone. Advancing needs verifyrun to bring loopback up in its namespace, or row 5 narrowed or reclassed, then a re-verify.
- Grounding, written before reading the diff or tests: flag accepted only with --dry-run (exit 5 otherwise, before any child process); one validator with three fail-closed checks (sanctioned prefix after symlink resolution; show-toplevel equals the path and git-common-dir equals the item repo's; not the shared checkout), each exit 5 with its own reason; both placeholder sites render the path; PLAN banner carries worktree=<path> (operator-supplied, verified); three named tests; usage and README text. Found on main exactly as expected: validateOperatorWorktree in worktree.go (shared-checkout identity check first, then prefix, then toplevel and common-dir), the refusal first in validateCallerPreconditions (dispatch.go:647), assemblePrompt(o, plan, plan.home) on the dry-run branch, both render sites (prompt.go home line and recreate-worktree command) using the one home variable, usage in main.go, a paragraph in tools/desk/README.md. The existing placeholder assertions in deskdispatch_test.go (homeUnknown) are intact.
- End-to-end dry runs (binary built from the clone, --kit verifier, no token in the environment, nothing claimed): the dispatched worktree as --worktree gave exit 0, banner "worktree=<that path> (operator-supplied, verified)", prompt file carries the path 2 times and the placeholder 0 times. Refused with exit 5 and no prompt file: <scratch>/clone (outside prefix), <scratch>/e2e plain directory (outside prefix), <shared> main worktree (shared-checkout reason), a tracker-* worktree of a different local repository (different-repo reason). No real (non-dry) dispatch was run; row 4 covers that path.
- Observation, within the brief's three checks: the checkout base (itself a tracker-* LINKED worktree of the shared checkout) was accepted as --worktree with exit 0, because the brief defines the shared checkout as the common dir's main worktree only. Handing an agent the dispatcher's own --root as home is not refused. Not a Verify-row failure; a possible tightening for a follow-up.
- Row 5 witness FAIL is an instrument artifact, not the deliverable: verifyrun runs check:ci rows under unshare --net --map-root-user, whose fresh network namespace has loopback DOWN. Proof in the same container: a bash connect to 127.0.0.1 gives "Connection refused" under docker --network none but "Network is unreachable" inside unshare. The same command in the same copy gives exit 0 without the unshare wrapper and exit 1 with it; the 12 failing tests (TestStamp* and TestGitLab* stamp and claim tests from other briefs) all fail on "dial tcp 127.0.0.1:<port>: connect: network is unreachable" against their own httptest server. None of this brief's tests fail.
- Also noted: the stamp tests read the forge from the CURRENT directory's origin remote (deskkit originRemoteHost). In a clone whose origin is a local path, 7 TestStamp* tests fail with exit 6 (forge could not be resolved). That is ambient-state dependence in unrelated tests; the hand run above used a clone whose origin URL matches the dispatched worktree's.
- Rows 2-4: the lint reports gotest-run-vacuous NOTICEs because the commands carry no --- PASS assertion. This pass observed each --- PASS line with -v, so the rows are not vacuous here, but the row wording could be tightened.
- Witness instrument: Linux docker (OrbStack, golang:1.25-bookworm pinned by digest, go1.25.14 linux/arm64, --network none, GOPROXY=off), statusgen built in-container from the copy at e03f4f5 (sha256 03a471ef729659fbe273b7913b87859b3a399aef9a0f812b7236705be75eec3b), UNSHARE_OK, DNS probe failed as expected, tree clean before the run. Not the darwin witness: every row is check:ci, which needs unshare --net.
- statusgen brief --check-verified on throwaway --no-hardlinks clones with the README row flipped to verified and a dated Verified cell: exit 1 against main as-is ("requires passing execution witnesses: row 1..7 could-not-run", from the 2026-09-27 darwin witness); exit 1 with this pass's Linux witness appended ("requires passing execution witnesses: row 5: fail — the witness records a failure"). Rows 1-4, 6, 7 are no longer blocking.
- Outcome: every row passes by hand, but row 5 cannot produce a passing execution witness under the hermetic check:ci sandbox as authored. That is a check-definition and instrument failure (the row names the whole package, which contains loopback-httptest tests that cannot pass inside a loopback-down namespace), not a defect in this brief's diff. Needs either verifyrun bringing loopback up inside its namespace, or row 5 narrowed or reclassed, before the witness can go green. Item stays implemented.

VERIFY: BLOCKED

## Review

Gate: model (all four risk answers no). The reviewer confirms row 4 is present and that the
validation is the three checks named, not a subset.
