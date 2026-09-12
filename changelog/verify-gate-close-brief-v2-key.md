### Fixed
- `verify-gate-close.yml` now accepts a `<!-- verify-gate: ... -->` marker written in either
  brief-id grammar: the legacy `<stream>/<NN>[a]` form, or the `<org>:<alias>:<stream>:<NN>[a]`
  brief-v2 key form that `statusgen --verify-issues` now emits. Previously only the legacy form
  passed the workflow's grammar check, so a verify-gate issue carrying a brief-v2-keyed marker was
  rejected outright and its brief was never advanced to `done` on close. The extracted marker is
  normalised to the legacy form before the existing grammar check runs, so every later use in the
  step still sees exactly one shape (#804).

### Changed
- Added migration `migrations/0002-v1.0.6-to-v1.0.7-verify-gate-close-brief-v2-key.md` so an
  adopter running `upgrade-assay` across this span is told, as their release note, that their own
  copy of `verify-gate-close.yml` needs the same patch — the migration runner has no file-patch
  operation, so it records the owed step in `docs/UPGRADING.txt` and prints the patch instructions
  rather than applying them.
