#!/usr/bin/env bash
# model-namespace.test.sh — per-harness model-pin NAMESPACES and the TIER-MAP fallback (#986).
#
# The bug: `--harness codex` passed DESK_MODEL_<role> (a Claude model name — `fable`, `opus`,
# `sonnet`, ...) straight through to `codex -m`, which codex does not understand, and the reverse
# (a codex-only pin reaching the claude arm) was equally broken. This file proves the fix:
#
# What it proves (each an `assert` below):
#   desk    a cell pinned Claude-only (DESK_MODEL_the_desk=fable, no CODEX_MODEL_the_desk) resolves
#           --harness codex via the TIER MAP to the compiled top-tier codex model name, with no
#           manual re-pin — and --harness claude (the default) still resolves to the unchanged
#           Claude pin (fable), proving the two namespaces never cross-read each other
#   desk    a per-role CODEX_MODEL_<role> pin wins over the tier map when both harnesses' calls
#           target the same role, and CODEX_MODEL_default is consulted before the tier map falls in
#   desk    an explicit --model <m> on --harness codex passes through VERBATIM — it is never routed
#           through the namespace/tier resolution, even when it happens to name a Claude alias
#   desk    a role/harness combination with NEITHER a per-role pin NOR a tier match (the tier entry
#           itself removed via cell.env) is a clean refusal (non-zero exit) naming what was checked
#   check   prints one "model pin: role=... harness=... model=..." row per role, on the cell's
#           pinned CELL_HARNESS; a role with no pin and no tier match is a MISS naming what was
#           checked, surfaced here rather than as a startup failure
#   set     `cellctl set <cell> <role> --harness codex --model X` writes CODEX_MODEL_<role>=X, never
#           DESK_MODEL_<role>; the plain `--harness claude` (or no --harness, cell.env default) form
#           writes DESK_MODEL_<role>=X as before; the tier map's compiled defaults are themselves
#           overridable per entry via a TIER_MODEL_<TIER>_<HARNESS> cell.env key
#
# No network, no tmux, no real desk-tools: `claude` and `codex` are stubs on a private PATH, and the
# "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The binary under test. $CELLCTL lets the SAME suite run against either implementation
# (the bash oracle, the default, or the Go port) — desk-containers/10.
CELLCTL="${CELLCTL:-$HERE/../cellctl}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-modelns.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/CODEX_MODEL_*/
# TIER_MODEL_*/DESK_ROOTS/DESK_LOOP/DESK_SESSION/CELL_HARNESS vars — unset them so the fixture below
# is the only source cellctl reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  CELL_HARNESS DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES \
  FORGE_API_BASE GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_MODEL_OVERRIDE \
  CODEX_MODEL_default CODEX_MODEL_the_desk CODEX_MODEL_worker_desk \
  TIER_MODEL_TOP_CLAUDE TIER_MODEL_MID_CLAUDE TIER_MODEL_FAST_CLAUDE \
  TIER_MODEL_TOP_CODEX TIER_MODEL_MID_CODEX TIER_MODEL_FAST_CODEX \
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
mkdir -p "$REPO/plugins/assay/codex"
printf '## Assay resident operating rules\n\n1. EVIDENCE-NOT-CLAIMS: ...\n' > "$REPO/plugins/assay/codex/AGENTS-assay.md"
cp "$REPO/plugins/assay/codex/AGENTS-assay.md" "$REPO/AGENTS.md"
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
export CODEX_TEST_DIR="$T/codex-state"; mkdir -p "$CODEX_TEST_DIR"
cat > "$T/bin/codex" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  --version) echo "codex-cli 0.1.0-test"; exit 0 ;;
esac
case "${1:-} ${2:-}" in
  "login status") exit 0 ;;
  "config --help") printf 'Commands:\n  get     read a config value\n'; exit 0 ;;
  "config get") echo "true"; exit 0 ;;
  "plugin --help") printf 'Commands:\n  list    list installed plugins\n'; exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","enabled":true}]\n'; exit 0 ;;
esac
echo "ARGS=$*" > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/codex"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"

set_kv(){ # replace or add VAR=VAL in a cell's cell.env; VAL="" removes the line entirely
  local cellenv="$1" var="$2" val="$3"
  grep -v "^${var}=" "$cellenv" > "$cellenv.tmp" 2>/dev/null || true
  [[ -z "$val" && "${4:-}" != "--empty-ok" ]] || printf '%s=%s\n' "$var" "$val" >> "$cellenv.tmp"
  mv "$cellenv.tmp" "$cellenv"
}

