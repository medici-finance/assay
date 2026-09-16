#!/usr/bin/env bash
# root-drift.test.sh — deskd preconditions are gated on DESKD, and CELL_ROOTS stream-root drift is
# reported by `check` (non-fatal) and optionally healed at `desk` boot.
#
# What it proves (each an `assert` below):
#   check  a k8s/github cell with DESKD=0 reports the App key + orgs rows n/a (they exist only to
#          MINT deskd's per-org tokens, and deskd_mint_github is reachable only from `cellctl
#          deskd`) — and the same cell with DESKD=1 FAILS on them, so the gate did not simply
#          drop the precondition. The gitlab arm is deliberately NOT gated this way: despite its
#          name DESKD_GITLAB_TOKEN_FILE is also the boot-fetch credential, so it stays required.
#   check  the no-deskd row names the cell's OWN kind, never "a house cell"
#   check  a stream root behind its upstream is a `warn` row naming the branch and the gap, and
#          does NOT change check's exit code; a current root is an `ok` row
#   desk   CELL_FF_ROOTS=1 fast-forwards a clean, 0-ahead root at boot; without the flag the root
#          is left exactly where it was (the default must not move the operator's checkouts)
#   desk   a root that is DIRTY or AHEAD is never moved, even with CELL_FF_ROOTS=1
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private PATH.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-rootdrift.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
export CELLS_ROOT="$T/cells"; mkdir -p "$CELLS_ROOT"
git(){ command git -c user.name=x -c user.email=x@example.invalid "$@"; }

# ---------------------------------------------------------------- fixtures
# One bare origin + a clone that will serve as BOTH CELL_REPO and the stream root.
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/root" 2>/dev/null
mkdir -p "$T/root/docs/streams"; echo "# streams" > "$T/root/docs/streams/README.md"
git -C "$T/root" add -A && git -C "$T/root" commit -q -m seed && git -C "$T/root" push -q origin main
BASE="$(git -C "$T/root" rev-parse HEAD)"
# A second commit pushed to origin from a throwaway clone: now $T/root is 1 BEHIND origin/main.
git clone -q "$T/origin.git" "$T/pusher" 2>/dev/null
echo "next" > "$T/pusher/docs/streams/NEXT.md"
git -C "$T/pusher" add -A && git -C "$T/pusher" commit -q -m next && git -C "$T/pusher" push -q origin main
AHEADSHA="$(git -C "$T/pusher" rev-parse HEAD)"
git -C "$T/root" fetch -q origin main   # remote-tracking ref current, local branch behind

# A github k8s cell, hand-written (the shape cellctl new cannot scaffold: k8s + DESKD=0).
d="$CELLS_ROOT/tcell"; mkdir -p "$d/home/.config/assay" "$d/bin" "$d/index" "$d/worktrees"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$d/home/.config/assay/roster.env"
printf 'DESK_APP_ID=1\n' > "$d/home/.config/assay/apps.env"
mkdir -p "$d/home/.config/gh"
printf 'cells:\n  - name: tcell\n' > "$d/cells-tcell.yaml"
cat > "$d/cell.env" <<CE
CELL=tcell
CELL_KIND=k8s
CELL_FORGE=github
CELL_REPO=$T/root
CELL_ROOTS=grp/root=$T/root
CELLS_CONFIG=$d/cells-tcell.yaml
ROLES="the-desk"
DESKD=0
CE

# Stubs: desk-tools + claude, so check's binary rows and desk's launch resolve offline.
mkdir -p "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$T/bin/$v"; chmod +x "$T/bin/$v"
done
printf '#!/usr/bin/env bash\necho "claude-stub $*" > "%s/launched"\nexit 0\n' "$T" > "$T/bin/claude"
chmod +x "$T/bin/claude"
printf '#!/usr/bin/env bash\nexit 0\n' > "$T/bin/tmux"; chmod +x "$T/bin/tmux"
export PATH="$T/bin:$PATH"
export DESK_TOOLS_BIN="$T/bin"

run_check(){ env DESK_TOOLS_BIN="$T/bin" bash "$CELLCTL" check tcell 2>&1; }

# ---------------------------------------------------------------- deskd gating
out="$(run_check || true)"; rc=0; run_check >/dev/null 2>&1 || rc=$?
assert 'DESKD=0: deskd App key / orgs rows are n/a, not MISS' \
  '[[ "$out" == *"deskd read App key / orgs to mint — not applicable with DESKD=0"* ]]'
