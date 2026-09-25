### Added
- New `deskmonitor` verb (`deskmonitor inbound`, `deskmonitor pr`): Go ports of the intake and review
  desks' stateful pollers (`inbound-monitor.sh`, `pr-monitor.sh`) with no `bash`/`gh`/`jq`
  dependency, so a desk role's inbound surface runs on native Windows. Same arguments, knobs, state
  files, exit codes and output grammar; a parity test runs each script and the verb over recorded
  forge fixtures and diffs them. The pr poller reports PRs opened against an empty baseline as
  `opened` (the script reports them as `closed`).
- New `desktick` verb: the tick summary-line grammar (`regexp` / `validate` / `check`), the Go port
  of `tick-summary.sh`, with its published regexp pinned equal to the script's.

### Changed
- `scanloop run` arms the `deskmonitor inbound` verb from PATH instead of running
  `inbound-monitor.sh` through a hard-coded `/bin/bash`. `--monitor <path.sh>` /
  `ASSAY_INBOUND_MONITOR` still arm the script, as an explicit parity mode through a `bash` found on
  PATH. The default monitor state dir now follows the OS temp dir (`%TMP%` on Windows).
- The portability audit gains the rows its first pass missed: `pr-monitor.sh`, `tick-summary.sh`,
  the harness Monitor invocation path, and the `jq` prerequisite.
