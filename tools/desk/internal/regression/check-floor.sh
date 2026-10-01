#!/usr/bin/env bash
# Run each manifest-named behavior in its owning module, bounded and offline.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../../.." && pwd)
export KUBECONFIG=/dev/null
rows=$(mktemp "${TMPDIR:-/tmp}/floor-rows.XXXXXX")
trap 'rm -f "$rows"' EXIT
awk -F '|' '
 /^## Dropped/ { exit }
 $2 ~ /^ #[0-9]+ $/ {
   pkg=$4; test=$5
   gsub(/^ +| +$/, "", pkg); gsub(/^ +| +$/, "", test)
   print pkg " " test
 }' "$here/MANIFEST.md" > "$rows"
test -s "$rows"
count=0
while read -r pkg name; do
  case "$pkg" in
    tools/desk/*) module=tools/desk; package="./${pkg#tools/desk/}" ;;
    statusgen/) module=statusgen; package=. ;;
    *) echo "invalid owning package: $pkg" >&2; exit 1 ;;
  esac
  # -run is anchored: a renamed/missing test cannot hide behind another match.
  # -v is retained so an empty selection is detected below.
  out=$(mktemp "${TMPDIR:-/tmp}/floor-test.XXXXXX")
  if ! ( cd "$root/$module" && go test -run "^${name}$" -count=1 -timeout 90s -v "$package" ) > "$out" 2>&1; then
    cat "$out"; rm -f "$out"; exit 1
  fi
  if ! grep -E -e "^--- PASS: $name [(]" "$out"; then
    cat "$out"; rm -f "$out"; echo "no top-level PASS for $name" >&2; exit 1
  fi
  rm -f "$out"
  count=$((count + 1))
done < "$rows"
echo "seed passes=$count"
