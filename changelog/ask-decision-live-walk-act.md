### Fixed

- `ask-decision` §Act: in a live walk, where the desk has just read the state and the driver runs each command with `!`, a `gh` step is handed over as one live line with the driver's identity pinned inline, not as a dry-run function block. A non-`gh` step, an act that needs a `# fill:` secret, and every act posted for later or cold reading keep the dry-run block.
