---
brief: desk-apps/07
title: statusgen Evidence-actor check — tamper-evident verified rows (closes [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md))
why: >-
  A `verified` row backed by prose ("runner: opus-verifier") is the [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) unbacked-row gap — anyone
  could write it. Once Evidence is committed by verifier-app[bot] (brief 04), statusgen can back a
  verified row by the Evidence commit's ACTOR, so "verified" means "the verifier App committed this,"
  at the same strength as the review gate.
wave: 3
depends: ["desk-apps/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by glm-5.2 session (human:<name>'s desk-apps direction, [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md))
sources: ["INTAKE [I-38](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-per-role-github-apps-verifier-issue-worker-actors.md) (statusgen backs a verified row by the Evidence commit's ACTOR, not prose runner cells; closes [I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md))", "tools/statusgen + ../assay-toolkit/statusgen (the generator — migration in progress; see facts)", "memory integrity-check-changes-are-human-gate (statusgen anti-falsification changes are human-gate — confirm with human:<name> at pickup)"]
---

# Brief 07 — statusgen Evidence-actor check

**CROSS-STREAM:** desk-tools' scoping explicitly excludes statusgen changes; this is the desk-apps
statusgen change (a reason desk-apps is its own stream). **Note:** statusgen anti-falsification /
integrity-check changes are human-gate per memory `integrity-check-changes-are-human-gate` — confirm
the gate with human:<name> at pickup (the actor-check may need `gate: human`).

## Context
files: `../assay-toolkit/statusgen/` (this repo) + `../assay-toolkit/statusgen/` (migration target — the
canonical home long-term). The actor-check lands in BOTH while the migration is live; the
implementer confirms which is source-of-truth at pickup.
facts:
- Today a stream-README `verified` cell is trusted prose (e.g. `2026-07-11 opus-verifier`).
- After brief 04, Evidence rows are committed by `assay-verifier-app[bot]`. statusgen gains a check:
  a `verified` row is backed ONLY if the corresponding Evidence commit's author/actor =
  `verifier-app[bot]` (or a human, `human:<name>`). A prose-only `verified` with no verifier-app
  Evidence commit → flagged (stale / unbacked), excluded from Next-up.
- This is an anti-falsification change — the memory flags this category as human-gate. Default the
  frontmatter to `gate: model` but FLAG for human:<name>'s call at pickup.

## Ground rules
- NEVER git push / trigger workflows. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.

## Task
1. Implement the actor-check: statusgen resolves each `verified` row's Evidence commit actor;
   require `verifier-app[bot]` or `human:<name>`; flag prose-only as unbacked.
2. Tests: a verified row with verifier-app Evidence → clean; a verified row with only prose →
   flagged; a verified row with a non-verifier-app actor → flagged.

## Verify (executable)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; incl. the three actor-check cases |
| 2 | `statusgen --root . --lint; echo $?` | 0 |
| 3 | **(live, blocked-on-human/brief-04)** a brief with verifier-app-committed Evidence → `--lint` clean; same brief prose-only → flagged | behaves as specified |

## Evidence
<!-- appended at implementation time by a NON-implementer. -->

## Review
Gate: model (confirm human-gate with human:<name> per `integrity-check-changes-are-human-gate`) +
`/security-review`. Reviewer confirms the actor-check cannot be satisfied by prose alone.
