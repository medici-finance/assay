#!/usr/bin/env bash
# Command corrections for desk-containers/26. Run from the repository root.
# This runner supplies no acceptance approval and never writes Evidence.
set -euo pipefail
export KUBECONFIG=/dev/null GOPROXY=off GOSUMDB=off
row=${1:?row 3, 5, 9, 10 or 12 required}
root=$(pwd -P)
need() { command -v "$1" >/dev/null || { echo "could-not-check: missing $1" >&2; exit 6; }; }
test -s containers/postgres/compose.yaml || { echo 'missing PostgreSQL compose input' >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
case "$row" in
3)
  need docker
  docker compose version >/dev/null
  mkdir "$work/secrets"
  # First prove the same configuration resolves with its required input present.
  ASSAY_PG_SECRETS_DIR="$work/secrets" docker compose --env-file /dev/null -f "$root/containers/postgres/compose.yaml" config -q
  rc=0
  env -u ASSAY_PG_SECRETS_DIR docker compose --env-file /dev/null -f "$root/containers/postgres/compose.yaml" config -q >"$work/refusal" 2>&1 || rc=$?
  test "$rc" -ne 0
  grep -F 'ASSAY_PG_SECRETS_DIR' "$work/refusal"
  ;;
5)
  # A missing directory, empty inventory or unreadable input is not a clean scan.
  test -d containers/postgres
  files=()
  find containers/postgres -type f -print0 >"$work/inventory"
  while IFS= read -r -d '' file; do files+=("$file"); done <"$work/inventory"
  test "${#files[@]}" -gt 0
  for file in "${files[@]}"; do test -r "$file"; done
  rc=0
  grep -c -e 'ghs_' -e 'ghp_' -e 'github_pat_' -e 'sk-ant-' -e 'PRIVATE KEY' -e 'POSTGRES_PASSWORD[[:space:]]*[:=]' -e 'PGPASSWORD' -- "${files[@]}" || rc=$?
  test "$rc" -eq 1
  ;;
9)
  need go
  go -C tools/desk build -o "$work/cellctl" ./cmd/cellctl
  CELLCTL_A="$root/tools/cellctl/testdata/cellctl-shell-oracle.sh" CELLCTL_B="$work/cellctl" bash tools/cellctl/tests/parity.test.sh
  ;;
10)
  need go
  test -s tools/desk/cmd/cellctl/celldb_test.go
  # Go fixtures must name the physical temp directory: macOS /var is a symlink,
  # which the cache custody positive control deliberately refuses.
  TMPDIR=$(cd "${TMPDIR:-/tmp}" && pwd -P)
  export TMPDIR
  # Print the delivery test's output before acting on its status, so a failure shows why.
  rc=0
  go -C tools/desk test ./cmd/cellctl/ -run '^TestCellDB$' -timeout 120s -count=1 -v >"$work/celldb.out" 2>&1 || rc=$?
  cat "$work/celldb.out"
  test "$rc" -eq 0
  grep -F -e '--- PASS: TestCellDB (' "$work/celldb.out"
  # Retain the original whole-package obligation as well as the delivery test.
  go -C tools/desk test ./cmd/cellctl/ -timeout 10m -count=1
  ;;
12)
  bash tools/cellctl/tests/postgres-scratch.sh
  ;;
*) echo "unknown Verify row: $row" >&2; exit 2 ;;
esac
