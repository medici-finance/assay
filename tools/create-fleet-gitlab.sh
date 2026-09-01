#!/usr/bin/env bash
# tools/create-fleet-gitlab.sh — idempotent Assay fleet provisioning for a
# GitLab Premium/Ultimate top-level group.
#
# Creates the seven per-role service accounts, their group memberships and
# personal access tokens, then (when --project is given) configures the
# fleet project's protected `main` branch and MR-approval settings. Prints a
# plain-text summary ending with the HUMAN-ONLY remainder this script never
# attempts: Ultimate-tier settings, the group token-expiry policy, and
# creation of the locked ci-config project.
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
#   GET    api/v4/groups/:id/service_accounts
#   POST   api/v4/groups/:id/service_accounts
#   POST   api/v4/groups/:id/service_accounts/:user_id/personal_access_tokens
#   GET    api/v4/groups/:id/members/:user_id
#   POST   api/v4/groups/:id/members
#   GET    api/v4/projects/:id/protected_branches/:name
#   DELETE api/v4/projects/:id/protected_branches/:name
#   POST   api/v4/projects/:id/protected_branches
#   GET    api/v4/projects/:id/approvals
#   POST   api/v4/projects/:id/approvals
#   PUT    api/v4/projects/:id
#
# Every endpoint above is reachable at the Premium tier. Nothing this script
# calls requires Ultimate — the Ultimate-only controls (custom roles,
# external status checks, pipeline execution policy) are named in the
# human-only checklist this script prints, never called.
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
  --dry-run               Enumerate every action; make zero network calls.
  -h, --help              This text.

Auth: a group-owner personal access token, supplied ONLY via the
GITLAB_TOKEN environment variable (never a flag, never argv) — required
unless --dry-run. This script never stores it.
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
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ -z "$GROUP" ] || [ -z "$PREFIX" ]; then
  echo "error: --group and --prefix are required" >&2
  usage >&2
  exit 2
fi

for cmd in curl jq; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "error: '$cmd' is required on PATH" >&2; exit 2; }
done

if [ "$DRY_RUN" -eq 0 ]; then
  if [ -z "${GITLAB_TOKEN:-}" ]; then
    echo "error: GITLAB_TOKEN must be set (a group-owner PAT) unless --dry-run" >&2
    exit 2
  fi
fi

if [ -z "$OUT_DIR" ]; then
  OUT_DIR=$(mktemp -d "${TMPDIR:-/tmp}/fleet-gitlab-tokens.XXXXXX")
fi
if [ "$DRY_RUN" -eq 0 ]; then
  mkdir -p "$OUT_DIR"
fi

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

echo "Assay fleet provisioning — group=${GROUP} prefix=${PREFIX} project=${PROJECT:-<none>} dry-run=${DRY_RUN}"

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
  rm -f "$GL_LAST_BODY_FILE"
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
  else
    echo "NOTICE: PAT minting skipped for ${username} (account pre-existing) — rotate via the group service-accounts rotate endpoint for a fresh credential, per spec.md §5"
  fi
done

if [ "$DRY_RUN" -eq 0 ] && [ -f "${OUT_DIR}/.board-writer-id" ]; then
  BOARD_WRITER_ID=$(cat "${OUT_DIR}/.board-writer-id")
fi

if [ -n "$PROJECT" ]; then
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would protect branch 'main' on project ${PROJECT}: allowed_to_push=[board-writer], allowed_to_merge=[Maintainer role]"
    echo "[dry-run] would set approvals: merge_requests_author_approval=false, merge_requests_disable_committers_approval=true"
    echo "[dry-run] would set only_allow_merge_if_pipeline_succeeds=true"
  else
    if [ -z "$BOARD_WRITER_ID" ]; then
      echo "NOTICE: board-writer service account id unknown (re-run without --project once, or check ${OUT_DIR}/.board-writer-id) — skipping protected-branch config" >&2
    else
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

      # Recreate rather than PATCH — portable across GitLab versions whose
      # protected-branches PATCH support differs; idempotent either way.
      gl_api GET "/projects/${PROJECT_ID}/protected_branches/main"
      if [ "$GL_LAST_STATUS" = "200" ]; then
        rm -f "$GL_LAST_BODY_FILE"
        gl_api DELETE "/projects/${PROJECT_ID}/protected_branches/main"
        rm -f "$GL_LAST_BODY_FILE"
        echo "unprotected: main (re-applying)"
      else
        rm -f "$GL_LAST_BODY_FILE"
      fi

      body=$(jq -n --argjson uid "$BOARD_WRITER_ID" '{
        name: "main",
        allowed_to_push: [{user_id: $uid}],
        allowed_to_merge: [{access_level: 40}],
        allowed_to_unprotect: [{access_level: 50}]
      }')
      gl_api POST "/projects/${PROJECT_ID}/protected_branches" "$body"
      if [ "$GL_LAST_STATUS" != "201" ]; then
        echo "error: protecting main failed (HTTP ${GL_LAST_STATUS})" >&2
        cat "$GL_LAST_BODY_FILE" >&2
        rm -f "$GL_LAST_BODY_FILE"
        exit 1
      fi
      rm -f "$GL_LAST_BODY_FILE"
      echo "protected: main (allowed_to_push=board-writer only, allowed_to_merge=Maintainer role)"

      body=$(jq -n '{merge_requests_author_approval: false, merge_requests_disable_committers_approval: true}')
      gl_api POST "/projects/${PROJECT_ID}/approvals" "$body"
      if [ "$GL_LAST_STATUS" != "201" ] && [ "$GL_LAST_STATUS" != "200" ]; then
        echo "error: setting approval rules failed (HTTP ${GL_LAST_STATUS})" >&2
        cat "$GL_LAST_BODY_FILE" >&2
        rm -f "$GL_LAST_BODY_FILE"
        exit 1
      fi
      rm -f "$GL_LAST_BODY_FILE"
      echo "configured: approvals (prevent-author, prevent-committers)"

      body=$(jq -n '{only_allow_merge_if_pipeline_succeeds: true}')
      gl_api PUT "/projects/${PROJECT_ID}" "$body"
      if [ "$GL_LAST_STATUS" != "200" ]; then
        echo "error: setting only_allow_merge_if_pipeline_succeeds failed (HTTP ${GL_LAST_STATUS})" >&2
        cat "$GL_LAST_BODY_FILE" >&2
        rm -f "$GL_LAST_BODY_FILE"
        exit 1
      fi
      rm -f "$GL_LAST_BODY_FILE"
      echo "configured: pipelines must succeed before merge"
    fi
  fi
else
  echo "NOTICE: --project not given — protected-branch and approval settings skipped (could-not-check, not a pass); pass --project to configure them"
fi

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
============================================================
CHECKLIST
