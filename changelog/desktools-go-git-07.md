### Changed
- desktools-go-git/07: fenced `deskmerge`'s trial merge (`merge --no-ff --no-commit`, its `--diff-filter=U` conflict-path enumeration, `merge --abort`, and the regenerable-conflict `add`) through `internal/gitexec` as the sole sanctioned git-binary caller for the tool, under a narrow (tool, verb) allowlist entry. Migrated every OTHER `deskmerge` git verb to in-process `gitcore`: `rev-parse`, `merge-base`, the `--left-right --count`/`--parents` `rev-list` reads, the non-conflict `diff` reads (CI-contract drift, semantic-probe scoping), `remote get-url`, `commit`, and `update-ref -d`. The tracked git-exec counter drops from 108 to 96 sites.
- `internal/gitcore` gained its first WRITE helpers: `Commit` (explicit `Parents` — a merge commit's shape is now a construction property, not something a separate `rev-list --parents` read has to verify after the fact), `CommitParents`, and `DeleteLocalRef`.

### Fixed
- N/A — no defect fixed by this brief; see "Not migrated" below for a gap found and deliberately fenced rather than worked around.

### Not migrated (documented, deliberate)
- `deskmerge`'s regenerable-conflict `add` stays on the git binary (fenced, not deferred): verified empirically that go-git's `Worktree.Add` does not clear a path's merge-conflict index stages (1/2/3) — the on-disk index still lists them as unmerged afterward, and a `gitcore.Commit` built from that index writes a tree with duplicate entries for the path (`git fsck`: `duplicateEntries`). Staging the resolved path with the git binary first, then committing via `gitcore`, produces a clean result; that's the sequence deskmerge now runs.
- `deskmerge`'s scratch-worktree family (`worktree` add/remove/prune) and its transport verbs (`fetch`/`push`) are untouched — explicitly out of scope for this brief (worktrees are the named follow-on stream's gap; fetch/push are briefs 05/06).
