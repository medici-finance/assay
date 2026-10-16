### Fixed
- The release workflow's Go cache restore now keeps the executable bit on restored files, so cached `go run` binaries no longer fail with "permission denied" when a release reuses a prior run's cache.
