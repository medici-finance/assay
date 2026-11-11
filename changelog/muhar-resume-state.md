### Added
- `muhar -state PATH` persists run-bound mutation observations and phase timings, refuses stale or concurrent reuse, and resumes completed definitive verdicts after interruption. Resumable state binds the running executable, and selection/schemata timing records describe the commands actually executed.

### Changed
- Only a direct suite-command death by a termination or resource signal (`SIGKILL`, `SIGTERM`, `SIGINT`, `SIGHUP`, `SIGXCPU`, `SIGXFSZ`) is nondefinitive (`COULD_NOT_MUTATE`). Fault signals such as `SIGABRT` and `SIGSEGV` are ordinary test failures again, so a crashing mutant is `CAUGHT`. This narrows the host-budget rule that treated every direct signal death as nondefinitive.
- Under `-state`, a test command that names `go` through shell escapes (`\go test`, `g\o test`) or quote-split spellings (`"g"o test`, `g''o test`) is refused instead of being treated as a non-Go command.
