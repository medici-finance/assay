---
brief: desk-tools/11
title: "`deskwt add` — a worktree whose directory is gone does not hold its branch"
why: >-
  `deskwt add` already reclaims a leftover branch that no worktree holds and that carries
  nothing its upstream lacks. A 24-hour sweep of fifteen desk-role and worker session
  transcripts still found a verify session clearing such branches by hand three times. The
  shape the source does not cover is the commonest way a dispatch dies: the agent's worktree
  DIRECTORY is removed without `git worktree remove`, so git still lists a worktree entry —
  marked prunable — that holds the branch. The holder scan reads that entry as a live owner
  and refuses; the operator, seeing a refusal that names a path that does not exist, deletes
  the branch by hand and bypasses the 0-ahead proof. A prunable entry is bookkeeping, not an
  owner, and `deskwt prune` already knows that; `add` should too.
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
  - "freshness-checked 2026-09-02 @ 547b708 — `tools/desk/cmd/deskwt/branchcollision.go` § branchHolders parses only the `worktree ` and `branch ` lines of `git worktree list --porcelain` and never the `prunable ` line, so a directory-gone entry is a holder; § reclaimStaleBranch refuses on any holder before the 0-ahead check. The covered shapes (no holder + 0 ahead of upstream-or-base → reclaimed; N ahead → refused; live holder → refused) are established at the same file and are NOT re-authored here."
  - "The bookkeeping prune `deskwt prune` already runs first: `tools/desk/cmd/deskwt/prune.go` § bookkeepingPrune (`git worktree prune`)."
  - "git's porcelain contract: a worktree whose directory is missing is listed with a `prunable <reason>` attribute line (`git worktree list --porcelain`, git ≥ 2.36)."
  - "Brief and Verify shape: `spec/brief-v1.md`; status semantics: `spec/lifecycle-v1.md`."
---

# Brief 11 — `deskwt add`: a worktree whose directory is gone does not hold its branch

## Dependencies
None.

## Context

files:
- `tools/desk/cmd/deskwt/branchcollision.go` (`branchHolders` reads the `prunable` attribute)
- `tools/desk/cmd/deskwt/branchcollision_test.go` (or the existing collision test file)
- `tools/desk/README.md` (one sentence in the `add` collision paragraph)

facts:
- `git worktree list --porcelain` emits per entry: `worktree <path>`, `HEAD <sha>`, `branch
  <ref>` (or `detached`), then optional attribute lines `locked [<reason>]`, `prunable
  <reason>`, separated by a blank line. `prunable` appears when the directory is missing.
- `branchHolders` (`branchcollision.go`) keys `holders[ref] = path` for every `branch` line; a
  prunable entry therefore counts as a holder and `reclaimStaleBranch` refuses naming a path
  that no longer exists.
- the fix is a READ: skip (do not record as holder) an entry that carries a `prunable` line.
  It does NOT run `git worktree prune` from `add` — that is `deskwt prune`'s verb, and `add`
  must not acquire a second mutation. The ordinary `worktree add` that follows the reclaim
  succeeds regardless of the stale entry because git's `-b` collision is on the BRANCH, which
  the reclaim has deleted.
- a `locked` AND `prunable` entry: still not a holder for the branch question (the lock
  protects the directory that is gone; the branch proof is 0-ahead, unchanged). Document it.
- every other shape keeps its verdict: a holder whose directory EXISTS is a live owner (refuse);
  N ahead of the comparison ref is unfinished work (refuse); 0 ahead + no holder is reclaimed.
- fixture: temp repo, `deskwt add` a worktree under a sanctioned prefix, `rm -rf` its directory
  (NOT `git worktree remove`), then `deskwt add` the same name again.

## Ground rules
- No new mutation in `add`; no `--force` anywhere.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **`branchHolders`**: track the current entry's `prunable` attribute and omit that entry's
   branch from the holder map; keep the fail-closed error propagation unchanged.
2. **Test**: the directory-gone shape reclaims (audit line present, second `add` exits 0 and
   prints the new path); the directory-PRESENT shape still refuses naming the path; the
   N-ahead shape still refuses naming the count; and a prunable entry whose branch is N ahead
   still refuses (the prunable reading removes the HOLDER objection only, never the proof).
3. **README**: one sentence — a prunable (directory-gone) entry is not an owner; `deskwt
   prune` drops the entry itself.
4. **Nothing else.**

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd tools/desk && go build ./... && go vet ./...` | exit 0 |
| 2 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestAddReclaimsBranchHeldByPrunableWorktree$' -count=1` | exit 0 — after `rm -rf` of the first worktree's directory, the second `add` of the same name exits 0 and the audit line says `reclaimed stale local branch` |
| 3 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestAddStillRefusesLiveHolder$' -count=1` | exit 0 — with the directory present, exit 5 naming the worktree path |
| 4 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -run '^TestAddStillRefusesAheadBranchEvenWhenPrunable$' -count=1` | exit 0 — the NEGATIVE control: prunable entry + 1 commit ahead → exit 5 naming the count; the branch survives |
| 5 | check:ci | `cd tools/desk && go test ./cmd/deskwt/ -count=1` | exit 0 — every existing collision, prune and lock test unchanged |
| 6 | check:ci | `gofmt -l tools/desk/cmd/deskwt > /tmp/dw-fmt.out; test ! -s /tmp/dw-fmt.out` | exit 0 |
| 7 | check:ci | `cd statusgen && go run . --root .. --lint; echo $?` | 0 |

Pre-mortem → detection map:

| Failure mode of the work | Caught by |
|---|---|
| The prunable reading also drops the 0-ahead proof | row 4 |
| A live holder is skipped because the attribute parse is sticky across entries | row 3 (the fixture lists the prunable entry BEFORE the live one) |
| `add` starts running `git worktree prune` and acquires a second mutation | review-only — the diff must touch no exec of `prune` in `cmdAdd` |

## Evidence
<!-- appended at implementation time: one witness row per Verify row —
     (command, exit code, output line(s), date, runner). -->

## Review

Gate: model (all four risk answers no). The reviewer confirms row 4 is present and red against
a naive "prunable ⇒ reclaim" implementation, and that `cmdAdd` gained no new git mutation.
