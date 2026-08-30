# desktools-go-git — frozen op-family inventory (brief 01 baseline)

**Frozen at:** main @ the brief-01 branch point (public assay). **How it was built:**
live grep of `exec.Command("git"` spawns and the per-tool seam helpers
(`runGit` / `gitOut` / `execCommand` / `execGit`) across `tools/desk`, then per-tool
verb extraction. The feasibility study's 25 op families are the key; each row names
every seam site the family appears at, and its go-git mapping or gap.

**Migration checklist contract:** every migration brief (02–07) ticks the rows it
empties, in its own Evidence. A row is emptied when every seam site on that row
routes through `internal/gitcore` — or is re-keyed to the `gitexec` fallback with a
cited reason. **The CI counter** (`tools/desk/scripts/count-git-exec.sh`) counts the
spawn + seam sites this table describes; its baseline N is recorded in brief 01's PR
body and must only decrease.

## Mapping legend

- **mapped** — go-git covers it cleanly (see spec § "go-git coverage at a glance").
- **exception** — stays on the git binary by decision (the deskmerge trial merge).
- **gap — follow-on** — go-git cannot express it; named follow-on stream handles it.

## The 25 op families

| # | Family (verbs) | Tools / seam sites | go-git mapping |
|---|---|---|---|
| 1 | `init` | deskadvisory (`advisory.go` scratch clones) | mapped (`git.Init`) |
| 2 | `add` | deskmerge (`exec.go`), verifyloop (`durable.go`) | mapped (`Add`) |
| 3 | `commit` | deskmerge (`exec.go`), verifyloop (`durable.go`) | mapped (explicit Author/Parents; `--no-verify` inherent — no hooks ever run) |
| 4 | `checkout <sha>` | deskadvisory (`advisory.go`) | mapped (`Checkout`) |
| 5 | `status` | deskwt (`exec.go`), deskboard (`board.go`) | mapped (`Status`) |
| 6 | `rev-parse` (path + sha plumbing) | deskwt (`exec.go` ×16), deskpr (`exec.go`), deskpushguard (`foreigncommit.go`), writeguard (`main.go`), desksourceguard (`verify.go`), deskreply (`exec.go`), deskgit (`exec.go`), deskboard (`board.go`, `main.go`) | mapped (`ResolveRevision`, `PlainOpen`; `--git-common-dir`/`--path-format` variants via Repo struct) |
| 7 | `symbolic-ref` | deskpr (`deskpr.go`), deskkit preflight (`preflight.go`) | mapped (Head / ref resolution) |
| 8 | `for-each-ref` | deskgit (`exec.go`) | mapped (`References` iterator) |
| 9 | `ls-remote` (+ `remote get-url`) | deskgit (`exec.go`), deskkit preflight (`preflight.go` — transport probe) | mapped (`remote.List`; the effective URL *is* `remote.origin.url` — the `insteadOf` smuggle vector disappears) |
| 10 | `log` | deskpushguard (`foreigncommit.go`) | mapped (`Log`) |
| 11 | `show` | deskpushguard (`foreigncommit.go`) | mapped (CommitObject / blob content) |
| 12 | `cat-file` | deskpushguard (`foreigncommit.go`) | mapped (object reads) |
| 13 | `ls-tree` | deskpushguard (`foreigncommit.go`) | mapped (tree walk) |
| 14 | `diff` | deskpr (`deskpr.go`), deskmerge (`exec.go`), deskscanbody (`main.go`), deskboard (`board.go`, `main.go`) | mapped (name-only + unified, rename detection) |
| 15 | `merge-base` / `is-ancestor` | deskmerge (`exec.go`), deskwt (`exec.go`), deskscanbody (`main.go`), deskpushguard (`foreigncommit.go`) | mapped (`MergeBase` / `IsAncestor`) |
| 16 | `rev-list` | deskmerge (`exec.go`), deskwt (`exec.go`), deskpr (`deskpr.go`), deskpushguard (`foreigncommit.go`) | mapped (commit iteration) |
| 17 | `config` | deskwt (`exec.go`), deskpr (`deskpr.go`), deskreply (`exec.go`), deskkit preflight (`preflight.go`) | mapped for reads; **per-worktree config = gap — follow-on** |
| 18 | `remote` (add/rename) | deskmerge (`exec.go`), deskpushguard (`foreigncommit.go`) | mapped (remote ops) |
| 19 | `fetch` | deskmerge (`exec.go` — base/PR), deskadvisory (`advisory.go` — third-party fork, hardened) | mapped (in-process transport, `BasicAuth` token-as-value; the fork-fetch hardening flag suite disappears — nothing to pin) |
| 20 | `push` | deskpr (`deskpr.go`), deskmerge (`exec.go`), verifyloop (`durable.go` — durable-Evidence push half), deskkit preflight (`preflight.go` — transport probe) | mapped (exact refspecs; force-push impossible by type; preflight probe → authenticated `List`) |
| 21 | `worktree` (linked worktrees) | deskwt (`exec.go` ×9), deskmerge (`exec.go` ×3), verifyloop (`dispatch_native.go` — `worktree add/remove`) | **gap — follow-on** (go-git has no linked-worktree support; agents must keep real linked worktrees) |
| 22 | `merge` (three-way trial) | deskmerge (`merge.go`) | **exception** — the single sanctioned git-binary caller; human-gated, desk-machine-only |
| 23 | `update-ref` | deskmerge (`exec.go`) | mapped (`Storer.SetReference`) |
| 24 | `clean` | deskscanbody (`main.go`) | mapped (`Clean`) |
| 25 | `pull --rebase` | verifyloop (`durable.go` — durable-Evidence push race) | **gap — follow-on** (go-git supports neither rebase nor non-FF pull; stream migrates only the push half) |

