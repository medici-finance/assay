---
brief: desk-tools/15
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
schema: brief-v1
authored: 2026-09-02 by a worker-desk authoring session, from a 24-hour transcript sweep across
  fifteen desk-role and worker sessions (tallied per session)
sources:
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskdispatch/prompt.go` § homeUnknown renders the placeholder on `--dry-run` (used at the home line and again in the recreate-worktree command, two sites); `dispatch.go` § the dry-run branch calls `assemblePrompt(o, plan, \"\")`; `deskdispatch_test.go` pins that a dry run must not print a PREDICTED path. No `--worktree` flag exists."
  - "The path rules an operator-supplied home must satisfy, already implemented: `tools/desk/cmd/deskwt/deskwt.go` § pathGuard (resolves under the sanctioned prefixes; the shared checkout refused by identity) and § currentRepo (the worktree belongs to the item's repo)."
  - "The verifier prompt this is previewed for: `tools/desk/cmd/deskdispatch/references/verifier-prompt.md` and `tools/desk/cmd/verifyloop/dispatch.go` § assertNoSharedCheckout."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
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

## Review

Gate: model (all four risk answers no). The reviewer confirms row 4 is present and that the
validation is the three checks named, not a subset.
