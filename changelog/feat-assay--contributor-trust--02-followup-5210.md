### Fixed
- The contributor-ledger reader no longer accepts a ledger row that names the `maintainer` tier. That tier comes only from the operator's roster, so such a row now grants nothing, which is what the published trust model already said.
### Changed
- The contributor-trust/02 brief's Verify rows 1-6 and 10 now select their tests by exact name and require each test's own pass line, so they fail on the tree before the trust tiers landed instead of passing on "no tests to run". The amended rows still need a human acceptance-transition approval before a verifier can credit them.
