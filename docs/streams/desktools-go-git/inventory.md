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
| deskgit | `cmd/deskgit/exec.go` | `runGit` + env allowlist (issue #1555) |
| deskmerge | `cmd/deskmerge/exec.go` | `execCommand` seam |
| deskwt | `cmd/deskwt/exec.go` | `execCommand` seam |
| deskscanbody | `cmd/deskscanbody/exec.go` | `gitOut` |
| deskpr | `cmd/deskpr/exec.go` | `execCommand` seam |
| deskreply | `cmd/deskreply/exec.go` | `runCmd`/`git` wrapper — READ-ONLY by design (no push path exists) |
| deskadvisory | `cmd/deskadvisory/advisory.go` | direct `exec.Command("git"` + fork-fetch hardening |
| deskpushguard | `cmd/deskpushguard/foreigncommit.go` | direct spawns |
| deskboard | `cmd/deskboard/board.go`, `main.go` | direct one-off spawns |
| writeguard | `cmd/writeguard/main.go` | direct one-off spawns |
| desksourceguard | `cmd/desksourceguard/verify.go` | direct one-off spawns |
| verifyloop | `cmd/verifyloop/durable.go`, `dispatch_native.go` | direct spawns |
| deskkit | `internal/deskkit/preflight.go` | preflight transport probe |

## Baseline counter

`sh tools/desk/scripts/count-git-exec.sh` — see the brief-01 PR body for the recorded
baseline N. The gate stays advisory (exit 0) until brief 08.
