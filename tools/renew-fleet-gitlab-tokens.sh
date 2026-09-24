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
# TWO AUTHORITY MODELS, chosen explicitly — never inferred, never a fallback:
#
#   --group <top-level-group>   GROUP OWNER. Uses `glab api` against the group
#                               service-account PAT endpoints, which admit a
#                               group Owner:
#     GET  groups/:id                                                    (resolve, top-level check)
#     GET  user                                                          (the active identity)
#     GET  groups/:id/members/all/:user_id                               (Owner = access_level 50)
#     GET  groups/:id/service_accounts                                   (username -> id)
#     GET  groups/:id/service_accounts/:user_id/personal_access_tokens?state=active
#     POST groups/:id/service_accounts/:user_id/personal_access_tokens/:token_id/rotate
#     POST groups/:id/service_accounts/:user_id/personal_access_tokens
#
#   --instance-admin            INSTANCE ADMINISTRATOR. Uses `glab token
#                               list|create|rotate --user <service-account>`,
#                               which glab documents as administrator-only for
#                               another user's tokens:
#     GET  application/settings  (glab api; admin-only — the authority probe)
#     GET  users?username=<u>    (glab api; account resolution)
#     glab token list   --user <u> --active --output json
#     glab token rotate <token-id> --user <u> --duration <N>d --output text
#     glab token create <name> --user <u> --duration <N>d --scope <s>... --output text
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
INSTANCE_ADMIN=0
PREFIX=""
OUT_DIR=""
DURATION="30d"
DRY_RUN=0
ONLY=""
ROLE_RECORDS=()

usage() {
  cat <<'USAGE'
Usage: renew-fleet-gitlab-tokens.sh --hostname <host>
         (--group <top-level-group> | --instance-admin)
         (--prefix <prefix> | --role ROLE=USERNAME:TOKEN_NAME:SCOPES[:FILE] ...)
         --out-dir <dir> [--duration 30d] [--only ROLE[,ROLE...]] [--dry-run]

Renews every configured Assay fleet role PAT in one run: rotates each role's
active PAT (matched by name), creates one only when none exists, and replaces
<out-dir>/gitlab-<role>.token atomically.

Required:
  --hostname <host>       GitLab hostname (no scheme), e.g. gitlab.example.com.
  --group <group>         GROUP-OWNER authority: the top-level group owning the
                           fleet's service accounts. Uses the group
                           service-account PAT endpoints via `glab api`.
  --instance-admin        INSTANCE-ADMIN authority: uses
                           `glab token list|create|rotate --user`. Exactly one
                           of --group / --instance-admin.
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
  --duration <N>d|<N>w    PAT lifetime from today, 1d..365d. Default: 30d.
  --only ROLE[,ROLE...]   Renew only these configured roles (resume a partial
                           run with the list the failed run printed).
  --dry-run               Run every preflight read and print the plan
                           (rotate / create per role). No token is rotated or
                           created and no file is written.
  -h, --help              This text.

Auth: whatever identity `glab` is logged in as for --hostname (glab's own
credential store or its environment). This script never reads, stores or
passes that credential.

A rotation invalidates the role's previous PAT immediately. Each role's file
swap is atomic; the fleet-wide operation is NOT — stop the fleet's desk
sessions first, and resume a partial run with --only.

Exit status: 0 = every selected role renewed (or, with --dry-run, planned);
1 = a preflight refusal (nothing mutated) or a role failed part-way;
2 = usage error.
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
    --instance-admin) INSTANCE_ADMIN=1; shift ;;
    --prefix) need_val "$1" $#; PREFIX="$2"; shift 2 ;;
    --role) need_val "$1" $#; ROLE_RECORDS+=("$2"); shift 2 ;;
    --out-dir) need_val "$1" $#; OUT_DIR="$2"; shift 2 ;;
    --duration) need_val "$1" $#; DURATION="$2"; shift 2 ;;
    --only) need_val "$1" $#; ONLY="$2"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die_usage "unknown argument: $1" ;;
  esac
done

# --- argument preflight ------------------------------------------------------
[ -n "$GL_HOSTNAME" ] || die_usage "--hostname is required"
printf '%s' "$GL_HOSTNAME" | grep -Eq '^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]+)?$' \
  || die_usage "--hostname must be a bare host[:port] (no scheme, no path): '${GL_HOSTNAME}'"

