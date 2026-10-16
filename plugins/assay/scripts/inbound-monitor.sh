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
#      never polls as whatever token the launching shell happened to carry — and
#      it never polls as the ambient `gh` login either, which is some OTHER
#      identity than the desk's role and under a replaced HOME resolves to no
#      usable account at all. The identity it polls as is, in order:
#        1. an EXPLICIT read identity the caller names per owner with
#           `--token-file OWNER=PATH` — a 0600 file holding an installation
#           token already minted for that owner. The file is read, never
#           copied; the token reaches only the one `gh` read of that owner's
#           repo. This is what makes the poller work under a replaced HOME (a
#           desk cell's), where an ambient login resolves to no usable
#           account and every read 401s.
#        2. otherwise a token minted for THAT repo, INSIDE the poll, for the
#           session's role: `desktoken <role> --repo <owner>/<repo>` (the role
#           comes from $DESK_LOOP). Its content reaches only the one `gh` read
#           of that repo as `GH_TOKEN=... gh ...`; it is never exported, never
#           printed, never reused for another repo.
#      A mint that fails — no role for $DESK_LOOP, no `desktoken` on PATH, a
#      refused or unverifiable mint, a missing/empty token file — is
#      MONITOR-DEGRADED (could-not-check) for THAT repo with no `gh` call at
#      all: never an empty result, and NEVER a fallback to the ambient `gh`
#      login. A named --token-file that cannot be used (missing, empty, not
#      0600) is a precondition failure: the poller never swaps the identity it
#      was told to use for another one.
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
#        · the read pages through the whole open set up to a CEILING (--limit),
#          and asks `gh` for ONE row past it: a read that holds MORE than the
#          ceiling is TRUNCATED (`gh` keeps only the newest it was asked for and
#          gives no truncation signal of its own), so it is a moving window, not
#          ground truth: retain + MONITOR-DEGRADED, never diffed. A set of
#          exactly the ceiling is the whole set;
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
# App token the launching shell exported, this monitor is not it. This only
# scrubs the environment; the identity a read DOES run under is the owner's
# `--token-file` token (parsed below) or the per-repo minted role token
# (mint_for_repo, below) — never this environment and never the ambient login.
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
# LIMIT is the per-repo read CEILING, not a page window. The default (2000) is
# the Go verb's (deskmonitor inbound, inboundDefaultLimit), and parity pins the
# two together. The verb reads through a forge client whose open-issue walk
# stops at its own page guard, so the default sits below that guard: one row
# past the ceiling still reads whole there, and the two pollers decide the
# ceiling's boundary the same way. It replaced a fixed 500 that sat at the
# same number as the truncation threshold: a repo past 500 open issues read as
# TRUNCATED on every cycle.
LIMIT="${INBOUND_MONITOR_LIMIT:-2000}"
BURST_CAP="${INBOUND_MONITOR_BURST_CAP:-25}"
# Retain floor: a read that returns fewer than this PERCENT of the repo's
# previous count is treated as a partial/could-not-check read (retain + go loud),
# not trusted as ground truth. It covers the middle of the range the bare
# zero-check leaves open. 0 disables the proportional floor (the zero-check and
# the at-limit check still stand).
RETAIN_FLOOR="${INBOUND_MONITOR_RETAIN_FLOOR:-50}"
# Pacing (shared vocabulary with pr-monitor.sh): ASSAY_MONITOR_PACE_SECONDS is
# slept BETWEEN consecutive repo reads so a wide repo set does not become a
# tight-loop poll that trips the forge's secondary rate limit. Default 2; 0
# disables the sleep (the test's own speed).
PACE="${ASSAY_MONITOR_PACE_SECONDS:-2}"

