#!/bin/sh
# desk-run.test.sh — offline tests for containers/desk-run.sh.
#
# No docker daemon is needed. The cases drive --dry-run and the refusal paths
# directly, and exercise the real-run path against a stub `docker` placed first
# on PATH that records its arguments and never touches a daemon.
#
# Every case prints one PASS or FAIL line. The credential fixtures hold
# sentinel values generated at run time (nothing key-shaped is committed here);
# one case proves that no sentinel appears in ANY output of ANY case.
#
# Usage: sh containers/desk-run.test.sh
#   DESK_RUN=<path>   test another copy of the script (the mutation runs use it)
# Exit: 0 all cases passed; 1 a case failed.
set -u

here=$(cd "$(dirname "$0")" && pwd)
run=${DESK_RUN:-$here/desk-run.sh}
[ -f "$run" ] || {
  echo "FAIL: setup: no script at $run" >&2
  exit 1
}

tmp=$(mktemp -d "${TMPDIR:-/tmp}/desk-run-test.XXXXXX") || exit 1
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# --- fixtures ----------------------------------------------------------------
home=$tmp/home
creds=$tmp/creds
mkdir -p "$home" "$creds" "$tmp/bin" "$tmp/emptybin"
pem=$creds/app.pem
envf=$creds/desk.env
sent_pem="FIXTURE-PEM-SENTINEL-$$-$(date +%s)"
sent_env="FIXTURE-ENV-SENTINEL-$$-$(date +%s)"
begin=BEGIN
{
  printf -- '-----%s FIXTURE NOT A KEY-----\n' "$begin"
  printf '%s\n' "$sent_pem"
} >"$pem"
printf 'ANTHROPIC_API_KEY=%s\n' "$sent_env" >"$envf"

# Stub docker: records every invocation on one line, answers `container ls`
# from STUB_EXISTING (a container name, or empty for none) and fails it when
# STUB_LS_FAIL is set, and succeeds at everything else.
cat >"$tmp/bin/docker" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >>"$STUB_LOG"
if [ "$1" = container ] && [ "$2" = ls ]; then
  [ -z "${STUB_LS_FAIL:-}" ] || exit 1
  [ -z "${STUB_EXISTING:-}" ] || printf '%s\n' "$STUB_EXISTING"
  exit 0
fi
exit 0
STUB
chmod +x "$tmp/bin/docker"

log=$tmp/docker.log
: >"$log"
all=$tmp/all-output.log
: >"$all"
base_path=/usr/bin:/bin:/usr/sbin:/sbin
stub_path=$tmp/bin:$base_path

# Nothing from the operator's own environment may reach the script.
unset ASSAY_DESK_PEM_FILE ASSAY_DESK_ENV_FILE XDG_CONFIG_HOME DOCKER_HOST
STUB_LOG=$log
export STUB_LOG

failures=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() {
  printf 'FAIL: %s: %s\n' "$1" "$2"
  failures=$((failures + 1))
}

# drun [VAR=val ...] /bin/sh <script> args... : run in a scrubbed environment,
# with the stub docker first on PATH. Captures stdout, stderr and the status.
drun() {
  env -i HOME="$home" PATH="$stub_path" STUB_LOG="$log" "$@" >"$tmp/out" 2>"$tmp/err"
  rc=$?
  cat "$tmp/out" "$tmp/err" >>"$all"
}
# desk <args...> : run the script under test with the fixture credentials.
desk() {
  drun ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" "$@"
}

has() { grep -F -q -- "$2" "$1"; }
combined() { cat "$tmp/out" "$tmp/err"; }
docker_calls() { wc -l <"$log" | tr -d ' '; }

# expect <desk> <tag> : the exact command secrets.md section 4.1 prescribes.
expect() {
  printf 'docker run -it --name %s -v %s-work:/work --mount type=bind,src=%s,dst=/run/secrets/assay/app.pem,ro --env-file %s ghcr.io/medici-finance/assay/%s:%s' \
    "$1" "$1" "$pem" "$envf" "$1" "$2"
}

