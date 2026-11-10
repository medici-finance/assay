### Fixed

- `ask-decision` §Act: in a live walk, where the desk has just read the state and the driver runs each command with `!` in a fresh shell, each step is handed over as one live line with the driver's token pinned inline, not as a dry-run function block. Filed and cold-read acts keep the dry-run block.
