#!/usr/bin/env bash
# herdr-orca-launch.test.sh — the herdr and orca cockpit arms' REAL (non-DRY_RUN) launch/teardown
# mechanics, against stubs shaped like the REAL binaries on this machine (herdr 0.8.2, a real orca
# desktop build) rather than the "--help advertises a verb" shape cockpit.test.sh's stubs use for
# cockpit SELECTION (which never reaches these arms — `cmd_up`'s DRY_RUN branch returns before
# `up_herdr`/`up_orca` are ever called, so cockpit.test.sh's herdr/orca stubs exercise none of
# this file's code).
#
# What it proves (each an `assert` below), against fixtures built from LIVE probing of the real
# binaries on this machine (recorded in the PR body) rather than guessed shapes:
#
#   herdr:
#     up   `herdr tab create --label <l>` is called once per window (FIRST_NAME + each role);
#          the returned pane_id (from `result.root_pane.pane_id` — the real JSON shape) is what
#          `herdr pane run <pane_id> <cmd>` receives as its target, with <cmd> the EXACT role
#          command string every other cockpit runs (`'<cellctl>' desk '<cell>' '<role>' ''`) —
#          NOT `agent start ... -- bash -lc <cmd>` (verified live against real herdr 0.8.2 that
#          `agent start`'s AGENT_ARG is appended to the KIND's canonical executable, not run as a
#          wrapping shell command, so it cannot host this composite command)
#          a build with no `pane run` (`herdr pane --help` advertises no `run`) refuses up-front,
#          naming --cockpit tmux, rather than silently trying agent start
#     up (assay#985) when `herdr workspace list` (herdr's own noun for what this cockpit and
#          #985's issue call a "herdr window" — verified live against herdr 0.8.2) reports no
#          workspace open at all, `herdr workspace create --label <cell>-<the first window>` is
#          called BEFORE any tab create — the same trigger point and create-if-absent shape as the
#          tmux arm's `tmux has-session || tmux new-session` — and every tab this run creates
#          targets the new workspace explicitly (`tab create --workspace <id> --label <l>`); the
#          workspace's own auto-seeded default tab is closed once the cell's real tabs exist in it.
#          When a workspace is ALREADY open, `workspace create` is never called and tab create runs
#          exactly as before (no `--workspace` flag) — unchanged behaviour, regression-checked.
#          A build that cannot list/create workspaces, or whose `workspace create` fails, refuses
#          up-front naming herdr and the exact command tried, rather than opening no window at all.
#     down `herdr tab list` is read, matched by label, and only the tab_ids found are closed via
#          `herdr tab close <tab_id>` — never `herdr tab close --label <l>` (real herdr 0.8.2's
#          `tab close` takes a positional tab_id and advertises no `--label` at all)
#
#   orca:
#     up   `orca repo add --path <cell-dir>` is called (idempotent registration — real orca 404s
#          a `--worktree path:<dir>` selector on an unregistered path) before any terminal is
#          created; `orca terminal create` receives `--worktree path:<cell-dir>` (a real orca's
#          create-a-terminal flag for "where", confirmed live — NOT `--cwd`, which real orca does
#          not advertise on this verb at all) plus `--command <role_cmd>` and `--title <cell>-<role>`
#     down `orca terminal close --worktree path:<cell-dir> --all` is called once (closes every
#          terminal orca owns for the cell in one call) rather than tracking individual handles;
#          this also proves `down_orca` even RUNS to completion — the pre-existing body (before
#          this suite, and before this commit's fix) had `local roles; roles="$(up_roles 0)" r`,
#          a malformed `local` line (a bare `VAR=value r` command, not a second local declared)
#          that made `r`'s later use in `for r in cell $roles` an unbound variable expansion and
#          the shell tried to run a command literally named `r` — `down --cockpit orca` DIED with
#          "r: command not found" every time, undetected because nothing had ever driven this arm
#          to completion before
#
# No network, no real herdr/orca, no tmux: every binary is a stub on a private PATH that RECORDS
# its invocation (one line per call, argv only, to a file this suite inspects) and answers with
# the JSON shapes captured live against the real binaries. `claude`/desk-tools are not invoked at
# all by these arms (they hand a COMMAND STRING to herdr/orca; nothing here executes it).
# shellcheck disable=SC2016,SC2034
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CELLCTL="$HERE/../cellctl"
T="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/cellctl-herdr-orca.XXXXXX")" && pwd -P)"
trap 'rm -rf "$T"' EXIT
fails=0
assert(){ if eval "$2"; then echo "  ok    $1"; else echo "  FAIL  $1"; fails=$((fails+1)); fi; }

