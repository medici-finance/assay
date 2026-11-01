#!/bin/sh
# desk-run.mutate.sh — the negative controls for desk-run.test.sh.
#
# Each mutant is a copy of desk-run.sh with ONE guard removed or weakened. The
# suite is run against every mutant through its DESK_RUN hook. A mutant is
# KILLED when the suite exits non-zero and a FAIL line names the fixture that
# pins that guard. A guard whose mutant survives is a guard the suite cannot
# see being deleted, which is how a credential control reads as present and
# cannot fail. The unmutated script must pass the suite first, or the run is
# meaningless.
#
# The mutants (name: what is removed):
#   ro-mount          the ,ro option of the PEM bind mount
#   pem-comma         the comma branch of the PEM path guard
#   pem-quote         the double-quote branch of the PEM path guard
#   pem-newline       the newline branch of the PEM path guard
#   pem-regular       the PEM "not a regular file" check
#   pem-readable      the PEM "not readable" check
#   env-regular       the env-file "not a regular file" check
#   env-readable      the env-file "not readable" check
#   empty-pem-flag    --pem given empty reads as "not given" again
#   empty-env-flag    --env-file given empty reads as "not given" again
#   empty-pem-var     an empty ASSAY_DESK_PEM_FILE reads as unset again
#   empty-env-var     an empty ASSAY_DESK_ENV_FILE reads as unset again
#   attach-creds      the refusal of --pem/--env-file with --attach
#   quote-off         shell quoting of the dry-run line
#   tag-limit         the 128-character image tag limit
#   print-pem         the dry run also prints the PEM content (stdout)
#   print-env         the run also prints the env-file content (stderr)
#   echo-option       an unknown option is echoed back on stderr
#   echo-desk         an unknown desk name is echoed back on stderr
#   echo-desk2        a second desk name is echoed back on stderr
#   echo-version      an invalid tag is echoed back on stderr
#   ambient-fallback  a missing default PEM falls back to a file in $HOME
#   daemon-down       an unreachable daemon is read as "no container"
#   name-exists       the refusal when the desk's container already exists
#
# Exit 0 = every control holds. Exit 1 = a mutant survived or the baseline was
# red. Exit 2 = could not mutate or could not run.
#
# Run:  sh containers/desk-run.mutate.sh
set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/desk-run.sh"
TEST="$HERE/desk-run.test.sh"

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t deskrunmut)
trap 'rm -rf "$WORK"' EXIT
trap 'exit 2' HUP INT TERM

cannot() {
  echo "COULD-NOT-RUN: $*" >&2
  exit 2
}

[ -f "$SCRIPT" ] || cannot "no script at $SCRIPT"
[ -f "$TEST" ] || cannot "no suite at $TEST"

# --- the unmutated script must pass --------------------------------------------
if ! sh "$TEST" >"$WORK/base.out" 2>&1; then
  echo "BASELINE RED: the unmutated script fails its own suite" >&2
  grep '^FAIL' "$WORK/base.out" >&2
  exit 1
fi
base_pass=$(grep -c '^PASS' "$WORK/base.out")
echo "baseline: exit 0, $base_pass PASS, 0 FAIL"

# --- mutant definitions ----------------------------------------------------------
# Three lines each, then a blank line: "name | fixture fragment(s)", the exact
# script line to replace, and its replacement. The fragments are matched
# against FAIL lines; a mutant is killed when one of them is red. @@NL@@ in the
# replacement stands for a newline. Every rewrite must match exactly one line:
# if the script is refactored so a line no longer matches, this fails as
# could-not-mutate rather than running an unchanged copy and calling it a mutant.
cat >"$WORK/mutants" <<'MUTANTS'
ro-mount | composes the secrets.md docker run
    --mount "type=bind,src=$pem,dst=$pem_dst,ro" \
    --mount "type=bind,src=$pem,dst=$pem_dst" \

pem-comma | PEM path with a comma
    *,* | *\"* | *"$nl"*)
    *\"* | *"$nl"*)

pem-quote | PEM path with a double quote
    *,* | *\"* | *"$nl"*)
    *,* | *"$nl"*)

pem-newline | PEM path with a newline
    *,* | *\"* | *"$nl"*)
    *,* | *\"*)

pem-regular | PEM path that is a directory
  elif [ ! -f "$pem" ]; then
  elif false; then

pem-readable | an unreadable PEM
  elif [ ! -r "$pem" ]; then
  elif false; then

env-regular | env-file path that is a directory
  elif [ ! -f "$envf" ]; then
  elif false; then

env-readable | an unreadable env-file
  elif [ ! -r "$envf" ]; then
  elif false; then

empty-pem-flag | empty --pem
  if [ "$pem_set" -eq 1 ]; then
  if [ -n "$pem_arg" ]; then

empty-env-flag | empty --env-file
  if [ "$env_set" -eq 1 ]; then
  if [ -n "$env_arg" ]; then

empty-pem-var | empty ASSAY_DESK_PEM_FILE
  elif [ "${ASSAY_DESK_PEM_FILE+set}" = set ]; then
  elif [ -n "${ASSAY_DESK_PEM_FILE:-}" ]; then

empty-env-var | empty ASSAY_DESK_ENV_FILE
  elif [ "${ASSAY_DESK_ENV_FILE+set}" = set ]; then
  elif [ -n "${ASSAY_DESK_ENV_FILE:-}" ]; then

