### Added
- **forge-neutral/17 — `deskrun log`/`deskrun retry` brief**
  (`docs/streams/forge-neutral/brief-17-deskrun-log-retry.md`): specifies read-only run-log
  access (GitHub `actions: read` / GitLab `read_api`) as safe to grant broadly to both worker
  and reviewer Apps, and `deskrun retry` (GitHub `actions: write` / GitLab `api`) as
  roster-bound the same way `RunWorkflow` is — the same over-broad-scope shape, refusing
  (exit 5) rather than borrowing an ambient human credential when the roster binds the retry
  role to a human.
