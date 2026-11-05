#!/usr/bin/env bash
# provision-cell.sh — one database and one restricted role per cell, on the live server
# (docs/postgres.md §"Provision a cell"). Idempotent: run it again and it converges.
set -euo pipefail

PG_PROG=provision-cell.sh
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd -P)/lib.sh"

usage() {
	cat <<'EOF'
Usage:
  provision-cell.sh --cell <name> --passfile <abs path> [--generate] [--connection-limit <n>] [--project <p>]
  provision-cell.sh --cell <name> --disable-login [--project <p>]
  provision-cell.sh --cell <name> --enable-login  [--project <p>]

Creates (or converges) the cell's database and role on the running server of compose project
<p> (default assay-postgres). The name of both is `cell_` plus the cell name lowercased, with
each '-', '/' and '.' replaced by '_' (example-cell -> cell_example_cell).

  --passfile <path>      the cell's libpq pgpass file: ONE line host:port:database:user:password,
                         mode 0600 or 0400, password of 32 or more [A-Za-z0-9_-]. Absolute path.
  --generate             create the passfile when it does not exist: mode 0600, 32 random bytes
                         base64url-encoded, host 127.0.0.1, port ${ASSAY_PG_HOST_PORT:-5432}.
                         An existing file is used unchanged.
  --connection-limit <n> the role's CONNECTION LIMIT (default 20).
  --disable-login        set the role NOLOGIN (nothing else changes).
  --enable-login         set the role LOGIN — after a restore, once the consumer's reconcile
                         procedure has run (docs/postgres.md §Restore). The two login flags
                         are mutually exclusive.
  --project <p>          the compose project (default assay-postgres).

The role is LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS and owns its
database; PUBLIC loses CONNECT and TEMPORARY on it. An existing role keeps its LOGIN/NOLOGIN state:
only --enable-login turns a login back on.

The password never reaches the server: a SCRAM-SHA-256 verifier is computed on this host
(scram-verifier.py, after an RFC 7677 self-test) and only that is sent, on stdin, in a session
that first switches off statement logging. The password is never on an argv or in an environment
variable.

Needs: docker compose, python3, ASSAY_PG_SECRETS_DIR set (as for starting the server).
Prints the DSN and the two `cellctl set` lines on success.
EOF
}

cell="" passfile="" generate=0 limit=20 login_action=""
while [ $# -gt 0 ]; do
	case "$1" in
	--cell) cell=${2:?--cell needs a value}; shift 2 ;;
	--passfile) passfile=${2:?--passfile needs a value}; shift 2 ;;
	--generate) generate=1; shift ;;
	--connection-limit) limit=${2:?--connection-limit needs a value}; shift 2 ;;
	--disable-login | --enable-login)
		# One login action per run: the pair is refused, never resolved by whichever came last.
		want=${1#--}; want=${want%-login}
		{ [ -z "$login_action" ] || [ "$login_action" = "$want" ]; } || { usage >&2; die "--disable-login and --enable-login are mutually exclusive" 2; }
		login_action=$want; shift ;;
	--project) PG_PROJECT=${2:?--project needs a value}; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument '$1'" 2 ;;
	esac
done

[ -n "$cell" ] || { usage >&2; die "--cell is required" 2; }
check_project "$PG_PROJECT"
db=$(cell_db_name "$cell")
need_tools docker
need_secrets_dir

if [ -n "$login_action" ]; then
	[ -z "$passfile" ] && [ "$generate" = 0 ] || die "--$login_action-login takes no --passfile or --generate" 2
	require_running
	exists=$(printf "SELECT count(*) FROM pg_roles WHERE rolname = '%s';\n" "$db" | pg_value -d postgres)
	[ "$exists" = 1 ] || die "role $db does not exist — provision the cell first"
	if [ "$login_action" = enable ]; then
		running=$(printf "SELECT count(*) FROM pg_roles WHERE rolname = 'assay_restore';\n" | pg_value -d postgres)
		[ "$running" = 0 ] ||
			die "role assay_restore exists: a restore is running or its teardown failed — resolve that first (docs/postgres.md §Restore)"
		printf 'ALTER ROLE "%s" LOGIN;\n' "$db" | pg_super -d postgres
		say "cell '$cell': role $db is LOGIN"
	else
		printf 'ALTER ROLE "%s" NOLOGIN;\n' "$db" | pg_super -d postgres
		say "cell '$cell': role $db is NOLOGIN"
	fi
	exit 0
fi

