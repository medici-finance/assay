### Fixed
- `cellctl desk` no longer boots on a previous boot's `main` when the checkout's `FETCH_HEAD` is read-only. git could not rewrite the file, the fetch failed, and the old sha was read. `FETCH_HEAD` is now removed before the fetch, and a boot whose `FETCH_HEAD` cannot be removed is refused (exit 3, fetch lock released).
- A boot fetch that fails outright (an expired credential, say) now refuses with exit 3, names the likely cause and the way to recover, and releases the fetch lock instead of leaving it for the next boot to wait out.