"$CELLCTL" new ns-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/ns-cell"

# ---------------------------------------------------------------- 1: tier-map fallback, no re-pin
echo "[desk: Claude-only cell, --harness codex resolves via the tier map]"
assert "fixture starts Claude-only: DESK_MODEL_the_desk=fable" 'grep -qx "DESK_MODEL_the_desk=fable" "$CELL/cell.env"'
assert "fixture carries no CODEX_MODEL_the_desk" '! grep -q "^CODEX_MODEL_the_desk=" "$CELL/cell.env"'
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell the-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "resolves (exit 0), no manual re-pin needed" '[[ $rc -eq 0 ]]'
assert "resolves to the tier map's top-tier codex model, tagged with its source" \
  'grep -q "model=gpt-5.6-terra (tier:top (TIER_MODEL_TOP_CODEX))" <<<"$out"'
export CELLCTL_TEST_OUT="$T/launch-tier.env"
"$CELLCTL" desk ns-cell the-desk --harness codex >/dev/null
assert "the resolved tier value actually reaches the codex launch (-m gpt-5.6-terra)" \
  'grep -q -- "-m gpt-5.6-terra" "$CELLCTL_TEST_OUT"'

echo "[desk: --harness claude (default) is unaffected — still fable]"
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "claude arm resolves to the unchanged Claude pin" 'grep -q "model=fable" <<<"$out" && [[ $rc -eq 0 ]]'

# ---------------------------------------------------------------- 2: per-role/default codex pins
echo "[desk: CODEX_MODEL_<role> wins over the tier map]"
set_kv "$CELL/cell.env" CODEX_MODEL_the_desk "codex-top-special"
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell the-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "the explicit CODEX_MODEL_the_desk pin wins (exit 0)" '[[ $rc -eq 0 ]]'
assert "shows the pinned codex value, no tier tag" 'grep -q "model=codex-top-special" <<<"$out" && ! grep -q "tier:" <<<"$out"'
set_kv "$CELL/cell.env" CODEX_MODEL_the_desk ""

echo "[desk: CODEX_MODEL_default is consulted before the tier map]"
set_kv "$CELL/cell.env" CODEX_MODEL_default "codex-house-default"
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell worker-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "worker-desk (no per-role codex pin) resolves to CODEX_MODEL_default" 'grep -q "model=codex-house-default" <<<"$out"'
set_kv "$CELL/cell.env" CODEX_MODEL_default ""

# ---------------------------------------------------------------- 3: --model bypasses everything
echo "[desk: --harness codex --model <explicit> passes through verbatim]"
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell the-desk --harness codex --model claude-fable-9 2>&1)" && rc=0 || rc=$?
assert "an explicit --model is not remapped, even naming a Claude alias" \
  '[[ $rc -eq 0 ]] && grep -q "model=claude-fable-9 (override)" <<<"$out"'
export CELLCTL_TEST_OUT="$T/launch-explicit.env"
"$CELLCTL" desk ns-cell worker-desk --harness codex --model claude-fable-9 >/dev/null
assert "the literal override value reaches the codex launch line, untouched" \
  'grep -q -- "-m claude-fable-9" "$CELLCTL_TEST_OUT"'

# ---------------------------------------------------------------- 4: no pin, no tier match → refusal
echo "[desk: no per-role pin and no tier match is refused]"
set_kv "$CELL/cell.env" TIER_MODEL_MID_CODEX "" --empty-ok
printf 'TIER_MODEL_MID_CODEX=\n' >> "$CELL/cell.env"
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell worker-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit) rather than launching on a bogus/empty model" '[[ $rc -ne 0 ]]'
assert "names what it checked" 'grep -q "no model resolves for role .worker-desk. harness .codex." <<<"$out" && grep -q "CODEX_MODEL_worker_desk" <<<"$out" && grep -q "CODEX_MODEL_default" <<<"$out" && grep -q "mid" <<<"$out"'

