#!/usr/bin/env bash
# gitlab-fetch-cred.test.sh — the gitlab arm's CELL_REPO fetch transport (issue-1080's sibling
# defect to the already-enabled NOTICE fix).
#
# What it proves (each an `assert` below):
#   desk   on a gitlab cell, the boot fetch in cmd_desk runs with GIT_TERMINAL_PROMPT=0 and the
#          inline credential helper reading DESKD_GITLAB_TOKEN_FILE — never a token in the URL,
#          never persisted into the operator's git config (a stray git-credential-store cache is
#          untouched); on a github cell the fetch is the plain, unchanged form; a fetch that fails
#          outright stops the boot (exit 3, naming the recovery, releasing the fetch lock); a
#          FETCH_HEAD git cannot rewrite (read-only) never hands the boot a previous boot's main;
#          one that cannot be removed either is a refusal (exit 3, lock released)
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
CELLCTL="${CELLCTL:-$HERE/../testdata/cellctl-shell-oracle.sh}"; [[ "$CELLCTL" == /* ]] || CELLCTL="$PWD/$CELLCTL"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-gitlab-fetch.XXXXXX")" && pwd -P)"
# chmod first: the unremovable-FETCH_HEAD case below leaves a read-only directory if it aborts.
trap 'chmod -R u+w "$T" 2>/dev/null; rm -rf "$T"' EXIT
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
printf 'ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
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
write_roster(){ mkdir -p "$1/home/.config/assay"; printf 'ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\n' > "$1/home/.config/assay/roster.env"; }

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

# ---------------------------------------------------------------- failed fetch: refused, with the way out
# The boot above left a FETCH_HEAD behind. A fetch that then fails outright (a credential helper
# answering with a dead token, in the field; an unreachable origin here) must stop the boot with
# exit 3, a message naming the likely cause and the recovery, and the fetch lock released. git
# itself truncates FETCH_HEAD before it contacts the remote, so this case never read a stale sha
# even before the fix; what it pins is the refusal's shape. The stale-main cases are the next two.
echo "[desk: failed fetch refuses with the recovery named, lock released]"
assert "precondition: the earlier boot left a FETCH_HEAD" '"$REALGIT" -C "$T/gh-checkout" rev-parse -q --verify FETCH_HEAD >/dev/null'
"$REALGIT" -C "$T/gh-checkout" remote set-url origin "$T/no-such-origin.git"
out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
assert "boot refuses (exit 3) when the fetch fails" '[[ $rc -eq 3 ]]'
[[ "$rc" -eq 3 ]] || echo "$out"
assert "the refusal names the stale-main risk and the credential-helper check" 'grep -q "wrote no FETCH_HEAD — refusing to boot on a stale main" <<<"$out" && grep -qF "config --show-origin --get-regexp" <<<"$out"'
assert "the copy-paste command quotes the checkout path" 'grep -qF "git -C '"'"'$T/gh-checkout'"'"' config" <<<"$out"'
assert "the refusal warns not to paste the helper output anywhere public" 'grep -q "do not paste it into a PR or issue" <<<"$out"'
assert "the refusal names the recovery and the re-run" 'grep -q "Re-mint or replace the dead credential, or remove the stale helper entry" <<<"$out" && grep -q "re-run: cellctl desk gh-cell worker-desk" <<<"$out"'
assert "the NOTICE no longer claims a ref-lock race" '! grep -q "ref-lock race" <<<"$out" && grep -q "checking whether it wrote FETCH_HEAD" <<<"$out"'
assert "the fetch lock is released on the refusal (the next boot is not a 60s wait)" '[[ ! -e "$T/gh-checkout/.git/cellctl-fetch.lock" ]]'
"$REALGIT" -C "$T/gh-checkout" remote set-url origin "$T/origin.git"

# A new commit on origin's main, returned so the boot can be checked for having it.
advance_origin(){
  echo "$1" > "$T/seed/$1.txt"
  "$REALGIT" -C "$T/seed" add -A && "$REALGIT" -C "$T/seed" -c user.name=x -c user.email=x@example.invalid commit -q -m "$1"
  "$REALGIT" -C "$T/seed" push -q origin main
  "$REALGIT" -C "$T/seed" rev-parse HEAD
}
GHWT="$GHCELL/worktrees/worker-desk"

# ---------------------------------------------------------------- read-only FETCH_HEAD: never a stale main
# The stale-main path git's own truncate does not cover: a FETCH_HEAD git cannot open for writing.
# The fetch then fails and leaves the PREVIOUS boot's sha in the file; without the pre-fetch
# removal, the boot read it and went ahead on the old main with exit 0. Removal (the directory is
# writable) lets the fetch write a fresh one, so the boot must land on origin's NEW main.
# Permission bits do not bind root, so the case would pass vacuously there — skipped instead.
echo "[desk: read-only FETCH_HEAD — the boot still lands on origin's new main]"
if [[ "$(id -u)" -eq 0 ]]; then
  echo "  skip  running as root: file modes do not stop git from writing FETCH_HEAD"
else
  out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
  assert "precondition: a good boot leaves a FETCH_HEAD" '[[ $rc -eq 0 ]] && "$REALGIT" -C "$T/gh-checkout" rev-parse -q --verify FETCH_HEAD >/dev/null'
  new_main="$(advance_origin readonly-fetch-head)"
  chmod 444 "$T/gh-checkout/.git/FETCH_HEAD"
  out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
  assert "boot succeeds (exit 0)" '[[ $rc -eq 0 ]]'
  [[ "$rc" -eq 0 ]] || echo "$out"
  assert "the booted worktree contains origin's NEW main (not the previous boot's)" '"$REALGIT" -C "$GHWT" merge-base --is-ancestor "$new_main" HEAD'
  chmod 644 "$T/gh-checkout/.git/FETCH_HEAD" 2>/dev/null || true
fi

# ---------------------------------------------------------------- unremovable FETCH_HEAD: refused
# If the removal itself fails (and git cannot rewrite the file either), reading FETCH_HEAD would
# be the previous boot's main — so the boot is refused, exit 3, lock released. Made portable
# without root or file flags: CELL_REPO is pointed at a LINKED worktree, whose FETCH_HEAD lives in
# its own per-worktree git dir, and that directory is made read-only. The fetch lock lives in the
# COMMON git dir, so it is still taken and must still be released.
echo "[desk: unremovable FETCH_HEAD — refused, never a stale main]"
if [[ "$(id -u)" -eq 0 ]]; then
  echo "  skip  running as root: directory modes do not stop the removal"
else
  "$REALGIT" -C "$T/gh-checkout" worktree add -q --detach "$T/gh-linked" main 2>/dev/null
  sed -i.bak "s#^CELL_REPO=.*#CELL_REPO=$T/gh-linked#" "$GHCELL/cell.env"; rm -f "$GHCELL/cell.env.bak"
  out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
  linked_fh="$("$REALGIT" -C "$T/gh-linked" rev-parse --path-format=absolute --git-path FETCH_HEAD)"
  assert "precondition: a good boot from the linked worktree leaves its own FETCH_HEAD" '[[ $rc -eq 0 && -s "$linked_fh" && "$linked_fh" != "$T/gh-checkout/.git/FETCH_HEAD" ]]'
  new_main="$(advance_origin unremovable-fetch-head)"
  chmod 444 "$linked_fh"; chmod 555 "$(dirname "$linked_fh")"
  out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
  chmod 755 "$(dirname "$linked_fh")"; chmod 644 "$linked_fh"
  assert "boot refuses (exit 3) when FETCH_HEAD cannot be removed" '[[ $rc -eq 3 ]]'
  [[ "$rc" -eq 3 ]] || echo "$out"
  assert "the refusal names the file and the stale-main risk" 'grep -qF "cannot remove $linked_fh before the fetch — refusing to boot" <<<"$out" && grep -q "re-run: cellctl desk gh-cell worker-desk" <<<"$out"'
  assert "the fetch lock is released on this refusal too" '[[ ! -e "$T/gh-checkout/.git/cellctl-fetch.lock" ]]'
  # With the file removable again, the same cell boots onto the new main — the refusal was the
  # file, not the fixture.
  out="$("$CELLCTL" desk gh-cell worker-desk 2>&1)" && rc=0 || rc=$?
  assert "once removable, the boot succeeds and lands on origin's new main" '[[ $rc -eq 0 ]] && "$REALGIT" -C "$GHWT" merge-base --is-ancestor "$new_main" HEAD'
  sed -i.bak "s#^CELL_REPO=.*#CELL_REPO=$T/gh-checkout#" "$GHCELL/cell.env"; rm -f "$GHCELL/cell.env.bak"
fi

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
