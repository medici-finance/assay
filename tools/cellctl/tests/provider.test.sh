#!/usr/bin/env bash
# provider.test.sh — cellctl's --provider / CELL_PROVIDER, against a fixture house cell.
#
# What it proves (each an `assert` below):
#   desk   no provider (unset CELL_PROVIDER, no --provider): launches exactly as before — no
#          ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN, dry-run says provider=anthropic
#          --provider <name> with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV declared and the named
#          token env var set in THIS shell: launches with ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN
#          exported from those, dry-run says provider=<name>
#          --provider <name> with the BASE_URL var missing: refused, names the missing variable
#          --provider <name> with the TOKEN_ENV var missing: refused, names the missing variable
#          --provider <name> with TOKEN_ENV declared but that env var unset in this shell: refused,
#          names the env var — and never the token value, because there is none to leak
#          CELL_PROVIDER in cell.env is the default when --provider is not given; --provider
#          overrides it for one run without touching cell.env
#   check  CELL_PROVIDER unset → n/a row, not a MISS (a provider is opt-in)
#          CELL_PROVIDER set and fully configured, token present → all three rows ok
#          CELL_PROVIDER set but the token env var is unset in this shell → MISS (per the brief:
#          "check MISSes when the token env var named by CELL_PROVIDER_<NAME>_TOKEN_ENV is unset")
#   up     --provider threads onto every role window's own `cellctl desk` invocation (the same
#          mechanism --model uses, verified in model-override.test.sh)
#   set    `cellctl set <cell> CELL_PROVIDER=<name>` and the CELL_PROVIDER_<NAME>_* keys are
#          accepted without --force (an unrelated key still needs it)
#
# No network: the "provider" here is a fake base URL, and the token is a fixture string in an
# env var this suite sets — nothing is sent anywhere. `claude` is a stub that records its argv
# and env instead of running. The assert strings are single-quoted on purpose (eval'd at assert
# time), and the variables they read look unused to a static pass.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-provider.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

unset CELL CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELL_COCKPIT CELLS_CONFIG ROLES DESKD \
      DESKD_ADDR DESKD_INDEX DESK_MODEL_DEFAULT DESK_MODEL_the_desk TMUX_SESSION CELL_ATTENDED \
      CELL_PROVIDER CELL_PROVIDER_ZAI_BASE_URL CELL_PROVIDER_ZAI_TOKEN_ENV ZAI_API_KEY \
      MODEL_OVERRIDE DESK_MODEL_OVERRIDE

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

export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
printf '#!/usr/bin/env bash\necho "$HOME" > "%s/deskroster.home"\nexit 0\n' "$T" > "$DESK_TOOLS_BIN/deskroster"
chmod +x "$DESK_TOOLS_BIN/deskroster"
# claude stub: plugin housekeeping answers as enabled; anything else records env+argv instead of
# running (mirrors house-cell.test.sh's fixture exactly).
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
{
  echo "ANTHROPIC_BASE_URL=${ANTHROPIC_BASE_URL:-}"
  echo "ANTHROPIC_AUTH_TOKEN=${ANTHROPIC_AUTH_TOKEN:-}"
  echo "ARGS=$*"
} > "$CELLCTL_TEST_OUT"
EOF
chmod +x "$T/bin/claude"
printf '#!/usr/bin/env bash\nexit 1\n' > "$T/bin/tmux"; chmod +x "$T/bin/tmux"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"

# ---------------------------------------------------------------- desk: no provider (baseline)
echo "[desk: no provider]"
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk 2>&1)"
assert "dry-run with no provider says provider=anthropic" 'grep -q "provider=anthropic" <<<"$out"'
export CELLCTL_TEST_OUT="$T/launch-baseline.env"
"$CELLCTL" desk example-cell worker-desk >/dev/null 2>&1
assert "no provider → ANTHROPIC_BASE_URL is NOT exported" 'grep -qx "ANTHROPIC_BASE_URL=" "$CELLCTL_TEST_OUT"'
assert "no provider → ANTHROPIC_AUTH_TOKEN is NOT exported" 'grep -qx "ANTHROPIC_AUTH_TOKEN=" "$CELLCTL_TEST_OUT"'

# ---------------------------------------------------------------- desk: --provider, fully configured
echo "[desk: --provider zai, fully configured]"
export CELL_PROVIDER_ZAI_BASE_URL="https://api.z.ai/api/anthropic"
export CELL_PROVIDER_ZAI_TOKEN_ENV="ZAI_API_KEY"
export ZAI_API_KEY="fixture-token-value-not-real"
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk --provider zai 2>&1)"
assert "dry-run with --provider zai says provider=zai" 'grep -q "provider=zai" <<<"$out"'
export CELLCTL_TEST_OUT="$T/launch-zai.env"
"$CELLCTL" desk example-cell worker-desk --provider zai >/dev/null 2>&1
assert "ANTHROPIC_BASE_URL exported from CELL_PROVIDER_ZAI_BASE_URL" 'grep -qxF "ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic" "$CELLCTL_TEST_OUT"'
assert "ANTHROPIC_AUTH_TOKEN exported from \$ZAI_API_KEY (never the var name)" 'grep -qxF "ANTHROPIC_AUTH_TOKEN=fixture-token-value-not-real" "$CELLCTL_TEST_OUT"'
assert "the model NAME is unaffected by the provider" 'grep -q -- "--model sonnet" "$CELLCTL_TEST_OUT"'

# ---------------------------------------------------------------- desk: missing pieces refuse cleanly
echo "[desk: provider misconfiguration refuses, never guesses]"
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk --provider kimi 2>&1)" && rc=0 || rc=$?
assert "unknown provider (no BASE_URL declared) is refused" '[[ $rc -ne 0 ]] && grep -q "CELL_PROVIDER_KIMI_BASE_URL is not set" <<<"$out"'

