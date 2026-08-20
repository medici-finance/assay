#!/usr/bin/env bash
# inbound-monitor.sh — the durable, stateful "did anything new arrive?" poller
# that a desk window arms behind the harness `Monitor` tool.
#
# It is the port of the session-local reference monitor the intake desk grew,
# carrying the three properties that a hand-rolled `gh`-polling loop keeps
# getting wrong — each property closing one way a desk monitor SILENTLY GOES
# BLIND:
#
#   A. EXPLICIT IDENTITY. A background monitor inherits the environment of the
#      shell that launched it. Desk work routinely exports a role token into
#      GH_TOKEN to post as an App, and a private App installation token cannot
#      see the desk's own private repo set at all — it 404s every read with
#      "Could not resolve to a Repository", which is byte-indistinguishable from
#      "this repo has no open issues" once stderr is discarded. So this monitor
#      sets its own identity: it UNSETS GH_TOKEN and GITHUB_TOKEN at the top and
#      always polls as the keyring account, never as whatever App token the
#      launching shell happened to carry.
#
#   B. PER-SOURCE STATE WITH RETENTION. A single global "too few records" floor
#      is useless when one source dominates the set: with 324 of 440 issues in
#      the home repo, losing EVERY one of a 94-issue repo still clears any sane
#      global floor — and the next good cycle then reports all 94 as new, a
#      flood of stale items that reads exactly like real inbound work. So state
#      is kept PER REPO, one file each, and liveness is judged
#      per repo:
#        · a repo whose read FAILS retains its previous lines and emits a
#          MONITOR-DEGRADED line naming itself — it never silently goes empty;
#        · a repo that returns ZERO when it previously had issues is treated the
#          same way (retain + MONITOR-DEGRADED) — a repo is never allowed to go
#          empty silently, even on a "successful" read;
#        · a read that comes back AT the --limit is TRUNCATED (`gh` keeps only
#          the newest LIMIT and gives no truncation signal), so it is a moving
#          window, not ground truth: retain + MONITOR-DEGRADED, never diffed;
#        · a read that COLLAPSES below the retain floor (a partial read — the
#          middle of the range the bare zero-check misses, and exactly what a
#          silent truncation or half-parse produces) is retained too;
#        · because the untrusted read's baseline is RETAINED (not rewritten),
#          the next good cycle diffs against the real baseline and the outage is
#          ABSORBED — zero phantom INBOUND events.
#
#   C. BURST CAP. A genuine mass update (a relabel sweep, a migration) must not
#      spam the session either, so more than INBOUND_MONITOR_BURST_CAP new keys
#      in one cycle for one repo collapses to a single
#      `INBOUND-BURST: N over <cap> — listing suppressed` line.
#
# Read-only: shells `gh issue list` only; never writes to any issue. Writes only
# its own per-repo state files under the state dir. bash 3.2 safe (stock
# /bin/bash on macOS) — no mapfile/readarray, no associative arrays, no ${v^^}.
# inbound-monitor.test.sh pins every property above against a stubbed `gh`.
#
# ---------------------------------------------------------------------------
# A. EXPLICIT IDENTITY — the very first thing, before any `gh` read. Whatever
# App token the launching shell exported, this monitor is not it. Unsetting
# these makes `gh` fall back to the keyring account (`gh auth`), which is the
# human/desk identity that can actually see the private repo set.
unset GH_TOKEN GITHUB_TOKEN

set -uo pipefail

