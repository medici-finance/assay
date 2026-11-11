### Fixed
- measured-status/05: Verify row 2 now names its two tests and counts their own PASS lines with `grep -c`, so it no longer exits 141 under `bash -o pipefail` the way the earlier `grep -q 'PASS'` pipe did.
