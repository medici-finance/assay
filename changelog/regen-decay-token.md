### Fixed
- The board regen job on the default branch now passes the workflow token to statusgen and grants `pull-requests: read`, so dead-claim decay can read branch pull-request state in CI instead of running without credentials.
