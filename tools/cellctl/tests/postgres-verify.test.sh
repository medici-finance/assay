#!/usr/bin/env bash
# regression: #5822 — missing/error inputs must not satisfy a negative assertion.
set -euo pipefail
runner=$(cd "$(dirname "$0")" && pwd -P)/postgres-verify.sh
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/bin" "$work/repo/containers/postgres"
cat >"$work/bin/docker" <<'EOF'
#!/usr/bin/env bash
case "$*" in *version*) exit 0;; esac
if [[ "${BROKEN_COMPOSE:-0}" == 1 ]]; then echo 'unrelated configuration error' >&2; exit 2; fi
if [[ -z "${ASSAY_PG_SECRETS_DIR:-}" ]]; then echo 'required variable ASSAY_PG_SECRETS_DIR is missing' >&2; exit 1; fi
exit 0
EOF
chmod +x "$work/bin/docker"
# shellcheck source=SCRIPTDIR/private-path.sh
source "$(dirname "$runner")/private-path.sh"
private_path "$work/bin"
reject() {
  if (cd "$work/repo" && bash "$runner" "$1") >"$work/out" 2>&1; then
    echo "FAIL: accepted $2" >&2; cat "$work/out"; exit 1
  fi
  printf 'ok rejected %s\n' "$2"
}
# Original commands pass when delivery inputs are absent; corrected commands fail.
(cd "$work/repo" && bash -c 'env -u ASSAY_PG_SECRETS_DIR docker compose -f containers/postgres/compose.yaml config -q >/dev/null 2>&1; test $? -ne 0')
(cd "$work/repo" && bash -c 'grep -c -e ghs_ $(find containers/postgres -type f) </dev/null; test $? -eq 1')
reject 3 'missing compose input (base shape)'
reject 5 'empty scan input (base shape)'
printf 'name: example\n' >"$work/repo/containers/postgres/compose.yaml"
(cd "$work/repo" && bash "$runner" 3 && bash "$runner" 5)
BROKEN_COMPOSE=1 reject 3 'unrelated compose failure'
for plant in 'POSTGRES_PASSWORD: synthetic' 'PGPASSWORD=synthetic'; do
  # A second file, outside the reported compose site, proves the inventory sweep.
  printf '%s\n' "$plant" >"$work/repo/containers/postgres/second.sh"
  reject 5 'credential use in a sibling file'
done
rm "$work/repo/containers/postgres/second.sh"
reject 10 'missing delivery test (base shape)'
# The scratch project must be wholly absent, including a leftover network.
# Stub every daemon operation and stop at the build if the guard is removed.
scratch=$(dirname "$runner")/postgres-scratch.sh
cat >"$work/bin/docker" <<'EOF'
#!/usr/bin/env bash
if [[ "${DOCKER_FAIL:-}" == "${1:-}" ]]; then exit 9; fi
if [[ -n "${DOCKER_LOG:-}" ]]; then printf '%s\n' "$*" >>"$DOCKER_LOG"; fi
case "$*" in
  'context inspect '*) echo unix:///example-fixture.sock;;
  info|'ps -aq '*|'volume ls -q '*) ;;
  'network ls -q '*) echo synthetic-leftover-network;;
  *'config --images') echo postgres:example;;
  'image inspect '*) ;;
  *) exit 2;;
esac
EOF
cat >"$work/bin/go" <<'EOF'
#!/usr/bin/env bash
printf 'unguarded build\n' >"$FENCE_MARK"
exit 77
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$work/bin/psql"
chmod +x "$work/bin/docker" "$work/bin/go" "$work/bin/psql"
export FENCE_MARK="$work/build-reached"
network_guard() {
  local rc=0
  (cd "$work/repo" && DOCKER_CONTEXT=fixture bash "$1") >"$work/network.out" 2>&1 || rc=$?
  test "$rc" -eq 1 && grep -F 'scratch network already exists; refusing' "$work/network.out" && test ! -e "$FENCE_MARK"
}
network_guard "$scratch"
# shellcheck disable=SC2016 # Match literal shell source, never expand its variables.
sed -e '/^networks=$(docker network ls -q/d' -e '/^test -z "$networks"/d' "$scratch" >"$work/no-network-guard.sh"
if network_guard "$work/no-network-guard.sh"; then
  echo 'FAIL: removed network guard was not detected' >&2; exit 1
fi
test -s "$FENCE_MARK"
printf 'ok network guard mutation detected before any real daemon effect\n'
rm "$FENCE_MARK"
for inspection in ps volume network; do
  rc=0
  (cd "$work/repo" && DOCKER_CONTEXT=fixture DOCKER_FAIL="$inspection" bash "$scratch") >"$work/inspection.out" 2>&1 || rc=$?
  test "$rc" -eq 9 && test ! -e "$FENCE_MARK"
  printf 'ok rejected failed %s inspection\n' "$inspection"
done
distinct_projects() {
  : >"$work/docker.log"
  local first second r1=0 r2=0 n
  (cd "$work/repo" && DOCKER_CONTEXT=fixture DOCKER_LOG="$work/docker.log" bash "$1") >"$work/first.out" 2>&1 & first=$!
  (cd "$work/repo" && DOCKER_CONTEXT=fixture DOCKER_LOG="$work/docker.log" bash "$1") >"$work/second.out" 2>&1 & second=$!
  wait "$first" || r1=$?
  wait "$second" || r2=$?
  test "$r1" -eq 1 && test "$r2" -eq 1 || return 1
  n=$(grep -o 'label=com.docker.compose.project=assay-postgres-scratch-[0-9a-f]*' "$work/docker.log" | sort -u | wc -l) || return 1
  test "$n" -eq 2
}
distinct_projects "$scratch"
# Keep the prefix so the distinct-count comparison, not the pattern, must reject it.
sed 's/^project=.*/project=assay-postgres-scratch-shared/' "$scratch" >"$work/shared-project.sh"
if distinct_projects "$work/shared-project.sh"; then
  echo 'FAIL: shared-project mutation was not detected' >&2; exit 1
fi
printf 'ok simultaneous fixtures own distinct projects; shared-project mutation detected\n'
printf 'ok PostgreSQL Verify controls\n'
