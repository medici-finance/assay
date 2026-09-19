#!/usr/bin/env bash
# gen-shims-gh-token.test.sh — assay#1145: gen_shims' generated shim (tools/cellctl/cellctl,
# gen_shims()) resolves gh's ambient credential BEFORE swapping HOME for the wrapped desk verb,
# and threads it through as GH_TOKEN, so a `gh` subprocess the verb shells out to still
# authenticates even though the verb's own config/state is isolated to the cell home.
#
# What it proves (each an `assert` below):
#   isolation   the wrapped verb's OWN $HOME is still the cell home, unchanged (the regression
#               floor — this fix must never widen the HOME swap gen_shims exists to provide)
#   gh auth     a `gh` subprocess the verb shells out to (simulating e.g. `gh api ...`) now
#               authenticates, carrying the token resolved under the REAL (pre-swap) HOME —
#               reproducing the issue's own repro (`env HOME=<cell-home> gh api ...` → 401) and
#               showing it now succeeds
#   override    an explicit GH_TOKEN already in the caller's env is passed through UNCHANGED and
#               the shim never shells out to `gh auth token` to overwrite it
#   no-gh       when `gh` is not on PATH at all, the shim falls back to its pre-fix behaviour
#               (HOME swap only, no crash) — not a new failure mode
#
# No network, no tmux, no real desk-tools, no real `gh`: everything on PATH is a fixture. The
# fixture `gh` mimics real gh's OWN priority order (GH_TOKEN env wins; otherwise its ambient
# lookup is keyed to $HOME) — it is not the real credential store, so a "REAL" HOME here just
# means "the HOME the fixture treats as having a working ambient credential", never an actual
# keychain/hosts.yml. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-shim-gh.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_* vars — unset them so the
# fixture below is the only source cellctl reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_MODEL_OVERRIDE \
  DESK_ROOTS DESK_LOOP DESK_SESSION GH_TOKEN GH_ENTERPRISE_TOKEN 2>/dev/null || true

# ---------------------------------------------------------------- fixtures
# The "REAL" (operator) HOME the live session keeps — the fixture `gh`'s ambient lookup only
# succeeds when it sees THIS HOME, mirroring the real keychain/hosts.yml being keyed to it.
REAL_HOME="$T/home"
export HOME="$REAL_HOME"; mkdir -p "$HOME/.config/gh"
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
for v in deskboot deskroster deskwt deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done

# The wrapped desk verb under test: records the $HOME it was launched with (the isolation floor),
# then shells out to `gh` exactly the way the issue describes ("a `gh api` call made from inside a
# shimmed verb's child process") and records the outcome.
MARKER="$T/deskboard.out"
cat > "$DESK_TOOLS_BIN/deskboard" <<EOF
#!/usr/bin/env bash
{
  echo "VERB_HOME=\$HOME"
  if out="\$(gh api some-endpoint 2>&1)"; then rc=0; else rc=\$?; fi
  echo "GH_CALL_RC=\$rc"
  echo "GH_CALL_OUT=\$out"
} > "$MARKER"
EOF
chmod +x "$DESK_TOOLS_BIN/deskboard"

# The fixture `gh`: `gh auth token` succeeds only under REAL_HOME (models the ambient
# keychain/hosts.yml lookup being keyed to the real HOME, per the issue); any other invocation
# (e.g. `gh api ...`) authenticates only when GH_TOKEN is already set — exactly real gh's own
# priority (an explicit token wins over ambient resolution).
cat > "$T/bin/gh" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "auth" && "\${2:-}" == "token" ]]; then
  if [[ "\$HOME" == "$REAL_HOME" ]]; then
    echo "FIXTURE_TOKEN_9f8e7d"; exit 0
  else
    echo "gh: authentication failed: no credential found for HOME=\$HOME" >&2; exit 1
  fi
fi
if [[ -n "\${GH_TOKEN:-}" ]]; then
  echo "GH_API_OK token=\$GH_TOKEN"; exit 0
else
  echo "gh: authentication failed (401) home=\$HOME token=<unset>" >&2; exit 1
fi
EOF
chmod +x "$T/bin/gh"

# The stub `claude` the boot launches (never invoked interactively by this test, but `desk` still
# execs it at the end of the boot, so it must exit cleanly).
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/claude"

export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"

# ---------------------------------------------------------------- boot the cell (real HOME, real gh on PATH)
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"
"$CELLCTL" desk example-cell worker-desk >/dev/null 2>"$T/desk-worker.err" || true
assert "shim generated for deskboard" '[[ -x "$CELL/shim/deskboard" ]]'
assert "isolation floor: shim still swaps HOME to the cell home for HOME=\"...\" literal" 'grep -q "HOME=\"$CELL/home\"" "$CELL/shim/deskboard"'

# ---------------------------------------------------------------- case: verb's own gh call now authenticates
echo "[fixed: gh subprocess inside a shimmed verb authenticates]"
"$CELL/shim/deskboard" >/dev/null 2>&1
assert "verb's own HOME is STILL isolated to the cell home (regression floor)" 'grep -qxF "VERB_HOME=$CELL/home" "$MARKER"'
assert "verb's gh subprocess authenticated (rc=0)" 'grep -qx "GH_CALL_RC=0" "$MARKER"'
assert "verb's gh subprocess carried the ambient token resolved under the REAL home" 'grep -qxF "GH_CALL_OUT=GH_API_OK token=FIXTURE_TOKEN_9f8e7d" "$MARKER"'

# ---------------------------------------------------------------- case: explicit GH_TOKEN override wins, no re-resolve
echo "[override: explicit GH_TOKEN in the caller env is never replaced]"
rm -f "$MARKER"
GH_TOKEN=OPERATOR_OVERRIDE_TOKEN "$CELL/shim/deskboard" >/dev/null 2>&1
assert "override still isolates HOME" 'grep -qxF "VERB_HOME=$CELL/home" "$MARKER"'
assert "override token passed through unchanged (never re-resolved to the fixture token)" 'grep -qxF "GH_CALL_OUT=GH_API_OK token=OPERATOR_OVERRIDE_TOKEN" "$MARKER"'

# ---------------------------------------------------------------- case: no gh on PATH — falls back, no crash
echo "[no-gh: falls back to the pre-fix shape, does not crash]"
rm -f "$MARKER"
PATH="/usr/bin:/bin" "$CELL/shim/deskboard" >/dev/null 2>&1
assert "no-gh case still isolates HOME (no crash)" 'grep -qxF "VERB_HOME=$CELL/home" "$MARKER"'
assert "no-gh case: verb's own gh call fails exactly as pre-fix (not a NEW failure mode — no gh on PATH at all, same as before this fix)" 'grep -qx "GH_CALL_RC=127" "$MARKER"'

echo
if [[ "$fails" -eq 0 ]]; then echo "gen-shims-gh-token.test.sh: OK"; else echo "gen-shims-gh-token.test.sh: $fails FAILED"; exit 1; fi
