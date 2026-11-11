### Added

- `deskinstall --harness cursor` (placement, `--check`, `--dry-run`) and `upgrade-assay --harness cursor` now report the host's Cursor CLI commit attribution (`attribution.attributeCommitsToAgent`, `attribution.attributePRsToAgent` in the global `~/.cursor/cli-config.json`; both default to true) as off, on or could-not-check, naming the file and the `false` value to set. The report never edits that file and never changes the exit code.

### Changed

- The Cursor binding (`references/cursor.md`) names both attribution settings, what they do, their global-only scope, and the required `false` on hosts that run desks.
- The no-attribution guardrail in every desk skill, and the worker dispatch kits, now say what to do when a push is refused over an attribution trailer the harness added: stop, report the setting and the refusal, and never rewrite history or push by another route.
