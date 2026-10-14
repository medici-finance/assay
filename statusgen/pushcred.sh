# pushcred.sh — preflight and classify a board push (#2910).
#
# A generated-board writer (the STATUS.md regen, the QUALITY.md regen, the
# gate:model auto-flip) pushes straight to the default branch and retries when
# the push loses a race with a concurrent merge. Before this file, EVERY push
# failure read as that race: an expired credential printed "push rejected: main
# moved", spent the whole retry budget, and then blamed a merge burst that never
# happened. This file is the one place that tells the failures apart:
#
#   expired | revoked | scope | auth | nonff | other
#
# Only `nonff` (the remote moved under us) may retry or say that main moved.
# Every other class fails ONCE, with its own message, and spends no retry.
#
# The credential value is never echoed: no token rides in a URL or an argument,
# git output is masked before it is shown (URL userinfo, the named variable's
# value, token-shaped strings), and xtrace is paused around the one read of it.
#
# POSIX sh. Sourced (`. statusgen/pushcred.sh`) by the workflow steps that push
# a generated board, and embedded verbatim in the GitLab CI template that
# `statusgen init --forge gitlab` scaffolds. Defines functions only.
#
# Inputs (all optional):
#   PUSHCRED_FORGE      `gitlab` turns on token self-introspection.
#   PUSHCRED_TOKEN_VAR  NAME of the variable holding the push token. When set,
#                       pushes authenticate through an inline credential helper
#                       that reads it, so the value never enters a URL or argv.
#   PUSHCRED_LOG        where the masked output of the last push lands
#                       (default: a mktemp file; a path that does not exist yet
#                       is created mode 0600).

# pushcred_valid_var NAME — true when NAME is a plain shell variable name.
pushcred_valid_var() {
  case $1 in
    '' | [0-9]* | *[!A-Za-z0-9_]*) return 1 ;;
  esac
  return 0
}

# pushcred_mask — stdin to stdout with credentials masked: URL userinfo,
# GitHub token shapes (classic gh?_ and fine-grained github_pat_), GitLab token
# shapes including the dotted suffix of the routable format, and the exact
# value of the variable PUSHCRED_TOKEN_VAR names.
pushcred_mask() {
  sed -E \
    -e 's#(://)[^/@[:space:]]+@#\1***@#g' \
    -e 's#gh[opsur]_[A-Za-z0-9]{16,}#***#g' \
    -e 's#github_pat_[A-Za-z0-9_]{16,}#***#g' \
    -e 's#(glpat|gldt|glrt|glcbt|glptt|glft|glimt|glagent|gloas|glsoat|glffct)-[A-Za-z0-9_.-]{16,}#***#g' |
    awk -v v="${PUSHCRED_TOKEN_VAR:-}" '{
      if (v != "") {
        t = ENVIRON[v]
        if (length(t) >= 4) {
          while ((i = index($0, t)) > 0) $0 = substr($0, 1, i - 1) "***" substr($0, i + length(t))
        }
      }
      print
    }'
}

# pushcred_class LOG — the class of a failed push, read from its masked output.
# Prints exactly one of: expired revoked scope auth nonff other.
pushcred_class() {
  if [ -z "${1:-}" ] || [ ! -r "$1" ]; then
    echo other
  elif grep -Eiq 'token (is|has) expired|expired (access |personal access |project access )?token' "$1"; then
    echo expired
  elif grep -Eiq 'token (was|has been|is) revoked|revoked token' "$1"; then
    echo revoked
  elif grep -Eiq 'insufficient_scope|not allowed to (push|upload)|Permission to [^ ]+ denied|Resource not accessible by integration|returned error: 403' "$1"; then
    echo scope
  elif grep -Eiq 'Authentication failed|HTTP Basic: Access denied|Invalid username or password|Bad credentials|returned error: 401|could not read (Username|Password)|terminal prompts disabled|invalid_token' "$1"; then
    echo auth
  elif grep -Eq '\[rejected\].*\((fetch first|non-fast-forward)\)|Updates were rejected because|cannot lock ref|incorrect old value provided' "$1"; then
    echo nonff
  else
    echo other
  fi
}

