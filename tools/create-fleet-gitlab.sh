#!/usr/bin/env bash
# tools/create-fleet-gitlab.sh — idempotent Assay fleet provisioning for a
# GitLab Premium/Ultimate top-level group.
#
# Creates the seven per-role service accounts, their group memberships and
# personal access tokens, has each new account set its own avatar, then (when
# --project is given) configures the fleet project's protected `main` branch,
# MR-approval settings, protected release tags, and the pipeline /
# all-discussions-resolved merge gates. Prints a plain-text summary ending with the
# HUMAN-ONLY remainder this script never attempts: Ultimate-tier settings, the
# group token-expiry policy, and creation of the locked ci-config project.
#
# Tier-safety (issue #346, measured on a free-tier gitlab.com group
# 2026-09-02): the protected-branch step branches on the group's tier and
# NEVER leaves `main` unprotected across a failure — it prefers a no-op or a
# PATCH, and where a DELETE+POST is unavoidable it re-applies the rule it read
# if the POST is refused. Every settings step runs even when an earlier one
# fails; failures are collected and reported at the end, and the script then
# exits non-zero.
#
# Avatars (issue #346): a group Owner cannot set a service account's avatar
# (`PUT /users/:id` -> 403 on gitlab.com), so each account sets its own with
# its freshly minted PAT, at mint time — the one moment this script holds that
# credential. See --avatars-dir / --no-avatars / --avatars-only.
#
# Design reference: docs/streams/forge-gitlab/spec.md
#   §2 (identity model)  — the role -> access-level -> scope table below.
#   §4 (ci-config project) — why ci-config creation stays human-only here.
#   §5 (token custody)    — rotate-on-mint + 0600 file custody, path-only
#                            printing, never in env or argv.
# Adopter walkthrough: docs/adopting-assay-gitlab.md
#
# REST endpoints used (GitLab API v4 — dereferenced against docs.gitlab.com
# by the reviewer per this brief's Verify table; listed once, here, so the
# check has one place to look):
#   GET    api/v4/groups/:id                       (id resolve + `plan` tier probe)
#   GET    api/v4/groups/:id/service_accounts
#   POST   api/v4/groups/:id/service_accounts
#   POST   api/v4/groups/:id/service_accounts/:user_id/personal_access_tokens
#   GET    api/v4/groups/:id/members/:user_id
#   POST   api/v4/groups/:id/members
#   PUT    api/v4/user/avatar                       (as the ROLE's own PAT)
#   GET    api/v4/projects/:id                      (id resolve)
#   GET    api/v4/projects/:id/protected_branches/:name
#   PATCH  api/v4/projects/:id/protected_branches/:name
#   DELETE api/v4/projects/:id/protected_branches/:name
#   POST   api/v4/projects/:id/protected_branches
#   GET    api/v4/projects/:id/approvals            (read-back, tier check)
#   POST   api/v4/projects/:id/approvals
#   GET    api/v4/projects/:id/protected_tags       (idempotency check)
#   POST   api/v4/projects/:id/protected_tags       (create_access_level SCALAR — Free)
#   PUT    api/v4/projects/:id                       (pipeline + all-discussions-resolved merge checks)
#   GET    api/v4/projects/:id                       (merge-checks read-back)
#
# One non-API URL is fetched, and only when avatars are left at their default:
#   GET    https://assay.guide/assets/app-icon-<role>.png   (public role icons)
#
# Every endpoint above is reachable at the Premium tier, and every one except
# the `allowed_to_*` arrays of POST protected_branches is reachable at Free —
# which is what the tier branching below is for. Nothing this script calls
# requires Ultimate — the Ultimate-only controls (custom roles, external
# status checks, pipeline execution policy) are named in the human-only
# checklist this script prints, never called.
#
# NEVER hits live infrastructure unless invoked without --dry-run and with
# GITLAB_TOKEN set; --dry-run makes zero network calls.

set -euo pipefail

# ---------------------------------------------------------------------------
# Role table (spec.md §2) — one line per Assay role this script provisions.
# `promote` is deliberately absent: per spec §2 it usually has no GitLab
# identity at all, so there is nothing here to create for it.
#
# role:access_level_name:access_level_num:csv_scopes
# ---------------------------------------------------------------------------
ROLE_TABLE='
reviewer:developer:30:api
worker:developer:30:api,write_repository
verifier:developer:30:api,write_repository
desk:developer:30:api
issue-loop:reporter:20:api
intake-loop:reporter:20:api
board-writer:developer:30:api,write_repository
'

GITLAB_URL="https://gitlab.com"
PAT_EXPIRY_DAYS=7
OUT_DIR=""
GROUP=""
PROJECT=""
PREFIX=""
DRY_RUN=0
AVATARS_DIR=""
AVATARS=1
AVATARS_ONLY=0
AVATAR_ICON_BASE="https://assay.guide/assets"
# Intended merge access level for protected `main`: 40 = Maintainers. Named
# here because a hand repair that used 30 (Developers) let every Developer
# service account merge its own MR — issue #346's comment 1.
MERGE_ACCESS_LEVEL=40
# Protected release tags (issue #346 comment 1 §4 / pilot D-4). The glob covers
# every tag and create_access_level is the SCALAR Free-tier field (40 =
# Maintainers) — NEVER the Premium `allowed_to_create` array, which is exactly
# the array shape that produced this issue's 400. With this rule only a human
# owner can create or move a tag, so a release tag is immutable to every bot.
PROTECTED_TAG_GLOB='*'
PROTECTED_TAG_CREATE_LEVEL=40

