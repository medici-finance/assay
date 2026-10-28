#!/bin/bash
# shellcheck shell=bash
# witness.sh: the one fixed entrypoint of the witness Job, for its init container `seed` and its
# container `witness`. Baked into the verify-runner image at /usr/local/bin/witness.sh,
# root-owned and mode 0755. Neither container sets a command: each one's args are this path and
# exactly one word.
#
#   witness.sh seed              init container: copy the clone from /cache/clone into /work
#   witness.sh seed-probe        init container of the probe variant: an empty /work
#   witness.sh <stream>/<NN>     the whole run of the one brief /work/docs/streams/<stream>/
#                                brief-<NN>-<slug>.md (exactly one such regular file, else exit 2)
#   witness.sh probe             the probe measurements
#
# With no argument, or any other argument list, it prints its usage on stderr and exits 2
# before it reads or writes anything.
#
# A run is the container half of statusgen's verification bracket. The host opens the bracket
# (`statusgen verifyrun --open-bracket`) and hands its receipt in, with the digest of the image
# this container runs, as two read-only files under /run/verify-admission: receipt.json and
# image-digest. Without both, or with a digest that is not sha256:<64 hex>, the run exits 2 and
# no row runs. statusgen itself then refuses any mounted tree that does not digest to the
# receipt's tree. The witness it writes is printed on the last line of the log, base64 on one
# line after `vr-witness `, for the host to close the bracket with.
#
# The run enters the empty network namespace as ONE process tree: this script records its own
# namespace id, then runs
#   unshare --user --map-root-user --net statusgen __verifyrun-netns <outer-id> \
#     statusgen verifyrun --sandbox container-netns --brief <path> --root /work \
#       --admission-receipt <receipt> --container-image-digest <digest> --witness <file>
# statusgen's own helper raises lo inside the new namespace (a new namespace starts with lo
# DOWN) and proves lo is up, no other interface is up and the namespace differs from <outer-id>,
# and only then execs verifyrun. Every boundary failure is terminal: there is no unsandboxed
# retry and no lo-less retry, and no row command runs before the helper has approved.
set -u

usage() {
  printf 'usage: witness.sh seed | seed-probe | probe | <stream>/<NN>\n' >&2
  exit 2
}
die() { printf 'witness: %s\n' "$*" >&2; exit 2; }

# The argument list is checked before anything is read or written.
KEY_RE='^[a-z0-9-]+/[0-9]{2,}$'
[ $# -eq 1 ] || usage
case $1 in
  seed|seed-probe|probe) ;;
  *) [[ $1 =~ $KEY_RE ]] || usage ;;
esac

# VR_CACHE_DIR, VR_WORK_DIR, VR_ADMISSION_DIR and VR_SCRATCH_DIR are test hooks; the Job's env
# allowlist never sets them
CACHE=${VR_CACHE_DIR:-/cache}
WORK=${VR_WORK_DIR:-/work}
ADMIT=${VR_ADMISSION_DIR:-/run/verify-admission}
SCRATCH=${VR_SCRATCH_DIR:-/tmp}
FX=$SCRATCH/vr-fx
WITNESS=$SCRATCH/vr-witness.md
NETNS_RE='^net:\[[0-9]+\]$'
BASE_RE='^brief-[0-9]{2,}-[A-Za-z0-9._-]+[.]md$'
DIGEST_RE='^sha256:[0-9a-f]{64}$'

# netns_run <command...>: <command> as one process tree in a new network namespace, behind
# statusgen's lo-raising helper. Its exit status is the command's, or the boundary's refusal.
netns_run() {
  local outer
  outer=$(readlink /proc/self/ns/net) || die "cannot read this process's network namespace"
  [[ $outer =~ $NETNS_RE ]] || die "unreadable network namespace id: $outer"
  unshare --user --map-root-user --net \
    statusgen __verifyrun-netns "$outer" \
    "$@"
}

