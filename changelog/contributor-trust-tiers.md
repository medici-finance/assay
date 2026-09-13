### Added
- Contributor trust tiers (`unknown` < `blessed-once` < `contributor` < `maintainer`)
  and an operator-side ledger reader (`ResolveTier`), so a repeat contributor can
  carry a recorded standing instead of being assessed as a stranger on every
  submission. The ledger never lands as a file in this repository; see
  `docs/contributor-trust.md`. Inert on landing — no caller is wired to it yet.
