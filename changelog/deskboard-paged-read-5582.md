### Fixed
- `deskboard actions` and `deskboard prs` no longer fail on a repository with many open pull requests: the open-PR reads now walk the queue in pages of 25 and, when the forge answers a page with a 504, an empty 200 or cut-off JSON, retry that same page at half the size (floor 6, at most 4 smaller retries per walk) before reporting could-not-check. Authentication, permission and rate-limit answers are still never retried, and a queue larger than the 100-change cap is still reported as truncated.

### Added
- `forgeread/adapters/pager`: one shared, forge-neutral cursor walk with adaptive page-size backoff, a caller-supplied transient classifier and request accounting. It carries no transport or credential, so the desk tools, statusgen and the future forge client can share it.
