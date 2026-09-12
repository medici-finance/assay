#!/usr/bin/env bash
# cockpit.test.sh — cellctl's COCKPIT selection, against stub cockpit binaries on a private PATH.
#
# What it proves (each an `assert` below):
#   new     scaffolds CELL_COCKPIT=auto into cell.env
#   auto    no herdr and no orca            → tmux, reason "fallback: no herdr/orca on PATH"
#           herdr on PATH                   → herdr, reason "auto: on PATH"
#           orca on PATH and app reachable  → orca,  reason "auto: on PATH"
#           orca on PATH, app UNREACHABLE   → tmux, reason "orca on PATH but app unreachable"
#           herdr beats a reachable orca
#   explicit --cockpit herdr with no herdr  → non-zero, the refusal names herdr
#           --cockpit orca, app unreachable → non-zero, the refusal names the unreachable app
#           --cockpit <nonsense>            → non-zero
#           cell.env CELL_COCKPIT=tmux      → tmux even with herdr on PATH
#   plan    DRY_RUN=1 prints the resolved cockpit AND one command per role, and launches nothing
#   guard   --automate is refused unless the resolved cockpit is orca
#   check   carries a cockpit row, and states orca reachability whenever orca is installed
#
# No network, no tmux server, no real cockpit and no real desk-tools: every binary the script
# probes is a stub on a private PATH, and `DRY_RUN=1` means nothing is ever launched.
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and the
# variables they read look unused to a static pass.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-cockpit.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# A caller's shell may already carry a cell's exported environment (cellctl sources cell.env with
# `set -a`), which would leak into every cell loaded here. Clear it.
unset CELL CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELL_COCKPIT CELLS_CONFIG ROLES DESKD \
      DESKD_ADDR DESKD_INDEX DESK_MODEL_DEFAULT TMUX_SESSION CELL_ATTENDED

# ---------------------------------------------------------------- fixtures
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
git -C "$T/seed" add -A && git -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m seed
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"
ROOTS="example-org/example-repo=$REPO"

# A PRIVATE PATH: the stubs, then the system directories only — so a cockpit installed on the
# host cannot decide a case. `tmux` is stubbed too: `command -v tmux` is the only thing the
# selection asks of it, and DRY_RUN never reaches the tmux arm.
mkdir -p "$T/bin"
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
exit 0
EOF
printf '#!/usr/bin/env bash\nexit 1\n' > "$T/bin/tmux"
chmod +x "$T/bin/claude" "$T/bin/tmux"
export PATH="$T/bin:/usr/bin:/bin:/usr/sbin:/sbin"
command -v herdr >/dev/null && { echo "cockpit.test.sh: a real herdr is on the base PATH — cannot run hermetically"; exit 2; }
command -v orca  >/dev/null && { echo "cockpit.test.sh: a real orca is on the base PATH — cannot run hermetically"; exit 2; }

# The cockpit stubs, installed and removed per case.
herdr_on(){ cat > "$T/bin/herdr" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "agent --help") echo "Usage: herdr agent <cmd>"; echo "  start   start an agent"; exit 0 ;;
  "tab --help")   echo "Usage: herdr tab <cmd>"; echo "  create  create a tab"; echo "  close   close a tab"; exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/herdr"; }
herdr_off(){ rm -f "$T/bin/herdr"; }
# ORCA_UP=0 → the desktop app answers the `repo list` probe; anything else → unreachable.
orca_on(){ cat > "$T/bin/orca" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "repo list") exit "${ORCA_UP:-0}" ;;
  "terminal --help") echo "Usage: orca terminal <cmd>"; echo "  create  open a terminal"; echo "  list"; exit 0 ;;
  "terminal create --help") echo "Usage: orca terminal create"; echo "  --cwd <dir>"; echo "  --name <name>"; echo "  --command <cmd>"; exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/orca"; }
orca_off(){ rm -f "$T/bin/orca"; }

export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"

# plan <args...> — `up` in DRY_RUN, stdout+stderr, never launching anything.
plan(){ DRY_RUN=1 "$CELLCTL" up example-cell "$@" 2>&1; }

# ---------------------------------------------------------------- new
echo "[new]"
assert "cell.env carries CELL_COCKPIT=auto" 'grep -qx "CELL_COCKPIT=auto" "$CELL/cell.env"'
assert "cell.env explains the values above it" 'grep -q "auto (herdr if on PATH" "$CELL/cell.env"'

# ---------------------------------------------------------------- auto
echo "[auto]"
herdr_off; orca_off
out="$(plan)"
assert "no herdr and no orca → tmux (fallback)" 'grep -qF "[cockpit] tmux (fallback: no herdr/orca on PATH)" <<<"$out"'
assert "dry-run prints one command per role" '[[ "$(grep -c "^\[dry-run\] .*-desk: " <<<"$out")" -eq 5 ]]'
assert "dry-run prints the role command that every cockpit runs" 'grep -q "desk .example-cell. .worker-desk." <<<"$out"'
assert "dry-run launched nothing (no worktree cut)" '[[ ! -e "$CELL/worktrees/worker-desk" ]]'

