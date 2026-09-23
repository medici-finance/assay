### Added
- harness-portability/17 (spec): a skillslint conformance rule that fails CI when a shipped
  skill's `description` exceeds 1024 characters or its `name` breaks the agentskills 64-char /
  lowercase-hyphen / matches-directory rules (the limits Codex truncates or refuses on), with
  body size and the bundle-wide description budget reported as advisory NOTICEs, a
  `--skills-dir` flag so adopters can run it over their own skills, and the two over-limit
  descriptions (`install`, `pr-review-desk`) shortened.
