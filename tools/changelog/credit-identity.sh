#!/usr/bin/env bash
# credit-identity.sh — is this pull-request author somebody the release notes
# should THANK, and did they ask not to be?
#
# The FORGE half of external-contributor credit (contributor-trust/09) is the
# release workflow step: it reads a pull request's author login and body with
# `gh`, asks this script the two questions below, and writes the credits map
# that `aggregate.py highlights --credits` reads. This script itself makes NO
# network call and reads NO file but the body file it is handed, so the offline
# aggregate suite can exercise it directly.
#
#   credit-identity.sh classify <login> [<body-file>]
#
# prints exactly one word on stdout and always exits 0:
#
#   credit         an identity the operator's roster does NOT list, with no
#                  opt-out marker in the body: thank them.
#   skip:opt-out   the body carries the documented opt-out marker on a line of
#                  its own.
#   skip:roster    the roster already lists this identity — a maintainer, a
#                  mapped human, or a role automation account. The credit exists
#                  to name somebody the project does not already list.
#   skip:unknown   the question could not be answered: an empty login, or an
#                  UNCONFIGURED roster (with no roster there is no "external",
#                  so everybody would be thanked). Silence is the fail-closed
#                  direction here — a wrong name in a published release is worse
#                  than a missing one, and no release is ever refused for want
#                  of a credit.
#
# THE ROSTER IT READS is the operator's existing one, by its existing variable
# names — never a second roster invented for this one feature:
#
#   ASSAY_TRUSTED_LOGINS     comma-separated `login[:id]`
#   ASSAY_TRUSTED_BOT_SLUGS  comma-separated `[role=]slug[:id]`
#   ASSAY_HUMAN_LOGIN_MAP    comma-separated `name:login`
#
# INTERIM. This is a small local reader of those three variables, not the
# identity resolver contributor-trust/02 introduces. When 02 lands, this script
# is replaced by a call into that single resolver and deleted; the questions it
# answers, its output vocabulary and its callers do not change.
set -uo pipefail

# The documented opt-out: this marker on a line of its own in the pull-request
# body suppresses the credit for that pull request. Documented in
# changelog/README.md and tools/changelog/README.md.
OPT_OUT_MARKER='<!-- changelog-credit: no -->'

usage() {
  echo "usage: credit-identity.sh classify <login> [<body-file>]" >&2
}

# lowercase, trimmed
norm() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

# Split a comma-separated roster value into one entry per line. The trailing
# newline is load-bearing: without it `read` drops the LAST entry, which for a
# single-entry roster is every entry.
entries() {
  printf '%s\n' "${1:-}" | tr ',' '\n'
}

classify() {
  local login="${1:-}" body_file="${2:-}"
  local l
  l="$(norm "$login")"
  [ -n "$l" ] || { echo "skip:unknown"; return 0; }

  # Unset roster: nobody is knowably internal, so nobody is credited.
  if [ -z "${ASSAY_TRUSTED_LOGINS:-}" ] \
     && [ -z "${ASSAY_TRUSTED_BOT_SLUGS:-}" ] \
     && [ -z "${ASSAY_HUMAN_LOGIN_MAP:-}" ]; then
    echo "skip:unknown"
    return 0
  fi

  # The opt-out is read BEFORE the roster: an author who asked not to be thanked
  # is not thanked, whatever the roster says about them.
  if [ -n "$body_file" ] && [ -f "$body_file" ]; then
    # Fixed-string, whole-line match after trimming trailing carriage returns —
    # the marker must be on a line of its own, not buried in prose.
    if tr -d '\r' < "$body_file" | grep -qxF "$OPT_OUT_MARKER"; then
      echo "skip:opt-out"
      return 0
    fi
  fi

  local e v
  # ASSAY_TRUSTED_LOGINS — `login[:id]`, the login is the first field.
  while IFS= read -r e; do
    v="$(norm "${e%%:*}")"
    [ -n "$v" ] || continue
    if [ "$v" = "$l" ]; then echo "skip:roster"; return 0; fi
  done < <(entries "${ASSAY_TRUSTED_LOGINS:-}")

  # ASSAY_HUMAN_LOGIN_MAP — `name:login`, the login is the LAST field.
  while IFS= read -r e; do
    case "$e" in *:*) ;; *) continue ;; esac
    v="$(norm "${e##*:}")"
    [ -n "$v" ] || continue
    if [ "$v" = "$l" ]; then echo "skip:roster"; return 0; fi
  done < <(entries "${ASSAY_HUMAN_LOGIN_MAP:-}")

  # ASSAY_TRUSTED_BOT_SLUGS — `[role=]slug[:id]`. An App authors under
  # `<slug>[bot]` (REST) or `app/<slug>`, so compare against the bare slug with
  # either rendering stripped.
  local bare="$l"
  case "$bare" in
    *'[bot]') bare="${bare%'[bot]'}" ;;
    app/*)    bare="${bare#app/}" ;;
  esac
  while IFS= read -r e; do
    e="${e#*=}"           # drop any `role=` prefix
    v="$(norm "${e%%:*}")" # drop any `:id` suffix
    [ -n "$v" ] || continue
    if [ "$v" = "$bare" ] || [ "$v" = "$l" ]; then echo "skip:roster"; return 0; fi
  done < <(entries "${ASSAY_TRUSTED_BOT_SLUGS:-}")

  echo "credit"
}

main() {
  [ $# -ge 1 ] || { usage; echo "skip:unknown"; return 0; }
  case "$1" in
    classify)
      shift
      [ $# -ge 1 ] || { usage; echo "skip:unknown"; return 0; }
      classify "${1:-}" "${2:-}"
      ;;
    *)
      usage
      echo "skip:unknown"
      ;;
  esac
  return 0
}

main "$@"
