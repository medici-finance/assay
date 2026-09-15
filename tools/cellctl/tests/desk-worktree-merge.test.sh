#!/usr/bin/env bash
# desk-worktree-merge.test.sh — `cellctl desk` on an EXISTING role worktree that cannot
# fast-forward (#1157).
#
# The defect: the existing-worktree arm ran `git merge --ff-only <origin/main>` and, when that
# failed, printed one NOTICE and booted the desk on the stale tree. A role worktree that ever
# carried a local commit can never fast-forward again, so every later boot was stale and silent.
#
# What it proves (each an `assert` below):
#   merge     a role worktree carrying a local commit is MERGED with origin/main at boot — a real
#             two-parent merge commit, never a rebase, never "left as is" — and the desk launches
#             on a tree that contains origin/main
#   conflict  a local commit that conflicts with origin/main on a hand-maintained file STOPS the
#             boot: non-zero exit, the conflicting path named, the desk NOT launched, and the
#             worktree left clean (merge aborted, no MERGE_HEAD, HEAD unchanged)
#   generated the single-writer generated files (STATUS.md, docs/streams/FINDINGS.md) are taken
#             from origin/main on conflict, never hand-merged, and the boot proceeds
#   mixed     a generated-file conflict does not mask a hand-file conflict in the same merge —
#             the boot still STOPS and names the hand-maintained path
#
# No network, no tmux, no real desk-tools: `claude` and the desk verbs are stubs on a private
# PATH; CELLCTL_DESKWT=0 keeps cellctl's own worktree path in charge so the tree under test is
# deterministic. Runs with plain bash.
# The assert strings are single-quoted on purpose (expanded by eval at assert time), and the
# variables they read look unused to a static pass.
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-wtmerge.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }
real(){ (cd "$1" && pwd -P); }
# commit <dir> <msg>: stage everything under <dir> and commit with a fixture identity.
commit(){ git -C "$1" add -A && git -C "$1" -c user.name=x -c user.email=x@example.invalid commit -q -m "$2"; }
# parents <dir>: the number of parents HEAD carries — 2 is a real merge, 1 is a rebase or a
# fast-forward wearing a merge's clothes.
parents(){ git -C "$1" cat-file -p HEAD | grep -c '^parent '; }

# ---------------------------------------------------------------- fixtures
export HOME="$T/home"; mkdir -p "$HOME/.config/gh"
printf '[user]\n\tname = Example Operator\n\temail = operator@example.invalid\n' > "$HOME/.gitconfig"
export GIT_CONFIG_NOSYSTEM=1
export ASSAY_CONFIG_HOME="$T/operator-config"; mkdir -p "$ASSAY_CONFIG_HOME"
printf 'ASSAY_TRUSTED_LOGINS=example-human:1\n' > "$ASSAY_CONFIG_HOME/roster.env"
# Fixture repo: a bare "origin" with a main branch carrying docs/streams/, cloned as CELL_REPO.
# `seed` is the clone every upstream change is pushed from; `checkout` is the cell's CELL_REPO.
git init -q --bare -b main "$T/origin.git"
git clone -q "$T/origin.git" "$T/seed" 2>/dev/null
mkdir -p "$T/seed/docs/streams"; echo "# streams" > "$T/seed/docs/streams/README.md"
commit "$T/seed" "seed"
git -C "$T/seed" push -q origin main
git clone -q "$T/origin.git" "$T/checkout"
REPO="$T/checkout"
ROOTS="example-org/example-repo=$REPO"
export DESK_TOOLS_BIN="$T/desk-tools"; mkdir -p "$DESK_TOOLS_BIN" "$T/bin"
for v in deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken; do
  printf '#!/usr/bin/env bash\nexit 0\n' > "$DESK_TOOLS_BIN/$v"; chmod +x "$DESK_TOOLS_BIN/$v"
done
# The claude stub records the launch (cwd) instead of running; a boot that STOPS never reaches it.
cat > "$T/bin/claude" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "plugin enable") exit 0 ;;
  "plugin list") printf '[{"id":"assay@assay","version":"0.0.0","scope":"project","enabled":true}]\n'; exit 0 ;;