## Seam-site legend (where the families live today)

| Tool | Seam | Shape |
|---|---|---|
| deskgit | `tools/desk/cmd/deskgit/exec.go` | `runGit` + env allowlist (issue #1555) |
| deskmerge | `tools/desk/cmd/deskmerge/exec.go` | `execCommand` seam |
| deskwt | `tools/desk/cmd/deskwt/exec.go` | `execCommand` seam |
| deskscanbody | `tools/desk/cmd/deskscanbody/exec.go` | `gitOut` |
| deskpr | `tools/desk/cmd/deskpr/exec.go` | `execCommand` seam |
| deskreply | `tools/desk/cmd/deskreply/exec.go` | `runCmd`/`git` wrapper — READ-ONLY by design (no push path exists) |
| deskadvisory | `tools/desk/cmd/deskadvisory/advisory.go` | direct `exec.Command("git"` + fork-fetch hardening |
| deskpushguard | `tools/desk/cmd/deskpushguard/foreigncommit.go` | direct spawns |
| deskboard | `tools/desk/cmd/deskboard/board.go`, `main.go` | direct one-off spawns |
| writeguard | `tools/desk/cmd/writeguard/main.go` | direct one-off spawns |
| desksourceguard | `tools/desk/cmd/desksourceguard/verify.go` | direct one-off spawns |
| verifyloop | `tools/desk/cmd/verifyloop/durable.go`, `dispatch_native.go` | direct spawns |
| deskkit | `tools/desk/internal/deskkit/preflight.go` | preflight transport probe |

## Brief 02 — gitcore capability now backing these families

Brief 02 stands up `internal/gitcore` and its transport/auth layer but rewires no
caller — per the migration checklist contract above, a row is only **ticked/emptied**
once every seam site on it routes through `internal/gitcore` (briefs 03-07). This note
records, separately from that contract, which op families now have a working
`gitcore` implementation for those later briefs to swap callers onto, golden-verified
against the brief-01 harness (`internal/gitcore/gitcore_test.go`):

- **#6** `rev-parse` (path + sha plumbing) — `Repo.Resolve` (`ResolveRevision`).
- **#8** `for-each-ref` — `Repo.Refs`.
- **#9** `ls-remote` (+ `remote get-url`) — `gitcore.List` (no local repo required).
- **#10** `log` — `Repo.Log`.
- **#11** `show` / **#12** `cat-file` — `Repo.FileAt` (object/blob reads).
- **#13** `ls-tree` — `Repo.Files`.
- **#14** `diff` — `Repo.DiffNames` (name-only, with rename detection).
- **#15** `merge-base` / `is-ancestor` — `Repo.MergeBase` / `Repo.IsAncestor`.
- **#19** `fetch` — `Repo.Fetch` (explicit `URL` + per-call `Auth`).
- **#20** `push` — `Repo.Push` (explicit `URL` + per-call `Auth`; `Force` is
  type-level — off unless set).

Not yet covered by this brief (left for the briefs that need them): `init`/`add`/
`commit`/`checkout`/`status`/`config`/`remote add-rename`/`update-ref`/`clean` (op
families #1-5, #17-18, #23-24) — none of Fetch/Push/List/the read helpers above
requires them, and adding them here would be scope creep past this brief's own Task.

## Baseline counter

`sh tools/desk/scripts/count-git-exec.sh` — see the brief-01 PR body for the recorded
baseline N. The gate stays advisory (exit 0) until brief 08.
