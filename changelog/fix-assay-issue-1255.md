### Fixed
- `statusgen --scan-issues` — the scan `scanloop run` shells in its scan lane — no longer shells out to `gh` for any forge read. The trust-gate blessing read (was `gh api graphql`) and the un-block comment read (was `gh api --paginate`) now go through the `deskread` verb on the native `Forge` seam, completing what #1223 started for the open-issue list. Under the replaced `HOME` a scanloop pass runs with, those two reads returned `gh: HTTP 401` on every rostered repo; the native client attaches the per-installation App token explicitly on every request. (#1255)

### Added
- `deskread` serves two per-issue read kinds, `trust` (`Forge.IssueTrustEvents`) and `comments` (`Forge.ListCommentsTyped` on an issue), addressed as `--issue owner/name#N`. Their envelope is keyed by repo and number and keeps the same partial-is-a-result contract as the `issues` kind. The `issues` kind's output is unchanged.

### Changed
- The GitHub backend's issue-thread read (`ListCommentsTyped` on an issue) now follows the comment connection's cursor to the end of the thread, capped at 20 pages. A thread longer than the cap, or a thread that reports another page without giving a cursor for it, is now could-not-check, where before it was cut to its first 100 comments without any warning. Reads of pull-request comment threads are unchanged.
