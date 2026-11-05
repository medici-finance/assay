# shellcheck shell=bash
# lib.sh — shared helpers for the containers/postgres scripts (docs/postgres.md). Sourced, never
# run. Every helper refuses rather than guesses, and none puts a password on an argv or in an
# environment variable: secrets travel on stdin and in mode-0600 files only.

PG_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
PG_COMPOSE_FILE="$PG_DIR/compose.yaml"
PG_SCRAM="$PG_DIR/scram-verifier.py"
# The operator's compose project. Every script takes --project to point elsewhere (the drill
# always does); nothing here ever starts or stops a project except drill.sh's own.
PG_PROJECT=assay-postgres
PG_PROG=${PG_PROG:-postgres}

die() {
	printf '%s: %s\n' "$PG_PROG" "$1" >&2
	exit "${2:-1}"
}

say() {
	printf '%s: %s\n' "$PG_PROG" "$*"
}

# need_tools: refuse when a client tool the script relies on is missing.
need_tools() {
	local t
	for t in "$@"; do
		command -v "$t" >/dev/null 2>&1 || die "'$t' not found on PATH — install it first"
	done
}

# need_secrets_dir: compose resolves the file (exec included) only with ASSAY_PG_SECRETS_DIR set.
need_secrets_dir() {
	[ -n "${ASSAY_PG_SECRETS_DIR:-}" ] ||
		die "ASSAY_PG_SECRETS_DIR is unset — set it to the directory holding superuser.password, as for 'docker compose up' (docs/postgres.md)"
}

check_project() {
	[[ "$1" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die "--project '$1' is not a compose project name ([a-z0-9][a-z0-9_-]*)"
}

pg_compose() {
	docker compose -p "$PG_PROJECT" -f "$PG_COMPOSE_FILE" "$@"
}

# pg_super [psql args...] — a superuser psql INSIDE the container, over its local socket as the OS
# user postgres (peer authentication; compose.yaml's initdb arguments). SQL arrives on stdin.
pg_super() {
	pg_compose exec -T -u postgres postgres psql -X -q --no-password -v ON_ERROR_STOP=1 "$@"
}

# pg_value [psql args...] — one unaligned, tuples-only result from SQL on stdin.
pg_value() {
	pg_super -At "$@"
}

# require_running: the server is up and answering. Never starts it.
require_running() {
	printf 'SELECT 1;\n' | pg_value -d postgres >/dev/null 2>&1 ||
		die "cannot reach the server of compose project '$PG_PROJECT' — start it first (docs/postgres.md §Start and stop)"
}

# cell_db_name <cell> — the database and role name for a cell, or a refusal: `cell_` plus the cell
# name lowercased, with each '-', '/' and '.' replaced by '_'.
cell_db_name() {
	local cell=$1 mapped
	[[ "$cell" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ ]] ||
		die "cell name '$cell' is outside ^[A-Za-z0-9][A-Za-z0-9._/-]*\$"
	mapped=$(printf '%s' "$cell" | LC_ALL=C tr '[:upper:]' '[:lower:]' | tr -- '-/.' '___')
	mapped="cell_$mapped"
	[ "${#mapped}" -le 63 ] ||
		die "cell name '$cell' maps to '$mapped', ${#mapped} bytes — over PostgreSQL's 63-byte identifier limit"
	printf '%s\n' "$mapped"
}

# cell_comment <cell> — what COMMENT ON DATABASE records for a cell database; the collision check
# compares against it.
cell_comment() {
	printf 'assay cell: %s\n' "$1"
}

# file_mode <path> — the permission bits in octal (GNU stat, then BSD stat).
file_mode() {
	stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"
}

# require_secret_mode <path> <what> — a regular file at mode 0600 or 0400.
require_secret_mode() {
	local m
	[ -f "$1" ] || die "$2 '$1' is not a regular file"
	m=$(file_mode "$1") || die "cannot read the mode of $2 '$1'"
	case "$m" in
	600 | 400) ;;
	*) die "$2 '$1' has mode $m — it must be 0600 or 0400" ;;
	esac
}

# sha256_of <file> — the hex digest alone.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# scram_selftest: the client-side verifier must reproduce the RFC 7677 vector before any
# password is hashed with it. There is no fallback to sending plaintext.
scram_selftest() {
	need_tools python3
	python3 -I "$PG_SCRAM" selftest ||
		die "the SCRAM-SHA-256 verifier failed its RFC 7677 self-test — refusing (no plaintext fallback)"
}

# scram_verifier_from_passfile <pgpass file> — the verifier for the password on the file's one
# line, computed on this host. The password goes file -> awk -> python on pipes only.
scram_verifier_from_passfile() {
	local v
	v=$(awk -F: '!/^[[:space:]]*#/ && NF == 5 { print $5 }' "$1" | python3 -I "$PG_SCRAM") ||
		die "could not compute a SCRAM verifier from '$1' — refusing (no plaintext fallback)"
	[[ "$v" =~ ^SCRAM-SHA-256\$[0-9]+:[A-Za-z0-9+/=]+\$[A-Za-z0-9+/=]+:[A-Za-z0-9+/=]+$ ]] ||
		die "the computed verifier is not in SCRAM-SHA-256 form — refusing (no plaintext fallback)"
	printf '%s\n' "$v"
}

# random_password — 32 random bytes, base64url without padding (43 characters of [A-Za-z0-9_-]).
# Written straight to stdout for a pipe or a mode-0600 file; never an argument.
random_password() {
	head -c 32 /dev/urandom | base64 | tr -d '\n=' | tr '+/' '-_'
	printf '\n'
}

# into_container <path in container> — copy stdin to a mode-0600 file inside the container, as
# the OS user postgres. Only the path is an argument; the content travels on stdin.
into_container() {
	# shellcheck disable=SC2016 # $1 is expanded by the container's sh, on purpose.
	pg_compose exec -T -u postgres postgres sh -c 'umask 077 && cat > "$1"' sh "$1"
}

# container_tmpdir <prefix> — a fresh mktemp -d directory inside the container, owned by postgres.
container_tmpdir() {
	local d
	d=$(pg_compose exec -T -u postgres postgres mktemp -d "/tmp/$1.XXXXXXXX" | tr -d '\r')
	[[ "$d" =~ ^/tmp/[a-z-]+\.[A-Za-z0-9]+$ ]] || die "could not create a temporary directory in the container"
	printf '%s\n' "$d"
}
