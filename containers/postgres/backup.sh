#!/usr/bin/env bash
# backup.sh — per-cell logical dumps plus globals, with a manifest (docs/postgres.md §Backups).
set -euo pipefail

PG_PROG=backup.sh
# shellcheck source-path=SCRIPTDIR
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd -P)/lib.sh"

usage() {
	cat <<'EOF'
Usage: backup.sh --out <dir> [--keep <n>] [--project <p>]

Writes <dir>/<UTC stamp>/ holding, for every cell_% database of compose project <p> (default
assay-postgres):
  <database>.dump   pg_dump -Fc of that database
  globals.sql       pg_dumpall --globals-only --no-role-passwords (roles and their attributes,
                    no password hashes)
  manifest          the dump start time, the server version and each file's sha256

Runs under umask 077: the stamp directory is 0700 and every file 0600. Refuses an --out inside
this repository's working tree. Keeps the newest <n> stamp directories (default 7) and removes
older ones; a failed run removes its own partial stamp directory and nothing else.

A restore returns a cell to the START of the dump it restores: the recovery point is the dump
interval (24 hours on the suggested daily schedule). No point-in-time recovery, no zero-loss
claim. aside_* databases left by a restore are NOT dumped (docs/postgres.md §Restore).

Needs: docker compose, ASSAY_PG_SECRETS_DIR set (as for starting the server).
EOF
}

out="" keep=7
while [ $# -gt 0 ]; do
	case "$1" in
	--out) out=${2:?--out needs a value}; shift 2 ;;
	--keep) keep=${2:?--keep needs a value}; shift 2 ;;
	--project) PG_PROJECT=${2:?--project needs a value}; shift 2 ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument '$1'" 2 ;;
	esac
done
[ -n "$out" ] || { usage >&2; die "--out is required" 2; }
[[ "$keep" =~ ^[0-9]+$ ]] && [ "$keep" -ge 1 ] || die "--keep '$keep' is not a positive integer" 2
check_project "$PG_PROJECT"
need_tools docker
need_secrets_dir

repo=$(git -C "$PG_DIR" rev-parse --show-toplevel 2>/dev/null || (cd "$PG_DIR/../.." && pwd -P))
repo=$(cd -- "$repo" && pwd -P)
# not_in_repo <absolute path> — checked before anything is created, and again once the path resolves.
not_in_repo() {
	case "$1/" in
	"$repo"/* | "$PG_DIR"/*) die "--out '$1' is inside the repository working tree '$repo' — dumps never go there" ;;
	esac
}
case "$out" in
/*) ;;
*) out="$(pwd -P)/$out" ;;
esac
# The nearest existing ancestor, resolved, plus the part still to create.
probe=$out rest=""
while [ ! -d "$probe" ]; do
	rest="/$(basename -- "$probe")$rest"
	probe=$(dirname -- "$probe")
done
not_in_repo "$(cd -- "$probe" && pwd -P)$rest"

umask 077
mkdir -p -- "$out" || die "cannot create --out '$out'"
out=$(cd -- "$out" && pwd -P)
not_in_repo "$out"

require_running

started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
stamp=$(date -u +%Y%m%dT%H%M%SZ)
dir="$out/$stamp"
mkdir -- "$dir" || die "stamp directory '$dir' already exists — refusing to overwrite"
partial=1
cleanup() {
	if [ "$partial" = 1 ]; then
		rm -rf -- "$dir"
		printf '%s: backup failed — removed the partial %s\n' "$PG_PROG" "$dir" >&2
	fi
}
trap cleanup EXIT

version=$(printf 'SHOW server_version;\n' | pg_value -d postgres)
dbs=$(printf "SELECT datname FROM pg_database WHERE datname LIKE 'cell\\_%%' ORDER BY 1;\n" | pg_value -d postgres)

files=()
for db in $dbs; do
	[[ "$db" =~ ^cell_[a-z0-9_]+$ ]] || die "unexpected database name '$db' — refusing"
	pg_compose exec -T -u postgres postgres pg_dump -Fc --no-password -d "$db" >"$dir/$db.dump" ||
		die "pg_dump of $db failed"
	files+=("$db.dump")
done
pg_compose exec -T -u postgres postgres pg_dumpall --globals-only --no-role-passwords --no-password >"$dir/globals.sql" ||
	die "pg_dumpall --globals-only failed"
files+=(globals.sql)

{
	printf 'started %s\n' "$started"
	printf 'server_version %s\n' "$version"
	for f in "${files[@]}"; do
		printf 'sha256 %s  %s\n' "$(sha256_of "$dir/$f")" "$f"
	done
} >"$dir/manifest"
partial=0

say "wrote $dir (${#files[@]} files: $((${#files[@]} - 1)) cell database(s) + globals; manifest)"

# Retention: only directories named like a stamp are ever considered, newest kept.
stamps=()
while IFS= read -r s; do
	stamps+=("$s")
done < <(find "$out" -mindepth 1 -maxdepth 1 -type d -name '[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]T[0-9][0-9][0-9][0-9][0-9][0-9]Z' -exec basename {} \; | sort)
excess=$((${#stamps[@]} - keep))
for ((i = 0; i < excess; i++)); do
	rm -rf -- "${out:?}/${stamps[$i]}"
	say "retention: removed ${stamps[$i]} (keeping the newest $keep)"
done
