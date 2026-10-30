#!/usr/bin/env bash
# restore.sh — restore ONE cell from ONE dump; the cell's role ends NOLOGIN
# (docs/postgres.md §Restore).
#
# A dump of a cell database is cell-controlled input: the cell role owns that database, so the
# dump can carry functions that run while the data loads (a CHECK constraint, a generated
# column). The data load therefore never runs as the superuser. It runs over a session that
# AUTHENTICATES as the one-shot role assay_restore — a non-inheriting member of this one cell role
# only — so cell code that escapes `SET ROLE` with `RESET ROLE` lands on a role that holds nothing
# but CONNECT on the fresh database. A trap tears assay_restore down on every exit path, NOLOGIN
# first; VALID UNTIL is the second layer.
set -euo pipefail

PG_PROG=restore.sh
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd -P)/lib.sh"

usage() {
	cat <<'EOF'
Usage: restore.sh --cell <name> --dump <file> [--deadline-minutes <n>] [--allow-extension <name>]... [--project <p>]

Restores one cell's database from one `pg_dump -Fc` file (backup.sh writes them), on the running
server of compose project <p> (default assay-postgres). Other cells are not touched.

Order:
  1. the cell's role is set NOLOGIN — before anything else, and it STAYS NOLOGIN;
  2. refuses while any session of that role is open: stop the cell first (`cellctl down <cell>`);
  3. the current database is renamed aside to aside_<YYYYMMDDTHHMMSSZ>_<8 hex of sha256(db name)>
     (never dropped; not a cell_% name, so backup.sh skips it);
  4. a fresh database with the same owner, the same REVOKE ... FROM PUBLIC and the cell comment;
  5. each EXTENSION in the dump is created by NAME as the superuser (the image ships its scripts;
     the dump's own SQL for it is never run); refused when the image lacks it, when it is
     neither trusted nor named with --allow-extension, or when it lives in a schema the dump
     itself creates;
  6. the one-shot role assay_restore (NOINHERIT, CONNECTION LIMIT 1, VALID UNTIL now +
     --deadline-minutes, default 60) is granted the cell role WITH INHERIT FALSE, SET TRUE and
     CONNECT on the fresh database; refused if assay_restore already exists (one restore at a
     time);
  7. the data loads with pg_restore --role=<cell role> --no-owner --exit-on-error
     --single-transaction, connected AS assay_restore (TCP to 127.0.0.1 inside the container,
     SCRAM, password in a mode-0600 pgpass file in a mktemp -d directory);
  8. teardown, on every exit path, as the superuser: (1) ALTER ROLE assay_restore NOLOGIN;
     (2) terminate its sessions; (3) in the restored database REASSIGN OWNED BY assay_restore TO
     the cell role, then DROP OWNED BY assay_restore; (4) DROP ROLE assay_restore. A failed step
     is named and the restore fails, even when the data load succeeded.

Afterwards the cell cannot connect until, after the consumer's reconcile procedure:
  provision-cell.sh --cell <name> --enable-login

Needs: docker compose, python3, awk, ASSAY_PG_SECRETS_DIR set (as for starting the server).
EOF
}

cell="" dump="" deadline=60 allow=""
while [ $# -gt 0 ]; do
	case "$1" in
	--cell) cell=${2:?--cell needs a value}; shift 2 ;;
	--dump) dump=${2:?--dump needs a value}; shift 2 ;;
	--deadline-minutes) deadline=${2:?--deadline-minutes needs a value}; shift 2 ;;
	--allow-extension) allow="$allow ${2:?--allow-extension needs a value} "; shift 2 ;;
	--project) PG_PROJECT=${2:?--project needs a value}; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument '$1'" 2 ;;
	esac
done
[ -n "$cell" ] && [ -n "$dump" ] || { usage >&2; die "--cell and --dump are required" 2; }
[[ "$deadline" =~ ^[0-9]+$ ]] && [ "$deadline" -ge 1 ] || die "--deadline-minutes '$deadline' is not a positive integer" 2
[ -f "$dump" ] && [ -r "$dump" ] || die "--dump '$dump' is not a readable file"
check_project "$PG_PROJECT"
db=$(cell_db_name "$cell")
need_tools docker python3 awk
need_secrets_dir
scram_selftest
require_running

q() { pg_value -d postgres; }

[ "$(printf "SELECT count(*) FROM pg_roles WHERE rolname = '%s';\n" "$db" | q)" = 1 ] ||
	die "role $db does not exist — provision the cell first (provision-cell.sh --cell $cell ...), then restore"
[ "$(printf "SELECT count(*) FROM pg_roles WHERE rolname = 'assay_restore';\n" | q)" = 0 ] ||
	die "role assay_restore already exists: another restore is running, or one's teardown failed — refusing"

# 1. NOLOGIN before anything else.
printf 'ALTER ROLE "%s" NOLOGIN;\n' "$db" | pg_super -d postgres
say "role $db is NOLOGIN (it stays NOLOGIN until provision-cell.sh --enable-login)"

