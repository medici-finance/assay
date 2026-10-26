### Fixed
- The verifyloop association fixture helper is now pinned against a caller-exported `GIT_DIR`, `GIT_WORK_TREE` or `GIT_INDEX_FILE`: a hostile-environment regression proves fixture init, config, remote, staging, commit and ref updates leave the caller repository byte-unchanged, and a mutation entry shows it fails when the isolation is removed.