# --- the probe's fixture: one row of each class, each recording what the row itself sees -----
# A container-netns run gives every row class the one namespace of its process tree, so the
# probe runs both rows through netns_run itself: the probe has no host receipt to run under.
write_fixture() {
  rm -rf "$FX" && mkdir -p "$FX/out" || return 1
  cat > "$FX/probe.go" <<'GO'
package main

import (
	"fmt"
	"net"
)

func main() {
	iface, lo := "loopback-only", "down"
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		up := i.Flags&net.FlagUp != 0
		if i.Flags&net.FlagLoopback != 0 {
			if up {
				lo = "up"
			}
			continue
		}
		if up {
			iface = "other"
		}
	}
	lis := "refused"
	if l, err := net.Listen("tcp", "127.0.0.1:0"); err == nil {
		go func() {
			if c, err := l.Accept(); err == nil {
				c.Close()
			}
		}()
		if c, err := net.Dial("tcp", l.Addr().String()); err == nil {
			c.Close()
			lis = "ok"
		}
		l.Close()
	}
	fmt.Printf("iface=%s lo=%s listener=%s\n", iface, lo, lis)
}
GO
  cat > "$FX/row.sh" <<'ROW'
#!/bin/sh
# row.sh <fixture dir> <class>: record what a row of that class sees inside the run's namespace
fx=$1
outer=$(cat "$fx/outer" 2>/dev/null)
me=$(readlink /proc/self/ns/net)
ns=differs
[ -n "$outer" ] && [ "$me" != "$outer" ] || ns=same
res=$(go run "$fx/probe.go" 2>/dev/null) || res="iface=other lo=down listener=refused"
echo "ns=$ns $res" > "$fx/out/$2.txt"
exit 0
ROW
}

# field <file> <key>: the value of key= in the one-line record, or the empty string
field() { tr ' ' '\n' < "$1" 2>/dev/null | sed -n "s/^$2=//p" | head -n 1; }

# class_state <class>: sets C_NS (loopback-only|other), C_LO (up|down), C_LIS (ok|refused)
class_state() {
  local f=$FX/out/$1.txt ns iface
  C_NS=other; C_LO=down; C_LIS=refused
  [ -f "$f" ] || return 0
  ns=$(field "$f" ns); iface=$(field "$f" iface)
  [ "$ns" = differs ] && [ "$iface" = loopback-only ] && C_NS=loopback-only
  [ "$(field "$f" lo)" = up ] && C_LO=up
  [ "$(field "$f" listener)" = ok ] && C_LIS=ok
  return 0
}

