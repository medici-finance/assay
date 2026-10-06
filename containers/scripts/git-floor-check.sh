#!/bin/sh
# git-floor-check.sh — prove a git meets the verifier-admission floor.
#
# Verifier admission renders a converted file (an `eol` or
# `working-tree-encoding` attribute) from the ATTESTED commit's own attributes,
# with `git --attr-source=<commit>`. That option arrived in git 2.41. An older
# git cannot render such a file, so admission refuses it, by design. This
# repository ships `*.ps1` and `*.psm1` with `eol=crlf`, so a verifier home on a
# git older than 2.41 is refused for every brief here (#2318).
#
# This script checks the floor three ways, and all must pass:
#   1. `git version` reports 2.41 or later.
#   2. Every OTHER `git` reachable on PATH reports 2.41 or later too, so an
#      older distro git left behind a newer one cannot be reached by a process
#      whose PATH puts it first.
#   3. Behaviour: in a scratch repository whose ONLY attributes live in the
#      committed tree (none in the work tree or the index), rendering a file
#      with `--attr-source=HEAD` applies the committed `eol=crlf`, and rendering
#      it without the option does not. A git that rejects the option fails
#      here, and so does one that accepts it and ignores it.
#
# Usage:
#   git-floor-check.sh               check the git on PATH
#   git-floor-check.sh <image-ref>   run this same check inside <image-ref>
#
# containers/base/Dockerfile runs the first form at build time, so an image
# below the floor does not build. The second form checks an image that has
# already been built or pulled, and needs docker.
#
# Exit: 0 floor met; 1 floor NOT met (git absent, too old, or rejects or
# ignores --attr-source); 2 could-not-check (no docker, image not runnable,
# unreadable `git version`, no scratch directory). Could-not-check is never
# reported as a pass.
#
# Proof that the check fires: tools/desk/internal/deskkit/gitfloor_test.go runs
# this script against stand-in gits that are too old, that reject the option,
# and that silently ignore it, and requires each one to fail.
set -u

FLOOR_MAJOR=2
FLOOR_MINOR=41

fail() {
  echo "FAIL: git floor ${FLOOR_MAJOR}.${FLOOR_MINOR} not met: $*" >&2
  exit 1
}

cnc() {
  echo "COULD-NOT-CHECK: $*" >&2
  exit 2
}

if [ "$#" -gt 1 ]; then
  echo "usage: git-floor-check.sh [<image-ref>]" >&2
  exit 2
fi

# ---- image form: run this script inside the image ---------------------------
if [ "$#" -eq 1 ]; then
  ref=$1
  command -v docker >/dev/null 2>&1 || cnc "docker is not on PATH, so ${ref} was not checked"
  # --entrypoint overrides a desk image's boot entrypoint, which would demand
  # runtime credentials before it ran anything.
  docker run --rm -i --entrypoint /bin/sh "$ref" -s <"$0"
  rc=$?
  case "$rc" in
    0)
      echo "OK: ${ref} meets the git floor"
      exit 0
      ;;
    1) exit 1 ;;
    *) cnc "could not run the check inside ${ref} (exit ${rc})" ;;
  esac
fi

# ---- local form: check the git on PATH --------------------------------------

command -v git >/dev/null 2>&1 || fail "git is not on PATH"

# Inherited GIT_* variables (GIT_DIR, GIT_ATTR_SOURCE, GIT_CONFIG_PARAMETERS, …)
# would point the probe somewhere else or change what it renders.
for v in $(env | sed -n 's/^\(GIT_[A-Za-z0-9_]*\)=.*/\1/p'); do
  unset "$v"
done
GIT_CONFIG_NOSYSTEM=1
GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM GIT_CONFIG_GLOBAL

# check_version <git>: fail or could-not-check unless <git> reports the floor.
check_version() {
  vline=$("$1" version 2>&1) || fail "\`$1 version\` failed: ${vline}"
  ver=$(printf '%s\n' "$vline" | sed -n '1s/^git version \([0-9][0-9]*\)\.\([0-9][0-9]*\).*/\1 \2/p')
  [ -n "$ver" ] || cnc "unrecognised \`$1 version\` output: ${vline}"
  major=${ver% *}
  minor=${ver#* }
  if [ "$major" -lt "$FLOOR_MAJOR" ] || { [ "$major" -eq "$FLOOR_MAJOR" ] && [ "$minor" -lt "$FLOOR_MINOR" ]; }; then
    fail "$1: ${vline} is older than ${FLOOR_MAJOR}.${FLOOR_MINOR} (--attr-source arrived in 2.41)"
  fi
}

check_version git
line=$vline

# Every other git on PATH. An empty PATH entry means the current directory.
oldifs=$IFS
IFS=:
# shellcheck disable=SC2086 # split PATH on ':' on purpose
set -- $PATH
IFS=$oldifs
for d in "$@"; do
  [ -n "$d" ] || d=.
  if [ -f "$d/git" ] && [ -x "$d/git" ]; then
    check_version "$d/git"
  fi
done

tmp=$(mktemp -d 2>/dev/null) || cnc "cannot create a scratch directory"
trap 'rm -rf "$tmp"' EXIT
trap 'exit 2' HUP INT TERM
repo=$tmp/repo

g() {
  git -c core.autocrlf=false -c core.eol=lf -c core.attributesFile=/dev/null -C "$repo" "$@"
}

git init -q "$repo" >/dev/null 2>&1 || cnc "cannot init a scratch repository"
printf '*.ps1 text eol=crlf\n' >"$repo/.gitattributes"
printf 'one\ntwo\n' >"$repo/a.ps1"
g add .gitattributes a.ps1 >/dev/null 2>&1 || cnc "cannot stage the scratch files"
g -c user.name=git-floor-check -c user.email=git-floor-check@example.invalid \
  commit -q -m probe >/dev/null 2>&1 || cnc "cannot commit in the scratch repository"

# Leave the attributes ONLY in the commit. A checkout-direction render reads the
# work tree's .gitattributes and falls back to the index's; with both gone, the
# committed tree is the only place eol=crlf can come from.
g rm -q --cached .gitattributes >/dev/null 2>&1 || cnc "cannot drop .gitattributes from the index"
rm -f "$repo/.gitattributes"

out=$(g --attr-source=HEAD version 2>&1) || fail "\`git --attr-source=HEAD version\` was rejected: ${out}"

printf 'one\r\ntwo\r\n' >"$tmp/want-crlf"
printf 'one\ntwo\n' >"$tmp/want-lf"

# Control: without --attr-source nothing asks for CRLF. If it renders CRLF
# anyway, the probe cannot tell the option apart from something else.
g cat-file --filters --path=a.ps1 HEAD:a.ps1 >"$tmp/plain" 2>"$tmp/err" ||
  cnc "cannot render a.ps1 without --attr-source: $(cat "$tmp/err")"
cmp -s "$tmp/plain" "$tmp/want-lf" ||
  cnc "a.ps1 rendered without --attr-source is not plain LF, so the probe cannot isolate the option"

g --attr-source=HEAD cat-file --filters --path=a.ps1 HEAD:a.ps1 >"$tmp/rendered" 2>"$tmp/err" ||
  fail "rendering a.ps1 with --attr-source=HEAD failed: $(cat "$tmp/err")"
cmp -s "$tmp/rendered" "$tmp/want-crlf" ||
  fail "--attr-source=HEAD did not apply the committed eol=crlf to a.ps1 (the option was accepted and ignored)"

echo "OK: ${line} renders a converted file from --attr-source (floor ${FLOOR_MAJOR}.${FLOOR_MINOR})"
exit 0
