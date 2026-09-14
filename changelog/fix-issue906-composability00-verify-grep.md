### Fixed
- `composability/00`'s Verify row 1 documented a `grep -c '^\| \`assay\.'` check that was
  false-permissive on both GNU and BSD grep: the escaped `\|` parses as a GNU alternation
  extension, so the pattern matched every line via its `^` branch and silently counted the
  whole file instead of `components/KEYS.md` table rows. Documented the corrected,
  unescaped-pipe form (confirmed to return the real row count, 30, instead of the file's
  total line count, 97) in a dated Evidence addendum on the already-`done` brief, per the
  house mid-flight-edit convention (#906).
