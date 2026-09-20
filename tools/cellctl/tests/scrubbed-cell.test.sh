#!/usr/bin/env bash
# scrubbed-cell.test.sh — cellctl's `--kind scrubbed` host-local harness cell (desk-containers/09):
# a per-cell environment fully COMPOSED (`env -i` plus an explicit allowlist — nothing from the
# launching shell leaks in), `smoke`, `status`, the two-layer session lock, and the stricter
# `check` rows a scrubbed cell needs (real, never-symlinked config home; 0600 PEMs; a roster
# scoped to exactly one repo; harness login under the cell's own home).
#
# `--case <name>` runs exactly one case (each Verify row in the brief names its case); with no
# flag, every case runs in sequence. Unknown case name: exit 2.
#
# No network, no real credentials, no live model: `claude`, `codex` are stubs on a private PATH;
# `tmux` is the REAL binary (a private socket per cell — proving isolation needs a real session).
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and the
# variables they read look unused to a static pass.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The binary under test. $CELLCTL lets the SAME suite run against either implementation
# (the bash oracle, the default, or the Go port) — desk-containers/10.
CELLCTL="${CELLCTL:-$HERE/../cellctl}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
# /tmp, never $TMPDIR: a scrubbed cell's session lives on a private UNIX-socket tmux server
# (`<cell>/run/tmux.sock`), and `sun_path` has a hard ~104-108 byte limit every platform enforces
# — macOS's default $TMPDIR (/var/folders/.../T/, ~50 bytes on its own) blows that budget once
# `cells/<longest-case-name>/run/tmux.sock` is appended; /tmp does not.
T="$(cd "$(mktemp -d "/tmp/cellctl-scrubbed.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported CELL_*/DESK_* vars — unset them so the
# fixture below is the only source cellctl reads (harness.test.sh's own precaution).
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  CELL_HARNESS CELL_REPO_SLUG CELL_PATH DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM \
  DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE \
  DESKD_GITLAB_TOKEN_FILE DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk \
  DESK_ROOTS DESK_LOOP DESK_SESSION 2>/dev/null || true

# ---------------------------------------------------------------- shared fixture
export HOME="$T/home"; mkdir -p "$HOME/.config/gh" "$HOME/.claude"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
mkdir -p "$T/seed/plugins/assay/codex"
printf '## Assay resident operating rules\n\n1. EVIDENCE-NOT-CLAIMS: ...\n' > "$T/seed/plugins/assay/codex/AGENTS-assay.md"
cp "$T/seed/plugins/assay/codex/AGENTS-assay.md" "$T/seed/AGENTS.md"
git -C "$T/seed" add -A && git -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "seed"
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"

export CELLS_ROOT="$T/cells"
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done

# Stub claude/codex. Output paths are baked into the stub's SOURCE (expanded when the heredoc is
# written), never read from an environment variable at run time — the whole point of the scrubbed
# launch is that `env -i` strips everything not on the allowlist, a test-only var included.
CLAUDE_STATE="$T/claude-state"; CODEX_STATE="$T/codex-state"; mkdir -p "$CLAUDE_STATE" "$CODEX_STATE"
cat > "$T/bin/claude" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in
  --version) echo "claude 0.0.0-test"; exit 0 ;;
esac
if [[ "\${1:-}" == "-p" ]]; then
  { echo "ARGS=\$*"; echo "HOME=\$HOME"; } > "$T/claude-smoke.out"
  rc=0; [[ -f "$CLAUDE_STATE/exit-code" ]] && rc="\$(cat "$CLAUDE_STATE/exit-code")"
  if [[ -f "$CLAUDE_STATE/answer" ]]; then cat "$CLAUDE_STATE/answer"; else echo READY; fi
  exit "\$rc"
fi
{
  echo "PWD=\$(pwd -P)"
  for e in "\$@"; do echo "ARG=\$e"; done
  env | sort
} > "$T/claude-launch.out"
sleep 3
EOF
chmod +x "$T/bin/claude"
cat > "$T/bin/codex" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in --version) echo "codex-cli 0.1.0-test"; exit 0 ;; esac
case "\${1:-} \${2:-}" in
  "login status")
    [[ -f "$CODEX_STATE/authed" ]] && exit 0 || exit 1 ;;
  "config --help") printf 'Commands:\n  get     read a config value\n'; exit 0 ;;
  "config get")
    if [[ -f "$CODEX_STATE/multiagent" ]]; then cat "$CODEX_STATE/multiagent"; else echo ""; fi
    exit 0 ;;
  "plugin --help") printf 'Commands:\n  list    list installed plugins\n'; exit 0 ;;
  "plugin list")
    if [[ -f "$CODEX_STATE/skills" ]]; then cat "$CODEX_STATE/skills"; else echo ""; fi
    exit 0 ;;