herdr_on
out="$(plan)"
assert "herdr on PATH → herdr (auto)" 'grep -qF "[cockpit] herdr (auto: on PATH)" <<<"$out"'

herdr_off; orca_on; export ORCA_UP=0
out="$(plan)"
assert "orca on PATH and app reachable → orca (auto)" 'grep -qF "[cockpit] orca (auto: on PATH)" <<<"$out"'

export ORCA_UP=1
out="$(plan)"
assert "orca on PATH but app unreachable → tmux, with the reachability reason" 'grep -qF "[cockpit] tmux (orca on PATH but app unreachable)" <<<"$out"'

export ORCA_UP=0; herdr_on
out="$(plan)"
assert "herdr wins over a reachable orca" 'grep -qF "[cockpit] herdr (auto: on PATH)" <<<"$out"'

# ---------------------------------------------------------------- explicit
echo "[explicit]"
herdr_off; orca_off
out="$(plan --cockpit herdr)" && rc=0 || rc=$?
assert "--cockpit herdr with no herdr → non-zero" '[[ $rc -ne 0 ]]'
assert "... and the refusal names herdr and PATH" 'grep -q "cockpit herdr" <<<"$out" && grep -q "not on PATH" <<<"$out"'

orca_on; export ORCA_UP=1
out="$(plan --cockpit orca)" && rc=0 || rc=$?
assert "--cockpit orca with an unreachable app → non-zero" '[[ $rc -ne 0 ]]'
assert "... and the refusal names the unreachable desktop app, not PATH" 'grep -q "not reachable" <<<"$out" && ! grep -q "is not on PATH" <<<"$out"'
export ORCA_UP=0
out="$(plan --cockpit orca)"
assert "--cockpit orca with a reachable app → orca (explicit)" 'grep -qF "[cockpit] orca (explicit: --cockpit)" <<<"$out"'
orca_off

out="$(plan --cockpit nope)" && rc=0 || rc=$?
assert "--cockpit with an unknown value → non-zero, names the four" '[[ $rc -ne 0 ]] && grep -q "auto|tmux|herdr|orca" <<<"$out"'

# cell.env pins tmux: a herdr on PATH does NOT override what the cell asked for.
herdr_on
sed -i.bak 's/^CELL_COCKPIT=auto$/CELL_COCKPIT=tmux/' "$CELL/cell.env"
out="$(plan)"
assert "cell.env CELL_COCKPIT=tmux → tmux even with herdr on PATH" 'grep -qF "[cockpit] tmux (explicit: cell.env CELL_COCKPIT)" <<<"$out"'
out="$(plan --cockpit herdr)"
assert "--cockpit overrides cell.env for the run" 'grep -qF "[cockpit] herdr (explicit: --cockpit)" <<<"$out"'

# ---------------------------------------------------------------- --automate guard
echo "[automate]"
out="$(plan --automate '*/30 * * * *')" && rc=0 || rc=$?
assert "--automate on a non-orca cockpit → non-zero, says it is orca-only" '[[ $rc -ne 0 ]] && grep -q "orca-only" <<<"$out"'
herdr_off; orca_on; export ORCA_UP=0
sed -i.bak 's/^CELL_COCKPIT=tmux$/CELL_COCKPIT=orca/' "$CELL/cell.env"
out="$(plan --automate '*/30 * * * *')"
assert "--automate on orca plans one automation per role, precheck = cellctl check" '[[ "$(grep -c "automations create --name example-cell-" <<<"$out")" -eq 5 ]] && grep -q -- "--precheck .*check example-cell" <<<"$out"'
sed -i.bak 's/^CELL_COCKPIT=orca$/CELL_COCKPIT=auto/' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"

# ---------------------------------------------------------------- check
echo "[check]"
orca_off; herdr_off
out="$("$CELLCTL" check example-cell 2>&1)" && rc=0 || rc=$?
assert "check exits 0 and carries a cockpit row" '[[ $rc -eq 0 ]] && grep -qF "ok    cockpit: tmux (fallback: no herdr/orca on PATH)" <<<"$out"'
assert "check says orca is not installed" 'grep -q "n/a   orca — not installed" <<<"$out"'
orca_on; export ORCA_UP=1
out="$("$CELLCTL" check example-cell 2>&1)"
assert "check states an unreachable orca app rather than passing it over silently" 'grep -q "orca on PATH but its desktop app is not reachable" <<<"$out"'
export ORCA_UP=0
out="$("$CELLCTL" check example-cell 2>&1)"
assert "check states a reachable orca app" 'grep -q "ok    orca desktop app reachable" <<<"$out"'
herdr_off; orca_off
sed -i.bak 's/^CELL_COCKPIT=auto$/CELL_COCKPIT=herdr/' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"
out="$("$CELLCTL" check example-cell 2>&1)" && rc=0 || rc=$?
assert "an explicit cockpit that is not installed is a MISS row, and check fails" '[[ $rc -eq 1 ]] && grep -q "MISS  cockpit: cockpit herdr" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "cockpit.test.sh: OK"; else echo "cockpit.test.sh: $fails FAILED"; exit 1; fi
