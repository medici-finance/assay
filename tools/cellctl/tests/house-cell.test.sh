#!/usr/bin/env bash
# house-cell.test.sh — cellctl's HOUSE cell kind, end to end against a fixture git repo.
#
# What it proves (each an `assert` below):
#   new    scaffolds cell.env (CELL_KIND=house, CELL_ROOTS) + home/ with the operator config home
#          reached by symlink, never copied; a second `new` on the same name REFUSES
#   check  passes on a well-formed house cell and FAILS (exit 1, a MISS row) when a root lacks docs/streams/
#   desk   creates the role worktree under <cell>/worktrees/<role>, LOCKS it, starts the (stubbed)
#          `claude` with cwd = that worktree and DESK_ROOTS / DESK_LOOP / DESK_SESSION exported,
#          and leaves the fixture checkout's .git/config byte-identical; a second boot fast-forwards
#          the same tree; a `deskwt role-init` that supports the role is preferred when present, and
#          one that refuses the probe is not
#   legacy a cell.env with no CELL_KIND still loads as a k8s cell
#
# No network, no tmux, no real desk-tools: `claude`, `deskwt` and the desk verbs are stubs on a
# private PATH, and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and the
# variables they read look unused to a static pass.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
# Resolved with pwd -P: a TMPDIR with a trailing slash or a symlinked temp root would otherwise
# make the paths cellctl prints (it normalises) differ from the ones the test compares against.
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-house.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }
sha(){ if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
real(){ (cd "$1" && pwd -P); }

# ---------------------------------------------------------------- fixtures
# A private HOME so nothing of the operator's is read or linked; git identity via its .gitconfig.
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
# The operator config home the house cell links to.
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
# Fixture repo: a bare "origin" with a main branch carrying docs/streams/, cloned as CELL_REPO.
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
git -C "$T/seed" add -A && git -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "seed"
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"
# A second stream root, and a directory that is NOT a stream root (no docs/streams/).
mkdir -p "$T/second/docs/streams" "$T/noroot"
ROOTS="example-org/example-repo=$REPO,example-org/second=$T/second"
# Stub desk verbs (the ones a house `check` proves), a stub claude, and a private PATH.
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
# `deskroster repos --scope scan` must exit 0 under the CELL home — record the HOME it saw.
printf '#!/usr/bin/env bash\necho "$HOME" > "%s/deskroster.home"\nexit 0\n' "$T" > "$DESK_TOOLS_BIN/deskroster"
# The claude stub: `plugin enable` and `plugin list --json` answer as an enabled plugin would;
# anything else is the session launch, recorded (cwd, env, argv) instead of run.
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
{
  echo "PWD=$(pwd -P)"
  echo "DESK_ROOTS=${DESK_ROOTS:-}"
  echo "DESK_LOOP=${DESK_LOOP:-}"
  echo "DESK_SESSION=${DESK_SESSION:-}"
  echo "PATH0=${PATH%%:*}"
  echo "ARGS=$*"
} > "$CELLCTL_TEST_OUT"
EOF
chmod +x "$T/bin/claude"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
CONFIG_SHA_BEFORE="$(sha "$REPO/.git/config")"

# ---------------------------------------------------------------- new
echo "[new]"
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"
assert "cell.env written" '[[ -f "$CELL/cell.env" ]]'
assert "cell.env CELL_KIND=house" 'grep -qx "CELL_KIND=house" "$CELL/cell.env"'
assert "cell.env CELL_ROOTS is the --roots value" 'grep -qxF "CELL_ROOTS=$ROOTS" "$CELL/cell.env"'
assert "cell.env CELL_REPO is the --repo value" 'grep -qxF "CELL_REPO=$REPO" "$CELL/cell.env"'
assert "cell.env DESKD=0 (no deskd on a house cell)" 'grep -qx "DESKD=0" "$CELL/cell.env"'
assert "home/.config/assay is a SYMLINK to the operator config home (nothing copied)" '[[ -L "$CELL/home/.config/assay" && "$(readlink "$CELL/home/.config/assay")" == "$ASSAY_CONFIG_HOME" ]]'
assert "roster reachable through the cell home" '[[ -f "$CELL/home/.config/assay/roster.env" ]]'
assert "gitconfig + gh config linked" '[[ -L "$CELL/home/.gitconfig" && -L "$CELL/home/.config/gh" ]]'
rc=0; "$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null 2>&1 || rc=$?
assert "second new on the same name REFUSES (exit 3), cell untouched" '[[ $rc -eq 3 ]] && grep -qxF "CELL_ROOTS=$ROOTS" "$CELL/cell.env"'
assert "new refuses a malformed --roots" '! "$CELLCTL" new bad-roots --kind house --repo "$REPO" --roots "not-a-map" >/dev/null 2>&1'
assert "new refuses a --roots path that does not exist" '! "$CELLCTL" new bad-path --kind house --repo "$REPO" --roots "example-org/x=$T/does-not-exist" >/dev/null 2>&1'

# ---------------------------------------------------------------- check
echo "[check]"
out="$("$CELLCTL" check example-cell 2>&1)"; rc=$?
assert "check exits 0 on the house cell" '[[ $rc -eq 0 ]]'
assert "check reports all preconditions met" 'grep -q "all preconditions met" <<<"$out"'
assert "check ran deskroster under the CELL home" '[[ "$(cat "$T/deskroster.home")" == "$CELL/home" ]]'
assert "check proves every root carries docs/streams/" '[[ "$(grep -c "carries docs/streams/" <<<"$out")" -eq 2 ]]'
assert "check reports deskd n/a on a house cell" 'grep -q "n/a   deskd" <<<"$out"'
assert "check reports the plugin row ok" 'grep -q "ok    plugin assay@assay" <<<"$out"'
"$CELLCTL" new bad-cell --kind house --repo "$REPO" --roots "example-org/x=$T/noroot" >/dev/null
out="$("$CELLCTL" check bad-cell 2>&1)" && rc=0 || rc=$?
assert "check exits 1 when a root lacks docs/streams/" '[[ $rc -eq 1 ]]'
assert "check names the MISS row" 'grep -q "MISS  root example-org/x carries docs/streams/" <<<"$out"'

# ---------------------------------------------------------------- desk (dry run, then real)
echo "[desk]"
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk 2>&1)"
assert "dry-run prints kind, role and desk_roots, touches nothing" 'grep -q "kind=house role=worker-desk" <<<"$out" && grep -qF "desk_roots=$ROOTS" <<<"$out" && [[ ! -e "$CELL/worktrees/worker-desk" ]]'

