### Fixed
- `docs/codex-smoke-protocol.md`'s preamble and Steps 1/3 named a stale nine-skill roster
  (including two skills — `dailies`, `market-intelligence` — no longer in the bundle) while
  the packaged bundle ships thirteen. The preamble, Step 1's `Expect:` line, Step 3's
  Action/Expect, and the run-log skeleton now name the correct count and the full current
  roster (`adopt`, `ask-decision`, `author-brief`, `human-runsheet`, `install`,
  `intake-desk`, `pdfingest`, `pr-review-desk`, `pr-shepherd`, `the-desk`, `upgrade-assay`,
  `verify-desk`, `worker-desk`), matching `plugins/assay/codex/packaging.md`'s
  `assay:codex-packaging` roster and `plugins/assay/skills/`. (#938)
