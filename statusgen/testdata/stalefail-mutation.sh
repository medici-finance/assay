#!/usr/bin/env bash
# Run from statusgen/. Plant a second unclassified route and require the class
# guard to name it. The trap removes only the source file minted by this run.
set -euo pipefail
seed=$(mktemp "./planted_route_XXXXXX")
plant="$seed.go"
mv "$seed" "$plant"
log=$(mktemp)
trap 'rm -f "$plant" "$log"' EXIT
cat > "$plant" <<'GO'
package main
func plantedRoute() string { return "no decision-issue — file one via --decision-issues" }
GO
if go test . -run '^TestFailNoticeClass$' -count=1 -timeout=60s > "$log" 2>&1; then
  cat "$log"
  echo 'FAIL: second route escaped the guard'
  exit 1
fi
if ! grep -F "unclassified waiting-brief route in ${plant#./}:plantedRoute" "$log"; then
  cat "$log"
  echo 'FAIL: test failed without detecting the planted route'
  exit 1
fi
echo 'PASS: the planted second route was detected'