usage() {
  cat <<'EOF'
Usage: inbound-monitor.sh [--token-file OWNER=PATH ...] [owner/repo ...]

Stateful open-issue monitor across the given repos (or ./.assay/repos.txt, or the
current repo's origin remote). Meant to be re-run on a cadence behind the harness
`Monitor` tool. On the first run it SEEDS (prints `MONITOR-ARMED: <total>`); on
every run after that it prints one `INBOUND: <slug>#<num> <updatedAt>` per newly
seen or updated issue.

It never silently goes blind:
  · it never polls as an inherited token (GH_TOKEN/GITHUB_TOKEN are unset) and
    never as the ambient `gh` login: a repo whose owner was given --token-file is
    read as that file's token, every other repo as the session's role App, with
    that repo's token minted by `desktoken <role> --repo <slug>` ($DESK_LOOP names
    the role) and handed to that one `gh` call only;
  · a repo whose token cannot be minted (no role for $DESK_LOOP, no `desktoken`,
    a refused mint, an empty token file) prints
    `MONITOR-DEGRADED: <slug> token mint FAILED (...)`, makes no read and keeps its
    previous state: could-not-check, never an empty result;
  · a repo whose read fails, or which returns zero when it previously had issues,
    RETAINS its previous state and prints `MONITOR-DEGRADED: <slug> ...`;
  · the read pages through the whole open set; a repo holding MORE than the
    --limit ceiling is TRUNCATED (gh gives no truncation signal of its own), so
    it is treated as could-not-check: retain + go loud (a set of exactly the
    ceiling is the whole set);
  · a read that collapses below the retain floor of its previous count is a
    partial read: retain + go loud, so the recovery cycle absorbs it instead of
    flooding;
  · a burst of more than the cap new items for one repo collapses to a single
    `INBOUND-BURST: N over <cap> — listing suppressed` line.

Options:
  --token-file OWNER=PATH     read OWNER's repos as the installation token held in
                              PATH (a 0600 file, read and never copied). Repeatable,
                              one per owner. Outranks the per-repo minted token.
                              An unusable file is a precondition failure (exit 1),
                              never a silent fallback to another identity.

Environment:
  INBOUND_MONITOR_STATE_DIR   where per-repo state lives (default $TMPDIR/assay-inbound-monitor).
  INBOUND_MONITOR_LIMIT       per-repo read ceiling; `gh issue list` is asked for one
                              row past it (default 2000).
  INBOUND_MONITOR_BURST_CAP   new-items-per-repo-per-cycle listing cap (default 25).
  INBOUND_MONITOR_RETAIN_FLOOR percent-of-previous below which a read is treated as
                              partial (retain + degrade); 0 disables (default 50).

Exit codes:
  0  every repo polled cleanly (armed, quiet, or emitted INBOUND lines)
  1  precondition failure — gh/jq missing, no repos resolvable (or an invalid repo
     token), unusable state dir, malformed or unusable --token-file. When the repo
     set cannot be resolved, a `MONITOR-DEGRADED: <reason>` line is printed on
     STDOUT first (the line the watch filter matches); stderr alone is invisible to it.
  2  at least one repo went DEGRADED (token mint failed, read failed, truncated, or
     collapsed) — state RETAINED
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
for _kv in "LIMIT=$LIMIT" "BURST_CAP=$BURST_CAP" "RETAIN_FLOOR=$RETAIN_FLOOR" "PACE=$PACE"; do
  if [[ ! "${_kv#*=}" =~ ^[0-9]+$ ]]; then
    echo "inbound-monitor: ${_kv%%=*} must be a non-negative integer, got '${_kv#*=}'" >&2
    exit 1
  fi
done
# FETCH — what `gh issue list` is asked for: one row past the ceiling, so a set
# of exactly LIMIT is told apart from a set clipped at LIMIT.
FETCH=$((LIMIT + 1))

# lower <s> — ASCII lowercase (bash 3.2 has no ${v,,}). Owners compare
# case-insensitively, as the forge does.
lower() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

# ---- A. explicit read identity: --token-file OWNER=PATH --------------------
# Parsed out of the argument list (it may appear anywhere); every other argument
# is a repo. bash 3.2 has no associative arrays, so the owner -> token map is two
# parallel indexed arrays.
REPO_ARGS=()
TF_OWNERS=()
TF_PATHS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --token-file)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "inbound-monitor: --token-file needs a value (OWNER=PATH)" >&2
        exit 1
      fi
      _tf="$2"
      shift 2
      if [[ ! "$_tf" =~ ^([A-Za-z0-9._-]+)=(.+)$ ]]; then
        echo "inbound-monitor: --token-file '$_tf' — expected OWNER=PATH (an installation token belongs to one owner's installation)" >&2
        exit 1
      fi
      _owner=$(lower "${BASH_REMATCH[1]}")
      _path="${BASH_REMATCH[2]}"
      for _seen in ${TF_OWNERS[@]+"${TF_OWNERS[@]}"}; do
        if [[ "$_seen" == "$_owner" ]]; then
          echo "inbound-monitor: --token-file for owner '$_owner' given twice" >&2
          exit 1
        fi
      done
      TF_OWNERS+=("$_owner")
      TF_PATHS+=("$_path")
      ;;
    *)
      REPO_ARGS+=("$1")
      shift
      ;;
  esac
done

