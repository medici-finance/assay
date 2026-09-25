### Added
- `skillslint` gates the per-skill frontmatter hard limits Codex and the agentskills spec enforce (`description` ≤ 1024 characters, `name` ≤ 64 characters matching `^[a-z0-9]+(-[a-z0-9]+)*$`) as exit-code-bearing findings, and reports the soft body/bundle budgets (8000 bytes, 500 lines, ~5000 tokens, 8000-character bundle description total) as advisory NOTICEs; a new repeatable `--skills-dir <dir>` flag runs just the structural + conformance checks over an adopter's own skills directory (`harness-portability/17`).

### Fixed
- `install` and `pr-review-desk`'s `SKILL.md` descriptions are shortened under the 1024-character hard limit so a Codex CLI no longer silently truncates or refuses either skill (`harness-portability/17`).
