### Changed
- `statusgen` attribution: in a multi-identity repository, a brief whose authoring
  commit and every commit that touched its Evidence section share one git identity
  now fails the committer-identity cross-check as a hard PROBLEM (blocking
  verified/done) instead of a NOTICE, closing the self-verification independence
  hole; the single-identity (inconclusive) case remains a NOTICE. Landed in the
  code via an earlier change; this records the measured-status board row moving to
  implemented.
