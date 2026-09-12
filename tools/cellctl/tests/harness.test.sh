#!/usr/bin/env bash
# harness.test.sh — `--harness <claude|codex>` on `desk`/`up`, CELL_HARNESS in cell.env, the codex
# launch arm, and `check`'s codex harness block (issue-946).
#
# What it proves (each an `assert` below):
#   new     scaffolds CELL_HARNESS=claude in cell.env (default) for k8s and house alike
#   desk    default (no --harness, CELL_HARNESS=claude) boots the claude arm unchanged; --harness
#           codex is visible in DRY_RUN=1 (harness=codex) and, live, execs
#           `codex --sandbox danger-full-access -C <wt> -m <model> "<invoke-by-name prompt>"` with
#           the same DESK_LOOP/DESK_SESSION/DESK_ROOTS/shim-PATH env the claude arm gets; an
#           unknown --harness value is refused (non-zero exit); the resident-rules fragment is
#           appended to the worktree's AGENTS.md on first codex boot and NOT duplicated on a second
#           boot of the same worktree; the roster beacon (DESK_SESSION) notes a non-claude harness;
#           the the-desk/Opus refusal binds the claude arm only — an Opus pin is NOT refused when
#           --harness codex is given, and prints the resolved model as is
#   check   a claude cell reports the codex harness block n/a; a codex cell reports MISS rows when
#           codex is missing from PATH, unauthenticated, or multi_agent is off, and passes when
#           every codex precondition holds
#   up      DRY_RUN=1 --harness codex threads `--harness 'codex'` onto every role window's own
#           `cellctl desk` invocation, and announces the override once
#
# No network, no tmux, no real desk-tools: `claude`, `codex` and the desk verbs are stubs on a
# private PATH, and the "operator config home" is a temp directory. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-harness.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_MODEL_*/DESK_ROOTS/DESK_LOOP/
# DESK_SESSION/CELL_HARNESS vars (a cellctl-booted session sets them for its own window, and a
# nested one inherits them on exec) — unset them so the fixture below is the only source cellctl
# reads.
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  CELL_HARNESS DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES \
  FORGE_API_BASE GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
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
# The resident-rules fragment lives in the CHECKOUT (CELL_REPO), not in the worktree — cellctl
# reads it from there and appends it to the worktree's own AGENTS.md at boot.
mkdir -p "$REPO/plugins/assay/codex"
printf '## Assay resident operating rules\n\n1. EVIDENCE-NOT-CLAIMS: ...\n' > "$REPO/plugins/assay/codex/AGENTS-assay.md"
# The checkout's OWN root AGENTS.md already carries the fragment (the steady state an adopter
# reaches by committing it) — `check`'s resident-rules row reads THIS file; the codex boot arm's
# own idempotent-append (proven separately below) targets the per-role WORKTREE's AGENTS.md, a
# different file entirely.
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
{
  echo "PWD=$(pwd -P)"
  echo "DESK_ROOTS=${DESK_ROOTS:-}"
  echo "DESK_LOOP=${DESK_LOOP:-}"
  echo "DESK_SESSION=${DESK_SESSION:-}"
  echo "ARGS=$*"
} > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/claude"
# The codex stub. Control files under $CODEX_TEST_DIR (exported, so it survives cellctl's own
# `env VAR=... codex ...` exec — env only ADDS vars, it does not clear the inherited ones) toggle
# authentication, the multi_agent config value, and skills-discoverable, independent of PATH
# presence (removing/restoring the binary itself is the "codex missing" case).
export CODEX_TEST_DIR="$T/codex-state"; mkdir -p "$CODEX_TEST_DIR"
cat > "$T/bin/codex" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  --version) echo "codex-cli 0.1.0-test"; exit 0 ;;
esac
case "${1:-} ${2:-}" in
  "login status")
    [[ -f "$CODEX_TEST_DIR/authed" ]] && exit 0 || exit 1 ;;
  "config --help") printf 'Commands:\n  get     read a config value\n  set     write a config value\n'; exit 0 ;;
  "config get")
    if [[ -f "$CODEX_TEST_DIR/multiagent" ]]; then cat "$CODEX_TEST_DIR/multiagent"; else echo ""; fi
    exit 0 ;;
  "plugin --help") printf 'Commands:\n  list    list installed plugins\n  add     add a plugin\n'; exit 0 ;;
  "plugin list")
    if [[ -f "$CODEX_TEST_DIR/skills" ]]; then cat "$CODEX_TEST_DIR/skills"; else echo ""; fi
    exit 0 ;;
