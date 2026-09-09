### Added
- **`DESK_TRACE=1` — a diagnostic switch for the desk tools.** Turn it on (or pass a global
  `--trace`) and a failing verb prints the full cause chain, every child process it started with
  the command line as executed, that child's exit status and elapsed time, and the failing
  child's stderr in full. Credentials are redacted at one choke point — GitHub token prefixes,
  URL userinfo such as `https://x-access-token:…@`, `Authorization:` headers and secret-shaped
  environment assignments — and marked `<redacted>` rather than silently elided. With the switch
  off, output is byte-identical to before. Retrofitted onto `deskdispatch`, `deskwt`, `desktoken`
  and `deskfile`; other verbs are unaffected.

### Fixed
- **A failed child's own message now reaches the operator on the first read.** Every desk tool
  opens stderr with its `assay-config:` echo, so a step report built from the first stderr line
  printed the echo and never the diagnosis — `deskdispatch` reported claim and roster failures as
  `(assay-config: …)`, and its worktree-path checks returned a bare `exit status 128` with
  nothing in it to search for. Failures now end on `— <tool> said: <the tool's own first line>`,
  with the rest carried on the error for the trace to print.
- **`desktoken` says that stdout is a PATH.** A caller that used the output as a credential got
  `401 Bad credentials` from its next forge call, three processes downstream and naming nothing.
  A one-line NOTICE now accompanies the path on stderr, with the incantation that reads the
  value. Stdout is unchanged, so callers that pipe it are untouched.
- **`deskwt` and `deskfile` failures carry the command line and the child's exit status**, so a
  trace has something to show for them; `deskfile` can still tell a `401` from a `403` from a
  `429` on its fail-closed dedupe-search path.

### Changed
- `deskkit` grows one shared subprocess runner (`deskkit.Run`) that replaces three divergent
  per-command runners, and a shared error-report path (`deskkit.ReportError`) that every
  retrofitted verb's `main()` uses. `DeskError` carries subprocess detail out of band and gains a
  `Cause()` accessor and a `RefusedWithCause` constructor; `Refused`'s signature is unchanged, and
  a refusal that gains a cause is still a refusal with the same exit code.
