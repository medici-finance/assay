### Added
- Opt-in host-wide mutation worker leases through `MUHAR_LEASE_DIR` and `MUHAR_TOTAL_SLOTS`, with queue timing, worker clamping and explicit propagation across cellctl launch environments.

### Fixed
- Track discovery and coverage child processes during interruption; release mutation capacity and unwind source/workspace cleanup on SIGINT and SIGTERM.
