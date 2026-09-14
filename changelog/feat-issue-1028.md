### Fixed
- `fanoutloop plan` no longer offers Next-up rows whose own stream README Status cell has already
  moved past `todo`/`in-progress` (implemented/verified/done/blocked): `readNextUp` now
  cross-checks each row against its own stream README before offering it, closing the gap where
  STATUS.md's rendered `## Next up` table could lag a row's live status (medici-finance/assay#1028).
