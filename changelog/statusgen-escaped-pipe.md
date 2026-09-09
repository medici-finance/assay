### Fixed
- `statusgen` no longer reds a whole board over a legal table row. A briefs-table
  cell may hold a backslash-escaped pipe (`\|`) — in GitHub Flavored Markdown
  that is the only way to write a pipe inside a cell, and it applies inside a
  `code span` too — but the row splitter cut on every `|` byte, so such a row
  came out one cell too long and the exact cell-count check rejected it. Because
  a stream README parse error aborts the whole load, that one row turned every
  other check in the run into could-not-check. The splitter now treats an escaped
  pipe as cell content and keeps the escape sequence verbatim, so a
  parse-then-re-render round trip is byte-identical. A genuinely column-shifted
  row (a PR reference decorating the Status cell, a stray `||`) is still
  rejected — the count check is unchanged.