# 2. No open session of the cell may survive into the restore.
open=$(printf "SELECT count(*) FROM pg_stat_activity WHERE usename = '%s';\n" "$db" | q)
[ "$open" = 0 ] ||
	die "$open session(s) of role $db are still open — stop the cell (cellctl down $cell) and re-run; the role stays NOLOGIN"

htmp="" ctmp="" role_created=0 aside=""

teardown() {
	local failed=0
	say "teardown step 1: ALTER ROLE assay_restore NOLOGIN"
	printf 'ALTER ROLE assay_restore NOLOGIN;\n' | pg_super -d postgres ||
		{ say "teardown step 1: ALTER ROLE assay_restore NOLOGIN failed" >&2; failed=1; }
	say "teardown step 2: terminate every session of assay_restore"
	printf "SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE usename = 'assay_restore';\n" | pg_value -d postgres >/dev/null ||
		{ say "teardown step 2: pg_terminate_backend for assay_restore failed" >&2; failed=1; }
	say "teardown step 3: in $db, REASSIGN OWNED BY assay_restore TO $db; DROP OWNED BY assay_restore"
	printf 'REASSIGN OWNED BY assay_restore TO "%s";\nDROP OWNED BY assay_restore;\n' "$db" | pg_super -d "$db" ||
		{ say "teardown step 3: REASSIGN OWNED / DROP OWNED BY assay_restore failed" >&2; failed=1; }
	say "teardown step 4: DROP ROLE assay_restore"
	printf 'DROP ROLE assay_restore;\n' | pg_super -d postgres ||
		{ say "teardown step 4: DROP ROLE assay_restore failed" >&2; failed=1; }
	return "$failed"
}

finish() {
	local rc=$?
	set +e
	if [ "$role_created" = 1 ]; then
		teardown || rc=1
	fi
	if [ -n "$ctmp" ]; then
		pg_compose exec -T -u postgres postgres rm -rf -- "$ctmp" ||
			say "could not remove $ctmp inside the container" >&2
	fi
	[ -z "$htmp" ] || rm -rf -- "$htmp"
	if [ "$rc" = 0 ]; then
		say "restored cell '$cell' into $db${aside:+ (previous database kept as $aside)}"
		say "role $db is NOLOGIN: run the consumer's reconcile procedure, then: provision-cell.sh --cell $cell --enable-login"
	else
		say "restore of cell '$cell' FAILED (exit $rc); role $db stays NOLOGIN${aside:+; previous database kept as $aside}" >&2
	fi
	exit "$rc"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP

htmp=$(mktemp -d)
ctmp=$(container_tmpdir assay-restore)

into_container "$ctmp/cell.dump" <"$dump"
pg_compose exec -T -u postgres postgres pg_restore -l "$ctmp/cell.dump" >"$htmp/full.list" ||
	die "'$dump' is not a pg_dump custom-format archive this server's pg_restore reads"

# 5a. Extension entries: take the NAME only, and decide from the image, never from the dump text.
exts=$(awk '/^[0-9]+;/ && $4 == "EXTENSION" && $5 == "-" { print $6 }' "$htmp/full.list")
ext_schemas=""
for ext in $exts; do
	[[ "$ext" =~ ^[a-z0-9_]+$ ]] || die "extension name '$ext' in the dump is not [a-z0-9_]+ — refusing"
	trusted=$(printf "SELECT coalesce(bool_or(trusted)::text, 'absent') FROM pg_available_extension_versions WHERE name = '%s';\n" "$ext" | q)
	case "$trusted" in
	absent) die "the dump needs extension '$ext', which this image does not provide — refusing" ;;
	true) ;;
	*)
		case "$allow" in
		*" $ext "*) ;;
		*) die "extension '$ext' is not trusted in this image (a cell owner could not have created it) — refusing; --allow-extension $ext overrides after review" ;;
		esac
		;;
	esac
	# Which schema the entry names: read (never run) the entry's text, accept only a schema that
	# exists before any cell object does.
	awk -v e="$ext" '/^[0-9]+;/ && $4 == "EXTENSION" && $5 == "-" && $6 == e' "$htmp/full.list" | into_container "$ctmp/ext.list"
	schema=$(pg_compose exec -T -u postgres postgres pg_restore -L "$ctmp/ext.list" -f - "$ctmp/cell.dump" |
		sed -n -E 's/^CREATE EXTENSION .* WITH SCHEMA ([a-z_][a-z0-9_]*);.*$/\1/p' | head -n 1)
	case "$schema" in
	public | pg_catalog) ext_schemas="$ext_schemas $ext:$schema" ;;
	"") die "cannot tell which schema extension '$ext' lives in — refusing" ;;
	*) die "extension '$ext' lives in schema '$schema', which the dump itself creates — it cannot be created before the cell's own objects exist; refusing" ;;
	esac
