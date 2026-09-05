### Added
- `deskgit push --as <role>` and `deskgit fetch --as <role>` — authenticated git transport from a role's App token file, the sanctioned replacement for hand-retyped credential-helper recipes. Push is fixed to the current branch (never main or a detached HEAD), refuses `--force`/`--delete`/`--prune`/`--mirror`/`--tags`/`--no-verify` by name, and never writes the token to the audit line.
