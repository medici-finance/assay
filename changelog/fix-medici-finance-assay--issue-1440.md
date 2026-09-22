### Fixed
- `deskdispatch`'s worktree-create failure message now routes through the same
  scrub pass as every other diagnostic, and `ToolRun.FailVerbatim` scrubs the
  message it is given before it becomes part of the error. A caller that
  composes its own `FailVerbatim` message from a child process's raw stderr can
  no longer let a credential-shaped string on that stderr reach the operator
  unredacted, on any `DESK_TRACE` setting. (#1440)
