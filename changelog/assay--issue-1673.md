### Fixed
- 12 `gate: human` briefs' `## Human decision` sections carried a default line the decision-gate parser (`tools/decision-issue.sh`'s `has_parseable_default`) could not parse — missing entirely, or close-but-not-matching the literal grammar. Every offender now carries the literal `Default if no answer: none — blocks until answered` (#1673); none needed a dated default, since none had both a default and its date already stated in the author's own text.
- `windows-port/brief-14`'s `decision-trigger: start` was corrected to `decision-trigger: spec` — its Task section already has the executor author `## Human decision` at pickup, which is `spec`'s own definition, and `start` made `deskdispatch --gate-human` refuse the brief on its still-unauthored placeholder section.
- `tools/lint-human-decision-defaults.sh`'s placeholder exemption applied to every `decision-trigger`, not just `spec` as `tools/decision-issue.sh` itself gates it — a `start`-trigger comment-only section reported clean while the real gate refuses it. The lint now checks `decision-trigger` before exempting.

### Added
- `tools/lint-human-decision-defaults.sh` — an offline, repo-wide enumerator for the same grammar, so the defect class can be swept in one pass instead of discovered one refusal at a time at dispatch.
- `tools/lint-human-decision-defaults_test.sh` — an offline fixture test for the lint's grammar and trigger-gating, including a regression case for the placeholder-exemption fix above.