[ -n "$passfile" ] || { usage >&2; die "--passfile is required" 2; }
case "$passfile" in
/*) ;;
*) die "--passfile '$passfile' is not an absolute path" 2 ;;
esac
[[ "$limit" =~ ^[0-9]+$ ]] && [ "$limit" -ge 1 ] || die "--connection-limit '$limit' is not a positive integer" 2
need_tools python3 awk

if [ ! -e "$passfile" ]; then
	[ "$generate" = 1 ] || die "passfile '$passfile' does not exist — create it (docs/postgres.md §Credentials) or pass --generate"
	[ -d "$(dirname "$passfile")" ] || die "the directory of '$passfile' does not exist"
	# noclobber: a file that appears between the test and the write is never overwritten.
	(
		umask 077
		set -o noclobber
		{
			printf '127.0.0.1:%s:%s:%s:' "${ASSAY_PG_HOST_PORT:-5432}" "$db" "$db"
			random_password
		} >"$passfile"
	) || die "could not create passfile '$passfile'"
	say "generated passfile $passfile (mode 0600)"
fi
require_secret_mode "$passfile" "passfile"

# One entry, for this cell's database (or '*') and role, with a password in the safe set. The
# check prints a reason, never a field value.
bad=$(awk -F: -v db="$db" '
	/^[[:space:]]*(#|$)/ { next }
	{
		n++
		if (NF != 5) bad = "a line that is not host:port:database:user:password"
		else if ($3 != db && $3 != "*") bad = "the database field is not " db
		else if ($4 != db) bad = "the user field is not " db
		else if ($5 !~ /^[A-Za-z0-9_-]+$/ || length($5) < 32) bad = "a password outside [A-Za-z0-9_-]{32,}"
	}
	END {
		if (n != 1) print "it must hold exactly one entry (found " n + 0 ")"
		else if (bad != "") print bad
	}' "$passfile")
[ -z "$bad" ] || die "passfile '$passfile': $bad"

scram_selftest
require_running

# Preflight, before anything changes: a database of this name must be this cell's (its comment
# names the cell), so two cell names that map to one identifier are refused.
want=$(cell_comment "$cell")
have=$(printf "SELECT 'db:' || coalesce(shobj_description(oid, 'pg_database'), '') FROM pg_database WHERE datname = '%s';\n" "$db" | pg_value -d postgres)
case "$have" in
"") ;; # no database yet
"db:$want") ;;
"db:") die "database $db exists with no cell comment — it is not a provisioned cell; refusing" ;;
*) die "database $db belongs to another cell (${have#db:}); cell '$cell' maps to the same name — refusing" ;;
esac

# The verifier is computed here; only it is sent. password_sql is the one value that reaches the
# server as the role's PASSWORD.
password_sql=$(scram_verifier_from_passfile "$passfile")

attrs="NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"
# printf is a shell builtin: the verifier is written to the pipe, never to an argv.
{
	printf '%s\n' 'SET log_min_error_statement = panic;'
	printf '%s\n' 'SET log_statement = none;'
	printf '%s\n' 'SET pg_stat_statements.track_utility = off;'
	printf "SELECT NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') AS create_role %s\n" "$db" '\gset'
	printf '%s\n' '\if :create_role'
	printf "CREATE ROLE \"%s\" LOGIN %s CONNECTION LIMIT %s PASSWORD '%s';\n" "$db" "$attrs" "$limit" "$password_sql"
	printf '%s\n' '\else'
	printf "ALTER ROLE \"%s\" %s CONNECTION LIMIT %s PASSWORD '%s';\n" "$db" "$attrs" "$limit" "$password_sql"
	printf '%s\n' '\endif'
	printf "SELECT NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s') AS create_db %s\n" "$db" '\gset'
	printf '%s\n' '\if :create_db'
	printf 'CREATE DATABASE "%s" OWNER "%s";\n' "$db" "$db"
	printf '%s\n' '\endif'
	printf 'ALTER DATABASE "%s" OWNER TO "%s";\n' "$db" "$db"
	printf 'REVOKE CONNECT, TEMPORARY ON DATABASE "%s" FROM PUBLIC;\n' "$db"
	printf "COMMENT ON DATABASE \"%s\" IS '%s';\n" "$db" "$want"
} | pg_super -d postgres || die "provisioning $db failed (the server log holds no statement text from this session)"
unset password_sql

login=$(printf "SELECT CASE WHEN rolcanlogin THEN 'LOGIN' ELSE 'NOLOGIN' END FROM pg_roles WHERE rolname = '%s';\n" "$db" | pg_value -d postgres)

# The DSN names the passfile's host and port (a '*' falls back to the published loopback port).
host=$(awk -F: '!/^[[:space:]]*(#|$)/ { print $1 }' "$passfile")
port=$(awk -F: '!/^[[:space:]]*(#|$)/ { print $2 }' "$passfile")
[ "$host" != "*" ] || host=127.0.0.1
[ "$port" != "*" ] || port=${ASSAY_PG_HOST_PORT:-5432}
dsn="host=$host port=$port dbname=$db user=$db sslmode=disable"

say "cell '$cell': database $db, role $db ($login, connection limit $limit); PUBLIC holds no CONNECT on $db"
[ "$login" = LOGIN ] || say "role $db stays NOLOGIN — run '$0 --cell $cell --enable-login' when the cell may connect"
printf 'CELL_DB_DSN=%s\n' "$dsn"
printf "cellctl set %s 'CELL_DB_DSN=%s'\n" "$cell" "$dsn"
printf "cellctl set %s 'CELL_DB_PASSFILE=%s'\n" "$cell" "$passfile"
