---
brief: assay:assay:desk-containers:38
title: Desk skills and docker guidance adopt the cell dev-env file
why: >-
  Once the file, admission and launcher exist, the role skills still tell windows to keep their
  own environment files, and the docker guide still shows per-desk env-files for anything a desk
  needs. Agents follow the skills, so until those change the old habit returns. This brief updates
  each role's instructions to match: the coordinator maintains the file, the others read it, rows
  declare names, and no value reaches Evidence.
wave: 3
depends: [desk-containers/35, desk-containers/36, desk-containers/37]
unblocks: [desk-containers/39]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [6615]
schema: brief-v2
version: 1
authored: 2026-11-16 by assay-worker-app (issue #6615)
sources:
  - "Issue #6615 and briefs desk-containers/35, 36 and 37."
  - "freshness-checked 2026-11-16 @ 0d369ac33 (origin/main): plugins/assay/skills/verify-desk/SKILL.md (\"Rows that need a project secret or a live service\"), plugins/assay/skills/worker-desk/SKILL.md (\"A window holding a project role secret\"), plugins/assay/skills/the-desk/SKILL.md, plugins/assay/skills/pr-review-desk/SKILL.md, docs/docker.md (\"Run a desk\", per-desk env-file table)."
exec-tier: any
domain: clear
outcome: none
consumers:
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desk-containers/38 (dev-env rows, names and digest in Evidence, no values)"
  - "plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-containers/38 (read only, never write, no per-desk .env)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desk-containers/38 (coordinator maintains the file through the write verb)"
  - "docs/docker.md: follow-up desk-containers/38 (per-desk env-files carry model and App settings only)"
---

# Brief 38 — Desk skills adopt the cell dev-env file

## Context

files: `plugins/assay/skills/{verify-desk,worker-desk,pr-review-desk,the-desk}/SKILL.md`,
`docs/docker.md`, `changelog/desk-skills-dev-env.md` (planned).

facts (freshness-checked 2026-11-16 @ 0d369ac33):

- verify-desk's section "Rows that need a project secret or a live service — witnessed by an
  operator, never by the implementer's report" covers role secrets and the Environment-blocked
  queue; it says nothing about a cell dev-env file.
- worker-desk's "A window holding a project role secret dispatches only work proven trusted"
  covers role secrets only.
- `docs/docker.md` "Run a desk" passes one operator-held env-file to every desk
  (`--env-file=/path/to/desk.env`), and its table sends per-desk settings to a per-desk env-file
  (for example `~/.config/assay/worker-desk.env`). Those files carry model and App settings.

## Read first

Briefs 35, 36 and 37 as merged; `containers/secrets.md` §4.5.

## Ground rules

- Text only; one neutral body per skill. No skill gains a value specific to one deployment.
- Never loosen the role-secret sections; the dev-env text sits beside them.

## Task

1. **verify-desk.** A row whose Env declares names runs with those names from the cell dev-env
   file; Evidence records the declared names and the file sha256 from the run's attestation and
   never a value; a could-not-check from a missing name or a refused file is recorded as such and
   never re-run by hand with a value pasted in.
2. **worker-desk and pr-review-desk.** Read the dev-env file, never write it; never create a
   per-desk `.env`; a needed variable goes to the coordinator as a request, and the row declares
   its name.
3. **the-desk.** The coordinator maintains the file with `cellctl devenv set` and `unset` only,
   never by editing it, and dev-only values only.
4. **docs/docker.md.** Per-desk env-files keep model and App settings; dev credentials for
   Verify rows move to the cell dev-env file; point at `containers/secrets.md` §4.5.
5. Changelog fragment.

## Definition of Done

Every Verify row below passes.

## Verify

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference | tree | `grep -F 'ASSAY_CELL_DEV_ENV_FILE' plugins/assay/skills/verify-desk/SKILL.md` | At least one match. |
| 2 | check:ci | tree | `grep -F 'cellctl devenv set' plugins/assay/skills/the-desk/SKILL.md` | At least one match. |
| 3 | check:ci | tree | `grep -F 'dev-env file' plugins/assay/skills/worker-desk/SKILL.md` | At least one match. |
| 4 | check:ci | tree | `grep -F 'dev-env file' plugins/assay/skills/pr-review-desk/SKILL.md` | At least one match. |
| 5 | check:ci | tree | `grep -F '§4.5' docs/docker.md` | At least one match. |
| 6 | check +flow | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS"; the skills, the docker guide and the briefs lint together. |
| 8 | check | tree | `statusgen --consumers --brief desk-containers/38 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 7 | gate:model | tree | Review of the four skill diffs against brief 35 | Each role's text matches the contract; no role-secret section loosened. |

## Evidence

_Not yet run._
