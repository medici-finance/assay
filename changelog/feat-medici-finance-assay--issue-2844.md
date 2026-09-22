### Fixed
- `issueboard`'s escalation clock no longer fails the WHOLE board when one
  decision-owed issue's comment thread exceeds a single page. The clock now reads
  the whole thread through a new bounded, paginated forge read
  (`Forge.IssueContentEvents`) instead of sharing the trust gate's deliberately
  single-page read, which had returned could-not-check (exit 6) — and took down
  the board for every scanned repo — the moment one thread overflowed 100
  comments. A thread that even bounded pagination cannot walk to the end degrades
  that ONE row conservatively (rendered ESCALATE with a could-not-check marker)
  while the rest of the board renders; an overflowed thread is never read as "no
  escalation owed". (#2844)
