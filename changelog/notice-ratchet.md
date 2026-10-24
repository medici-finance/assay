### Added
- `statusgen --lint --diff-base` now also compares the NOTICEs of the briefs the diff adds, edits
  or renames. An actionable finding (structural Verify-row rules, row strength, identifier
  dereference, risk and `why:` metadata, `consumers:` routing) that the merge-base copy of the
  brief did not carry is reported as `PROBLEM: [notice-ratchet] ...` and fails the run; findings
  already at the base, `COULD-NOT-CHECK` lines, heuristic prompts, historical summaries and every
  brief the diff does not touch stay NOTICEs. Findings are compared by rule, repository-relative
  path and row identity, so rewording a message or moving the checkout root creates none. When the
  base or the changed-brief list cannot be read, the run prints an explicit could-not-check
  instead of a clean comparison. See `docs/verify-row-strength.md`.