# --- dry-run output for each of the five desks -------------------------------
for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do
  name="dry-run composes the secrets.md docker run for $d"
  : >"$log"
  desk "$d" --dry-run
  want=$(expect "$d" latest)
  got=$(cat "$tmp/out")
  if [ "$rc" -ne 0 ]; then
    fail "$name" "exit $rc: $(combined)"
  elif [ "$got" != "$want" ]; then
    fail "$name" "got: $got / want: $want"
  elif [ "$(docker_calls)" -ne 0 ]; then
    fail "$name" "--dry-run called docker"
  else
    pass "$name"
  fi
done

name="dry-run output carries the row-3 tokens"
desk pr-review-desk --dry-run
miss=""
for tok in ' -it ' '--name pr-review-desk' ':/work' '/run/secrets/assay/app.pem' ',ro ' '--env-file' 'ghcr.io/medici-finance/assay/pr-review-desk'; do
  has "$tmp/out" "$tok" || miss="$miss [$tok]"
done
if [ -z "$miss" ]; then pass "$name"; else fail "$name" "missing:$miss"; fi

name="the PEM mount is read-only and the last mount option"
desk worker-desk --dry-run
if grep -q -E -- 'dst=/run/secrets/assay/app\.pem,ro( |$)' "$tmp/out"; then pass "$name"; else fail "$name" "$(cat "$tmp/out")"; fi

name="--version and --version= choose the image tag"
desk worker-desk --dry-run --version v1.2.3
a=$(cat "$tmp/out")
desk worker-desk --dry-run --version=sha-abc1234
b=$(cat "$tmp/out")
if [ "$a" = "$(expect worker-desk v1.2.3)" ] && [ "$b" = "$(expect worker-desk sha-abc1234)" ]; then pass "$name"; else fail "$name" "$a / $b"; fi

name="--pem and --env-file override the environment"
alt=$tmp/alt
mkdir -p "$alt"
cp "$pem" "$alt/k.pem"
cp "$envf" "$alt/m.env"
drun ASSAY_DESK_PEM_FILE="$tmp/does-not-exist" ASSAY_DESK_ENV_FILE="$tmp/does-not-exist" \
  /bin/sh "$run" the-desk --dry-run --pem "$alt/k.pem" --env-file="$alt/m.env"
if [ "$rc" -eq 0 ] && has "$tmp/out" "src=$alt/k.pem," && has "$tmp/out" "--env-file $alt/m.env "; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="the default locations are <config>/assay/app.pem and desk.env"
mkdir -p "$home/.config/assay"
cp "$pem" "$home/.config/assay/app.pem"
cp "$envf" "$home/.config/assay/desk.env"
drun /bin/sh "$run" intake-desk --dry-run
if [ "$rc" -eq 0 ] && has "$tmp/out" "src=$home/.config/assay/app.pem," && has "$tmp/out" "--env-file $home/.config/assay/desk.env "; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi
rm -rf "$home/.config"

name="a relative path is anchored so docker gets an absolute mount source"
(cd "$creds" && env -i HOME="$home" PATH="$stub_path" ASSAY_DESK_PEM_FILE=app.pem ASSAY_DESK_ENV_FILE=desk.env \
  /bin/sh "$run" the-desk --dry-run >"$tmp/out" 2>"$tmp/err")
rc=$?
cat "$tmp/out" "$tmp/err" >>"$all"
if [ "$rc" -eq 0 ] && has "$tmp/out" "src=$pem,"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

# --- unknown desk, usage errors ----------------------------------------------
name="unknown desk is refused and the five valid desks are listed"
: >"$log"
desk nonsense-desk --dry-run
miss=""
for d in intake-desk worker-desk pr-review-desk verify-desk the-desk; do
  combined | grep -F -q -- "$d" || miss="$miss $d"
done
if [ "$rc" -ne 0 ] && [ -z "$miss" ] && [ "$(docker_calls)" -eq 0 ]; then pass "$name"; else fail "$name" "rc=$rc missing:$miss"; fi

name="near-miss and hostile desk names are refused"
bad=""
for d in worker Worker-Desk 'worker-desk ' 'worker-desk;ls' '../worker-desk' 'the-desk/x' '-it'; do
  desk "$d" --dry-run
  [ "$rc" -ne 0 ] || bad="$bad [$d]"
done
if [ -z "$bad" ]; then pass "$name"; else fail "$name" "accepted:$bad"; fi

name="a missing desk name is refused"
desk --dry-run
if [ "$rc" -eq 2 ] && combined | grep -q 'valid desks'; then pass "$name"; else fail "$name" "rc=$rc"; fi

