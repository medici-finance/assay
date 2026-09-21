### Added
- `cellctl desk`/`up` now disables Claude Code's next-prompt suggestions
  (`CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false`) by default on every claude-arm
  launch — a house/k8s desk, a scrubbed desk/smoke session, and a GLM/Kimi
  provider window alike — overriding an inherited `=true` from the launching
  shell rather than merely leaving it alone. Shown in `DRY_RUN=1`/`[plan]`
  output; Codex is untouched (no equivalent knob). `images/assay-harness/Dockerfile`
  and `containers/base/Dockerfile` set the same default for a container-based
  direct or scheduled launch that never goes through `cellctl` at all. (#1436)
