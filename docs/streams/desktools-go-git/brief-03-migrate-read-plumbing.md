---
brief: desktools-go-git/03
title: migrate read/plumbing verbs (read-heavy tools) to gitcore
wave: 3
depends: ["desktools-go-git/01", "desktools-go-git/02"]
unblocks: ["desktools-go-git/05", "desktools-go-git/06", "desktools-go-git/08"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/spec.md — decision 3 (seam-swap, not rewrite)"
  - "docs/streams/desktools-go-git/inventory.md — op families 12-25 (read-only plumbing) and their seam sites"
  - "docs/streams/desktools-go-git/brief-02-gitcore-transport-auth.md — the gitcore read helpers this brief routes callers onto"
why: >-
  The bulk of the git surface is read-only plumbing — rev-parse, rev-list, merge-base,
  diff, log, cat-file, show, for-each-ref, status, config reads, remote get-url. These
  map cleanly onto gitcore's read helpers and carry no auth. Migrating them first (behind
  goldens, no behaviour change) shrinks the git-exec count the CI gate tracks and de-risks
  the transport briefs that follow.
---

# Brief 03 — migrate read/plumbing verbs to gitcore

## Context

files:
- `tools/desk/cmd/writeguard/guard.go`, `tools/desk/cmd/writeguard/main.go` — `rev-parse` reads.
- `tools/desk/cmd/desksourceguard/verify.go` — `rev-parse` reads.
- `tools/desk/cmd/deskboard/board.go`, `tools/desk/cmd/deskboard/main.go` — `rev-parse` reads (dozens of tiny
  reads per run — likely faster in-process).
- `tools/desk/cmd/deskscanbody/main.go` — `merge-base`, `diff` reads.
- `tools/desk/internal/deskkit/preflight.go` — read paths only: `remote get-url`, `symbolic-ref`,
  `config --get`, `rev-parse` (the transport dry-run PROBE is brief 06, not here).
- `tools/desk/cmd/deskwt/deskwt.go`, `tools/desk/cmd/deskwt/prune.go`, `tools/desk/cmd/deskwt/ambiguousbase.go`,
  `tools/desk/cmd/deskwt/roleinit.go` — read paths: `status`, `rev-parse`, `rev-list`,
  `for-each-ref`, repo/global `config --get` reads. (Linked-worktree add/remove/lock/
  prune and per-worktree `config --worktree` STAY on the binary — out of scope, see spec.)
- `tools/desk/cmd/deskgit/deskgit.go` — read paths: `for-each-ref`, `rev-parse`
  (`ls-remote --get-url` collapses to reading `remote.origin.url` — no insteadOf layer).
  (deskgit's `fetch` is brief 05.)
- `tools/desk/cmd/deskpr/deskpr.go`, `tools/desk/cmd/deskreply/deskreply.go` — `rev-parse`, `rev-list`,
  `symbolic-ref`, `diff`, repo `config --get` reads. (Their `push` is brief 06.)
- `docs/streams/desktools-go-git/inventory.md` (planned) — tick migrated op families.

facts:
- Mapping (go-git API): `rev-parse` -> `Repository.ResolveRevision` / `Repository.Head`;
  `--show-toplevel`/`--git-common-dir` -> `PlainOpenWithOptions{DetectDotGit}` + storer
  paths; `<ref>:path` tree-id -> `commit.Tree()` walk; `rev-list` -> `Repository.Log` +
  count (left-right = two counted walks from merge-base); `merge-base` -> `commit.MergeBase`
  + `IsAncestor`; `diff` -> `commit.Patch`/`Tree.Diff` (name-only + unified, rename
  detection `-M`); `for-each-ref` -> `Repository.References`; `remote get-url` ->
  `Repository.Remote("origin").Config().URLs[0]`; `config --get` -> `Repository.Config`;
  `status` -> `Worktree.Status`; `cat-file -e`/`show` -> object API.
- Pure swaps, golden-verified, NO behaviour change. The brief-01 harness snapshots the
  outcome of each migrated read against the same fixtures the old argv tests used.
- Parity risks to cover in goldens: diff rename detection (`-M`, used by deskscanbody),
  `status --untracked-files=no` semantics, `@{u}` upstream resolution, and an
  index-extension case (a fixture worktree the local `git` binary has touched) — go-git
  does not model index v1/v3, so the golden must prove the read still resolves.
- deskpushguard is migrated separately in brief 04 (it is a security-detection control
  and gets its own parity + mutation test). deskmerge's read verbs are in brief 07.
- Out of scope: transport verbs (fetch/push), linked-worktree ops, per-worktree config,
  any behaviour change.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. For each tool above, replace its read/plumbing seam calls with `gitcore` read helpers,
   keeping the callers and their return contracts intact.
2. Convert each affected tool's argv-asserting tests to brief-01 outcome goldens; add the
   parity-risk goldens named in facts (rename detection, untracked-files, `@{u}`,
   index-extension worktree).
3. Tick the migrated op families in `inventory.md`; leave the transport/worktree/merge
   rows explicitly un-ticked with their owning brief noted.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/writeguard/ ./cmd/desksourceguard/ ./cmd/deskboard/ ./cmd/deskscanbody/ ./cmd/deskwt/ ./cmd/deskgit/ ./cmd/deskpr/ ./cmd/deskreply/ ./internal/deskkit/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/writeguard/ ./cmd/desksourceguard/ ./cmd/deskboard/ ./cmd/deskscanbody/ ./cmd/deskwt/ ./cmd/deskgit/ ./cmd/deskpr/ ./cmd/deskreply/ ./internal/deskkit/` | exit 0; migrated reads pass their outcome goldens |
| 3 | `cd tools/desk && go test ./cmd/deskscanbody/ -run Rename` | exit 0; diff rename-detection parity golden passes |
| 4 | `sh tools/desk/scripts/count-git-exec.sh` | prints `git-exec sites: <N>`; N strictly below the brief-01 baseline recorded in this stream's PRs |
| 5 | `cd tools/desk && grep -rcE 'gitcore\.' cmd/deskboard/board.go` | exit 0; count >= 1 (deskboard rev reads now route through gitcore) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — behaviour-preserving read-only refactor, golden-
verified, no auth and no writes touched). Reviewer records verdict + date in the stream
README table.