esac
if [[ "\${1:-}" == "exec" ]]; then
  { echo "ARGS=\$*"; echo "HOME=\$HOME"; } > "$T/codex-smoke.out"
  rc=0; [[ -f "$CODEX_STATE/exit-code" ]] && rc="\$(cat "$CODEX_STATE/exit-code")"
  if [[ -f "$CODEX_STATE/answer" ]]; then cat "$CODEX_STATE/answer"; else echo READY; fi
  exit "\$rc"
fi
{
  echo "PWD=\$(pwd -P)"
  for e in "\$@"; do echo "ARG=\$e"; done
  env | sort
} > "$T/codex-launch.out"
sleep 3
EOF
chmod +x "$T/bin/codex"
# tmux stays the REAL binary — resolved from the system PATH appended below.
export PATH="$T/bin:$PATH"

# fresh_deadpid: a pid that WAS real (so it is not accidentally reused by something else right
# now) and is now reaped — a dead pid without guessing a magic number.
fresh_deadpid(){ ( : ) & local p=$!; wait "$p" 2>/dev/null || true; printf '%s' "$p"; }

# ---------------------------------------------------------------- new
case_new(){
  local cell="case-new"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  assert "new: cell.env written" '[[ -f "$C/cell.env" ]]'
  assert "new: cell.env CELL_KIND=scrubbed" 'grep -qx "CELL_KIND=scrubbed" "$C/cell.env"'
  assert "new: cell.env CELL_REPO_SLUG is the --repo-slug value" 'grep -qx "CELL_REPO_SLUG=example-org/example-repo" "$C/cell.env"'
  assert "new: cell.env DESKD=0" 'grep -qx "DESKD=0" "$C/cell.env"'
  assert "new: home/.config/assay is a REAL directory (never a symlink)" '[[ -d "$C/home/.config/assay" && ! -L "$C/home/.config/assay" ]]'
  assert "new: home/.config/assay mode 0700" '[[ "$(stat -f "%Lp" "$C/home/.config/assay" 2>/dev/null || stat -c "%a" "$C/home/.config/assay")" == "700" ]]'
  assert "new: roster scopes ASSAY_ALLOWED_REPOS to the slug" 'grep -qx "ASSAY_ALLOWED_REPOS=example-org/example-repo" "$C/home/.config/assay/roster.env"'
  assert "new: gitconfig present, empty, never linked to the operator's" '[[ -f "$C/home/.gitconfig" && ! -L "$C/home/.gitconfig" && ! -s "$C/home/.gitconfig" ]]'
  assert "new: gh config dir present, a REAL directory" '[[ -d "$C/home/.config/gh" && ! -L "$C/home/.config/gh" ]]'
  local rc=0
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null 2>&1 || rc=$?
  assert "new: second new on the same name REFUSES (exit 3)" '[[ $rc -eq 3 ]]'
  rc=0; "$CELLCTL" new case-new-norepo --kind scrubbed --repo-slug example-org/x >/dev/null 2>&1 || rc=$?
  assert "new: refuses without --repo" '[[ $rc -ne 0 ]]'
  rc=0; "$CELLCTL" new case-new-noslug --kind scrubbed --repo "$REPO" >/dev/null 2>&1 || rc=$?
  assert "new: refuses without --repo-slug" '[[ $rc -ne 0 ]]'
}

