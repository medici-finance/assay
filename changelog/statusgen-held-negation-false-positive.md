### Fixed
- `statusgen`'s HELD/could-not-check Evidence scan no longer refuses a
  genuinely clean PASS whose prose merely reports the ABSENCE or COUNT of a
  held state ("VERIFY: PASS ... no could-not-check", "summary: 0 HELD") — only
  an actual, un-negated disposition still contradicts the PASS. This NARROWS a
  flip-refusal detector (the model autoflip, the verify-gate card and
  closeVerify's `verified` path), so the excusal fails closed and what may
  precede a cue is an allowlist. A "no"/"not"/"zero" cue excuses only after
  the line start, a list marker, a count label ("summary:"), a clause break
  (",", ";", ".", "(", an em or en dash, "→") or a linking word ("is", "with",
  "and", …) — so not right after "?", "=", "|", ")", "-", a non-count ":"
  label (bold or plain), struck text or any other word ("rc zero HELD"). A
  "0" cue excuses only in a count position: the line start, a list marker, a
  count label, or right after a verdict-count item ("7 PASS, 0 HELD"). An
  occurrence followed — past whitespace, emphasis, or "," ";" ":" "(" or a
  dash — by a hold-reason word ("pending", "awaiting", "until", "for", …), or
  by a colon, is never excused. The reason word's own boundary accepts a
  trailing underscore as well as a normal word boundary, so a markdown-
  emphasised reason ("0 HELD _pending_", "0 HELD __pending__") is still
  detected — underscore is a word character, so a bare `\b` would otherwise
  miss it. Residuals: a clause break or linking word
  admits whatever precedes it ("runner available — no HELD" is excused), a
  hold reason that uses none of the reason words ("0 HELD — runner offline")
  is not detected, a hold-reason word that follows a sentence break ("."),
  "→" or ")" is not in the lookahead's skip set and so does not block the
  excusal ("0 HELD. pending runner" is excused), and each physical line is
  judged on its own.
