### Fixed
- The cellctl `harness`, `provider` and `scrubbed-cell` shell suites pass again. Their Codex stubs answer `codex features list`, the `set` refusal rows accept the Go binary's three-harness wording, and the suites no longer read provider or telemetry variables from the launching shell.
- `go test` now runs `harness.test.sh` and `provider.test.sh` against the shell oracle and `harness.test.sh` against the Go binary. It also runs every `scrubbed-cell.test.sh` case against the Go binary; the cases that need a real tmux skip by name where tmux is not installed.
