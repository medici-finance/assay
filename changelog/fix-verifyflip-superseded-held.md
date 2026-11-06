### Fixed
- `statusgen verifyflip`, close-verify and the verify-gate card no longer refuse a
  later strict **VERIFY: PASS** because an earlier, superseded run still says HELD.
  The held scan's cover now reads the Result cell `statusgen verifyrun` writes for a
  passing row that expects a non-zero exit (`pass exit=2`) as a clean re-run, so the
  latest PASS run supersedes the earlier hold as it already did for `pass exit=0`.
