### Added
- Opt-in host-wide mutation worker leases through `MUHAR_LEASE_DIR` and `MUHAR_TOTAL_SLOTS`, with queue timing, worker clamping and explicit propagation across cellctl launch environments.

### Fixed
- Track discovery and coverage child processes during interruption; release mutation capacity and unwind source/workspace cleanup on SIGINT and SIGTERM.

### Changed
- Direct suite-command signal termination is nondefinitive (`COULD_NOT_MUTATE`) even without budgeting; a control terminated this way breaks the harness. Wrappers translating a descendant signal into an ordinary non-zero exit still use the ordinary verdict path.
