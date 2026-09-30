### Fixed
- The `commsloop` tests no longer leave an untracked `mailbox/` directory in the source tree: the refusal test now roots its loop under a temp dir, and the package test run fails if any test writes a new path into the package source directory.