done

# The data load's list: everything but the extension entries and their comments.
awk '!(/^[0-9]+;/ && (($4 == "EXTENSION" && $5 == "-") || ($4 == "COMMENT" && $5 == "-" && $6 == "EXTENSION")))' "$htmp/full.list" |
	into_container "$ctmp/load.list"

# 3. Rename the current database aside (never dropped). A second restore of this cell in the same
# second fails here, before anything else changes.
if [ "$(printf "SELECT count(*) FROM pg_database WHERE datname = '%s';\n" "$db" | q)" = 1 ]; then
	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	printf '%s' "$db" >"$htmp/dbname"
	h=$(sha256_of "$htmp/dbname" | cut -c1-8)
	target="aside_${stamp}_$h"
	printf 'ALTER DATABASE "%s" RENAME TO "%s";\n' "$db" "$target" | pg_super -d postgres ||
		die "renaming $db aside to $target failed (another session on it, or a restore of this cell in the same second) — nothing else changed"
	aside=$target
	printf "COMMENT ON DATABASE \"%s\" IS 'assay aside: cell %s, was %s, restore %s';\n" "$aside" "$cell" "$db" "$stamp" | pg_super -d postgres
	say "renamed $db aside to $aside"
else
	say "database $db does not exist — restoring into a new one"
fi

# 4. The fresh database: same owner, same REVOKE, same cell comment.
{
	printf 'CREATE DATABASE "%s" OWNER "%s";\n' "$db" "$db"
	printf 'REVOKE CONNECT, TEMPORARY ON DATABASE "%s" FROM PUBLIC;\n' "$db"
	printf "COMMENT ON DATABASE \"%s\" IS '%s';\n" "$db" "$(cell_comment "$cell")"
} | pg_super -d postgres

# 5b. Extensions, by name, as the superuser, before the cell's objects exist.
for pair in $ext_schemas; do
	printf 'CREATE EXTENSION IF NOT EXISTS "%s" SCHEMA "%s";\n' "${pair%%:*}" "${pair#*:}" | pg_super -d "$db"
	say "created extension ${pair%%:*} in $db (by name; the dump's entry was not replayed)"
done

# 6. The one-shot role. Its password exists only in mode-0600 files inside the two mktemp -d
# directories; the server receives a verifier, in a session that logs no statement text.
(
	umask 077
	{
		printf '127.0.0.1:5432:%s:assay_restore:' "$db"
		random_password
	} >"$htmp/restore.pgpass"
)
verifier=$(scram_verifier_from_passfile "$htmp/restore.pgpass")
until=$(printf "SELECT to_char((now() + interval '%s minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') || '+00';\n" "$deadline" | q)
{
	printf '%s\n' 'SET log_min_error_statement = panic;'
	printf '%s\n' 'SET log_statement = none;'
	printf '%s\n' 'SET pg_stat_statements.track_utility = off;'
	printf "CREATE ROLE assay_restore LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT CONNECTION LIMIT 1 VALID UNTIL '%s' PASSWORD '%s';\n" "$until" "$verifier"
} | pg_super -d postgres || die "creating role assay_restore failed"
unset verifier
role_created=1
{
	printf 'GRANT "%s" TO assay_restore WITH INHERIT FALSE, SET TRUE;\n' "$db"
	printf 'GRANT CONNECT ON DATABASE "%s" TO assay_restore;\n' "$db"
} | pg_super -d postgres
say "created one-shot role assay_restore (VALID UNTIL $until; member of $db WITH INHERIT FALSE, SET TRUE)"
into_container "$ctmp/restore.pgpass" <"$htmp/restore.pgpass"

# as_restore <client> [args...] — a client inside the container, authenticated as assay_restore
# over TCP with the pgpass file. Only the file's PATH is in the environment.
as_restore() {
	local client=$1
	shift
	pg_compose exec -T -u postgres -e PGPASSFILE="$ctmp/restore.pgpass" postgres \
		"$client" --host=127.0.0.1 --port=5432 --username=assay_restore --no-password "$@"
}

# Refuse rather than fall back: the session the data loads over is assay_restore, not a superuser.
who=$(printf 'SELECT session_user || chr(124) || (SELECT rolsuper::text FROM pg_roles WHERE rolname = session_user);\n' |
	as_restore psql -X -q -At -v ON_ERROR_STOP=1 --dbname="$db" | tr -d '\r') || true
[ "$who" = "assay_restore|false" ] ||
	die "the data-load session is not a non-superuser assay_restore session (got '${who:-no connection}') — refusing"

# 7. The data load.
as_restore pg_restore --dbname="$db" --role="$db" --no-owner --exit-on-error --single-transaction \
	--use-list="$ctmp/load.list" "$ctmp/cell.dump" ||
	die "pg_restore into $db failed — the load ran in one transaction, so $db holds no partial data"
say "data loaded into $db as assay_restore (SET ROLE $db)"
