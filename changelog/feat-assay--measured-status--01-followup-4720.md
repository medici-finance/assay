### Fixed
- The measured-status/01 Verify table now holds only offline, pipefail-safe rows that fail on the pre-delivery tree and pass on the delivered one; the old pinning-test row exited 141 under `bash -o pipefail`, and the old rows 1, 2 and 5 passed even where the pinning test did not exist.
