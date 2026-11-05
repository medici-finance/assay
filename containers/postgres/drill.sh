#!/usr/bin/env bash
# drill.sh — the restore drill (docs/postgres.md §Drill). Brings up two THROWAWAY compose projects
# (unique names, fresh volumes, free loopback ports), provisions two cells, checks the
# database-layer controls between their roles, plants an escalation and persistence probe, backs
# up, restores into the second server, checks what the restore left behind, and tears everything
# down on every exit path. It never names, starts, stops or reads the operator's assay-postgres
# project or its volume.
#
# A pass shows the DATABASE layer holds between cell roles. It says nothing about the credential
# layer: host cells on one OS user are not separated there (docs/postgres.md §"Isolation boundary").
set -euo pipefail

PG_PROG=drill.sh
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd -P)/lib.sh"

usage() {
	cat <<'EOF'
Usage: drill.sh [--keep-going]

Runs the restore drill on two throwaway compose projects named assay-pg-drill-<id>-a and
assay-pg-drill-<id>-b, each with its own fresh volume and a free 127.0.0.1 port, and removes both
(down -v) on every exit path. Cells used: example-cell-a, example-cell-b, example-cell-c.

Checks, each printed as `ok` or `FAIL`; exit 0 only when every one is ok:
  - two cells provisioned; cell A's role creates a schema in its own database;
  - cell A's role cannot connect to cell B's database, nor to postgres (and, should the first
    connect succeed, that it is still refused SELECT on cell B's table);
  - neither cell role holds SUPERUSER, CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS;
  - a wrong password is refused;
  - a provisioning run forced to fail at its role statement leaves neither that cell's password
    nor any SCRAM verifier in `docker compose logs`;
  - after backup.sh and restore.sh into the second server, with an escalation and persistence
    probe planted in cell A: no role but postgres has SUPERUSER, assay_restore is gone and owns
    nothing, the probe's password does not log in, the marker table is empty, the teardown ran
    steps 1-4 in order, the role is NOLOGIN until --enable-login, check-isolation.sh passes and
    the restored data equals the dump's;
  - no drill volume is left after teardown.

Needs: docker with compose and a running daemon, python3, awk. Pulls the pinned image when absent.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument '$1'" 2 ;;
	esac
done
need_tools docker python3 awk sort
docker info >/dev/null 2>&1 || die "the Docker daemon is not reachable — the drill needs it (could-not-check, not a pass)"

id="$(date -u +%Y%m%d%H%M%S)-$$"
prefix="assay-pg-drill-$id"
proj_a="$prefix-a" proj_b="$prefix-b"
check_project "$proj_a"
check_project "$proj_b"

umask 077
work=$(mktemp -d)
export ASSAY_PG_SECRETS_DIR="$work/secrets"
mkdir "$ASSAY_PG_SECRETS_DIR"
random_password >"$ASSAY_PG_SECRETS_DIR/superuser.password"
mkdir "$work/cells" "$work/backups"

free_port() {
	python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}
port_a=$(free_port)
port_b=$(free_port)
while [ "$port_b" = "$port_a" ]; do port_b=$(free_port); done

fail=0
ok() { printf 'ok    %s\n' "$*"; }
bad() { printf 'FAIL  %s\n' "$*"; fail=1; }
step() { printf '\n== %s\n' "$*"; }

started=""
# on <a|b> — point lib.sh's helpers (and the scripts' --project) at one drill server.
on() {
	case "$1" in
	a) PG_PROJECT=$proj_a ASSAY_PG_HOST_PORT=$port_a ;;
	b) PG_PROJECT=$proj_b ASSAY_PG_HOST_PORT=$port_b ;;
	esac
	export ASSAY_PG_HOST_PORT
}

