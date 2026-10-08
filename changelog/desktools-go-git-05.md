### Changed
- The desk tools' `fetch` now runs in-process (`gitcore`) instead of spawning the git binary: `deskgit fetch` and `deskmerge`'s base and PR-head fetch carry the role App token in memory only, and `deskadvisory`'s fork-tree fetch carries the operator's GitHub credential (the same one it already used for the API) in memory only. The bespoke argv/env hardening and the askpass-script credential path are gone, and `deskadvisory` writes only the fork's tree (no repository, no credential file) before running its checks.
- A local-path or `file://` origin is fetched in-process as well: `gitcore` replaces go-git's stock local transport, which started `git-upload-pack` with the caller's whole environment, for every `Fetch`, `List` and `FetchTree` caller.

### Fixed
- `deskgit fetch --branch`/`--pr` refuses a branch checked out in any worktree of the repository, a linked one included (previously only the current worktree's branch), and stops when that set cannot be read.
