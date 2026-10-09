### Changed
- The desk tools' `fetch` now runs in-process (`gitcore`) instead of spawning the git binary: `deskgit fetch` and `deskmerge`'s base and PR-head fetch carry the role App token in memory only, and `deskadvisory`'s fork-tree fetch carries the operator's GitHub credential (the same one it already used for the API) in memory only. The bespoke argv/env hardening and the askpass-script credential path are gone, and `deskadvisory` writes only the fork's tree (no repository, no credential file) before running its checks.
- A local-path or `file://` origin is fetched in-process as well: `gitcore` replaces go-git's stock local transport, which started `git-upload-pack` with the caller's whole environment, for every `Fetch`, `List` and `FetchTree` caller.
- `deskgit fetch` and `deskmerge`'s fetch no longer fetch tags: they write only their refspecs' refs and never create or replace a `refs/tags/*` ref. The git binary auto-followed tags (without replacing an existing local one); the in-process fetch follows none.

### Fixed
- `deskgit fetch --branch`/`--pr` refuses a branch checked out in any worktree of the repository, a linked one included (previously only the current worktree's branch), and stops when that set cannot be read. The set is git's own: a branch a worktree is rebasing or bisecting, and the branches an in-progress `rebase --update-refs` will rewrite, are refused too.
- A fetch from a local-path or `file://` origin succeeds from a checkout that holds commits the origin lacks, instead of failing with `object not found`.
- An in-process fetch whose origin ref was rewritten to a commit that does not descend from the local one no longer reports success with the local ref unchanged: `deskgit fetch --pr`/`--branch` refuses the non-fast-forward update and exits 6 naming the ref, as `git fetch` did.
- `deskgit fetch --prune` no longer deletes `refs/remotes/origin/HEAD`: a symbolic ref is never pruned or written through by the in-process fetch.
