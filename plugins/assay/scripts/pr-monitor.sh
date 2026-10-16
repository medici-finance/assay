#!/usr/bin/env bash
# pr-monitor.sh — the durable, stateful "did any open PR change?" poller that a
# review-desk window arms behind the harness `Monitor` tool. It is the PR-side
# twin of inbound-monitor.sh: same repo resolution, same per-repo state files,
# same seed-silently-on-first-sight rule, same truncation guard, same
# MONITOR-ARMED / MONITOR-DEGRADED vocabulary — carrying the three properties a
# hand-rolled `gh pr list` loop keeps getting wrong, plus one the issue poll does
# not need: PACING.
#
#   A. EXPLICIT IDENTITY. A background monitor inherits the environment of the
#      shell that launched it, and desk work routinely exports a role App token
#      into GH_TOKEN. An installation token minted for one owner cannot see
#      another owner's repos — it 404s every read, byte-indistinguishable from
#      "this repo has no open PRs" once stderr is discarded. And the ambient `gh`
#      login is no answer either: it is some OTHER identity than the desk's role,
#      and a monitor that silently reads as it makes the watch's trust model
#      depend on whoever happens to be logged in. So this one UNSETS
#      GH_TOKEN/GITHUB_TOKEN at the top (an inherited token is never a read
#      identity) and, for EACH repo, mints that repo's installation token for the
#      session's role with `desktoken <role> --repo <owner>/<repo>` INSIDE the
#      poll (the role comes from $DESK_LOOP). The token file's content is handed
#      to that single `gh pr list` as `GH_TOKEN=... gh ...` and is never exported,
#      never printed, never reused for another repo. A mint that fails — no role
#      for $DESK_LOOP, no `desktoken` on PATH, a refused or unverifiable mint, a
#      missing/empty token file — is MONITOR-DEGRADED (could-not-check) for THAT
#      repo, with no `gh` call at all: it never reads as an empty result and
#      NEVER falls back to the ambient `gh` login. It never writes to the forge:
#      it only ever shells `gh pr list`.
#
#   B. PER-SOURCE STATE WITH RETENTION. State is one file per repo. A repo whose
#      read FAILS RETAINS its previous lines and emits a MONITOR-DEGRADED line
#      naming itself — it never silently goes empty. A read that comes back AT the
#      --limit is TRUNCATED (`gh` keeps only the newest LIMIT and gives no
#      truncation signal), so it is a moving window, not ground truth: retain +
#      go loud, never diffed. Because the untrusted read's baseline is RETAINED,
#      the next good cycle diffs against the real baseline and the outage is
#      absorbed — zero phantom PR-EVENT lines.
#
#      (Unlike the issue monitor there is NO zero-where-it-had-some guard: an open
#      PR queue draining to zero is a normal steady state for a review desk, and a
#      PR leaving the set is reported as a `closed` PR-EVENT, not a degradation.)
#
#   C. PACING — the property the issue poll does not need but the PR poll does.
#      One review loop wrote a 16-repo tight-loop poller that tripped the forge's
#      secondary rate limit, which then blocked the same session's flip tool from
#      reading its model floor. So:
#        · ASSAY_MONITOR_PACE_SECONDS (default 2) is slept BETWEEN consecutive
#          repo reads — never after the last read, and never around a repo that
#          made no call;
#        · ASSAY_MONITOR_MAX_REPOS_PER_CYCLE (default 0 = all) caps how many repos
#          a single cycle reads and carries a cursor to the next run in the state
#          dir, so a large set is swept across cycles rather than in one burst;
#        · a `gh` exit carrying the secondary-limit signature (a 403 whose stderr
#          says `secondary rate limit`, or a 429) marks EVERY remaining repo
#          `MONITOR-DEGRADED: <slug> rate-limited, skipped` and ENDS the cycle
#          without another `gh` call — one tripped limit is never compounded by
#          the remaining reads.
#      The pace and cap in force are echoed on the MONITOR-ARMED line so a
#      transcript records what was set.
#
# bash 3.2 safe (stock /bin/bash on macOS) — no mapfile/readarray, no associative
# arrays, no ${v^^}. pr-monitor.test.sh pins every property above against a
# stubbed `gh`; no test calls the real `gh`.
#
# ---------------------------------------------------------------------------
# A. EXPLICIT IDENTITY — the very first thing, before any `gh` read. This only
# scrubs what the launching shell carried; the identity a read DOES run under is
# the per-repo minted role token (mint_for_repo, below), never this environment.
unset GH_TOKEN GITHUB_TOKEN