esac
echo "PWD=$(pwd -P)" > "$CELLCTL_TEST_OUT"
EOF
chmod +x "$T/bin/claude"
export PATH="$T/bin:$PATH"
export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
export CELLCTL_DESKWT=0

"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"
WT="$CELL/worktrees/worker-desk"
# First boot: creates the role worktree at origin/main.
export CELLCTL_TEST_OUT="$T/launch-0.env"
"$CELLCTL" desk example-cell worker-desk >/dev/null 2>&1
assert "fixture: first boot created the role worktree" '[[ -e "$WT/.git" && -s "$CELLCTL_TEST_OUT" ]]'

# ---------------------------------------------------------------- merge (local commit, main advanced)
echo "[merge]"
echo "local" > "$WT/local-note.md"; commit "$WT" "local commit in the role worktree"
LOCAL_SHA="$(git -C "$WT" rev-parse HEAD)"
echo "up" > "$T/seed/upstream.md"; commit "$T/seed" "upstream advances"; git -C "$T/seed" push -q origin main
MAIN_SHA="$(git -C "$T/seed" rev-parse HEAD)"
export CELLCTL_TEST_OUT="$T/launch-merge.env"
rc=0; "$CELLCTL" desk example-cell worker-desk >"$T/merge.out" 2>"$T/merge.err" || rc=$?
assert "boot exits 0" '[[ $rc -eq 0 ]]'
assert "desk launched, cwd = the role worktree" 'grep -qxF "PWD=$(real "$WT")" "$CELLCTL_TEST_OUT"'
assert "HEAD is a real two-parent merge (not a rebase, not left as is)" '[[ "$(parents "$WT")" -eq 2 ]]'
assert "HEAD contains origin/main" 'git -C "$WT" merge-base --is-ancestor "$MAIN_SHA" HEAD'
assert "HEAD contains the local commit (merge, never rebase)" 'git -C "$WT" merge-base --is-ancestor "$LOCAL_SHA" HEAD'
assert "both sides present in the tree" '[[ -f "$WT/local-note.md" && -f "$WT/upstream.md" ]]'
assert "no 'left as is' notice — the stale-and-silent boot is gone" '! grep -q "left as is" "$T/merge.err"'
assert "worktree reports the merge" 'grep -q "\[worktree\].*merged origin/main" "$T/merge.out"'
assert "no merge in progress after boot" '! git -C "$WT" rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1'

# Second boot with nothing new upstream: already current, no new merge commit is manufactured.
BEFORE="$(git -C "$WT" rev-parse HEAD)"
export CELLCTL_TEST_OUT="$T/launch-current.env"
"$CELLCTL" desk example-cell worker-desk >"$T/current.out" 2>&1
assert "already-current tree: HEAD unchanged, desk launched" '[[ "$(git -C "$WT" rev-parse HEAD)" == "$BEFORE" && -s "$CELLCTL_TEST_OUT" ]]'

# ---------------------------------------------------------------- conflict (hand-maintained file)
echo "[conflict]"
echo "# streams (local)" > "$WT/docs/streams/README.md"; commit "$WT" "local edit of a hand file"
HEAD_BEFORE="$(git -C "$WT" rev-parse HEAD)"
echo "# streams (upstream)" > "$T/seed/docs/streams/README.md"; commit "$T/seed" "upstream edit of the same line"; git -C "$T/seed" push -q origin main
export CELLCTL_TEST_OUT="$T/launch-conflict.env"
rc=0; "$CELLCTL" desk example-cell worker-desk >"$T/conflict.out" 2>"$T/conflict.err" || rc=$?
assert "boot STOPS (non-zero exit)" '[[ $rc -ne 0 ]]'
assert "desk NOT launched on a conflicting tree" '[[ ! -e "$CELLCTL_TEST_OUT" ]]'
assert "the conflicting path is named" 'grep -q "docs/streams/README.md" "$T/conflict.err"'
assert "the refusal says merge, and names the worktree" 'grep -q "merge" "$T/conflict.err" && grep -qF "$WT" "$T/conflict.err"'
assert "merge aborted: no MERGE_HEAD left behind" '! git -C "$WT" rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1'
assert "merge aborted: no unmerged paths" '[[ -z "$(git -C "$WT" diff --name-only --diff-filter=U)" ]]'
assert "merge aborted: HEAD unchanged" '[[ "$(git -C "$WT" rev-parse HEAD)" == "$HEAD_BEFORE" ]]'
assert "never a rebase: no rebase in progress" '[[ ! -d "$(git -C "$WT" rev-parse --git-dir)/rebase-merge" && ! -d "$(git -C "$WT" rev-parse --git-dir)/rebase-apply" ]]'

