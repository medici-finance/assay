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

### Non-implementer verifier run — 2026-11-10 claude-opus-5-5 verifier at 612750a12406 (re-verification; all five rows pass, host verifyrun witness)

Fresh non-implementer re-verification on merged main 612750a12406. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#5947 run=99c21efa0c3b, carried in the attestation line under the table). containers/compose.yaml and containers/desk.env.example are unchanged since the 2026-11-02 run at dbfa166d5d96 (their last change is still f16f4f1e9); the deliverables arrived in 022d1744c, merged by medici-finance/assay#5227. No Verify row is classed check:ci, so the container bracket does not apply. The table below is the witness `statusgen verifyrun` (v1.0.81) wrote in write mode, run on the darwin-arm64 host from the admitted home on a clean tree, with TMPDIR outside the home and KUBECONFIG=/dev/null. The cell's statusgen shim runs the rows with the cell's config HOME, under which the docker CLI resolves Compose v2.35.1. Row 1's fixture: ASSAY_DESK_ENV_FILE named a one-line placeholder env-file and ASSAY_DESK_PEM_FILE_DIR named a directory of five placeholder PEM files (one line of text each, not keys), both outside the home. DOCKER_HOST named a socket under /var/empty that does not exist, so the docker CLI could reach no engine: every row only parsed and resolved the compose file, and no container, image pull or network call took place.

An earlier write-mode run in this dispatch was discarded as an environment fault: its fixture's DOCKER_HOST named a missing socket under the long session scratch path, and the Compose CLI refused to start because the unix socket path was too long, so rows 1 to 4 exited 1 before reading the compose file (row 1's output digest sha256:2e1b553bcc6f, reproduced by hand under the same HOME). Its witness was removed from the home and the rows were re-run once with the short socket path above; the table below is that second run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `docker compose -f containers/compose.yaml config` | pass exit=0 | sha256:5befcf3300ef | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `docker compose -f containers/compose.yaml config --services \| sort` | pass exit=0 | sha256:687c0691217c | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `docker compose -f containers/compose.yaml config \| grep -c '/run/secrets/assay/app.pem'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `docker compose -f containers/compose.yaml config --volumes \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' containers/compose.yaml containers/desk.env.example; test $? -eq 1` | pass exit=0 | sha256:2d8d2678bc4d | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5947 run=99c21efa0c3bf03bc0032178d85f74cf25935b1d48e77288aea647904cca7c69 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-containers/brief-05-docker-compose.md model=claude-opus-5-5 tier=any

Reading of the witness (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: 5 pass, 0 fail, 0 could-not-run or missing of 5 Verify rows; 0 author-added.
- Rows 2, 3 and 4 exit 0 whatever the count, so their Expect cells were read from the output. A supplemental hand re-run of each row command from the home, under the same HOME, fixture and environment (no row credit), reproduced all five digests exactly. Row 2's output is the five lines intake-desk, pr-review-desk, the-desk, verify-desk, worker-desk. Row 3's is 5. Row 4's is 5, padded by wc; the volumes are intake-desk-work, worker-desk-work, pr-review-desk-work, verify-desk-work and the-desk-work. Row 5's grep prints a count of 0 for both files and exits 1, so the test exits 0. Row 1's resolved config gives every service stdin_open and tty set to true, its own PEM secret at the app.pem target, its own work volume on /work, and only the fixture's placeholder model variable in its environment.
- Supplemental mutation probes, run in a throwaway copy of the two files outside the home (no row credit). Row 5 with one commented placeholder line carrying the Anthropic key prefix added to the example env-file: exit 1, the count for that file is 1. Row 3 with the worker-desk secret entry removed: the count drops to 4 and the exit stays 0. Row 4 with the-desk's volume removed and the-desk pointed at the worker volume: the count drops to 4 and the exit stays 0. Row 2 with the the-desk service removed: four names listed, exit 0. So rows 2, 3 and 4 discriminate on their output, not their exit status, and their outputs above meet their Expect cells.
- Parity with the launcher, checked by hand at 612750a12406: the PEM target equals pem_dst in containers/desk-run.sh line 53 and the entrypoint default in containers/entrypoint.sh line 34; the volumes follow the launcher's desk-work naming (desk-run.sh line 369); the default env-file path matches desk-run.sh line 301. The Definition of Done's interactive-session parity claim needs a live engine and image, and no Verify row covers it; it is not attested here.
- Fail-first: no row is risk-bearing (every risk answer is no and no row carries a risk tag), so no base run applies.

Enumeration (risk-bearing values). The brief carries risk fields that are all no, and it is not irreversible. Literals introduced by 022d1744c and f16f4f1e9 in containers/compose.yaml, unchanged at 612750a12406: the secret target /run/secrets/assay/app.pem (lines 48, 57, 66, 75, 84), the image tag default latest (lines 45, 54, 63, 72, 81), the five volume names (lines 106 to 115), the env-file default path (line 40), the PEM directory default (lines 93 to 101), stdin_open and tty true (lines 35 and 36) and the project name assay-desks (line 32). containers/desk.env.example sets no live value: every line is commented out. Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key), then the volume names (a compose desk and a launcher desk would see different /work), then the rest. Every one is undone by an edit; none is irreversible.