usage() {
  cat <<'USAGE'
Usage: create-fleet-gitlab.sh --group <group> --prefix <prefix> [options]

Idempotently provisions the Assay fleet's seven per-role GitLab service
accounts (spec.md §2), then — when --project is given — the fleet project's
protected `main` branch and MR-approval settings (spec.md §3).

Required:
  --group <path-or-id>    Top-level GitLab group that owns the fleet's
                           service accounts (service accounts are a
                           top-level-group-only feature).
  --prefix <name>         Prefix for generated usernames, e.g. "myorg" ->
                           myorg-reviewer-bot, myorg-worker-bot, ...

Options:
  --project <path-or-id>  Fleet project to configure protected-branch and
                           approval settings on. Omit to provision accounts
                           only (project-level steps are then skipped with a
                           NOTICE, never silently).
  --gitlab-url <url>      Default: https://gitlab.com (self-managed EE:
                           pass your instance's base URL).
  --pat-expiry-days <n>   Default: 7 (spec.md §5's RECOMMENDED backstop).
  --out-dir <dir>         Where minted token files are written, 0600 each.
                           Default: a fresh mktemp -d under $TMPDIR.
  --avatars-dir <dir>     Upload <dir>/<role>.png as each new account's own
                           avatar. Default (no flag): fetch the public role
                           icons from
                           https://assay.guide/assets/app-icon-<role>.png
                           into a temp dir. A missing/unfetchable icon is a
                           NOTICE, never a failure.
  --no-avatars            Skip the avatar step entirely (prints a NOTICE).
  --avatars-only          Re-upload avatars for accounts that ALREADY exist,
                           using the token files already under --out-dir.
                           Mints nothing, creates nothing, and touches no
                           project settings. Requires --prefix and --out-dir.
  --dry-run               Enumerate every action; make zero network calls.
  -h, --help              This text.

Auth: a group-owner personal access token, supplied ONLY via the
GITLAB_TOKEN environment variable (never a flag, never argv) — required
unless --dry-run or --avatars-only. This script never stores it.

Avatars are set by each account ON ITSELF (PUT /user/avatar with that role's
own PAT): a group Owner cannot set a service account's avatar — PUT /users/:id
is admin-only and answers 403 on gitlab.com. The role token is read from its
0600 file into a curl config file, so it never appears in argv.

Exit status: 0 when every step succeeded; non-zero when any step failed. A
failed settings step no longer aborts the ones after it — every step runs and
the failures are listed in the summary.
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --group) GROUP="$2"; shift 2 ;;
    --project) PROJECT="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --gitlab-url) GITLAB_URL="$2"; shift 2 ;;
    --pat-expiry-days) PAT_EXPIRY_DAYS="$2"; shift 2 ;;
    --out-dir) OUT_DIR="$2"; shift 2 ;;
    --avatars-dir) AVATARS_DIR="$2"; AVATARS=1; shift 2 ;;
    --no-avatars) AVATARS=0; shift ;;
    --avatars-only) AVATARS_ONLY=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$AVATARS_ONLY" -eq 1 ]; then
  if [ "$AVATARS" -eq 0 ]; then
    echo "error: --avatars-only and --no-avatars are contradictory" >&2
    exit 2
  fi
  if [ -z "$PREFIX" ] || [ -z "$OUT_DIR" ]; then
    echo "error: --avatars-only requires --prefix and --out-dir (the token files it re-uses)" >&2
    usage >&2
    exit 2
  fi
elif [ -z "$GROUP" ] || [ -z "$PREFIX" ]; then
  echo "error: --group and --prefix are required" >&2
  usage >&2
  exit 2
fi

for cmd in curl jq; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: '$cmd' is required on PATH" >&2; exit 2; }
done

# --avatars-only authenticates as each ROLE (its own token file), so it needs
# no owner PAT; every other mode does.
if [ "$DRY_RUN" -eq 0 ] && [ "$AVATARS_ONLY" -eq 0 ]; then
  if [ -z "${GITLAB_TOKEN:-}" ]; then
    echo "error: GITLAB_TOKEN must be set (a group-owner PAT) unless --dry-run or --avatars-only" >&2
    exit 2
  fi
fi

if [ -z "$OUT_DIR" ]; then
  OUT_DIR=$(mktemp -d "${TMPDIR:-/tmp}/fleet-gitlab-tokens.XXXXXX")
fi
if [ "$DRY_RUN" -eq 0 ]; then
  mkdir -p "$OUT_DIR"
fi

# --- failure ledger -------------------------------------------------------
# A settings step that fails must not abort the steps after it (issue #346:
# the protect step's `exit 1` also skipped the pipeline gate below it). Each
# failure is appended here, printed under the HUMAN-ONLY REMAINDER, and turns
# the final exit status non-zero. It is a FILE, not a variable, because the
# role loop below runs in a `while` subshell.
FAILURES_FILE=$(mktemp "${TMPDIR:-/tmp}/fleet-gitlab-failures.XXXXXX")
record_failure() { printf '%s\n' "$1" >> "$FAILURES_FILE"; }
trap 'rm -f "$FAILURES_FILE"' EXIT

# expires_at: portable across BSD date (macOS) and GNU date (Linux CI).
pat_expiry_date() {
  if date -v+1d +%Y-%m-%d >/dev/null 2>&1; then
    date -v+"${PAT_EXPIRY_DAYS}"d +%Y-%m-%d
  else
    date -d "+${PAT_EXPIRY_DAYS} days" +%Y-%m-%d
  fi
}

urlencode() {
  jq -rn --arg s "$1" '$s|@uri'
}

