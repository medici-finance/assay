### Added
- `cellctl`: per-role starting context (#2438). A cell can set `CELL_ROLE_CONTEXT` to a JSON
  declaration that, per role, switches off plugins, skills, built-in tools, agent types,
  connectors and instruction files, points the role at its own memory directory, appends a role
  instruction file, and installs agent definitions for what the role dispatches. `cellctl desk`
  and `cellctl up` apply it at launch, `cellctl check` reports what each role will start with and
  refuses a declaration naming something missing. A declaration can only take things away; a cell
  that declares nothing launches exactly as before. Ships a `builtin:lean-reviewer` agent
  definition (shell and file read/write only), which roughly halved a dispatched agent's starting
  context in measurement. See `docs/cellctl-role-context.md`.
