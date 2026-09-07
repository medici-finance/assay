### Fixed
- `deskdispatch`'s model-stamp step now REPLACES a present-but-unreadable dispatch
  attestation with a clean one, instead of only clearing foreign-applied labels. A
  conflicting, stale, or malformed `dispatched-*` label left by an EARLIER run of the
  dispatcher itself (a second model slug, a stale tier, an out-of-vocabulary half) was not
  removed on re-dispatch, so the stamp stayed unreadable and every authority-bearing write
  (`deskpost review` / `security-review`, ready-flip) kept refusing — a deadlock a re-dispatch
  reported OK yet could not break. The new `deskkit.ReStampRemovals` clears every present
  `dispatched-*` label that is not part of the pair being applied, so a dispatcher-identity
  re-dispatch is a supported recovery for a corrupt stamp.

### Changed
- `deskkit.ReStampRemovals(timeline, want, isDispatcher)` computes the re-stamp removal set as
  a superset of `ForeignStampLabels`. The model-capability floor's reader is UNCHANGED: a
  self-applied stamp and a genuinely below-floor tier still refuse — only the dispatcher can
  replace a corrupt stamp with a good one.
