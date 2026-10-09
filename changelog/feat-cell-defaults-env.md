### Added
- `cellctl`: a machine-wide cell defaults file. `$CELLS_ROOT/defaults.env`, in `cell.env`'s
  grammar, sets a key once for every cell under that cells root — `CELL_GO_CACHE=on` is the
  motivating case. A value comes from the highest layer that sets it: cellctl's compiled default,
  then the process environment, then `defaults.env`, then the cell's own `cell.env`, so one cell
  opts out with a line in its own file. The file is optional and a machine without one behaves
  exactly as before. A file that is present is read strictly: one that cannot be read, a line
  that is not an assignment, or a key that names or scopes a single cell (`CELL`, `CELL_KIND`,
  `CELL_REPO`, `CELL_REPO_SLUG`, `CELL_ROOTS`, `ROLES`, a cell's `deskd` address, session, cache
  root, container binding and forge binding) refuses and names the key and the file. `cellctl
  set` resolves through the same layers and still writes the cell's `cell.env` only. `cellctl
  check` says whether a defaults file was read and which layer supplied `CELL_GO_CACHE`;
  `cellctl show` labels a value the file supplied. It applies to every cell kind and adds no
  variable to a scrubbed or container launch. See `docs/cellctl.md`.