# gl_api METHOD PATH [JSON_BODY] — sets GL_LAST_STATUS and GL_LAST_BODY_FILE.
# Caller is responsible for `rm -f "$GL_LAST_BODY_FILE"` when done reading it.
gl_api() {
  local method="$1" path="$2" body="${3:-}"
  local tmp
  tmp=$(mktemp "${TMPDIR:-/tmp}/gl-api-body.XXXXXX")
  if [ -n "$body" ]; then
    GL_LAST_STATUS=$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" \
      -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" -H "Content-Type: application/json" \
      -d "$body" "${GITLAB_URL}/api/v4${path}")
  else
    GL_LAST_STATUS=$(curl -sS -o "$tmp" -w '%{http_code}' -X "$method" \
      -H "PRIVATE-TOKEN: ${GITLAB_TOKEN}" "${GITLAB_URL}/api/v4${path}")
  fi
  GL_LAST_BODY_FILE="$tmp"
}

# ---------------------------------------------------------------------------
# Avatars (issue #346). The owner cannot set a service account's avatar
# (`PUT /users/:id` is admin-only -> 403 on gitlab.com) and the service-account
# PATCH endpoint has no avatar field, so each account sets its OWN via
# `PUT /user/avatar` with its own PAT. That is why this happens at mint time:
# it is the one moment the script holds that credential.
# ---------------------------------------------------------------------------
AVATAR_TMPDIR=""

# avatar_source ROLE — sets AVATAR_FILE to the local PNG for ROLE, or to the
# empty string after printing a NOTICE when there is no icon to upload. A
# missing icon is never a failure.
avatar_source() {
  local role="$1" file
  AVATAR_FILE=""

  if [ -n "$AVATARS_DIR" ]; then
    file="${AVATARS_DIR}/${role}.png"
    if [ ! -f "$file" ]; then
      echo "NOTICE: no avatar for ${role} — ${file} does not exist (skipped, not a failure)"
      return 0
    fi
    AVATAR_FILE="$file"
    return 0
  fi

  if [ -z "$AVATAR_TMPDIR" ]; then
    AVATAR_TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/fleet-gitlab-avatars.XXXXXX")
  fi
  file="${AVATAR_TMPDIR}/${role}.png"
  if [ ! -s "$file" ]; then
    if ! curl -fsSL -o "$file" "${AVATAR_ICON_BASE}/app-icon-${role}.png" || [ ! -s "$file" ]; then
      rm -f "$file"
      echo "NOTICE: could not fetch the default icon ${AVATAR_ICON_BASE}/app-icon-${role}.png for ${role} (skipped, not a failure)"
      return 0
    fi
  fi
  AVATAR_FILE="$file"
}

# upload_avatar ROLE USERNAME TOKEN_FILE — PUT /user/avatar as the role itself.
# The token is passed through a 0600 curl config file (`curl -K`), never argv.
upload_avatar() {
  local role="$1" username="$2" token_file="$3"
  local file cfg body status

  if [ ! -s "$token_file" ]; then
    echo "NOTICE: avatar for ${username} skipped — no token file at ${token_file} (skipped, not a failure)"
    return 0
  fi

  avatar_source "$role"
  file="$AVATAR_FILE"
  [ -n "$file" ] || return 0

  cfg=$(mktemp "${TMPDIR:-/tmp}/gl-curlrc.XXXXXX")
  body=$(mktemp "${TMPDIR:-/tmp}/gl-avatar-body.XXXXXX")
  ( umask 077; printf 'header = "PRIVATE-TOKEN: %s"\n' "$(cat "$token_file")" > "$cfg" )
  status=$(curl -sS -K "$cfg" -o "$body" -w '%{http_code}' -X PUT \
    -F "avatar=@${file}" "${GITLAB_URL}/api/v4/user/avatar" || echo "000")
  rm -f "$cfg" "$body"

  echo "avatar: ${username} <- ${file} (HTTP ${status})"
  if [ "$status" != "200" ]; then
    record_failure "avatar upload for ${username} returned HTTP ${status} (expected 200) — re-run with --avatars-only once the cause is fixed"
  fi
}

echo "Assay fleet provisioning — group=${GROUP} prefix=${PREFIX} project=${PROJECT:-<none>} dry-run=${DRY_RUN} avatars-only=${AVATARS_ONLY}"

# --- summary + exit ---------------------------------------------------------
# Printed by every exit path that reaches the end of a mode, so a failed step
# is always reported rather than swallowed by the step after it.
print_summary_and_exit() {
  cat <<CHECKLIST

============================================================
HUMAN-ONLY REMAINDER — this script does not and cannot do these:
============================================================
1. Ultimate-tier settings (require an Ultimate license):
   - Custom role for the reviewer service account (Developer without push).
   - External status checks wired to CI verdicts.
   - Pipeline execution policy enforcing the ci-config project's pipeline.
2. Group token-expiry policy: set the group/instance PAT max lifetime to
   ${PAT_EXPIRY_DAYS} days or less (Settings > General > Permissions), the
   backstop behind rotate-on-mint (spec.md §5).
3. Create the locked ci-config project: a Maintainer-humans-only project,
   protected main, approval rules on, NO bot membership. Point each fleet
   project's CI/CD configuration file at
   "<path>/.gitlab-ci.yml@<group>/<ci-config-project>" (Settings > CI/CD >
   General pipelines > CI/CD configuration file). See
   docs/adopting-assay-gitlab.md's ci-config runbook section.
CHECKLIST

  if [ -s "$FAILURES_FILE" ]; then
    echo "------------------------------------------------------------"
    echo "FAILED STEPS — this run did NOT complete cleanly:"
    while IFS= read -r line; do
      echo "   - ${line}"
    done < "$FAILURES_FILE"
    echo "============================================================"
    exit 1
  fi
  echo "============================================================"
  exit 0
}

# --- tier detection (issue #346) -------------------------------------------
# GROUP_TIER is one of: premium (allowed_to_* arrays usable), free (scalar
# access-level fields only), unknown (probe by trying the Premium form first
# and falling back on a 400 that names allowed_to_).
GROUP_TIER="unknown"
classify_plan() {
  case "$1" in
    premium|ultimate|gold|silver) echo "premium" ;;
    free|default|bronze|"")       echo "free" ;;
    *)                            echo "unknown" ;;
  esac
}

