### Added
- Desk inbox: `deskfile new --to <role>` addresses an issue to a desk (stamps a
  `to:<role>` label, reusing the `--raised-by` role vocabulary). The addressee's own sweep
  leads with it — `fanoutloop plan` emits `to:worker` items first, and `issueboard issues`
  renders addressed items `ADDRESSED→<role>` in their own priority band.
- `issueboard issues --to <role>` — a per-desk inbox view showing only that role's
  addressed items. Un-flagged, a `to:<role>` item is held out of the un-briefed
  (CREATE-PLACEHOLDER) work and, once aged past `--sla-days` with no comment from that
  role's App, ESCALATES — so an unread inbox surfaces without the addressee's cooperation.
- `deskack "<restatement>"` — a new verb that prints a desk's receipt line
  (`ack <role>@<repo-short>: <restatement>`) for a human-typed message and appends a
  `{ts, role, repo, restatement}` record to the session's roster beacon. Refuses a
  restatement over twelve words.

### Changed
- The five desk-role skills (`the-desk`, `worker-desk`, `pr-review-desk`, `verify-desk`,
  `intake-desk`) now run `deskack` as the first, one-permitted line after any human-typed
  message, and route cross-desk hand-offs through `deskfile new --to <role>` rather than a
  typed relay through the human.
