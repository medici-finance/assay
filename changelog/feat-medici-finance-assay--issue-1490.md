### Fixed
- A dispatched agent's worktree no longer INHERITS the shared checkout's git commit identity.
  `deskdispatch`'s worktree-create step now stamps the DISPATCHED agent's own role commit
  identity (worker/reviewer/verifier, mapped from `--kit`) into the new worktree's own
  worktree-scoped config, so a verifier dispatched from a desk checkout commits — and reports
  its runner — under the verifier App, not the desk App. Before this, the worktree carried
  whatever `user.name`/`user.email` the shared `.git/config` held, and `statusgen verifyrun`
  stamped that wrong identity into every Evidence witness Runner cell (silent misattribution).
  A `--kit` whose role has no roster commit identity is now REFUSED pre-claim (exit 5) naming
  the kit, the role and the roster key, and the worktree-create OK line prints
  `identity=<slug> <bot-user-id>`.

### Changed
- `deskwt add` now takes `--role R` and, given it, stamps that desk role's App commit identity
  into the new worktree (the same shared resolver `role-init` uses); an unbound role is refused
  (exit 5) before the worktree is created. WITHOUT `--role`, `deskwt add` now CLEARS the new
  worktree's `user.name`/`user.email` (an empty worktree-scoped value that shadows the shared
  config), so a bare `deskwt add` worktree can never silently commit under an inherited identity
  — a commit there fails closed until an identity is set. The commit-identity resolver
  (GitHub/GitLab shape choice + fail-closed refusals) is extracted to one shared helper,
  `deskkit.RoleWorktreeCommitIdentity`, so `role-init`, `deskwt add --role` and `deskdispatch`
  cannot resolve one role to three identities.
