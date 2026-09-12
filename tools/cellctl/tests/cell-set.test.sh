#!/usr/bin/env bash
# cell-set.test.sh — `cellctl set <cell> KEY=VALUE [...]`, persisting a cell.env change in place
# (issue-940 part 2), plus the `cellctl desk ... --model <m> --set` sugar that overrides-and-persists
# in one call.
#
# What it proves (each an `assert` below):
#   set     rewrites an EXISTING key's line in place (value changes, line position, surrounding
#           comments and every other line's ordering untouched); APPENDS a key that has no active
#           line yet; REFUSES a key that is not a known cell.env key unless --force (and touches
#           nothing when it refuses); writes exactly ONE backup file (cell.env.bak-<ts>) before the
#           first edit of a multi-key call, byte-identical to the pre-edit file; prints each key's
#           before/after value; applies the same the-desk/Opus refusal to DESK_MODEL_the_desk as a
#           live boot and `check` do, and touches nothing when it refuses (no backup either); a
#           call with zero KEY=VALUE arguments is a usage error
#   desk    `--model <m> --set` applies the override for this run AND persists it into cell.env
#           (the next plain boot, no --model, resolves to the persisted value); `--set` without
#           `--model`/DESK_MODEL_OVERRIDE is refused (nothing to persist); DRY_RUN=1 with --set
#           does not write cell.env (prints what it would do)
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private PATH,
# and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-set.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }
sha(){ if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/DESK_ROOTS/DESK_LOOP/
# DESK_SESSION vars — unset them so the fixture below is the only source cellctl reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_MODEL_OVERRIDE \
  DESK_ROOTS DESK_LOOP DESK_SESSION 2>/dev/null || true

# ---------------------------------------------------------------- fixtures
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
git -C "$T/seed" add -A && git -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "seed"
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"
ROOTS="example-org/example-repo=$REPO"
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
printf '#!/usr/bin/env bash\necho "$HOME" > "%s/deskroster.home"\nexit 0\n' "$T" > "$DESK_TOOLS_BIN/deskroster"
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
echo "ARGS=$*" > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/claude"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"

"$CELLCTL" new house-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/house-cell"

# ---------------------------------------------------------------- set: existing key updated
echo "[set: existing key updated]"
cp "$CELL/cell.env" "$T/before-update.env"
before_lines="$(grep -vE '^DESK_MODEL_the_desk=' "$CELL/cell.env")"
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=sonnet 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
assert "cell.env now carries the new value" 'grep -qx "DESK_MODEL_the_desk=sonnet" "$CELL/cell.env"'
assert "every OTHER line is untouched" '[[ "$(grep -vE "^DESK_MODEL_the_desk=" "$CELL/cell.env")" == "$before_lines" ]]'
before_ln="$(grep -n "^DESK_MODEL_the_desk=" "$T/before-update.env" | cut -d: -f1)"
after_ln="$(grep -n "^DESK_MODEL_the_desk=" "$CELL/cell.env" | cut -d: -f1)"
assert "the updated line keeps its original position (line $before_ln)" '[[ -n "$before_ln" && "$before_ln" == "$after_ln" ]]'
assert "prints before -> after" 'grep -q "DESK_MODEL_the_desk: fable -> sonnet" <<<"$out"'

# ---------------------------------------------------------------- set: missing key appended
echo "[set: missing key appended]"
assert "TMUX_SESSION is not yet in cell.env" '! grep -q "^TMUX_SESSION=" "$CELL/cell.env"'
out="$("$CELLCTL" set house-cell TMUX_SESSION=my-session 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
assert "cell.env now carries the appended key" 'grep -qx "TMUX_SESSION=my-session" "$CELL/cell.env"'
assert "prints <unset> -> value for a fresh key" 'grep -q "TMUX_SESSION: <unset> -> my-session" <<<"$out"'

# ---------------------------------------------------------------- set: unknown key refused
echo "[set: unknown key refused without --force]"
cp "$CELL/cell.env" "$T/before-unknown.env"
out="$("$CELLCTL" set house-cell NOT_A_REAL_KEY=x 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names the key and --force" 'grep -q "NOT_A_REAL_KEY" <<<"$out" && grep -q -- "--force" <<<"$out"'
assert "cell.env is untouched by the refusal" '[[ "$(sha "$CELL/cell.env")" == "$(sha "$T/before-unknown.env")" ]]'
out="$("$CELLCTL" set house-cell NOT_A_REAL_KEY=x --force 2>&1)" && rc=0 || rc=$?
assert "--force allows the unknown key" '[[ $rc -eq 0 ]] && grep -qx "NOT_A_REAL_KEY=x" "$CELL/cell.env"'

# ---------------------------------------------------------------- set: comments/ordering preserved
echo "[set: comments and ordering preserved]"
printf '# a leading comment\nCELL_COCKPIT=auto\n# a trailing comment about the roles\nROLES="the-desk worker-desk"\n' > "$CELL/cell.env"
cp "$CELL/cell.env" "$T/before-comments.env"
"$CELLCTL" set house-cell CELL_COCKPIT=tmux >/dev/null
assert "comment lines are byte-identical and stay in place" \
  '[[ "$(sed -n 1p "$CELL/cell.env")" == "# a leading comment" && "$(sed -n 3p "$CELL/cell.env")" == "# a trailing comment about the roles" ]]'
assert "the edited line keeps its position (line 2)" '[[ "$(sed -n 2p "$CELL/cell.env")" == "CELL_COCKPIT=tmux" ]]'
assert "the line after it (ROLES) is untouched and still line 4" '[[ "$(sed -n 4p "$CELL/cell.env")" == "ROLES=\"the-desk worker-desk\"" ]]'
assert "file has the same line count as before" '[[ "$(wc -l < "$CELL/cell.env")" == "$(wc -l < "$T/before-comments.env")" ]]'

# ---------------------------------------------------------------- set: backup file written
echo "[set: backup file written]"
"$CELLCTL" new backup-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
BCELL="$CELLS_ROOT/backup-cell"
cp "$BCELL/cell.env" "$T/before-backup.env"
before_count="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | wc -l | tr -d ' ')"
out="$("$CELLCTL" set backup-cell DESK_MODEL_DEFAULT=haiku ROLES="worker-desk" 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
after_count="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | wc -l | tr -d ' ')"
assert "exactly one backup file was written for a two-key call" '[[ $((after_count - before_count)) -eq 1 ]]'
backup_file="$(find "$BCELL" -maxdepth 1 -name 'cell.env.bak-*' | head -n1)"
assert "the backup matches the pre-edit file" '[[ "$(sha "$backup_file")" == "$(sha "$T/before-backup.env")" ]]'
assert "the backup announcement names the file" 'grep -q "backup written: $backup_file" <<<"$out"'

# ---------------------------------------------------------------- set: opus-for-the-desk refused
echo "[set: opus-for-the-desk refused]"
cp "$CELL/cell.env" "$T/before-opus-set.env"
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=opus 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "same refusal message as a live boot / check" 'grep -q "the-desk runs on the top tier" <<<"$out" && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'
assert "cell.env is untouched (no backup, no edit)" '[[ "$(sha "$CELL/cell.env")" == "$(sha "$T/before-opus-set.env")" ]]'
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=claude-opus-9 2>&1)" && rc=0 || rc=$?
assert "a claude-opus-* id is refused too" '[[ $rc -ne 0 ]] && grep -q "claude-opus-9" <<<"$out"'
out="$("$CELLCTL" set house-cell DESK_MODEL_the_desk=opus --force 2>&1)" && rc=0 || rc=$?
assert "--force does not bypass the opus refusal (it is a value check, not a key-allowlist check)" \
  '[[ $rc -ne 0 ]] && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'

