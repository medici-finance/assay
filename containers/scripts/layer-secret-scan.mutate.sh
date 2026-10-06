#!/bin/sh
# layer-secret-scan.mutate.sh — the negative controls for the layered fixtures
# (E, F, G, #2256) and the fail-closed fixtures (H to W) in
# layer-secret-scan.test.sh.
#
# Each mutant is a copy of layer-secret-scan.sh with one defect planted. The
# test is run against every mutant, and the fixtures that pin that defect must
# fail there while every other assertion still passes. That shows each fixture
# can fail on its own, and that each mutant changes only what it claims to.
#
# The mutants:
#   merged     reads a MERGED view of the image filesystem, so a file one layer
#              carries and a later layer overwrites or replaces with a
#              directory is never read, although it ships in the image. The
#              reported instance merged in blob discovery order; this mutant
#              merges in true LAYER order (the scan extracts the layers in
#              manifest.json order), which is how a union filesystem shows the
#              image and how a careful merged scan would be written. The
#              fixtures must catch it too, or they only pin one way of getting
#              the merge wrong.
#   save       goes on after the saved image fails to unpack in full.
#   manifest   goes on when the manifest.json layer list cannot be read.
#   missing    goes on past a listed layer the save does not hold.
#   digest     goes on past a layer that does not match its digest name.
#   layer      goes on after a layer tar fails to extract in full.
#   nolayer    goes on with no layer extracted.
#   compressed raw-greps a compressed blob tar cannot list.
#   unread     skips making the extracted files readable.
#   unreadfind goes on past a file the readability check lists.
#   grepwalk   goes on after grep errors on the layer walk.
#   grepblob   goes on after grep errors on a non-tar blob.
#   newline    goes on past a path with a newline in its name.
#   signal     cleans up on TERM but does not exit.
#
# Each fail-closed fixture also requires its own exit-2 reason, so a later
# check that exits 2 for another reason cannot stand in for a mutated one.
#
# Exit 0 = every control holds. Exit 1 = a fixture survived its mutant, an
# unrelated assertion broke, or the baseline was red. Exit 2 = could not
# mutate or could not run.
#
# Run:  sh containers/scripts/layer-secret-scan.mutate.sh
set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SCAN="$HERE/layer-secret-scan.sh"
TEST="$HERE/layer-secret-scan.test.sh"

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t layerscanmut)
trap 'rm -rf "$WORK"' EXIT
trap 'exit 2' HUP INT TERM

# One pass line per assertion in layer-secret-scan.test.sh. A mutant KILLS an
# assertion when its pass line is missing from the test output.
CHECKS="RED on baked-key fixture
GREEN on clean fixture
RED on false-positive fixture's real secret
GREEN on all five allowlisted-path mimics
RED on the generic sk--in-binary regression fixture
RED on overwrite fixture 1 —
RED on overwrite fixture 2 —
RED on overwrite fixture 3 —
RED on overwrite fixture 4 —
RED on overwrite fixture 5 —
RED on overwrite fixture 6 —
RED on sandwich fixture 1 —
RED on sandwich fixture 2 —
RED on sandwich fixture 3 —
RED on directory-swap fixture —
RED on directory-swap fixture's whited-out /app/gone.pem
GREEN on stand-in clean control —
CLOSED on bad save fixture —
CLOSED on partial save fixture —
CLOSED on refused member fixture —
CLOSED on no layer fixture —
CLOSED on truncated gzip fixture —
CLOSED on zstd blob fixture —
CLOSED on newline name fixture —
CLOSED on signal fixture —
CLOSED on grep walk error fixture —
CLOSED on grep blob error fixture —
CLOSED on unreadable backstop fixture —
CLOSED on truncated layer fixture —
CLOSED on missing layer fixture —
CLOSED on layer digest fixture —
CLOSED on no manifest fixture —
CLOSED on bad manifest fixture —
RED on unreadable fixture —"

# --- build the mutants ---------------------------------------------------------
# Every rewrite is exact-line and must match exactly the expected number of
# times: if the scan is refactored so a line no longer matches, this fails as
# could-not-mutate rather than running an unchanged copy and calling it a
# mutant.
mutated() { # <mutant-file> <awk exit status>
  if [ "$2" -ne 0 ] || cmp -s "$SCAN" "$1"; then
    echo "COULD-NOT-MUTATE: the rewrite did not apply to $SCAN for $1 (awk exit $2)" >&2
    exit 2
  fi
  if ! sh -n "$1"; then
    echo "COULD-NOT-MUTATE: $1 is not valid sh" >&2
    exit 2
  fi
}

# swap <name> <from-line> <to-line>: replace one exact line. The lines travel
# through the environment, so awk applies no escape processing to them.
swap() {
  FROM="$2" TO="$3" awk '
    $0 == ENVIRON["FROM"] { print ENVIRON["TO"]; hits++; next }
    { print }
    END { if (hits != 1) { print "mutate: expected 1 rewrite, made " hits+0 > "/dev/stderr"; exit 3 } }
  ' "$SCAN" > "$WORK/$1.sh"
  mutated "$WORK/$1.sh" "$?"
}

# merged: two exact-line rewrites, so every layer extracts into one directory.
awk '
  $0 == "  mkdir -p \"$WORK/layers/$n\"" {
    print "  mkdir -p \"$WORK/layers/1\""; hits++; next
  }
  $0 == "  if ! tar -xf \"$1\" -C \"$WORK/layers/$n\" 2>\"$WORK/err\"; then" {
    print "  if ! tar -xf \"$1\" -C \"$WORK/layers/1\" 2>\"$WORK/err\"; then"; hits++; next
  }
  { print }
  END { if (hits != 2) { print "mutate: expected 2 rewrites, made " hits+0 > "/dev/stderr"; exit 3 } }
