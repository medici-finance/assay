### Changed
- `deskboard` re-reads only what changed: GitHub reads are sent as conditional requests (ETag / Last-Modified, served from a cache under the desk-tools state dir only after the forge answers 304), and `actions` reuses a PR's changed-file reads while its head, `updatedAt`, base and check rollup are unchanged — a repeat sweep of an unchanged board now costs a constant handful of charged calls for PRs by trusted authors instead of growing with the open-PR count (each open PR by an untrusted author still costs one uncached GraphQL trust-blessing read per sweep).

### Added
- `deskboard actions` reports what each sweep spent (`cost` in the JSON header plus one `deskboard: cost …` stderr line, with the lowest remaining quota and its reset time) and refuses to print a board when the quota drops below a floor (`--rate-floor` / `DESKBOARD_RATE_FLOOR`, default 500), exiting 4 with the reset time instead of a partial board.
