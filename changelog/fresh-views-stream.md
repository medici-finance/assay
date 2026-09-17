### Added
- New stream `fresh-views` (parked, pending approval): a scoping doc plus 6 `todo` briefs
  proposing that every derived view — a dispatch plan, a board snapshot, the open-PR set, a
  mergeability verdict, a mirrored version string, a reconciled lifecycle cell, a landed sha —
  be treated as a pure function of `main` at a known sha, stamped with its input sha, and made
  to refuse (not guess) when that input is stale. Authoring only; implements nothing. (#334)