name="two desk names are refused"
desk worker-desk the-desk --dry-run
if [ "$rc" -eq 2 ]; then pass "$name"; else fail "$name" "rc=$rc"; fi

name="an unknown option is refused"
desk worker-desk --dry-run --privileged
if [ "$rc" -eq 2 ] && has "$tmp/err" '--privileged'; then pass "$name"; else fail "$name" "rc=$rc"; fi

name="a flag missing its value is refused"
desk worker-desk --version
if [ "$rc" -eq 2 ]; then pass "$name"; else fail "$name" "rc=$rc"; fi

name="an invalid image tag is refused"
bad=""
for v in 'v1;rm' 'a b' '-x' '.x' 'x/y' 'x:y'; do
  desk worker-desk --dry-run --version "$v"
  [ "$rc" -eq 2 ] || bad="$bad [$v]"
done
desk worker-desk --dry-run --version=
[ "$rc" -eq 2 ] || bad="$bad [empty]"
if [ -z "$bad" ]; then pass "$name"; else fail "$name" "accepted:$bad"; fi

name="--help prints usage and exits 0"
desk --help
if [ "$rc" -eq 0 ] && has "$tmp/out" 'Usage:'; then pass "$name"; else fail "$name" "rc=$rc"; fi

# --- fail-closed credential preflight ----------------------------------------
name="a missing PEM is refused and its expected location is named"
: >"$log"
drun ASSAY_DESK_PEM_FILE="$creds/absent/app.pem" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" "$creds/absent/app.pem" && [ "$(docker_calls)" -eq 0 ]; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="a missing env-file is refused and its expected location is named"
drun ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$creds/absent.env" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" "$creds/absent.env"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="both missing files are reported in one pass"
drun ASSAY_DESK_PEM_FILE="$creds/a.pem" ASSAY_DESK_ENV_FILE="$creds/b.env" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" "$creds/a.pem" && has "$tmp/err" "$creds/b.env"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="missing defaults name the <config>/assay locations and do not fall back"
# Ambient credential-looking files exist elsewhere under HOME; none may be used.
mkdir -p "$home/.ssh" "$home/.aws"
cp "$pem" "$home/.ssh/app.pem"
cp "$envf" "$home/.aws/desk.env"
cp "$pem" "$home/app.pem"
drun /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" "$home/.config/assay/app.pem" && has "$tmp/err" "$home/.config/assay/desk.env" \
  && ! has "$tmp/out" 'docker'; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="XDG_CONFIG_HOME moves the default locations"
drun XDG_CONFIG_HOME="$tmp/xdg" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" "$tmp/xdg/assay/app.pem"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="no HOME and no override is refused, not guessed"
env -i PATH="$stub_path" /bin/sh "$run" worker-desk --dry-run >"$tmp/out" 2>"$tmp/err"
rc=$?
cat "$tmp/out" "$tmp/err" >>"$all"
if [ "$rc" -eq 3 ]; then pass "$name"; else fail "$name" "rc=$rc"; fi

name="a PEM path that is a directory is refused"
mkdir -p "$creds/adir"
drun ASSAY_DESK_PEM_FILE="$creds/adir" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && has "$tmp/err" 'not a regular file'; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="an unreadable PEM is refused"
if [ "$(id -u)" -eq 0 ]; then
  printf 'SKIP (could-not-check): %s: root reads any file\n' "$name"
else
  cp "$pem" "$creds/locked.pem"
  chmod 000 "$creds/locked.pem"
  drun ASSAY_DESK_PEM_FILE="$creds/locked.pem" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" worker-desk --dry-run
  if [ "$rc" -eq 3 ] && has "$tmp/err" 'not readable'; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi
fi

name="a PEM path with a comma is refused, not passed to --mount"
cp "$pem" "$creds/a,b.pem"
drun ASSAY_DESK_PEM_FILE="$creds/a,b.pem" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" worker-desk --dry-run
if [ "$rc" -eq 3 ] && ! has "$tmp/out" 'docker'; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

# --- no credential value in any output ---------------------------------------
name="dry-run prints credential paths, never contents (row 6: BEGIN count is 0)"
desk worker-desk --dry-run
n=$(grep -c -F -- "$begin" "$tmp/out" || [ $? -eq 1 ])
if [ "$rc" -eq 0 ] && [ "$n" = "0" ]; then pass "$name"; else fail "$name" "rc=$rc count=$n"; fi