esac
{
  echo "PWD=$(pwd -P)"
  echo "DESK_ROOTS=${DESK_ROOTS:-}"
  echo "DESK_LOOP=${DESK_LOOP:-}"
  echo "DESK_SESSION=${DESK_SESSION:-}"
  echo "CLAUDE_CONFIG_DIR=${CLAUDE_CONFIG_DIR:-}"
  echo "ARGS=$*"
} > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/codex"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
printf 'cells: []\n' > "$T/cells.yaml"
printf 'placeholder, not a real key\n' > "$T/app.pem"

# codex authenticated, multi_agent on, skills discoverable — the "everything holds" baseline the
# individual MISS tests below flip one piece of at a time.
touch "$CODEX_TEST_DIR/authed"
printf 'true\n' > "$CODEX_TEST_DIR/multiagent"
printf '[{"id":"assay@assay","enabled":true}]\n' > "$CODEX_TEST_DIR/skills"

# ---------------------------------------------------------------- new: CELL_HARNESS scaffolded
echo "[new: CELL_HARNESS scaffolded]"
"$CELLCTL" new gh-cell --kind k8s --forge github --repo "$REPO" --cells-yaml "$T/cells.yaml" \
  --orgs example-org --deskd-app-pem "$T/app.pem" >/dev/null
assert "k8s/github cell.env pins CELL_HARNESS=claude" 'grep -qx "CELL_HARNESS=claude" "$CELLS_ROOT/gh-cell/cell.env"'
"$CELLCTL" new house-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/house-cell"
assert "house cell.env pins CELL_HARNESS=claude" 'grep -qx "CELL_HARNESS=claude" "$CELL/cell.env"'

# ---------------------------------------------------------------- desk: default is claude
echo "[desk: default harness is claude]"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "dry-run exits 0" '[[ $rc -eq 0 ]]'
assert "dry-run shows harness=claude" 'grep -q "harness=claude" <<<"$out"'
export CELLCTL_TEST_OUT="$T/launch-default.env"
"$CELLCTL" desk house-cell the-desk >/dev/null
assert "the claude stub ran (unchanged arm)" 'grep -q "^ARGS=" "$CELLCTL_TEST_OUT" && grep -q -- "/assay:the-desk" "$CELLCTL_TEST_OUT"'

# ---------------------------------------------------------------- desk: --harness codex
echo "[desk: --harness codex]"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell worker-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "dry-run exits 0" '[[ $rc -eq 0 ]]'
assert "dry-run shows harness=codex" 'grep -q "harness=codex" <<<"$out"'

WT="$CELL/worktrees/worker-desk"
export CELLCTL_TEST_OUT="$T/launch-codex.env"
out="$("$CELLCTL" desk house-cell worker-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "live --harness codex exits 0" '[[ $rc -eq 0 ]]'
assert "launch line shows harness=codex" 'grep -q "harness=codex" <<<"$out"'
assert "the codex stub ran, not claude" 'grep -q "^ARGS=" "$CELLCTL_TEST_OUT"'
assert "codex invoked with --sandbox danger-full-access" 'grep -q -- "--sandbox danger-full-access" "$CELLCTL_TEST_OUT"'
assert "codex invoked with -C <worktree>" "grep -q -- \"-C $WT \" \"\$CELLCTL_TEST_OUT\""
assert "codex invoked with -m <model> (the resolved DESK_MODEL_DEFAULT)" 'grep -q -- "-m sonnet" "$CELLCTL_TEST_OUT"'
assert "codex prompt names the skill by invoke-by-name form" 'grep -q "assay:worker-desk" "$CELLCTL_TEST_OUT"'
assert "same DESK_ROOTS/DESK_LOOP/DESK_SESSION env as the claude arm" \
  'grep -qxF "DESK_ROOTS=$ROOTS" "$CELLCTL_TEST_OUT" && grep -qxF "DESK_LOOP=worker-desk" "$CELLCTL_TEST_OUT"'

