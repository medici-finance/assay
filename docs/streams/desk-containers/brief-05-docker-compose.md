---
brief: assay:assay:desk-containers:05
title: docker-compose definition for the five desks
wave: 3
depends: ["desk-containers/02", "desk-containers/03"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
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
version: 1
id: 59f032dd-24bb-42f6-93f6-5e736c848eea
---

# Brief 05 — docker-compose definition

## Context

files:
- `containers/compose.yaml` (new) — the five desk services.
- `containers/desk.env.example` (new) — placeholder env-file (variable NAMES with dummy values only, per `containers/secrets.md` (planned)).
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

### Non-implementer verifier run — 2026-11-02 claude-opus-5-5 verifier at dbfa166d5d96 (first verification; all five rows pass, host verifyrun witness)

Fresh non-implementer verification on merged main dbfa166d5d96. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#5308 run=0e202752244042a1, carried in the attestation line under the table). The deliverables arrived in 022d1744c (merged by medici-finance/assay#5227, merge 379607771) with the follow-up f16f4f1e9. No Verify row is classed check:ci, so the container bracket does not apply to any row. The table below is the witness `statusgen verifyrun` (v1.0.69) wrote in write mode, run on the darwin host from the admitted home on a clean tree, with TMPDIR outside the home. Row 1's fixture: ASSAY_DESK_ENV_FILE named a one-line placeholder env-file and ASSAY_DESK_PEM_FILE_DIR named a directory of five placeholder PEM files (one line of text each, not keys), both outside the home. DOCKER_HOST named a socket that does not exist, so the docker CLI could reach no engine: every row only parsed and resolved the compose file, and no container, image pull or network call took place.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `docker compose -f containers/compose.yaml config` | pass exit=0 | sha256:97a528fd6b0d | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 2 | `docker compose -f containers/compose.yaml config --services \| sort` | pass exit=0 | sha256:687c0691217c | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 3 | `docker compose -f containers/compose.yaml config \| grep -c '/run/secrets/assay/app.pem'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 4 | `docker compose -f containers/compose.yaml config --volumes \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' containers/compose.yaml containers/desk.env.example; test $? -eq 1` | pass exit=0 | sha256:2d8d2678bc4d | 2026-11-02 | assay-verifier-app[bot] @ dbfa166d5d96 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5308 run=0e202752244042a1ed93fc5f097c721207d6af1d89de5999b19cf2ee4bc7bf7b source=dbfa166d5d967904f4b14956b8735933d48579d6 brief=docs/streams/desk-containers/brief-05-docker-compose.md model=claude-opus-5-5 tier=strong

Reading of the witness (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 5 pass, 0 fail, 0 could-not-run or missing of 5 Verify rows.
- Rows 2, 3 and 4 exit 0 whatever the count, so their Expect cells were read from the output. A supplemental hand re-run of each row command from the home, with the same fixture and environment (no row credit), reproduced all five digests exactly. Row 2's output is the five lines intake-desk, pr-review-desk, the-desk, verify-desk, worker-desk. Row 3's is 5. Row 4's is 5, padded by wc; the volumes are intake-desk-work, worker-desk-work, pr-review-desk-work, verify-desk-work and the-desk-work. Row 5's grep prints a count of 0 for both files and exits 1, so the test exits 0. Row 1's resolved config gives every service stdin_open and tty set to true, its own PEM secret at the app.pem target, its own work volume on /work, and only the fixture's placeholder model variable in its environment.
- Supplemental mutation probes, run in a throwaway copy of the two files outside the home (no row credit). Row 5 with one placeholder line containing the Anthropic key prefix added to the example env-file: exit 1, the count for that file is 1. Row 3 with the app.pem target removed from one service: the count drops to 4 and the exit stays 0. So row 3, like rows 2 and 4, discriminates on its output, not its exit status.
- Parity with the launcher, checked by hand: the PEM target equals pem_dst in containers/desk-run.sh line 53 and the entrypoint default in containers/entrypoint.sh line 34; the volumes follow the launcher's desk-work naming (desk-run.sh line 369); the default env-file path matches desk-run.sh line 301. The Definition of Done's interactive-session parity claim needs a live engine and image, and no Verify row covers it; it is not attested here.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals introduced by 022d1744c and f16f4f1e9 in containers/compose.yaml: the secret target /run/secrets/assay/app.pem (lines 48, 57, 66, 75, 84), the image tag default latest (lines 45, 54, 63, 72, 81), the five volume names (lines 106 to 115), the env-file default path (line 40), the PEM directory default (lines 93 to 101), stdin_open and tty true (lines 35 and 36) and the project name assay-desks (line 32). containers/desk.env.example sets no live value: every line is commented out. Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key), then the volume names (a compose desk and a launcher desk would see different /work), then the rest. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — target = /run/secrets/assay/app.pem @ containers/compose.yaml:48 — it is the path the runtime contract fixes in containers/secrets.md section 2 and the path the image's entrypoint reads by default (containers/entrypoint.sh:34), and the same path desk-run.sh mounts (containers/desk-run.sh:53), so compose and the launcher present one credential surface.

RISK-VALUE: DERIVED — name = intake-desk-work @ containers/compose.yaml:107 (and the four sibling names, lines 109 to 115) — desk-run.sh mounts the volume named for the desk plus -work (containers/desk-run.sh:369), so the names must be exactly these, with no project prefix, for the two surfaces to share one /work per desk. One volume per desk follows the brief's own fact that a volume is never shared across desks.

**VERIFY: PASS** — 5/5 Verify rows pass with 0 fail and 0 could-not-run at dbfa166d5d96. Gate: model; status left where it is.

### Non-implementer verifier re-run — 2026-11-06 claude-opus-5-5 verifier at e9410e9e1354 (all five rows pass, host verifyrun witness)

Fresh non-implementer re-verification on merged main e9410e9e1354, admitted by the dispatcher check (exit 0, binding medici-finance/assay#5584 run=05f090f1f6b6, carried in the attestation line under the table). containers/compose.yaml and containers/desk.env.example are unchanged since f16f4f1e9 (the follow-up to the delivering merge 379607771, medici-finance/assay#5227). No Verify row is classed check:ci, so the container bracket applies to no row. The table below is the witness `statusgen verifyrun` (v1.0.75) wrote in write mode on the darwin host from the admitted home, normal HOME, no forge variable set, TMPDIR outside the home. Row 1's fixture, outside the home: ASSAY_DESK_ENV_FILE named a one-line placeholder env-file (ANTHROPIC_MODEL only) and ASSAY_DESK_PEM_FILE_DIR a directory of five one-line placeholder PEM files (plain text, not keys). DOCKER_HOST named a socket that does not exist, so no engine was reachable: every row only parsed and resolved the compose file; no container, pull or network call took place.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `docker compose -f containers/compose.yaml config` | pass exit=0 | sha256:2e76a8900f39 | 2026-11-06 | assay-verifier-app[bot] @ e9410e9e1354 (on-behalf-of human:ian) (forge-identity) |
| 2 | `docker compose -f containers/compose.yaml config --services \| sort` | pass exit=0 | sha256:687c0691217c | 2026-11-06 | assay-verifier-app[bot] @ e9410e9e1354 (on-behalf-of human:ian) (forge-identity) |
| 3 | `docker compose -f containers/compose.yaml config \| grep -c '/run/secrets/assay/app.pem'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-11-06 | assay-verifier-app[bot] @ e9410e9e1354 (on-behalf-of human:ian) (forge-identity) |
| 4 | `docker compose -f containers/compose.yaml config --volumes \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-11-06 | assay-verifier-app[bot] @ e9410e9e1354 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' containers/compose.yaml containers/desk.env.example; test $? -eq 1` | pass exit=0 | sha256:2d8d2678bc4d | 2026-11-06 | assay-verifier-app[bot] @ e9410e9e1354 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5584 run=05f090f1f6b613c5b6f5e7b023a709578f0ca4f2f76de8afded3a6be50e97109 source=e9410e9e13540d3f9565de9d986334ea4b644bce brief=docs/streams/desk-containers/brief-05-docker-compose.md model=claude-opus-5-5 tier=any

Reading of the witness (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 5 pass, 0 fail, 0 could-not-run/missing of 5 Verify rows.
- Rows 2, 3 and 4 exit 0 whatever the count, so their Expect cells were read from output. A supplemental hand re-run of the row commands from the home, same fixture and environment (no row credit), reproduced row 1's digest 2e76a8900f39 and printed: row 2 the five lines intake-desk, pr-review-desk, the-desk, verify-desk, worker-desk; row 3 the count 5; row 4 the count 5 (wc-padded; volumes intake-desk-work, worker-desk-work, pr-review-desk-work, verify-desk-work, the-desk-work); row 5 a count of 0 for both files with grep exit 1, so the test exits 0. Rows 2 to 5 digests equal the 2026-11-02 run's; row 1's differs only because the resolved config embeds this run's fixture paths. Row 1's resolved config gives every service stdin_open and tty true, its own PEM secret at the app.pem target, its own volume on /work and only the fixture's placeholder model variable.
- Fail-first: not run, because no row is risk-bearing. Every risk answer in the frontmatter is no and the Verify table has no row tags (columns #, Command, Expect only), so `--fail-first` has no row to run at base.
- The Definition of Done's interactive-session parity claim needs a live engine and image; no Verify row covers it and it is not attested here.

Enumeration (risk-bearing values). Not irreversible; risk metadata present and all no; the delivering diff (022d1744c, f16f4f1e9) touches no risk-classed path. Literals in containers/compose.yaml: secret target /run/secrets/assay/app.pem (lines 48, 57, 66, 75, 84); image tag default latest (lines 45, 54, 63, 72, 81); env-file default path (line 40); PEM directory default (lines 93 to 101); volume names (lines 106 to 115); stdin_open and tty true (lines 35, 36); project name assay-desks (line 32). containers/desk.env.example sets no live value (every line commented out). Ranked: the PEM target first (a desk would start without its App key), then the volume names (compose and launcher desks would see different /work), then the rest; every one is undone by an edit and redeploy.

RISK-VALUE: DERIVED — target = /run/secrets/assay/app.pem @ containers/compose.yaml:48 — the path containers/secrets.md fixes (its table line 39 and section 2, line 55), the entrypoint's default (containers/entrypoint.sh:34) and the launcher's pem_dst (containers/desk-run.sh:53), so compose and desk-run.sh present one PEM surface.

RISK-VALUE: DERIVED — name = intake-desk-work @ containers/compose.yaml:107 (siblings at lines 109, 111, 113, 115) — desk-run.sh mounts the volume named for the desk plus -work (containers/desk-run.sh:369), so these exact unprefixed names give one shared /work per desk across both surfaces and never one volume across desks.

**VERIFY: PASS** — 5/5 Verify rows pass with 0 fail and 0 could-not-run at e9410e9e1354. Gate: model; status left where it is.

## Review
Gate: model (all four risk answers no — declarative wiring of the human-gated brief-02
contract; committed files carry placeholders only). Reviewer confirms rows 3-5 and
parity with the desk-run.sh surface.