# --- --attach ----------------------------------------------------------------
name="--attach --dry-run prints docker start -ai and needs no credentials"
: >"$log"
drun /bin/sh "$run" worker-desk --attach --dry-run
if [ "$rc" -eq 0 ] && [ "$(cat "$tmp/out")" = "docker start -ai worker-desk" ] && [ "$(docker_calls)" -eq 0 ]; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="--attach with --version is refused"
desk worker-desk --attach --version v1
if [ "$rc" -eq 2 ]; then pass "$name"; else fail "$name" "rc=$rc"; fi

# --- real-run path against the stub docker -----------------------------------
name="a real run execs the same docker run that --dry-run prints"
: >"$log"
desk the-desk --version v9.9.9
want="run $(expect the-desk v9.9.9 | sed 's/^docker //; s/^run //')"
if [ "$rc" -eq 0 ] && grep -q -x -F -- "$want" "$log"; then pass "$name"; else fail "$name" "rc=$rc log: $(cat "$log")"; fi

name="a real run refuses when a container of that name already exists"
: >"$log"
drun ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$envf" STUB_EXISTING=worker-desk /bin/sh "$run" worker-desk
if [ "$rc" -eq 4 ] && has "$tmp/err" '--attach' && ! grep -q '^run ' "$log"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="a name-prefix match does not count as an existing container"
: >"$log"
drun ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$envf" STUB_EXISTING=worker-desk-2 /bin/sh "$run" worker-desk
if [ "$rc" -eq 0 ] && grep -q '^run ' "$log"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="--attach re-attaches to the existing container"
: >"$log"
drun STUB_EXISTING=worker-desk /bin/sh "$run" worker-desk --attach
if [ "$rc" -eq 0 ] && grep -q -x 'start -ai worker-desk' "$log" && ! grep -q '^run ' "$log"; then pass "$name"; else fail "$name" "rc=$rc $(combined) log: $(cat "$log")"; fi

name="--attach with no such container is refused"
: >"$log"
drun /bin/sh "$run" worker-desk --attach
if [ "$rc" -eq 4 ] && ! grep -q '^start ' "$log"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="an unreachable docker daemon is could-not-check, never a fresh start"
: >"$log"
drun ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$envf" STUB_LS_FAIL=1 /bin/sh "$run" worker-desk
if [ "$rc" -eq 4 ] && ! grep -q '^run ' "$log"; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="a missing docker binary is refused for a real run"
env -i HOME="$home" PATH="$tmp/emptybin" ASSAY_DESK_PEM_FILE="$pem" ASSAY_DESK_ENV_FILE="$envf" \
  /bin/sh "$run" worker-desk >"$tmp/out" 2>"$tmp/err"
rc=$?
cat "$tmp/out" "$tmp/err" >>"$all"
if [ "$rc" -eq 4 ] && has "$tmp/err" 'docker is not on PATH'; then pass "$name"; else fail "$name" "rc=$rc $(combined)"; fi

name="a real run never sees a refused credential (no docker call on preflight failure)"
: >"$log"
drun ASSAY_DESK_PEM_FILE="$creds/absent.pem" ASSAY_DESK_ENV_FILE="$envf" /bin/sh "$run" worker-desk
if [ "$rc" -eq 3 ] && [ "$(docker_calls)" -eq 0 ]; then pass "$name"; else fail "$name" "rc=$rc calls=$(docker_calls)"; fi

# --- the whole run: no fixture credential value anywhere ---------------------
name="no output line of any case contains a fixture credential value"
if ! grep -F -q -- "$sent_pem" "$pem" || ! grep -F -q -- "$sent_env" "$envf"; then
  fail "$name" "positive control: the fixtures do not hold their sentinels"
elif [ ! -s "$all" ]; then
  fail "$name" "positive control: no output was captured"
elif grep -F -q -e "$sent_pem" -e "$sent_env" "$all"; then
  fail "$name" "a sentinel appears in the captured output"
else
  pass "$name"
fi

if [ "$failures" -ne 0 ]; then
  printf '%s case(s) FAILED\n' "$failures"
  exit 1
fi
echo "all cases passed"
