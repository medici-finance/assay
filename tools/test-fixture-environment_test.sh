#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$root/tools/test-fixture-environment.sh"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/fixture-environment-controls.XXXXXX")"
trap 'rm -rf -- "$scratch"' EXIT
git_bin="$(command -v git)"; python_bin="$(command -v python3)"
mkdir -p "$scratch/repo/statusgen" "$scratch/repo/tools/desk"

for mode in healthy git-global git-repo installed-tool suite-failure absent-execution; do
  case_root="$scratch/$mode"
  mkdir -p "$case_root"
  fake_go="$case_root/fake-go"
  cat > "$fake_go" <<SH
#!/bin/bash
set -eu
test "\$*" = 'test -json -count=1 -timeout=20m ./...'
test "\${GIT_CONFIG_COUNT:-}" = 2
test "\${GIT_CONFIG_KEY_0:-}" = grep.patternType
test "\${GIT_CONFIG_VALUE_0:-}" = default
test "\${GIT_NO_REPLACE_OBJECTS:-}" = 1
test "\${GH_TOKEN+x}" != x
test "\${GITHUB_TOKEN+x}" != x
test "\${GITLAB_TOKEN+x}" != x
printf '%s\\n' "\${PWD##*/}" >> '$case_root/invocations'
case '$mode' in
  git-global) git config --global fixture.leaked yes ;;
  git-repo) git -C '$case_root/victim' config fixture.leaked yes ;;
  installed-tool) env -i "PATH=\$PATH" deskdecision || true ;;
  suite-failure) exit 8 ;;
  absent-execution) exit 0 ;;
esac
printf '%s\\n' '{"Action":"run","Package":"fixture.example/module","Test":"TestFixtureControl"}' '{"Action":"pass","Package":"fixture.example/module"}'
SH
  chmod +x "$fake_go"
  fixture_prepare "$scratch/repo" "$case_root" "$fake_go" "$git_bin" "$python_bin" "$case_root/cache" "$case_root/modcache" "$case_root/gopath"
  failed=0
  if ! fixture_run_module statusgen > "$case_root/result" 2>&1; then failed=1; fi
  if ! fixture_run_module tools/desk >> "$case_root/result" 2>&1; then failed=1; fi
  test "$(wc -l < "$case_root/invocations" | tr -d ' ')" = 2
  if [ "$mode" = healthy ]; then
    test "$failed" = 0 || { cat "$case_root/result"; exit 1; }
  else
    test "$failed" = 1 || { echo "control did not detect $mode" >&2; exit 1; }
  fi
  echo "PASS: $mode"
done

# A caller may name owned scratch through a symlink (Darwin's /tmp is one).
# Fixtures must see physical paths, matching Git/native path witnesses.
canonical="$scratch/canonical"
mkdir -p "$canonical"
ln -s "$canonical" "$scratch/scratch-alias"
fixture_prepare "$scratch/repo" "$scratch/scratch-alias" /bin/true "$git_bin" "$python_bin" "$canonical/cache" "$canonical/modcache" "$canonical/gopath"
physical="$(cd -- "$canonical" && pwd -P)"
test "$fixture_scratch" = "$physical"
printf '%s\n' "${fixture_env[@]}" | grep -Fx "TMPDIR=$physical/tmp" >/dev/null
printf '%s\n' "${fixture_env[@]}" | grep -Fx "HOME=$physical/home" >/dev/null
echo 'PASS: physical owned scratch paths'

# Demonstrate credentials are explicit, rather than asserting only absence.
case_root="$scratch/credentials"
mkdir -p "$case_root"
fixture_prepare "$scratch/repo" "$case_root" /bin/true "$git_bin" "$python_bin" "$case_root/cache" "$case_root/modcache" "$case_root/gopath"
if env -i "${fixture_env[@]}" /bin/sh -c 'test -n "${GH_TOKEN:-}"'; then
  echo 'control did not detect inherited credential absence' >&2; exit 1
fi
env -i "${fixture_env[@]}" 'GH_TOKEN=synthetic-fixture-value' /bin/sh -c 'test "$GH_TOKEN" = synthetic-fixture-value'
echo 'PASS: explicit credential presence and absence'

# The actual public entry point must still launch the second complete module
# after a first-module failure. The fake only substitutes Go; no runner branch
# or selected-package flag exists for this control.
entry="$scratch/entry"
mkdir -p "$entry/bin" "$entry/cache" "$entry/modcache" "$entry/gopath"
ln -s "$git_bin" "$entry/bin/git"
ln -s "$python_bin" "$entry/bin/python3"
cat > "$entry/bin/go" <<SH
#!/bin/bash
set -eu
if [ "\$1" = env ]; then
  case "\$2" in
    GOCACHE) echo '$entry/cache' ;;
    GOMODCACHE) echo '$entry/modcache' ;;
    GOPATH) echo '$entry/gopath' ;;
    *) exit 2 ;;
  esac
  exit
fi
test "\$*" = 'test -json -count=1 -timeout=20m ./...'
test "\${GH_TOKEN+x}" != x
printf '%s\\n' "\${PWD##*/}" >> '$entry/invocations'
exit 8
SH
chmod +x "$entry/bin/go"
if env -i "PATH=$entry/bin:/usr/bin:/bin" "HOME=$scratch" "GH_TOKEN=synthetic-parent-value" \
    /bin/bash "$root/tools/test-fixture-environment.sh" > "$entry/result" 2>&1; then
  echo 'public entry accepted failed module suites' >&2; exit 1
fi
test "$(wc -l < "$entry/invocations" | tr -d ' ')" = 2
grep -Fx statusgen "$entry/invocations" >/dev/null
grep -Fx desk "$entry/invocations" >/dev/null
echo 'PASS: public entry executes both complete modules after failure'
