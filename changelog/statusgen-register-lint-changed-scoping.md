### Fixed
- `statusgen --lint --changed <file>` (the PR-side gate) now path-scopes the
  register-integrity check to the diff, the same way the DAR and product-scope
  checks already honour `--changed`. Previously the register lint walked the whole
  register regardless of the diff, so a single pre-existing register defect on
  `main` — a duplicate id, an unparseable date, an invalid id, an unauthorized
  field-gutting, a malformed park — hard-failed the `statusgen` check on *every*
  open PR that touched `docs/streams/**`, even PRs that never touched the
  defective entry, and the stale red never cleared until the unrelated main-side
  defect was fixed. Now a defect on a register file the PR's own diff never
  changed demotes to a `NOTICE:` (surfaced, never silently dropped — it is
  already red on main's own status-regen, which owns it), while a defect the diff
  introduces or touches — its file in the `--changed` set — still fails. With no
  `--changed` set (a full-tree / main run) behavior is unchanged: every register
  defect remains a hard `PROBLEM`.
