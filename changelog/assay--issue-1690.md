### Fixed
- `skillslint --sync` (`make guardrail-sync`) computes a guardrail copy's removal
  boundary from the length of the block actually being replaced, never the new
  canonical text's length — closes a silent-data-loss class where a grown block
  swallowed trailing content that was never part of it (the
  medici-finance/assay#1687 incident) or a shrunk block left stale lines behind
  (#1690).
