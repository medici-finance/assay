### Added
- `deskpr` accepts a resolving form of the issue link trailer — `Fixes: #N` (also `Closes:` / `Resolves:`, and `owner/repo#N` for another repository). It is the same single link edge as `Issue: #N` to every reader, and because GitHub honours the colon form as a closing keyword, merging the PR into the default branch closes the issue. `Issue: #N` keeps meaning "related, do not close".

### Changed
- A colon-form closing keyword line (`Fixes: #N`, `Closes: #N`, `Resolves: #N`) is now a link trailer, so it can no longer sit beside a `Brief:`, `Authors:`, `Cross-repo-brief:` or `Issue:` line; a brief-delivering PR that also closes an issue uses the colon-less `Closes #N`. `deskpr edit` refuses switching an existing PR between `Issue:` and the resolving form.
