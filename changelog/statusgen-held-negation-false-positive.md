### Fixed
- `statusgen`'s HELD/could-not-check Evidence scan no longer refuses a
  genuinely clean PASS whose prose merely reports the ABSENCE or COUNT of a
  held state ("VERIFY: PASS ... no could-not-check", "0 HELD") — only an
  actual, un-negated disposition still contradicts the PASS.
