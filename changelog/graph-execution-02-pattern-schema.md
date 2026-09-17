### Added
- `spec/workflow-pattern-v1.md`: the workflow-pattern schema — a node contract
  (kind, role, inputs, outputs, evidence, effects, budget, outcomes), risk
  class as a declared `risk-input`, and the integration-check `join` node —
  plus `schemas/workflow-pattern-v1.json`, so a workflow's shape can be
  reviewed as one versioned artifact and validated by an independent tool.
- `spec/workflow-patterns/implementation-v1.yaml` and `research-v1.yaml`: the
  two workflow patterns the fleet already runs, stated as reviewed pattern
  files rather than left implicit across desk-skill procedure text.
- `statusgen patterns --lint [--root DIR]`: validates every
  `spec/workflow-patterns/*.yaml` file against the schema and five MUST rules
  — `pattern-effect-target-not-owned` (a non-`effect`-kind node's effect
  target must be among its own outputs), `pattern-effect-exceeds-role` (a node
  cannot declare an effect kind its role does not hold),
  `pattern-review-same-role` (a review node cannot share the role of whoever
  it is reviewing), `pattern-join-not-check` (the integration check must be a
  `check`-kind node), and `pattern-risk-input-missing-verdict` (all four
  risk-class verdicts must be mapped). All five are registered in
  `statusgen enforcement-status`.
- `topology.yaml`'s `apps:` role names are now compiled into a derivation
  (`topologyAppRoles`) bound to the source by `TestTopologyValuesMatchSource`,
  the same derive-or-diff convention as the rest of `topologyvalues.go`.

### Changed
- `docs/lifecycle.md` §Review gates names which `implementation-v1` pattern
  node each existing review gate is.
