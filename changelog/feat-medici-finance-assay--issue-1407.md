### Fixed
- statusgen's brief-parse memo now keys on a hash of the file's content instead of
  an `(mtime, size)` stamp. The old stamp could not tell two same-size versions of
  a brief apart when a coarse-granularity filesystem recorded both writes under one
  mtime tick, so a length-preserving in-place edit (e.g. flipping a `gates:` target
  from one brief to another of equal-length id) could be served from the stale
  pre-edit parse. This made `TestEligibilityDeclarationChangesDispatch` flake on the
  self-hosted release runner while passing on nanosecond-mtime macOS, and — more
  importantly — could have let any consumer read a stale gate/eligibility verdict for
  a brief edited during a run. The cache is now correct on every filesystem regardless
  of its timestamp resolution.