' "$SCAN" > "$WORK/merged.sh"
mutated "$WORK/merged.sh" "$?"

# shellcheck disable=SC2016  # the lines are scan source, not to be expanded
{
  swap save \
    '  cannot_scan "the saved image does not unpack: $(head -3 "$WORK/err")"' \
    '  :'
  swap manifest \
    'if ! layer_list > "$WORK/listed" 2>"$WORK/err"; then' \
    'if ! { layer_list || :; } > "$WORK/listed" 2>"$WORK/err"; then'
  swap missing \
    '  if [ ! -f "$WORK/img/$p" ]; then' \
    '  if false; then'
  swap digest \
    '  if ! digest_ok "$WORK/img/$p" "$p"; then' \
    '  if false; then'
  swap layer \
    '    cannot_scan "layer $2 does not extract in full: $(head -3 "$WORK/err")"' \
    '    :'
  swap nolayer \
    '  cannot_scan "manifest.json lists no layer"' \
    '  :'
  swap compressed \
    '    cannot_scan "$rel is compressed but tar cannot read it"' \
    '    printf '"'"'%s\n'"'"' "$blob" >> "$WORK/nontar-blobs"'
  swap unread \
    'if ! chmod -R u+rX "$WORK/layers" 2>"$WORK/err"; then' \
    'if false; then'
  swap unreadfind \
    'if [ -s "$WORK/unreadable" ]; then' \
    'if false; then'
  swap grepwalk \
    '  if [ "$rc" -gt 1 ]; then' \
    '  if false; then'
  swap grepblob \
    '        *) cannot_scan "grep could not read blob ${blob#"$WORK/img/"}" ;;' \
    '        *) ;;'
  swap newline \
    'if [ -s "$WORK/nlpaths" ]; then' \
    'if false; then'
  swap signal \
    "trap 'cannot_scan \"interrupted by a signal\"' HUP INT TERM" \
    "trap 'rm -rf \"\$WORK\"' HUP INT TERM"
}

# --- 1. baseline: the real scan passes every assertion -------------------------
sh "$TEST" > "$WORK/out.real" 2>&1
RC_REAL=$?
if [ "$RC_REAL" -ne 0 ]; then
  echo "FAIL: baseline — layer-secret-scan.test.sh is red against the real scan (exit $RC_REAL); the mutant results would mean nothing" >&2
  cat "$WORK/out.real" >&2
  exit 1
fi
echo "baseline: layer-secret-scan.test.sh exit 0 against the real scan"

# --- 2. the mutants ------------------------------------------------------------
fail=0
# run_mutant <name> <killed pass lines, one per line>
run_mutant() {
  LAYER_SECRET_SCAN="$WORK/$1.sh" sh "$TEST" > "$WORK/out.$1" 2>&1
  _rc=$?
  if [ "$_rc" -eq 0 ]; then
    echo "FAIL: $1 mutant — the test passed against it; no fixture caught it" >&2
    fail=1
    return
  elif [ "$_rc" -ne 1 ]; then
    echo "COULD-NOT-CHECK: the test exited $_rc (not 1) against the $1 mutant — it did not run to its assertions" >&2
    cat "$WORK/out.$1" >&2
    exit 2
  fi
  _bad=0
  while IFS= read -r _c; do
    if printf '%s\n' "$2" | grep -qxF "$_c"; then
      if grep -qF "$_c" "$WORK/out.$1"; then
        echo "FAIL: $1 mutant — survived by: $_c" >&2
        _bad=1
      else
        echo "killed: $1 mutant fails: $_c"
      fi
    elif ! grep -qF "$_c" "$WORK/out.$1"; then
      echo "FAIL: $1 mutant — an assertion it should not touch also broke: $_c" >&2
      _bad=1
    fi
  done <<EOF
$CHECKS
EOF
  if [ "$_bad" -ne 0 ]; then
    cat "$WORK/out.$1" >&2
    fail=1
  else
    echo "held: every other assertion passes under the $1 mutant"
  fi
}

run_mutant merged "RED on overwrite fixture 1 —
RED on overwrite fixture 2 —
RED on overwrite fixture 3 —
RED on overwrite fixture 4 —
RED on overwrite fixture 5 —
RED on overwrite fixture 6 —
RED on sandwich fixture 1 —
RED on sandwich fixture 2 —
RED on sandwich fixture 3 —
RED on directory-swap fixture —"
run_mutant save "CLOSED on bad save fixture —
CLOSED on partial save fixture —"
run_mutant manifest "CLOSED on no manifest fixture —
CLOSED on bad manifest fixture —"
run_mutant missing "CLOSED on missing layer fixture —"
run_mutant digest "CLOSED on layer digest fixture —"
run_mutant layer "CLOSED on refused member fixture —
CLOSED on truncated gzip fixture —
CLOSED on truncated layer fixture —"
run_mutant nolayer "CLOSED on no layer fixture —"
run_mutant compressed "CLOSED on zstd blob fixture —"
run_mutant unread "RED on unreadable fixture —"
run_mutant unreadfind "CLOSED on unreadable backstop fixture —"
run_mutant grepwalk "CLOSED on grep walk error fixture —"
run_mutant grepblob "CLOSED on grep blob error fixture —"
run_mutant newline "CLOSED on newline name fixture —"
run_mutant signal "CLOSED on signal fixture —"

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "PASS: every fixture fails against its mutant, and only those fixtures do"
exit 0