export CELLCTL_TEST_OUT="$T/launch-worker.env"
"$CELLCTL" desk example-cell worker-desk >/dev/null 2>"$T/desk-worker.err"
WT="$CELL/worktrees/worker-desk"
assert "worktree exists under <cell>/worktrees/<role>" '[[ -e "$WT/.git" ]]'
assert "worktree is at origin/main" '[[ "$(git -C "$WT" rev-parse HEAD)" == "$(git -C "$REPO" rev-parse FETCH_HEAD)" ]]'
assert "worktree is LOCKED" 'git -C "$REPO" worktree list --porcelain | grep -A3 -F "worktree $(real "$WT")" | grep -q "^locked"'
assert "claude stub ran (launch recorded)" '[[ -s "$CELLCTL_TEST_OUT" ]]'
assert "cwd of the session = the worktree" 'grep -qxF "PWD=$(real "$WT")" "$CELLCTL_TEST_OUT"'
assert "DESK_ROOTS exported = CELL_ROOTS" 'grep -qxF "DESK_ROOTS=$ROOTS" "$CELLCTL_TEST_OUT"'
assert "DESK_LOOP exported = role" 'grep -qx "DESK_LOOP=worker-desk" "$CELLCTL_TEST_OUT"'
assert "DESK_SESSION = <cell>-<role>-<UTC stamp>" 'grep -qE "^DESK_SESSION=example-cell-worker-desk-[0-9]{8}T[0-9]{6}Z$" "$CELLCTL_TEST_OUT"'
assert "shim dir first on PATH" 'grep -qxF "PATH0=$CELL/shim" "$CELLCTL_TEST_OUT"'
assert "model pinned (sonnet) and the role skill is the first prompt" 'grep -q -- "--model sonnet /assay:worker-desk" "$CELLCTL_TEST_OUT"'
assert "fixture .git/config byte-unchanged" '[[ "$(sha "$REPO/.git/config")" == "$CONFIG_SHA_BEFORE" ]]'
assert "no deskd notice on a house cell" '! grep -q "deskd is NOT up" "$T/desk-worker.err"'
assert "shims wrap the desk verbs with the cell HOME" 'grep -q "HOME=\"$CELL/home\"" "$CELL/shim/deskboard"'
# Second boot of the same role: the tree is reused and fast-forwarded, still locked.
export CELLCTL_TEST_OUT="$T/launch-worker-2.env"
"$CELLCTL" desk example-cell worker-desk >/dev/null 2>&1
assert "second boot reuses the same worktree" 'grep -qxF "PWD=$(real "$WT")" "$CELLCTL_TEST_OUT"'
assert "second boot leaves it locked" 'git -C "$REPO" worktree list --porcelain | grep -A3 -F "worktree $(real "$WT")" | grep -q "^locked"'

