### Added
- `cellctl`: per-role starting context (#2438). A cell can set `CELL_ROLE_CONTEXT` to a JSON
  declaration that, per role, switches off plugins, skills, built-in tools, agent types,
  connectors and instruction files, points the role at its own memory directory, appends a role
  instruction file, and installs agent definitions for what the role dispatches. `cellctl desk`
  and `cellctl up` apply it at launch and print what they applied; `cellctl check` reports what
  each role will start with and refuses a declaration naming something missing. No key grants a
  plugin, permission, hook, connector or model. Two keys do more than remove context and are
  bounded: `plugins_off` drops a plugin whole, hooks included (the role's own plugin is refused,
  and `check`, a dry run and the launch name every plugin switched off), and `memory_dir` is a
  write location that must resolve below `<cell-dir>/memory/`. A repeated key, and a declaration
  or named file inside a role worktree or a memory directory, are refused. A cell that declares
  nothing launches and checks exactly as before. Ships a `builtin:lean-reviewer` agent definition
  (shell and file read/write only), which roughly halved a dispatched agent's starting context
  in measurement. See `docs/cellctl-role-context.md`.
