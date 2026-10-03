### Fixed
- `deskreply`, `desktoken`, `deskpost` and `deskwt remove`/`prune` now treat a multi-token `--help` (for example `deskreply <repo> <n> --help`) as a successful help request: the usage screen prints, the exit code is 0, and no `refused` row is written to the audit ledger. A new structural test fails if any other desk verb's subcommand flag parse lacks the same recognition.