# deskwt preference: a role-init that SUPPORTS the role (probe exits 0) names the tree.
cat > "$T/bin/deskwt" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == "role-init" ]] || exit 5
[[ "\${2:-}" == "--help" ]] && exit 0
d="$T/dwt-\$2"; git worktree add -q "\$d" -b "deskwt/\$2" FETCH_HEAD; echo "some chatter"; echo "\$d"
EOF
chmod +x "$T/bin/deskwt"
export CELLCTL_TEST_OUT="$T/launch-verify.env"
"$CELLCTL" desk example-cell verify-desk >/dev/null 2>&1
assert "deskwt role-init preferred when it supports the role (cell path links to its tree)" '[[ -L "$CELL/worktrees/verify-desk" && "$(readlink "$CELL/worktrees/verify-desk")" == "$T/dwt-verify-desk" ]]'
assert "session cwd = the deskwt-created tree" 'grep -qxF "PWD=$(real "$T/dwt-verify-desk")" "$CELLCTL_TEST_OUT"'
assert "deskwt-created tree is locked too" 'git -C "$REPO" worktree list --porcelain | grep -A3 -F "worktree $(real "$T/dwt-verify-desk")" | grep -q "^locked"'
# ... and one that REFUSES the probe (exit 5, today's installed behaviour) is not used.
printf '#!/usr/bin/env bash\nexit 5\n' > "$T/bin/deskwt"
export CELLCTL_TEST_OUT="$T/launch-intake.env"
"$CELLCTL" desk example-cell intake-desk >/dev/null 2>&1
assert "deskwt that refuses the probe → cellctl's own worktree path" '[[ ! -L "$CELL/worktrees/intake-desk" && -e "$CELL/worktrees/intake-desk/.git" ]]'
# ... and CELLCTL_DESKWT=0 forces the own path even when deskwt would support the role.
cat > "$T/bin/deskwt" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == "role-init" ]] || exit 5
[[ "\${2:-}" == "--help" ]] && exit 0
d="$T/dwt-\$2"; git worktree add -q "\$d" -b "deskwt/\$2" FETCH_HEAD; echo "\$d"
EOF
export CELLCTL_TEST_OUT="$T/launch-review.env"
CELLCTL_DESKWT=0 "$CELLCTL" desk example-cell pr-review-desk >/dev/null 2>&1
assert "CELLCTL_DESKWT=0 forces cellctl's own path even when deskwt supports the role" '[[ ! -L "$CELL/worktrees/pr-review-desk" && -e "$CELL/worktrees/pr-review-desk/.git" && ! -e "$T/dwt-pr-review-desk" ]]'

# ---------------------------------------------------------------- legacy k8s cell.env
echo "[legacy]"
mkdir -p "$CELLS_ROOT/legacy/home/.config/assay"
printf 'CELL=legacy\nCELL_REPO=%s\n' "$REPO" > "$CELLS_ROOT/legacy/cell.env"
ln -s "$ASSAY_CONFIG_HOME/roster.env" "$CELLS_ROOT/legacy/home/.config/assay/roster.env"
out="$(DRY_RUN=1 "$CELLCTL" desk legacy the-desk 2>&1)"
assert "cell.env without CELL_KIND loads as k8s" 'grep -q "kind=k8s role=the-desk" <<<"$out"'
assert "k8s cell without CELL_ROOTS says so (NOTICE) and boots with desk_roots=unset" 'grep -q "NOTICE: cell.env has no CELL_ROOTS" <<<"$out" && grep -q "desk_roots=unset" <<<"$out"'
assert "k8s session name keeps <cell>-<short role>" 'grep -q "session=legacy-the-desk " <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "house-cell.test.sh: OK"; else echo "house-cell.test.sh: $fails FAILED"; exit 1; fi