unset CELL CELL_KIND CELL_FORGE CELL_REPO CELL_ROOTS CELL_COCKPIT CELLS_CONFIG ROLES DESKD \
      DESKD_ADDR DESKD_INDEX DESK_MODEL_DEFAULT DESK_MODEL_the_desk TMUX_SESSION CELL_ATTENDED \
      CELL_PROVIDER

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

mkdir -p "$T/bin"
printf '#!/usr/bin/env bash\nexit 1\n' > "$T/bin/tmux"; chmod +x "$T/bin/tmux"
export PATH="$T/bin:/usr/bin:/bin:/usr/sbin:/sbin"
command -v herdr >/dev/null 2>&1 && { echo "herdr-orca-launch.test.sh: a real herdr is on PATH — cannot run hermetically"; exit 2; }
command -v orca  >/dev/null 2>&1 && { echo "herdr-orca-launch.test.sh: a real orca is on PATH — cannot run hermetically"; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "herdr-orca-launch.test.sh: python3 required (cellctl's own JSON parsing dependency)"; exit 2; }

export CELLS_ROOT="$T/cells" CLAUDE_CONFIG_DIR="$T/claude-config"; mkdir -p "$CLAUDE_CONFIG_DIR"
"$CELLCTL" new example-cell --kind house --repo "$REPO" --roots "$ROOTS" >/dev/null
CELL="$CELLS_ROOT/example-cell"
sed -i.bak 's/^ROLES=.*/ROLES="worker-desk"/' "$CELL/cell.env"; rm -f "$CELL/cell.env.bak"

HERDR_CALLS="$T/herdr-calls.log"
ORCA_CALLS="$T/orca-calls.log"

# herdr stub: shaped like a real herdr 0.8.2 for the calls these arms make, extended (assay#985)
# for the workspace verbs — herdr's own noun for what this cockpit calls a "window".
#   tab create [--workspace <id>] --label <l> → JSON with result.root_pane.pane_id (real shape,
#                                                 captured live)
#   pane run <pane_id> <cmd>                  → records and succeeds
#   pane --help / tab --help / workspace --help → advertise run/create/close/list (real herdr does)
#   tab list                                  → JSON with the labelled tabs open so far
#   tab close <tab_id>                        → records and succeeds
#   workspace list                            → JSON with the workspaces open so far
#   workspace create --label <l>              → opens one, seeded with its own default (unlabelled,
#                                                 label "1") tab — exactly like real herdr 0.8.2
#
# herdr_stub [seeded]: with no argument, no workspace is open yet (the "no herdr window" fixture);
# "seeded" pre-opens one workspace before cellctl ever runs (the "herdr already has a window"
# fixture, for the assay#985 regression check that behaviour there is unchanged).
herdr_stub(){
local seeded="${1:-}"
rm -f "$T"/tab-seq "$T"/tab-label-* "$T"/ws-seq "$T"/ws-label-*
# Pre-create the sequence-counter files (empty = count 0): `wc -l < FILE` on a FILE that does not
# exist yet fails the redirection itself, and that failure is reported by the shell before the
# command's own `2>/dev/null` takes effect (verified: it leaks onto fd2 regardless) — which would
# otherwise corrupt the very first workspace/tab create call's captured JSON in the "no window
# open" fixture below, where that first call has to succeed for the test to mean anything.
: > "$T/tab-seq"; : > "$T/ws-seq"
if [[ "$seeded" == "seeded" ]]; then
  echo 1 >> "$T/ws-seq"
  echo "pre-existing" > "$T/ws-label-1"
fi
cat > "$T/bin/herdr" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$HERDR_CALLS"
case "\$1 \$2" in
  "tab --help") echo "Usage: herdr tab <cmd>"; echo "  create  Create a tab"; echo "  close   Close a tab"; echo "  list    List tabs"; exit 0 ;;
  "pane --help") echo "Usage: herdr pane <cmd>"; echo "  run  Run a command in a pane"; exit 0 ;;
  "workspace --help") echo "Usage: herdr workspace <cmd>"; echo "  list    List workspaces"; echo "  create  Create a workspace"; exit 0 ;;