# ---------------------------------------------------------------- desk: resident-rules fragment
echo "[desk: resident-rules fragment appended, idempotently]"
assert "AGENTS.md was created in the worktree" '[[ -f "$WT/AGENTS.md" ]]'
assert "fragment content is present" 'grep -qF "Assay resident operating rules" "$WT/AGENTS.md"'
occurrences_before="$(grep -c "Assay resident operating rules" "$WT/AGENTS.md")"
"$CELLCTL" desk house-cell worker-desk --harness codex >/dev/null 2>&1 || true
occurrences_after="$(grep -c "Assay resident operating rules" "$WT/AGENTS.md")"
assert "a second codex boot does not duplicate the fragment" '[[ "$occurrences_before" -eq 1 && "$occurrences_after" -eq 1 ]]'

# ---------------------------------------------------------------- desk: unknown harness refused
echo "[desk: unknown harness refused]"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --harness bogus 2>&1)" && rc=0 || rc=$?
assert "refused (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "names claude or codex" 'grep -q "must be claude or codex" <<<"$out"'

# ---------------------------------------------------------------- desk: Opus refusal binds claude only
echo "[desk: the-desk/Opus refusal binds the claude arm only]"
printf '%s\n' "DESK_MODEL_the_desk=opus" >> "$CELL/cell.env"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk 2>&1)" && rc=0 || rc=$?
assert "claude arm still refuses an Opus pin" '[[ $rc -ne 0 ]] && grep -q "resolved DESK_MODEL_the_desk=opus" <<<"$out"'
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell the-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "codex arm does NOT refuse the same Opus pin" '[[ $rc -eq 0 ]]'
assert "codex arm prints the resolved model as is" 'grep -q "model=opus harness=codex" <<<"$out"'
# restore a non-opus pin for the remaining tests
grep -v '^DESK_MODEL_the_desk=' "$CELL/cell.env" > "$CELL/cell.env.tmp" && mv "$CELL/cell.env.tmp" "$CELL/cell.env"

# ---------------------------------------------------------------- desk: roster beacon notes codex
echo "[desk: roster beacon / DESK_SESSION notes a non-claude harness]"
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell verify-desk --harness codex 2>&1)" && rc=0 || rc=$?
assert "DESK_SESSION in the dry-run plan carries a -codex suffix" 'grep -qE "session=house-cell-verify-desk-[0-9]{8}T[0-9]{6}Z-codex" <<<"$out"'
out="$(DRY_RUN=1 "$CELLCTL" desk house-cell verify-desk 2>&1)" && rc=0 || rc=$?
assert "the plain claude boot carries no -codex suffix" '! grep -q -- "-codex " <<<"$out"'

# ---------------------------------------------------------------- check: codex harness block
echo "[check: codex harness block is n/a on a claude cell]"
out="$("$CELLCTL" check house-cell 2>&1)" && rc=0 || rc=$?
assert "check passes on the claude cell" '[[ $rc -eq 0 ]]'
assert "codex harness row is n/a" 'grep -q "n/a   codex harness preconditions" <<<"$out"'

echo "[check: codex cell — everything holds]"
"$CELLCTL" new codex-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CCELL="$CELLS_ROOT/codex-cell"
printf 'CELL_HARNESS=codex\n' >> "$CCELL/cell.env"
out="$("$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check passes when every codex precondition holds" '[[ $rc -eq 0 ]]'
assert "codex on PATH row is ok" 'grep -q "ok    codex on PATH" <<<"$out"'
assert "codex --version row is ok" 'grep -q "ok    codex --version" <<<"$out"'
assert "codex authenticated row is ok" 'grep -q "ok    codex authenticated" <<<"$out"'
assert "multi_agent row is ok" 'grep -q "ok    \[features\] multi_agent" <<<"$out"'
assert "resident-rules fragment row is ok" 'grep -q "ok    resident-rules fragment present" <<<"$out"'
assert "skills discoverable row is ok" 'grep -q "ok    skills discoverable" <<<"$out"'

