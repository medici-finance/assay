- Desk supervision now directs agents to wait on existing completion handles instead of
  repeatedly polling unchanged work, and to start each new item with a fresh child context.
  Compact checkpoints and file-backed logs reduce repeated context loading while preserving
  review scope, independent lanes, prior findings and required freshness checks. Ordinary and
  scrubbed Codex desk launches include this guidance; already-running sessions require an
  explicit instruction update or a later launch with the updated tools and plugin.
- Supervisors now react to worker events and due checks, re-arm early timeouts without
  rereading unchanged state, and keep routine CI watches outside the model. Codex no
  longer receives a desk-imposed 60-second wait cap. Where harness limits prevent
  efficient idle waits, the documented fallback is the existing operator-selected
  outer cadence with a clean handoff; Goals are not treated as event monitors.