unset CELL_PROVIDER_ZAI_TOKEN_ENV
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk --provider zai 2>&1)" && rc=0 || rc=$?
assert "BASE_URL set but TOKEN_ENV missing is refused, names the variable" '[[ $rc -ne 0 ]] && grep -q "CELL_PROVIDER_ZAI_TOKEN_ENV is not set" <<<"$out"'
export CELL_PROVIDER_ZAI_TOKEN_ENV="ZAI_API_KEY"

unset ZAI_API_KEY
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk --provider zai 2>&1)" && rc=0 || rc=$?
assert "TOKEN_ENV declared but unset in this shell is refused, names the env var" '[[ $rc -ne 0 ]] && grep -q "\$ZAI_API_KEY" <<<"$out" && grep -q "not set in this shell" <<<"$out"'
export ZAI_API_KEY="fixture-token-value-not-real"

# ---------------------------------------------------------------- CELL_PROVIDER default in cell.env
echo "[desk: CELL_PROVIDER default]"
printf 'CELL_PROVIDER=zai\n' >> "$CELL/cell.env"
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk 2>&1)"
assert "cell.env CELL_PROVIDER is used when --provider is not given" 'grep -q "provider=zai" <<<"$out"'
out="$(DRY_RUN=1 "$CELLCTL" desk example-cell worker-desk --provider anthropic 2>&1)" && rc=0 || rc=$?
# "anthropic" is not a declared provider name (no CELL_PROVIDER_ANTHROPIC_* vars) — an explicit
# --provider always resolves through the same mechanism, it does not special-case the word
# "anthropic" as "no provider". Declaring anthropic's own vars would be how to override BACK.
assert "an explicit --provider overrides cell.env's default for one run" '[[ $rc -ne 0 ]] && grep -q "CELL_PROVIDER_ANTHROPIC_BASE_URL is not set" <<<"$out"'
sed -i.bak '/^CELL_PROVIDER=zai$/d' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"

# ---------------------------------------------------------------- check
echo "[check]"
out="$("$CELLCTL" check example-cell 2>&1)"; rc=$?
assert "CELL_PROVIDER unset → n/a, not a MISS (opt-in), check still exits 0" '[[ $rc -eq 0 ]] && grep -q "n/a   provider — CELL_PROVIDER unset" <<<"$out"'

printf 'CELL_PROVIDER=zai\n' >> "$CELL/cell.env"
out="$("$CELLCTL" check example-cell 2>&1)"; rc=$?
assert "CELL_PROVIDER set + fully configured + token present → exits 0" '[[ $rc -eq 0 ]]'
assert "... base URL row ok" 'grep -q "ok    provider zai: CELL_PROVIDER_ZAI_BASE_URL" <<<"$out"'
assert "... token-env-name row ok" 'grep -q "ok    provider zai: CELL_PROVIDER_ZAI_TOKEN_ENV" <<<"$out"'
assert "... token-present row ok" 'grep -q "ok    provider zai: \$ZAI_API_KEY is set in this shell" <<<"$out"'

unset ZAI_API_KEY
out="$("$CELLCTL" check example-cell 2>&1)" && rc=0 || rc=$?
assert "the named token env var unset in THIS shell → MISS, check fails" '[[ $rc -eq 1 ]] && grep -q "MISS  provider zai: \$ZAI_API_KEY is set in this shell" <<<"$out"'
export ZAI_API_KEY="fixture-token-value-not-real"
sed -i.bak '/^CELL_PROVIDER=zai$/d' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"

# ---------------------------------------------------------------- up: threads to every role window
echo "[up: --provider threads to every role window]"
out="$(DRY_RUN=1 "$CELLCTL" up example-cell --cockpit tmux --provider zai 2>&1)"
assert "up --provider dry-run announces it" 'grep -q "provider=zai (override)" <<<"$out"'
assert "the-desk window command carries --provider zai" "grep -q \"the-desk: .*desk 'example-cell' 'the-desk' --provider 'zai'\" <<<\"\$out\""
assert "worker-desk window command carries --provider zai too" "grep -q \"worker-desk: .*desk 'example-cell' 'worker-desk' --provider 'zai'\" <<<\"\$out\""

# ---------------------------------------------------------------- set: known-key allowlist
echo "[set]"
out="$("$CELLCTL" set example-cell CELL_PROVIDER=zai 2>&1)" && rc=0 || rc=$?
assert "'cellctl set' accepts CELL_PROVIDER without --force" '[[ $rc -eq 0 ]] && grep -qx "CELL_PROVIDER=zai" "$CELL/cell.env"'
out="$("$CELLCTL" set example-cell CELL_PROVIDER_ZAI_BASE_URL=https://api.z.ai/api/anthropic 2>&1)" && rc=0 || rc=$?
assert "'cellctl set' accepts CELL_PROVIDER_<NAME>_BASE_URL without --force" '[[ $rc -eq 0 ]]'
out="$("$CELLCTL" set example-cell CELL_PROVIDER_ZAI_TOKEN_ENV=ZAI_API_KEY 2>&1)" && rc=0 || rc=$?
assert "'cellctl set' accepts CELL_PROVIDER_<NAME>_TOKEN_ENV without --force" '[[ $rc -eq 0 ]]'
out="$("$CELLCTL" set example-cell SOME_RANDOM_KEY=x 2>&1)" && rc=0 || rc=$?
assert "an unrelated unknown key still needs --force (the allowlist is not wide open)" '[[ $rc -ne 0 ]] && grep -q "not a known cell.env key" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "provider.test.sh: OK"; else echo "provider.test.sh: $fails FAILED"; exit 1; fi
