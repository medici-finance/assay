### Fixed
- `gitcore.PinnedRemoteURLs` no longer accepts a worktree-scope `remote.<name>.url` key written with no value (a bare `url` line) as the empty reset. Git refuses that shape as missing a value, so the reader now refuses it with `ErrRemoteScopeUnmerged` and names the valueless key; the explicit `url =` reset followed by one URL still resolves as before.
