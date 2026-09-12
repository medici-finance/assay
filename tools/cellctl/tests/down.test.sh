#!/usr/bin/env bash
# down.test.sh — cellctl's `down` dispatch, against a fixture house cell.
#
# What it proves (each an `assert` below):
#   bare `cellctl down <cell>` (no flags) does NOT die on a phantom empty positional — the
#   dispatcher used to always pass a 3rd arg to cmd_down even when the caller gave none
#   (`down) cmd_down "${2:?cell}" "${3:-}" ;;`), and cmd_down's flag loop rejected that empty
#   string as `down: unexpected argument ''` (assay-toolkit#916 / driver report, reproduced here
#   against the pre-fix binary before this suite existed)
#   `--keep-deskd` alone still works (the single-flag case the old code accidentally allowed)
#   `--cockpit tmux` alone still works
#   `--keep-deskd --cockpit tmux` TOGETHER both take effect — the old dispatcher line
#   (`cmd_down "${2:?cell}" "${3:-}"`) only ever forwarded ONE token past the cell, so the second
#   flag's value silently never reached cmd_down
#   an actually-unexpected positional (not a flag) is still refused, same as before
#
# No network, no tmux server: `tmux`/`herdr`/`orca` are absent from this test's PATH entirely, so
# `down` resolves to the tmux cockpit (which reports "no session" rather than launching anything).
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-down.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# A caller's shell may already carry a cell's exported environment; clear it so this suite's
# result depends only on the fixture below, never on whatever launched this test process.
unset CELL CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELL_COCKPIT CELLS_CONFIG ROLES DESKD \
      DESKD_ADDR DESKD_INDEX DESK_MODEL_DEFAULT DESK_MODEL_the_desk TMUX_SESSION CELL_ATTENDED \
      CELL_PROVIDER

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

# A private PATH with NO tmux/herdr/orca stub at all: `down`'s tmux teardown ("no session
# <name>") is the one path every cockpit falls through to, and it needs `command -v tmux` to
# resolve — so a minimal tmux stub goes on PATH (a real `tmux has-session` against a name that
# was never created is exactly "no session", which the stub below reproduces without a real
# tmux server).
mkdir -p "$T/bin"
cat > "$T/bin/tmux" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  has-session) exit 1 ;;
  kill-session) exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/tmux"
export PATH="$T/bin:/usr/bin:/bin:/usr/sbin:/sbin"
command -v herdr >/dev/null && { echo "down.test.sh: a real herdr is on the base PATH — cannot run hermetically"; exit 2; }
command -v orca  >/dev/null && { echo "down.test.sh: a real orca is on the base PATH — cannot run hermetically"; exit 2; }

export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null

# ---------------------------------------------------------------- bare down
echo "[bare down]"
out="$("$CELLCTL" down example-cell 2>&1)" && rc=0 || rc=$?
assert "bare 'cellctl down <cell>' (no flags) does not die on a phantom positional" '[[ $rc -eq 0 ]]'
assert "... and it reports no session, not an argument error" 'grep -q "no session" <<<"$out" && ! grep -q "unexpected argument" <<<"$out"'

# ---------------------------------------------------------------- single flags
echo "[single flags]"
out="$("$CELLCTL" down example-cell --keep-deskd 2>&1)" && rc=0 || rc=$?
assert "--keep-deskd alone still works" '[[ $rc -eq 0 ]] && ! grep -q "unexpected argument" <<<"$out"'

out="$("$CELLCTL" down example-cell --cockpit tmux 2>&1)" && rc=0 || rc=$?
assert "--cockpit tmux alone still works" '[[ $rc -eq 0 ]] && grep -qF "[cockpit] tmux (explicit: --cockpit)" <<<"$out"'

# ---------------------------------------------------------------- both flags together
echo "[both flags together]"
# The pre-fix dispatcher (`cmd_down "${2:?cell}" "${3:-}"`) forwarded only ONE token past the
# cell name — so `--cockpit tmux` as the SECOND flag pair would have had its value ("tmux")
# silently dropped, and cmd_down's flag loop would have seen a bare `--cockpit` with nothing
# after it, tripping its own `${2:?--cockpit needs a value...}` usage error instead of running.
out="$("$CELLCTL" down example-cell --keep-deskd --cockpit tmux 2>&1)" && rc=0 || rc=$?
assert "--keep-deskd --cockpit tmux TOGETHER: exits 0" '[[ $rc -eq 0 ]]'
assert "... --cockpit's value (tmux) actually arrived" 'grep -qF "[cockpit] tmux (explicit: --cockpit)" <<<"$out"'
assert "... and neither flag was rejected as an unexpected argument" '! grep -q "unexpected argument" <<<"$out" && ! grep -q "needs a value" <<<"$out"'

# order swapped: --cockpit before --keep-deskd
out="$("$CELLCTL" down example-cell --cockpit tmux --keep-deskd 2>&1)" && rc=0 || rc=$?
assert "flags in the other order also both take effect" '[[ $rc -eq 0 ]] && grep -qF "[cockpit] tmux (explicit: --cockpit)" <<<"$out"'

# ---------------------------------------------------------------- genuine bad input still refused
echo "[still refused]"
out="$("$CELLCTL" down example-cell not-a-flag 2>&1)" && rc=0 || rc=$?
assert "a real stray positional is still refused (not silently swallowed)" '[[ $rc -ne 0 ]] && grep -q "unexpected argument .not-a-flag." <<<"$out"'
out="$("$CELLCTL" down example-cell --cockpit 2>&1)" && rc=0 || rc=$?
assert "--cockpit with no value is still refused" '[[ $rc -ne 0 ]] && grep -q "needs a value" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "down.test.sh: OK"; else echo "down.test.sh: $fails FAILED"; exit 1; fi
