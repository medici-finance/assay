### Fixed
- **`bodycheck` clears three measured false-positive classes** without widening what the
  secret scan admits: a doc PATH whose filename or directory segment is an exactly-32-hex
  string (`…/2026-08-30-<32hex>.md`, `…/findings/<32hex>/README.md`), a slash-separated list
  of short issue numbers (`#101/102/104/…`), and a `kind: Secret` TEMPLATE whose every value
  is a placeholder (`<…>`, `${…}`, `{{…}}`, `REDACTED`, `PLACEHOLDER`). Each fix is bounded by
  a paired POSITIVE corpus fixture of the same shape carrying a credential — a length other
  than 32, a hex pair with no word-shaped neighbour, an 8-digit numeric token, or one literal
  value among placeholders — which must still refuse, so a rule that cleared its negative by
  shape alone reds its pair.

### Added
- **`--explain` on `deskpr`, `deskpost` and `deskreply`** — on a secret-scan refusal, an
  optional stderr line names the rule id and the 1-based line of the first offending span,
  its length, and a REDACTED shape (first two + last two characters, a character-class
  summary), so a refused caller can act on the first round instead of guessing which span
  tripped it. The line NEVER prints the offending span — the refusal must not become the
  leak — and without the flag the refusal message is byte-for-byte unchanged.
