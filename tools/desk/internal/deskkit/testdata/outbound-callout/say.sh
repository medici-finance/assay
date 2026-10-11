#!/bin/sh
# Invented stub: records that it ran and what it was asked, then replays say.out (stdout) and
# say.err (stderr) from beside itself and exits with the status in say.code (default 0). A
# test writes those three files to make the stub answer anything at all.
d="${0%/*}"
echo run >> "$d/ran.log"
cat > "$d/request.json"
if [ -f "$d/say.out" ]; then cat "$d/say.out"; fi
if [ -f "$d/say.err" ]; then cat "$d/say.err" >&2; fi
code=0
if [ -f "$d/say.code" ]; then code=$(cat "$d/say.code"); fi
exit "$code"