esac
case "\$1" in
  workspace)
    case "\$2" in
      list)
        printf '{"result":{"workspaces":['
        first=1
        for f in "$T"/ws-label-*; do
          [[ -e "\$f" ]] || continue
          idx="\${f##*-}"
          [[ "\$first" == "1" ]] || printf ','
          printf '{"workspace_id":"w%s"}' "\$idx"
          first=0
        done
        printf ']}}\n'
        exit 0 ;;
      create)
        local_label=""
        shift 2
        while [[ \$# -gt 0 ]]; do case "\$1" in --label) local_label="\$2"; shift 2;; *) shift;; esac; done
        wn=\$(( \$(wc -l < "$T/ws-seq" 2>/dev/null || echo 0) + 1 ))
        echo "\$wn" >> "$T/ws-seq"
        echo "\$local_label" > "$T/ws-label-\$wn"
        tn=\$(( \$(wc -l < "$T/tab-seq" 2>/dev/null || echo 0) + 1 ))
        echo "\$tn" >> "$T/tab-seq"
        echo "1" > "$T/tab-label-\$tn"
        printf '{"result":{"workspace":{"workspace_id":"w%s"},"tab":{"tab_id":"w1:t%s"}}}\n' "\$wn" "\$tn"
        exit 0 ;;
    esac ;;
  tab)
    case "\$2" in
      create)
        local_label=""
        shift 2
        while [[ \$# -gt 0 ]]; do case "\$1" in --label) local_label="\$2"; shift 2;; --workspace) shift 2;; *) shift;; esac; done
        n=\$(( \$(wc -l < "$T/tab-seq" 2>/dev/null || echo 0) + 1 ))
        echo "\$n" >> "$T/tab-seq"
        echo "\$local_label" > "$T/tab-label-\$n"
        printf '{"result":{"root_pane":{"pane_id":"w1:p%s"}}}\n' "\$n"
        exit 0 ;;
      list)
        printf '{"result":{"tabs":['
        first=1
        for f in "$T"/tab-label-*; do
          [[ -e "\$f" ]] || continue
          idx="\${f##*-}"
          lbl="\$(cat "\$f")"
          [[ "\$first" == "1" ]] || printf ','
          printf '{"tab_id":"w1:t%s","label":"%s"}' "\$idx" "\$lbl"
          first=0
        done
        printf ']}}\n'
        exit 0 ;;
      close) exit 0 ;;
    esac ;;
  pane)
    case "\$2" in run) exit 0 ;; esac ;;
esac
exit 0
EOF
chmod +x "$T/bin/herdr"
}

# orca stub: shaped like a real, RUNNING orca desktop app for the calls these arms make.
#   repo list                                  → exit 0 (app reachable — the auto/orca_reachable probe)
#   repo add --path <dir>                      → records, ok
#   terminal --help                            → advertises `create`
#   terminal create --help                     → advertises --worktree / --command / --title (real shape)
#   terminal create --worktree ... --command ... --title ...  → records, ok
#   terminal close --help                      → advertises --worktree / --all (real shape)
#   terminal close --worktree ... --all        → records, ok
orca_stub(){ cat > "$T/bin/orca" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$ORCA_CALLS"
case "\$1 \$2" in
  "repo list") exit 0 ;;
  "repo add") exit 0 ;;
  "repo --help") echo "Usage: orca repo <cmd>"; echo "  list  List repos registered in Orca"; echo "  add   Add a project to Orca by filesystem path"; exit 0 ;;
  "terminal --help") echo "Usage: orca terminal <cmd>"; echo "  create  Create a terminal session in the current worktree"; echo "  close   Close one terminal"; exit 0 ;;
  "terminal create") : ;;
  "terminal close") : ;;
esac
if [[ "\$1 \$2" == "terminal create" && "\$3" == "--help" ]]; then
  echo "Usage: orca terminal create [--worktree <selector>] [--title <name>] [--command <text>] [--focus] [--json]"
  echo "  --worktree <selector>  Worktree selector such as identity:<identity>, id:<repo-id>::<path>, name:<displayName>, branch:<branch>, issue:<number>, path:<path>, or active/current"
  echo "  --command <text>       Command to run in the terminal on startup"
  echo "  --title <text>         Custom title for the terminal tab"
  exit 0
fi
if [[ "\$1 \$2" == "terminal close" && "\$3" == "--help" ]]; then
  echo "Usage: orca terminal close ([--terminal <handle>] [--tab] | --worktree <selector> --all) [--json]"
  echo "  --terminal <handle>  Runtime-issued terminal handle"
  echo "  --worktree <selector>  Worktree selector such as identity:<identity>, id:<repo-id>::<path>, name:<displayName>, branch:<branch>, issue:<number>, path:<path>, or active/current"
  echo "  --all"
  exit 0
fi
exit 0
EOF
chmod +x "$T/bin/orca"; }