# ---------------------------------------------------------------- check-pass
case_check_pass(){
  local cell="case-check-pass"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  printf 'CELL_HARNESS=codex\n' >> "$C/cell.env"
  printf 'fake pem\n' > "$C/home/.config/assay/worker-desk-app.pem"; chmod 600 "$C/home/.config/assay/worker-desk-app.pem"
  # FULLY provisioned means the roster hand step the scaffold's own README names has run too.
  # `cellctl new --kind scrubbed` writes ASSAY_ALLOWED_REPOS and a "fill in by hand" comment for
  # the rest, and a roster with no bless authority does not LOAD — the real `deskroster` exits 6
  # on one. Without these two lines the case was asserting "all preconditions met" on a cell
  # whose roster no desk verb would accept.
  printf 'ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\n' >> "$C/home/.config/assay/roster.env"
  rm -rf "$CODEX_STATE"; mkdir -p "$CODEX_STATE"
  touch "$CODEX_STATE/authed"
  printf 'true\n' > "$CODEX_STATE/multiagent"
  printf '[{"id":"assay@assay","enabled":true}]\n' > "$CODEX_STATE/skills"
  local out rc=0
  out="$("$CELLCTL" check "$cell" 2>&1)" || rc=$?
  assert "check-pass: exits 0 on a fully-provisioned scrubbed cell" '[[ $rc -eq 0 ]]'
  assert "check-pass: reports all preconditions met" 'grep -q "all preconditions met" <<<"$out"'
  assert "check-pass: PEM row ok" 'grep -q "ok    PEM .*worker-desk-app.pem is regular, mode 0600" <<<"$out"'
  assert "check-pass: roster ASSAY_ALLOWED_REPOS row ok" 'grep -q "ok    roster ASSAY_ALLOWED_REPOS is exactly example-org/example-repo" <<<"$out"'
  assert "check-pass: harness login row ok (codex, under the cell home)" 'grep -q "ok    harness login under the cell home: codex login status" <<<"$out"'
  assert "check-pass: config home real-directory row ok" 'grep -q "ok    config home is a REAL directory" <<<"$out"'
  assert "check-pass: config home mode 0700 row ok" 'grep -q "ok    config home mode 0700" <<<"$out"'
  assert "check-pass: generic codex harness block also passes (not n/a on a codex cell)" 'grep -q "ok    codex on PATH" <<<"$out" && grep -q "ok    codex authenticated" <<<"$out"'
}

# ---------------------------------------------------------------- check-pem
case_check_pem(){
  local cell="case-check-pem"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  printf 'fake\n' > "$T/external.pem"; chmod 600 "$T/external.pem"
  ln -s "$T/external.pem" "$C/home/.config/assay/worker-desk-app.pem"
  local out rc=0
  out="$("$CELLCTL" check "$cell" 2>&1)" || rc=$?
  assert "check-pem: a symlink PEM is a MISS naming symlink" 'grep -q "MISS  PEM .*worker-desk-app.pem is a regular file (not a symlink)" <<<"$out"'
  assert "check-pem: exits 1 on a symlink PEM" '[[ $rc -eq 1 ]]'
  rm -f "$C/home/.config/assay/worker-desk-app.pem"
  printf 'fake\n' > "$C/home/.config/assay/worker-desk-app.pem"; chmod 644 "$C/home/.config/assay/worker-desk-app.pem"
  rc=0; out="$("$CELLCTL" check "$cell" 2>&1)" || rc=$?
  assert "check-pem: a regular 0644 PEM is a MISS naming 0600" 'grep -q "MISS  PEM .*worker-desk-app.pem is regular, mode 0600" <<<"$out"'
  assert "check-pem: exits 1 on a wrong-mode PEM" '[[ $rc -eq 1 ]]'
  chmod 600 "$C/home/.config/assay/worker-desk-app.pem"
  out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "check-pem: a regular 0600 PEM is ok" 'grep -q "ok    PEM .*worker-desk-app.pem is regular, mode 0600" <<<"$out"'
}

# ---------------------------------------------------------------- check-roster
case_check_roster(){
  local cell="case-check-roster"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  printf 'ASSAY_ALLOWED_REPOS=example-org/example-repo,example-org/other\n' > "$C/home/.config/assay/roster.env"
  local out
  out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "check-roster: multiple entries is a MISS" 'grep -q "MISS  roster ASSAY_ALLOWED_REPOS is exactly example-org/example-repo" <<<"$out"'
  printf 'ASSAY_ALLOWED_REPOS=\n' > "$C/home/.config/assay/roster.env"
  out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "check-roster: an empty value is a MISS" 'grep -q "MISS  roster ASSAY_ALLOWED_REPOS is exactly example-org/example-repo" <<<"$out"'
  printf 'ASSAY_ALLOWED_REPOS=example-org/example-repo\n' > "$C/home/.config/assay/roster.env"
  out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "check-roster: the exact slug is ok" 'grep -q "ok    roster ASSAY_ALLOWED_REPOS is exactly example-org/example-repo" <<<"$out"'
}

