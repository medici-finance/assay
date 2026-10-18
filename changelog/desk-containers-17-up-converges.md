### Added
- `cellctl up` converges the cell: a role whose cadence lease is held is reported `already-up` and never relaunched, an absent role is launched, and a stale one is a `failed` row that points at `up --plan`. Launches are serialized on the lease, and `--role <name>` (repeatable) and `--lease-wait` select and bound them.
- `cellctl up --plan` writes one cell-op plan clearing every stale role of the cell; `up --apply <plan-id>` applies it on a terminal after the typed confirm, refusing `plan-stale` by role name, and reports the cleared roles `reconciled+up`.
- `cellctl up --json` prints the `cell-op-result` document with one entry per role. The final table and exit code (0 only when every role ends up) are the same without it.
