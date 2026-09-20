#!/usr/bin/env bash
# gitlab-fetch-cred.test.sh — the gitlab arm's CELL_REPO fetch transport (issue-1080's sibling
# defect to the already-enabled NOTICE fix).
#
# What it proves (each an `assert` below):
#   desk   on a gitlab cell, the boot fetch in cmd_desk runs with GIT_TERMINAL_PROMPT=0 and the
#          inline credential helper reading DESKD_GITLAB_TOKEN_FILE — never a token in the URL,
#          never persisted into the operator's git config (a stray git-credential-store cache is
#          untouched); on a github cell the fetch is the plain, unchanged form
#   check  the gitlab arm gets a new row proving `git ls-remote` succeeds with prompts disabled,
#          using the same helper — ok when the token file is readable and the remote answers,
#          MISS when the token file is missing/unreadable or the remote is unreachable, so a
#          broken token is caught at check time rather than as a boot hang
#
# A real local "origin" bare repo stands in for GitLab: it needs no auth, so this proves the
# INVOCATION SHAPE (the credential-helper flags + GIT_TERMINAL_PROMPT=0 reach `git fetch`/
# `git ls-remote`), which is what a boot-time interactive-prompt hang or a check-time false-ok
# both turn on — not a live GitLab endpoint (C3: no live infrastructure in a test either).
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private
# PATH; `git` is a thin spy that logs any `fetch`/`ls-remote` invocation (its args and
# GIT_TERMINAL_PROMPT) before delegating to the real binary, so the rest of cellctl's git use
# (worktree add, rev-parse, …) is completely unaffected. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time).
# shellcheck disable=SC2016
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The binary under test. $CELLCTL lets the SAME suite run against either implementation
# (the bash oracle, the default, or the Go port) — desk-containers/10.
CELLCTL="${CELLCTL:-$HERE/../cellctl}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-gitlab-fetch.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

# This shell may have inherited a live cell's exported vars — unset them so the fixture below is
# the only source cellctl reads (model-pin.test.sh's list).
unset CELL CELL_DIR CELL_HOME CELL_CONFIG CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELLS_CONFIG \
  DESKD DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS ROLES FORGE_API_BASE \
  GITLAB_API_BASE GITLAB_GROUP GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE \
  DESK_MODEL_DEFAULT DESK_MODEL_the_desk DESK_MODEL_worker_desk DESK_ROOTS DESK_LOOP \
  DESK_SESSION 2>/dev/null || true

# ---------------------------------------------------------------- fixtures
REALGIT="$(command -v git)"
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
"$REALGIT" init -q --bare -b main "$T/origin.git"
"$REALGIT" clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
"$REALGIT" -C "$T/seed" add -A && "$REALGIT" -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "seed"
"$REALGIT" -C "$T/seed" push -q origin main
"$REALGIT" clone -q "$T/origin.git" "$T/gl-checkout"
"$REALGIT" clone -q "$T/origin.git" "$T/gh-checkout"

export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
echo "ARGS=$*" > "${CELLCTL_TEST_OUT:-/dev/null}"
EOF
chmod +x "$T/bin/claude"
# The git spy: logs any fetch/ls-remote invocation's full argv plus GIT_TERMINAL_PROMPT, then
# delegates to the REAL git so every other cellctl git call (worktree add, rev-parse, the lock
# dir, …) behaves exactly as it does installed. `case " $* "` bounds the match to a standalone
# "fetch"/"ls-remote" token — the credential-helper argument itself contains neither word, so it
# can never false-trigger the log.
cat > "$T/bin/git" <<EOF
#!/usr/bin/env bash
case " \$* " in
  *" fetch "*|*" ls-remote "*)
    { printf 'GIT_TERMINAL_PROMPT=%s ARGS=%s\n' "\${GIT_TERMINAL_PROMPT:-<unset>}" "\$*"; } >> "\${CELLCTL_TEST_GIT_LOG:-/dev/null}"
    ;;
esac
exec "$REALGIT" "\$@"
EOF
chmod +x "$T/bin/git"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
printf 'cells: []\n' > "$T/cells.yaml"

