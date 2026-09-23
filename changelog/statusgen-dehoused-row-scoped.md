### Fixed
- statusgen board-honesty: the `dehoused` phantom-class check now keys on the row's own
  board cell and the brief body, never the whole stream README. Previously a single row
  whose title/README merely mentioned "de-housed" (for example a brief *about* de-housing
  code) flagged every other `todo` row in that stream `NON-DISPATCHABLE (dehoused)` and
  dropped them from Next-up — the same false-positive class the `re-homed` check was already
  narrowed against (statusgen #709). No detector reads the stream README any more.
