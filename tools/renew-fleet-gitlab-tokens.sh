#!/usr/bin/env bash
# tools/renew-fleet-gitlab-tokens.sh — renew every Assay fleet role PAT on a
# GitLab instance in one run, and replace each role's local custody file.
#
# The companion to tools/create-fleet-gitlab.sh. The provisioner mints a PAT
# only for an account it creates in that run; for a pre-existing account it
# prints a NOTICE and moves on. This script is the one-shot renewal for every
# role that already exists: for each configured role it ROTATES the role's
# live PAT (matched by its stable name), or CREATES one when no active PAT of
# that name exists, and writes the returned secret to `gitlab-<role>.token`.
#
# The role table (role -> scopes) and the naming convention (username, PAT
# name, custody file) come from tools/fleet-gitlab-roles.sh — the same file
# the provisioner sources. There is no second copy here.
#
# ONE AUTHORITY MODEL: GROUP OWNER (#1630 item 3). `--group <top-level-group>`
# names the group; the script uses `glab api` against the group
# service-account PAT endpoints, which admit a group Owner:
#     GET  groups/:id                                                    (resolve, top-level check)
#     GET  user                                                          (the active identity)
#     GET  groups/:id/members/all/:user_id                               (Owner = access_level 50)
#     GET  groups/:id/service_accounts                                   (username -> id)
#     GET  groups/:id/service_accounts/:user_id/personal_access_tokens?state=active
#     POST groups/:id/service_accounts/:user_id/personal_access_tokens/:token_id/rotate
#     POST groups/:id/service_accounts/:user_id/personal_access_tokens
# It never requires, and never probes for, instance administrator: there is no
# `application/settings` read and no `glab token … --user` path.
#
# ORDER OF OPERATIONS. Every check runs before the first token mutation:
# arguments, tools, the output directory and every destination file, the
# active glab identity's authority, each service account's resolution, and
# each role's active PATs (rotate / create / REFUSE on more than one active
# PAT of the same name). Any preflight problem aborts with nothing rotated.
#
# SECRET CUSTODY. A returned PAT goes straight from glab's stdout into a fresh
# owner-only (umask 077, 0600) temp file in the destination's own directory;
# it is never held in argv, an environment variable, a log line, or this
# script's report. The file is validated (exactly one token-shaped line — any
# other shape is malformed and fails closed), normalised into a second 0600
# temp file, and renamed onto the destination: an atomic replace. When the
# destination is a symlink (the documented `gitlab-<role>.token` ->
# `<prefix>-<role>-bot.token` layout, docs/adopting-assay-gitlab.md §2) the
# rename lands on the link's target, which must sit in the output directory,
# so the link survives. Every temp file is removed on every exit path.
#
# NOT GLOBALLY ATOMIC. A rotation invalidates the role's previous PAT the
# moment the forge accepts it. Each role's swap is atomic; the fleet as a
# whole is not — a run that stops part-way leaves the roles before the
# failure renewed and the ones after it untouched. The run stops at the first
# failed role (fail closed) and prints the `--only` list that resumes it.
#
# Reports carry only role, destination path and outcome.

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLEET_ROLES_FILE="${here}/fleet-gitlab-roles.sh"
if [ ! -f "$FLEET_ROLES_FILE" ]; then
  echo "error: the shared fleet role table is missing: ${FLEET_ROLES_FILE}" >&2
  exit 2
fi
# shellcheck source=fleet-gitlab-roles.sh
# shellcheck disable=SC1091
. "$FLEET_ROLES_FILE"

GL_HOSTNAME=""
GROUP=""
PREFIX=""
OUT_DIR=""
DURATION="${FLEET_PAT_DAYS}d"
DRY_RUN=0
ONLY=""
ROTATE_IN_USE=0
ROLE_RECORDS=()

# IN_USE_WINDOW_DAYS — a role's active PAT is "in use" (protected from a
# default rotation, #1630 required behavior 5) when its last_used_at falls
# within this many days of now. A PAT whose last_used_at is null has never
# been used and is never in-use; a record WITHOUT the key is malformed and
# fails closed (MATCH_FILTER) rather than reading as never-used. A PAT used
# earlier than the window rotates normally. 1 day catches a PAT a live desk
# session touched recently; it does not block the fleet-wide renewal cadence.
IN_USE_WINDOW_DAYS=1

usage() {
  cat <<'USAGE'
Usage: renew-fleet-gitlab-tokens.sh --hostname <host> --group <top-level-group>
         (--prefix <prefix> | --role ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE] ...)
         --out-dir <dir> [--duration 7d] [--only ROLE[,ROLE...]]
         [--rotate-in-use] [--dry-run]

Renews every configured Assay fleet role PAT in one run: rotates each role's
active PAT (matched by name), creates one only when none exists, and replaces
<out-dir>/gitlab-<role>.token atomically.

