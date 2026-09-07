### Added
- New harness-neutral reference `plugins/assay/references/desk-shell.md` — the shell and transport
  mechanics every desk role re-derives (one call/one chain, workspace isolation and
  content-triggered write-guard refusals, per-commit inline commit identity, loop/session marker
  export, authenticated push/fetch transport, and role/repo coverage), stated as mechanism + signal
  + correct form with no house-specific values. Each of the six desk-role skill bodies (`the-desk`,
  `worker-desk`, `pr-review-desk`, `verify-desk`, `intake-desk`, `pr-shepherd`) now points at it.
