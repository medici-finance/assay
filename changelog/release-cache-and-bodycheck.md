### Changed

- Release cache restore skips log and artifact reads for runs that could not have saved a cache, and reports rejection counts by stage.

### Added

- A promotion patch runs the release bodycheck mutation sweep in a separate test job and keeps its report in the release evidence pack.