attach-creds | --attach with
if [ "$attach" -eq 1 ] && { [ "$pem_set" -eq 1 ] || [ "$env_set" -eq 1 ]; }; then
if false; then

quote-off | pastes back to the same arguments
    *[!A-Za-z0-9_@%+=:,./-]*)
    ZZ-never-matches)

tag-limit | 129-character image tag
[ "${#version}" -le 128 ] || die 2 "$prog: invalid --version: longer than 128 characters"
:

print-pem | credential paths, never contents
# --- compose -----------------------------------------------------------------
[ -z "$pem" ] || cat "$pem"@@NL@@# --- compose -----------------------------------------------------------------

print-env | no output line of any case contains a fixture credential value
# --- compose -----------------------------------------------------------------
[ -z "$envf" ] || cat "$envf" >&2@@NL@@# --- compose -----------------------------------------------------------------

echo-option | unknown option is refused and its text is not echoed
      die 2 "$prog: unknown option (argument $argn, ${#1} characters; not echoed)" \
      die 2 "$prog: unknown option: $1" \

echo-desk | unknown desk name is refused and its text is not echoed
  die 2 "$prog: unknown desk name (${#desk} characters; not echoed)" "valid desks: $desks"
  die 2 "$prog: unknown desk '$desk'" "valid desks: $desks"

echo-desk2 | second desk name is refused and its text is not echoed
      [ -z "$desk" ] || die 2 "$prog: more than one desk name given (second one is argument $argn, ${#1} characters; not echoed)" \
      [ -z "$desk" ] || die 2 "$prog: more than one desk name given: '$desk' and '$1'" \

echo-version | invalid image tag is refused and its text is not echoed
    die 2 "$prog: invalid --version (${#version} characters; not echoed)" \
    die 2 "$prog: invalid --version '$version'" \

ambient-fallback | missing defaults name
    [ -n "$pem" ] || pem=$cfg/assay/app.pem
    [ -n "$pem" ] || { pem=$cfg/assay/app.pem; [ -e "$pem" ] || pem=${HOME:-}/app.pem; }

daemon-down | unreachable docker daemon
if ! existing=$(docker container ls -a --filter "name=^/$desk\$" --format '{{.Names}}' 2>/dev/null); then
if ! existing=$(docker container ls -a --filter "name=^/$desk\$" --format '{{.Names}}' 2>/dev/null || :); then

name-exists | refuses when a container of that name already exists
  [ "$have" -eq 0 ] || die 4 "$prog: a container named '$desk' already exists" \
  [ 0 -eq 0 ] || die 4 "$prog: a container named '$desk' already exists" \

MUTANTS

# --- build and run each mutant -----------------------------------------------------
survived=0
killed=0
skipped=0
root=0
[ "$(id -u)" -eq 0 ] && root=1

printf '%-18s %-6s %s\n' MUTANT RESULT "SUITE (exit, failed cases) / first red fixture"
while IFS= read -r header; do
  [ -n "$header" ] || continue
  IFS= read -r from || cannot "truncated mutant definition: $header"
  IFS= read -r to || cannot "truncated mutant definition: $header"
  name=${header%% | *}
  frag=${header#* | }
  file="$WORK/$name.sh"

  FROM=$from TO=$to awk '
    BEGIN { to = ENVIRON["TO"]; gsub(/@@NL@@/, "\n", to) }
    $0 == ENVIRON["FROM"] { print to; hits++; next }
    { print }
    END { if (hits != 1) { print "mutate: expected 1 rewrite, made " hits + 0 > "/dev/stderr"; exit 3 } }
  ' "$SCRIPT" >"$file" || cannot "$name: the rewrite did not apply to $SCRIPT"
  cmp -s "$SCRIPT" "$file" && cannot "$name: the rewrite changed nothing"
  sh -n "$file" || cannot "$name: the mutant is not valid sh"

  # The two "unreadable" fixtures cannot fail for root, which reads any file.
  case $name in
    pem-readable | env-readable)
      if [ "$root" -eq 1 ]; then
        printf '%-18s %-6s %s\n' "$name" SKIP "could-not-check: root reads any file"
        skipped=$((skipped + 1))
        continue
      fi
      ;;
  esac

  DESK_RUN=$file sh "$TEST" >"$WORK/$name.out" 2>&1
  rc=$?
  nfail=$(grep -c '^FAIL' "$WORK/$name.out")
  first=$(grep '^FAIL' "$WORK/$name.out" | head -1 | cut -c1-90)
  hit=0
  [ "$rc" -ne 0 ] && grep '^FAIL' "$WORK/$name.out" | grep -F -q -- "$frag" && hit=1
  if [ "$hit" -eq 1 ]; then
    printf '%-18s %-6s exit %s, %s failed / %s\n' "$name" KILLED "$rc" "$nfail" "$first"
    killed=$((killed + 1))
  else
    printf '%-18s %-6s exit %s, %s failed (wanted a FAIL naming: %s)\n' "$name" SURVIVED "$rc" "$nfail" "$frag"
    survived=$((survived + 1))
  fi
done <"$WORK/mutants"

echo "killed $killed, survived $survived, skipped $skipped"
[ "$survived" -eq 0 ] || exit 1
exit 0
