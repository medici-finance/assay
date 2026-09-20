### Added
- The deploy model (sdlc/06): `docs/deploy-model.md` specifies environments as a typed
  declaration, deploy as a gated transition on its own record (never a sixth brief-lifecycle
  state — `spec/lifecycle-v1.md` §9), the rollback obligation (a stated reverse path, or an
  explicitly accepted absence naming an approver — reconciled against, never contradicting,
  `docs/distribution.md`'s "there is no rollback" release statement), and runbooks as a
  typed recovery artefact with drill rows whose Evidence is filled by whoever ran the drill.
- A new DEPLOYS register (`docs/streams/deploys/`, reference implementation
  `statusgen/deploygate.go`): a DEPLOY record's `brief:` precondition (the carried brief
  must be `verified` or `done`, never merely `implemented`) is a hard `--lint` PROBLEM when
  unmet or dangling; the rollback grammar (`rollback: none-accepted` requires a named
  `rollback-approver`) is validated; and an undrilled RUNBOOK drill row is reported
  `could-not-check` — visible, never a silent pass and never a hard failure. Reuses the
  existing `blocked-by: env` marker for a deploy waiting on an environment, rather than
  minting a second one.