# Validate every named token file BEFORE any read, in the order deskclaim-ref
# checks its own --token-file: exists, regular file, owner-only (0600), readable,
# non-empty. Group/world bits are tested with POSIX `find -perm -NNN`, which
# reads the same on BSD and GNU find. The token VALUE is never printed.
TF_TOKENS=()
_i=0
while [[ "$_i" -lt "${#TF_PATHS[@]}" ]]; do
  _path="${TF_PATHS[$_i]}"
  if [[ ! -e "$_path" ]]; then
    echo "inbound-monitor: cannot read --token-file $_path for owner '${TF_OWNERS[$_i]}' (no such file)" >&2
    exit 1
  fi
  if [[ ! -f "$_path" ]]; then
    echo "inbound-monitor: --token-file $_path is not a regular file" >&2
    exit 1
  fi
  if [[ -n "$(find "$_path" -prune \( -perm -040 -o -perm -020 -o -perm -010 \
        -o -perm -004 -o -perm -002 -o -perm -001 \) -print 2>/dev/null)" ]]; then
    echo "inbound-monitor: --token-file $_path is group/world accessible; it must be 0600" >&2
    exit 1
  fi
  if [[ ! -r "$_path" ]] || ! _tok=$(tr -d '[:space:]' < "$_path" 2>/dev/null); then
    echo "inbound-monitor: cannot read --token-file $_path" >&2
    exit 1
  fi
  if [[ -z "$_tok" ]]; then
    echo "inbound-monitor: --token-file $_path is empty" >&2
    exit 1
  fi
  TF_TOKENS+=("$_tok")
  _i=$((_i + 1))
done
unset _tok

# token_for <owner/name> -> the explicit token for that repo's owner, or nothing
# (=> the repo is read as a token minted for it, mint_for_repo below).
token_for() {
  local owner i=0
  owner=$(lower "${1%%/*}")
  while [[ "$i" -lt "${#TF_OWNERS[@]}" ]]; do
    if [[ "${TF_OWNERS[$i]}" == "$owner" ]]; then
      printf '%s' "${TF_TOKENS[$i]}"
      return
    fi
    i=$((i + 1))
  done
}

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
done < <(resolve_repos ${REPO_ARGS[@]+"${REPO_ARGS[@]}"})