# ---------------------------------------------------------------- herdr
echo "[herdr up: no window open (assay#985)]"
herdr_stub
# --no-the-desk keeps this to exactly two windows (the FIRST/"cell" window plus one role), so the
# call counts and pane-id sequence below are unambiguous. No workspace is seeded, so
# `herdr workspace list` reports none open — up_herdr must bring one up before any tab lands.
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr --no-the-desk 2>&1)" && rc=0 || rc=$?
assert "up --cockpit herdr exits 0" '[[ $rc -eq 0 ]]'
assert "'workspace list' is checked before anything else (has-window probe)" 'grep -q "^workspace list" "$HERDR_CALLS"'
assert "'workspace create --label' brings up the window, labelled <cell>-cell" 'grep -q "^workspace create --label example-cell-cell" "$HERDR_CALLS"'
assert "workspace create runs BEFORE any tab create" '[[ "$(grep -nE "^workspace create|^tab create" "$HERDR_CALLS" | head -1)" == *"workspace create"* ]]'
assert "one 'tab create ... --label' per window (cell + worker-desk), targeting the new workspace" '[[ "$(grep -cE "^tab create --workspace w1 --label" "$HERDR_CALLS")" -eq 2 ]]'
assert "the FIRST window is labelled <cell>-cell, in the new workspace" 'grep -q "tab create --workspace w1 --label example-cell-cell" "$HERDR_CALLS"'
assert "the role window is labelled <cell>-worker-desk, in the new workspace" 'grep -q "tab create --workspace w1 --label example-cell-worker-desk" "$HERDR_CALLS"'
assert "the new workspace's own default tab is closed once the real tabs exist" 'grep -q "^tab close w1:t1" "$HERDR_CALLS"'
assert "the default tab is closed AFTER both role tabs are created, never before" '[[ "$(grep -nE "^tab create|^tab close" "$HERDR_CALLS" | tail -1)" == *"tab close"* ]]'
assert "'pane run' is called (not 'agent start')" 'grep -q "^pane run" "$HERDR_CALLS"'
assert "'agent start' is NEVER called (it cannot host this composite command)" '! grep -q "agent start" "$HERDR_CALLS"'
assert "pane run's command is the exact role_cmd every cockpit runs" "grep -q \"pane run w1:p3 .*desk 'example-cell' 'worker-desk'\" \"\$HERDR_CALLS\""

echo "[herdr up: window already open — unchanged (assay#985 regression check)]"
herdr_stub seeded
: > "$HERDR_CALLS"
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr --no-the-desk 2>&1)" && rc=0 || rc=$?
assert "up --cockpit herdr exits 0" '[[ $rc -eq 0 ]]'
assert "'workspace create' is NEVER called — a window is already open" '! grep -q "^workspace create" "$HERDR_CALLS"'
assert "one 'tab create --label' per window (cell + worker-desk)" '[[ "$(grep -c "^tab create" "$HERDR_CALLS")" -eq 2 ]]'
assert "the FIRST window is labelled <cell>-cell" 'grep -q "^tab create --label example-cell-cell" "$HERDR_CALLS"'
assert "the role window is labelled <cell>-worker-desk" 'grep -q "^tab create --label example-cell-worker-desk" "$HERDR_CALLS"'
assert "no tab create carries --workspace — identical invocation to before assay#985" '! grep -q -- "--workspace" "$HERDR_CALLS"'
assert "'pane run' is called (not 'agent start')" 'grep -q "^pane run" "$HERDR_CALLS"'
assert "'agent start' is NEVER called (it cannot host this composite command)" '! grep -q "agent start" "$HERDR_CALLS"'
assert "pane run's command is the exact role_cmd every cockpit runs" "grep -q \"pane run w1:p2 .*desk 'example-cell' 'worker-desk'\" \"\$HERDR_CALLS\""

echo "[herdr down]"
: > "$HERDR_CALLS"
out="$(DRY_RUN=0 "$CELLCTL" down example-cell --cockpit herdr 2>&1)" && rc=0 || rc=$?
assert "down --cockpit herdr exits 0" '[[ $rc -eq 0 ]]'
assert "'tab list' is read before closing" 'grep -q "^tab list" "$HERDR_CALLS"'
assert "'tab close' is called with a tab_id, never --label" 'grep -qE "^tab close w1:t[0-9]+$" "$HERDR_CALLS"'
assert "no close call carries --label (real herdr 0.8.2 tab close has no such flag)" '! grep -q "tab close --label" "$HERDR_CALLS"'
assert "down reports how many of the cell's tabs it closed" 'grep -qE "closed [0-9]+/[0-9]+ tabs" <<<"$out"'

