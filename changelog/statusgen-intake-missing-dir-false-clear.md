### Fixed
- `statusgen` no longer renders "the front door is clear" over a missing intake
  register. When the per-entry `docs/streams/intake/` directory is absent, the
  intake-debt alarm now falls back to the monolithic `docs/streams/INTAKE.md`
  view — the same legacy fallback the findings register already has — so a repo
  whose intake still lives in the single-file register is read rather than
  silently rounded to zero untriaged. Previously a missing directory produced an
  empty entry set, which the board rendered as a confident "clear" over a
  register it never actually read.
- When NEITHER an intake directory nor an `INTAKE.md` view exists, the intake set
  is genuinely undetermined and now renders as **could-not-check** rather than
  "clear", closing a three-state-instrument-rule gap where a missing register
  became a false negative with no could-not-check state.
