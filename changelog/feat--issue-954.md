### Fixed
- `statusgen --consumers --brief <id>` now resolves a `schema: brief-v2` brief
  under either the short `<stream>/<NN>` form or its own fully-qualified
  `<cell>:<repo>:<stream>:<NN>` form, instead of only an exact string match
  against the file's `brief:` field. Reuses `normalizeBriefKey` (the helper
  `verifyMarker`/`loadExistingMarkers`/`closeVerify` already use post flag-day,
  #840) so a verify-gate row keyed by the short form can corroborate a
  brief-v2 brief's `consumers:` claims instead of failing with
  "no brief-v1 file for ...".
