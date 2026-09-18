### Added
- `desk-supervision` stream: three new briefs (13-15, renumbered from 10-12 to clear a collision
  with the workflow-App-landing lane that landed on main as 10-12) scoping the **worker-operations
  vitals** delta — a self-reported `resource` block (context-%, tokens, session age,
  subagents, model) filling the reserved `desksupervise-status-v1` `tokens` stub, a
  budget-driven graceful recycle of a healthy-but-full worker, and a local supervisor host
  (`cellctl` + a fleet-vitals aggregation contract) so supervision reaches non-k8s operator
  desks.
