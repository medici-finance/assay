# DEPLOYS register — deploy records and runbooks

This directory is the DEPLOYS register (
[`../../deploy-model.md`](../../deploy-model.md)): one record per file, either
`DEPLOY-<slug>.md` (a deploy transition) or `RUNBOOK-<slug>.md` (a recovery runbook with
drill rows). It is a register, not a stream — stream discovery skips it.

A **DEPLOY** record is the artifact a deploy transition leaves. It carries the environment
targeted, the brief it deploys (which MUST already be `verified` or `done` — never merely
`implemented`), the named human deploy authority for that environment, and the rollback
obligation (a reverse path, or an explicitly accepted absence naming an approver).

A **RUNBOOK** record is a typed recovery artefact: a trigger, a cadence, the steps, and
**drill rows** whose Evidence is filled by whoever ran the drill. A runbook with any
undrilled row (empty Evidence) is reported `could-not-check` by `--lint` — visibly unproven,
never silently "ready".

See [`../../deploy-model.md`](../../deploy-model.md) for the full specification and both
records' frontmatter templates. Reference implementation: `statusgen/deploygate.go`.
