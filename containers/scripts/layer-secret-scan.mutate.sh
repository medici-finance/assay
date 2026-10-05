#!/bin/sh
# layer-secret-scan.mutate.sh — the negative control for the layered fixtures
# (E, F, G) in layer-secret-scan.test.sh (#2256).
#
# The defect class: the scan reads a MERGED view of the image filesystem, so
# a file one layer carries and a later layer shadows (overwrites, replaces with
# a directory, or deletes) is never read, although it ships in the image.
#
# This script plants a second instance of that class, separate from the one
# that was reported. The reported instance merged layers in blob discovery
# order. The mutant here merges them in true LAYER order, read from the save's
# manifest.json, which is how a union filesystem shows the image and how a
# careful merged scan would be written. The fixtures must catch this mutant
# too, or they only pin one way of getting the merge wrong.
#
# It runs layer-secret-scan.test.sh twice:
#   1. against the real scan — every assertion must pass (exit 0);
#   2. against the mutant — every E/F/G fixture must FAIL by name, while every
#      older assertion still passes, which shows the mutant changes only the
#      merge and that each fixture can fail on its own.
#
# Exit 0 = the control holds. Exit 1 = a fixture survived the mutant (or the
# baseline was red). Exit 2 = could not mutate or could not run.
#
# Run:  sh containers/scripts/layer-secret-scan.mutate.sh
set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SCAN="$HERE/layer-secret-scan.sh"
TEST="$HERE/layer-secret-scan.test.sh"

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t layerscanmut)
trap 'rm -rf "$WORK"' EXIT INT TERM
MUTANT="$WORK/layer-secret-scan.merged.sh"

# --- build the mutant ----------------------------------------------------------
# Three exact-line rewrites. Each must match exactly once: if the scan is
# refactored so a line no longer matches, this fails as could-not-mutate rather
# than running an unchanged copy and calling it a mutant.
awk '
  $0 == "    mkdir -p \"$WORK/layers/$n\"" {
    print "    mkdir -p \"$WORK/layers/1\""; hits++; next
  }
  $0 == "    tar -xf \"$blob\" -C \"$WORK/layers/$n\" 2>/dev/null || true" {
    print "    tar -xf \"$blob\" -C \"$WORK/layers/1\" 2>/dev/null || true"; hits++; next
  }
  $0 == "find \"$WORK/img\" -type f 2>/dev/null | while IFS= read -r blob; do" {
    # Non-tar blobs first (their order does not matter), then the layer tars
    # in manifest order, so the LAST layer is extracted last and wins.
    print "merged_order() {"
    print "  find \"$WORK/img\" -type f 2>/dev/null | while IFS= read -r f; do"
    print "    tar -tf \"$f\" >/dev/null 2>&1 || printf \"%s\\n\" \"$f\""
    print "  done"
    print "  sed \"s/.*\\\"Layers\\\":\\[\\([^]]*\\)\\].*/\\1/\" \"$WORK/img/manifest.json\" \\"
    print "    | tr \",\" \"\\n\" | tr -d \"\\\"\" | sed \"s#^#$WORK/img/#\""
    print "}"
    print "merged_order | while IFS= read -r blob; do"
    hits++; next
  }
  { print }
  END { if (hits != 3) { print "mutate: expected 3 rewrites, made " hits > "/dev/stderr"; exit 3 } }
' "$SCAN" > "$MUTANT"
rc=$?
if [ "$rc" -ne 0 ] || cmp -s "$SCAN" "$MUTANT"; then
  echo "COULD-NOT-MUTATE: the merged-view rewrite did not apply to $SCAN (awk exit $rc)" >&2
  exit 2
fi
if ! sh -n "$MUTANT"; then
  echo "COULD-NOT-MUTATE: the merged-view mutant is not valid sh" >&2
  exit 2
fi

# --- 1. baseline: the real scan passes every assertion -------------------------
sh "$TEST" > "$WORK/out.real" 2>&1
RC_REAL=$?
if [ "$RC_REAL" -ne 0 ]; then
  echo "FAIL: baseline — layer-secret-scan.test.sh is red against the real scan (exit $RC_REAL); the mutant result would mean nothing" >&2
  cat "$WORK/out.real" >&2
  exit 1
fi
echo "baseline: layer-secret-scan.test.sh exit 0 against the real scan"

# --- 2. the mutant -------------------------------------------------------------
LAYER_SECRET_SCAN="$MUTANT" sh "$TEST" > "$WORK/out.mutant" 2>&1
RC_MUT=$?
fail=0
if [ "$RC_MUT" -eq 0 ]; then
  echo "FAIL: layer-secret-scan.test.sh passed against the merged-view mutant — no fixture caught it" >&2
  fail=1
elif [ "$RC_MUT" -ne 1 ]; then
  echo "COULD-NOT-CHECK: the test exited $RC_MUT (not 1) against the mutant — it did not run to its assertions" >&2
  cat "$WORK/out.mutant" >&2
  exit 2
fi

# Every layered fixture must fail BY NAME under the mutant.
for f in "overwrite fixture 1" "overwrite fixture 2" "overwrite fixture 3" \
         "overwrite fixture 4" "overwrite fixture 5" "overwrite fixture 6" \
         "sandwich fixture 1" "sandwich fixture 2" "sandwich fixture 3" \
         "directory-swap fixture"; do
  if grep -qF "FAIL: $f — the key an earlier layer wrote at /app/key.pem was NOT caught" "$WORK/out.mutant"; then
    echo "killed: $f fails under the merged-view mutant"
  else
    echo "FAIL: $f survived the merged-view mutant" >&2
    fail=1
  fi
done

# Every older assertion must still hold, so the mutant is the merge and only it.
for ok in "RED on baked-key fixture" "GREEN on clean fixture" \
          "RED on false-positive fixture's real secret" \
          "GREEN on all five allowlisted-path mimics" \
          "RED on the generic sk--in-binary regression fixture" \
          "RED on directory-swap fixture's whited-out /app/gone.pem"; do
  if grep -qF "$ok" "$WORK/out.mutant"; then
    echo "held: $ok"
  else
    echo "FAIL: under the mutant, an assertion unrelated to the merge also broke: $ok" >&2
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  cat "$WORK/out.mutant" >&2
  exit 1
fi
echo "PASS: every layered fixture fails against a merged-view scan, and only those fixtures do"
exit 0