if [ "$AVATARS_ONLY" -eq 1 ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "$ROLE_TABLE" | while IFS=: read -r role _ _ _; do
      [ -z "$role" ] && continue
      echo "[dry-run] would upload avatar for ${PREFIX}-${role}-bot via PUT /user/avatar, as that role's own PAT from ${OUT_DIR}/${PREFIX}-${role}-bot.token"
    done
    print_summary_and_exit
  fi
  echo "avatars-only: re-uploading avatars from the token files under ${OUT_DIR} — no accounts, tokens or project settings are touched"
  while IFS=: read -r role _ _ _; do
    [ -z "$role" ] && continue
    username="${PREFIX}-${role}-bot"
    upload_avatar "$role" "$username" "${OUT_DIR}/${username}.token"
  done <<EOF
$ROLE_TABLE
EOF
  print_summary_and_exit
fi

if [ "$DRY_RUN" -eq 0 ]; then
  GROUP_ENC=$(urlencode "$GROUP")
  gl_api GET "/groups/${GROUP_ENC}"
  if [ "$GL_LAST_STATUS" != "200" ]; then
    echo "error: could not resolve group '${GROUP}' (HTTP ${GL_LAST_STATUS})" >&2
    cat "$GL_LAST_BODY_FILE" >&2
    rm -f "$GL_LAST_BODY_FILE"
    exit 1
  fi
  GROUP_ID=$(jq -r '.id' "$GL_LAST_BODY_FILE")
  GROUP_PLAN=$(jq -r '.plan // ""' "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"
  if [ -n "$GROUP_PLAN" ]; then
    GROUP_TIER=$(classify_plan "$GROUP_PLAN")
    echo "tier: group plan='${GROUP_PLAN}' -> ${GROUP_TIER}"
  else
    echo "tier: group response carries no 'plan' field — will probe (Premium form first, free-tier form on a 400 naming allowed_to_)"
  fi
fi

if [ "$AVATARS" -eq 0 ]; then
  echo "NOTICE: --no-avatars — service accounts keep the default Gravatar identicon; re-run with --avatars-only to set them later"
fi

BOARD_WRITER_ID=""

echo "$ROLE_TABLE" | while IFS=: read -r role access_name access_num scopes; do
  [ -z "$role" ] && continue

  username="${PREFIX}-${role}-bot"
  display_name="Assay ${role} (fleet bot)"

  if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would create service account ${username} (role=${role}, access=${access_name}, scopes=${scopes})"
    echo "[dry-run]   would ensure group membership at access_level=${access_num}"
    echo "[dry-run]   would mint a PAT (scopes=${scopes}, expires in ${PAT_EXPIRY_DAYS}d) if the account is new"
    if [ "$AVATARS" -eq 1 ]; then
      if [ -n "$AVATARS_DIR" ]; then
        echo "[dry-run]   would upload avatar for ${username} <- ${AVATARS_DIR}/${role}.png (PUT /user/avatar, as that account's own PAT)"
      else
        echo "[dry-run]   would upload avatar for ${username} <- ${AVATAR_ICON_BASE}/app-icon-${role}.png (PUT /user/avatar, as that account's own PAT)"
      fi
    else
      echo "[dry-run]   would skip the avatar upload for ${username} (--no-avatars)"
    fi
    continue
  fi

  # --- service account: idempotent on username collision (spec: GitLab's
  # own uniqueness error is the backstop; we check first so a re-run reads
  # as a named no-op instead of a raw 409). ---
  gl_api GET "/groups/${GROUP_ID}/service_accounts"
  existing_id=$(jq -r --arg u "$username" '.[] | select(.username==$u) | .id' "$GL_LAST_BODY_FILE" | head -1)
  rm -f "$GL_LAST_BODY_FILE"

  created_now=0
  if [ -n "$existing_id" ]; then
    user_id="$existing_id"
    echo "no-op: service account ${username} already exists (id=${user_id})"
  else
    body=$(jq -n --arg n "$display_name" --arg u "$username" \
      '{name: $n, username: $u}')
    gl_api POST "/groups/${GROUP_ID}/service_accounts" "$body"
    if [ "$GL_LAST_STATUS" != "201" ]; then
      echo "error: creating service account ${username} failed (HTTP ${GL_LAST_STATUS})" >&2
      cat "$GL_LAST_BODY_FILE" >&2
      rm -f "$GL_LAST_BODY_FILE"
      exit 1
    fi
    user_id=$(jq -r '.id' "$GL_LAST_BODY_FILE")
    rm -f "$GL_LAST_BODY_FILE"
    created_now=1
    echo "created: service account ${username} (id=${user_id})"
  fi

  if [ "$role" = "board-writer" ]; then
    BOARD_WRITER_ID="$user_id"
    echo "$BOARD_WRITER_ID" > "${OUT_DIR}/.board-writer-id"
  fi

  # --- group membership: idempotent existence check. ---
  gl_api GET "/groups/${GROUP_ID}/members/${user_id}"
  if [ "$GL_LAST_STATUS" = "200" ]; then
    have_level=$(jq -r '.access_level' "$GL_LAST_BODY_FILE")
    rm -f "$GL_LAST_BODY_FILE"
    if [ "$have_level" != "$access_num" ]; then
      echo "NOTICE: ${username} is already a group member at access_level=${have_level}, expected ${access_num} — not changed automatically, check by hand"
    else
      echo "no-op: ${username} already a group member at access_level=${access_num}"
    fi
  else
    rm -f "$GL_LAST_BODY_FILE"
    body=$(jq -n --argjson uid "$user_id" --argjson lvl "$access_num" '{user_id: $uid, access_level: $lvl}')
    gl_api POST "/groups/${GROUP_ID}/members" "$body"
    if [ "$GL_LAST_STATUS" != "201" ]; then
      echo "error: adding ${username} to group failed (HTTP ${GL_LAST_STATUS})" >&2
      cat "$GL_LAST_BODY_FILE" >&2
      rm -f "$GL_LAST_BODY_FILE"
      exit 1
    fi
    rm -f "$GL_LAST_BODY_FILE"
    echo "added: ${username} to group at access_level=${access_num}"
  fi

  # --- PAT: only minted for an account created THIS run (spec's
  # single-point-of-failure note: re-run degrades to a NAMED no-op, never a
  # duplicate; a fresh credential for an existing account is a rotate, not
  # a re-provision — use the service-accounts rotate endpoint for that). ---
  if [ "$created_now" -eq 1 ]; then
    expires_at=$(pat_expiry_date)
    scopes_json=$(printf '%s' "$scopes" | tr ',' '\n' | jq -R . | jq -s .)
    body=$(jq -n --arg n "assay-${role}-fleet" --arg e "$expires_at" --argjson s "$scopes_json" \
      '{name: $n, scopes: $s, expires_at: $e}')
    gl_api POST "/groups/${GROUP_ID}/service_accounts/${user_id}/personal_access_tokens" "$body"
    if [ "$GL_LAST_STATUS" != "201" ]; then
      echo "error: minting PAT for ${username} failed (HTTP ${GL_LAST_STATUS})" >&2
      cat "$GL_LAST_BODY_FILE" >&2
      rm -f "$GL_LAST_BODY_FILE"
      exit 1
    fi
    token=$(jq -r '.token' "$GL_LAST_BODY_FILE")
    rm -f "$GL_LAST_BODY_FILE"
    token_file="${OUT_DIR}/${username}.token"
    ( umask 077; printf '%s' "$token" > "$token_file" )
    chmod 0600 "$token_file"
    unset token
    echo "minted: PAT for ${username} -> ${token_file} (0600, expires ${expires_at}) — path printed, value never echoed"

    # The avatar goes on NOW: this is the only moment the script holds this
    # account's own credential, and only the account itself can set it
    # (owner PUT /users/:id -> 403). Issue #346.
    if [ "$AVATARS" -eq 1 ]; then
      upload_avatar "$role" "$username" "$token_file"
    fi
  else
    echo "NOTICE: PAT minting skipped for ${username} (account pre-existing) — rotate via the group service-accounts rotate endpoint for a fresh credential, per spec.md §5"
    if [ "$AVATARS" -eq 1 ]; then
      if [ -s "${OUT_DIR}/${username}.token" ]; then
        upload_avatar "$role" "$username" "${OUT_DIR}/${username}.token"
      else
        echo "NOTICE: avatar for ${username} skipped — pre-existing account and no token file under ${OUT_DIR}; re-run with --avatars-only --out-dir <dir holding its token> to set it"
      fi
    fi
  fi
done

if [ "$DRY_RUN" -eq 0 ] && [ -f "${OUT_DIR}/.board-writer-id" ]; then
  BOARD_WRITER_ID=$(cat "${OUT_DIR}/.board-writer-id")
fi

# ---------------------------------------------------------------------------
# Protected `main` (spec.md §3, issue #346).
#
# Two rules this block exists to keep:
#  1. The `allowed_to_*` arrays are Premium/Ultimate (edition matrix B2). Below
#     that tier the scalar push_access_level / merge_access_level /
#     allow_force_push fields are the whole vocabulary, and sending an array
#     rejects the WHOLE request with a 400.
#  2. `main` is never left unprotected across a failure. Preference order:
#     no-op (already as intended) > PATCH (the fields PATCH accepts) >
#     DELETE+POST with an immediate re-POST of the rule that was read.
# ---------------------------------------------------------------------------
PREV_RULE=0
PREV_PUSH=0
PREV_MERGE=$MERGE_ACCESS_LEVEL
PREV_FORCE=false
PREV_PUSH_USER=""

read_protected_main() {
  gl_api GET "/projects/${PROJECT_ID}/protected_branches/main"
  if [ "$GL_LAST_STATUS" != "200" ]; then
    rm -f "$GL_LAST_BODY_FILE"
    PREV_RULE=0
    return 0
  fi
  PREV_RULE=1
  PREV_PUSH=$(jq -r '.push_access_levels[0].access_level // 0' "$GL_LAST_BODY_FILE")
  PREV_MERGE=$(jq -r '.merge_access_levels[0].access_level // 40' "$GL_LAST_BODY_FILE")
  PREV_FORCE=$(jq -r 'if has("allow_force_push") then (.allow_force_push|tostring) else "false" end' "$GL_LAST_BODY_FILE")
  PREV_PUSH_USER=$(jq -r '.push_access_levels[0].user_id // ""' "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"
}

# protect_body FORM — the POST payload. `premium` names the board-writer in
# allowed_to_push; `free` uses only the three scalar fields Free accepts.
protect_body() {
  if [ "$1" = "premium" ]; then
    jq -n --argjson uid "$BOARD_WRITER_ID" --argjson m "$MERGE_ACCESS_LEVEL" '{
      name: "main",
      allowed_to_push: [{user_id: $uid}],
      allowed_to_merge: [{access_level: $m}],
      allowed_to_unprotect: [{access_level: 50}],
      allow_force_push: false
    }'
  else
    jq -n --argjson m "$MERGE_ACCESS_LEVEL" '{
      name: "main",
      push_access_level: 0,
      merge_access_level: $m,
      allow_force_push: false
    }'
  fi
}

# Re-apply exactly what read_protected_main saw, so a refused POST cannot leave
# `main` open. Scalar form unless the previous rule named a user.
restore_protected_main() {
  local body
  if [ -n "$PREV_PUSH_USER" ]; then
    body=$(jq -n --argjson uid "$PREV_PUSH_USER" --argjson m "$PREV_MERGE" --argjson f "$PREV_FORCE" \
      '{name: "main", allowed_to_push: [{user_id: $uid}], allowed_to_merge: [{access_level: $m}], allow_force_push: $f}')
  else
    body=$(jq -n --argjson p "$PREV_PUSH" --argjson m "$PREV_MERGE" --argjson f "$PREV_FORCE" \
      '{name: "main", push_access_level: $p, merge_access_level: $m, allow_force_push: $f}')
  fi
  gl_api POST "/projects/${PROJECT_ID}/protected_branches" "$body"
  local st="$GL_LAST_STATUS"
  rm -f "$GL_LAST_BODY_FILE"
  [ "$st" = "201" ]
}

# Read the three decided fields BACK and print them, so a wrong rule is visible
# at provisioning time rather than at parity-walk time (issue #346, comment 1).
readback_protected_main() {
  gl_api GET "/projects/${PROJECT_ID}/protected_branches/main"
  if [ "$GL_LAST_STATUS" != "200" ]; then
    rm -f "$GL_LAST_BODY_FILE"
    echo "NOTICE: could not read protected 'main' back (HTTP ${GL_LAST_STATUS}) — the three decided fields are could-not-check, not a pass"
    record_failure "protected-branch read-back on 'main' failed — verify push/merge/force-push by hand"
    return 0
  fi
  local p m f u
  p=$(jq -r '.push_access_levels[0].access_level // "none"' "$GL_LAST_BODY_FILE")
  m=$(jq -r '.merge_access_levels[0].access_level // "none"' "$GL_LAST_BODY_FILE")
  # jq's `//` treats `false` as absent, so a boolean read-back must test for
  # the KEY, never fall back on the value. Reading false as "unknown" would
  # report a correct rule as a failure.
  f=$(jq -r 'if has("allow_force_push") then (.allow_force_push|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  u=$(jq -r '.push_access_levels[0].user_id // ""' "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"
  echo "read-back: main push_access_level=${p}${u:+ (user_id=${u})} merge_access_level=${m} allow_force_push=${f}"
  if [ "$m" != "$MERGE_ACCESS_LEVEL" ]; then
    echo "NOTICE: merge_access_level is ${m}, intended ${MERGE_ACCESS_LEVEL} (Maintainers) — at 30 every Developer service account can merge its own MR"
    record_failure "protected 'main' merge_access_level is ${m}, intended ${MERGE_ACCESS_LEVEL} (Maintainers)"
  fi
  if [ "$f" != "false" ]; then
    echo "NOTICE: allow_force_push reads ${f}, intended false"
    record_failure "protected 'main' allow_force_push is ${f}, intended false"
  fi
}

configure_protected_main() {
  read_protected_main

  local want_user=""
  case "$GROUP_TIER" in premium) want_user="$BOARD_WRITER_ID" ;; esac

  # Preference 1 — already as intended: touch nothing at all.
  if [ "$PREV_RULE" -eq 1 ] && [ "$PREV_MERGE" = "$MERGE_ACCESS_LEVEL" ] && [ "$PREV_FORCE" = "false" ] \
     && { { [ -n "$want_user" ] && [ "$PREV_PUSH_USER" = "$want_user" ]; } \
       || { [ -z "$want_user" ] && [ -z "$PREV_PUSH_USER" ] && [ "$PREV_PUSH" = "0" ]; }; }; then
    echo "no-op: main is already protected as intended — nothing deleted, nothing re-created"
    readback_protected_main
    return 0
  fi

  # Preference 2 — only allow_force_push is wrong: PATCH accepts that field on
  # every tier, so the rule is never removed.
  if [ "$PREV_RULE" -eq 1 ] && [ "$PREV_FORCE" != "false" ] && [ "$PREV_MERGE" = "$MERGE_ACCESS_LEVEL" ] \
     && { { [ -n "$want_user" ] && [ "$PREV_PUSH_USER" = "$want_user" ]; } \
       || { [ -z "$want_user" ] && [ -z "$PREV_PUSH_USER" ] && [ "$PREV_PUSH" = "0" ]; }; }; then
    gl_api PATCH "/projects/${PROJECT_ID}/protected_branches/main" '{"allow_force_push": false}'
    local pst="$GL_LAST_STATUS"
    rm -f "$GL_LAST_BODY_FILE"
    if [ "$pst" = "200" ]; then
      echo "patched: main allow_force_push=false (PATCH — the rule was never removed)"
      readback_protected_main
      return 0
    fi
    echo "NOTICE: PATCH of allow_force_push returned HTTP ${pst} — falling back to a guarded re-create"
  fi

  # Preference 3 — guarded DELETE + POST. The forms to try, in order.
  local forms
  case "$GROUP_TIER" in
    premium) forms="premium" ;;
    free)    forms="free" ;;
    *)       forms="premium free" ;;
  esac

  local deleted=0 applied="" form st msg
  for form in $forms; do
    if [ "$PREV_RULE" -eq 1 ] && [ "$deleted" -eq 0 ]; then
      gl_api DELETE "/projects/${PROJECT_ID}/protected_branches/main"
      rm -f "$GL_LAST_BODY_FILE"
      deleted=1
      echo "unprotected: main (re-applying now; the previous rule goes back immediately if that fails)"
    fi
    gl_api POST "/projects/${PROJECT_ID}/protected_branches" "$(protect_body "$form")"
    st="$GL_LAST_STATUS"
    msg=$(cat "$GL_LAST_BODY_FILE")
    rm -f "$GL_LAST_BODY_FILE"
    if [ "$st" = "201" ]; then
      applied="$form"
      break
    fi
    echo "NOTICE: protect '${form}' form refused (HTTP ${st}): ${msg}"
    if [ "$form" = "premium" ] && [ "$st" = "400" ] && [ "$msg" != "${msg#*allowed_to_}" ]; then
      echo "tier: the Premium form was refused for its allowed_to_ arrays — retrying with the free-tier form"
      continue
    fi
    break
  done

  if [ -z "$applied" ]; then
    if [ "$PREV_RULE" -eq 1 ]; then
      if restore_protected_main; then
        echo "error: protecting main failed — protection re-applied to its previous state (push_access_level=${PREV_PUSH}${PREV_PUSH_USER:+ user_id=${PREV_PUSH_USER}} merge_access_level=${PREV_MERGE} allow_force_push=${PREV_FORCE})" >&2
        record_failure "protected-branch step failed; 'main' was restored to its PREVIOUS rule (merge_access_level=${PREV_MERGE}) — the intended rule is NOT in place"
      else
        echo "error: protecting main failed AND the previous rule could not be re-applied — 'main' is UNPROTECTED, fix it by hand now" >&2
        record_failure "URGENT: 'main' is UNPROTECTED — protect it by hand: POST /projects/${PROJECT_ID}/protected_branches with push_access_level=0&merge_access_level=${MERGE_ACCESS_LEVEL}&allow_force_push=false"
      fi
    else
      echo "error: protecting main failed and there was no previous rule to restore — 'main' is UNPROTECTED, fix it by hand now" >&2
      record_failure "URGENT: 'main' is UNPROTECTED — protect it by hand: POST /projects/${PROJECT_ID}/protected_branches with push_access_level=0&merge_access_level=${MERGE_ACCESS_LEVEL}&allow_force_push=false"
    fi
    return 0
  fi

  if [ "$applied" = "premium" ]; then
    echo "protected: main (allowed_to_push=board-writer only, allowed_to_merge=Maintainer role (${MERGE_ACCESS_LEVEL}), allow_force_push=false)"
  else
    echo "NOTICE: free tier — board-writer push allowlist not available (failed-at-tier, remediation: Premium)"
    echo "protected: main (push_access_level=0 'No one', merge_access_level=${MERGE_ACCESS_LEVEL} Maintainers, allow_force_push=false) — every write, board regeneration included, goes through an MR"
  fi
  readback_protected_main
}

