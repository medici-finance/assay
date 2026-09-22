### Fixed

- `deskevidence` refuses to land an `"outcome":"verified"` row on
  `docs/streams/verify-outcomes.jsonl` unless the landing tree presents a
  lint-valid `verified` closure for that brief — the Verified stamp AND a passing
  execution witness for every Verify row. Previously the sidecar recorded
  `verified` on a PASS run unconditionally, so a brief whose Evidence was filled
  while its board stayed `implemented` (no flip, or no witness) still got a
  `verified` row; `verifyloop` then bucketed the mismatch as a stuck-flip (#1309)
  and review refused to merge it. The acceptance decision lives in a new
  read-only `statusgen verifyclosure --brief <stream>/<NN> [--root <dir>]`
  sub-command that reuses the board's own Status/Verified read and the existing
  witness audit (`checkWitnesses`), so the criteria are defined in one place. The
  gate is scoped to `verified`: a `verify-fail` row (and every other outcome) is
  never gated, and a brief the check could not evaluate refuses the landing as
  could-not-check rather than passing silently.
