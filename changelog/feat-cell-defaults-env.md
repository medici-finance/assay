### Added
- `cellctl`: a machine-wide cell defaults file. `$CELLS_ROOT/defaults.env`, in `cell.env`'s
  grammar, sets a key once for every cell under that cells root — `CELL_GO_CACHE=on` is the
  motivating case. A value comes from the highest layer that sets it: cellctl's compiled default,
  then the process environment, then `defaults.env`, then the cell's own `cell.env`, so one cell
  opts out with a line in its own file. `cellctl set` resolves through the same layers and still
  writes the cell's `cell.env` only; `cellctl show` labels a value the file supplied. It applies
  to every cell kind and adds no variable to a scrubbed or container launch. See
  `docs/cellctl.md`.
- `cellctl`: the defaults file lists every key cellctl reads. `cellctl new` writes
  `defaults.env` into a cells root that has none, from a template in which every machine-wide
  key is a commented-out `# KEY=<compiled default>` line under its description, followed by the
  keys the file refuses and why. The new verb `cellctl defaults init` writes the same file for a
  cells root that already has cells, and `cellctl defaults print` writes the template to
  standard output to read or to diff an existing file against. An existing file is never
  overwritten, appended to or replaced. The template is generated from one registry of keys,
  and a test fails when cellctl reads a key the registry does not classify.
- `cellctl`: what a defaults file changes. A file that sets no key — none at all, or the
  template as written — changes no resolved value and no launch. `cellctl check` gains two rows
  (the file read and the keys it sets; the managed Go cache's state and the layer that set
  `CELL_GO_CACHE`): a cells root with a defaults file prints them for every cell, and one
  without prints them only for a cell that sets `CELL_GO_CACHE`.
- `cellctl`: the defaults file is read strictly. A file that cannot be read, a line that is not
  an assignment, or a key that is not a machine-wide lever refuses, naming the key, the file and
  the line. Refused are the keys that name or scope one cell (`CELL`, `CELL_KIND`, `CELL_REPO`,
  `CELL_REPO_SLUG`, `CELL_ROOTS`, `ROLES`, a cell's `deskd` address, session, cache root,
  container binding and forge binding); the switches for one run (`CELLS_ROOT`, `DRY_RUN`,
  `CELL_ATTENDED`, `DESK_MODEL_OVERRIDE`); the locations and host variables cellctl follows from
  the launching shell (`HOME`, `USERPROFILE`, `XDG_DATA_HOME`, the config homes, `PATH`, `TERM`,
  `LANG`); and cellctl's own internal variables. A key cellctl does not read is carried as a
  `cell.env` line would be.

### Fixed
- `cellctl`: a cell with a nested name (`team/demo`) is re-entered under the cells root its
  launch used. The model-policy hook, a container cell's console, a managed scratch task and
  the cadence and comms panes took the cell directory's parent as the cells root, which for a
  nested cell is a different directory, so the later process looked for the cell, the shared
  provider defaults and the machine-wide defaults file in the wrong place. Each is now handed
  the launch's cells root, and the hook refuses a cell directory that is not under the root it
  was given.
- `cellctl set`: a `$VAR` reference in a `cell.env` or `defaults.env` value is expanded as a
  launch expands it, against the process environment and the lines above it, so `set` no longer
  judges a cell on a value the cell does not boot with.