echo "[check: codex cell — codex missing from PATH]"
# A real `codex` may exist further down the operator's own $PATH (this machine has one for the
# harness-portability smoke runs) — moving the stub out of $T/bin is not enough, since the lookup
# would just fall through to that real binary. Build a PATH that mirrors every command reachable
# today EXCEPT codex (first match per basename wins, same precedence a normal PATH lookup gives).
mkdir -p "$T/bin-nocodex"
IFS=':' read -r -a _pdirs <<<"$PATH"
for _d in "${_pdirs[@]}"; do
  [[ -d "$_d" ]] || continue
  for _f in "$_d"/*; do
    [[ -e "$_f" ]] || continue
    _b="$(basename "$_f")"
    [[ "$_b" == "codex" ]] && continue
    [[ -e "$T/bin-nocodex/$_b" ]] && continue
    ln -sf "$_f" "$T/bin-nocodex/$_b"
  done
done
out="$(PATH="$T/bin-nocodex" "$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check fails when codex is missing (exit 1)" '[[ $rc -ne 0 ]]'
assert "names codex on PATH as the MISS" 'grep -q "MISS  codex on PATH" <<<"$out"'

echo "[check: codex cell — unauthenticated]"
rm -f "$CODEX_TEST_DIR/authed"
out="$("$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check fails when codex is unauthenticated (exit 1)" '[[ $rc -ne 0 ]]'
assert "names codex authenticated as the MISS" 'grep -q "MISS  codex authenticated" <<<"$out"'
touch "$CODEX_TEST_DIR/authed"

echo "[check: codex cell — multi_agent off]"
printf 'false\n' > "$CODEX_TEST_DIR/multiagent"
out="$("$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check fails when multi_agent is off (exit 1)" '[[ $rc -ne 0 ]]'
assert "names the multi_agent row as the MISS" 'grep -q "MISS  \[features\] multi_agent" <<<"$out"'
printf 'true\n' > "$CODEX_TEST_DIR/multiagent"

echo "[check: codex cell — resident-rules fragment missing]"
mv "$REPO/AGENTS.md" "$T/AGENTS.md.bak"
out="$("$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check fails when the fragment is not present in the checkout AGENTS.md (exit 1)" '[[ $rc -ne 0 ]]'
assert "names the resident-rules row as the MISS" 'grep -q "MISS  resident-rules fragment present" <<<"$out"'
mv "$T/AGENTS.md.bak" "$REPO/AGENTS.md"

echo "[check: codex cell — skills not discoverable]"
printf '' > "$CODEX_TEST_DIR/skills"
out="$("$CELLCTL" check codex-cell 2>&1)" && rc=0 || rc=$?
assert "check fails when neither discovery arm is present (exit 1)" '[[ $rc -ne 0 ]]'
assert "names skills discoverable as the MISS" 'grep -q "MISS  skills discoverable" <<<"$out"'
printf '[{"id":"assay@assay","enabled":true}]\n' > "$CODEX_TEST_DIR/skills"

# ---------------------------------------------------------------- up: --harness threads through
echo "[up: --harness threads to every role window]"
out="$(DRY_RUN=1 "$CELLCTL" up house-cell --cockpit tmux --harness codex 2>&1)" && rc=0 || rc=$?
assert "up --harness dry-run exits 0" '[[ $rc -eq 0 ]]'
assert "up announces the override once" 'grep -q "harness=codex — applied to every role window below" <<<"$out"'
assert "the-desk window command carries --harness codex" "grep -q \"the-desk: .*desk 'house-cell' 'the-desk' --harness 'codex'\" <<<\"\$out\""
assert "worker-desk window command carries --harness codex too" "grep -q \"worker-desk: .*desk 'house-cell' 'worker-desk' --harness 'codex'\" <<<\"\$out\""

out="$(DRY_RUN=1 "$CELLCTL" up house-cell --cockpit tmux 2>&1)" && rc=0 || rc=$?
assert "up without --harness carries no --harness flag in the per-role commands" '! grep -q -- "--harness" <<<"$out"'

echo
if [[ "$fails" -eq 0 ]]; then echo "harness.test.sh: OK"; else echo "harness.test.sh: $fails FAILED"; exit 1; fi
