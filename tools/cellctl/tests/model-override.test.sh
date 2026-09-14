#!/usr/bin/env bash
# model-override.test.sh — `--model` as a per-run override of the cell.env pin (issue-940 part 1).
#
# What it proves (each an `assert` below):
#   desk    `--model <m>` wins over DESK_MODEL_<role> and DESK_MODEL_DEFAULT for that invocation
#           only, without touching cell.env; the equivalent env form (DESK_MODEL_OVERRIDE) does the
#           same, and an explicit `--model` wins when both are given; the resolved model and the
#           fact it came from an override are both visible ("model=<m> (override)") in the
#           DRY_RUN=1 plan and (non-dry-run) the [launch] line; a bare `--model` with no value is a
#           usage error (non-zero exit)
#   desk    the-desk + an Opus override (alias or `claude-opus-*` id) is refused exactly as an Opus
#           PIN is — the same message, naming the override's value — even when the pin itself is a
#           perfectly fine non-Opus value; a non-the-desk role overridden to opus is not refused
#   check   is never affected by --model or DESK_MODEL_OVERRIDE — it has no --model flag and does
#           not read the env form, so its the-desk-model row reports the plain pin
#   up      `--model <m>` in DRY_RUN=1 threads the override onto EVERY role window's own `cellctl
#           desk` invocation (`--model '<m>'` appears in each printed command), and is announced
#           once up front
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private PATH,
# and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-override.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/DESK_ROOTS/DESK_LOOP/
# DESK_SESSION vars (a cellctl-booted session sets them for its own window, and a nested one
# inherits them on exec) — unset them so the fixture below is the only source cellctl reads.
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

set_model(){ # replace or add DESK_MODEL_<var> in the fixture cell.env
  local var="$1" val="$2"
  grep -v "^${var}=" "$CELL/cell.env" > "$CELL/cell.env.tmp"
  [[ -z "$val" ]] || printf '%s=%s\n' "$var" "$val" >> "$CELL/cell.env.tmp"
  mv "$CELL/cell.env.tmp" "$CELL/cell.env"
}

# ---------------------------------------------------------------- desk: override beats pin
echo "[desk: --model beats the pin]"
set_model DESK_MODEL_the_desk fable
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --model sonnet 2>&1)" && rc=0 || rc=$?
assert "--model overrides a non-opus pin (exit 0)" '[[ $rc -eq 0 ]]'
assert "dry-run shows the OVERRIDDEN model, not the pin" 'grep -q "model=sonnet (override)" <<<"$out" && ! grep -q "model=fable" <<<"$out"'
assert "cell.env pin is untouched by a bare --model run" 'grep -qx "DESK_MODEL_the_desk=fable" "$CELL/cell.env"'

echo "[desk: env form DESK_MODEL_OVERRIDE]"
out="$(DRY_RUN=1 DESK_MODEL_OVERRIDE=haiku "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_OVERRIDE overrides the pin too (exit 0)" '[[ $rc -eq 0 ]]'
assert "dry-run shows the env-form override" 'grep -q "model=haiku (override)" <<<"$out"'

echo "[desk: --model wins over DESK_MODEL_OVERRIDE when both given]"
out="$(DRY_RUN=1 DESK_MODEL_OVERRIDE=haiku "$CELLCTL" desk house-cell the-desk --model sonnet 2>&1)" && rc=0 || rc=$?
assert "an explicit --model beats the env form" 'grep -q "model=sonnet (override)" <<<"$out"'

echo "[desk: missing --model value is a usage error]"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --model 2>&1)" && rc=0 || rc=$?
assert "--model with no value is refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "the refusal names --model" 'grep -q -- "--model needs a value" <<<"$out"'

# ---------------------------------------------------------------- desk: the-desk + Opus override
echo "[desk: Opus override refused on the-desk]"
set_model DESK_MODEL_the_desk fable   # the PIN is fine — only the override is Opus
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --model opus 2>&1)" && rc=0 || rc=$?
assert "an opus --model is refused even over a fable pin (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names the OVERRIDE value, not the pin" 'grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'

out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --model claude-opus-5 2>&1)" && rc=0 || rc=$?
assert "a claude-opus-* --model id is refused too (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names claude-opus-5" 'grep -q "resolved DESK_MODEL_the_desk=claude-opus-5" <<<"$out"'

out="$(DRY_RUN=1 DESK_MODEL_OVERRIDE=opus "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_OVERRIDE=opus is refused too (non-zero exit)" '[[ $rc -ne 0 ]]'

out="$(DRY_RUN=1 "$CELLCTL" desk house-cell worker-desk --model opus 2>&1)" && rc=0 || rc=$?
assert "worker-desk overridden to opus is NOT refused" '[[ $rc -eq 0 ]] && grep -q "model=opus (override)" <<<"$out"'

# ---------------------------------------------------------------- check: unaffected by override
echo "[check: unaffected by --model / DESK_MODEL_OVERRIDE]"
set_model DESK_MODEL_the_desk fable
out="$(DESK_MODEL_OVERRIDE=opus "$CELLCTL" check house-cell 2>&1)" && rc=0 || rc=$?
assert "check ignores DESK_MODEL_OVERRIDE and still passes on the fable pin" '[[ $rc -eq 0 ]] && grep -q "ok    the-desk model: fable" <<<"$out"'

# ---------------------------------------------------------------- up: --model to every window
echo "[up: --model threads to every role window]"
set_model DESK_MODEL_the_desk fable
out="$(DRY_RUN=1 "$CELLCTL" up house-cell --cockpit tmux --model sonnet 2>&1)" && rc=0 || rc=$?
assert "up --model dry-run exits 0" '[[ $rc -eq 0 ]]'
assert "up announces the override once" 'grep -q "model=sonnet (override) — applied to every role window below" <<<"$out"'
assert "the-desk window command carries --model sonnet" "grep -q \"the-desk: .*desk 'house-cell' 'the-desk' --model 'sonnet'\" <<<\"\$out\""
assert "worker-desk window command carries --model sonnet too" "grep -q \"worker-desk: .*desk 'house-cell' 'worker-desk' --model 'sonnet'\" <<<\"\$out\""

echo
if [[ "$fails" -eq 0 ]]; then echo "model-override.test.sh: OK"; else echo "model-override.test.sh: $fails FAILED"; exit 1; fi
