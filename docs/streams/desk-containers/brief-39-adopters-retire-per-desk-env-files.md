---
brief: assay:assay:desk-containers:39
title: Adopter migration — retire per-desk .env files for the cell dev-env file
why: >-
  Adopters already run cells with a .env file per desk. Without a migration note they keep them,
  and the duplication, drift and unrecorded reach the issue names stay. This brief tells an
  adopter how to move each dev value into the cell's one file, declare the names in their Verify
  rows, and delete the old files.
wave: 4
depends: [desk-containers/38]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [6615]
schema: brief-v2
version: 1
authored: 2026-11-16 by assay-worker-app (issue #6615)
sources:
  - "Issue #6615 and briefs desk-containers/35 to 38."
  - "freshness-checked 2026-11-16 @ 0d369ac33 (origin/main): docs/adopting-assay.md (§8 Upgrading after install), docs/cellctl.md (§cell.env), docs/docker.md."
exec-tier: any
domain: clear
outcome: none
consumers:
  - "docs/adopting-assay.md: follow-up desk-containers/39 (migration steps)"
  - "docs/cellctl.md: follow-up desk-containers/39 (link from §cell.env to the migration steps)"
---

# Brief 39 — Adopters retire per-desk .env files

## Context

files: `docs/adopting-assay.md`, `docs/cellctl.md`,
`changelog/retire-per-desk-env.md` (planned).

facts (freshness-checked 2026-11-16 @ 0d369ac33): `docs/adopting-assay.md` has "## 8. Upgrading
after install — the `assay:upgrade-assay` skill"; `docs/cellctl.md` has "## `cell.env`". Neither
mentions a cell dev-env file.

## Read first

Briefs 35 to 38 as merged; `containers/secrets.md` §4.5.

## Task

1. Add a migration subsection under `docs/adopting-assay.md` §8: list the per-desk `.env` files
   a cell uses; sort each value (model or App setting stays in the per-desk env-file; a role
   secret moves to `CELL_ROLE_SECRETS`; a dev value moves to the dev-env file through the
   coordinator's `cellctl devenv set`; a production value is removed); set `CELL_DEV_ENV_FILE`;
   run `cellctl check`; add declared names to each Verify row that needs one, renaming each into
   the `DEVENV_` namespace brief 35 allows and passing it to the tool in the row's own command;
   then delete the old files.
2. Link to it from `docs/cellctl.md` §`cell.env`.
3. Changelog fragment.

## Definition of Done

Every Verify row below passes.

## Verify

| # | Class | Env | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +dereference | tree | `grep -F 'CELL_DEV_ENV_FILE' docs/adopting-assay.md` | At least one match. |
| 2 | check:ci | tree | `grep -F 'cellctl devenv set' docs/adopting-assay.md` | At least one match. |
| 3 | check +flow | host | `statusgen --root . --lint --acceptance-forge` | exit 0; output contains "LINT: PASS"; the migration doc, the cellctl doc and the briefs lint together. |
| 4 | check | tree | `statusgen --consumers --brief desk-containers/39 --base $(git merge-base origin/main HEAD)` | Pre-merge, on the implementation branch: exit 0. |
| 5 | gate:model | tree | Review of the migration steps on a scratch cell description | An adopter following them ends with no per-desk dev values and a passing `cellctl check`. |

## Evidence

_Not yet run._