# roster.env is a file-EXISTENCE precondition only ($CELL_CONFIG/roster.env, both in cmd_desk and
# cmd_check's common rows) — `new` never writes one for a k8s cell (unlike house, which symlinks
# the operator's), so both fixture cells need one written by hand.
write_roster(){ mkdir -p "$1/home/.config/assay"; printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$1/home/.config/assay/roster.env"; }

# ---------------------------------------------------------------- gitlab cell: boot fetch
echo "[desk: gitlab arm boot fetch]"
mkdir -p "$T/gitlab-tokens"
printf 'super-secret-pat\n' > "$T/gitlab-tokens/gitlab-deskd.token"; chmod 600 "$T/gitlab-tokens/gitlab-deskd.token"
"$CELLCTL" new gl-cell --kind k8s --forge gitlab --repo "$T/gl-checkout" --cells-yaml "$T/cells.yaml" \
  --group example-group --gitlab-token-store "$T/gitlab-tokens" >/dev/null
GLCELL="$CELLS_ROOT/gl-cell"
# DESKD defaults to 1 on a k8s cell.env unless stated — force it off so this fixture never needs
# deskd binaries; unrelated to the fetch-transport fix under test.
printf 'DESKD=0\n' >> "$GLCELL/cell.env"
write_roster "$GLCELL"

export CELLCTL_TEST_GIT_LOG="$T/git-gitlab.log"; : > "$CELLCTL_TEST_GIT_LOG"
export CELLCTL_TEST_OUT="$T/launch-gitlab.env"
out="$("$CELLCTL" desk gl-cell worker-desk 2>&1)" && rc=0 || rc=$?
assert "gitlab cell boots (exit 0)" '[[ $rc -eq 0 ]]'
[[ "$rc" -eq 0 ]] || echo "$out"
assert "boot fetch logged exactly once" '[[ "$(wc -l < "$CELLCTL_TEST_GIT_LOG" | tr -d " ")" -eq 1 ]]'
assert "boot fetch ran with GIT_TERMINAL_PROMPT=0" 'grep -q "^GIT_TERMINAL_PROMPT=0 " "$CELLCTL_TEST_GIT_LOG"'
assert "boot fetch cleared the credential helper before setting its own (no OS keychain shadow)" 'grep -q -- "-c credential.helper= -c credential.helper=" "$CELLCTL_TEST_GIT_LOG"'
assert "boot fetch's inline helper reads DESKD_GITLAB_TOKEN_FILE, never a literal token" 'grep -qF "cat \"\$DESKD_GITLAB_TOKEN_FILE\"" "$CELLCTL_TEST_GIT_LOG" && ! grep -q "super-secret-pat" "$CELLCTL_TEST_GIT_LOG"'
assert "boot fetch's helper answers username=oauth2" 'grep -qF "username=oauth2" "$CELLCTL_TEST_GIT_LOG"'
assert "still the same fetch: --no-tags origin main" 'grep -q -- "fetch --no-tags origin main" "$CELLCTL_TEST_GIT_LOG"'
assert "the operator git config carries no persisted credential.helper (only ever passed with -c)" '! "$REALGIT" config --global --get-all credential.helper >/dev/null 2>&1'
assert "worktree boot itself still succeeded (fetch is not the only thing that has to work)" '[[ -e "$GLCELL/worktrees/worker-desk/.git" ]]'

# ---------------------------------------------------------------- github cell: unchanged
echo "[desk: github arm unchanged]"
printf 'placeholder, not a real key\n' > "$T/app.pem"
"$CELLCTL" new gh-cell --kind k8s --forge github --repo "$T/gh-checkout" --cells-yaml "$T/cells.yaml" \
  --orgs example-org --deskd-app-pem "$T/app.pem" >/dev/null
GHCELL="$CELLS_ROOT/gh-cell"
printf 'DESKD=0\n' >> "$GHCELL/cell.env"
write_roster "$GHCELL"

export CELLCTL_TEST_GIT_LOG="$T/git-github.log"; : > "$CELLCTL_TEST_GIT_LOG"
export CELLCTL_TEST_OUT="$T/launch-github.env"
out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
assert "github cell boots (exit 0)" '[[ $rc -eq 0 ]]'
[[ "$rc" -eq 0 ]] || echo "$out"
assert "boot fetch logged exactly once" '[[ "$(wc -l < "$CELLCTL_TEST_GIT_LOG" | tr -d " ")" -eq 1 ]]'
assert "github boot fetch carries no credential.helper flag (gh's own helper already answers)" '! grep -q "credential.helper" "$CELLCTL_TEST_GIT_LOG"'
assert "github boot fetch carries no GIT_TERMINAL_PROMPT override" 'grep -q "^GIT_TERMINAL_PROMPT=<unset> " "$CELLCTL_TEST_GIT_LOG"'
assert "still the plain fetch: -C <repo> fetch --no-tags origin main" 'grep -q -- "fetch --no-tags origin main" "$CELLCTL_TEST_GIT_LOG"'

# ---------------------------------------------------------------- check: the new gitlab-arm row
echo "[check: gitlab fetch-transport row]"
out="$("$CELLCTL" check gl-cell 2>&1)" && rc=0 || rc=$?
assert "check passes when the token is readable and the remote answers" '[[ $rc -eq 0 ]]'
assert "check's new row is ok" 'grep -q "ok    GitLab fetch transport reachable" <<<"$out"'

echo "[check: gitlab fetch-transport row — token unreadable]"
mv "$T/gitlab-tokens/gitlab-deskd.token" "$T/gitlab-deskd.token.bak"
out="$("$CELLCTL" check gl-cell 2>&1)" && rc=0 || rc=$?
assert "check fails (exit 1) when the token file is gone" '[[ $rc -eq 1 ]]'
# The row is deliberately NOT named "deskd …": the token is also the boot-fetch credential, and
# the old name invited gating it on DESKD (which would break every DESKD=0 gitlab cell's fetch).
assert "the existing readable-token row is the MISS named" 'grep -q "MISS  GitLab cell token" <<<"$out"'
mv "$T/gitlab-deskd.token.bak" "$T/gitlab-tokens/gitlab-deskd.token"

echo "[check: gitlab fetch-transport row — remote unreachable]"
sed -i.bak "s#^CELL_REPO=.*#CELL_REPO=$T/does-not-exist#" "$GLCELL/cell.env"; rm -f "$GLCELL/cell.env.bak"
out="$("$CELLCTL" check gl-cell 2>&1)" && rc=0 || rc=$?
assert "check fails (exit 1) when CELL_REPO's fetch transport cannot be reached" '[[ $rc -eq 1 ]]'
assert "the new row itself names the MISS (not just the earlier checkout-exists row)" 'grep -q "MISS  GitLab fetch transport reachable" <<<"$out"'
sed -i.bak "s#^CELL_REPO=.*#CELL_REPO=$T/gl-checkout#" "$GLCELL/cell.env"; rm -f "$GLCELL/cell.env.bak"

echo
if [[ "$fails" -eq 0 ]]; then echo "gitlab-fetch-cred.test.sh: OK"; else echo "gitlab-fetch-cred.test.sh: $fails FAILED"; exit 1; fi
