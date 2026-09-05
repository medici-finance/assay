### Fixed
- `deskboard`'s freshness banner no longer reports a permanent `STALE-UNKNOWN` when
  run from a consumer checkout. The drift check keyed on the in-tree
  `origin/main:tools/desk` git ref, which stopped resolving once the desk tools moved
  out of consumer trees to their release home — so every consumer run was
  could-not-check forever, unable to tell "the binary is behind the source" from "the
  source is not in this tree". The check now compares the running binary's embedded
  `releaseTag` against the `desk-tools` tag the consumer pins in its own
  `.assay-versions` (the source that resolves where the binary actually runs), keeps
  the in-tree `tools/desk` ref as a fallback for the source repo, and reports
  `could-not-check` only when neither source resolves — naming which one was missing.
