### Changed
- CI now runs the `harnessgen`, `harnesslint` and `plugindrift` test suites in the `build-test` job, and a new gating `harnesslint` job checks the shipped skill bodies and binding references for harness neutrality against the real plugin tree (harness-portability/15).