# pushcred_message CLASS — the one line a class prints. Only nonff names a
# moved main.
pushcred_message() {
  case $1 in
    expired) echo "push credential has EXPIRED. Rotate it, then re-run this job. This is not a race with main; no retry was spent." ;;
    revoked) echo "push credential was REVOKED. Issue a new one, then re-run this job. This is not a race with main; no retry was spent." ;;
    scope) echo "push credential lacks push permission (wrong scope, or its role cannot push this branch). Fix the credential's scope or role, then re-run this job. This is not a race with main; no retry was spent." ;;
    auth) echo "push credential was rejected (authentication failed; the forge did not say whether it is invalid, expired or revoked). Check or rotate it, then re-run this job. This is not a race with main; no retry was spent." ;;
    nonff) echo "push rejected: main moved." ;;
    *) echo "push failed for a reason that is neither a credential failure nor a moved main (a ruleset, a hook, or the network; see the masked git output above). No retry was spent." ;;
  esac
}

# pushcred_fail CLASS LABEL — print the class's line as an error.
pushcred_fail() {
  _pc_pre=
  if [ "${GITHUB_ACTIONS:-}" = true ]; then
    _pc_pre='::error::'
  fi
  echo "${_pc_pre}$2: $(pushcred_message "$1")"
}

# pushcred_push [GIT-PUSH-ARGS...] — `git push`, output masked into
# PUSHCRED_LOG and onto stderr; returns git's exit code.
pushcred_push() {
  if [ -z "${PUSHCRED_LOG:-}" ]; then
    PUSHCRED_LOG=$(mktemp "${TMPDIR:-/tmp}/pushcred.XXXXXX") || return 1
  fi
  # The unmasked output lands in its own mktemp file (mode 0600 by
  # construction, whatever the umask) and is removed once masked.
  _pc_raw=$(mktemp "${TMPDIR:-/tmp}/pushcred-raw.XXXXXX") || return 1
  _pc_rc=0
  if [ -n "${PUSHCRED_TOKEN_VAR:-}" ]; then
    if ! pushcred_valid_var "$PUSHCRED_TOKEN_VAR"; then
      echo "pushcred: PUSHCRED_TOKEN_VAR is not a variable name; refusing rather than guessing" >&2
      return 2
    fi
    # The helper text carries the variable's NAME; the helper's own shell reads
    # the value at run time, so it is in no URL, no argv and no config dump.
    GIT_TERMINAL_PROMPT=0 git -c credential.helper= \
      -c "credential.helper=!f() { test \"\$1\" = get || return 0; echo username=oauth2; printf 'password=%s\\n' \"\${$PUSHCRED_TOKEN_VAR}\"; }; f" \
      push "$@" >"$_pc_raw" 2>&1 || _pc_rc=$?
  else
    GIT_TERMINAL_PROMPT=0 git push "$@" >"$_pc_raw" 2>&1 || _pc_rc=$?
  fi
  (umask 077 && pushcred_mask <"$_pc_raw" >"$PUSHCRED_LOG")
  rm -f "$_pc_raw"
  cat "$PUSHCRED_LOG" >&2
  return "$_pc_rc"
}

