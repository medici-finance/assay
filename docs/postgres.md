# Local PostgreSQL for a multi-cell host

One PostgreSQL server in Docker, on loopback, serving the host cells on one machine: one
database and one restricted role per cell, a runtime-only superuser credential, per-cell logical
backups, a one-cell restore that never runs cell code as the superuser, and a drill that proves
all of it on throwaway servers. The files live in [`containers/postgres/`](../containers/postgres/).

This profile owns the server, the per-cell database and role, their credentials, backups, and the
two `cell.env` keys that tell a cell where its database is. It creates no consumer table, schema
or migration: what goes inside a cell's database belongs to the consumer that owns it. Each cell
role owns its database, so a consumer creates its own schemas there without further privilege.
Cell roles cannot create databases; a test suite that creates and drops databases needs its own
disposable server, not this one.

**Read [§Isolation boundary](#isolation-boundary) before you rely on any of this to keep cells
apart.** On a shared OS user it keeps cell roles apart at the database layer and nothing more.

## Files

| Path | What it is |
|------|------------|
| `compose.yaml` | The `assay-postgres` compose project: the pinned image, the loopback port, the data volume, the health check, the superuser secret. |
| `initdb/00-harden.sql` | Runs once on a fresh volume: PUBLIC loses `CONNECT` and `TEMPORARY` on `postgres` and `template1`. |
| `provision-cell.sh` | Creates or converges one cell's database and role on the live server; `--enable-login` / `--disable-login`. |
| `backup.sh` | Per-cell `pg_dump -Fc` files plus globals and a manifest, with retention. |
| `restore.sh` | Restores one cell from one dump; the cell's role ends `NOLOGIN`. |
| `check-isolation.sh` | Read-only drift check of the database-layer controls. |
| `drill.sh` | The restore drill on two throwaway compose projects. |
| `lib.sh` | Helpers the scripts share (sourced, never run). |
| `scram-verifier.py` | Computes a SCRAM-SHA-256 verifier on the client (Python standard library only), after an RFC 7677 self-test. |

Every script takes `--help`, runs with `set -euo pipefail`, refuses rather than guesses, and never
puts a password on an argv or in an environment variable.

## The server

- **Image.** `postgres:18.6-trixie`, pinned by digest in `compose.yaml`. To move the pin, resolve
  the new digest from the registry (`docker buildx imagetools inspect postgres:<tag>`), never copy
  one from a document. PostgreSQL 18.6 matches the CloudNativePG 1.30.1 default operand, so a later
  move to such a cluster is same-major and same-minor.
- **Volume.** The named volume `assay-postgres-data` (compose prefixes the project:
  `assay-postgres_assay-postgres-data`) at `/var/lib/postgresql`. The PostgreSQL 18 image keeps
  `PGDATA` at `/var/lib/postgresql/18/docker`; mounting the pre-18 path `/var/lib/postgresql/data`
  leaves the data in an anonymous volume. **Never put this volume on a network filesystem.**
- **Network.** Published on `127.0.0.1:${ASSAY_PG_HOST_PORT:-5432}` only. **No TLS**: the
  listener is loopback-only. Cells on other machines need a server with TLS, not this one.
- **Authentication.** `POSTGRES_INITDB_ARGS=--auth-local=peer --auth-host=scram-sha-256`
  replaces the image's default `trust` for the socket and loopback: inside the container the socket
  takes only the matching OS user (the scripts' `docker compose exec -u postgres` superuser path),
  and every TCP connection, loopback included, needs a SCRAM password. These are initdb arguments:
  they apply to a fresh volume.
- **Separate project.** The database is its own compose project, not part of any desk compose
  file, so `docker compose down` on the desks never stops it and host cells that are not containers
  use it too. A desk compose file that wants its network references it as `external:`; it never
  copies the service.
- **`pg_stat_statements` is not preloaded.** Its view would keep the text of a `CREATE ROLE`,
  verifier included. The provisioning and restore sessions also run
  `SET pg_stat_statements.track_utility = off`, so the property survives if you add the module
  later.

## Credentials

The **superuser password** is a compose secret, read at runtime from
`$ASSAY_PG_SECRETS_DIR/superuser.password` (the runtime-injection rule of
[`containers/secrets.md`](../containers/secrets.md) §4.2). Compose refuses to resolve the file
when `ASSAY_PG_SECRETS_DIR` is unset; there is no default path inside the repository. A suggested
home is `~/.config/assay/postgres/`, directory mode 0700, file mode 0600:

```sh
export ASSAY_PG_SECRETS_DIR="$HOME/.config/assay/postgres"
mkdir -p -m 700 "$ASSAY_PG_SECRETS_DIR"
( umask 077; head -c 32 /dev/urandom | base64 | tr -d '\n=' | tr '+/' '-_' \
    > "$ASSAY_PG_SECRETS_DIR/superuser.password" )
```

The image's entrypoint reads the file through `POSTGRES_PASSWORD_FILE` and, inside the
container, exports its value as `POSTGRES_PASSWORD` into the server process's environment. That
is the image's behaviour (the container's configuration carries only the file path); anyone who
can `docker compose exec` into the container can already act as the superuser (see
[§Isolation boundary](#isolation-boundary)).

Each **cell's credential** is one libpq pgpass file, `host:port:database:user:password`, outside
the repository, beside the cell's other hand-provisioned credentials. Create it yourself or let
`provision-cell.sh --generate` write it (mode 0600, 32 random bytes base64url-encoded). The script
refuses a file whose mode is not 0600 or 0400, a file with more than one entry, and a password
outside `[A-Za-z0-9_-]{32,}` (no character pgpass or psql quoting treats specially). Mode 0600
guards against other OS users only — not against other cells on the same OS user.

**The plaintext password never reaches the server.** PostgreSQL logs a failing statement in full
by default, and the server log is the container log. So `provision-cell.sh` and `restore.sh`
compute a SCRAM-SHA-256 verifier on this host (`scram-verifier.py`, 4096 iterations, a 16-byte
random salt: the form `psql`'s `\password` sends) and send only that, on stdin. The script runs
the RFC 7677 test vector first and refuses on any failure; there is no plaintext fallback. As an
independent second layer the session first runs `SET log_min_error_statement = panic` and
`SET log_statement = none`, so even a failing statement is not written out.

The superuser secret, the pgpass files and the backup directory are database credentials that
[`containers/secrets.md`](../containers/secrets.md)'s inventory does not list yet. This profile
follows that document's rules unchanged; adding them to the inventory is a separate decision.

## Start and stop

```sh
export ASSAY_PG_SECRETS_DIR="$HOME/.config/assay/postgres"
docker compose -f containers/postgres/compose.yaml up -d --wait    # returns once healthy
docker compose -f containers/postgres/compose.yaml stop            # keeps the volume
docker compose -f containers/postgres/compose.yaml start
```

`down` removes the container and keeps the volume. **`down -v` deletes every cell's data.** The
health check is `pg_isready` over TCP to `127.0.0.1` inside the container: during first
initialisation the image runs a socket-only temporary server, which a socket probe would report as
ready too early.

## Provision a cell

```sh
containers/postgres/provision-cell.sh --cell example-cell \
  --passfile "$HOME/.config/assay/cells/example-cell.pgpass" --generate
```

The database and the role are both named `cell_` plus the cell name lowercased, with each `-`,
`/` and `.` replaced by `_` (`example-cell` becomes `cell_example_cell`). Names outside
`^[A-Za-z0-9][A-Za-z0-9._/-]*$`, or mapping to more than 63 bytes, are refused. Two cell names
can map to one identifier (`a-b` and `a/b`): the database's comment records the cell name, and a
second cell that maps onto it is refused.

The role is `LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS` with
`CONNECTION LIMIT` from `--connection-limit` (default 20), and owns its database; PUBLIC loses
`CONNECT` and `TEMPORARY` on it. Running the script again converges the role, its password, the
ownership and the grants. An existing role keeps its `LOGIN`/`NOLOGIN` state: only
`--enable-login` turns a login back on, so re-provisioning cannot undo a restore's `NOLOGIN`.

The script prints the DSN and the two lines that point the cell at it:

```sh
cellctl set example-cell 'CELL_DB_DSN=host=127.0.0.1 port=5432 dbname=cell_example_cell user=cell_example_cell sslmode=disable'
cellctl set example-cell "CELL_DB_PASSFILE=$HOME/.config/assay/cells/example-cell.pgpass"
```

With `CELL_DB_DSN` set, every role window of the cell gets `ASSAY_CELL_DB_DSN` (the DSN, which
carries no password) and `PGPASSFILE` (the path); libpq reads the password from the file itself.
Unset, neither variable exists in the window, and one the launching shell carried (another
database cell's) is removed rather than passed through; with `CELL_DB_DSN` set and no
`CELL_DB_PASSFILE`, the cell gets no `PGPASSFILE` at all. A DSN that carries a password is never
composed: `cellctl desk` refuses the launch without printing it. `cellctl check` adds rows for a cell
with a database: the DSN carries no password, the passfile is absolute, readable and 0600/0400,
and a `container` kind cell fails (container cells cannot reach this database yet). See
[`docs/cellctl.md`](cellctl.md).

`check-isolation.sh` checks the server for drift in the database-layer controls (PUBLIC grants on
`cell_%`, `aside_%`, `postgres` and `template1`; cell database ownership; forbidden role
attributes; no superuser but `postgres`; no leftover `assay_restore`). It never changes anything.

## Backups

```sh
containers/postgres/backup.sh --out "$HOME/.local/share/assay/postgres-backups" --keep 7
```

Each run writes `<out>/<UTC stamp>/`: one `pg_dump -Fc` file per `cell_%` database,
`globals.sql` (`pg_dumpall --globals-only --no-role-passwords`) and a `manifest` holding the start
time, the server version and each file's sha256. It runs under `umask 077` (directory 0700, files
0600), refuses an `--out` inside the repository working tree, and keeps the newest `--keep` stamp
directories. The backup directory holds every cell's data: it is as sensitive as the database.

Scheduling is yours; daily is suggested. One example each (set `ASSAY_PG_SECRETS_DIR`, and use
absolute paths):

```sh
# cron (crontab -e): 03:15 every day
15 3 * * * ASSAY_PG_SECRETS_DIR=$HOME/.config/assay/postgres /path/to/checkout/containers/postgres/backup.sh --out $HOME/.local/share/assay/postgres-backups
```

```xml
<!-- launchd: $HOME/Library/LaunchAgents/com.example.assay-postgres-backup.plist -->
<plist version="1.0"><dict>
  <key>Label</key><string>com.example.assay-postgres-backup</string>
  <key>ProgramArguments</key><array>
    <string>/path/to/checkout/containers/postgres/backup.sh</string>
    <string>--out</string><string>/path/to/homedir/.local/share/assay/postgres-backups</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <key>ASSAY_PG_SECRETS_DIR</key><string>/path/to/homedir/.config/assay/postgres</string>
  </dict>
  <key>StartCalendarInterval</key><dict><key>Hour</key><integer>3</integer><key>Minute</key><integer>15</integer></dict>
</dict></plist>
```

```ini
# systemd --user: ~/.config/systemd/user/assay-postgres-backup.service
[Service]
Type=oneshot
Environment=ASSAY_PG_SECRETS_DIR=%h/.config/assay/postgres
ExecStart=/path/to/checkout/containers/postgres/backup.sh --out %h/.local/share/assay/postgres-backups

# ~/.config/systemd/user/assay-postgres-backup.timer
[Timer]
OnCalendar=*-*-* 03:15
Persistent=true
[Install]
WantedBy=timers.target
```

**Recovery point.** A restore returns a cell to the START of the dump it restores. The recovery
point objective is the dump interval: 24 hours on the daily schedule. This profile has no
point-in-time recovery and no WAL archiving, and claims no zero-loss recovery. Each cell restores
on its own, without touching the others.

## Restore

```sh
cellctl down example-cell
containers/postgres/restore.sh --cell example-cell \
  --dump "$HOME/.local/share/assay/postgres-backups/<stamp>/cell_example_cell.dump"
# ... run the consumer's reconcile procedure ...
containers/postgres/provision-cell.sh --cell example-cell --enable-login
```

In order, `restore.sh`:

1. sets the cell's role `NOLOGIN` before anything else — **it stays `NOLOGIN`**;
2. refuses while any session of that role is open (stop the cell first);
3. renames the current database aside to `aside_<YYYYMMDDTHHMMSSZ>_<8 hex>` (never dropped);
4. creates a fresh database with the same owner, the same `REVOKE ... FROM PUBLIC` and the cell
   comment;
5. creates each extension the dump lists, by **name** only, as the superuser (the image ships its
   scripts; the dump's own text for it is never run). It refuses an extension the image lacks,
   one the image does not mark trusted unless you pass `--allow-extension <name>` after review,
   and one that lives in a schema the dump itself creates;
6. creates the one-shot role `assay_restore` (`NOINHERIT`, `CONNECTION LIMIT 1`,
   `VALID UNTIL` now plus `--deadline-minutes`, default 60, a random password set by verifier),
   granted the cell role `WITH INHERIT FALSE, SET TRUE` and `CONNECT` on the fresh database; it
   refuses if `assay_restore` already exists, so only one restore runs at a time;
7. loads the data with `pg_restore --role=<cell role> --no-owner --exit-on-error
   --single-transaction`, **authenticated as `assay_restore`**: TCP to `127.0.0.1` inside the
   container (SCRAM; the container's socket would be peer-authenticated and make the password
   moot), the password in a 0600 pgpass file in a `mktemp -d` directory. It first checks that the
   session is a non-superuser `assay_restore` session and refuses otherwise;
8. on every exit path, as the superuser: (1) `ALTER ROLE assay_restore NOLOGIN`; (2) terminates
   its sessions; (3) `REASSIGN OWNED BY assay_restore TO <cell role>` and `DROP OWNED BY
   assay_restore` in the restored database; (4) `DROP ROLE assay_restore`. A failed step is
   named (`teardown step 4: DROP ROLE assay_restore failed`) and the restore fails even when the
   data loaded.

Why the data load never runs as the superuser: a cell's dump is cell-controlled input. The cell
role owns its database, so it can define a `SECURITY INVOKER` function called from a `CHECK`
constraint, and the load calls it for every row as the restoring session. `SET ROLE` alone does not
contain it, because the function can `RESET ROLE`. Here that lands on `assay_restore`, which holds
nothing but `CONNECT` on the fresh database. `VALID UNTIL` is the second layer behind the
teardown: a role that survives a failed teardown cannot log in past the deadline, whatever
password cell code set for it, and `check-isolation.sh` fails while it exists.

The cell stays unable to connect until the consumer's reconcile procedure has run and you run
`--enable-login`. **Aside databases are not backed up** (`backup.sh` dumps `cell_%` only) and
nothing treats one as a cell. To keep one, dump it by hand
(`docker compose -f containers/postgres/compose.yaml exec -T -u postgres postgres pg_dump -Fc -d <aside name> > <file>`);
to discard it, `DROP DATABASE` it as the superuser once you no longer need it.

## Drill

```sh
containers/postgres/drill.sh
```

The drill brings up two throwaway compose projects, `assay-pg-drill-<id>-a` and `-b`, each with
a fresh volume and a free loopback port, and tears both down (`down -v`) on every exit path. It
provisions two cells and checks the database layer between their roles (no connection to the
other cell's database or to `postgres`, no forbidden role attribute, a wrong password refused),
forces one provisioning run to fail at its role statement and checks that the container log holds
neither that cell's password nor any SCRAM verifier, plants an escalation and persistence probe
in one cell, backs up, restores into the second server, and checks the restore: no new superuser,
no `assay_restore` left (nor anything it owns), the probe's password does not log in, the probe's
marker row was never written, the teardown ran steps 1–4 in order, the role is `NOLOGIN` until
`--enable-login`, `check-isolation.sh` passes, and the restored data equals the dump's. Finally it
checks that no drill volume is left. It never names the operator's `assay-postgres` project.
Without a running Docker daemon it refuses (could-not-check, never a pass). Run it after any
change to these scripts, and after moving the image pin.

## Major upgrades

PostgreSQL does not open an existing data directory with a newer major version. A major upgrade
is: `backup.sh`, stop the server, start the new major on a **new** volume, provision every cell
again (same passfiles), `restore.sh` each cell, run each consumer's reconcile procedure, and
`--enable-login`. Keep the old volume until the new server has run through a backup cycle.

## Isolation boundary

**What holds: the database layer isolates cell roles from each other.** One cell's role, used as
that role, cannot open another cell's database or act outside its own: PUBLIC holds no `CONNECT`
on any cell database (the single control), and a role that did connect owns nothing there and is
granted nothing beyond PostgreSQL's default PUBLIC privileges (`USAGE` on schema `public`,
`EXECUTE` on functions whose owner has not revoked it) — the second layer. `check-isolation.sh`
detects drift in the first.

**The administrator is trusted.** This profile does not isolate cells from the database
superuser. A profile that needs that gets a separate server.

**Host cells on one OS user are NOT separated at the credential layer.** Every host cell runs as
the operator's OS user, with that user's file authority, and every pgpass file and the backup
directory belong to that same user. Any host cell's process can read another cell's pgpass file
and connect as that cell, and `REVOKE CONNECT` never comes into play. Worse, **any such cell can
reach the superuser**, by either of two paths:

- **the secret file:** `superuser.password` under `ASSAY_PG_SECRETS_DIR` belongs to the same OS
  user, and the port is published on loopback, so a cell can read it and connect as `postgres`;
- **the Docker daemon:** any process that can reach the daemon gets an in-container superuser
  session through `docker compose exec` — the path `provision-cell.sh` itself uses.

So on a shared OS user the boundary does not hold against a same-user cell at all: such a cell can
become the administrator this profile trusts, and every control here that guards against a cell
(the restore role, `NOLOGIN`, `REVOKE CONNECT`) is moot for it.

Separating cells needs a mechanism this profile does not ship. Whatever it is must withhold, from
every cell, **all three** of: other cells' credential and backup paths; the superuser secret
directory; and Docker daemon access (no membership in the `docker` group, no reach to the daemon
socket, no rootless daemon under the cell's user). Candidates:

- a separate OS user per cell that meets all three;
- a harness sandbox that denies all three;
- a separate server per cell.

A per-cell OS user that is in the `docker` group, or a sandbox that still allows the secret
directory or the daemon socket, **does not qualify**. Nothing in this profile, its check rows or
its drill claims that cells are separated from each other beyond the database layer.
