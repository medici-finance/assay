#!/bin/bash
# shellcheck shell=bash
# stage-source.sh: the stage Job's init container `source`, the ONLY container that mounts the
# source credential. Baked into the verify-runner image at /usr/local/bin/stage-source.sh,
# root-owned and mode 0755; the Job runs it as its single-element command.
#
#   stage-source.sh --repo=<owner/repo> --sha=<40-hex sha>   fetch one commit into /cache/clone
#   stage-source.sh --probe                                  make the probe directories, print
#                                                            this process's confinement, read
#                                                            no credential
#
# With no argument, or any other argument list, it prints its usage on stderr and exits 2
# before it reads or writes anything.
#
# Fetches exactly one commit of exactly one repository over HTTPS from github.com through a
# credential helper that answers only for that host and that repository path, and only before
# the token's recorded expiry. The token is read by the helper from the mounted file at the
# moment git asks. It is never in argv, in the environment, in a URL, in a Git config file, or
# in the clone.
set -u

usage() {
  printf 'usage: stage-source.sh --repo=<owner/repo> --sha=<40-hex sha>\n       stage-source.sh --probe\n' >&2
  exit 2
}
die() { printf 'stage-source: %s\n' "$*" >&2; exit 1; }

# The argument list is checked before anything is read or written.
MODE=
REPO=
SHA=
case $# in
  1) [ "$1" = --probe ] || usage; MODE=probe ;;
  2)
    case $1 in --repo=*) ;; *) usage ;; esac
    case $2 in --sha=*) ;; *) usage ;; esac
    MODE=run; REPO=${1#--repo=}; SHA=${2#--sha=}
    [[ $REPO =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]] || usage
    case /$REPO/ in */./*|*/../*) usage ;; esac
    [[ $SHA =~ ^[0-9a-f]{40}$ ]] || usage
    ;;
  *) usage ;;
esac

# The three VR_* names are test hooks. The Job template gives this container no env at all, so
# in a Job these are the defaults.
CRED=${VR_CRED_DIR:-/run/source-credential}
CACHE=${VR_CACHE_DIR:-/cache}
CLONE=$CACHE/clone
TMPB=${VR_TMP_DIR:-/tmp}
WORK=$TMPB/vr-src
HOMEDIR=$TMPB/vr-home
HELPERDIR=$TMPB/vr-helper

# The credential helper. git calls it as `<helper> <cred-dir> <owner/repo> <operation>` and
# writes the credential request (protocol=, host=, path=) on stdin. Only `get` for HTTPS
# github.com (or github.com:443) and exactly <owner/repo> or <owner/repo>.git answers, and only
# while the recorded expiry is in the future. store, erase and anything else are refused.
write_helper() {
  local dir=$1
  mkdir -p "$dir" || return 1
  cat > "$dir/helper" <<'HELPER'
#!/bin/bash
set -u
cred=${1:-}; repo=${2:-}; op=${3:-}
[ "$op" = get ] || exit 1
[ -n "$cred" ] && [ -n "$repo" ] || exit 1
protocol=; host=; path=
while IFS='=' read -r k v; do
  [ -n "$k" ] || break
  case $k in
    protocol) protocol=$v ;;
    host) host=$v ;;
    path) path=$v ;;
  esac
done
[ "$protocol" = https ] || exit 1
case $host in github.com|github.com:443) ;; *) exit 1 ;; esac
case $path in "$repo"|"$repo.git") ;; *) exit 1 ;; esac
exp=$(cat "$cred/expires-at" 2>/dev/null) || exit 1
[[ $exp =~ ^[0-9]{9,12}$ ]] || exit 1
now=$(date +%s)
[ "$now" -lt "$exp" ] || exit 1
tok=$(cat "$cred/token" 2>/dev/null) || exit 1
[ "${#tok}" -le 2048 ] && [[ $tok =~ ^[A-Za-z0-9_.-]+$ ]] || exit 1
printf 'username=x-access-token\npassword=%s\n' "$tok"
HELPER
  chmod 755 "$dir/helper"
}

if [ "$MODE" = probe ]; then
  # the probe variant: the directories the witness probe's sub-path mounts need, and one line
  mkdir -p "$CLONE" "$CACHE/mod" 2>/dev/null
  seccomp=$(awk '$1 == "Seccomp:" { print $2; exit }' /proc/self/status 2>/dev/null)
  apparmor=
  if read -r first _ < /proc/self/attr/current 2>/dev/null; then apparmor=$first; fi
  printf 'stage seccomp=%s apparmor=%s\n' "${seccomp:-unknown}" "${apparmor:-unknown}"
  exit 0
fi

# --- custody: a missing, malformed or expired credential fails before any clone ---------------
for f in token issued-at expires-at; do
  [ -r "$CRED/$f" ] || die "source custody is missing ($f): no clone was attempted"
done
TOK=$(cat "$CRED/token")
IAT=$(cat "$CRED/issued-at")
EXP=$(cat "$CRED/expires-at")
[ "${#TOK}" -le 2048 ] && [[ $TOK =~ ^[A-Za-z0-9_.-]+$ ]] || die "source custody is malformed (token): no clone was attempted"
[[ $IAT =~ ^[0-9]{9,12}$ ]] || die "source custody is malformed (issued-at): no clone was attempted"
[[ $EXP =~ ^[0-9]{9,12}$ ]] || die "source custody is malformed (expires-at): no clone was attempted"
NOW=$(date +%s)
[ "$EXP" -gt "$NOW" ] || die "source custody expired at $EXP: no clone was attempted"
[ "$IAT" -le "$((NOW + 300))" ] || die "source custody issued in the future: no clone was attempted"
[ "$((EXP - IAT))" -le 3600 ] || die "source custody outlives one hour: no clone was attempted"
TOK=

# --- a clean Git world: no system or global config, no prompt, no askpass, no hooks -----------
# Every inherited GIT_* variable goes (trace and curl-verbose output, GIT_SSL_NO_VERIFY,
# GIT_DIR, askpass and config injection alike), with the proxy variables and the startup files
# a child bash or sh would read (the helper is a bash script git starts).
rm -rf "$WORK" "$HOMEDIR" "$HELPERDIR"
mkdir -p "$HOMEDIR" || die "no scratch space"
while IFS= read -r v; do
  case $v in GIT_*) unset "$v" ;; esac
done < <(compgen -e)
unset SSH_ASKPASS BASH_ENV ENV \
  http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY no_proxy NO_PROXY
export HOME=$HOMEDIR
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never
write_helper "$HELPERDIR" || die "could not write the helper"

git init -q "$WORK" || die "git init failed"
cd "$WORK" || die "no scratch tree"
git -c core.hooksPath=/dev/null \
  -c protocol.allow=never -c protocol.https.allow=always \
  -c http.followRedirects=false -c credential.useHttpPath=true \
  -c credential.helper= -c "credential.helper=!$HELPERDIR/helper $CRED $REPO" \
  -c submodule.recurse=false -c fetch.recurseSubmodules=false \
  fetch -q --no-tags --no-recurse-submodules "https://github.com/$REPO.git" "$SHA" \
  || die "the fetch of the exact sha failed"
GOT=$(git rev-parse --verify 'FETCH_HEAD^{commit}') || die "no commit after the fetch"
[ "$GOT" = "$SHA" ] || die "the fetch returned a different commit"
git -c core.hooksPath=/dev/null -c advice.detachedHead=false checkout -q --detach "$SHA" \
  || die "checkout failed"
git update-ref refs/remotes/origin/main "$SHA" || die "could not pin refs/remotes/origin/main"
git remote add origin "https://github.com/$REPO.git" || die "could not record the origin"

# --- clear every helper and config artifact before anything reaches the PVC ------------------
rm -rf "$HELPERDIR" "$HOMEDIR"
LEFT=$(git config --local --list | grep -c -i -e credential -e token -e extraheader -e askpass)
[ "$LEFT" = 0 ] || die "the clone's Git config holds credential-shaped settings"
if grep -r -I -l -F -f "$CRED/token" "$WORK" >/dev/null 2>&1; then
  die "the credential value appears in the clone"
fi

[ ! -e "$CLONE" ] || die "the PVC already holds a clone: one Job stages one clone"
mkdir -p "$CLONE" || die "could not create the PVC clone directory"
cp -a "$WORK/." "$CLONE/" || die "could not write the clone to the PVC"
rm -rf "$WORK"
printf 'source-done %s\n' "$SHA"