# ---------------------------------------------------------------- 5: check — per-role rows
echo "[check: per-role harness + resolved-model rows]"
# The cell's PINNED harness is still claude at this point — the missing TIER_MODEL_MID_CODEX entry
# is a codex-only gap, so a claude-harness check is UNAFFECTED (namespaces never cross-read) and
# still passes; the mismatch only surfaces once the cell's harness is actually codex, below.
out="$("$CELLCTL" check ns-cell 2>&1)" && rc=0 || rc=$?
assert "check still passes on the claude harness (the codex-only gap does not affect it)" '[[ $rc -eq 0 ]]'
assert "ok row for the-desk on the claude harness" 'grep -q "ok    model pin: role=the-desk harness=claude model=fable (from DESK_MODEL_the_desk)" <<<"$out"'
assert "ok row for worker-desk on the claude harness too" 'grep -q "ok    model pin: role=worker-desk harness=claude" <<<"$out"'

# Switch the cell to codex as its pinned harness to see the mismatch surface there specifically.
"$CELLCTL" set ns-cell CELL_HARNESS=codex >/dev/null
out="$("$CELLCTL" check ns-cell 2>&1)" && rc=0 || rc=$?
assert "check fails (exit non-zero) with CELL_HARNESS=codex and the mid tier entry missing" '[[ $rc -ne 0 ]]'
assert "MISS row for worker-desk on the codex harness names the gap" \
  'grep -q "MISS  model pin: role=worker-desk harness=codex" <<<"$out"'
assert "the-desk (top tier, untouched) still resolves ok under codex" \
  'grep -q "ok    model pin: role=the-desk harness=codex model=gpt-5.6-terra (from tier:top (TIER_MODEL_TOP_CODEX))" <<<"$out"'

# restore the mid-tier codex entry and the claude harness for the remaining tests
grep -v '^TIER_MODEL_MID_CODEX=' "$CELL/cell.env" > "$CELL/cell.env.tmp" && mv "$CELL/cell.env.tmp" "$CELL/cell.env"
"$CELLCTL" set ns-cell CELL_HARNESS=claude >/dev/null
out="$("$CELLCTL" check ns-cell 2>&1)" && rc=0 || rc=$?
assert "check passes again once the tier entry is restored" '[[ $rc -eq 0 ]]'

# ---------------------------------------------------------------- 6: TIER_MODEL_* override
echo "[desk: TIER_MODEL_<TIER>_<HARNESS> is overridable in cell.env]"
"$CELLCTL" set ns-cell TIER_MODEL_TOP_CODEX=my-custom-top-codex >/dev/null
out="$(DRY_RUN=1 "$CELLCTL" desk ns-cell the-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "the-desk now resolves to the overridden tier value" 'grep -q "model=my-custom-top-codex" <<<"$out"'
"$CELLCTL" set ns-cell TIER_MODEL_TOP_CODEX=gpt-5.6-terra >/dev/null

# ---------------------------------------------------------------- 7: set — role-sugar, harness-aware
echo "[set: role-sugar form writes the ACTIVE harness's own namespace]"
out="$("$CELLCTL" set ns-cell worker-desk --harness codex --model my-codex-worker 2>&1)" && rc=0 || rc=$?
assert "set exits 0" '[[ $rc -eq 0 ]]'
assert "writes CODEX_MODEL_worker_desk, not DESK_MODEL_worker_desk" \
  'grep -qx "CODEX_MODEL_worker_desk=my-codex-worker" "$CELL/cell.env" && ! grep -q "^DESK_MODEL_worker_desk=my-codex-worker" "$CELL/cell.env"'

out="$("$CELLCTL" set ns-cell worker-desk --harness claude --model my-claude-worker 2>&1)" && rc=0 || rc=$?
assert "the claude form writes DESK_MODEL_worker_desk as before" \
  'grep -qx "DESK_MODEL_worker_desk=my-claude-worker" "$CELL/cell.env"'

# the cell's own CELL_HARNESS (claude, restored above) is used when --harness is omitted
out="$("$CELLCTL" set ns-cell worker-desk --model my-implicit-claude-worker 2>&1)" && rc=0 || rc=$?
assert "omitting --harness resolves the namespace from the cell's own CELL_HARNESS (claude)" \
  'grep -qx "DESK_MODEL_worker_desk=my-implicit-claude-worker" "$CELL/cell.env"'

out="$("$CELLCTL" set ns-cell worker-desk 2>&1)" && rc=0 || rc=$?
assert "the role-sugar form needs --model" '[[ $rc -ne 0 ]] && grep -q -- "--model" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "model-namespace.test.sh: OK"; else echo "model-namespace.test.sh: $fails FAILED"; exit 1; fi