Required:
  --hostname <host>       GitLab hostname (no scheme), e.g. gitlab.example.com.
  --group <group>         The top-level group owning the fleet's service
                           accounts. The active glab identity must be its
                           Owner; the group service-account PAT endpoints are
                           driven via `glab api`. Instance administrator is
                           never required and never probed.
  --out-dir <dir>         The role-token store (the config-home the desk verbs
                           read). Created 0700 if absent.

Roles (at least one source):
  --prefix <prefix>       Every role in tools/fleet-gitlab-roles.sh, with the
                           provisioner's naming: username <prefix>-<role>-bot,
                           PAT name assay-<role>-fleet, file gitlab-<role>.token.
  --role REC              An explicit record, repeatable:
                           ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE]
                           SCOPES is comma-separated; FILE is a base name
                           (default gitlab-<ROLE>.token). With --prefix, a
                           record for a table role replaces that row; any other
                           role is added.

Options:
  --duration <N>d|<N>w    PAT lifetime from today, 1d..365d. Default: 7d (the
                           shared fleet-gitlab-roles.sh FLEET_PAT_DAYS backstop,
                           spec.md §5). A longer value widens that backstop —
                           state the reason when you pass one.
  --only ROLE[,ROLE...]   Renew only these configured roles (resume a partial
                           run with the list the failed run printed).
  --rotate-in-use         Rotate a role even when its active PAT's
                           last_used_at is within the in-use window (default:
                           such a role is SKIPPED and reported, never rotated,
                           because rotation invalidates the previous secret
                           immediately and would 401 a session holding it).
  --dry-run               Run every preflight read and print the plan
                           (rotate / create / skip-in-use per role). No token
                           is rotated or created and no file is written.
  -h, --help              This text.

Auth: whatever identity `glab` is logged in as for --hostname (glab's own
credential store or its environment). This script never reads, stores or
passes that credential.

A rotation invalidates the role's previous PAT immediately. Each role's file
swap is atomic; the fleet-wide operation is NOT — stop the fleet's desk
sessions first, and resume a partial run with --only.

Exit status:
  0 = every selected role renewed (with --dry-run: the plan passed preflight);
  1 = a preflight refusal (nothing mutated) or a role failed part-way;
  2 = usage error;
  3 = the run finished but one or more roles were SKIPPED as in-use and not
      renewed — those PATs keep their previous expiry. The summary names them;
      re-run with --only <them> --rotate-in-use once nothing holds them.
USAGE
}

die_usage() { echo "error: $1" >&2; usage >&2; exit 2; }

need_val() {  # need_val FLAG ARGC
  [ "$2" -ge 2 ] || die_usage "$1 needs a value"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --hostname) need_val "$1" $#; GL_HOSTNAME="$2"; shift 2 ;;
    --group) need_val "$1" $#; GROUP="$2"; shift 2 ;;
    --prefix) need_val "$1" $#; PREFIX="$2"; shift 2 ;;
    --role) need_val "$1" $#; ROLE_RECORDS+=("$2"); shift 2 ;;
    --out-dir) need_val "$1" $#; OUT_DIR="$2"; shift 2 ;;
    --duration) need_val "$1" $#; DURATION="$2"; shift 2 ;;
    --only) need_val "$1" $#; ONLY="$2"; shift 2 ;;
    --rotate-in-use) ROTATE_IN_USE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die_usage "unknown argument: $1" ;;
  esac
done

# --- argument preflight ------------------------------------------------------
[ -n "$GL_HOSTNAME" ] || die_usage "--hostname is required"
printf '%s' "$GL_HOSTNAME" | grep -Eq '^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]+)?$' \
  || die_usage "--hostname must be a bare host[:port] (no scheme, no path): '${GL_HOSTNAME}'"

[ -n "$GROUP" ] || die_usage "--group <top-level-group> is required (the renewal runs as that group's Owner)"
printf '%s' "$GROUP" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_.-]*$' \
  || die_usage "--group must be a top-level group path or numeric id (no '/'): '${GROUP}'"