# ---------------------------------------------------------------- generated single-writer files
echo "[generated]"
# Drop the conflicting local commit so the tree can move on, then plant an add/add conflict on
# BOTH generated files and a clean local commit alongside it.
git -C "$WT" reset -q --hard "$(git -C "$T/seed" rev-parse HEAD)"
mkdir -p "$WT/docs/streams"
echo "local board" > "$WT/STATUS.md"; echo "local findings" > "$WT/docs/streams/FINDINGS.md"
echo "kept" > "$WT/kept-local.md"
commit "$WT" "local regen of the generated files"
echo "main board" > "$T/seed/STATUS.md"; echo "main findings" > "$T/seed/docs/streams/FINDINGS.md"
commit "$T/seed" "main regenerates"; git -C "$T/seed" push -q origin main
MAIN_SHA="$(git -C "$T/seed" rev-parse HEAD)"
export CELLCTL_TEST_OUT="$T/launch-generated.env"
rc=0; "$CELLCTL" desk example-cell worker-desk >"$T/generated.out" 2>"$T/generated.err" || rc=$?
assert "boot exits 0 when the only conflicts are generated files" '[[ $rc -eq 0 ]]'
assert "desk launched" 'grep -qxF "PWD=$(real "$WT")" "$CELLCTL_TEST_OUT"'
assert "STATUS.md is main's (never hand-merged)" '[[ "$(cat "$WT/STATUS.md")" == "main board" ]]'
assert "docs/streams/FINDINGS.md is main's" '[[ "$(cat "$WT/docs/streams/FINDINGS.md")" == "main findings" ]]'
assert "no conflict markers left in the generated files" '! grep -q "^<<<<<<<" "$WT/STATUS.md" "$WT/docs/streams/FINDINGS.md"'
assert "the clean local commit survives (merge, not reset)" '[[ -f "$WT/kept-local.md" ]]'
assert "HEAD is a two-parent merge containing origin/main" '[[ "$(parents "$WT")" -eq 2 ]] && git -C "$WT" merge-base --is-ancestor "$MAIN_SHA" HEAD'
assert "the generated take is reported" 'grep -q "STATUS.md" "$T/generated.out"'
assert "no merge in progress after boot" '! git -C "$WT" rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1'

# ---------------------------------------------------------------- mixed (generated + hand file)
echo "[mixed]"
echo "local board 2" > "$WT/STATUS.md"; echo "# streams (local again)" > "$WT/docs/streams/README.md"
commit "$WT" "local regen plus a hand edit"
HEAD_BEFORE="$(git -C "$WT" rev-parse HEAD)"
echo "main board 2" > "$T/seed/STATUS.md"; echo "# streams (upstream again)" > "$T/seed/docs/streams/README.md"
commit "$T/seed" "main regen plus a hand edit"; git -C "$T/seed" push -q origin main
export CELLCTL_TEST_OUT="$T/launch-mixed.env"
rc=0; "$CELLCTL" desk example-cell worker-desk >"$T/mixed.out" 2>"$T/mixed.err" || rc=$?
assert "boot STOPS: the generated take does not mask a hand-file conflict" '[[ $rc -ne 0 && ! -e "$CELLCTL_TEST_OUT" ]]'
assert "the hand-maintained path is named" 'grep -q "docs/streams/README.md" "$T/mixed.err"'
assert "merge aborted cleanly (HEAD unchanged, no MERGE_HEAD)" '[[ "$(git -C "$WT" rev-parse HEAD)" == "$HEAD_BEFORE" ]] && ! git -C "$WT" rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1'

echo
if [[ "$fails" -eq 0 ]]; then echo "desk-worktree-merge.test.sh: OK"; else echo "desk-worktree-merge.test.sh: $fails FAILED"; exit 1; fi
