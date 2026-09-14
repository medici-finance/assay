### Changed
- The audit ledger is no longer parsed whole to answer questions that need one line.
  `Guard()` reads the last entry by a bounded tail read, and the rate limiter reads
  backwards only as far as its own meters would have read — falling back to the full parse
  whenever its answer is not provably identical, so no budget, breaker or idempotency
  verdict changes. Measured on a 109 MB / 460,000-row ledger: the guard's p50 goes from
  883 ms to 0.09 ms (#1035).
- `desktoken` no longer appends an audit row when it serves a cached token. One row per
  real mint, and every refusal and failure still recorded.
- The ledger rotates daily into `audit.jsonl.<YYYY-MM-DD>` segments. Nothing is deleted and
  no state is reset: every reader — including `deskaudit recover` — spans the segments, so
  the rate-limit counter and the idempotency store see exactly the history they saw before.

### Added
- `deskaudit tail [N]` — print the newest N ledger entries (default 10) across the daily
  segments, as the raw lines they are on disk.