set -uo pipefail

# Repo resolution order (same contract as inbound-monitor.sh / assay-inbox.sh):
#   1. repo args on the command line (owner/name ...)
#   2. ./.assay/repos.txt (one owner/name per line; blank/#-comment lines ignored)
#   3. the current repo's `origin` remote, parsed to owner/name
STATE_DIR="${PR_MONITOR_STATE_DIR:-${TMPDIR:-/tmp}/assay-pr-monitor}"
LIMIT="${PR_MONITOR_LIMIT:-100}"
# Pacing knobs (shared vocabulary with inbound-monitor.sh).
PACE="${ASSAY_MONITOR_PACE_SECONDS:-2}"
MAX_REPOS="${ASSAY_MONITOR_MAX_REPOS_PER_CYCLE:-0}"

usage() {
  cat <<'EOF'
Usage: pr-monitor.sh [owner/repo ...]

Stateful open-PR monitor across the given repos (or ./.assay/repos.txt, or the
current repo's origin remote). Meant to be re-run on a cadence behind the harness
`Monitor` tool. On the first run it SEEDS each repo silently (prints
`MONITOR-ARMED: <n> repos (pace <p>s, cap <c>)`); on every run after that it
prints one line per change:

  PR-EVENT: <slug>#<num> <kind> <old> -> <new>
    kind ∈ opened | pushed | draft-flip | state | merge-state | closed

It never silently goes blind:
  · it reads each repo as the session's role App, minting that repo's token with
    `desktoken <role> --repo <slug>` ($DESK_LOOP names the role) and handing it to
    that one `gh` call only; an inherited GH_TOKEN/GITHUB_TOKEN is unset and the
    ambient `gh` login is NEVER used;
  · a repo whose token cannot be minted (no role for $DESK_LOOP, no `desktoken`,
    a refused mint, an empty token file) prints
    `MONITOR-DEGRADED: <slug> token mint FAILED (...)`, makes no read, and keeps
    its previous state — could-not-check, never an empty result;
  · a repo whose read fails RETAINS its previous state and prints
    `MONITOR-DEGRADED: <slug> ...`;
  · a read that comes back AT the --limit is TRUNCATED (gh gives no truncation
    signal of its own), so it is treated as could-not-check: retain + go loud.

It never floods the forge:
  · ASSAY_MONITOR_PACE_SECONDS is slept between consecutive repo reads;
  · ASSAY_MONITOR_MAX_REPOS_PER_CYCLE caps a cycle and carries a cursor forward;
  · a secondary-rate-limit or 429 signature ends the cycle without further calls,
    marking every remaining repo `MONITOR-DEGRADED: <slug> rate-limited, skipped`.

Environment:
  PR_MONITOR_STATE_DIR             per-repo state (default $TMPDIR/assay-pr-monitor).
  PR_MONITOR_LIMIT                 per-repo fetch cap for `gh pr list` (default 100).
  ASSAY_MONITOR_PACE_SECONDS       seconds slept between repo reads (default 2).
  ASSAY_MONITOR_MAX_REPOS_PER_CYCLE  repos read per cycle, 0 = all (default 0).

Exit codes:
  0  every repo read cleanly (armed, quiet, or emitted PR-EVENT lines)
  1  precondition failure — gh/jq missing, no repos resolvable (or an invalid repo
     token), unusable state dir. When the repo set cannot be resolved, a
     `MONITOR-DEGRADED: <reason>` line is printed on STDOUT first (the line the
     watch filter matches); stderr alone is invisible to it.
  2  at least one repo went DEGRADED (token mint failed, read failed, truncated, or
     rate-limited) — state RETAINED
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

command -v gh >/dev/null 2>&1 || { echo "pr-monitor: gh CLI not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "pr-monitor: jq not found" >&2; exit 1; }

# Numeric knobs must be non-negative integers — a non-numeric value would poison
# the arithmetic guards below and could silently disable a fail-closed check.
for _kv in "LIMIT=$LIMIT" "PACE=$PACE" "MAX_REPOS=$MAX_REPOS"; do
  if [[ ! "${_kv#*=}" =~ ^[0-9]+$ ]]; then
    echo "pr-monitor: ${_kv%%=*} must be a non-negative integer, got '${_kv#*=}'" >&2
    exit 1
  fi
done

# origin_to_slug <url> -- reduce a GitHub remote URL to owner/name. Handles every
# supported github.com form: https/http/git/ssh/git+ssh with
# an optional user[:password]@ and an optional :port, the scp-style
# git@github.com:owner/name, a trailing slash and a trailing .git. A URL it does
# not recognise passes through unchanged so the owner/name validator rejects it
# loudly (never a silent skip). Pure bash regex: bash 3.2 safe.
origin_to_slug() {
  local u="$1" p
  if [[ "$u" =~ ^(https|http|git|ssh|git\+ssh)://([^/@]*@)?github\.com(:[0-9]+)?/(.*)$ ]]; then
    p="${BASH_REMATCH[4]}"
  elif [[ "$u" =~ ^([^/@:]*@)?github\.com:(.*)$ ]]; then
    p="${BASH_REMATCH[2]}"
  else
    p="$u"
  fi
  while [[ "$p" == */ ]]; do p="${p%/}"; done
  p="${p%.git}"
  printf '%s\n' "$p"
}

# Display only: remove userinfo even when a malformed URL failed parsing. Keep
# the scheme and suffix after the last @; no escaped replacement (bash 3.2).
origin_display() {
  local u="$1" prefix=""
  if [[ "$u" == *@* ]]; then
    if [[ "$u" =~ ^([A-Za-z][A-Za-z0-9+.-]*://) ]]; then prefix="${BASH_REMATCH[1]}"; fi
    u="$prefix${u##*@}"
  fi
  printf '%s' "$u"
}

resolve_repos() {
  if [[ $# -gt 0 ]]; then
    printf '%s\n' "$@"
    return
  fi
  if [[ -f "./.assay/repos.txt" ]]; then
    grep -vE '^[[:space:]]*(#|$)' "./.assay/repos.txt"
    return
  fi
  local origin_url owner_name
  origin_url=$(git remote get-url origin 2>/dev/null || true)
  [[ -z "$origin_url" ]] && return
  owner_name=$(origin_to_slug "$origin_url")
  printf '%s\n' "$owner_name"
}

# bash 3.2 has no `mapfile`; a `while read` fed by process substitution keeps the
# appends in the current shell.
REPOS=()
while IFS= read -r _line; do
  [[ -z "${_line//[[:space:]]/}" ]] && continue
  REPOS+=("$_line")
done < <(resolve_repos "$@")

if [[ -z "${REPOS[*]+x}" || ${#REPOS[@]} -eq 0 ]]; then
  echo "MONITOR-DEGRADED: no repos to query (no repo args, no ./.assay/repos.txt, and no usable git origin remote)"
  echo "pr-monitor: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)" >&2
  exit 1
fi

# Validate every repo token before it can reach a shell: GitHub's own owner/name
# alphabet, nothing else.
for _repo in "${REPOS[@]}"; do
  if [[ ! "$_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
    # Print the token with any user[:password]@ stripped: stdout is a watch
    # stream, and an origin URL can carry a credential.
    _shown="$(origin_display "$_repo")"
    echo "MONITOR-DEGRADED: invalid repo '$_shown' (expected owner/name)"
    echo "pr-monitor: invalid repo '$_shown' (expected owner/name)" >&2
    exit 1
  fi
done

mkdir -p "$STATE_DIR" 2>/dev/null || { echo "pr-monitor: cannot create state dir '$STATE_DIR'" >&2; exit 1; }
[[ -w "$STATE_DIR" ]] || { echo "pr-monitor: state dir '$STATE_DIR' is not writable" >&2; exit 1; }

# statefile <owner/name> -> path. Slashes are the only reserved char; map to "__".
statefile() {
  local slug="$1"
  printf '%s/%s.state' "$STATE_DIR" "$(printf '%s' "$slug" | sed 's#/#__#g')"
}

CURSOR_FILE="$STATE_DIR/.cursor"

TMP_CUR=$(mktemp)
TMP_ERR=$(mktemp)
TMP_EVT=$(mktemp)
TMP_MINT=$(mktemp)
trap 'rm -f "$TMP_CUR" "$TMP_ERR" "$TMP_EVT" "$TMP_MINT"' EXIT

# `grep -c` prints "0" AND exits 1 on no match, so `grep -c ... || echo 0` would
# emit TWO zeros. Count via this helper: exactly one number, "0" for an empty or
# absent file.
countlines() {
  local n
  n=$(grep -c . "$1" 2>/dev/null)
  printf '%s' "${n:-0}"
}

# ---- A. the read identity: a per-repo minted role token ---------------------
# The App role this session acts as, from its loop identity ($DESK_LOOP). The
# table is the compiled loop-to-role one (see "Forge CLI as the role" in the desk
# shell reference); a loop that carries no role — or an unset $DESK_LOOP — leaves
# ROLE empty, which degrades EVERY repo (below): the monitor never guesses an
# identity and never substitutes the ambient `gh` login for one.
ROLE=""
ROLE_WHY="DESK_LOOP is unset - no App role to mint a read token as"
if [[ -n "${DESK_LOOP:-}" ]]; then
  case "$DESK_LOOP" in
    the-desk)                  ROLE="desk" ;;
    worker-desk|batch-fanout)  ROLE="worker" ;;
    pr-review-desk)            ROLE="reviewer" ;;
    verify-desk)               ROLE="verifier" ;;
    intake-desk|issue-loop)    ROLE="issue-loop" ;;
    *) ROLE_WHY="DESK_LOOP '$(printf '%s' "$DESK_LOOP" | tr -c 'A-Za-z0-9._-' '?' | cut -c1-40)' has no App role to mint a read token as" ;;
  esac
fi

# mint_for_repo <owner/name> -- mint that repo's installation token for $ROLE.
# On success sets MINT_TOK (the token VALUE, read from the custody file `desktoken`
# named exactly once and whitespace-stripped) and returns 0. On
# ANY failure sets MINT_WHY (one short line, the token never in it) and returns 1;
# the caller then makes no `gh` call for the repo. A mint that "succeeds" with no
# usable token is a failure too: an empty GH_TOKEN makes `gh` fall back to the
# host's stored login, which is exactly the ambient identity this refuses.
#
# The file is read ONCE, here, and the caller passes this very value to `gh`: the
# cache is rewritten in place by every concurrent mint for the role, so a second
# read at the call site (after the pacing sleep) could see it truncated and hand
# `gh` an empty GH_TOKEN that nothing checked.
MINT_TOK=""
MINT_WHY=""
mint_for_repo() {
  local repo="$1" out rc tok
  MINT_TOK=""; MINT_WHY=""
  if [[ -z "$ROLE" ]]; then MINT_WHY="$ROLE_WHY"; return 1; fi
  if ! command -v desktoken >/dev/null 2>&1; then
    MINT_WHY="desktoken not found on PATH"; return 1
  fi
  # ASSAY_CONFIG_ECHO=full: the minter prints its effective-config preamble on stderr,
  # which this captures and mostly discards; "full" keeps that echo stateless so no
  # digest is recorded as seen that nobody saw. The diagnosis is the LAST stderr line.
  out=$(ASSAY_CONFIG_ECHO=full desktoken "$ROLE" --repo "$repo" 2>"$TMP_MINT"); rc=$?
  if [[ "$rc" -ne 0 ]]; then
    MINT_WHY="desktoken $ROLE exited $rc: $(grep . "$TMP_MINT" | tail -n 1 | cut -c1-200)"
    return 1
  fi
  out=$(printf '%s\n' "$out" | tail -n 1)
  if [[ -z "$out" || ! -f "$out" ]]; then
    MINT_WHY="desktoken $ROLE named no token file"; return 1
  fi
  if ! tok=$(tr -d '[:space:]' < "$out" 2>/dev/null) || [[ -z "$tok" ]]; then
    MINT_WHY="desktoken $ROLE's token file is empty or unreadable"; return 1
  fi
  MINT_TOK="$tok"
  return 0
}

# The secondary-rate-limit / 429 signature, read from the captured gh stderr.
is_ratelimit() {
  grep -qiE 'secondary rate limit|(http )?429|too many requests' "$TMP_ERR"
}

# ---- select this cycle's slice (the cap + cursor) --------------------------
nrepos=${#REPOS[@]}
start=0
if [[ "$MAX_REPOS" -gt 0 ]]; then
  # Resume from the carried cursor; a fresh, out-of-range or missing cursor
  # restarts at 0.
  if [[ -f "$CURSOR_FILE" ]]; then
    _c=$(head -1 "$CURSOR_FILE" 2>/dev/null)
    [[ "$_c" =~ ^[0-9]+$ ]] && start="$_c"
  fi
  [[ "$start" -ge "$nrepos" ]] && start=0
fi

PROC=()
if [[ "$MAX_REPOS" -eq 0 ]]; then
  PROC=("${REPOS[@]}")
  next_cursor=0
else
  i="$start"
  taken=0
  while [[ "$taken" -lt "$MAX_REPOS" && "$i" -lt "$nrepos" ]]; do
    PROC+=("${REPOS[$i]}")
    i=$((i + 1)); taken=$((taken + 1))
  done
  # No wrap within a cycle: the next cycle resumes where this one stopped (0 at
  # the end of the list).
  next_cursor="$i"
  [[ "$next_cursor" -ge "$nrepos" ]] && next_cursor=0
fi

# An ARM run only when NOT ONE repo in this cycle's slice already has state — a
# repo seen for the first time (a fresh set, or a repo the cursor reaches for the
# first time) seeds silently and does not re-flood.
armed_run=1
for repo in "${PROC[@]}"; do
  if [[ -f "$(statefile "$repo")" ]]; then armed_run=0; break; fi
done

degraded=0
armed_total=0
read_done=0        # how many gh calls this cycle has made (for pacing)
proc_n=${#PROC[@]}

idx=0
while [[ "$idx" -lt "$proc_n" ]]; do
  repo="${PROC[$idx]}"
  sf=$(statefile "$repo")

  # ---- A. mint this repo's read token BEFORE any pacing sleep: a repo that
  # cannot be read makes no call, so it is never paced around and never counts
  # as a read. A failed mint is a failed read for THIS repo only: no `gh` call,
  # and the captured-stderr file is emptied so a stale rate-limit signature from
  # an earlier repo cannot be mistaken for this one's.
  mint_failed=0
  if ! mint_for_repo "$repo"; then
    mint_failed=1
    : > "$TMP_ERR"
  fi

  if [[ "$mint_failed" -eq 1 ]]; then
    read_ok=0
    hits=""
  else
    # PACING: sleep between consecutive reads, never before the first read and
    # never around a repo that makes no call.
    if [[ "$read_done" -gt 0 && "$PACE" -gt 0 ]]; then
      sleep "$PACE"
    fi

    # Poll. The token is handed to this one `gh` as its GH_TOKEN and is never
    # exported. `if hits=$(...)` keeps `set -e`-free status inspection; gh's own
    # diagnostics are captured, never discarded.
    if hits=$(GH_TOKEN="$MINT_TOK" gh pr list --repo "$repo" --state open --limit "$LIMIT" \
        --json number,headRefOid,isDraft,state,mergeStateStatus 2>"$TMP_ERR"); then
      read_ok=1
    else
      read_ok=0
    fi
    read_done=$((read_done + 1))
  fi

  # ---- C. stop-on-limit -------------------------------------------------
  if [[ "$read_ok" -eq 0 && "$mint_failed" -eq 0 ]] && is_ratelimit; then
    # The tripped repo and every repo after it in this cycle degrade WITHOUT a
    # further call. Retain each baseline; end the cycle here.
    degraded=1
    j="$idx"
    while [[ "$j" -lt "$proc_n" ]]; do
      printf 'MONITOR-DEGRADED: %s rate-limited, skipped\n' "${PROC[$j]}"
      j=$((j + 1))
    done
    # Next cycle resumes at the tripped repo so the skipped tail is retried.
    if [[ "$MAX_REPOS" -gt 0 ]]; then
      # Global index of the tripped repo = start + idx.
      next_cursor=$((start + idx))
      [[ "$next_cursor" -ge "$nrepos" ]] && next_cursor=0
    fi
    break
  fi

  at_limit=0
  if [[ "$read_ok" -eq 1 ]]; then
    [[ -z "$hits" ]] && hits="[]"
    # Current keyset: "<num> <head-sha> <draft|ready> <state> <mergeState>",
    # one per line, sorted numerically by PR number for a stable field order.
    printf '%s' "$hits" \
      | jq -r '.[] | "\(.number) \(.headRefOid) \(if .isDraft then "draft" else "ready" end) \(.state) \(.mergeStateStatus)"' \
      | LC_ALL=C sort -n > "$TMP_CUR"
    cur_n=$(countlines "$TMP_CUR")
    # A read that comes back at EXACTLY the limit is TRUNCATED: `gh` keeps only
    # the newest LIMIT and gives no truncation signal of its own.
    [[ "$cur_n" -ge "$LIMIT" ]] && at_limit=1
  else
    cur_n=0
  fi

  # ---- seed: first sight of this repo -----------------------------------
  if [[ ! -f "$sf" ]]; then
    if [[ "$read_ok" -eq 1 && "$at_limit" -eq 1 ]]; then
      # A TRUNCATED seed would bake a moving-window baseline in as the whole set.
      degraded=1
      printf 'MONITOR-DEGRADED: %s seed returned %s == --limit %s — results TRUNCATED, no baseline established; raise PR_MONITOR_LIMIT and retry\n' \
        "$repo" "$cur_n" "$LIMIT"
    elif [[ "$read_ok" -eq 1 ]]; then
      cp "$TMP_CUR" "$sf"
      armed_total=$((armed_total + 1))
    elif [[ "$mint_failed" -eq 1 ]]; then
      degraded=1
      printf 'MONITOR-DEGRADED: %s token mint FAILED (%s) — could-not-check, no baseline established, will retry next cycle\n' \
        "$repo" "$MINT_WHY"
    else
      degraded=1
      printf 'MONITOR-DEGRADED: %s seed read FAILED (gh: %s) — no baseline established, will retry next cycle\n' \
        "$repo" "$(tr '\n' ' ' <"$TMP_ERR")"
    fi
    idx=$((idx + 1))
    continue
  fi

  prev_n=$(countlines "$sf")

  if [[ "$mint_failed" -eq 1 ]]; then
    # No read identity for this repo — RETAIN the previous baseline, go loud. The
    # repo is neither empty nor unchanged: it is could-not-check.
    degraded=1
    printf 'MONITOR-DEGRADED: %s token mint FAILED (%s) — could-not-check, keeping its previous %s PR(s)\n' \
      "$repo" "$MINT_WHY" "$prev_n"
    idx=$((idx + 1))
    continue
  fi

  if [[ "$read_ok" -eq 0 ]]; then
    # Read FAILED (not a rate limit) — RETAIN the previous baseline, go loud.
    degraded=1
    printf 'MONITOR-DEGRADED: %s read FAILED (gh: %s) — keeping its previous %s PR(s)\n' \
      "$repo" "$(tr '\n' ' ' <"$TMP_ERR")" "$prev_n"
    idx=$((idx + 1))
    continue
  fi

  if [[ "$at_limit" -eq 1 ]]; then
    # TRUNCATED read on an established repo — a moving window, not ground truth.
    degraded=1
    printf 'MONITOR-DEGRADED: %s returned %s == --limit %s — results TRUNCATED, treating as could-not-check; keeping its previous %s PR(s)\n' \
      "$repo" "$cur_n" "$LIMIT" "$prev_n"
    idx=$((idx + 1))
    continue
  fi

  # Healthy read. Diff current against the retained baseline, per PR number, and
  # emit one PR-EVENT line per CHANGED dimension — a push and a draft-flip on the
  # same PR in one cycle are two lines, never collapsed to one.
  awk -v repo="$repo" '
    NR==FNR {
      b_sha[$1]=$2; b_draft[$1]=$3; b_state[$1]=$4; b_ms[$1]=$5; b_seen[$1]=1; next
    }
    {
      num=$1; sha=$2; draft=$3; state=$4; ms=$5; c_seen[num]=1
      if (!(num in b_seen)) { printf "PR-EVENT: %s#%s opened - -> %s\n", repo, num, sha; next }
      if (b_sha[num]   != sha)   printf "PR-EVENT: %s#%s pushed %s -> %s\n",      repo, num, b_sha[num],   sha
      if (b_draft[num] != draft) printf "PR-EVENT: %s#%s draft-flip %s -> %s\n",  repo, num, b_draft[num], draft
      if (b_state[num] != state) printf "PR-EVENT: %s#%s state %s -> %s\n",       repo, num, b_state[num], state
      if (b_ms[num]    != ms)    printf "PR-EVENT: %s#%s merge-state %s -> %s\n", repo, num, b_ms[num],    ms
    }
    END {
      for (n in b_seen) if (!(n in c_seen)) printf "PR-EVENT: %s#%s closed %s -> -\n", repo, n, b_sha[n]
    }
  ' "$sf" "$TMP_CUR" > "$TMP_EVT"

  # Emit closed events sorted first is not required; print as produced, then keep
  # the opened/pushed/... order awk already imposes.
  cat "$TMP_EVT"

  # Advance this repo's baseline to the fresh read.
  cp "$TMP_CUR" "$sf"
  idx=$((idx + 1))
done

# Carry the cursor for the next cycle (only meaningful when a cap is in force).
if [[ "$MAX_REPOS" -gt 0 ]]; then
  printf '%s\n' "$next_cursor" > "$CURSOR_FILE"
fi

if [[ "$armed_run" -eq 1 && "$degraded" -eq 0 ]]; then
  _capdesc="$MAX_REPOS"; [[ "$MAX_REPOS" -eq 0 ]] && _capdesc="all"
  printf 'MONITOR-ARMED: %s repos (pace %ss, cap %s)\n' "$armed_total" "$PACE" "$_capdesc"
fi

[[ "$degraded" -eq 0 ]] || exit 2
exit 0
