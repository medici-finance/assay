### Fixed
- `statusgen --flow`: the test that proves the CI-slot forge read is gated on `--forge` (never on a token in the environment) now pins a target repository, so it catches a removed flag gate in a hermetic tree with no git remote, where it previously passed regardless.