[ -n "$OUT_DIR" ] || die_usage "--out-dir is required"
case "$OUT_DIR" in /*) ;; *) OUT_DIR="${PWD}/${OUT_DIR}" ;; esac
OUT_DIR="${OUT_DIR%/}"
[ -n "$OUT_DIR" ] || OUT_DIR="/"

if [ -z "$PREFIX" ] && [ "${#ROLE_RECORDS[@]}" -eq 0 ]; then
  die_usage "no roles configured: pass --prefix <prefix> and/or one or more --role records"
fi
if [ -n "$PREFIX" ]; then
  printf '%s' "$PREFIX" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9_.-]*$' \
    || die_usage "--prefix may contain only letters, digits, '.', '_' and '-': '${PREFIX}'"
fi

case "$DURATION" in
  *d) DAYS="${DURATION%d}" ;;
  *w) DAYS="${DURATION%w}"; printf '%s' "$DAYS" | grep -Eq '^[0-9]+$' && DAYS=$((10#$DAYS * 7)) ;;
  *) DAYS="" ;;
esac
if ! printf '%s' "$DAYS" | grep -Eq '^[0-9]+$' || [ $((10#$DAYS)) -lt 1 ] || [ $((10#$DAYS)) -gt 365 ]; then
  die_usage "--duration must be <N>d or <N>w between 1d and 365d: '${DURATION}'"
fi
DAYS=$((10#$DAYS))

# --- role list ---------------------------------------------------------------
# Parallel indexed arrays (bash 3.2 has no associative arrays).
R_ROLE=(); R_USER=(); R_NAME=(); R_SCOPES=(); R_FILE=()

role_index() {  # role_index ROLE -> echoes the index, returns 1 when absent
  local i
  for i in "${!R_ROLE[@]}"; do
    [ "${R_ROLE[$i]}" = "$1" ] && { echo "$i"; return 0; }
  done
  return 1
}

if [ -n "$PREFIX" ]; then
  while IFS=: read -r t_role _ _ t_scopes; do
    [ -z "$t_role" ] && continue
    R_ROLE+=("$t_role")
    R_USER+=("$(fleet_username "$PREFIX" "$t_role")")
    R_NAME+=("$(fleet_pat_name "$t_role")")
    R_SCOPES+=("$t_scopes")
    R_FILE+=("$(fleet_token_file "$t_role")")
  done <<EOF
$ROLE_TABLE
EOF
fi

SEEN_RECORDS=" "
for rec in ${ROLE_RECORDS[@]+"${ROLE_RECORDS[@]}"}; do
  case "$rec" in *=*) ;; *) die_usage "malformed --role record (expected ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE]): '${rec}'" ;; esac
  rr_role="${rec%%=*}"
  rr_fields="${rec#*=}"
  IFS=: read -r rr_user rr_name rr_scopes rr_file rr_extra <<EOF
$rr_fields
EOF
  if [ -z "$rr_role" ] || [ -z "${rr_user:-}" ] || [ -z "${rr_name:-}" ] || [ -z "${rr_scopes:-}" ] || [ -n "${rr_extra:-}" ]; then
    die_usage "malformed --role record for '${rr_role:-<unknown>}' (expected ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE])"
  fi
  printf '%s' "$rr_role" | grep -Eq '^[a-z0-9][a-z0-9-]*$' || die_usage "invalid role name in --role: '${rr_role}'"
  printf '%s' "$rr_user" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_.-]*$' || die_usage "invalid USERNAME in --role ${rr_role}"
  printf '%s' "$rr_name" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_.-]*$' || die_usage "invalid TOKEN_NAME in --role ${rr_role}"
  printf '%s' "$rr_scopes" | grep -Eq '^[a-z_]+(,[a-z_]+)*$' || die_usage "invalid SCOPES in --role ${rr_role} (comma-separated scope names)"
  rr_file="${rr_file:-$(fleet_token_file "$rr_role")}"
  printf '%s' "$rr_file" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_.-]*$' \
    || die_usage "FILE in --role ${rr_role} must be a plain base name, not a path: '${rr_file}'"
  case "$SEEN_RECORDS" in *" ${rr_role} "*) die_usage "--role ${rr_role} given more than once" ;; esac
  SEEN_RECORDS="${SEEN_RECORDS}${rr_role} "
  if idx=$(role_index "$rr_role"); then
    R_USER[$idx]="$rr_user"; R_NAME[$idx]="$rr_name"; R_SCOPES[$idx]="$rr_scopes"; R_FILE[$idx]="$rr_file"
  else
    R_ROLE+=("$rr_role"); R_USER+=("$rr_user"); R_NAME+=("$rr_name"); R_SCOPES+=("$rr_scopes"); R_FILE+=("$rr_file")
  fi
done

if [ -n "$ONLY" ]; then
  printf '%s' "$ONLY" | grep -Eq '^[a-z0-9][a-z0-9-]*(,[a-z0-9][a-z0-9-]*)*$' || die_usage "--only must be a comma-separated role list: '${ONLY}'"
  for o in $(printf '%s' "$ONLY" | tr ',' ' '); do
    role_index "$o" >/dev/null || die_usage "--only names '${o}', which is not a configured role"
  done
  K_ROLE=(); K_USER=(); K_NAME=(); K_SCOPES=(); K_FILE=()
  for i in "${!R_ROLE[@]}"; do
    case ",${ONLY}," in
      *",${R_ROLE[$i]},"*)
        K_ROLE+=("${R_ROLE[$i]}"); K_USER+=("${R_USER[$i]}"); K_NAME+=("${R_NAME[$i]}")
        K_SCOPES+=("${R_SCOPES[$i]}"); K_FILE+=("${R_FILE[$i]}") ;;
    esac
  done
  R_ROLE=("${K_ROLE[@]}"); R_USER=("${K_USER[@]}"); R_NAME=("${K_NAME[@]}"); R_SCOPES=("${K_SCOPES[@]}"); R_FILE=("${K_FILE[@]}")
fi

# Two roles may not share a destination file, nor one account's PAT name — a
# renewal of one would silently overwrite or re-rotate the other.
dup_check() {  # dup_check LABEL VALUES...
  local label="$1"; shift
  local d
  d=$(printf '%s\n' "$@" | sort | uniq -d | head -1)
  [ -z "$d" ] || die_usage "two roles resolve to the same ${label}: '${d}'"
}
FILE_KEYS=(); PAT_KEYS=()
for i in "${!R_ROLE[@]}"; do
  FILE_KEYS+=("${R_FILE[$i]}")
  PAT_KEYS+=("${R_USER[$i]}/${R_NAME[$i]}")
done
dup_check "destination file" "${FILE_KEYS[@]}"
dup_check "service account + PAT name" "${PAT_KEYS[@]}"

for cmd in glab jq; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: '$cmd' is required on PATH" >&2; exit 2; }
done

# --- working state + cleanup -------------------------------------------------
# WORK holds only non-secret scratch (API listings, request bodies, glab's
# stderr) and is glab's working directory, so no enclosing git checkout can
# steer glab's host resolution. SECRET_TMPS lists the secret-bearing temp
# files, which live beside their destination; both are removed on every exit.
umask 077
WORK=$(mktemp -d "${TMPDIR:-/tmp}/renew-fleet-gitlab.XXXXXX")
SECRET_TMPS=()
cleanup() {
  local f
  # An array, quoted: an --out-dir holding whitespace must not split a temp
  # path and leave a secret-bearing file behind.
  for f in ${SECRET_TMPS[@]+"${SECRET_TMPS[@]}"}; do rm -f "$f"; done
  rm -rf "$WORK"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# gl OUT ARGS... — run glab with the host pinned (GITLAB_HOST is the first of
# glab's host variables to win) from the neutral work dir; stdout -> OUT,
# stderr -> $WORK/stderr. Returns glab's exit status.
gl() {
  local out="$1"; shift
  ( cd "$WORK" && GITLAB_HOST="$GL_HOSTNAME" GLAB_NO_PROMPT=1 GLAB_CHECK_UPDATE=false glab "$@" ) \
    >"$out" 2>"$WORK/stderr"
}

# relay_stderr — show glab's error text, indented, with anything token-shaped
# redacted. Defence in depth: no call below expects a secret on stderr.
relay_stderr() {
  [ -s "$WORK/stderr" ] || return 0
  sed -E -e 's/glpat-[A-Za-z0-9._-]+/[REDACTED]/g' -e 's/^/    glab: /' "$WORK/stderr" | head -20 >&2
}

urlencode() { jq -rn --arg s "$1" '$s|@uri'; }

utc_date_plus() {  # utc_date_plus DAYS -> YYYY-MM-DD (BSD and GNU date)
  if date -u -v+1d +%Y-%m-%d >/dev/null 2>&1; then
    date -u -v+"$1"d +%Y-%m-%d
  else
    date -u -d "+$1 days" +%Y-%m-%d
  fi
}

# json_field FILE FILTER — jq -r FILTER over FILE, "" on any parse failure.
json_field() { jq -r "$2" "$1" 2>/dev/null || true; }

# listing_array IN OUT — decide a listing from its PARSED values, never its
# byte count (F-empty-listing). IN must hold at least one JSON value and every
# value must be an array (`glab api --paginate` may emit one array per page);
# they are merged into one array in OUT. Returns 2 when IN parses to no value
# at all — zero bytes OR whitespace only, on which a plain `jq FILTER` runs
# nothing and exits 0 — and 1 when it is not JSON or holds a non-array. Either
# is fail-closed for the caller: an unreadable listing is never "no rows".
listing_array() {
  local n
  n=$(jq -s 'length' "$1" 2>/dev/null) || return 1
  [ "$n" != "0" ] || return 2
  jq -s 'if all(.[]; type == "array") then add else error("expected JSON arrays") end' "$1" \
    > "$2" 2>/dev/null || return 1
}

# in_use_recent LAST_USED_AT -> 0 (true) when LAST_USED_AT falls within
# IN_USE_WINDOW_DAYS of now (#1630 required behavior 5). An empty or "null"
# value means the PAT has never been used, so it is never in-use. A value
# present but unparseable fails CLOSED (treated as in-use) rather than
# silently allowing a rotation this check exists to prevent.
in_use_recent() {
  local lu="$1" lu_trim lu_epoch now_epoch window_secs
  case "$lu" in "" | null) return 1 ;; esac
  lu_trim="${lu:0:19}"
  if lu_epoch=$(date -u -d "$lu_trim" +%s 2>/dev/null); then
    :
  elif lu_epoch=$(date -j -u -f '%Y-%m-%dT%H:%M:%S' "$lu_trim" +%s 2>/dev/null); then
    :
  else
    return 0  # unparseable last_used_at: fail closed, treat as in-use
  fi
  now_epoch=$(date -u +%s)
  window_secs=$((IN_USE_WINDOW_DAYS * 86400))
  [ $((now_epoch - lu_epoch)) -lt "$window_secs" ]
}

PREFLIGHT_ERRORS=""
preflight_error() { PREFLIGHT_ERRORS="${PREFLIGHT_ERRORS}${1}"$'\n'; }

echo "Assay fleet PAT renewal — host=${GL_HOSTNAME} authority=group-owner (group=${GROUP}) roles=${#R_ROLE[@]} out-dir=${OUT_DIR} duration=${DAYS}d dry-run=${DRY_RUN}"

# --- output directory + destinations ------------------------------------------
if [ -e "$OUT_DIR" ] && [ ! -d "$OUT_DIR" ]; then
  echo "error: --out-dir ${OUT_DIR} exists and is not a directory" >&2
  exit 1
fi
if [ ! -d "$OUT_DIR" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would create the output directory ${OUT_DIR} (0700)"
  else
    if ! mkdir -p "$OUT_DIR" || ! chmod 700 "$OUT_DIR"; then
      echo "error: cannot create --out-dir ${OUT_DIR}" >&2
      exit 1
    fi
  fi
fi
OUT_PHYS=""
if [ -d "$OUT_DIR" ]; then
  OUT_PHYS=$(cd "$OUT_DIR" && pwd -P)
  if [ "$DRY_RUN" -eq 1 ]; then
    [ -w "$OUT_DIR" ] || preflight_error "output directory ${OUT_DIR} is not writable"
  else
    if probe=$(mktemp "${OUT_DIR}/.renew-write-probe.XXXXXX" 2>/dev/null); then
      rm -f "$probe"
    else
      preflight_error "output directory ${OUT_DIR} is not writable (could not create a temp file in it)"
    fi
  fi
fi

# resolve_write_target PATH — the file a rename must land on: PATH itself, or
# for a symlink the end of its link chain. Echoes an absolute physical path;
# returns 1 on a loop or an unresolvable directory.
resolve_write_target() {
  local p="$1" n=0 l d
  while [ -L "$p" ]; do
    n=$((n + 1)); [ "$n" -le 16 ] || return 1
    l=$(readlink "$p") || return 1
    case "$l" in /*) p="$l" ;; *) p="$(dirname "$p")/$l" ;; esac
  done
  d=$(cd "$(dirname "$p")" 2>/dev/null && pwd -P) || return 1
  printf '%s/%s' "$d" "$(basename "$p")"
}

R_DEST=(); R_WRITE=(); R_LINKED=()
for i in "${!R_ROLE[@]}"; do
  dest="${OUT_DIR}/${R_FILE[$i]}"
  R_DEST+=("$dest"); R_WRITE+=(""); R_LINKED+=(0)
  if [ -L "$dest" ]; then
    R_LINKED[$i]=1
    if ! target=$(resolve_write_target "$dest"); then
      preflight_error "role=${R_ROLE[$i]} path=${dest}: the symlink cannot be resolved (loop or missing directory)"
      continue
    fi
    if [ "$(dirname "$target")" != "$OUT_PHYS" ]; then
      preflight_error "role=${R_ROLE[$i]} path=${dest}: the symlink resolves outside the output directory — refusing to write a credential there"
      continue
    fi
    if [ -e "$target" ] && [ ! -f "$target" ]; then
      preflight_error "role=${R_ROLE[$i]} path=${dest}: the symlink target is not a regular file"
      continue
    fi
    R_WRITE[$i]="$target"
  elif [ -e "$dest" ] && [ ! -f "$dest" ]; then
    preflight_error "role=${R_ROLE[$i]} path=${dest}: exists and is not a regular file"
  else
    R_WRITE[$i]="${OUT_PHYS:-$OUT_DIR}/${R_FILE[$i]}"
  fi
done

# Two roles may also not share a RESOLVED write target: dup_check above only
# compares configured R_FILE base names, but a symlink can make two different
# base names land on the same physical file. Renewing both would silently drop
# the first role's freshly rotated credential when the second role's write
# overwrites it, while both roles report outcome=rotated/created and rc=0.
# Skip entries with no resolved target (already preflight_error'd above) so a
# shared blank never masquerades as a collision.
WRITE_KEYS=()
for i in "${!R_ROLE[@]}"; do
  [ -n "${R_WRITE[$i]}" ] && WRITE_KEYS+=("${R_WRITE[$i]}")
done
if [ "${#WRITE_KEYS[@]}" -gt 0 ]; then
  dup=$(printf '%s\n' "${WRITE_KEYS[@]}" | sort | uniq -d | head -1)
  [ -z "$dup" ] || preflight_error "two roles resolve to the same write target: '${dup}' (a symlink alias of two different destination files) — a renewal of one would silently overwrite or re-rotate the other"
fi

# --- glab authority -------------------------------------------------------------
if ! gl "$WORK/auth" auth status --hostname "$GL_HOSTNAME"; then
  echo "error: glab is not authenticated for ${GL_HOSTNAME} (glab auth status failed) — nothing was rotated" >&2
  exit 1
fi

if ! gl "$WORK/group.json" api --hostname "$GL_HOSTNAME" "groups/$(urlencode "$GROUP")"; then
  echo "error: could not resolve group '${GROUP}' on ${GL_HOSTNAME} — nothing was rotated" >&2
  relay_stderr; exit 1
fi
GROUP_ID=$(json_field "$WORK/group.json" 'if type=="object" and ((.id|tostring)|test("^[0-9]+$")) then (.id|tostring) else "" end')
parent=$(json_field "$WORK/group.json" 'if type=="object" and has("parent_id") then (.parent_id|tostring) else "missing" end')
if [ -z "$GROUP_ID" ] || [ "$parent" = "missing" ]; then
  echo "error: malformed group response for '${GROUP}' (no numeric id / parent_id) — failing closed, nothing was rotated" >&2
  exit 1
fi
if [ "$parent" != "null" ]; then
  echo "error: '${GROUP}' is a subgroup — service accounts belong to a TOP-LEVEL group; pass that group — nothing was rotated" >&2
  exit 1
fi
if ! gl "$WORK/user.json" api --hostname "$GL_HOSTNAME" user; then
  echo "error: could not read the active glab identity (GET user) — nothing was rotated" >&2
  relay_stderr; exit 1
fi
me=$(json_field "$WORK/user.json" 'if type=="object" and ((.id|tostring)|test("^[0-9]+$")) then (.id|tostring) else "" end')
[ -n "$me" ] || { echo "error: malformed GET user response — failing closed, nothing was rotated" >&2; exit 1; }
level=""
if gl "$WORK/member.json" api --hostname "$GL_HOSTNAME" "groups/${GROUP_ID}/members/all/${me}"; then
  level=$(json_field "$WORK/member.json" 'if type=="object" then (.access_level|tostring) else "" end')
fi
if [ "$level" != "50" ]; then
  echo "error: authority refused — the active glab identity is not an Owner (access_level 50) of '${GROUP}' (read: ${level:-not a member}). The renewal requires the group Owner role on the top-level group that owns the service accounts. Nothing was rotated." >&2
  exit 1
fi
echo "authority: group Owner of '${GROUP}' confirmed"

# --- service accounts ------------------------------------------------------------
if ! gl "$WORK/sas.raw" api --hostname "$GL_HOSTNAME" --paginate "groups/${GROUP_ID}/service_accounts"; then
  echo "error: could not list the service accounts of '${GROUP}' — nothing was rotated" >&2
  relay_stderr; exit 1
fi
if ! listing_array "$WORK/sas.raw" "$WORK/sas.json"; then
  echo "error: malformed or empty service-account listing for '${GROUP}' — failing closed, nothing was rotated" >&2
  exit 1
fi
R_UID=()
for i in "${!R_ROLE[@]}"; do
  uid=$(jq -r --arg u "${R_USER[$i]}" '[.[] | select(type=="object" and .username==$u) | (.id|tostring) | select(test("^[0-9]+$"))] | if length==1 then .[0] else "" end' "$WORK/sas.json" 2>/dev/null || true)
  R_UID+=("$uid")
  [ -n "$uid" ] || preflight_error "role=${R_ROLE[$i]}: service account '${R_USER[$i]}' not found in '${GROUP}' (exactly one match required)"
done

# --- each role's active PATs: rotate / create / skip-in-use / refuse -----------
# The listing is decided from its parsed values (listing_array): zero bytes,
# whitespace only, non-JSON or a non-array all fail closed — never read as
# "zero active PATs", which would create a second live credential while the
# existing one stays live and drops out of custody (F-empty-listing). Every
# record must then be well-formed (numeric id, string name, an `active` field,
# a `last_used_at` key — null when never used); anything else fails closed
# rather than being read as "no match" or "never used". A matching record
# must be active and not revoked: `?state=active` filters server-side, and
# the name match re-checks it here.
MATCH_FILTER='
  map(if type=="object" and has("id") and has("name") and has("active") and has("last_used_at")
          and ((.id|tostring)|test("^[0-9]+$")) and (.name|type)=="string"
        then . else error("malformed token record") end)
  | map(select(.name == $n and ((.active|tostring) == "true") and ((.revoked // false)|tostring) != "true"))
  | .[] | [(.id|tostring), ((.last_used_at // "") | tostring)] | @tsv'
R_ACTION=(); R_TID=(); R_LAST_USED=()
for i in "${!R_ROLE[@]}"; do
  R_ACTION+=(""); R_TID+=(""); R_LAST_USED+=("")
  [ -n "${R_UID[$i]}" ] || continue
  raw_list="$WORK/pats-${i}.raw"
  list="$WORK/pats-${i}.json"
  if ! gl "$raw_list" api --hostname "$GL_HOSTNAME" --paginate \
      "groups/${GROUP_ID}/service_accounts/${R_UID[$i]}/personal_access_tokens?state=active"; then
    preflight_error "role=${R_ROLE[$i]}: listing the PATs of '${R_USER[$i]}' failed"
    relay_stderr
    continue
  fi
  lrc=0; listing_array "$raw_list" "$list" || lrc=$?
  if [ "$lrc" -eq 2 ]; then
    preflight_error "role=${R_ROLE[$i]}: empty PAT listing for '${R_USER[$i]}' (no JSON value: zero bytes or whitespace only) — failing closed rather than reading it as no active PAT (which would create a second live credential)"
    continue
  fi
  if [ "$lrc" -ne 0 ] || ! rows=$(jq -r --arg n "${R_NAME[$i]}" "$MATCH_FILTER" "$list" 2>/dev/null); then
    preflight_error "role=${R_ROLE[$i]}: malformed PAT listing for '${R_USER[$i]}' — failing closed"
    continue
  fi
  count=$(printf '%s' "$rows" | grep -c . || true)
  case "$count" in
    0) R_ACTION[$i]="create" ;;
    1)
      R_TID[$i]=$(printf '%s' "$rows" | cut -f1)
      R_LAST_USED[$i]=$(printf '%s' "$rows" | cut -f2)
      if [ "$ROTATE_IN_USE" -ne 1 ] && in_use_recent "${R_LAST_USED[$i]}"; then
        R_ACTION[$i]="skip-in-use"
      else
        R_ACTION[$i]="rotate"
      fi
      ;;
    *) preflight_error "role=${R_ROLE[$i]}: '${R_USER[$i]}' has ${count} active PATs named '${R_NAME[$i]}' — refusing to guess which is live; revoke the extras and re-run" ;;
  esac
done

if [ -n "$PREFLIGHT_ERRORS" ]; then
  echo "PREFLIGHT FAILED — nothing was rotated or created:" >&2
  printf '%s' "$PREFLIGHT_ERRORS" | sed 's/^/  - /' >&2
  exit 1
fi

if [ "$DRY_RUN" -eq 1 ]; then
  for i in "${!R_ROLE[@]}"; do
    if [ "${R_ACTION[$i]}" = "skip-in-use" ]; then
      echo "[dry-run] role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=would-skip-in-use (last_used_at=${R_LAST_USED[$i]}; within the ${IN_USE_WINDOW_DAYS}d in-use window — pass --rotate-in-use to override)"
    else
      echo "[dry-run] role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=would-${R_ACTION[$i]}"
    fi
  done
  echo "dry-run: preflight passed for ${#R_ROLE[@]} role(s); no token was rotated or created, no file was written"
  exit 0
fi

# --- renewal -------------------------------------------------------------------

# secret_file_ok FILE — exactly one line holding one token-shaped value (an
# optional trailing CR/LF). Read by grep/awk from the file, never via argv.
secret_file_ok() {
  [ -s "$1" ] || return 1
  [ "$(tr -d '\r' < "$1" | awk 'END { print NR }')" = "1" ] || return 1
  tr -d '\r' < "$1" | LC_ALL=C grep -Eq '^[A-Za-z0-9._-]{20,}$'
}

EXPIRES_AT=$(utc_date_plus "$DAYS")
FAILED_ROLE=""

# renew_one I — rotate or create role I's PAT and replace its custody file.
# Prints the role's report line; returns 1 on any failure.
renew_one() {
  local i="$1" role="${R_ROLE[$1]}" dest="${R_DEST[$1]}" write="${R_WRITE[$1]}"
  local dir raw cand final body base rc=0
  dir=$(dirname "$write")
  raw=$(mktemp "${dir}/.renew-${role}.out.XXXXXX")
  cand=$(mktemp "${dir}/.renew-${role}.cand.XXXXXX")
  final=$(mktemp "${dir}/.renew-${role}.new.XXXXXX")
  SECRET_TMPS+=("$raw" "$cand" "$final")
  chmod 600 "$raw" "$cand" "$final"

  body="$WORK/body-${i}.json"
  base="groups/${GROUP_ID}/service_accounts/${R_UID[$i]}/personal_access_tokens"
  if [ "${R_ACTION[$i]}" = "rotate" ]; then
    jq -n --arg e "$EXPIRES_AT" '{expires_at: $e}' > "$body"
    gl "$raw" api --hostname "$GL_HOSTNAME" -X POST -H "Content-Type: application/json" \
      --input "$body" "${base}/${R_TID[$i]}/rotate" || rc=$?
  else
    jq -n --arg n "${R_NAME[$i]}" --arg e "$EXPIRES_AT" --arg s "${R_SCOPES[$i]}" \
      '{name: $n, scopes: ($s | split(",")), expires_at: $e}' > "$body"
    gl "$raw" api --hostname "$GL_HOSTNAME" -X POST -H "Content-Type: application/json" \
      --input "$body" "$base" || rc=$?
  fi
  if [ "$rc" -eq 0 ]; then
    # The response is JSON; its .token goes file -> file, never through argv.
    jq -j --arg n "${R_NAME[$i]}" \
      'if type=="object" and (.token|type)=="string" and .name==$n then .token else error("malformed") end' \
      < "$raw" > "$cand" 2>/dev/null || rc=97
  fi
  rm -f "$raw"

  if [ "$rc" -ne 0 ] && [ "$rc" -ne 97 ]; then
    rm -f "$cand" "$final"
    echo "role=${role} path=${dest} outcome=failed (glab exited ${rc} during ${R_ACTION[$i]}; if the forge accepted it, the previous credential is already invalid)"
    relay_stderr
    return 1
  fi
  if [ "$rc" -eq 97 ] || ! secret_file_ok "$cand"; then
    rm -f "$cand" "$final"
    echo "role=${role} path=${dest} outcome=failed (malformed ${R_ACTION[$i]} output — failing closed, nothing written; the previous credential may already be invalid)"
    return 1
  fi
  tr -d '\r\n' < "$cand" > "$final"
  rm -f "$cand"
  chmod 600 "$final"
  if ! mv -f "$final" "$write"; then
    rm -f "$final"
    echo "role=${role} path=${dest} outcome=failed (could not replace the custody file; the new PAT was discarded, the previous one is already invalid)"
    return 1
  fi
  sync 2>/dev/null || true
  # Read back THROUGH the custody path: the bytes landed, the layout survived.
  if ! secret_file_ok "$dest" || [ -z "$(find "$write" -prune -perm 600 2>/dev/null)" ] \
     || { [ "${R_LINKED[$i]}" = "1" ] && [ ! -L "$dest" ]; }; then
    echo "role=${role} path=${dest} outcome=failed (read-back of the custody file did not verify)"
    return 1
  fi
  if [ "${R_ACTION[$i]}" = "rotate" ]; then
    echo "role=${role} path=${dest} outcome=rotated"
  else
    echo "role=${role} path=${dest} outcome=created"
  fi
}

# RENEWED counts only roles whose PAT was actually rotated or created. A role
# skipped as in-use is counted apart (F4): it was NOT renewed, its PAT keeps
# its previous expiry, and a run that skipped any role never exits 0 — a
# fleet where one role is left behind while the rest look current is the
# failure #1630 was filed from.
RENEWED=0
SKIPPED=0
SKIPPED_LIST=""
REMAINING=""
for i in "${!R_ROLE[@]}"; do
  if [ -n "$FAILED_ROLE" ]; then
    echo "role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=not-attempted"
    REMAINING="${REMAINING:+${REMAINING},}${R_ROLE[$i]}"
    continue
  fi
  if [ "${R_ACTION[$i]}" = "skip-in-use" ]; then
    echo "role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=skipped-in-use (last_used_at=${R_LAST_USED[$i]}; within the ${IN_USE_WINDOW_DAYS}d in-use window — NOT renewed, the PAT keeps its previous expiry; re-run with --rotate-in-use to override)"
    SKIPPED=$((SKIPPED + 1))
    SKIPPED_LIST="${SKIPPED_LIST:+${SKIPPED_LIST},}${R_ROLE[$i]}"
    continue
  fi
  if renew_one "$i"; then
    RENEWED=$((RENEWED + 1))
  else
    FAILED_ROLE="${R_ROLE[$i]}"
    REMAINING="${FAILED_ROLE}"
  fi
done

N_ROLES=${#R_ROLE[@]}
SKIP_NOTE=""
[ "$SKIPPED" -eq 0 ] || SKIP_NOTE=", ${SKIPPED} skipped-in-use (${SKIPPED_LIST})"
if [ -n "$FAILED_ROLE" ]; then
  echo "PARTIAL RUN — ${RENEWED} of ${N_ROLES} role(s) renewed${SKIP_NOTE}; stopped at role=${FAILED_ROLE}." >&2
  echo "Resume: fix the cause, then re-run the same command with --only ${REMAINING}" >&2
  exit 1
fi
if [ "$RENEWED" -gt 0 ]; then
  echo "renewed: ${RENEWED} of ${N_ROLES} role(s); each renewed PAT expires ${EXPIRES_AT} — file paths printed, values never echoed"
else
  echo "renewed: 0 of ${N_ROLES} role(s) — no PAT was rotated or created"
fi
if [ "$SKIPPED" -gt 0 ]; then
  echo "skipped-in-use: ${SKIPPED} of ${N_ROLES} role(s) (${SKIPPED_LIST}) — NOT renewed; their PATs keep their previous expiry." >&2
  echo "Once nothing holds them (stop those desk sessions), re-run with --only ${SKIPPED_LIST} --rotate-in-use" >&2
  exit 3
fi