# ---------------------------------------------------------------- check-home-mode
case_check_home_mode(){
  local cell="case-check-home-mode"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  chmod 755 "$C/home/.config/assay"
  local out rc=0
  out="$("$CELLCTL" check "$cell" 2>&1)" || rc=$?
  assert "check-home-mode: 0755 config home is a MISS naming 0700" 'grep -q "MISS  config home mode 0700" <<<"$out"'
  assert "check-home-mode: exits 1" '[[ $rc -eq 1 ]]'
  chmod 700 "$C/home/.config/assay"
  out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "check-home-mode: 0700 config home is ok" 'grep -q "ok    config home mode 0700" <<<"$out"'
}

# ---------------------------------------------------------------- env-scrub
case_env_scrub(){
  local cell="case-env-scrub"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo \
    --roots "example-org/example-repo=$REPO" >/dev/null
  local C="$CELLS_ROOT/$cell"
  mkdir -p "$T/canary"; : > "$T/canary/canary-tool"; chmod +x "$T/canary/canary-tool"
  rm -f "$T/claude-launch.out"
  local expected_path="$C/shim:$DESK_TOOLS_BIN:$T/bin:/usr/bin:/bin:/usr/sbin:/sbin"
  (
    GH_TOKEN=canary-parent SSH_AUTH_SOCK=/nonexistent ANTHROPIC_API_KEY=canary \
    PATH="$T/canary:$PATH" TERM="${TERM:-xterm-256color}" LANG="${LANG:-en_US.UTF-8}" \
    "$CELLCTL" desk "$cell" worker-desk </dev/null >"$T/env-scrub-desk.out" 2>&1
  )
  for i in $(seq 1 50); do [[ -s "$T/claude-launch.out" ]] && break; sleep 0.1; done
  local envf="$T/claude-launch.out"
  assert "env-scrub: the harness stub ran (launch happened)" '[[ -s "$envf" ]]'
  assert "env-scrub: no GH_TOKEN canary reaches the harness" '! grep -q "^GH_TOKEN=" "$envf"'
  assert "env-scrub: no SSH_AUTH_SOCK canary reaches the harness" '! grep -q "^SSH_AUTH_SOCK=" "$envf"'
  assert "env-scrub: no ANTHROPIC_API_KEY canary reaches the harness" '! grep -q "^ANTHROPIC_API_KEY=" "$envf"'
  assert "env-scrub: HOME is the cell home" 'grep -qxF "HOME=$C/home" "$envf"'
  assert "env-scrub: TMPDIR is the cell tmp dir" 'grep -qxF "TMPDIR=$C/tmp" "$envf"'
  assert "env-scrub: KUBECONFIG=/dev/null" 'grep -qx "KUBECONFIG=/dev/null" "$envf"'
  assert "env-scrub: GIT_TERMINAL_PROMPT=0" 'grep -qx "GIT_TERMINAL_PROMPT=0" "$envf"'
  assert "env-scrub: CLAUDE_CONFIG_DIR under the cell home (claude arm)" 'grep -qxF "CLAUDE_CONFIG_DIR=$C/home/.claude" "$envf"'
  assert "env-scrub: no CODEX_HOME on the claude arm (harness-namespaced, mutually exclusive)" '! grep -q "^CODEX_HOME=" "$envf"'
  assert "env-scrub: DESK_ROOTS exported from CELL_ROOTS" 'grep -qxF "DESK_ROOTS=example-org/example-repo=$REPO" "$envf"'
  assert "env-scrub: composed PATH is EXACTLY the 7 named elements, in order — no canary dir" 'grep -qxF "PATH=$expected_path" "$envf"'
  "$CELLCTL" down "$cell" >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------- plan-grammar
case_plan_grammar(){
  local cell="case-plan-grammar"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo \
    --roots "example-org/example-repo=$REPO" >/dev/null
  local out
  out="$(TERM=xterm-256color LANG=en_US.UTF-8 DRY_RUN=1 "$CELLCTL" desk "$cell" the-desk 2>&1)"
  local lines; lines="$(printf '%s\n' "$out" | grep -E '^\[dry-run\]|^\[plan\]')"
  assert "plan-grammar: the existing [dry-run] line comes first" '[[ "$(head -n1 <<<"$lines")" == \[dry-run\]* ]]'
  local env_keys; env_keys="$(grep '^\[plan\] env ' <<<"$lines" | sed -E 's/^\[plan\] env //; s/=.*$//')"
  local env_keys_sorted; env_keys_sorted="$(sort <<<"$env_keys")"
  assert "plan-grammar: [plan] env lines are sorted by KEY" '[[ "$env_keys" == "$env_keys_sorted" ]]'
  local n_env; n_env="$(grep -c '^\[plan\] env ' <<<"$lines")"
  local argv_line_no; argv_line_no="$(grep -n '^\[plan\] argv' <<<"$lines" | head -n1 | cut -d: -f1)"
  # +2: the leading [dry-run] line (1) plus every [plan] env line, then argv is the next one.
  assert "plan-grammar: [plan] argv follows every [plan] env line" '[[ -n "$argv_line_no" && "$argv_line_no" -eq $((n_env + 2)) ]]'
  assert "plan-grammar: [plan] cwd present, right after argv" '[[ "$(sed -n "$((argv_line_no+1))p" <<<"$lines")" == \[plan\]\ cwd* ]]'
  assert "plan-grammar: [plan] lock present, last" '[[ "$(tail -n1 <<<"$lines")" == \[plan\]\ lock* ]]'
  # Dereferencing: the set of [plan] env KEYs must equal the script's own SCRUBBED_ENV_KEYS, minus
  # the INACTIVE harness's namespaced var (the-desk here boots the default claude harness, so
  # CODEX_HOME never appears) — a KEY added to one and not the other fails right here.
  # The declared allowlist is read from the SHELL ORACLE's source, at its fixed path, not from
  # "$CELLCTL": the suite runs against either implementation (desk-containers/10) and a compiled
  # binary has no source to grep. The oracle stays in the tree as the reference until the cutover,
  # so this keeps the dereference honest for BOTH — the Go port has to emit the same set the
  # declared allowlist names, which is exactly the claim worth proving.
  local want; want="$(grep -oE 'SCRUBBED_ENV_KEYS="[^"]*"' "$HERE/../cellctl" | sed -E 's/^SCRUBBED_ENV_KEYS="//; s/"$//' | tr ' ' '\n' | grep -vx 'CODEX_HOME' | sort)"
  local got; got="$(sort <<<"$env_keys")"
  assert "plan-grammar: [plan] env KEY set equals SCRUBBED_ENV_KEYS minus the inactive harness var" '[[ "$got" == "$want" ]]'
}

# ---------------------------------------------------------------- smoke-ready
case_smoke_ready(){
  local cell="case-smoke-ready"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  rm -rf "$CLAUDE_STATE" "$CODEX_STATE"; mkdir -p "$CLAUDE_STATE" "$CODEX_STATE"
  printf 'READY\n' > "$CLAUDE_STATE/answer"; printf 'READY\n' > "$CODEX_STATE/answer"
  rm -f "$T/claude-smoke.out" "$T/codex-smoke.out"
  local out rc=0
  out="$("$CELLCTL" smoke "$cell" --harness claude 2>&1)" || rc=$?
  assert "smoke-ready(claude): exits 0" '[[ $rc -eq 0 ]]'
  assert "smoke-ready(claude): prints READY" '[[ "$(tail -n1 <<<"$out")" == "READY" ]]'
  assert "smoke-ready(claude): invoked with -p (claude arm)" 'grep -q -- "-p" "$T/claude-smoke.out"'
  assert "smoke-ready(claude): under the cell home" "grep -qxF \"HOME=\$C/home\" \"\$T/claude-smoke.out\""
  rc=0
  out="$("$CELLCTL" smoke "$cell" --harness codex 2>&1)" || rc=$?
  assert "smoke-ready(codex): exits 0" '[[ $rc -eq 0 ]]'
  assert "smoke-ready(codex): prints READY" '[[ "$(tail -n1 <<<"$out")" == "READY" ]]'
  assert "smoke-ready(codex): invoked with exec --ephemeral --sandbox read-only (codex arm)" 'grep -q -- "--ephemeral --sandbox read-only" "$T/codex-smoke.out"'
  assert "smoke-ready(codex): under the cell home" "grep -qxF \"HOME=\$C/home\" \"\$T/codex-smoke.out\""
}

# ---------------------------------------------------------------- smoke-not-ready
case_smoke_not_ready(){
  local cell="case-smoke-not-ready"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  rm -rf "$CLAUDE_STATE"; mkdir -p "$CLAUDE_STATE"
  printf 'READY\n' > "$CLAUDE_STATE/answer"; printf '2\n' > "$CLAUDE_STATE/exit-code"
  local out rc=0
  out="$("$CELLCTL" smoke "$cell" --harness claude 2>&1)" || rc=$?
  assert "smoke-not-ready: READY-but-nonzero-exit → exit 1" '[[ $rc -eq 1 ]]'
  assert "smoke-not-ready: READY-but-nonzero-exit → 'smoke: not ready:' line" 'grep -q "smoke: not ready:" <<<"$out"'
  rm -rf "$CLAUDE_STATE"; mkdir -p "$CLAUDE_STATE"
  printf 'I cannot\n' > "$CLAUDE_STATE/answer"; printf '0\n' > "$CLAUDE_STATE/exit-code"
  rc=0
  out="$("$CELLCTL" smoke "$cell" --harness claude 2>&1)" || rc=$?
  assert "smoke-not-ready: a wrong-but-well-formed answer → exit 1" '[[ $rc -eq 1 ]]'
  assert "smoke-not-ready: names what it said instead" 'grep -q "smoke: not ready: I cannot" <<<"$out"'
}

# ---------------------------------------------------------------- lock
case_lock(){
  local cell="case-lock"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  "$CELLCTL" desk "$cell" worker-desk </dev/null >/dev/null 2>&1
  for i in $(seq 1 50); do [[ -f "$C/run/lock.d/pid" ]] && break; sleep 0.1; done
  local out rc=0
  out="$("$CELLCTL" desk "$cell" worker-desk </dev/null 2>&1)" || rc=$?
  assert "lock: a second desk while the pid is alive exits 4" '[[ $rc -eq 4 ]]'
  assert "lock: names the exact refusal" 'grep -q "is already running (pid" <<<"$out"'
  assert "lock: names cellctl down as the release" 'grep -q "cellctl down $cell to release" <<<"$out"'
  "$CELLCTL" down "$cell" >/dev/null 2>&1
  local dead; dead="$(fresh_deadpid)"
  mkdir -p "$C/run/lock.d"; printf '%s' "$dead" > "$C/run/lock.d/pid"
  # stdout-only, exactly as case_status: `stale-lock <pid>` is the status stdout contract, so this
  # capture must not merge the Go port's stderr P3 echo (see the note above case_status).
  out="$("$CELLCTL" status "$cell")"
  assert "lock: status reports stale-lock on a dead pid" '[[ "$out" == "stale-lock $dead" ]]'
  "$CELLCTL" down "$cell" >/dev/null 2>&1
  assert "lock: down clears the stale lock" '[[ ! -d "$C/run/lock.d" ]]'
  rc=0
  "$CELLCTL" desk "$cell" worker-desk </dev/null >/dev/null 2>&1 || rc=$?
  assert "lock: after down, desk proceeds" '[[ $rc -eq 0 ]]'
  "$CELLCTL" down "$cell" >/dev/null 2>&1
}

# ---------------------------------------------------------------- status
# `status` has a MACHINE-READABLE stdout contract — exactly one token per state (`stopped`,
# `running <session>`, `stale-lock <pid>`) — so these captures assert stdout alone and do NOT
# merge stderr with `2>&1`. The Go port writes deskkit's P3 effective-config echo to stderr once
# per run, like every roster-reading desk main; the bash oracle writes none. Merging stderr into
# an EXACT-MATCH capture was the bug: it is not part of the status contract, and dropping the
# `2>&1` is what asserts the contract, not a filter over a polluted stream. The grep-based
# captures elsewhere in this file keep their `2>&1` on purpose — they assert on refusal text that
# cellctl prints to stderr, and a substring match tolerates the echo lines.
case_status(){
  local cell="case-status"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local out
  out="$("$CELLCTL" status "$cell")"
  assert "status: a fresh cell is stopped" '[[ "$out" == "stopped" ]]'
  "$CELLCTL" desk "$cell" worker-desk </dev/null >/dev/null 2>&1
  out="$("$CELLCTL" status "$cell")"
  assert "status: a live cell reports running <session>" '[[ "$out" == "running ${cell}-cell" ]]'
  "$CELLCTL" down "$cell" >/dev/null 2>&1
  out="$("$CELLCTL" status "$cell")"
  assert "status: after down, stopped again" '[[ "$out" == "stopped" ]]'
}

# ---------------------------------------------------------------- down
case_down(){
  local cell="case-down"
  "$CELLCTL" new "$cell" --kind scrubbed --repo "$REPO" --repo-slug example-org/example-repo >/dev/null
  local C="$CELLS_ROOT/$cell"
  "$CELLCTL" desk "$cell" worker-desk </dev/null >/dev/null 2>&1
  local wt="$C/worktrees/worker-desk"
  assert "down: the role worktree exists before down" '[[ -e "$wt/.git" ]]'
  "$CELLCTL" down "$cell" >/dev/null 2>&1
  assert "down: the private-socket tmux session is gone" '! tmux -S "$C/run/tmux.sock" has-session -t "${cell}-cell" 2>/dev/null'
  assert "down: the lock dir is cleared" '[[ ! -d "$C/run/lock.d" ]]'
  assert "down: the worktree under <cell>/worktrees/ is KEPT" '[[ -e "$wt/.git" ]]'
}

# ---------------------------------------------------------------- legacy-kinds
case_legacy_kinds(){
  local cell="case-legacy-local"
  mkdir -p "$CELLS_ROOT/$cell/home/.config/assay"
  printf 'CELL=%s\nCELL_KIND=local\nCELL_REPO=%s\n' "$cell" "$REPO" > "$CELLS_ROOT/$cell/cell.env"
  local rc=0
  "$CELLCTL" check "$cell" >/dev/null 2>&1 || rc=$?
  assert "legacy-kinds: a retired CELL_KIND=local registration still refuses (exit 3)" '[[ $rc -eq 3 ]]'
  local out; out="$("$CELLCTL" check "$cell" 2>&1)" || true
  assert "legacy-kinds: names every known kind, scrubbed included" 'grep -q "k8s|house|container|scrubbed" <<<"$out"'
  local kcell="case-legacy-k8s"
  mkdir -p "$CELLS_ROOT/$kcell/home/.config/assay"
  printf 'CELL=%s\nCELL_REPO=%s\n' "$kcell" "$REPO" > "$CELLS_ROOT/$kcell/cell.env"
  : > "$CELLS_ROOT/$kcell/home/.config/assay/roster.env"
  out="$(DRY_RUN=1 "$CELLCTL" desk "$kcell" the-desk 2>&1)"
  assert "legacy-kinds: cell.env without CELL_KIND still loads as k8s (untouched neighbour)" 'grep -q "kind=k8s" <<<"$out"'
}

# ---------------------------------------------------------------- driver
run_case(){
  case "$1" in
    new) case_new ;;
    check-pass) case_check_pass ;;
    check-pem) case_check_pem ;;
    check-roster) case_check_roster ;;
    check-home-mode) case_check_home_mode ;;
    env-scrub) case_env_scrub ;;
    plan-grammar) case_plan_grammar ;;
    smoke-ready) case_smoke_ready ;;
    smoke-not-ready) case_smoke_not_ready ;;
    lock) case_lock ;;
    status) case_status ;;
    down) case_down ;;
    legacy-kinds) case_legacy_kinds ;;
    *) echo "scrubbed-cell.test.sh: unknown --case '$1'" >&2; exit 2 ;;
  esac
}

if [[ "${1:-}" == "--case" ]]; then
  [[ -n "${2:-}" ]] || { echo "scrubbed-cell.test.sh: --case needs a name" >&2; exit 2; }
  echo "[case $2]"
  run_case "$2"
else
  for c in new check-pass check-pem check-roster check-home-mode env-scrub plan-grammar \
           smoke-ready smoke-not-ready lock status down legacy-kinds; do
    echo "[case $c]"
    run_case "$c"
  done
fi

echo
if [[ "$fails" -eq 0 ]]; then echo "scrubbed-cell.test.sh: OK"; else echo "scrubbed-cell.test.sh: $fails FAILED"; exit 1; fi
