---
brief: assay:assay:desk-containers:26
title: "Local PostgreSQL in Docker for a multi-cell host — pinned image, one database and restricted role per cell, runtime credentials, backup and restore drill, and the `cell.env` keys a cell reads its DSN from"
why: >-
  The non-CI target puts each cell's operational state in PostgreSQL, run first in a local
  Docker container and later on CloudNativePG. Nothing in the tree runs that server today.
  One host runs several cells, so the local profile is one shared server with a separate
  database and a restricted role per cell. This brief ships that server: a pinned image, a
  persistent volume, a health check, an idempotent per-cell provisioning step, credentials
  supplied at runtime and never committed, dump-based backups with a stated recovery point and
  a restore drill, and the two `cell.env` keys that tell a cell where its database is.
wave: 3
depends: []
unblocks: ["desk-containers/27"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-14 by the-desk (assay:author-brief, opus-5.5; requested by the driver)
sources:
  - "Driver request 2026-10-13: a local Docker PostgreSQL for one host running several cells — pinned version, compose service, persistent volume, one database and restricted role per cell, runtime credentials, health check, backup and restore, and cellctl wiring so a cell finds its DSN"
  - "docs/proposals/non-ci-architecture/11-target-architecture.md lines 196, 200 and 204 — PostgreSQL as the operational store, local Docker then CloudNativePG, per-cell databases and roles on a shared server with the administrator trusted"
  - "docs/proposals/non-ci-architecture/14-storage-forge-graph.md lines 7, 11, 13, 44, 46 and 56 — one runtime implementation; the local container is single-host and needs volumes, backups and recovery; restricted role per cell; ordinary-failure restore with no zero-loss claim; complete loss starts with admission paused; restore tooling must make the operator procedure unavoidable"
  - "docs/streams/graph-execution/README.md §\"Non-CI R2 routing — PostgreSQL coordination — 2026-10-13\" and docs/streams/graph-execution/brief-19-durable-instance-store.md — the consumer: 19 owns the store, `coordination-store-v1`, migrations and reconstruction; its tests use `ASSAY_TEST_POSTGRES_DSN` on a disposable server; 16 owns the operator incarnation outside PostgreSQL; 26 keeps its own schema in the same cell database"
  - "containers/secrets.md §4.2 (compose `secrets:`), §5 (fail closed, no ambient fallback) and §6 (no secret in any image layer) — the rules this brief follows unchanged"
  - "docs/streams/desk-containers/brief-05-docker-compose.md — plans `containers/compose.yaml` (planned) for the five desks (todo; no compose file is in the tree)"
  - "tools/desk/cmd/cellctl/envkeys.go (the env-key registry and `TestEnvKeyRegistryIsComplete`), launch.go (host launch env, `repairAdmissionValue` precedent), env.go (`scrubbedEnvKeys` and `scrubbedEnvValue`), codex_environment.go (forwarded key list), check.go (per-kind check rows; `DESKD_GITLAB_TOKEN_FILE` 0600 precedent), docs/cellctl.md §`cell.env` and §Parity with the shell oracle"
  - "Docker Hub `postgres` official image, fetched 2026-10-13: tag `18.6-trixie` exists; PostgreSQL 18 images use `PGDATA` set to `/var/lib/postgresql/18/docker` and `VOLUME /var/lib/postgresql`; `/docker-entrypoint-initdb.d` runs only on an empty data directory; `POSTGRES_PASSWORD_FILE` is supported; 19 is beta only"
  - "CloudNativePG v1.30 release notes (v1.30.1, 2026-09-23), fetched 2026-10-13: default operand image `18.6-system-trixie` — this brief pins the same PostgreSQL minor so brief 27's migration is same-version"
  - freshness-checked 2026-10-14 @ bcc02252e (origin/main) — no PostgreSQL compose service, container, script or doc under `containers/` or `docs/`; no DSN key in `tools/desk/cmd/cellctl/envkeys.go`; `containers/compose.yaml` (planned) does not exist
domain: complicated
outcome: none
consumers:
  - "docs/cellctl.md (the `cell.env` table gains `CELL_DB_DSN` and `CELL_DB_PASSFILE`; the scrubbed-cell env table and `check` rows gain the database entries): follow-up desk-containers/26 (this brief; flips to fixed-here when the implementation edits the path)"
  - "containers/README.md (the Layout table gains a `postgres/` row): follow-up desk-containers/26 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/docker.md (gains a one-line pointer to the PostgreSQL doc): follow-up desk-containers/26 (this brief; flips to fixed-here when the implementation edits the path)"
---

# Brief 26 — Local PostgreSQL in Docker for a multi-cell host

## Context
files:
- `containers/postgres/compose.yaml` (planned) (new — its own compose project, one `postgres` service, a named volume, a health check, the superuser password from a compose secret)
- `containers/postgres/initdb/00-harden.sql` (planned) (new — runs once, on first init: revokes `CONNECT` and `TEMPORARY` from `PUBLIC` on `postgres` and `template1`)
- `containers/postgres/provision-cell.sh` (planned) (new — idempotent per-cell database and role; `--disable-login` / `--enable-login`; prints the DSN and the `cellctl set` lines)
- `containers/postgres/backup.sh` (planned) (new — `pg_dump -Fc` per cell database plus globals, with a manifest)
- `containers/postgres/restore.sh` (planned) (new — restore one cell from one dump; the cell's role ends `NOLOGIN`)
- `containers/postgres/drill.sh` (planned) (new — the restore drill on a throwaway compose project; the Verify flow row)
- `containers/postgres/check-isolation.sh` (planned) (new — fails if `PUBLIC` holds `CONNECT` on any cell database or if a cell role has a forbidden attribute; brief 27 reuses it)
- `docs/postgres.md` (planned) (new — the operator doc: start, provision a cell, credentials, backup schedule, restore, drill, upgrade limits)
- `tools/desk/cmd/cellctl/envkeys.go` — register `CELL_DB_DSN` and `CELL_DB_PASSFILE` as `envCell` levers
- `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/cellctl/desk.go` — compose the two exported variables into the host launch and print them in the dry-run plan
- `tools/desk/cmd/cellctl/env.go` — add both exported variables to `scrubbedEnvKeys`, each with a `scrubbedEnvValue` case arm
- `tools/desk/cmd/cellctl/codex_environment.go` — add both exported variables to the forwarded key list
- `tools/desk/cmd/cellctl/check.go` — the database check rows
- `tools/desk/cmd/cellctl/celldb_test.go` (planned) (new)
- `docs/cellctl.md`, `containers/README.md`, `docs/docker.md` — see `consumers:`
- `changelog/desk-containers-26-local-postgres.md` (planned)

facts:
- **Who consumes this.** graph-execution/19 owns the cell coordination store: the `coordination-store-v1` contract, its schema, its migrations and its reconstruction. graph-execution/16 owns the operator incarnation, held outside PostgreSQL. graph-execution/26 keeps its process facts in its own schema in the same cell database. This brief owns only the server, the per-cell database and role, credentials, backups, and the cell's pointer to its database. It creates no consumer table and no migration. graph-execution/19's own tests need `ASSAY_TEST_POSTGRES_DSN` on a disposable server where they may create and drop databases. That is a separate test server; per-cell roles here deliberately lack `CREATEDB`. No `depends:`/`unblocks:` edge to graph-execution/19 is declared: 19 does not need this server to land.
- **A separate compose project, not brief 05's file.** Brief 05 plans `containers/compose.yaml` (planned) for the five desks, and it has not landed. The database gets its own project in `containers/postgres/compose.yaml` (planned), with project name `assay-postgres`. Reasons: the database outlives desk restarts, so `docker compose down` on the desks must never stop it. It serves host cells that are not containers. And it must not wait on brief 05. If brief 05 later wants the desks on the same network, it references this project's network as `external:`; it never copies the service. <!-- graph: not-a-gate -->
- **Image.** `postgres:18.6-trixie`, pinned by digest (`postgres:18.6-trixie@sha256:<digest>`). Re-establish the digest at implementation time from the registry, not from this brief. 18.6 is the CloudNativePG 1.30.1 default operand's PostgreSQL version, so brief 27's migration is same-major and same-minor. PostgreSQL 19 is beta only and is not used. Changing the major version of an existing volume in place is not supported by PostgreSQL: the doc says a major upgrade is dump, new volume, restore. The compose file refuses to start with an unpinned tag (Verify row 2).
- **Volume.** A named volume `assay-postgres-data` mounted at `/var/lib/postgresql`. The PostgreSQL 18 image moved its `VOLUME` there from `/var/lib/postgresql/data`, and its `PGDATA` is `/var/lib/postgresql/18/docker`. Mounting the old path leaves the data in an anonymous volume. Never place this volume on a network filesystem (doc 14 line 15).
- **Network exposure.** The port is published on `127.0.0.1` only, as `127.0.0.1:${ASSAY_PG_HOST_PORT:-5432}:5432`. No TLS in the local profile; the listener is loopback-only, and the doc says so. Remote cells use brief 27's cluster, which has TLS.
- **Health check.** `pg_isready -U postgres -d postgres` with a start period, interval and retries. `docker compose up --wait` returns only once the server is healthy.
- **Superuser password.** Arrives at runtime as a compose secret per `containers/secrets.md` §4.2, `secrets: pg_superuser: file: ${ASSAY_PG_SECRETS_DIR:?set ASSAY_PG_SECRETS_DIR}/superuser.password`, read through `POSTGRES_PASSWORD_FILE`, which points at `/run/secrets/pg_superuser`. `${VAR:?}` makes compose refuse to start when the directory is unset; there is no default path inside the repository. The doc suggests `~/.config/assay/postgres/` with directory mode 0700 and file mode 0600. No password, real or example, is committed. No image is built here, so §6's layer rule is met trivially.
- **Per-cell names.** Cell names may contain `-` and `/` (cells are paths under the cells root). The database and role name is `cell_` plus the cell name lowercased, with each `-`, `/` and `.` replaced by `_`. Example: `example-cell` maps to `cell_example_cell`. `provision-cell.sh` refuses a cell name outside `^[A-Za-z0-9][A-Za-z0-9._/-]*$` and a mapped name longer than 63 bytes (PostgreSQL's identifier limit). Two cell names can map to one identifier, for example `a-b` and `a/b`. To catch that, the script stores the original cell name with `COMMENT ON DATABASE`, and it refuses when an existing database's comment names a different cell.
- **Per-cell role.** `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`, with `CONNECTION LIMIT` set from a script flag (default 20). The role owns its database, so graph-execution/19 and /26 can create their own schemas in it without any other privilege.
- **Per-cell isolation.** `REVOKE CONNECT, TEMPORARY ON DATABASE <db> FROM PUBLIC`, so only the owner (and the trusted administrator) connects. `00-harden.sql` does the same on `postgres` and `template1`. Object ownership is the second layer: a role that did connect to another cell's database still owns nothing there and is granted nothing. This profile isolates cells from each other, not from the database administrator. The administrator is trusted (doc 11 line 204; doc 14 line 13), and a profile that needs more gets a separate server.
- **Provisioning is idempotent and runs on a live server.** `/docker-entrypoint-initdb.d` runs only when the data directory is empty, so it holds only `00-harden.sql`. Cells are added at any time with `provision-cell.sh --cell <name> --passfile <path>`. The script runs `psql` inside the container through `docker compose exec -T`, with SQL on stdin. The role password is never on any argv: the script reads it from the pgpass file and sends it on stdin as a psql `\set` value.
- **Per-cell credential file.** One file per cell in libpq pgpass format, `host:port:database:user:password`. The operator creates it, or `provision-cell.sh --generate` writes it at mode 0600 with 32 random bytes, base64url-encoded. The script refuses a file whose mode is not 0600 or 0400, and a password outside `[A-Za-z0-9_-]{32,}`; that set has no character pgpass or psql quoting treats specially. The file lives outside the repository, beside the cell's other hand-provisioned credentials, by the same rule as `DESKD_GITLAB_TOKEN_FILE`.
- **How a cell finds its database: two `cell.env` keys and no new cellctl verb.** `CELL_DB_DSN` is a libpq connection string with no password, for example `host=127.0.0.1 port=5432 dbname=cell_example_cell user=cell_example_cell sslmode=disable`. `CELL_DB_PASSFILE` is the absolute path of the cell's pgpass file. Both are registered in `envkeys.go` as `envCell` levers, so `cellctl set` accepts them and `TestEnvKeyRegistryIsComplete` covers them. When `CELL_DB_DSN` is set, every role window gets `ASSAY_CELL_DB_DSN=<dsn>` and `PGPASSFILE=<path>`. libpq reads `PGPASSFILE` itself, so the password never appears in an environment variable or on a command line. When the key is unset, both variables are absent: a cell with no database is unchanged. That follows the `repairAdmissionValue` precedent in `launch.go`. The composition goes in four places: host launch (`launch.go`), the dry-run plan (`desk.go`, one `[dry-run] env` line each), the scrubbed allowlist (`env.go` — append both to `scrubbedEnvKeys` and add `scrubbedEnvValue` arms that return "" when unset), and `codex_environment.go`'s forwarded key list. The parity fixtures set neither key, and an empty value is skipped. The shell-oracle parity diff must therefore stay clean with no oracle edit (Verify row 9).
- **`cellctl check` rows**, active only when `CELL_DB_DSN` is set; otherwise one `n/a` line:
  - the DSN has no `password=` keyword and no password in URL userinfo;
  - `CELL_DB_PASSFILE` is set, absolute and readable, and has mode 0600 or 0400. Use the same mode helper and Windows behaviour as the existing 0600 checks;
  - on a `container` kind cell, the check FAILS with a message saying container cells cannot reach the database yet. Container cells (`cell-containers-v1`) refuse any environment or mount beyond their plan and run on a dedicated network, so wiring them needs a schema change. That is out of scope here; the failure names it so it is never silently missing.
- **Backups: per-cell logical dumps.** `backup.sh --out <dir>` writes, for every `cell_%` database, `pg_dump -Fc` to `<dir>/<UTC stamp>/<db>.dump`. It also writes `pg_dumpall --globals-only --no-role-passwords` to `globals.sql`, plus a `manifest` holding each file's sha256, the server version and the dump start time. Retention: keep the newest N stamps (default 7, `--keep`). Scheduling is the operator's (cron, launchd or a systemd timer); the doc gives one example each and suggests daily.
- **Stated recovery point.** A restore returns a cell to the start of the dump it restores. The recovery point objective equals the dump interval, 24 hours with the suggested daily schedule. This profile has no point-in-time recovery; WAL archiving is brief 27's job. The doc states this plainly and claims no zero loss (doc 14 line 44). Each cell restores alone, without touching the other cells.
- **Restore leaves the cell unable to connect.** `restore.sh --cell <name> --dump <file>` first refuses if any session for the cell's role is open (`pg_stat_activity`); the operator stops the cell with `cellctl down <cell>` first. It then renames the current database aside as `<db>_pre_restore_<UTC stamp>` (never drops it), creates a fresh database with the same owner and the same `REVOKE ... FROM PUBLIC`, restores with `pg_restore --exit-on-error --single-transaction`, and sets the role `NOLOGIN`. The cell cannot write until the operator runs `provision-cell.sh --cell <name> --enable-login`. The doc places that step after the consumer's reconcile procedure (graph-execution/16's operator incarnation; doc 14 lines 46 and 56). This is the local layer that makes the procedure unavoidable. The consumer's own check that a mismatched activation record starts paused is the second, independent layer, owned by graph-execution/19.
- **The drill** is `drill.sh`. It brings up a throwaway compose project with a unique project name, a fresh volume and a free host port. It provisions two cells and proves isolation and attributes, then writes rows, backs up, and restores into a second throwaway server. It compares the dump against a dump of the restored database, with each side's data-only text sorted, and requires them equal. It proves the restored role is `NOLOGIN` until `--enable-login`, then tears everything down, including on failure (trap). It never touches the operator's `assay-postgres` project or volume.
- Not a core-system brief. The single control between a cell and another cell's data is `REVOKE CONNECT ... FROM PUBLIC`; object ownership is the second layer, and `check-isolation.sh` detects drift in the first.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands against any shared server, cluster or checkout. Every flow test uses a throwaway compose project with its own volume and is torn down afterwards; the operator's `assay-postgres` project is never started, stopped or written by a test.
- Stop at `implemented` — you do not set verified/done.
- Public repository: example values only (`example-cell`, `example.com`). No real organisation, host or person names, and no password, real or example, in any committed file.
- Do not edit `containers/secrets.md`. This brief follows its rules unchanged. Adding database credentials to that normative inventory, or changing any rule in it, is a separate decision under brief 02's human gate: record the need in the PR body.
- Do not create consumer tables, schemas or migrations; graph-execution/19 owns them.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `containers/postgres/compose.yaml` (planned) and `initdb/00-harden.sql` per the facts. Re-establish the image digest, the PostgreSQL 18 volume path and the `_FILE` support from the image's published documentation at implementation time, and record what you checked in the PR body.
2. Write `provision-cell.sh`, `backup.sh`, `restore.sh`, `check-isolation.sh` and `drill.sh`: POSIX sh or bash with `set -eu`, `shellcheck`-clean, each with `--help`. Each refuses rather than guesses, and no secret appears on any argv.
3. Wire cellctl: registry entries, the four composition sites, the check rows, and `tools/desk/cmd/cellctl/celldb_test.go` (planned). Update the CLI docs-parity and env-table tests the package already has.
4. Write `docs/postgres.md` (planned): start and stop, provisioning a cell, credentials, `cellctl set <cell> CELL_DB_DSN=…` and `CELL_DB_PASSFILE=…`, the backup schedule and recovery point, restore and re-enable, the drill, major upgrades, and the isolation boundary (administrator trusted). Add the pointer in `docs/docker.md`, the Layout row in `containers/README.md`, the `cell.env` rows and check rows in `docs/cellctl.md`, and the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `ASSAY_PG_SECRETS_DIR="$(mktemp -d)" docker compose -f containers/postgres/compose.yaml config -q` | exit 0 — the file parses. No docker → could-not-check. |
| 2 | check +dereference | `img=$(ASSAY_PG_SECRETS_DIR="$(mktemp -d)" docker compose -f containers/postgres/compose.yaml config --images) && printf '%s\n' "$img" \| grep -Eq '^postgres:18\.[0-9]+-trixie@sha256:[0-9a-f]{64}$' && docker buildx imagetools inspect "$img" > /dev/null` | exit 0 — the image is pinned by digest, and the pinned digest resolves in the registry. Offline → could-not-check. |
| 3 | check +flow | `env -u ASSAY_PG_SECRETS_DIR docker compose -f containers/postgres/compose.yaml config -q; test $? -ne 0` | exit 0 — compose refuses to resolve without a secrets directory (fail closed) |
| 4 | check | `ASSAY_PG_SECRETS_DIR="$(mktemp -d)" docker compose -f containers/postgres/compose.yaml config --format json \| jq -e '[.services.postgres.ports[].host_ip] \| all(. == "127.0.0.1")'` | `true` — published on loopback only |
| 5 | check | `grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' -e 'POSTGRES_PASSWORD=' $(find containers/postgres -type f); test $? -eq 1` | exit 0 — no credential-shaped value and no inline superuser password in any committed file |
| 6 | check | `shellcheck containers/postgres/*.sh` | exit 0 |
| 7 | gate:model +flow +mutation | `containers/postgres/drill.sh` (planned) | exit 0. The output shows: two cells provisioned; cell A can create a schema in its own database; cell A cannot connect to cell B's database, nor to `postgres`; neither role has `CREATEDB`, `CREATEROLE`, `SUPERUSER`, `REPLICATION` or `BYPASSRLS`; a wrong password is refused; the restored data matches; the role stays `NOLOGIN` after restore until `--enable-login`; `check-isolation.sh` passes; teardown leaves no `drill` volume (`docker volume ls`). Mutation: comment out the `REVOKE CONNECT` line for cell databases and re-run — the drill must fail on the cross-cell connect. No docker → could-not-check. |
| 8 | check | `cd tools/desk && go test ./cmd/cellctl/ -run '^TestCellDB$' -count=1 -v > "${TMPDIR:-/tmp}/celldb.out" 2>&1 && grep -F -e '--- PASS: TestCellDB (' "${TMPDIR:-/tmp}/celldb.out"` | exit 0 and one `--- PASS: TestCellDB` line. One top-level test, one subtest per case: set keys compose both variables into host, scrubbed and codex launches; unset keys compose neither; a DSN carrying `password=` or URL userinfo fails `check`; a passfile at mode 0644 fails `check`; a `container` cell with the key set fails `check` naming the gap |
| 9 | check +neighbour | `(cd tools/desk && go build -o cellctl ./cmd/cellctl) && CELLCTL_A=tools/cellctl/testdata/cellctl-shell-oracle.sh CELLCTL_B=tools/desk/cellctl bash tools/cellctl/tests/parity.test.sh` | exit 0 — the shell-oracle parity holds; no fixture sets the new keys, so no plan line changes |
| 10 | check | `cd tools/desk && go test ./cmd/cellctl/ -count=1` | exit 0 — the whole package, including the env-key registry, `set` lever and docs-parity tests |
| 11 | check | `statusgen --root . --lint` | exit 0 |
| 11a | check | `statusgen --root . --consumers --brief desk-containers/26` | exit 0 — the `consumers:` routing holds against the branch diff |
| 12 | gate:model +flow | `DRY_RUN=1 cellctl desk example-cell worker-desk` — run on a scratch cell named example-cell, after provisioning it on a throwaway compose project named assay-postgres-scratch (never the operator's) and setting both keys with cellctl set; then, from a shell carrying the composed variables, run psql against the DSN with `select current_user` | the dry-run prints `[dry-run] env` lines for `ASSAY_CELL_DB_DSN` and `PGPASSFILE`, neither containing a password. psql prints `cell_example_cell` without prompting for a password. Record the transcript. No docker or no scratch cell → could-not-check. |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model. All four risk answers are no. No credential is committed: the superuser password and each cell's password live in operator-created files outside the repository. The brief follows `containers/secrets.md` unchanged and leaves it unedited. Every test server is throwaway. A restore renames the previous database aside rather than dropping it, so nothing done here is irreversible. The reviewer checks:
- rows 3, 5 and 7, including row 7's mutation;
- that no script puts a password on an argv: grep the scripts for `PGPASSWORD`, `password=` and `-W` — every hit must be explained;
- that `restore.sh` never runs `DROP DATABASE` on the live name;
- that `containers/secrets.md` is not in the diff. If the PR edits it, that is a finding to escalate under brief 02's gate.