# pushcred_gitlab_introspect — ask GitLab about the token itself
# (GET /personal_access_tokens/self; project and group access tokens answer it
# too). Prints one of: ok expired revoked scope auth unknown. `unknown` is a
# could-not-check (no API URL, no curl, or the token may not read the API), and
# the dry-run push stays the check.
pushcred_gitlab_introspect() {
  _pc_var=${PUSHCRED_TOKEN_VAR:-}
  if [ -z "$_pc_var" ] || ! pushcred_valid_var "$_pc_var" || [ -z "${CI_API_V4_URL:-}" ] ||
    ! command -v curl >/dev/null 2>&1; then
    echo unknown
    return 0
  fi
  _pc_body=$(mktemp "${TMPDIR:-/tmp}/pushcred-self.XXXXXX") || {
    echo unknown
    return 0
  }
  _pc_xt=
  case $- in *x*) _pc_xt=1 ; set +x ;; esac
  # The token travels as a header read from stdin, never as an argument.
  _pc_code=$(eval "printf 'PRIVATE-TOKEN: %s\\n' \"\${$_pc_var}\"" |
    curl -sS --max-time 20 -o "$_pc_body" -w '%{http_code}' -H @- \
      "${CI_API_V4_URL}/personal_access_tokens/self" 2>/dev/null) || _pc_code=000
  if [ -n "$_pc_xt" ]; then set -x; fi
  _pc_out=unknown
  case $_pc_code in
    200)
      _pc_out=ok
      if grep -Eq '"revoked" *: *true' "$_pc_body"; then
        _pc_out=revoked
      elif grep -Eq '"active" *: *false' "$_pc_body"; then
        _pc_out=expired
      else
        _pc_exp=$(sed -n 's/.*"expires_at" *: *"\([0-9][0-9-]*\)".*/\1/p' "$_pc_body" | tr -d '-')
        _pc_today=$(date -u +%Y%m%d)
        if [ -n "$_pc_exp" ] && [ "$_pc_exp" -le "$_pc_today" ] 2>/dev/null; then
          _pc_out=expired
        elif grep -q '"scopes"' "$_pc_body" && ! grep -Eq '"(write_repository|api)"' "$_pc_body"; then
          _pc_out=scope
        fi
      fi
      ;;
    401)
      if grep -Eiq 'expired' "$_pc_body"; then
        _pc_out=expired
      elif grep -Eiq 'revoked' "$_pc_body"; then
        _pc_out=revoked
      else
        _pc_out=auth
      fi
      ;;
  esac
  rm -f "$_pc_body"
  echo "pushcred: token self-introspection answered HTTP $_pc_code ($_pc_out)" >&2
  echo "$_pc_out"
}

# pushcred_classify — the class of the last pushcred_push. On GitLab a
# credential-looking or unexplained failure is refined by self-introspection,
# because GitLab's push refusal is one message for invalid, expired and
# under-scoped tokens alike.
pushcred_classify() {
  _pc_c=$(pushcred_class "${PUSHCRED_LOG:-}")
  if [ "${PUSHCRED_FORGE:-}" = gitlab ]; then
    case $_pc_c in
      auth | scope | other)
        _pc_i=$(pushcred_gitlab_introspect)
        case $_pc_i in
          expired | revoked) _pc_c=$_pc_i ;;
          scope | auth) if [ "$_pc_c" = other ]; then _pc_c=$_pc_i; fi ;;
        esac
        ;;
    esac
  fi
  echo "$_pc_c"
}

# pushcred_preflight LABEL [GIT-PUSH-ARGS...] — check the credential before a
# board commit is written and before each retry. On GitLab the token is asked
# about itself first; then a dry-run push authenticates against the remote
# without writing. A credential class fails here (return 1). A moved main is
# fine (the push loop handles it); any other dry-run failure is a NOTICE and
# the real push classifies again.
pushcred_preflight() {
  _pc_label=$1
  shift
  if [ "${PUSHCRED_FORGE:-}" = gitlab ]; then
    _pc_i=$(pushcred_gitlab_introspect)
    case $_pc_i in
      expired | revoked | scope | auth)
        pushcred_fail "$_pc_i" "$_pc_label (credential preflight)"
        return 1
        ;;
      unknown)
        echo "NOTICE: $_pc_label: could-not-check the push token by self-introspection; the dry-run push below is the check."
        ;;
    esac
  fi
  if pushcred_push --dry-run "$@"; then
    echo "$_pc_label: credential preflight passed (dry-run push authenticated)"
    return 0
  fi
  _pc_c=$(pushcred_classify)
  case $_pc_c in
    nonff)
      echo "$_pc_label: credential preflight passed (dry-run push authenticated; the remote is ahead)"
      return 0
      ;;
    other)
      echo "NOTICE: $_pc_label: the dry-run push failed for a reason that is not a credential failure (see above); the real push classifies again."
      return 0
      ;;
  esac
  pushcred_fail "$_pc_c" "$_pc_label (credential preflight)"
  return 1
}