assert 'DESKD=0: no MISS row mentions the deskd App key' \
  '! grep -q "MISS.*deskd read App key" <<<"$out"'
assert 'DESKD=0: the no-deskd row names this cell kind, not "a house cell"' \
  '[[ "$out" == *"not required on this k8s cell with DESKD=0"* ]] && [[ "$out" != *"not required on a house cell"* ]]'

# Flip to DESKD=1: the same absent token must now be a MISS (the gate did not drop the check).
sed_i(){ python3 - "$@" <<'PY'
import sys,io
p,old,new=sys.argv[1],sys.argv[2],sys.argv[3]
s=io.open(p).read(); io.open(p,'w').write(s.replace(old,new))
PY
}
sed_i "$d/cell.env" 'DESKD=0' 'DESKD=1'
out1="$(run_check || true)"
assert 'DESKD=1: the absent deskd App key is still a MISS' \
  '[[ "$out1" == *"MISS"*"deskd read App key"* ]]'
sed_i "$d/cell.env" 'DESKD=1' 'DESKD=0'

# ---------------------------------------------------------------- drift reporting
out="$(run_check || true)"; rc=0; run_check >/dev/null 2>&1 || rc=$?
assert 'behind root is reported as a warn row naming the gap' \
  '[[ "$out" == *"warn"*"root grp/root"*"1 commit(s) BEHIND origin/main"* ]]'
assert 'a behind root does NOT fail check (warn is non-fatal)' '[[ "$rc" == "0" ]]'

# ---------------------------------------------------------------- boot: default leaves roots alone
env DESK_TOOLS_BIN="$T/bin" bash "$CELLCTL" desk tcell the-desk >/dev/null 2>&1 || true
assert 'without CELL_FF_ROOTS the root is NOT moved' \
  '[[ "$(git -C "$T/root" rev-parse HEAD)" == "$BASE" ]]'

# ---------------------------------------------------------------- boot: opt-in fast-forwards
env DESK_TOOLS_BIN="$T/bin" CELL_FF_ROOTS=1 bash "$CELLCTL" desk tcell the-desk >/dev/null 2>&1 || true
assert 'CELL_FF_ROOTS=1 fast-forwards the clean, 0-ahead root' \
  '[[ "$(git -C "$T/root" rev-parse HEAD)" == "$AHEADSHA" ]]'
assert 'the root is current after the ff, so check reports it ok' \
  '[[ "$(run_check || true)" == *"ok"*"root grp/root current with origin/main"* ]]'

# ---------------------------------------------------------------- dirty / ahead roots are never moved
echo "local edit" >> "$T/root/docs/streams/README.md"
before="$(git -C "$T/root" rev-parse HEAD)"
echo "third" > "$T/pusher/docs/streams/THIRD.md"
git -C "$T/pusher" add -A && git -C "$T/pusher" commit -q -m third && git -C "$T/pusher" push -q origin main
env DESK_TOOLS_BIN="$T/bin" CELL_FF_ROOTS=1 bash "$CELLCTL" desk tcell the-desk >/dev/null 2>&1 || true
assert 'a DIRTY root is never fast-forwarded, even with CELL_FF_ROOTS=1' \
  '[[ "$(git -C "$T/root" rev-parse HEAD)" == "$before" ]]'
git -C "$T/root" checkout -q -- docs/streams/README.md
git -C "$T/root" fetch -q origin main
git -C "$T/root" merge -q --ff-only origin/main
echo "mine" > "$T/root/docs/streams/MINE.md"
git -C "$T/root" add -A && git -C "$T/root" commit -q -m mine
ahead_sha="$(git -C "$T/root" rev-parse HEAD)"
env DESK_TOOLS_BIN="$T/bin" CELL_FF_ROOTS=1 bash "$CELLCTL" desk tcell the-desk >/dev/null 2>&1 || true
assert 'an AHEAD root is never moved, even with CELL_FF_ROOTS=1' \
  '[[ "$(git -C "$T/root" rev-parse HEAD)" == "$ahead_sha" ]]'

echo
if [[ "$fails" == "0" ]]; then echo "root-drift.test.sh: all assertions passed"; else echo "root-drift.test.sh: $fails FAILED"; exit 1; fi
