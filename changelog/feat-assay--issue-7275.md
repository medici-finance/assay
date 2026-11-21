### Fixed

- `deskdispatch` verifier dispatches now prune verification snapshots under the user cache directory (`assay/verify-snap`) after a week of inactivity, judged by both modification and inode change time, on macOS and Linux; a build for any other platform reads no change time, so it keeps every snapshot and names each on stderr as could-not-check. The dispatched claim's own snapshot is never removed, and `--dry-run` lists every snapshot with its verdict.
- A managed cell Go cache now exports `GOTMPDIR` under its root, and every maintenance pass removes `go-build*` work directories left by killed builds once they are past the build trim age and older than every live consumer's enrollment. Codex launches forward `GOTMPDIR` with the other managed Go variables.
