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

## Brief 07 — deskmerge exception fenced; its non-merge verbs migrated

Brief 07 fenced deskmerge's trial merge as the SOLE sanctioned `internal/gitexec`
caller and migrated everything else on its own seam to `gitcore`. Per the migration
checklist contract, a family row is ticked once EVERY seam site the frozen table
lists for it routes through `gitcore` or is re-keyed to `gitexec` with a cited reason.

| # | Family | Ticked? | Note |
|---|---|---|---|
| 2 | `add` | **re-keyed (exception)** | deskmerge's ONE `add` (regenerable-conflict resolution, `merge.go`) is NOT migrated — verified empirically that go-git's `Worktree.Add` cannot clear a path's conflict-stage (1/2/3) index entries; a `gitcore.Commit` built from that index writes a tree with DUPLICATE ENTRIES (`git fsck`: `duplicateEntries`). Fenced through `internal/gitexec` beside the merge it resolves — see `tools/desk/internal/gitcore/write.go`'s doc for the full experiment. verifyloop's `add` (durable.go) is untouched (not this brief's Context) |
| 3 | `commit` | partial | deskmerge's ONE `commit` (`merge.go`'s `commitMerge`) migrated — `gitcore.Commit` with explicit `Parents`, no separate `rev-parse HEAD` read-back needed. verifyloop's `commit` (durable.go) is untouched, brief unassigned |
| 6 | `rev-parse` | partial | deskmerge's 3 sites (`resolveRepoRoot`'s checkout-validity check — now `gitcore.Open`; `fetchState`'s post-fetch base/head resolution — now `Repo.Resolve`) migrated. NOTE: this family's frozen tool list (above) never named deskmerge as a seam site for it — a gap in the brief-01 freeze, not a re-scoping; recorded here so a later audit does not read deskmerge as never having had rev-parse sites. deskpushguard's sites remain brief 04's |
| 14 | `diff` | partial (deskmerge side ticked+exception) | deskmerge's CI-contract-drift diff and the semantic-probe's changed-path diff (both plain two-tree reads, `assess.go`) migrated to `Repo.DiffNames`, filtered client-side (`underAny`) where the git-binary call carried a pathspec — `gitcore.DiffNames` takes none. deskmerge's OTHER `diff` — the `--diff-filter=U` conflict-path enumeration/residual-check, `currency.go`'s `conflictedPaths` and `merge.go`'s post-regeneration check — reads the SAME mid-merge conflict-stage index the trial merge produces and is re-keyed to `gitexec` beside it, for the same reason as `add` above. deskboard's site remains untouched (not named in any brief's Context yet) |
| 15 | `merge-base` | partial | deskmerge's site (`assess.go`) migrated — `Repo.MergeBase`. deskpushguard's site remains brief 04's |
| 16 | `rev-list` | partial | deskmerge's two uses migrated: the `--left-right --count` ahead/behind measurement is now two `Repo.AheadCount(mergeBase, X)` calls (both sides counted from the already-computed merge base, which is exactly what the two-dot count means when — as here — the merge base truly is a common ancestor); the `--parents -n1` post-commit parent check is now `Repo.CommitParents`, a new read added in this brief. deskpushguard's sites remain brief 04's |
| 18 | `remote` | partial | deskmerge's ONE site (`resolveRepoRoot`'s `remote get-url origin`) migrated — `Repo.RemoteURL` (already existed, brief 03). deskpushguard's site (`foreigncommit.go`) remains untouched, brief unassigned |
| 21 | `worktree` (linked worktrees) | **untouched, unticked** | deskmerge's 3 scratch-worktree sites (`newWorktree`/`remove`) are explicitly OUT OF SCOPE for brief 07 (the brief's own Context: "scratch/linked worktree ops" excluded) — still the named follow-on stream's gap, not re-justified here |
| 22 | `merge` (three-way trial) | **ticked as fenced** | the trial merge itself now runs through `internal/gitexec` (`gitexec.Run("deskmerge", …)`) under a narrow allowlist entry, rather than deskmerge's own ad hoc exec seam — same git-binary op, now the audited one. Still THE decided exception; see brief 07 and the spec's decision 5 |
| 23 | `update-ref` | **ticked** | deskmerge's ONE site (`dropPRHeadRef`) migrated — `Repo.DeleteLocalRef`, a new write added in this brief (matches `git update-ref -d`'s own no-op-on-absent behaviour) |

New `gitcore` write helpers this brief added, in `tools/desk/internal/gitcore/write.go` (golden-
verified in `gitcore_test.go` against the real git binary reading the result back —
there is no pre-existing git-binary golden for a write helper to diff against, since
deskmerge is the stream's first migrated WRITE caller): `Commit` (explicit `Parents`,
identity falls back to go-git's own config resolution exactly as `git commit` does
with no identity flags — untouched by the worktree-scoped-config gap below, since no
deskmerge checkout in this stream's fixtures uses it), `CommitParents`, `DeleteLocalRef`.

**Bug found and NOT worked around in `gitcore`, by design (fenced instead) — same
class as the two `gitcore` bugs brief 03 found and fixed:** go-git's `Worktree.Add`
does not clear a path's conflict-stage (1/2/3) index entries left by a real
`git merge` conflict; `tools/desk/internal/gitcore/write.go`'s doc carries the full reproduction.
Unlike brief 03's two bugs (a storer wrapper and a linked-worktree path fix), this one
is not a `gitcore`-side workaround to build: it is the SAME class of gap as the trial
merge's own (no three-way merge, no conflict-stage awareness), so the fix is fencing
the one call site that touches it, not extending `gitcore`.

## Baseline counter

`sh tools/desk/scripts/count-git-exec.sh` — see the brief-01 PR body for the recorded
baseline N (117). Brief 03 left it at **108** (149 immediately before brief 03,
mid-stream after brief 02). Brief 07 leaves it at **96** (12 deskmerge seam-call sites
retired: the 3 `rev-parse`, `merge-base`, the `--left-right --count` `rev-list`, the
2 non-conflict `diff` reads, `remote get-url`, `commit`, the post-commit `rev-list
--parents`, and `update-ref` — all now `gitcore` calls with no `runGit(`/`gitOut(`/
`execCommand(`/`execGit(` text at the call site at all). The gate stays advisory
(exit 0) until brief 08.
