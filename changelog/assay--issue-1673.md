### Fixed
- 12 `gate: human` briefs' `## Human decision` sections carried a default line the decision-gate parser (`tools/decision-issue.sh`'s `has_parseable_default`) could not parse — missing entirely, or close-but-not-matching the literal grammar. Every offender now carries a parseable `Default if no answer: <text> after <YYYY-MM-DD>` or the literal `Default if no answer: none — blocks until answered` (#1673).

### Added
- `tools/lint-human-decision-defaults.sh` — an offline, repo-wide enumerator for the same grammar, so the defect class can be swept in one pass instead of discovered one refusal at a time at dispatch.
