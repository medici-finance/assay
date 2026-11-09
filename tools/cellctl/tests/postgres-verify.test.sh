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
export PATH="$work/bin:$PATH"
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
printf 'ok PostgreSQL Verify controls\n'
