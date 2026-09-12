### Changed
- `docs/adopting-assay-gitlab.md` §2 documents the `ASSAY_SCAN_REPOS` post-fleet-boot step
  (required, distinct from the `ASSAY_ALLOWED_REPOS` write boundary, verified via the
  `issueboard` stderr echo) so a green write-lane boot no longer looks complete while the
  GitLab issue lane is still could-not-check.
- `docs/adopting-assay.md` channel-D section documents how to prove the running binary on a
  from-source Windows lane (no `.assay-versions` pin line exists there) and warns against
  reading a stale-pin `deskboard` as an all-clear board.
