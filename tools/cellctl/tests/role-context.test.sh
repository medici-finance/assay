#!/usr/bin/env bash
# role-context.test.sh — per-role starting context, CELL_ROLE_CONTEXT (#2438).
#
# What it proves (each an `assert` below), on a house cell with NO model policy:
#   undeclared  a cell without CELL_ROLE_CONTEXT launches a role with exactly
#               `claude --name <session> --model <m> /assay:<role>`, and neither `check` nor a
#               dry run says anything about role context
#   desk        with a declaration for one role, that role's REAL launch argv gains the context
#               (--strict-mcp-config, --agents, --append-system-prompt, one --settings carrying
#               only keys that take something away), the prompt stays the last argument, and a
#               role the declaration does not name launches exactly as before
#   check       names what each role will start with, and turns a missing memory directory into
#               a MISS with a non-zero exit
#   refusal     desk refuses a declaration naming something missing BEFORE the harness runs
#
# The feature exists in the Go implementation only; the shell oracle (the default $CELLCTL) has
# nothing to exercise, so against it the suite states itself n/a rather than failing — the same
# convention house-cell.test.sh uses for implementation-specific cases.
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private PATH,
# and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="${CELLCTL:-$HERE/../testdata/cellctl-shell-oracle.sh}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
is_shell_impl(){ head -c2 "$CELLCTL" 2>/dev/null | grep -q '#!'; }
if is_shell_impl; then
  echo "  n/a   role context is implemented in the Go cellctl only — run with CELLCTL=<built cellctl binary>"
  echo "role-context.test.sh: OK (n/a against the shell oracle)"
  exit 0
fi
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-rolectx.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_MODEL_OVERRIDE \
  DESK_ROOTS DESK_LOOP DESK_SESSION CELL_ROLE_CONTEXT CELL_MODEL_POLICY 2>/dev/null || true

# ---------------------------------------------------------------- fixtures
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
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
# The claude stub records the launch argv ONE ELEMENT PER LINE (the context's JSON values are
# compact, so each stays one line) — a space-joined record could not tell where a value ends.
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
[[ "${1:-}" == "--version" ]] && { echo "2.1.295 (stub)"; exit 0; }
for a in "$@"; do printf '%s\n' "$a"; done > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/claude"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
# The launches below are REAL (not dry runs). cellctl prefers an installed `deskwt role-init` for
# the role worktree, and `deskwt` is found on PATH, not in DESK_TOOLS_BIN — so on a machine that
# has desk-tools installed the real one would run. Take cellctl's own worktree path instead.
export CELLCTL_DESKWT=0

"$CELLCTL" new ctx-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/ctx-cell"

launch(){ # launch <role> <record file> — a REAL (non-dry-run) launch against the stub harness
  CELLCTL_TEST_OUT="$2" "$CELLCTL" desk ctx-cell "$1" >/dev/null 2>"$2.err"
}
arg_after(){ awk -v f="$2" 'p{print; exit} $0==f{p=1}' "$1"; }  # the argv element after a flag
shape(){ sed -E 's/-[0-9]{8}T[0-9]{6}Z$//' "$1"; }                # drop the session timestamp

# ---------------------------------------------------------------- undeclared: nothing changes
echo "[undeclared: today's launch, and silence]"
launch pr-review-desk "$T/review-before" && rc=0 || rc=$?
launch worker-desk "$T/worker-before" && rc2=0 || rc2=$?
assert "both roles launch (exit 0)" '[[ $rc -eq 0 && $rc2 -eq 0 ]]'
assert "undeclared argv is exactly --name <s> --model <m> /assay:<role>" \
  '[[ "$(wc -l < "$T/review-before" | tr -d " ")" -eq 5 && "$(sed -n 1p "$T/review-before")" == "--name" && "$(sed -n 3p "$T/review-before")" == "--model" && "$(sed -n 5p "$T/review-before")" == "/assay:pr-review-desk" ]]'
out="$("$CELLCTL" check ctx-cell 2>&1; DRY_RUN=1 "$CELLCTL" desk ctx-cell pr-review-desk 2>&1)" || true
assert "check and a dry run say nothing about role context" '! grep -qi "context" <<<"$out"'

# ---------------------------------------------------------------- declared for one role
echo "[desk: one role declared]"
mkdir -p "$CELL/memory/pr-review-desk" "$CELL/context"
echo "- review notes" > "$CELL/memory/pr-review-desk/MEMORY.md"
echo "Review one change at a time." > "$CELL/context/review.md"
cat > "$CELL/role-context.json" <<'EOF'
{
  "version": 1,
  "roles": {
    "pr-review-desk": {
      "plugins_off": ["extras@example-market"],
      "memory_dir": "memory/pr-review-desk",
      "instructions": "context/review.md",
      "agents": ["builtin:lean-reviewer"],
      "dispatch_agent": "lean-reviewer",
      "agents_off": ["general-purpose"],
      "tools_off": ["WebFetch"],
      "connectors_off": true
    }
  }
}
EOF
out="$("$CELLCTL" set ctx-cell CELL_ROLE_CONTEXT=role-context.json 2>&1)" && rc=0 || rc=$?
assert "cellctl set accepts CELL_ROLE_CONTEXT without --force" '[[ $rc -eq 0 ]] && grep -qx "CELL_ROLE_CONTEXT=role-context.json" "$CELL/cell.env"'