echo "[herdr: no 'pane run' on this build]"
cat > "$T/bin/herdr" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "tab --help") echo "Usage: herdr tab <cmd>"; echo "  create  Create a tab"; exit 0 ;;
  "pane --help") echo "Usage: herdr pane <cmd>"; exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/herdr"
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr 2>&1)" && rc=0 || rc=$?
assert "a herdr build with no 'pane run' refuses up-front" '[[ $rc -ne 0 ]] && grep -q "advertises no .run.\|cockpit tmux" <<<"$out"'

echo "[herdr: no 'workspace list' on this build (assay#985)]"
cat > "$T/bin/herdr" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "tab --help") echo "Usage: herdr tab <cmd>"; echo "  create  Create a tab"; exit 0 ;;
  "pane --help") echo "Usage: herdr pane <cmd>"; echo "  run  Run a command in a pane"; exit 0 ;;
  "workspace --help") echo "Usage: herdr workspace <cmd>"; exit 0 ;;
esac
exit 0
EOF
chmod +x "$T/bin/herdr"
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr 2>&1)" && rc=0 || rc=$?
assert "a herdr build with no 'workspace list' refuses up-front, naming herdr" '[[ $rc -ne 0 ]] && grep -q "herdr" <<<"$out" && grep -q "advertises no .list.\|cockpit tmux" <<<"$out"'

echo "[herdr: 'workspace create' fails when no window is open (assay#985)]"
cat > "$T/bin/herdr" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "tab --help") echo "Usage: herdr tab <cmd>"; echo "  create  Create a tab"; exit 0 ;;
  "pane --help") echo "Usage: herdr pane <cmd>"; echo "  run  Run a command in a pane"; exit 0 ;;
  "workspace --help") echo "Usage: herdr workspace <cmd>"; echo "  list    List workspaces"; echo "  create  Create a workspace"; exit 0 ;;
esac
case "$1 $2" in
  "workspace list") echo '{"result":{"workspaces":[]}}'; exit 0 ;;
  "workspace create") echo "boom: server unreachable" >&2; exit 1 ;;
esac
exit 0
EOF
chmod +x "$T/bin/herdr"
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr 2>&1)" && rc=0 || rc=$?
assert "a herdr build whose 'workspace create' fails refuses up-front, naming herdr and the command tried" \
  '[[ $rc -ne 0 ]] && grep -q "herdr" <<<"$out" && grep -q "herdr workspace create --label" <<<"$out"'

echo "[herdr: not installed (assay#985)]"
rm -f "$T/bin/herdr"
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit herdr 2>&1)" && rc=0 || rc=$?
assert "--cockpit herdr with no herdr on PATH → non-zero, naming herdr" '[[ $rc -ne 0 ]] && grep -q "herdr" <<<"$out" && grep -q "not on PATH" <<<"$out"'
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit auto --no-attach 2>&1)" && rc=0 || rc=$?
assert "--cockpit auto with no herdr on PATH falls through to tmux" 'grep -qF "[cockpit] tmux (fallback: no herdr/orca on PATH)" <<<"$out"'

# ---------------------------------------------------------------- orca
echo "[orca up]"
orca_stub
out="$(DRY_RUN=0 "$CELLCTL" up example-cell --cockpit orca 2>&1)" && rc=0 || rc=$?
assert "up --cockpit orca exits 0" '[[ $rc -eq 0 ]]'
assert "'repo add --path <cell-dir>' registers the cell before any terminal create" "grep -q \"repo add --path \$CELL\\\$\" \"\$ORCA_CALLS\""
assert "terminal create uses --worktree path:<cell-dir> (real orca's selector), not --cwd" "grep -q \"terminal create --worktree path:\$CELL --title example-cell-worker-desk --command\" \"\$ORCA_CALLS\""
assert "no call ever uses --cwd (real orca's create-a-terminal verb advertises no such flag)" '! grep -q -- "--cwd" "$ORCA_CALLS"'
assert "the role window's --command is the exact role_cmd every cockpit runs" "grep -q \"terminal create .*--command .*desk 'example-cell' 'worker-desk'\" \"\$ORCA_CALLS\""

echo "[orca down]"
: > "$ORCA_CALLS"
out="$(DRY_RUN=0 "$CELLCTL" down example-cell --cockpit orca 2>&1)" && rc=0 || rc=$?
assert "down --cockpit orca exits 0" '[[ $rc -eq 0 ]]'
assert "'terminal close --worktree path:<cell-dir> --all' closes everything in one call" "grep -q \"terminal close --worktree path:\$CELL --all\" \"\$ORCA_CALLS\""

echo
if [[ "$fails" -eq 0 ]]; then echo "herdr-orca-launch.test.sh: OK"; else echo "herdr-orca-launch.test.sh: $fails FAILED"; exit 1; fi
