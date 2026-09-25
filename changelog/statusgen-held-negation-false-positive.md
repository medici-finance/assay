### Fixed
- `statusgen`'s HELD/could-not-check Evidence scan no longer refuses a
  genuinely clean PASS whose prose merely reports the ABSENCE or COUNT of a
  held state ("VERIFY: PASS ... no could-not-check", "summary: 0 HELD") — only
  an actual, un-negated disposition still contradicts the PASS. This NARROWS a
  flip-refusal detector (the model autoflip, the verify-gate card and
  closeVerify's `verified` path), so the excusal fails closed: a "no"/"not"/
  "zero" or "0" cue excuses only in a count or negation position, never after
  a question, a field or exit-code label, a table cell, struck text, or when
  the marker is followed by a hold reason ("pending", "until", "for", …).
