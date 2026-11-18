#!/usr/bin/env bash
# Full module tests under deliberately unfriendly caller state. No fixture may
# rely on this launcher to sanitize the inputs it is supposed to isolate.
set -euo pipefail

fixture_snapshot() {
  "$fixture_python" - "$1" <<'PY'
import hashlib, json, os, pathlib, stat, sys
root = pathlib.Path(sys.argv[1])
result = {}
for path in [root, *sorted(root.rglob('*'))]:
    mode = path.lstat().st_mode
    body = os.readlink(path).encode() if stat.S_ISLNK(mode) else path.read_bytes() if stat.S_ISREG(mode) else b''
    result[str(path.relative_to(root))] = [mode, hashlib.sha256(body).hexdigest()]
print(json.dumps(result, sort_keys=True))
PY
}

# Arguments are resolved by main before hostile state is installed. The shell
# controls source these functions with their own fake Go executable; the public
# command has no selected-package, fake-tool or ignore-failure option.
fixture_prepare() {
  fixture_root="$(cd -- "$1" && pwd -P)"
  fixture_scratch="$(cd -- "$2" && pwd -P)"
  fixture_go=$3 fixture_git=$4 fixture_python=$5
  local cache=$6 modcache=$7 gopath=$8 baseline=${9:-}
  local approved_bash=${10:-$(command -v bash)}
  if [[ "$approved_bash" != /* || ! -x "$approved_bash" ]] ||
      ! "$approved_bash" -c 'test "${BASH_VERSINFO[0]}" -ge 4'; then
    echo 'ERROR: fixture suites require a pre-resolved compatible Bash (4 or later)' >&2
    return 2
  fi
  if [[ "$fixture_python" != /* || ! -x "$fixture_python" ]] ||
      ! "$fixture_python" -c 'import sys; assert sys.version_info.major == 3'; then
    echo 'ERROR: fixture suites require a pre-resolved Python 3' >&2
    return 2
  fi
  mkdir -p "$fixture_scratch/home" "$fixture_scratch/home/.config" "$fixture_scratch/bin" "$fixture_scratch/tmp"
  printf '[fixture]\n\tcaller = unchanged\n' > "$fixture_scratch/global"
  : > "$fixture_scratch/system"
  cp "$fixture_scratch/global" "$fixture_scratch/global.before"
  ln -s "$fixture_go" "$fixture_scratch/bin/go"
  ln -s "$fixture_git" "$fixture_scratch/bin/git"
  ln -s "$approved_bash" "$fixture_scratch/bin/bash"
  ln -s "$fixture_python" "$fixture_scratch/bin/python3"
  # Preserve installed fixture capabilities before narrowing PATH. These
  # explicit runtime tools may live outside the fixed OS directories; omitting
  # them would manufacture missing-tool skips on a capable developer host.
  local optional_tool approved_tool
  for optional_tool in tmux shellcheck jq perl curl lsof; do
    if approved_tool="$(command -v "$optional_tool")"; then
      if [[ "$approved_tool" != /* || ! -x "$approved_tool" ]]; then
        echo "ERROR: fixture tool $optional_tool must resolve to an absolute executable" >&2
        return 2
      fi
      ln -s "$approved_tool" "$fixture_scratch/bin/$optional_tool"
    fi
  done
  # Approved interpreters override the fixed OS dependency directories below;
  # no caller PATH tail (or installed desk/forge tool directory) is inherited.
  # CI=true makes requireTool refuse missing required tools instead of skipping.
  local tool
  for tool in deskdecision deskdispatch desktoken gh glab; do
    "$fixture_python" - "$fixture_scratch/bin/$tool" "$fixture_scratch/decoys" "$tool" <<'PY'
import pathlib, shlex, sys
pathlib.Path(sys.argv[1]).write_text('#!/bin/sh\nprintf "%s\\n" ' + shlex.quote(sys.argv[3]) + ' >> ' + shlex.quote(sys.argv[2]) + '\nexit 97\n')
PY
    chmod +x "$fixture_scratch/bin/$tool"
  done
  fixture_env=(
    "PATH=$fixture_scratch/bin:/usr/bin:/bin:/usr/sbin:/sbin" "HOME=$fixture_scratch/home" "CI=true"
    "USERPROFILE=$fixture_scratch/home" "XDG_CONFIG_HOME=$fixture_scratch/home/.config"
    "TMPDIR=$fixture_scratch/tmp" "TMP=$fixture_scratch/tmp" "TEMP=$fixture_scratch/tmp"
    "GOCACHE=$cache" "GOMODCACHE=$modcache" "GOPATH=$gopath" "GOWORK=off"
    "GOTOOLCHAIN=local" "GOPROXY=off" "GOSUMDB=off" "GOFLAGS=-buildvcs=false"
    "GIT_CONFIG_GLOBAL=$fixture_scratch/global" "GIT_CONFIG_SYSTEM=$fixture_scratch/system"
    "GIT_CONFIG_NOSYSTEM=1" "GIT_NO_REPLACE_OBJECTS=1" "GIT_ATTR_NOSYSTEM=1" "GIT_TERMINAL_PROMPT=0" "GIT_ASKPASS="
    "GIT_CONFIG_COUNT=2" "GIT_CONFIG_KEY_0=grep.patternType" "GIT_CONFIG_VALUE_0=default"
    "GIT_CONFIG_KEY_1=grep.extendedRegexp" "GIT_CONFIG_VALUE_1=false"
    "KUBECONFIG=/dev/null" "LC_ALL=C" "TZ=UTC"
  )
  if [ -n "$baseline" ]; then fixture_env+=("SG_OLD_BIN=$baseline"); fi
  # Setup has a separately declared environment: the victim is not constructed
  # by the inherited fixture behavior this launcher intends to detect.
  env -i "PATH=/usr/bin:/bin" "HOME=$fixture_scratch/home" \
    "GIT_CONFIG_NOSYSTEM=1" "GIT_CONFIG_GLOBAL=$fixture_scratch/global" \
    "$fixture_git" -c maintenance.auto=false -c gc.auto=0 init -q "$fixture_scratch/victim"
  fixture_snapshot "$fixture_scratch/victim" > "$fixture_scratch/victim.before"
}

fixture_unchanged() {
  local failed=0
  if ! cmp -s "$fixture_scratch/global.before" "$fixture_scratch/global"; then
    echo 'ERROR: module tests modified caller global Git configuration' >&2; failed=1
  fi
  fixture_snapshot "$fixture_scratch/victim" > "$fixture_scratch/victim.after"
  if ! cmp -s "$fixture_scratch/victim.before" "$fixture_scratch/victim.after"; then
    echo 'ERROR: module tests modified caller victim repository' >&2; failed=1
  fi
  if [ -s "$fixture_scratch/decoys" ]; then
    echo 'ERROR: module tests selected an installed-command decoy' >&2; failed=1
  fi
  return "$failed"
}

fixture_run_module() {
  local module=$1 failed=0
  echo "fixture environment: $module: go test -json -count=1 -timeout=20m ./..."
  # pipefail preserves a failing test exit while JSON events expose progress;
  # a quiet Go package must not hide the next actionable failing fixture.
  if ! (cd "$fixture_root/$module" && env -i "${fixture_env[@]}" "$fixture_go" test -json -count=1 -timeout=20m ./...) \
      2>&1 | tee "$fixture_scratch/${module//\//-}.log"; then
    failed=1
  fi
  if ! "$fixture_python" - "$fixture_scratch/${module//\//-}.log" <<'PY'
import json, pathlib, sys
completed, started = set(), set()
for line in pathlib.Path(sys.argv[1]).read_text().splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    if event.get('Action') == 'pass' and event.get('Package') and not event.get('Test'):
        completed.add(event['Package'])
    if event.get('Action') == 'run' and event.get('Test'):
        started.add((event.get('Package'), event['Test']))
if not completed or not started:
    print('ERROR: required module test execution has no completed package/test events', file=sys.stderr)
    sys.exit(1)
print(f'fixture events: {len(completed)} packages completed, {len(started)} tests started')
PY
  then
    failed=1
  fi
  if ! fixture_unchanged; then failed=1; fi
  return "$failed"
}

fixture_main() (
  if [ "$#" -ne 0 ]; then echo 'usage: bash tools/test-fixture-environment.sh' >&2; return 2; fi
  local root go_bin git_bin python_bin bash_bin scratch cache modcache gopath baseline=${SG_OLD_BIN:-} failed=0
  root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
  go_bin="$(command -v go)"; git_bin="$(command -v git)"; python_bin="$(command -v python3)"; bash_bin="$(command -v bash)"
  for executable in "$go_bin" "$git_bin" "$python_bin" "$bash_bin"; do
    case "$executable" in /*) test -x "$executable" ;; *) echo 'ERROR: tools must resolve to absolute executable paths' >&2; return 2 ;; esac
  done
  cache="$("$go_bin" env GOCACHE)"; modcache="$("$go_bin" env GOMODCACHE)"; gopath="$("$go_bin" env GOPATH)"
  if [ -n "$baseline" ]; then
    case "$baseline" in /*) test -x "$baseline" ;; *) echo 'ERROR: SG_OLD_BIN must be an absolute prepared baseline' >&2; return 2 ;; esac
  fi
  scratch="$(mktemp -d "${TMPDIR:-/tmp}/fixture-environment.XXXXXX")"
  scratch="$(cd -- "$scratch" && pwd -P)"
  # The trap removes only the directory this invocation just created.
  trap 'rm -rf -- "$scratch"' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  fixture_prepare "$root" "$scratch" "$go_bin" "$git_bin" "$python_bin" "$cache" "$modcache" "$gopath" "$baseline" "$bash_bin"
  # No early-success path: both complete module invocations are owed even when
  # the first fails. env -i withholds every ambient forge credential. Fixtures
  # requiring credential presence declare their own synthetic value in tests.
  if ! fixture_run_module statusgen; then failed=1; fi
  if ! fixture_run_module tools/desk; then failed=1; fi
  return "$failed"
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then fixture_main "$@"; fi
