### Fixed
- Release preflight identifies a read-only arm64 registration interface as a runner security decision instead of suggesting stale-handler cleanup. It also refuses a nonzero installer exit even if the output contains a success line. Both failures stop before the arm64 probe and image builds.