configure_approvals() {
  local body st author committers available
  body=$(jq -n '{merge_requests_author_approval: false, merge_requests_disable_committers_approval: true}')
  gl_api POST "/projects/${PROJECT_ID}/approvals" "$body"
  st="$GL_LAST_STATUS"
  if [ "$st" != "201" ] && [ "$st" != "200" ]; then
    echo "error: setting approval rules failed (HTTP ${st})" >&2
    cat "$GL_LAST_BODY_FILE" >&2
    rm -f "$GL_LAST_BODY_FILE"
    record_failure "approval settings write failed (HTTP ${st})"
    return 0
  fi
  rm -f "$GL_LAST_BODY_FILE"

  # A 201 is NOT proof: on Free the write is accepted and the values stay at
  # their defaults (issue #346). Read them back before claiming anything.
  gl_api GET "/projects/${PROJECT_ID}/approvals"
  if [ "$GL_LAST_STATUS" != "200" ]; then
    rm -f "$GL_LAST_BODY_FILE"
    echo "NOTICE: approval settings could not be read back (HTTP ${GL_LAST_STATUS}) — could-not-check, not a pass"
    record_failure "approval settings read-back failed — verify by hand"
    return 0
  fi
  # Same `false`-is-not-absent rule as the protected-branch read-back above.
  author=$(jq -r 'if has("merge_requests_author_approval") then (.merge_requests_author_approval|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  committers=$(jq -r 'if has("merge_requests_disable_committers_approval") then (.merge_requests_disable_committers_approval|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  available=$(jq -r 'if has("merge_request_approvers_available") then (.merge_request_approvers_available|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"

  if [ "$author" = "false" ] && [ "$committers" = "true" ] && [ "$available" != "false" ]; then
    echo "configured: approvals (prevent-author, prevent-committers) — read back and confirmed"
    return 0
  fi
  echo "read-back: approvals merge_requests_author_approval=${author} merge_requests_disable_committers_approval=${committers} merge_request_approvers_available=${available}"
  echo "NOTICE: approval settings ignored at this tier (failed-at-tier, remediation: Premium)"
  if [ "$available" = "false" ]; then
    echo "NOTICE: approval RULES are unavailable here (merge_request_approvers_available=false) — the stored fields can read correct while an MR's own author still approves it; do not count approvals as a server-enforced gate on this tier"
  fi
}

# Protected release tags (issue #346 comment 1 §4 / pilot D-4 — "immutable
# release integrity", spec §3). Role-level protected tags are FREE tier and use
# the SCALAR create_access_level field, never the Premium allowed_to_create
# array (that array is the exact shape that produced this issue's 400). The
# rule is idempotent: an existing rule for the glob is a NAMED no-op, not a
# duplicate POST.
configure_protected_tags() {
  local st msg existing body
  gl_api GET "/projects/${PROJECT_ID}/protected_tags"
  if [ "$GL_LAST_STATUS" = "200" ]; then
    existing=$(jq -r --arg g "$PROTECTED_TAG_GLOB" '.[] | select(.name==$g) | .name' "$GL_LAST_BODY_FILE" | head -1)
    rm -f "$GL_LAST_BODY_FILE"
    if [ -n "$existing" ]; then
      echo "no-op: tags matching '${PROTECTED_TAG_GLOB}' are already protected (create_access_level unchanged)"
      return 0
    fi
  else
    rm -f "$GL_LAST_BODY_FILE"
    echo "NOTICE: could not list protected tags (HTTP ${GL_LAST_STATUS}) — could-not-check the existing rule; still attempting to create it"
  fi

  body=$(jq -n --arg g "$PROTECTED_TAG_GLOB" --argjson c "$PROTECTED_TAG_CREATE_LEVEL" \
    '{name: $g, create_access_level: $c}')
  gl_api POST "/projects/${PROJECT_ID}/protected_tags" "$body"
  st="$GL_LAST_STATUS"
  msg=$(cat "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"
  if [ "$st" = "201" ]; then
    echo "protected: tags '${PROTECTED_TAG_GLOB}' (create_access_level=${PROTECTED_TAG_CREATE_LEVEL} Maintainers) — only a human owner can create or move a release tag"
  else
    echo "error: protecting tags '${PROTECTED_TAG_GLOB}' failed (HTTP ${st}): ${msg}" >&2
    record_failure "protected-tags rule for '${PROTECTED_TAG_GLOB}' not created (HTTP ${st}) — any Developer bot can create/move a release tag until fixed"
  fi
}

# Merge checks (issue #346): only_allow_merge_if_pipeline_succeeds AND
# only_allow_merge_if_all_discussions_are_resolved. Both are FREE tier (matrix
# B6) and actually server-enforced — on a tier where approval rules are
# advisory, the unresolved-threads gate is one of the few merge checks that
# binds. Set together in one PUT, then read the two fields back off
# GET /projects/:id (three-state: a could-not-read is not a pass, and because
# both are enforced at Free a value that did not take is a real failure).
configure_merge_settings() {
  local body st p d
  body=$(jq -n '{only_allow_merge_if_pipeline_succeeds: true, only_allow_merge_if_all_discussions_are_resolved: true}')
  gl_api PUT "/projects/${PROJECT_ID}" "$body"
  st="$GL_LAST_STATUS"
  rm -f "$GL_LAST_BODY_FILE"
  if [ "$st" != "200" ]; then
    echo "error: setting merge checks failed (HTTP ${st})" >&2
    record_failure "merge checks (pipeline-succeeds, all-discussions-resolved) could not be set (HTTP ${st})"
    return 0
  fi
  echo "configured: pipelines must succeed before merge; all discussions must be resolved before merge"

  gl_api GET "/projects/${PROJECT_ID}"
  if [ "$GL_LAST_STATUS" != "200" ]; then
    rm -f "$GL_LAST_BODY_FILE"
    echo "NOTICE: could not read merge checks back (HTTP ${GL_LAST_STATUS}) — could-not-check, not a pass"
    record_failure "merge-checks read-back failed — verify the pipeline and discussion gates by hand"
    return 0
  fi
  # Same `false`-is-not-absent rule as the read-backs above: test for the KEY.
  p=$(jq -r 'if has("only_allow_merge_if_pipeline_succeeds") then (.only_allow_merge_if_pipeline_succeeds|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  d=$(jq -r 'if has("only_allow_merge_if_all_discussions_are_resolved") then (.only_allow_merge_if_all_discussions_are_resolved|tostring) else "unknown" end' "$GL_LAST_BODY_FILE")
  rm -f "$GL_LAST_BODY_FILE"
  echo "read-back: only_allow_merge_if_pipeline_succeeds=${p} only_allow_merge_if_all_discussions_are_resolved=${d}"
  if [ "$p" != "true" ]; then
    echo "NOTICE: only_allow_merge_if_pipeline_succeeds reads ${p}, intended true"
    record_failure "only_allow_merge_if_pipeline_succeeds read back as ${p}, intended true"
  fi
  if [ "$d" != "true" ]; then
    echo "NOTICE: only_allow_merge_if_all_discussions_are_resolved reads ${d}, intended true"
    record_failure "only_allow_merge_if_all_discussions_are_resolved read back as ${d}, intended true"
  fi
}

if [ -n "$PROJECT" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would protect branch 'main' on project ${PROJECT}: allowed_to_push=[board-writer], allowed_to_merge=[Maintainer role]"
    echo "[dry-run] would set approvals: merge_requests_author_approval=false, merge_requests_disable_committers_approval=true"
    echo "[dry-run] would protect tags '${PROTECTED_TAG_GLOB}' with create_access_level=${PROTECTED_TAG_CREATE_LEVEL} (Maintainers)"
    echo "[dry-run] would set only_allow_merge_if_pipeline_succeeds=true and only_allow_merge_if_all_discussions_are_resolved=true"
  else
    # A missing board-writer id no longer skips the whole block: it only rules
    # out the Premium push allowlist, which the free-tier form does not use.
    if [ -z "$BOARD_WRITER_ID" ]; then
      echo "NOTICE: board-writer service account id unknown (check ${OUT_DIR}/.board-writer-id) — the Premium push allowlist cannot be named; protecting main with the free-tier form instead" >&2
      GROUP_TIER="free"
    fi
    {
      PROJECT_ENC=$(urlencode "$PROJECT")
      gl_api GET "/projects/${PROJECT_ENC}"
      if [ "$GL_LAST_STATUS" != "200" ]; then
        echo "error: could not resolve project '${PROJECT}' (HTTP ${GL_LAST_STATUS})" >&2
        cat "$GL_LAST_BODY_FILE" >&2
        rm -f "$GL_LAST_BODY_FILE"
        exit 1
      fi
      PROJECT_ID=$(jq -r '.id' "$GL_LAST_BODY_FILE")
      rm -f "$GL_LAST_BODY_FILE"

      # Every settings step below runs even when the one before it failed —
      # issue #346: the protect step's `exit 1` also skipped the pipeline
      # gate, so the blast radius of one 400 was every later setting.
      configure_protected_main
      configure_approvals
      configure_protected_tags
      configure_merge_settings
    }
  fi
else
  echo "NOTICE: --project not given — protected-branch and approval settings skipped (could-not-check, not a pass); pass --project to configure them"
fi

print_summary_and_exit
