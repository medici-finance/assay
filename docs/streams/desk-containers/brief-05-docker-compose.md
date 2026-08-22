---
brief: desk-containers/05
title: docker-compose definition for the five desks
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by desk-containers scoping session
sources:
  - "medici-finance/assay#63 — the request (secondary aim: launched via docker compose)"
  - "docs/streams/desk-containers/spec.md — secondary launch targets"
  - "containers/secrets.md (brief 02) — secrets: + env_file recipe"
  - "freshness-checked 2026-08-22 @ b3a2067 — no compose file exists in the tree"
why: >-
  The secondary aim of the request: one `docker compose run <desk>` that launches any
  desk with the same volumes and runtime-injected credentials as the desktop script, so
  a machine hosting several desks manages them as one stack.
---

# Brief 05 — docker-compose definition

## Context

files:
- `containers/compose.yaml` (new) — the five desk services.
- `containers/desk.env.example` (new) — placeholder env-file (variable NAMES with
  dummy values only, per `containers/secrets.md` (planned)).
- `docs/docker.md` — add the compose section.

facts:
- Five services, each named exactly by its desk name, image
  `ghcr.io/medici-finance/assay/<desk-name>:${DESK_VERSION:-latest}`; shared config via
  a YAML anchor (x-desk-common).
- Interactive: `stdin_open: true` + `tty: true`; the supported invocation is
  `docker compose run <desk-name>` (and `docker compose up -d` + `attach` for standing
  sessions).
- Per-desk named volume on `/work` — never one volume shared across desks.
- Credentials per `containers/secrets.md` (planned): top-level `secrets:` entry binding the host
  PEM file, mounted to `/run/secrets/assay/app.pem` read-only in each service;
  `env_file` for model credentials. No credential value appears in `compose.yaml` or
  in `desk.env.example`.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Placeholder values only in the example env-file — never a real credential.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `containers/compose.yaml` (planned) per the facts (anchor for common config; five
   services; per-desk volumes; secrets + env_file wiring).
2. Write `containers/desk.env.example` naming every variable from the brief-02
   contract with obviously-fake values.
3. Document the compose flow in `docs/docker.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `docker compose -f containers/compose.yaml config` (with a fixture PEM + env-file present) | exit 0 — the file resolves |
| 2 | `docker compose -f containers/compose.yaml config --services \| sort` | exactly: intake-desk, pr-review-desk, the-desk, verify-desk, worker-desk |
| 3 | `docker compose -f containers/compose.yaml config \| grep -c '/run/secrets/assay/app.pem'` | count ≥ 5 — every service mounts the PEM secret |
| 4 | `docker compose -f containers/compose.yaml config --volumes \| wc -l` | 5 — one named work volume per desk |
| 5 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' containers/compose.yaml containers/desk.env.example; test $? -eq 1` | exit 0 — no credential-shaped value in the compose file or example env |

## Definition of Done
- Verify rows green, recorded in Evidence by a non-implementer.
- Credentials arrive only at runtime via `secrets:` + `env_file` per the brief-02
  contract; **no secret in any image layer** and none in any committed compose/env
  file (row 5).
- `docker compose run <desk-name>` gives the same interactive session, volume, and
  credential surface as `desk-run.sh <desk-name>`.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — declarative wiring of the human-gated brief-02
contract; committed files carry placeholders only). Reviewer confirms rows 3-5 and
parity with the desk-run.sh surface.