launch pr-review-desk "$T/review-after" && rc=0 || rc=$?
launch worker-desk "$T/worker-after" && rc2=0 || rc2=$?
assert "both roles still launch (exit 0)" '[[ $rc -eq 0 && $rc2 -eq 0 ]]'
assert "a role with no entry launches with the argv it had before" '[[ "$(shape "$T/worker-after")" == "$(shape "$T/worker-before")" ]]'
assert "the prompt is still the last argument" '[[ "$(tail -n 1 "$T/review-after")" == "/assay:pr-review-desk" ]]'
assert "--name and --model are unchanged" '[[ "$(arg_after "$T/review-after" --model)" == "$(arg_after "$T/review-before" --model)" && -n "$(arg_after "$T/review-after" --name)" ]]'
assert "connectors_off adds --strict-mcp-config" 'grep -qx -- "--strict-mcp-config" "$T/review-after"'
SETTINGS="$(arg_after "$T/review-after" --settings)"
WANT='{"autoMemoryDirectory":"'"$CELL"'/memory/pr-review-desk","disableClaudeAiConnectors":true,"enabledPlugins":{"extras@example-market":false},"permissions":{"deny":["WebFetch","Agent(general-purpose)"]}}'
assert "--settings carries exactly the narrowing keys" '[[ "$SETTINGS" == "$WANT" ]]'
AGENTS="$(arg_after "$T/review-after" --agents)"
assert "--agents installs the lean reviewer: shell + file read/write, no inherited instruction files" \
  'grep -q "\"lean-reviewer\":{" <<<"$AGENTS" && grep -q "\"tools\":\[\"Bash\",\"Read\",\"Write\",\"Edit\"\]" <<<"$AGENTS" && grep -q "\"omitClaudeMd\":true" <<<"$AGENTS"'
assert "an agent definition never carries a model or a permission mode" '! grep -qE "\"(model|permissionMode|hooks|mcpServers)\"" <<<"$AGENTS"'
assert "the role instruction file and the dispatch line are appended to the system prompt" \
  'grep -qx "Review one change at a time." "$T/review-after" && grep -q "dispatch it as the \"lean-reviewer\" agent type" "$T/review-after"'

# ---------------------------------------------------------------- check + dry run report it
echo "[check / dry run: what each role starts with]"
out="$("$CELLCTL" check ctx-cell 2>&1)" && rc=0 || rc=$?
assert "check passes on a valid declaration" '[[ $rc -eq 0 ]]'
assert "check names the declared role's context" 'grep -q "ok    role context: pr-review-desk plugins_off=extras@example-market" <<<"$out" && grep -q "agents=lean-reviewer dispatch_agent=lean-reviewer agents_off=general-purpose tools_off=WebFetch connectors=off" <<<"$out"'
assert "check says an undeclared role starts with the cell-wide context" 'grep -q "n/a   role context: worker-desk — nothing declared" <<<"$out"'
assert "check warns that plugins_off names a plugin nothing enables" 'grep -q "warn  role context: pr-review-desk plugins_off names extras@example-market" <<<"$out"'
out="$(DRY_RUN=1 "$CELLCTL" desk ctx-cell pr-review-desk 2>&1)" && rc=0 || rc=$?
assert "dry run shows the context and the flags it adds, sizes not contents" \
  '[[ $rc -eq 0 ]] && grep -q "\[dry-run\] context role=pr-review-desk source=" <<<"$out" && grep -q "context flags: --strict-mcp-config --agents=<" <<<"$out" && ! grep -q "Review one change" <<<"$out"'

# ---------------------------------------------------------------- something missing: refused
echo "[refusal: a declaration naming something missing]"
rm -rf "$CELL/memory/pr-review-desk"
out="$("$CELLCTL" check ctx-cell 2>&1)" && rc=0 || rc=$?
assert "check exits non-zero and names the missing directory" '[[ $rc -ne 0 ]] && grep -q "MISS  role-context: role pr-review-desk: memory_dir" <<<"$out"'
rm -f "$T/review-refused"
launch pr-review-desk "$T/review-refused" && rc=0 || rc=$?
assert "desk refuses and the harness never runs" '[[ $rc -ne 0 && ! -e "$T/review-refused" ]] && grep -q "role-context: role pr-review-desk: memory_dir" "$T/review-refused.err"'
launch worker-desk "$T/worker-refused" && rc=0 || rc=$?
assert "a broken declaration stops every role of the cell, not only the one it names" '[[ $rc -ne 0 && ! -e "$T/worker-refused" ]] && grep -q "role-context: role pr-review-desk: memory_dir" "$T/worker-refused.err"'

echo
if [[ "$fails" -eq 0 ]]; then echo "role-context.test.sh: OK"; else echo "role-context.test.sh: $fails FAILED"; exit 1; fi
