#!/usr/bin/env bash
# Row 12: a fresh local-only server and cell; never the operator's project.
set -euo pipefail
export KUBECONFIG=/dev/null GOPROXY=off GOSUMDB=off
for tool in docker go python3 psql git; do
  command -v "$tool" >/dev/null || { echo "could-not-check: missing $tool" >&2; exit 6; }
done
root=$(pwd -P)
test -s "$root/containers/postgres/compose.yaml"
# Check the destination before the first daemon call. No TCP/SSH Docker contexts.
if [[ -n "${DOCKER_CONTEXT:-}" ]]; then
  endpoint=$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')
else
  endpoint=${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}
fi
case "$endpoint" in unix:///*) ;; *) echo 'could-not-check: scratch requires a local Docker socket' >&2; exit 6;; esac
# Pin the checked destination; DOCKER_CONTEXT otherwise outranks DOCKER_HOST.
export DOCKER_HOST="$endpoint"
unset DOCKER_CONTEXT
work=$(mktemp -d)
project="assay-postgres-scratch-$(python3 -I -c 'import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest()[:16])' "$work")"
started=0
compose() { docker compose -p "$project" -f "$root/containers/postgres/compose.yaml" "$@"; }
finish() {
  rc=$?
  trap - EXIT
  if [ "$started" = 1 ]; then compose down -v --remove-orphans || rc=1; fi
  rm -rf -- "$work"
  exit "$rc"
}
trap finish EXIT
trap 'exit 130' INT TERM HUP
umask 077
export ASSAY_PG_SECRETS_DIR="$work/secrets"
mkdir "$ASSAY_PG_SECRETS_DIR"
python3 -I -c 'import secrets; print(secrets.token_urlsafe(32))' >"$ASSAY_PG_SECRETS_DIR/superuser.password"
export ASSAY_PG_HOST_PORT
ASSAY_PG_HOST_PORT=$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
docker info >/dev/null
# Keep inspection failure separate from an inspected empty result.
containers=$(docker ps -aq --filter "label=com.docker.compose.project=$project")
test -z "$containers" || { echo 'scratch project already exists; refusing' >&2; exit 1; }
volumes=$(docker volume ls -q --filter "label=com.docker.compose.project=$project")
test -z "$volumes" || { echo 'scratch volume already exists; refusing' >&2; exit 1; }
networks=$(docker network ls -q --filter "label=com.docker.compose.project=$project")
test -z "$networks" || { echo 'scratch network already exists; refusing' >&2; exit 1; }
image=$(compose config --images)
docker image inspect "$image" >/dev/null || { echo 'could-not-check: pinned image is not cached; no pull performed' >&2; exit 6; }
go -C tools/desk build -o "$work/cellctl" ./cmd/cellctl
mkdir -p "$work/home/.config/gh" "$work/home/.config/assay" "$work/home/.claude" "$work/bin" "$work/repo/docs/streams"
: >"$work/home/.gitconfig"
cat >"$work/home/.config/assay/roster.env" <<'EOF'
ASSAY_BLESS_LOGIN=example-operator:1001
ASSAY_TRUSTED_LOGINS=example-operator:1001
ASSAY_TRUSTED_BOT_SLUGS=github:example-worker-app:2001,github:example-reviewer-app:2002
ASSAY_REPO_FORGES=example-org/example-repo=github
ASSAY_ALLOWED_REPOS=example-org/example-repo:no-ci:public
ASSAY_SCAN_REPOS=example-org/example-repo
EOF
chmod 600 "$work/home/.config/assay/roster.env"
printf '#!/usr/bin/env bash\nprintf "3.0.0 (scratch stub)\\n"\n' >"$work/bin/claude"
chmod +x "$work/bin/claude"
git init -q -b main "$work/repo"
printf 'scratch input\n' >"$work/repo/README.md"
git -C "$work/repo" add README.md
git -C "$work/repo" -c user.name=Example -c user.email=example@example.invalid commit -qm fixture
run_cell() {
  env -i HOME="$work/home" PATH="$work/bin:/usr/bin:/bin:/usr/sbin:/sbin" SHELL=/bin/bash \
    KUBECONFIG=/dev/null ASSAY_CONFIG_HOME="$work/home/.config/assay" CELLS_ROOT="$work/cells" \
    DESK_TOOLS_BIN="$work/bin" DRY_RUN="${DRY_RUN:-0}" "$work/cellctl" "$@"
}
run_cell new example-cell --kind house --repo "$work/repo" --roots "example-org/example-repo=$work/repo" --roles worker-desk
run_cell set example-cell CELL_COCKPIT=tmux
printf '#!/usr/bin/env bash\nexit 0\n' >"$work/bin/tmux"
chmod +x "$work/bin/tmux"
started=1
compose up -d --wait --wait-timeout 180 --pull never --no-build
bash containers/postgres/provision-cell.sh --project "$project" --cell example-cell --passfile "$work/example-cell.pgpass" --generate >"$work/provision.out"
dsn=$(sed -n 's/^CELL_DB_DSN=//p' "$work/provision.out")
test "$dsn" = "host=127.0.0.1 port=$ASSAY_PG_HOST_PORT dbname=cell_example_cell user=cell_example_cell sslmode=disable"
run_cell set example-cell "CELL_DB_DSN=$dsn" "CELL_DB_PASSFILE=$work/example-cell.pgpass"
DRY_RUN=1 run_cell desk example-cell worker-desk >"$work/plan"
cat "$work/plan"
grep -F -x "[dry-run] env ASSAY_CELL_DB_DSN=$dsn (cell database, from CELL_DB_DSN)" "$work/plan"
grep -F -x "[dry-run] env PGPASSFILE=$work/example-cell.pgpass (cell database credential file, from CELL_DB_PASSFILE)" "$work/plan"
export ASSAY_CELL_DB_DSN="$dsn" PGPASSFILE="$work/example-cell.pgpass"
who=$(psql "$ASSAY_CELL_DB_DSN" --no-password -X -At -v ON_ERROR_STOP=1 -c 'select current_user')
printf '%s\n' "$who"
test "$who" = cell_example_cell
