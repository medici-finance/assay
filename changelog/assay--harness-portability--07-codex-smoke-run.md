### Added
- The first live Codex smoke run for harness-portability/07 is recorded at
  `docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md` — codex-cli 0.154.0 on OpenAI
  `gpt-5.6-terra`, run on the driver's workstation with a transcript excerpt per step.
  Both documented install arms resolved (the marketplace arm end to end, which the
  runbook still marks documented-not-demonstrated); the resident rules arrived through
  `AGENTS.md` with no tool call; all twelve packaged skills loaded their full body on
  by-name invocation; auto-trigger drew `worker-desk` unnamed; the isolation floor
  refused under `workspace-write` and created its worktree under `danger-full-access`;
  and `verify-desk` recorded command, exit code and output for a real Verify row. The
  dispatch step is **BLOCKED**: on 0.154.0 `features.multi_agent = false` no longer
  removes the subagent tools, so the degradation that step exists to observe cannot be
  provoked by the documented mechanism. No bundle file was edited to make any step
  pass; the drift is routed to issues, per the protocol.
