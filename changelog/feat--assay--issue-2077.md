### Fixed
- desktools-v2/14 Verify row 8's Expect now leads with `exit 0` and no longer says "exit 1" for the inner `go test` (#2077). The Expect parser reads the first `exit N` outside code spans and does not match "exits 0", so it took the inner mutation run's "exit 1" as the row's required status and scored a passing hand-run as a fail.