RISK-VALUE: DERIVED — target = /run/secrets/assay/app.pem @ containers/compose.yaml:48 — it is the path the runtime contract fixes in containers/secrets.md section 2 and the path the image's entrypoint reads by default (containers/entrypoint.sh:34), and the same path desk-run.sh mounts (containers/desk-run.sh:53), so compose and the launcher present one credential surface.

RISK-VALUE: DERIVED — name = intake-desk-work @ containers/compose.yaml:107 (and the four sibling names, lines 109 to 115) — desk-run.sh mounts the volume named for the desk plus -work (containers/desk-run.sh:369), so the names must be exactly these, with no project prefix, for the two surfaces to share one /work per desk. One volume per desk follows the brief's own fact that a volume is never shared across desks.

**VERIFY: PASS** — 5/5 Verify rows pass with 0 fail and 0 could-not-run at 612750a12406. Gate: model; status left where it is.

### Non-implementer verifier run — 2026-11-11 claude-opus-5-5 verifier at cb4946581f6e (re-verification under desk-tools v1.0.82; all five rows pass, host verifyrun witness)

Fresh non-implementer re-verification on merged main cb4946581f6e, run under desk-tools v1.0.82. An earlier run of this item today was invalidated by the desk-tools upgrade (medici-finance/assay#6125), and none of its output is used here. Admitted by the dispatcher check (exit 0, binding medici-finance/assay#6129 run=17e473f2d4a3, carried in the attestation line under the table). containers/compose.yaml and containers/desk.env.example are unchanged since the 2026-11-10 run at 612750a12406; their last change is still f16f4f1e9, and the deliverables arrived in 022d1744c, merged by medici-finance/assay#5227. The only later change to a deliverable is b52d35cfb, a two-line edit to docs/docker.md outside every Verify row. No Verify row is classed check:ci, so the container bracket does not apply. The table below is the witness `statusgen verifyrun` (v1.0.82) wrote in write mode, run once on the darwin-arm64 host from the admitted home on a clean tree, with TMPDIR outside the home and KUBECONFIG=/dev/null. The cell's statusgen shim runs the rows with the cell's config HOME, where the docker CLI resolves Compose v2.35.1. Row 1's fixture: ASSAY_DESK_ENV_FILE named a one-line placeholder env-file and ASSAY_DESK_PEM_FILE_DIR named a directory of five placeholder PEM files (one line of text each, not keys), both outside the home. DOCKER_HOST named a short unix socket path that does not exist, so the docker CLI could reach no engine (a preflight docker info exited 1). Every row only parsed and resolved the compose file; no container was started, no image pulled and no network call made. This dispatch made one attempt and discarded none.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `docker compose -f containers/compose.yaml config` | pass exit=0 | sha256:5b9ba58d774d | 2026-11-11 | assay-verifier-app[bot] @ cb4946581f6e (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `docker compose -f containers/compose.yaml config --services \| sort` | pass exit=0 | sha256:687c0691217c | 2026-11-11 | assay-verifier-app[bot] @ cb4946581f6e (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `docker compose -f containers/compose.yaml config \| grep -c '/run/secrets/assay/app.pem'` | pass exit=0 | sha256:f0b5c2c2211c | 2026-11-11 | assay-verifier-app[bot] @ cb4946581f6e (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `docker compose -f containers/compose.yaml config --volumes \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-11-11 | assay-verifier-app[bot] @ cb4946581f6e (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' containers/compose.yaml containers/desk.env.example; test $? -eq 1` | pass exit=0 | sha256:2d8d2678bc4d | 2026-11-11 | assay-verifier-app[bot] @ cb4946581f6e (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6129 run=17e473f2d4a3cadc7355b16e0cf6129361c3399badfe71a605c7b923292cdded source=cb4946581f6e4a95d9aa8d896fae3d0741c14e77 brief=docs/streams/desk-containers/brief-05-docker-compose.md model=claude-opus-5-5 tier=strong

Reading of the witness (plain prose, no row credit):
- `statusgen verifyrun --check` on the brief: exit 0, 5 pass, 0 fail, 0 could-not-run or missing of 5 Verify rows; 0 author-added.
- Rows 2, 3 and 4 exit 0 whatever the count, so their Expect cells were read from the output. A supplemental hand re-run of each row command from the home (same HOME, fixture and environment, no row credit) reproduced all five digests exactly. Row 2's output is the five lines intake-desk, pr-review-desk, the-desk, verify-desk, worker-desk. Row 3's is 5. Row 4's is 5, padded by wc; the volumes are intake-desk-work, worker-desk-work, pr-review-desk-work, verify-desk-work and the-desk-work. Row 5's grep prints a count of 0 for both files and exits 1, so the test exits 0. Row 1's resolved config gives every service stdin_open and tty set to true, its own PEM secret at the app.pem target, its own work volume on /work, and only the fixture's placeholder model variable in its environment. Row 1's digest differs from the earlier runs' because the resolved config embeds the fixture's absolute paths, which differ per run; rows 2 to 5 match the earlier digests.
- Supplemental mutation probes, run in a throwaway copy of the two files outside the home (no row credit). Row 5, with one commented placeholder line carrying the Anthropic key prefix added to the example env-file: exit 1, and the count for that file is 1. Row 3, with the worker-desk secret entry removed: the count drops to 4 and the exit stays 0. Row 4, with the-desk's volume removed and the-desk pointed at the worker volume: the count drops to 4 and the exit stays 0. Row 2, with the the-desk service removed: four names listed, exit 0. Row 1, with the env-file pointed at a missing file: exit 1, env file not found. With the PEM directory pointed at a missing directory it still exits 0, because compose config does not check a secret file exists. So rows 2, 3 and 4 discriminate on their output, not their exit status, and their outputs above meet their Expect cells. Row 1's fixture matters for the env-file, not the PEMs.
- Parity with the launcher, checked by hand at cb4946581f6e: the PEM target equals pem_dst in containers/desk-run.sh line 53 and the entrypoint default in containers/entrypoint.sh line 34. The volumes follow the launcher's desk-work naming (desk-run.sh line 369), and the default env-file path matches desk-run.sh line 301. The Definition of Done's interactive-session parity claim needs a live engine and image, and no Verify row covers it, so it is not attested here.
- Fail-first: no row is risk-bearing (every risk answer is no and no row carries a risk tag), so no base run applies.

Enumeration (risk-bearing values). The brief's risk fields are all no, and it is not irreversible. Literals introduced by 022d1744c and f16f4f1e9 in containers/compose.yaml, unchanged at cb4946581f6e: the secret target /run/secrets/assay/app.pem (lines 48, 57, 66, 75, 84), the image tag default latest (lines 45, 54, 63, 72, 81), the five volume names (lines 106 to 115), the env-file default path (line 40), the PEM directory default (lines 93 to 101), stdin_open and tty true (lines 35 and 36) and the project name assay-desks (line 32). containers/desk.env.example sets no live value: every line is commented out. Ranked by what breaks if wrong: the PEM target first (a desk would start without its App key), then the volume names (a compose desk and a launcher desk would see different /work), then the rest. An edit undoes every one, and none is irreversible.

RISK-VALUE: DERIVED — target = /run/secrets/assay/app.pem @ containers/compose.yaml:48 — the runtime contract fixes this path in containers/secrets.md section 2, the image's entrypoint reads it by default (containers/entrypoint.sh:34) and desk-run.sh mounts the same path (containers/desk-run.sh:53), so compose and the launcher present one credential surface.

RISK-VALUE: DERIVED — name = intake-desk-work @ containers/compose.yaml:107 (and the four sibling names, lines 109 to 115) — desk-run.sh mounts the volume named for the desk plus -work (containers/desk-run.sh:369), so the names must be exactly these, with no project prefix, for the two surfaces to share one /work per desk. One volume per desk follows the brief's own fact that a volume is never shared across desks.

**VERIFY: PASS** — 5/5 Verify rows pass with 0 fail and 0 could-not-run at cb4946581f6e. Gate: model; status left where it is.

## Review
Gate: model (all four risk answers no — declarative wiring of the human-gated brief-02
contract; committed files carry placeholders only). Reviewer confirms rows 3-5 and
parity with the desk-run.sh surface.
