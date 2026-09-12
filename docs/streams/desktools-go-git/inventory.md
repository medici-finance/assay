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
| deskscanbody | retired by desktools-go-git/03 (was tools/desk/cmd/deskscanbody/exec.go, deleted; callers now call `internal/gitcore` directly from `tools/desk/cmd/deskscanbody/main.go`) | `gitOut` (retired) |
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
against the brief-01 harness (`tools/desk/internal/gitcore/gitcore_test.go`):

- **#6** `rev-parse` (path + sha plumbing) — `Repo.Resolve` (`ResolveRevision`).
- **#8** `for-each-ref` — `Repo.Refs`.
- **#9** `ls-remote` (+ `remote get-url`) — `gitcore.List` (no local repo required).
- **#10** `log` — `Repo.Log`.
- **#11** `show` / **#12** `cat-file` — `Repo.FileAt` (object/blob reads).
- **#13** `ls-tree` — `Repo.Files`.
- **#14** `diff` — `Repo.DiffNames` (name-only, with rename detection at git's own
  50% similarity threshold, so a detected rename yields only the new path exactly as
  `git diff --name-only` does; go-git's plain `tree.Diff` would report both endpoints).
- **#15** `merge-base` / `is-ancestor` — `Repo.MergeBase` / `Repo.IsAncestor`.
- **#19** `fetch` — `Repo.Fetch` (explicit `URL` + per-call `Auth`).
- **#20** `push` — `Repo.Push` (explicit `URL` + per-call `Auth`; `Force` is
  type-level — off unless set).

