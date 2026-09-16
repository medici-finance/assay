### Added
- `desklabel add|rm <owner/repo> <number> <label>` — a role-keyed one-label verb. Every
  label-carrying write in the desk was bundled into a bigger verb's fixed set (deskflip's queue
  swap, deskclose's `superseded?` proposal, deskdisposition's `disposition:*` record), so a stale
  marker — the `superseded?` a dispute leaves behind — could only be cleared by a raw, unscoped
  forge call. `desklabel` sets or clears ONE label through the resolved forge under the session's
  own App role, against a closed vocabulary: `question` / `help wanted` / `needs-decision` for any
  role; `superseded?` and the `disposition:*` family for the worker; `authorization-needed` /
  `approval-needed` for the reviewer; `human-decided` refused for every role; anything else
  refused (exit 5) naming the label, its owner and the session's role. The check runs BEFORE any
  forge call; the role is read from the session (`DESK_LOOP`), never from a flag. The target
  kind comes from the seam's own read (`GetIssue`, or `GetIssueTyped` under `--kind issue|mr`
  for a GitLab project carrying both `#N` and `!N`); a present/absent label is a no-op with no
  write; `--dry-run` stops before the write; `vocabulary` prints the table. Both forges are
  covered end to end through the real backends, and the role-ownership guard carries a
  mutation map (`cmd/desklabel/mutations.json`).