probe() {
  local dns egress api token unsh seccomp apparmor first
  # the OUTER pod, before the inner wrapper; the targets are named before any is contacted
  printf 'probe-targets dns=proxy.golang.org egress=1.1.1.1:443 apiserver=10.96.0.1:443\n'
  if timeout 5 getent hosts proxy.golang.org >/dev/null 2>&1; then dns=ok; else dns=refused; fi
  if timeout 5 bash -c 'exec 3<>/dev/tcp/1.1.1.1/443' >/dev/null 2>&1; then egress=open; else egress=refused; fi
  if timeout 5 bash -c 'exec 3<>/dev/tcp/10.96.0.1/443' >/dev/null 2>&1; then api=open; else api=refused; fi
  if [ -e /var/run/secrets/kubernetes.io/serviceaccount/token ]; then token=present; else token=absent; fi
  if unshare --user --map-root-user --net true >/dev/null 2>&1; then unsh=ok; else unsh=refused; fi
  seccomp=$(awk '$1 == "Seccomp:" { print $2; exit }' /proc/self/status 2>/dev/null)
  apparmor=
  if read -r first _ < /proc/self/attr/current 2>/dev/null; then apparmor=$first; fi
  printf 'probe dns=%s egress=%s apiserver=%s token=%s unshare=%s seccomp=%s apparmor=%s\n' \
    "$dns" "$egress" "$api" "$token" "$unsh" "${seccomp:-unknown}" "${apparmor:-unknown}"

  # the run's namespace boundary, with one fixture row of each class
  write_fixture >/dev/null 2>&1 || die "could not write the probe fixture"
  readlink /proc/self/ns/net > "$FX/outer" || die "cannot read this process's network namespace"
  # shellcheck disable=SC2016 # $1 is the inner shell's argument, the fixture directory
  ( netns_run sh -c 'sh "$1/row.sh" "$1" ordinary; sh "$1/row.sh" "$1" ci' row "$FX" ) > "$FX/run.log" 2>&1
  class_state ordinary; o_ns=$C_NS; o_lo=$C_LO; o_lis=$C_LIS
  class_state ci; c_ns=$C_NS; c_lo=$C_LO; c_lis=$C_LIS
  w_ns=other; [ "$o_ns" = loopback-only ] && [ "$c_ns" = loopback-only ] && w_ns=loopback-only
  w_lo=down; [ "$o_lo" = up ] && [ "$c_lo" = up ] && w_lo=up
  w_lis=refused; [ "$o_lis" = ok ] && [ "$c_lis" = ok ] && w_lis=ok
  printf 'whole-netns=%s lo=%s listener=%s ordinary-check-netns=%s ordinary-check-listener=%s ci-netns=%s ci-listener=%s\n' \
    "$w_ns" "$w_lo" "$w_lis" "$o_ns" "$o_lis" "$c_ns" "$c_lis"
}

# resolve_key <stream>/<NN>: sets BRIEF to the one regular brief file of that key under $WORK.
# No directory on the way to it may be a symlink.
resolve_key() {
  local stream=${1%/*} nn=${1#*/} d f n=0
  BRIEF=
  for d in "$WORK/docs" "$WORK/docs/streams" "$WORK/docs/streams/$stream"; do
    [ ! -L "$d" ] || die "$d is a symlink: the brief must be a file of the clone itself"
  done
  for f in "$WORK/docs/streams/$stream/brief-$nn-"*.md; do
    [ -f "$f" ] && [ ! -L "$f" ] || continue
    [[ ${f##*/} =~ $BASE_RE ]] || continue
    n=$((n + 1)); BRIEF=$f
  done
  [ "$n" = 1 ] || die "$n brief files match $1 (need exactly one regular file docs/streams/$stream/brief-$nn-<slug>.md)"
}

# admission: sets RECEIPT and DIGEST from the host's hand-over, or exits 2 before any row runs
admission() {
  RECEIPT=$ADMIT/receipt.json
  [ -f "$RECEIPT" ] && [ -r "$RECEIPT" ] || die "no admission receipt at $RECEIPT: the host has not opened the bracket, no row ran"
  [ -f "$ADMIT/image-digest" ] && [ -r "$ADMIT/image-digest" ] || die "no image digest at $ADMIT/image-digest: no row ran"
  DIGEST=$(cat "$ADMIT/image-digest") || die "unreadable image digest: no row ran"
  [[ $DIGEST =~ $DIGEST_RE ]] || die "the image digest is not sha256:<64 hex>: no row ran"
}

# --- dispatch ---------------------------------------------------------------------------------
case $1 in
  seed)
    [ -d "$CACHE/clone/.git" ] || die "no clone on the PVC"
    cp -a "$CACHE/clone/." "$WORK/" || die "could not copy the clone"
    ;;
  seed-probe)
    ;;
  probe)
    probe
    ;;
  *)
    resolve_key "$1"
    admission
    netns_run statusgen verifyrun --sandbox container-netns --brief "$BRIEF" --root "$WORK" \
      --admission-receipt "$RECEIPT" --container-image-digest "$DIGEST" --witness "$WITNESS"
    rc=$?
    if [ -f "$WITNESS" ]; then
      printf 'vr-witness %s\n' "$(base64 < "$WITNESS" | tr -d '\n')"
    fi
    exit "$rc"
    ;;
esac
