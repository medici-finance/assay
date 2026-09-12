#!/usr/bin/env bash
# model-pin.test.sh — the-desk's top-tier model pin and the Opus refusal.
#
# What it proves (each an `assert` below):
#   new     scaffolds DESK_MODEL_the_desk=fable in cell.env for all three kinds/forges `new`
#           supports (k8s/github, k8s/gitlab, house)
#   desk    `DRY_RUN=1 cellctl desk <cell> the-desk` refuses (exit non-zero, names the resolved
#           value and the variable) on an Opus alias or a `claude-opus-*` id, whether the pin
#           comes from DESK_MODEL_the_desk or falls through to DESK_MODEL_DEFAULT; a non-Opus
#           value — including a full `claude-fable-*` id — passes; a non-the-desk role (e.g.
#           worker-desk) pinned to opus is NOT refused
#   check   flags an Opus pin on the-desk as a MISS with the same message, and passes a fable pin
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private
# PATH, and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and the
# last `out`/`rc` pair is read by its assert but shellcheck's flow analysis misses it.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-model.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/DESK_ROOTS/DESK_LOOP/
# DESK_SESSION vars (a cellctl-booted session sets them for its own window) — unset them so the
# fixture below is the only source cellctl reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_ROOTS DESK_LOOP \
  DESK_SESSION 2>/dev/null || true

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
# A minimal cells.yaml + App PEM for the k8s/github and k8s/gitlab paths.
printf 'cells: []\n' > "$T/cells.yaml"
printf -- '-----BEGIN FAKE KEY-----\nfake\n-----END FAKE KEY-----\n' > "$T/app.pem"

# ---------------------------------------------------------------- new: all three kinds/forges
echo "[new: scaffold pin]"
"$CELLCTL" new gh-cell --kind k8s --forge github --repo "$REPO" --cells-yaml "$T/cells.yaml" \
  --orgs example-org --deskd-app-pem "$T/app.pem" >/dev/null
assert "k8s/github cell.env pins DESK_MODEL_the_desk=fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$CELLS_ROOT/gh-cell/cell.env"'

"$CELLCTL" new gl-cell --kind k8s --forge gitlab --repo "$REPO" --cells-yaml "$T/cells.yaml" \
  --group example-group >/dev/null
assert "k8s/gitlab cell.env pins DESK_MODEL_the_desk=fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$CELLS_ROOT/gl-cell/cell.env"'

"$CELLCTL" new house-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
assert "house cell.env pins DESK_MODEL_the_desk=fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$CELLS_ROOT/house-cell/cell.env"'

# ---------------------------------------------------------------- desk: dry-run refusal
echo "[desk: opus refusal]"
CELL="$CELLS_ROOT/house-cell"

set_model(){ # replace or add DESK_MODEL_the_desk in the fixture cell.env
  grep -v '^DESK_MODEL_the_desk=' "$CELL/cell.env" > "$CELL/cell.env.tmp"
  [[ -z "${1:-}" ]] || printf 'DESK_MODEL_the_desk=%s\n' "$1" >> "$CELL/cell.env.tmp"
  mv "$CELL/cell.env.tmp" "$CELL/cell.env"
}

set_model opus
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_the_desk=opus refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names the resolved value and the variable" 'grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out" && grep -q "set DESK_MODEL_the_desk=fable" <<<"$out"'

set_model claude-opus-5
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_the_desk=claude-opus-5 refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "refusal names claude-opus-5" 'grep -q "resolved DESK_MODEL_the_desk=claude-opus-5" <<<"$out"'

set_model fable
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_the_desk=fable passes (exit 0)" '[[ $rc -eq 0 ]]'
assert "dry-run plan shows model=fable" 'grep -q "model=fable" <<<"$out"'

set_model claude-fable-5-1
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "DESK_MODEL_the_desk=claude-fable-5-1 passes (exit 0)" '[[ $rc -eq 0 ]]'

# DESK_MODEL_the_desk unset entirely; DESK_MODEL_DEFAULT carries an Opus pin instead.
set_model ""
sed -i.bak 's/^DESK_MODEL_DEFAULT=.*/DESK_MODEL_DEFAULT=opus/' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "unset DESK_MODEL_the_desk + DESK_MODEL_DEFAULT=opus is refused too" '[[ $rc -ne 0 ]] && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'
sed -i.bak 's/^DESK_MODEL_DEFAULT=.*/DESK_MODEL_DEFAULT=sonnet/' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"

# worker-desk is never checked: an Opus pin there is untouched.
printf 'DESK_MODEL_worker_desk=opus\n' >> "$CELL/cell.env"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell worker-desk 2>&1)" && rc=0 || rc=$?
assert "worker-desk pinned to opus is NOT refused" '[[ $rc -eq 0 ]] && grep -q "model=opus" <<<"$out"'

# ---------------------------------------------------------------- check: MISS / ok rows
echo "[check: opus pin]"
set_model opus
out="$("$CELLCTL" check house-cell 2>&1)" && rc=0 || rc=$?
assert "check fails (exit 1) on an Opus pin" '[[ $rc -eq 1 ]]'
assert "check's the-desk model row is a MISS naming the resolved value" 'grep -q "MISS  the-desk model:.*resolved DESK_MODEL_the_desk=opus" <<<"$out"'

set_model fable
out="$("$CELLCTL" check house-cell 2>&1)" && rc=0 || rc=$?
assert "check passes on a fable pin" 'grep -q "ok    the-desk model: fable" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "model-pin.test.sh: OK"; else echo "model-pin.test.sh: $fails FAILED"; exit 1; fi
