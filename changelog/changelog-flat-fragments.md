### Fixed
- `changelog-check` now refuses a fragment added below the top level of `changelog/` (the release aggregator reads only `changelog/*.md`, so nested fragments were silently left out of release notes), and 20 stranded nested fragments were moved to the top level so the next release picks them up.
