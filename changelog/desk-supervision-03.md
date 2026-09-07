### Added
- `desksupervise tick` now reconciles every in-flight dispatch claim's ELIGIBILITY before the
  liveness step: a run whose item became ineligible mid-run is STOPPED within one observer
  interval. Terminal cases — the item is FINISHED and no other party owns its claim (PR merged or
  closed, board row at `implemented`/`verified`/`done`, or the claim already released) — also
  release the claim for re-dispatch. Held cases — a human or another holder owns the next move
  (the claim reassigned to a different live holder, a `blocked` board row, a
  SUPERSEDED/RESOLVED-ELSEWHERE disposition, or a `needs-decision`/`question` label) — stop the
  run WITHOUT releasing it, so an unconditional ref delete can never re-free an item its new
  holder is working. A reconcile read that could-not-check keeps the run and retries next tick.
  This turns "a merged or closed PR is DONE, stop" from a rule a worker had to remember into a
  mechanical backstop.
