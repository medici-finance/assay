### Fixed
- `deskroster width --role <loop>` (the plain, non-`--verbose` read) no longer prints
  `(source=default, expires=n/a)` while a live width override is in force. The trailer was
  describing only the RESERVE field of the stored entry, so a plain `deskroster set --role
  <loop> --width N` — which stores no reserve — read as "default" on the very next read, while
  `--verbose` correctly reported `source="set by <session> at <time>"`; a coordinator reading
  the plain line took a live width for a lapsed one. The plain line now renders the same
  resolved source and expiry the verbose path computes: `(source=set-by:<session>,
  expires=<RFC3339>)` whenever the stored entry is fresh (width and reserve share that one
  TTL), and `(source=default, expires=n/a)` only when nothing is stored or the entry has
  decayed. `--verbose` output is unchanged.
