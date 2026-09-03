### Added
- `qualgen sweep` — a standing, current-tree code-slop forensic sweep lane:
  configured external linters nominate suspects (leg 1), a pluggable
  `AgentVerifier` adjudicates each new suspect with emitter-side evidence
  enforcement (leg 2), and an evidenced, report-only markdown artifact is
  rendered per run (leg 3). Incremental by fingerprint — a rerun over an
  unchanged tree re-verifies nothing — and read-only against the target repo.
  Ships an offline scripted `Fixture` reference verifier; a live coding-agent
  adapter is configuration.