# shellcheck disable=SC2329 # invoked by trap EXIT
finish() {
	local rc=$? p left
	set +e
	step "teardown"
	for p in $started; do
		docker compose -p "$p" -f "$PG_COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 ||
			{ printf 'FAIL  docker compose down -v for %s\n' "$p"; rc=1; }
	done
	left=$(docker volume ls -q --filter "name=$prefix")
	if [ -z "$left" ]; then
		printf 'ok    no drill volume left (docker volume ls --filter name=%s is empty)\n' "$prefix"
	else
		printf 'FAIL  drill volume(s) left behind: %s\n' "$left"
		rc=1
	fi
	rm -rf -- "$work"
	if [ "$rc" = 0 ]; then
		printf '\n%s: every check passed (database layer only — docs/postgres.md §"Isolation boundary")\n' "$PG_PROG"
	else
		printf '\n%s: FAILED (exit %s)\n' "$PG_PROG" "$rc" >&2
	fi
	exit "$rc"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

up() {
	on "$1"
	started="$started $PG_PROJECT"
	pg_compose up -d --wait --wait-timeout 180 >/dev/null || die "could not start drill project $PG_PROJECT"
	say "started $PG_PROJECT on 127.0.0.1:$ASSAY_PG_HOST_PORT"
}

# as_role <pgpass in container> <user> <db> — psql over TCP inside the current drill server's
# container, authenticated from a pgpass file; SQL on stdin, unaligned output, errors on stderr.
as_role() {
	pg_compose exec -T -u postgres -e PGPASSFILE="$1" postgres \
		psql -X -q -At -v ON_ERROR_STOP=1 --host=127.0.0.1 --port=5432 --username="$2" --no-password --dbname="$3"
}

# pgpass_into <host passfile> <container path> [password file] — an in-container pgpass line for
# the passfile's user (any database), with the passfile's password or the given file's.
pgpass_into() {
	if [ $# -ge 3 ]; then
		awk -F: 'NR == FNR { pw = $0; next } !/^[[:space:]]*(#|$)/ { print "127.0.0.1:5432:*:" $4 ":" pw }' "$3" "$1"
	else
		awk -F: '!/^[[:space:]]*(#|$)/ { print "127.0.0.1:5432:*:" $4 ":" $5 }' "$1"
	fi | into_container "$2"
}

cell_a=example-cell-a cell_b=example-cell-b cell_c=example-cell-c
db_a=$(cell_db_name "$cell_a")
db_b=$(cell_db_name "$cell_b")
pass_a="$work/cells/a.pgpass" pass_b="$work/cells/b.pgpass" pass_c="$work/cells/c.pgpass"

# ---------------------------------------------------------------------------------------------
step "server A: provision two cells"
up a
"$PG_DIR/provision-cell.sh" --project "$PG_PROJECT" --cell "$cell_a" --passfile "$pass_a" --generate >/dev/null
"$PG_DIR/provision-cell.sh" --project "$PG_PROJECT" --cell "$cell_b" --passfile "$pass_b" --generate >/dev/null
n=$(printf "SELECT count(*) FROM pg_roles WHERE rolname IN ('%s', '%s') AND rolcanlogin;\n" "$db_a" "$db_b" | pg_value -d postgres)
if [ "$n" = 2 ]; then ok "two cells provisioned ($db_a, $db_b; both LOGIN)"; else bad "two cells provisioned: found $n LOGIN cell roles"; fi

ctmp_a=$(container_tmpdir drill)
pgpass_into "$pass_a" "$ctmp_a/a.pgpass"
pgpass_into "$pass_b" "$ctmp_a/b.pgpass"

# The probe's password and a wrong password: random per run, in 0600 files, never committed.
random_password >"$work/probe.password"
random_password >"$work/wrong.password"
pgpass_into "$pass_a" "$ctmp_a/wrong.pgpass" "$work/wrong.password"
probe_pw=$(<"$work/probe.password")

step "server A: cell A's role works in its own database"
if as_role "$ctmp_a/a.pgpass" "$db_a" "$db_a" >/dev/null <<SQL
CREATE SCHEMA app;
CREATE TABLE app.data (id int PRIMARY KEY, note text NOT NULL);
INSERT INTO app.data SELECT g, 'row ' || g FROM generate_series(1, 50) g;
CREATE TABLE app.marker (what text);
CREATE FUNCTION app.probe() RETURNS boolean LANGUAGE plpgsql SECURITY INVOKER AS \$probe\$
DECLARE cell text := '$db_a';
BEGIN
  IF session_user = cell THEN RETURN true; END IF;
  BEGIN EXECUTE 'GRANT SELECT ON app.data TO assay_restore'; EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN EXECUTE 'GRANT CREATE ON SCHEMA app TO assay_restore'; EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN RESET ROLE; EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN EXECUTE format('ALTER ROLE %I SUPERUSER', cell); EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN CREATE TABLE IF NOT EXISTS app.escaped (x int); EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN EXECUTE 'ALTER ROLE CURRENT_USER PASSWORD ' || quote_literal('$probe_pw'); EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN INSERT INTO app.marker VALUES ('probe escaped as ' || current_user); EXCEPTION WHEN OTHERS THEN NULL; END;
  BEGIN EXECUTE format('SET ROLE %I', cell); EXCEPTION WHEN OTHERS THEN NULL; END;
  RETURN true;
END
\$probe\$;
CREATE TABLE app.probed (id int PRIMARY KEY, CONSTRAINT probe CHECK (app.probe()));
INSERT INTO app.probed SELECT generate_series(1, 3);
SQL
then
	ok "cell A's role created schema app (tables, the probe function and its CHECK table) in $db_a"
else
	bad "cell A's role could not create a schema in its own database"
fi
unset probe_pw
# shellcheck disable=SC2016 # $$ is SQL dollar quoting, not a shell expansion.
printf 'CREATE SCHEMA app; CREATE TABLE app.secret (v text); INSERT INTO app.secret VALUES ($$cell b row$$);\n' |
	as_role "$ctmp_a/b.pgpass" "$db_b" "$db_b" >/dev/null || bad "cell B's role could not create its table"

step "server A: database-layer checks between the two cell roles"
if out=$(printf 'SELECT current_user;\n' | as_role "$ctmp_a/a.pgpass" "$db_a" "$db_b" 2>&1); then
	bad "cell A's role CONNECTED to cell B's database $db_b (got: $out)"
	if out=$(printf 'SELECT count(*) FROM app.secret;\n' | as_role "$ctmp_a/a.pgpass" "$db_a" "$db_b" 2>&1); then
		bad "connected to $db_b, cell A's role READ cell B's table app.secret (got: $out)"
	else
		ok "connected to $db_b, cell A's role is still refused SELECT on cell B's table (ownership layer): $(printf '%s' "$out" | tail -n 1)"
	fi
else
	ok "cell A's role cannot connect to cell B's database: $(printf '%s' "$out" | tail -n 1)"
fi
if out=$(printf 'SELECT 1;\n' | as_role "$ctmp_a/a.pgpass" "$db_a" postgres 2>&1); then
	bad "cell A's role CONNECTED to the postgres database"
else
	ok "cell A's role cannot connect to postgres: $(printf '%s' "$out" | tail -n 1)"
fi
attrs=$(printf "SELECT rolname || ': ' || concat_ws(' ', CASE WHEN rolsuper THEN 'SUPERUSER' END, CASE WHEN rolcreatedb THEN 'CREATEDB' END, CASE WHEN rolcreaterole THEN 'CREATEROLE' END, CASE WHEN rolreplication THEN 'REPLICATION' END, CASE WHEN rolbypassrls THEN 'BYPASSRLS' END) FROM pg_roles WHERE rolname IN ('%s', '%s') AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls);\n" "$db_a" "$db_b" | pg_value -d postgres)
if [ -z "$attrs" ]; then ok "neither cell role has SUPERUSER, CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS"; else bad "forbidden role attributes: $attrs"; fi
if out=$(printf 'SELECT 1;\n' | as_role "$ctmp_a/wrong.pgpass" "$db_a" "$db_a" 2>&1); then
	bad "a wrong password for $db_a was ACCEPTED"
else
	ok "a wrong password is refused: $(printf '%s' "$out" | tail -n 1)"
fi
if "$PG_DIR/check-isolation.sh" --project "$PG_PROJECT" >"$work/iso-a.out" 2>&1; then
	ok "check-isolation.sh passes on server A"
else
	bad "check-isolation.sh fails on server A:"
	sed 's/^/        /' "$work/iso-a.out"
fi

step "server A: a provisioning run forced to fail at the role statement"
# default_transaction_read_only on the maintenance database makes CREATE ROLE the first statement
# of provision-cell.sh that fails; the SET and SELECT lines before it still run.
printf 'ALTER DATABASE postgres SET default_transaction_read_only = on;\n' | pg_super -d postgres
if "$PG_DIR/provision-cell.sh" --project "$PG_PROJECT" --cell "$cell_c" --passfile "$pass_c" --generate >"$work/c.out" 2>&1; then
	bad "the forced provisioning run of $cell_c SUCCEEDED — the log check below proves nothing"
else
	ok "the forced provisioning run of $cell_c failed, as intended"
fi
printf 'SET default_transaction_read_only = off;\nALTER DATABASE postgres RESET default_transaction_read_only;\n' | pg_super -d postgres
pg_compose logs --no-color postgres >"$work/logs-a.txt" 2>&1
if grep -q -F 'cannot execute CREATE ROLE in a read-only transaction' "$work/logs-a.txt"; then
	ok "the server log shows the failure was at the role statement"
else
	bad "the server log does not show a CREATE ROLE failure — the forced run failed somewhere else"
fi
# The values: the cell's password, and the verifier prefix (the salt is random, so any verifier at
# all counts). grep reads them from a file: they are never on an argv.
{
	awk -F: '!/^[[:space:]]*(#|$)/ { print $5 }' "$pass_c"
	printf '%s\n' 'SCRAM-SHA-256$'
} >"$work/c.values"
hits=$(grep -c -F -f "$work/c.values" "$work/logs-a.txt" || true)
if [ "$hits" = 0 ]; then ok "docker compose logs holds neither $cell_c's password nor any SCRAM verifier (grep -c -F -f: 0)"; else bad "docker compose logs holds $cell_c's password or a SCRAM verifier on $hits line(s)"; fi

step "server A: backup"
"$PG_DIR/backup.sh" --project "$PG_PROJECT" --out "$work/backups" || die "backup.sh failed"
dump=$(find "$work/backups" -name "$db_a.dump" -type f | head -n 1)
[ -n "$dump" ] || die "backup.sh wrote no $db_a.dump"

# ---------------------------------------------------------------------------------------------
step "server B: restore cell A"
up b
"$PG_DIR/provision-cell.sh" --project "$PG_PROJECT" --cell "$cell_a" --passfile "$pass_a" >/dev/null
rc=0
"$PG_DIR/restore.sh" --project "$PG_PROJECT" --cell "$cell_a" --dump "$dump" >"$work/restore.out" 2>&1 || rc=$?
sed 's/^/  | /' "$work/restore.out"
if [ "$rc" = 0 ]; then ok "restore.sh exited 0"; else bad "restore.sh exited $rc"; fi

ctmp_b=$(container_tmpdir drill)
pgpass_into "$pass_a" "$ctmp_b/a.pgpass"
printf '127.0.0.1:5432:*:assay_restore:' | cat - "$work/probe.password" | into_container "$ctmp_b/probe.pgpass"

supers=$(printf "SELECT string_agg(rolname, ' ') FROM pg_roles WHERE rolsuper AND rolname <> 'postgres';\n" | pg_value -d postgres)
if [ -z "$supers" ]; then ok "no role other than postgres has rolsuper"; else bad "role(s) with rolsuper besides postgres: $supers"; fi
left=$(printf "SELECT (SELECT count(*) FROM pg_roles WHERE rolname = 'assay_restore') || ' ' || (SELECT count(*) FROM pg_shdepend s JOIN pg_authid a ON a.oid = s.refobjid AND s.refclassid = 'pg_authid'::regclass WHERE a.rolname = 'assay_restore');\n" | pg_value -d postgres)
if [ "$left" = "0 0" ]; then ok "assay_restore does not exist in pg_roles and owns nothing (pg_shdepend)"; else bad "assay_restore left behind (roles, shared dependencies): $left"; fi
if out=$(printf 'SELECT 1;\n' | as_role "$ctmp_b/probe.pgpass" assay_restore "$db_a" 2>&1); then
	bad "a login as assay_restore with the probe's password SUCCEEDED"
else
	ok "a login as assay_restore with the probe's password is refused: $(printf '%s' "$out" | tail -n 1)"
fi
markers=$(printf 'SELECT count(*) FROM app.marker;\n' | pg_value -d "$db_a" || echo "query failed")
if [ "$markers" = 0 ]; then ok "the marker table app.marker holds no marker"; else bad "app.marker holds: $markers"; fi
order=$(sed -n -E 's/^restore\.sh: teardown step ([1-4]): .*/\1/p' "$work/restore.out" | awk '!seen[$0]++' | tr -d '\n')
if [ "$order" = 1234 ] && ! grep -q -E 'teardown step [1-4]: .* failed$' "$work/restore.out"; then
	ok "the restore's teardown ran steps 1-4 in order, none failed"
else
	bad "the restore's teardown log shows steps '$order'$(grep -E 'teardown step [1-4]: .* failed$' "$work/restore.out" | sed 's/^/; /' | tr -d '\n')"
fi
login=$(printf "SELECT rolcanlogin FROM pg_roles WHERE rolname = '%s';\n" "$db_a" | pg_value -d postgres)
if [ "$login" = f ] && ! printf 'SELECT 1;\n' | as_role "$ctmp_b/a.pgpass" "$db_a" "$db_a" >/dev/null 2>&1; then
	ok "after the restore $db_a is NOLOGIN and its login is refused"
else
	bad "after the restore $db_a is not NOLOGIN (rolcanlogin=$login)"
fi
"$PG_DIR/provision-cell.sh" --project "$PG_PROJECT" --cell "$cell_a" --enable-login >/dev/null
who=$(printf 'SELECT current_user;\n' | as_role "$ctmp_b/a.pgpass" "$db_a" "$db_a" 2>&1 || true)
if [ "$who" = "$db_a" ]; then ok "after --enable-login, select current_user prints $who"; else bad "after --enable-login the login gave: $who"; fi
if "$PG_DIR/check-isolation.sh" --project "$PG_PROJECT" >"$work/iso-b.out" 2>&1; then
	ok "check-isolation.sh passes on server B after the restore"
else
	bad "check-isolation.sh fails on server B:"
	sed 's/^/        /' "$work/iso-b.out"
fi

step "server B: the restored data equals the dump's"
# Data-only text of both sides, comments, \restrict lines and blanks dropped, sorted. app.escaped
# is the probe's table (created during the load, if at all), not data from the dump.
into_container "$ctmp_b/cell.dump" <"$dump"
norm() { grep -v -E '^(--|\\restrict |\\unrestrict |$)' | sort; }
pg_compose exec -T -u postgres postgres pg_restore -a -f - "$ctmp_b/cell.dump" | norm >"$work/want.txt"
pg_compose exec -T -u postgres postgres pg_dump -a --no-password --exclude-table=app.escaped -d "$db_a" | norm >"$work/got.txt"
if [ -s "$work/want.txt" ] && cmp -s "$work/want.txt" "$work/got.txt"; then
	ok "restored data matches the dump ($(grep -c . "$work/want.txt") sorted data-only lines)"
else
	bad "restored data differs from the dump:"
	diff "$work/want.txt" "$work/got.txt" | head -n 20 | sed 's/^/        /'
fi

exit "$fail"
