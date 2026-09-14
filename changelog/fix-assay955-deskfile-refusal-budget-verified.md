### Added
- `deskfile new`'s per-session budget accounting (`chargedNewEntry`) now carries an explicit
  citation and regression tests (`TestBudgetBodyCheckRefusalDoesNotConsumeSlot`,
  `TestBudgetDedupeRefusalDoesNotConsumeSlot`) proving that a REFUSED `new` — a BodyCheck
  secret-scan hit or a dedupe match — is audited but does not consume the 3-per-24h
  session budget slot: only a write that reaches `gh issue create` may charge it. The
  existing `ResultRefused` exclusion in `chargedNewEntry` already implemented this; these
  tests close the coverage gap end-to-end and pin the behaviour against regression. (#955)
