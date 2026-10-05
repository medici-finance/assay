#!/bin/sh
# Positive control for both advisory GitLab columns and their exclusion from totals.
set -eu
cd "$(dirname "$0")/.."
counter="$PWD/scripts/forge-ban.sh"
plant=$(mktemp -d "$PWD/internal/deskkit/ban-probe.XXXXXX")
trap 'rm -rf "$plant"' EXIT HUP INT TERM
sh "$counter" > "$plant/before"
cat > "$plant/probe.go" <<'EOF'
//go:build ignore
package deskkit
import "os/exec"
var probe = exec.Command("glab", "api")
var endpoint = "/api/v4"
EOF
sh "$counter" > "$plant/after"
for prefix in 'class e (glab subprocess):' 'class f (GitLab API literal):'; do
  old=$(awk -v p="$prefix" 'index($0,p)==1 {for(i=1;i<=NF;i++) if($i~/^desk=/) {split($i,a,"="); print a[2]}}' "$plant/before")
  new=$(awk -v p="$prefix" 'index($0,p)==1 {for(i=1;i<=NF;i++) if($i~/^desk=/) {split($i,a,"="); print a[2]}}' "$plant/after")
  if [ -z "$old" ] || [ -z "$new" ] || [ "$new" -ne "$((old + 1))" ]; then
    echo "PROBE FAIL: $prefix before=$old after=$new"; exit 1
  fi
done
for prefix in 'forge reach-around sites:' 'statusgen sites:'; do
  old=$(grep -F "$prefix" "$plant/before")
  new=$(grep -F "$prefix" "$plant/after")
  if [ -z "$old" ] || [ "$old" != "$new" ]; then echo "PROBE FAIL: $prefix"; exit 1; fi
done
echo 'PROBE PASS'
