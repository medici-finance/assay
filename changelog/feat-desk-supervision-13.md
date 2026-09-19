### Added
- `deskroster set` gains optional resource-vitals flags (`--tokens`, `--context-pct`,
  `--session-age-seconds`, `--subagents`, `--model`) so a desk session can self-report its
  own tokens/context/age/subagent/model state onto its roster beacon, each field
  three-state (measured / could-not-check / unset) and never a fabricated zero.
- `desksupervise status --json` fills the previously-reserved `tokens` stub with a full
  `resource` block per claim, joined from the claim holder's own roster beacon (or a new
  `--beacons-fixture` for offline Verify runs); a claim with no readable beacon renders
  every resource field `could-not-check`.
- `deskroster set` refuses a `--session` value that does not resolve to a single path
  segment (no `/`, no `..`), closing a beacon-path-join hardening gap identified in
  security review.
