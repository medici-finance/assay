### Added
- **`cellctl` gains a `house` cell kind (#845).** `cellctl new <cell> --kind house --repo <checkout>
  --roots '<owner>/<repo>=<abs path>,...'` scaffolds a LOCAL cell for the operator's own desks: the
  cell home reaches the operator's existing config home (roster, App keys) by one symlink and copies
  nothing, no `deskd` is required unless `cell.env` sets `DESKD=1`, and `cellctl desk <cell> <role>`
  boots each role in its own LOCKED worktree off a fresh `origin/main` with `DESK_ROOTS` (from
  `CELL_ROOTS`), `DESK_LOOP` and `DESK_SESSION=<cell>-<role>-<UTC stamp>` exported — the one-command
  replacement for the hand boot that could start a desk inside a shared checkout. `cellctl check`
  on a house cell proves the checkout, a PARSING roster, every root carrying `docs/streams/`, the
  desk verbs and the enabled plugin. `CELL_KIND` defaults to `k8s`, today's behaviour.
### Changed
- **Every `cellctl desk` window now exports `DESK_ROOTS` when `cell.env` carries `CELL_ROOTS`**, on
  k8s cells too, and says on stderr when it boots without one — a cell re-boot can no longer leave
  the desk verbs silently on their compiled placeholder topology. Role worktrees are locked at boot,
  the shared-fetch lock lives in the common git dir (so a `CELL_REPO` that is itself a linked
  worktree no longer waits 60s), and a `deskwt role-init` that supports the role is preferred for
  creating the tree so cellctl and the desk skills agree on its name.
- `tools/cellctl/tests/house-cell.test.sh` — a plain-bash, no-network test of the house kind against
  a fixture repo (scaffold, check, boot with a stubbed `claude`, lock, exported env, untouched
  `.git/config`, legacy `cell.env` still loads as k8s).