if [[ -z "${REPOS[*]+x}" || ${#REPOS[@]} -eq 0 ]]; then
  echo "MONITOR-DEGRADED: no repos to query (no repo args, no ./.assay/repos.txt, and no usable git origin remote)"
  echo "inbound-monitor: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)" >&2
  exit 1
fi

# Validate every repo token before it can reach a shell (assay-inbox parity):
# GitHub's own owner/name alphabet, nothing else.
for _repo in "${REPOS[@]}"; do
  if [[ ! "$_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
    # Print the token with any user[:password]@ stripped: stdout is a watch
    # stream, and an origin URL can carry a credential.
    _shown="$(origin_display "$_repo")"
    echo "MONITOR-DEGRADED: invalid repo '$_shown' (expected owner/name)"
    echo "inbound-monitor: invalid repo '$_shown' (expected owner/name)" >&2
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
TMP_MINT=$(mktemp)
trap 'rm -f "$TMP_CUR" "$TMP_ERR" "$TMP_NEW" "$TMP_MINT"' EXIT

# ---- A. the read identity: a per-repo minted role token ---------------------
# The App role this session acts as, from its loop identity ($DESK_LOOP). The
# table is the compiled loop-to-role one (see "Forge CLI as the role" in the desk
# shell reference); a loop that carries no role — or an unset $DESK_LOOP — leaves
# ROLE empty, which degrades EVERY repo that has no --token-file: the monitor
# never guesses an identity and never substitutes the ambient `gh` login.
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

# `grep -c` prints "0" AND exits 1 on no match, so `grep -c ... || echo 0` would
# emit TWO zeros and poison the arithmetic that follows. Count via this helper
# instead: exactly one number, "0" for an empty or absent file.
countlines() {
  local n
  n=$(grep -c . "$1" 2>/dev/null)
  printf '%s' "${n:-0}"
}

# The secondary-rate-limit / 429 signature, read from the captured gh stderr. A
# tripped limit must not be compounded by the remaining reads (see the loop).
is_ratelimit() {
  grep -qiE 'secondary rate limit|(http )?429|too many requests' "$TMP_ERR"
}

# Is this an ARM run? Only if NOT ONE resolved repo already has state. A repo
# added to an already-armed set seeds silently — we never re-flood on expansion.
armed_run=1
for repo in "${REPOS[@]}"; do
  if [[ -f "$(statefile "$repo")" ]]; then armed_run=0; break; fi
done

degraded=0
armed_total=0
read_done=0        # how many gh reads this cycle has made (paces the next one)
_ri=-1             # index of the current repo (for the stop-on-limit tail)

for repo in "${REPOS[@]}"; do
  _ri=$((_ri + 1))
  sf=$(statefile "$repo")

  # Identity (A): the owner's explicit --token-file token when one was named, else
  # a token minted for THIS repo (before any pacing sleep: a repo that cannot be
  # read makes no call, so it is never paced around and never counts as a read).
  # A failed mint is a failed read for this repo only: no `gh` call, and the
  # captured-stderr file is emptied so a stale rate-limit signature from an
  # earlier repo cannot be mistaken for this one's.
  mint_failed=0
  _rtok=$(token_for "$repo")
  if [[ -z "$_rtok" ]]; then
    if mint_for_repo "$repo"; then
      _rtok="$MINT_TOK"      # the value that was checked is the value gh gets
    else
      mint_failed=1
      : > "$TMP_ERR"
    fi
  fi

  if [[ "$mint_failed" -eq 1 ]]; then
    read_ok=0
    hits=""
  else
    # PACING: sleep between consecutive reads — never before the first, never
    # around a repo that makes no call.
    if [[ "$read_done" -gt 0 && "$PACE" -gt 0 ]]; then
      sleep "$PACE"
    fi

    # Poll. `if hits=$(...)` keeps `set -e`-free status inspection; gh's own
    # diagnostics are captured, never discarded — they are the only thing that
    # tells an operator their token expired or their identity is wrong. The token
    # is set for this one command only, so it never reaches another repo's read
    # and is never exported.
    if hits=$(GH_TOKEN="$_rtok" gh issue list --repo "$repo" --state open \
        --limit "$FETCH" --json number,updatedAt 2>"$TMP_ERR"); then
      read_ok=1
    else
      read_ok=0
    fi
    read_done=$((read_done + 1))
  fi

  # STOP-ON-LIMIT: a tripped secondary rate limit marks this repo and every
  # repo after it `rate-limited, skipped` (baselines retained) and ends the
  # cycle without another `gh` call.
  if [[ "$read_ok" -eq 0 && "$mint_failed" -eq 0 ]] && is_ratelimit; then
    degraded=1
    _rj=$_ri
    while [[ "$_rj" -lt "${#REPOS[@]}" ]]; do
      printf 'MONITOR-DEGRADED: %s rate-limited, skipped\n' "${REPOS[$_rj]}"
      _rj=$((_rj + 1))
    done
    break
  fi

  at_limit=0
  if [[ "$read_ok" -eq 1 ]]; then
    [[ -z "$hits" ]] && hits="[]"
    # Current keyset: "<slug>#<num> <updatedAt>", one per line, sorted.
    printf '%s' "$hits" \
      | jq -r --arg repo "$repo" '.[] | "\($repo)#\(.number) \(.updatedAt)"' \
      | LC_ALL=C sort > "$TMP_CUR"
    cur_n=$(countlines "$TMP_CUR")
    # A read that holds MORE than the ceiling is TRUNCATED: `gh` keeps only the
    # newest FETCH (= LIMIT + 1) and drops the rest, and gives no truncation
    # signal of its own — the one extra row is the signal. A set of exactly
    # LIMIT is the whole set. A truncated read is a moving window: it hides
    # everything past the cap AND fires false INBOUND when the window shifts, so
    # it cannot be trusted as ground truth.
    [[ "$cur_n" -gt "$LIMIT" ]] && at_limit=1
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
      printf 'MONITOR-DEGRADED: %s seed returned more than --limit %s — results TRUNCATED, no baseline established; raise INBOUND_MONITOR_LIMIT and retry\n' \
        "$repo" "$LIMIT"
    elif [[ "$read_ok" -eq 1 ]]; then
      cp "$TMP_CUR" "$sf"
      armed_total=$((armed_total + cur_n))
    elif [[ "$mint_failed" -eq 1 ]]; then
      degraded=1
      printf 'MONITOR-DEGRADED: %s token mint FAILED (%s) — could-not-check, no baseline established, will retry next cycle\n' \
        "$repo" "$MINT_WHY"
    else
      # Cannot even seed — surface it, but retain nothing (there was nothing).
      degraded=1
      printf 'MONITOR-DEGRADED: %s seed read FAILED (gh: %s) — no baseline established, will retry next cycle\n' \
        "$repo" "$(tr '\n' ' ' <"$TMP_ERR")"
    fi
    continue
  fi

  prev_n=$(countlines "$sf")

  if [[ "$mint_failed" -eq 1 ]]; then
    # No read identity for this repo — RETAIN the previous baseline, go loud. The
    # repo is neither empty nor unchanged: it is could-not-check.
    degraded=1
    printf 'MONITOR-DEGRADED: %s token mint FAILED (%s) — could-not-check, keeping its previous %s issue(s)\n' \
      "$repo" "$MINT_WHY" "$prev_n"
    continue
  fi

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
    printf 'MONITOR-DEGRADED: %s returned more than --limit %s — results TRUNCATED, treating as could-not-check; keeping its previous %s issue(s)\n' \
      "$repo" "$LIMIT" "$prev_n"
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
