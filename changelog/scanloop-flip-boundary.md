### Fixed
- `scanloop` no longer pushes a new placeholder batch onto a scan PR that has already been flipped
  ready-for-human. A flipped PR is push-quiet — a post-flip push re-signals the whole review loop for
  churn the reviewer has already sealed off — so the coalesce decision now honours a `--scan-pr-state`
  (`draft` / `ready`) reading: a flipped PR opens the NEXT scan PR regardless of the coalesce window,
  and a draft/ready state that cannot be read never coalesces (the same bounded direction the
  unreadable-age arm already takes).
