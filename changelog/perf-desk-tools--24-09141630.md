### Added
- desk-tools brief 24 — the audit ledger's read cost: a bounded tail read for `Guard`'s
  last-entry question, a bounded reverse read for the rate limiter that falls back to the full
  parse whenever its answer is not yet determined, no audit row for a `desktoken` cache reuse,
  daily `audit.jsonl.<date>` rotation that deletes nothing and carries the counter and the
  idempotency store forward, and a `deskaudit tail` read verb (#1035).
