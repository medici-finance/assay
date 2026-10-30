#!/usr/bin/env bash
# check-isolation.sh — detect drift in the database-layer controls between cell roles
# (docs/postgres.md §"Isolation boundary"). Read-only: it never changes the server.
#
# What a pass means, and no more: at the DATABASE layer, PUBLIC holds no CONNECT or TEMPORARY on
# any cell, aside or maintenance database, no cell role holds an attribute it must not, and no
# restore role is left behind. It says nothing about the credential layer: host cells on one OS
# user can read each other's credential files and reach the superuser (docs/postgres.md).
set -euo pipefail

PG_PROG=check-isolation.sh
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd -P)/lib.sh"

usage() {
	cat <<'EOF'
Usage: check-isolation.sh [--project <p>]

Checks the running server of compose project <p> (default assay-postgres) and exits non-zero
when any of these fails:
  - PUBLIC holds CONNECT or TEMPORARY on a cell_% or aside_% database, or on postgres or template1;
  - a cell_% database is not owned by the role of the same name;
  - a cell_% role has SUPERUSER, CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS;
  - a role other than postgres has SUPERUSER;
  - the restore role assay_restore exists (outside a running restore that means a failed teardown).

Read-only. Needs: docker compose, ASSAY_PG_SECRETS_DIR set (as for starting the server).
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--project) PG_PROJECT=${2:?--project needs a value}; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument '$1'" 2 ;;
	esac
done
check_project "$PG_PROJECT"
need_tools docker
need_secrets_dir
require_running

fail=0
# check <description> <SQL returning offending rows, one per line> — a pass prints nothing found.
check() {
	local what=$1 rows
	rows=$(printf '%s\n' "$2" | pg_value -d postgres) || { say "FAIL  $what: the query failed"; fail=1; return; }
	if [ -z "$rows" ]; then
		say "ok    $what"
	else
		say "FAIL  $what:"
		printf '%s\n' "$rows" | sed 's/^/        /'
		fail=1
	fi
}

check "PUBLIC holds no CONNECT/TEMPORARY on cell_%, aside_%, postgres or template1" \
	"SELECT datname || ': PUBLIC holds ' ||
	        concat_ws(', ', CASE WHEN has_database_privilege('public', oid, 'CONNECT') THEN 'CONNECT' END,
	                        CASE WHEN has_database_privilege('public', oid, 'TEMPORARY') THEN 'TEMPORARY' END)
	   FROM pg_database
	  WHERE (datname LIKE 'cell\\_%' OR datname LIKE 'aside\\_%' OR datname IN ('postgres', 'template1'))
	    AND (has_database_privilege('public', oid, 'CONNECT') OR has_database_privilege('public', oid, 'TEMPORARY'))
	  ORDER BY 1;"

check "every cell_% database is owned by its own role" \
	"SELECT d.datname || ': owned by ' || pg_get_userbyid(d.datdba)
	   FROM pg_database d
	  WHERE d.datname LIKE 'cell\\_%' AND pg_get_userbyid(d.datdba) <> d.datname
	  ORDER BY 1;"

check "no cell_% role has SUPERUSER, CREATEDB, CREATEROLE, REPLICATION or BYPASSRLS" \
	"SELECT rolname || ': ' || concat_ws(' ', CASE WHEN rolsuper THEN 'SUPERUSER' END,
	                                          CASE WHEN rolcreatedb THEN 'CREATEDB' END,
	                                          CASE WHEN rolcreaterole THEN 'CREATEROLE' END,
	                                          CASE WHEN rolreplication THEN 'REPLICATION' END,
	                                          CASE WHEN rolbypassrls THEN 'BYPASSRLS' END)
	   FROM pg_roles
	  WHERE rolname LIKE 'cell\\_%'
	    AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
	  ORDER BY 1;"

check "no role other than postgres has SUPERUSER" \
	"SELECT rolname FROM pg_roles WHERE rolsuper AND rolname <> 'postgres' ORDER BY 1;"

check "the restore role assay_restore does not exist" \
	"SELECT 'assay_restore exists (' ||
	        (SELECT count(*) FROM pg_shdepend WHERE refclassid = 'pg_authid'::regclass AND refobjid = r.oid) ||
	        ' shared dependencies) — a restore is running, or its teardown failed'
	   FROM pg_roles r WHERE r.rolname = 'assay_restore';"

if [ "$fail" = 0 ]; then
	say "database-layer controls hold (this does not separate host cells on one OS user — docs/postgres.md)"
fi
exit "$fail"