# ---------------------------------------------------------------- set: no KEY=VALUE is a usage error
echo "[set: no KEY=VALUE is a usage error]"
out="$("$CELLCTL" set house-cell 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "names KEY=VALUE" 'grep -q "KEY=VALUE" <<<"$out"'

# ---------------------------------------------------------------- desk --model --set sugar
echo "[desk --model --set: override now AND persist]"
"$CELLCTL" new sugar-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
SCELL="$CELLS_ROOT/sugar-cell"
assert "sugar-cell starts pinned to fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$SCELL/cell.env"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk --model sonnet --set 2>&1)" && rc=0 || rc=$?
assert "DRY_RUN + --set exits 0" '[[ $rc -eq 0 ]]'
assert "DRY_RUN + --set does not touch cell.env" 'grep -qx "DESK_MODEL_the_desk=fable" "$SCELL/cell.env"'
assert "DRY_RUN + --set says what it WOULD persist" 'grep -q "would persist DESK_MODEL_the_desk=sonnet" <<<"$out"'

out="$("$CELLCTL" desk sugar-cell the-desk --model sonnet --set 2>&1)" && rc=0 || rc=$?
assert "live --model --set exits 0" '[[ $rc -eq 0 ]]'
assert "this run's launch line shows the override" 'grep -q "model=sonnet (override)" <<<"$out"'
assert "the pin is now persisted in cell.env" 'grep -qx "DESK_MODEL_the_desk=sonnet" "$SCELL/cell.env"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "a later PLAIN boot (no --model) resolves to the persisted value" '[[ $rc -eq 0 ]] && grep -q "model=sonnet" <<<"$out" && ! grep -q "(override)" <<<"$out"'

out="$("$CELLCTL" desk sugar-cell worker-desk --set 2>&1)" && rc=0 || rc=$?
assert "--set without --model/DESK_MODEL_OVERRIDE is refused" '[[ $rc -ne 0 ]] && grep -q -- "--set needs --model" <<<"$out"'

out="$(DRY_RUN=1 "$CELLCTL" desk sugar-cell the-desk --model opus --set 2>&1)" && rc=0 || rc=$?
assert "--model opus --set is refused (opus rule outranks the sugar) and persists nothing" \
  '[[ $rc -ne 0 ]] && grep -qx "DESK_MODEL_the_desk=sonnet" "$SCELL/cell.env"'

echo
if [[ "$fails" -eq 0 ]]; then echo "cell-set.test.sh: OK"; else echo "cell-set.test.sh: $fails FAILED"; exit 1; fi
