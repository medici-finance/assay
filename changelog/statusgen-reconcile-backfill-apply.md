### Added
- `statusgen reconcile --backfill --apply` now WRITES a witnessed `todo`/`in-progress` →
  `implemented` cell back into the brief's stream README `Status` column — closing the
  wiring gap where `--backfill [--report]` only ever reported drift, never applied it.
  The write fires only for a real merged-PR witness (a `Brief:` trailer or the declared
  backfill branch/body match), never touches `Verified`/`Reviewed` or any `human:<name>`
  sign-off stamp, and never writes `verified`/`done`. Idempotent — a re-run with nothing
  witnessed exits 0 having written nothing.
