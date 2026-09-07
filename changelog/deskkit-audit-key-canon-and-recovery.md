### Fixed
- **Audit budget no longer escapes through variant tool keys.** The per-tool outward-write
  budget and the audit trail are now keyed by a single CANONICAL tool identity, resolved the
  same way wherever `audit.jsonl` is read or written (`deskkit`, `audittoolkey.go`). A binary
  invoked under a variant name — a test build (`deskpost.test`), a locally built or renamed
  copy (`deskpr-322`, `deskpr-bin`), a `go run` binary — previously wrote audit lines under
  that variant spelling and so earned a fresh, uncounted budget while splitting the audit
  trail. Variant spellings now collapse onto their tool's one budget, and a key that resolves
  to no known tool is a loud `Unverifiable` at the write gate rather than a silent new bucket.

### Added
- **`deskaudit recover`** — the sanctioned, non-destructive corruption recovery for the shared
  audit log. A single malformed line (a partial append from `kill -9`, a disk-full write, a
  sync-tool rewrite) makes every desk tool refuse. Moving the whole file aside cleared the
  corruption but RESET load-bearing state — budgets returned to full and the idempotency store
  forgot every prior write, so re-runs posted duplicates. `deskaudit recover`
  (`deskkit.RecoverCorruptAudit`) instead quarantines only the malformed line into an
  `audit.jsonl.corrupt-<ts>` sidecar and carries every good entry forward under the audit
  flock, so the rate-limit counter and the idempotency store survive the recovery. The
  corruption-refusal messages now point at this verb instead of the destructive move.