# Repo resolution order (same contract as assay-inbox.sh):
#   1. repo args on the command line (owner/name ...)
#   2. ./.assay/repos.txt (one owner/name per line; blank lines and #-comments ignored)
#   3. the current repo's `origin` remote, parsed to owner/name
#
# State: one file per repo under STATE_DIR, named "<owner>__<name>.state", each
# line "<owner>/<name>#<num> <updatedAt>". The presence of the file is the seed
# marker: no file => this repo has never been polled => SEED it silently.
STATE_DIR="${INBOUND_MONITOR_STATE_DIR:-${TMPDIR:-/tmp}/assay-inbound-monitor}"
LIMIT="${INBOUND_MONITOR_LIMIT:-500}"
BURST_CAP="${INBOUND_MONITOR_BURST_CAP:-25}"
# Retain floor: a read that returns fewer than this PERCENT of the repo's
# previous count is treated as a partial/could-not-check read (retain + go loud),
# not trusted as ground truth. It covers the middle of the range the bare
# zero-check leaves open. 0 disables the proportional floor (the zero-check and
# the at-limit check still stand).
RETAIN_FLOOR="${INBOUND_MONITOR_RETAIN_FLOOR:-50}"

usage() {
  cat <<'EOF'
Usage: inbound-monitor.sh [owner/repo ...]

Stateful open-issue monitor across the given repos (or ./.assay/repos.txt, or the
current repo's origin remote). Meant to be re-run on a cadence behind the harness
`Monitor` tool. On the first run it SEEDS (prints `MONITOR-ARMED: <total>`); on
every run after that it prints one `INBOUND: <slug>#<num> <updatedAt>` per newly
seen or updated issue.

It never silently goes blind:
  · it polls as the keyring account (GH_TOKEN/GITHUB_TOKEN are unset), never as
    an inherited App token that cannot see the private repo set;
  · a repo whose read fails, or which returns zero when it previously had issues,
    RETAINS its previous state and prints `MONITOR-DEGRADED: <slug> ...`;
  · a read that comes back AT the --limit is TRUNCATED (gh gives no truncation
    signal of its own), so it is treated as could-not-check: retain + go loud;
  · a read that collapses below the retain floor of its previous count is a
    partial read: retain + go loud, so the recovery cycle absorbs it instead of
    flooding;
  · a burst of more than the cap new items for one repo collapses to a single
    `INBOUND-BURST: N over <cap> — listing suppressed` line.

Environment:
  INBOUND_MONITOR_STATE_DIR   where per-repo state lives (default $TMPDIR/assay-inbound-monitor).
  INBOUND_MONITOR_LIMIT       per-repo fetch cap passed to `gh issue list` (default 500).
  INBOUND_MONITOR_BURST_CAP   new-items-per-repo-per-cycle listing cap (default 25).
  INBOUND_MONITOR_RETAIN_FLOOR percent-of-previous below which a read is treated as
                              partial (retain + degrade); 0 disables (default 50).

Exit codes:
  0  every repo polled cleanly (armed, quiet, or emitted INBOUND lines)
  1  precondition failure — gh/jq missing, no repos resolvable, unusable state dir
  2  at least one repo went DEGRADED (read failed, truncated, or collapsed) — state RETAINED
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

command -v gh >/dev/null 2>&1 || { echo "inbound-monitor: gh CLI not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "inbound-monitor: jq not found" >&2; exit 1; }

# Numeric knobs must be non-negative integers — a non-numeric value would poison
# the arithmetic guards below and could silently disable a fail-closed check.
for _kv in "LIMIT=$LIMIT" "BURST_CAP=$BURST_CAP" "RETAIN_FLOOR=$RETAIN_FLOOR"; do
  if [[ ! "${_kv#*=}" =~ ^[0-9]+$ ]]; then
    echo "inbound-monitor: ${_kv%%=*} must be a non-negative integer, got '${_kv#*=}'" >&2
    exit 1
  fi
done

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
  owner_name=$(printf '%s' "$origin_url" \
    | sed -E 's#^git@github\.com:##; s#^https://github\.com/##; s#\.git$##')
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
  echo "inbound-monitor: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)" >&2
  exit 1
fi

# Validate every repo token before it can reach a shell (assay-inbox parity):
# GitHub's own owner/name alphabet, nothing else.
for _repo in "${REPOS[@]}"; do
  if [[ ! "$_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
    echo "inbound-monitor: invalid repo '$_repo' (expected owner/name)" >&2
    exit 1
  fi
done

mkdir -p "$STATE_DIR" 2>/dev/null || { echo "inbound-monitor: cannot create state dir '$STATE_DIR'" >&2; exit 1; }
[[ -w "$STATE_DIR" ]] || { echo "inbound-monitor: state dir '$STATE_DIR' is not writable" >&2; exit 1; }

# statefile <owner/name> -> path. Slashes are the only reserved char; map to "__".
statefile() {
  local slug="$1"
  printf '%s/%s.state' "$STATE_DIR" "$(printf '%s' "$slug" | sed 's#/#__#g')"
}

TMP_CUR=$(mktemp)
TMP_ERR=$(mktemp)
TMP_NEW=$(mktemp)
trap 'rm -f "$TMP_CUR" "$TMP_ERR" "$TMP_NEW"' EXIT

# `grep -c` prints "0" AND exits 1 on no match, so `grep -c ... || echo 0` would
# emit TWO zeros and poison the arithmetic that follows. Count via this helper
# instead: exactly one number, "0" for an empty or absent file.
countlines() {
  local n
  n=$(grep -c . "$1" 2>/dev/null)
  printf '%s' "${n:-0}"
}

# Is this an ARM run? Only if NOT ONE resolved repo already has state. A repo
# added to an already-armed set seeds silently — we never re-flood on expansion.
armed_run=1
for repo in "${REPOS[@]}"; do
  if [[ -f "$(statefile "$repo")" ]]; then armed_run=0; break; fi
done

degraded=0
armed_total=0

for repo in "${REPOS[@]}"; do
  sf=$(statefile "$repo")

  # Poll. `if hits=$(...)` keeps `set -e`-free status inspection; gh's own
  # diagnostics are captured, never discarded — they are the only thing that
  # tells an operator their token expired or their identity is wrong.
  if hits=$(gh issue list --repo "$repo" --state open \
      --limit "$LIMIT" --json number,updatedAt 2>"$TMP_ERR"); then
    read_ok=1
  else
    read_ok=0
  fi

  at_limit=0
  if [[ "$read_ok" -eq 1 ]]; then
    [[ -z "$hits" ]] && hits="[]"
    # Current keyset: "<slug>#<num> <updatedAt>", one per line, sorted.
    printf '%s' "$hits" \
      | jq -r --arg repo "$repo" '.[] | "\($repo)#\(.number) \(.updatedAt)"' \
      | LC_ALL=C sort > "$TMP_CUR"
    cur_n=$(countlines "$TMP_CUR")
    # A read that comes back at EXACTLY the limit is TRUNCATED: `gh` keeps only
    # the newest LIMIT and drops the rest, and gives no truncation signal of its
    # own — "came back at the limit" is the only signal available. An at-limit
    # read is a moving window: it hides everything past the cap AND fires false
    # INBOUND when the window shifts, so it cannot be trusted as ground truth.
    [[ "$cur_n" -ge "$LIMIT" ]] && at_limit=1
  else
    cur_n=0
  fi

  # ---- B. per-source liveness -------------------------------------------
  if [[ ! -f "$sf" ]]; then
    # SEED — first sight of this repo. Record silently; never emit INBOUND on a
    # seed (that is precisely the phantom-flood the bug produced).
    if [[ "$read_ok" -eq 1 && "$at_limit" -eq 1 ]]; then
      # A TRUNCATED seed would bake a moving-window baseline in as if it were the
      # whole repo. Refuse it — establish no baseline, go loud, retry next cycle
      # (raise INBOUND_MONITOR_LIMIT above this repo's open count).
      degraded=1
      printf 'MONITOR-DEGRADED: %s seed returned %s == --limit %s — results TRUNCATED, no baseline established; raise INBOUND_MONITOR_LIMIT and retry\n' \
        "$repo" "$cur_n" "$LIMIT"
    elif [[ "$read_ok" -eq 1 ]]; then
      cp "$TMP_CUR" "$sf"
      armed_total=$((armed_total + cur_n))
    else
      # Cannot even seed — surface it, but retain nothing (there was nothing).
      degraded=1
      printf 'MONITOR-DEGRADED: %s seed read FAILED (gh: %s) — no baseline established, will retry next cycle\n' \
        "$repo" "$(tr '\n' ' ' <"$TMP_ERR")"
    fi
    continue
  fi

  prev_n=$(countlines "$sf")

  if [[ "$read_ok" -eq 0 ]]; then
    # Read FAILED — RETAIN the previous baseline, go loud.
    degraded=1
    printf 'MONITOR-DEGRADED: %s read FAILED (gh: %s) — keeping its previous %s issue(s)\n' \
      "$repo" "$(tr '\n' ' ' <"$TMP_ERR")" "$prev_n"
    continue
  fi

  if [[ "$cur_n" -eq 0 && "$prev_n" -gt 0 ]]; then
    # Went empty on a "successful" read — a repo is never allowed to go empty
    # silently. RETAIN + go loud. (A genuine drain-to-zero trips this too; that
    # is the deliberate fail-closed direction — a false blind spot is the one
    # thing this class must never produce.)
    degraded=1
    printf 'MONITOR-DEGRADED: %s returned 0 (had %s) — keeping its previous %s issue(s)\n' \
      "$repo" "$prev_n" "$prev_n"
    continue
  fi

  if [[ "$at_limit" -eq 1 ]]; then
    # TRUNCATED read on an established repo — a moving window, not ground truth.
    # RETAIN the real baseline and go loud rather than diff against a partial
    # slice (which would both blind us past the cap and fire false INBOUND when
    # the window shifts). Same fail-closed direction as the zero case.
    degraded=1
    printf 'MONITOR-DEGRADED: %s returned %s == --limit %s — results TRUNCATED, treating as could-not-check; keeping its previous %s issue(s)\n' \
      "$repo" "$cur_n" "$LIMIT" "$prev_n"
    continue
  fi

  if [[ "$RETAIN_FLOOR" -gt 0 && "$prev_n" -gt 0 && $((cur_n * 100)) -lt $((prev_n * RETAIN_FLOOR)) ]]; then
    # PARTIAL read — a "successful", non-empty read that nonetheless collapsed to
    # under RETAIN_FLOOR% of the prior count. The bare zero-check does not see
    # this middle of the range, and a partial read is exactly what a silent
    # truncation or a half-parsed page produces: trust it and the next good cycle
    # reports the recovered items as a phantom flood. RETAIN + go loud; the
    # recovery cycle then diffs against the real baseline and absorbs it.
    degraded=1
    printf 'MONITOR-DEGRADED: %s returned %s, under %s%% of its previous %s — treating as a partial read; keeping its previous %s issue(s)\n' \
      "$repo" "$cur_n" "$RETAIN_FLOOR" "$prev_n" "$prev_n"
    continue
  fi

  # Healthy read. New = current keys absent from the retained baseline (a brand
  # new issue, OR an existing one whose updatedAt moved = a new comment).
  LC_ALL=C comm -13 <(LC_ALL=C sort "$sf") "$TMP_CUR" > "$TMP_NEW"
  new_n=$(countlines "$TMP_NEW")

  if [[ "$new_n" -gt "$BURST_CAP" ]]; then
    # ---- C. burst cap --------------------------------------------------
    printf 'INBOUND-BURST: %s %s over %s — listing suppressed\n' "$repo" "$new_n" "$BURST_CAP"
  elif [[ "$new_n" -gt 0 ]]; then
    while IFS= read -r _k; do
      [[ -z "$_k" ]] && continue
      printf 'INBOUND: %s\n' "$_k"
    done < "$TMP_NEW"
  fi

  # Advance this repo's baseline to the fresh read.
  cp "$TMP_CUR" "$sf"
done

if [[ "$armed_run" -eq 1 && "$degraded" -eq 0 ]]; then
  printf 'MONITOR-ARMED: %s\n' "$armed_total"
fi

[[ "$degraded" -eq 0 ]] || exit 2
exit 0
