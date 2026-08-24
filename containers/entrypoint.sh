#!/bin/sh
# entrypoint.sh — the shared interactive boot for every assay desk image.
#
# ONE script, baked into all five thin per-desk images (intake-desk,
# worker-desk, pr-review-desk, verify-desk, the-desk). Each desk image differs
# only by the ASSAY_DESK env value; this entrypoint reads it and boots that
# desk. It does three things, in order:
#
#   1. FAIL-CLOSED CREDENTIAL PREFLIGHT — verify that the runtime credential
#      contract in containers/secrets.md is satisfied: the App-signing PEM is
#      mounted at its path, and at least one model credential is present in the
#      runtime environment. If either is missing, print exactly what is missing
#      (the env NAME and, for the PEM, the expected mount PATH) and exit
#      non-zero. No fallback to ambient host credentials, no anonymous degraded
#      mode. (secrets.md §5.)
#   2. PRINT DESK IDENTITY — the desk name and a by-name pointer to its skill
#      under the baked assay plugin, so the operator sees which desk booted.
#   3. LAND IN THE INTERACTIVE SESSION — exec the container command (default an
#      interactive shell) from which the desk skill is invoked.
#
# NO SECRET IS EVER PRINTED. The preflight checks for the PRESENCE of a
# credential (a readable file, a set env var) and never echoes a value. This
# script contains no credential and no default credential value — only the
# non-secret PATH default the contract explicitly permits (secrets.md §2).
#
# POSIX sh, no bashisms — this runs before the interactive shell is chosen.
set -eu

me="${ASSAY_DESK:-<unset>}"
plugin_dir="${ASSAY_PLUGIN_DIR:-/opt/assay/plugin}"

# The image MAY default the PEM mount PATH VALUE (a path string is not a
# secret); it must NEVER default the file's contents. secrets.md §2.
: "${ASSAY_APP_PEM_FILE:=/run/secrets/assay/app.pem}"

fail() {
  # Print a precise, credential-free diagnostic and exit fail-closed.
  echo "ERROR: $me cannot start — $1" >&2
}

missing=0

# --- 1a. PEM preflight -------------------------------------------------------
# The bot App signing key is the root GitHub secret: the desk mints its own
# short-lived per-role tokens FROM this PEM at runtime (desktoken). It must be
# mounted read-only at the contract path; the image never carries it.
if [ ! -f "$ASSAY_APP_PEM_FILE" ]; then
  fail "the bot App signing PEM is not mounted."
  {
    echo "  expected a readable file at: $ASSAY_APP_PEM_FILE"
    echo "  (env ASSAY_APP_PEM_FILE names the path; default /run/secrets/assay/app.pem)"
    echo "  mount it read-only, e.g.:"
    echo "    --mount type=bind,src=<host>/app.pem,dst=/run/secrets/assay/app.pem,ro"
    echo "  see containers/secrets.md §2."
  } >&2
  missing=1
elif [ ! -r "$ASSAY_APP_PEM_FILE" ]; then
  fail "the App PEM at $ASSAY_APP_PEM_FILE exists but is not readable by this user."
  echo "  mount it with mode 0400/0444 readable by the 'desk' user." >&2
  missing=1
fi

# --- 1b. Model-credential preflight -----------------------------------------
# At least one usable model credential must be present in the runtime
# environment, or an explicitly mounted, operator-supplied credential file must
# be configured (secrets.md §3/§5). The SDK's own fallback chain must terminate
# at the runtime-injected credential and NEVER at an ambient host credential —
# "started on an unknown credential" is the exact failure this forbids.
model_cred=0
for v in ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN ANTHROPIC_IDENTITY_TOKEN \
         AWS_BEARER_TOKEN_BEDROCK; do
  eval "val=\${$v:-}"
  if [ -n "$val" ]; then model_cred=1; break; fi
done
# A mounted WIF / Bedrock / Vertex credential FILE also counts — but only if the
# path is set AND the file is actually present (a bare env pointing at nothing
# is not a credential).
if [ "$model_cred" -eq 0 ]; then
  for f in ANTHROPIC_IDENTITY_TOKEN_FILE GOOGLE_APPLICATION_CREDENTIALS; do
    eval "p=\${$f:-}"
    if [ -n "$p" ] && [ -r "$p" ]; then model_cred=1; break; fi
  done
fi
# Bedrock via static AWS keys (with the CLI routed through Bedrock).
if [ "$model_cred" -eq 0 ] && [ -n "${CLAUDE_CODE_USE_BEDROCK:-}" ] \
   && [ -n "${AWS_ACCESS_KEY_ID:-}" ] && [ -n "${AWS_SECRET_ACCESS_KEY:-}" ]; then
  model_cred=1
fi

if [ "$model_cred" -eq 0 ]; then
  fail "no model credential found in the runtime environment."
  {
    echo "  set at least one of (see containers/secrets.md §3):"
    echo "    ANTHROPIC_API_KEY           — primary API key"
    echo "    ANTHROPIC_AUTH_TOKEN        — bearer / OAuth token"
    echo "    ANTHROPIC_IDENTITY_TOKEN    — workload-identity JWT (inline)"
    echo "    ANTHROPIC_IDENTITY_TOKEN_FILE / GOOGLE_APPLICATION_CREDENTIALS"
    echo "                                — a mounted credential file"
    echo "    AWS_* (+ CLAUDE_CODE_USE_BEDROCK) / GCP (+ CLAUDE_CODE_USE_VERTEX)"
    echo "                                — gateway credentials"
    echo "  supply them via --env-file / compose env_file / k8s envFrom."
    echo "  this desk does NOT fall back to any ambient host credential."
  } >&2
  missing=1
fi

if [ "$missing" -ne 0 ]; then
  echo "" >&2
  echo "$me: fail-closed preflight failed — refusing to start without its" >&2
  echo "runtime credentials (containers/secrets.md)." >&2
  exit 78   # EX_CONFIG — the runtime configuration is incomplete.
fi

# --- 2. Desk identity + skill pointer ---------------------------------------
skill_dir="$plugin_dir/skills/$me"
echo "================================================================"
echo " assay desk: $me"
if [ -d "$skill_dir" ]; then
  echo " skill:      $skill_dir"
  echo " invoke it in the interactive session as:  /$me"
else
  echo " skill:      (no baked skill dir found at $skill_dir)"
  echo " invoke it in the interactive session as:  /$me"
fi
echo " App PEM:    $ASSAY_APP_PEM_FILE (mounted, readable)"
echo " model cred: present in the runtime environment"
echo "================================================================"

# --- 3. Land in the interactive session -------------------------------------
# exec the container command (image CMD defaults to an interactive shell); the
# operator starts the desk by invoking /$me from there. Using exec makes the
# session PID 1 so signals reach it directly.
exec "$@"
