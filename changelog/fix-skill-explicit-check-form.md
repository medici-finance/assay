### Changed
- `author-brief` now asks the critical-path head check to be run against what the session can read (always the requester's own words and the material handed over) and to quote the line it rests on, so an offline plan can no longer answer the check with a bare "unproven".
- `verify-desk` now asks a FAIL bug to carry each failed row as `` `<command>` → exit <N> → <verbatim output> ``, and the Evidence to carry only the verdict marker it reached: a negated or code-quoted `**VERIFY: PASS**` under a FAIL reads as a pass to the strict gate and to the board reader.