if [ -n "$GROUP" ] && [ "$INSTANCE_ADMIN" -eq 1 ]; then
  die_usage "--group and --instance-admin are two different authority models — pass exactly one"
fi
if [ -z "$GROUP" ] && [ "$INSTANCE_ADMIN" -eq 0 ]; then
  die_usage "an authority model is required: --group <top-level-group> (group Owner) or --instance-admin"
fi
if [ -n "$GROUP" ]; then
  printf '%s' "$GROUP" | grep -Eq '^[A-Za-z0-9_][A-Za-z0-9_.-]*$' \
    || die_usage "--group must be a top-level group path or numeric id (no '/'): '${GROUP}'"
fi

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
SECRET_TMPS=""
cleanup() {
  local f
  for f in $SECRET_TMPS; do rm -f "$f"; done
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

PREFLIGHT_ERRORS=""
preflight_error() { PREFLIGHT_ERRORS="${PREFLIGHT_ERRORS}${1}"$'\n'; }

AUTH_LABEL="instance-admin"
[ -n "$GROUP" ] && AUTH_LABEL="group-owner (group=${GROUP})"
echo "Assay fleet PAT renewal — host=${GL_HOSTNAME} authority=${AUTH_LABEL} roles=${#R_ROLE[@]} out-dir=${OUT_DIR} duration=${DAYS}d dry-run=${DRY_RUN}"

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

GROUP_ID=""
if [ -n "$GROUP" ]; then
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
    echo "error: authority refused — the active glab identity is not an Owner (access_level 50) of '${GROUP}' (read: ${level:-not a member}). Group-owner mode requires the Owner role; an instance administrator uses --instance-admin instead. Nothing was rotated." >&2
    exit 1
  fi
  echo "authority: group Owner of '${GROUP}' confirmed"
else
  if ! gl "$WORK/settings.json" api --hostname "$GL_HOSTNAME" application/settings \
     || [ "$(json_field "$WORK/settings.json" 'type')" != "object" ]; then
    echo "error: authority refused — the active glab identity cannot read the admin-only application/settings endpoint on ${GL_HOSTNAME}. Instance-admin mode requires an instance administrator (glab token --user on another account is admin-only); a group Owner uses --group <top-level-group> instead. Nothing was rotated." >&2
    exit 1
  fi
  echo "authority: instance administrator confirmed"
fi

# --- service accounts ------------------------------------------------------------
R_UID=()
if [ -n "$GROUP" ]; then
  if ! gl "$WORK/sas.json" api --hostname "$GL_HOSTNAME" --paginate "groups/${GROUP_ID}/service_accounts"; then
    echo "error: could not list the service accounts of '${GROUP}' — nothing was rotated" >&2
    relay_stderr; exit 1
  fi
  if [ "$(json_field "$WORK/sas.json" 'type')" != "array" ]; then
    echo "error: malformed service-account listing for '${GROUP}' — failing closed, nothing was rotated" >&2
    exit 1
  fi
fi
for i in "${!R_ROLE[@]}"; do
  uid=""
  if [ -n "$GROUP" ]; then
    uid=$(jq -r --arg u "${R_USER[$i]}" '[.[] | select(type=="object" and .username==$u) | (.id|tostring) | select(test("^[0-9]+$"))] | if length==1 then .[0] else "" end' "$WORK/sas.json" 2>/dev/null || true)
  else
    if gl "$WORK/users.json" api --hostname "$GL_HOSTNAME" "users?username=$(urlencode "${R_USER[$i]}")"; then
      uid=$(jq -r --arg u "${R_USER[$i]}" 'if type=="array" then ([.[] | select(type=="object" and .username==$u) | (.id|tostring) | select(test("^[0-9]+$"))] | if length==1 then .[0] else "" end) else "" end' "$WORK/users.json" 2>/dev/null || true)
    fi
  fi
  R_UID+=("$uid")
  [ -n "$uid" ] || preflight_error "role=${R_ROLE[$i]}: service account '${R_USER[$i]}' not found (exactly one match required)"
done

# --- each role's active PATs: rotate / create / refuse -------------------------
# Every record must be well-formed (numeric id, string name, an `active`
# field); anything else fails closed rather than being read as "no match",
# which would create a second live credential.
MATCH_FILTER='
  if type != "array" then error("expected a JSON array") else . end
  | map(if type=="object" and has("id") and has("name") and has("active")
          and ((.id|tostring)|test("^[0-9]+$")) and (.name|type)=="string"
        then . else error("malformed token record") end)
  | map(select(.name == $n and ((.active|tostring) == "true") and ((.revoked // false)|tostring) != "true"))
  | .[] | (.id|tostring)'
R_ACTION=(); R_TID=()
for i in "${!R_ROLE[@]}"; do
  R_ACTION+=(""); R_TID+=("")
  [ -n "${R_UID[$i]}" ] || continue
  list="$WORK/pats-${i}.json"
  if [ -n "$GROUP" ]; then
    ok=0; gl "$list" api --hostname "$GL_HOSTNAME" --paginate \
      "groups/${GROUP_ID}/service_accounts/${R_UID[$i]}/personal_access_tokens?state=active" && ok=1
  else
    ok=0; gl "$list" token list --user "${R_USER[$i]}" --active --output json && ok=1
  fi
  if [ "$ok" -ne 1 ]; then
    preflight_error "role=${R_ROLE[$i]}: listing the PATs of '${R_USER[$i]}' failed"
    relay_stderr
    continue
  fi
  if ! ids=$(jq -r --arg n "${R_NAME[$i]}" "$MATCH_FILTER" "$list" 2>/dev/null); then
    preflight_error "role=${R_ROLE[$i]}: malformed PAT listing for '${R_USER[$i]}' — failing closed"
    continue
  fi
  count=$(printf '%s' "$ids" | grep -c . || true)
  case "$count" in
    0) R_ACTION[$i]="create" ;;
    1) R_ACTION[$i]="rotate"; R_TID[$i]="$ids" ;;
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
    echo "[dry-run] role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=would-${R_ACTION[$i]}"
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
  local dir raw cand final body rc=0 s
  dir=$(dirname "$write")
  raw=$(mktemp "${dir}/.renew-${role}.out.XXXXXX")
  cand=$(mktemp "${dir}/.renew-${role}.cand.XXXXXX")
  final=$(mktemp "${dir}/.renew-${role}.new.XXXXXX")
  SECRET_TMPS="$SECRET_TMPS $raw $cand $final"
  chmod 600 "$raw" "$cand" "$final"

  if [ -n "$GROUP" ]; then
    body="$WORK/body-${i}.json"
    local base="groups/${GROUP_ID}/service_accounts/${R_UID[$i]}/personal_access_tokens"
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
  else
    if [ "${R_ACTION[$i]}" = "rotate" ]; then
      gl "$raw" token rotate "${R_TID[$i]}" --user "${R_USER[$i]}" \
        --duration "${DAYS}d" --output text || rc=$?
    else
      local args=(token create "${R_NAME[$i]}" --user "${R_USER[$i]}" --duration "${DAYS}d")
      local scope_list
      scope_list=$(printf '%s' "${R_SCOPES[$i]}" | tr ',' ' ')
      for s in $scope_list; do args+=(--scope "$s"); done
      args+=(--output text)
      gl "$raw" "${args[@]}" || rc=$?
    fi
    [ "$rc" -ne 0 ] || cp "$raw" "$cand"
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

DONE=0
REMAINING=""
for i in "${!R_ROLE[@]}"; do
  if [ -n "$FAILED_ROLE" ]; then
    echo "role=${R_ROLE[$i]} path=${R_DEST[$i]} outcome=not-attempted"
    REMAINING="${REMAINING:+${REMAINING},}${R_ROLE[$i]}"
    continue
  fi
  if renew_one "$i"; then
    DONE=$((DONE + 1))
  else
    FAILED_ROLE="${R_ROLE[$i]}"
    REMAINING="${FAILED_ROLE}"
  fi
done

if [ -n "$FAILED_ROLE" ]; then
  echo "PARTIAL RUN — ${DONE} of ${#R_ROLE[@]} role(s) renewed; stopped at role=${FAILED_ROLE}." >&2
  echo "Resume: fix the cause, then re-run the same command with --only ${REMAINING}" >&2
  exit 1
fi
echo "renewed: ${DONE} of ${#R_ROLE[@]} role(s); each PAT expires ${EXPIRES_AT} — file paths printed, values never echoed"