Not yet covered by this brief (left for the briefs that need them): `init`/`add`/
`commit`/`checkout`/`status`/`config`/`remote add-rename`/`update-ref`/`clean` (op
families #1-5, #17-18, #23-24) — none of Fetch/Push/List/the read helpers above
requires them, and adding them here would be scope creep past this brief's own Task.

## Brief 03 — read-heavy tools migrated; rows ticked/emptied vs. still-owed

Brief 03 migrated every seam site named in its own Context section — the READ-ONLY
call sites of `writeguard`, `desksourceguard`, `deskboard`, `deskscanbody`, `deskwt`,
`deskgit`, `deskpr`, `deskreply`, plus `deskkit` preflight's non-probe reads — onto the
brief-02 `gitcore` helpers above (extended with the read families the table below
names). Per the migration checklist contract, a family row is **ticked (emptied)**
only when EVERY seam site the frozen table lists for it now routes through
`internal/gitcore`; a family with a remaining site elsewhere (deskpushguard's brief-04
sites, deskmerge's brief-06/07 sites, the transport/worktree/config-write exceptions)
stays **un-ticked**, its owning brief named.

| # | Family | Ticked? | Note |
|---|---|---|---|
| 5 | `status` | partial | deskwt's `status --porcelain --untracked-files=no` migrated (`Repo.DirtyTrackedPorcelain`); deskboard's `status` seam site is untouched (not named in brief 03's Context) |
| 6 | `rev-parse` | partial | writeguard, desksourceguard, deskboard, deskwt, deskgit, deskpr, deskreply migrated; deskpushguard's sites are brief 04 |
| 7 | `symbolic-ref` | **ticked** | deskpr and deskkit preflight are brief 03's whole seam-site set for this family (`Repo.SymbolicRefTarget` / `Repo.SymbolicRefShortHEAD`) |
| 8 | `for-each-ref` | **ticked** | deskgit's only site migrated (`Repo.LocalBranchNames`); NOTE: deskwt's `for-each-ref --contains=` (`detachedHeadOnRemote`) and `ambiguousbase.go`'s ref-candidate enumeration are a DIFFERENT family-8 shape this brief also touched/deliberately left — see the two bullets below the table |
| 9 | `ls-remote` (+ `remote get-url`) | partial | deskgit's `ls-remote --get-url` and deskkit preflight's plain `remote get-url` reads migrated (`Repo.RemoteURL`); the write-transport dry-run PROBE (`git push --dry-run`, preflight.go) is brief 06, untouched |
| 14 | `diff` | partial | deskscanbody (repo-wide, rename-aware) and deskpr (three-dot symmetric) migrated via the new `Repo.Diff`/`Repo.DiffSymmetric`; deskmerge's and deskboard's `diff` sites are untouched (deskboard's isn't named in brief 03's Context; deskmerge's is brief 06/07) |
| 15 | `merge-base` / `is-ancestor` | partial | deskscanbody (merge-base) and deskwt's `mergedToOriginMain` (is-ancestor) migrated; deskmerge's and deskpushguard's sites are untouched |
| 16 | `rev-list` | partial | deskwt's and deskpr's ahead-count sites migrated (`Repo.AheadCount`); deskmerge's and deskpushguard's sites are untouched |
| 17 | `config` | partial | ONLY `remote.<name>.url` reads migrated (`Repo.RemoteURL`) — deliberately, see below; every other `config --get`/`--list` read/write across deskwt, deskpr, deskreply, deskkit preflight stays on the git binary |

New `gitcore` read helpers this brief added (golden-verified against real git on
deterministic fixtures, `tools/desk/internal/gitcore/gitcore_test.go`): `Toplevel`,
`CommonDir`, `InsideWorkTree`, `AbbrevRefHEAD`, `SymbolicRefShortHEAD`,
`SymbolicRefTarget`, `UpstreamRef`, `AheadCount`, `RemoteURL`, `CommitVerifyQuiet`,
`HasStagedChanges`, `DirtyTrackedPorcelain`, `Diff`/`DiffSymmetric`, `TreeishID`,
`LocalBranchNames`, `RefsContaining`.

**Deliberately NOT migrated, with the reason (do not re-attempt without addressing
the reason):**

- **`tools/desk/cmd/deskwt/ambiguousbase.go`'s `refCandidates`** (the case-collision/ambiguous-`--base`
  security guard, family 8/9-adjacent) — `gitcore.Repo.Refs` (and everything built on
  it, including the new `RefsContaining`) does not surface a SYMBOLIC reference such as
  `refs/remotes/<name>/HEAD` the way real `git for-each-ref` does (verified empirically:
  go-git's reference iteration silently omits it). This guard exists specifically to
  catch every ref a short name could resolve to, so under-counting candidates would
  silently weaken it — exactly the failure mode it was written to close. Left on the git
  binary until `gitcore` can enumerate symbolic refs faithfully.
- **`config --get user.email` / `config --list -z`** (preflight's `commitEmailProbe`,
  deskpr/deskwt's `pushTransportGate` `ConfigZ` callback) — `go-git`'s
  `Repository.Config()` reads only the repository's OWN local `.git/config`; it does not
  merge a `extensions.worktreeConfig`-scoped `config.worktree` file the way real git
  does. This house's own tooling (`roleinit.go`, `workpad.go`) sets `user.name` /
  `user.email` AT THE WORKTREE SCOPE specifically so a linked worktree carries its own
  bot identity without touching the shared config — migrating these reads would have
  silently returned the wrong (or empty) value in exactly that case.

**Bug found and fixed in `gitcore` itself while wiring these callers in** (both pinned
by fail-first tests in `gitcore_test.go`): (1) go-git v5.19.2's `verifyExtensions`
lowercases an extension's name before checking its own mixed-case allowlist, so it
refused to open ANY repository carrying `extensions.worktreeConfig = true` — i.e.
almost every real worktree in this house; `gitcore.Open` now routes through a storer
wrapper scoped to exactly that one extension. (2) `gitcore.Open` on a LINKED worktree
must route config/refs/object reads through the shared common `.git`, not just the
per-worktree admin directory — without it, `RemoteURL`/`LocalBranchNames`/etc. failed
with "not found" on every linked worktree even though the shared checkout plainly has
the remote/branch.

## Baseline counter

`sh tools/desk/scripts/count-git-exec.sh` — see the brief-01 PR body for the recorded
baseline N (117). Brief 03 leaves it at **108** (149 immediately before brief 03,
mid-stream after brief 02). The gate stays advisory (exit 0) until brief 08.
