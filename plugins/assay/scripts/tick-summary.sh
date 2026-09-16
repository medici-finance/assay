#!/usr/bin/env bash
# tick-summary.sh — the ONE executable form of the tick summary-line grammar.
#
# A desk role invoked in tick mode (references/tick-contract.md) prints exactly
# one machine-readable line, as the LAST line of its output:
#
#   tick role=<role> outcome=<outcome> swept=<n> acted=<n> filed=<n> duration=<s>
#
# That line is the sole verdict channel: a one-shot harness call exits 0 for any
# completed turn, so the process exit status says nothing about what the pass
# concluded, and a killed pass prints nothing at all. A caller that receives no
# `tick …` line therefore knows the pass did not complete — which is the one
# distinction an empty log cannot otherwise make.
#
# WHY THIS FILE EXISTS. A grammar written only in prose gets two implementations
# the moment anyone parses it: the producer's and the consumer's, drifting. This
# script is the single implementation both are written against, and its hermetic
# suite (tick-summary.test.sh) is the proof it accepts exactly what the reference
# says and nothing more.
#
# THE CROSS-FIELD RULES ARE THE POINT. A pure shape check would accept
# `outcome=noop swept=-`, which is a blind pass wearing a healthy pass's clothes —
# exactly the confusion the three-state instrument rule exists to prevent. So the
# grammar is not only a shape:
#
#   ok               swept numeric, acted numeric and >= 1
#   noop             swept numeric, acted == 0
#   refused          swept == 0, acted == 0
#   could-not-check  swept == -        (the sweep did not complete: its count is unknown)
#
# A count that is genuinely unknown is written `-`, never `0`: a zero is a
# measurement claim, and a pass that did not finish looking has not made one.
# Because could-not-check REQUIRES `swept=-` and noop FORBIDS it, a blind pass
# cannot produce a well-formed noop line at all.
#
# USAGE
#   tick-summary.sh regexp                print the published extended regular expression
#   tick-summary.sh validate '<line>'     exit 0 iff that one line satisfies the grammar
#   tick-summary.sh check [< input]       exit 0 iff the LAST line of stdin satisfies it
#   tick-summary.sh --help | --version
#
# EXIT CODES
#   0  the line satisfies the grammar
#   1  it does not (the reason is printed to stderr)
#   2  usage error
#
# No network, no credential, no state. bash 3.2 compatible (stock /bin/bash on
# macOS): no associative arrays, no mapfile/readarray, no case-modification
# expansion.
set -uo pipefail

VERSION="1"

# The five public desk role names. A role outside this set is a typo or a role
# that has no standing loop, and either way the line is not a desk tick.
ROLES="the-desk intake-desk worker-desk pr-review-desk verify-desk"

# The closed outcome set, in the reference's order.
OUTCOMES="ok noop refused could-not-check"

# The published shape. Counts are a non-negative integer OR the literal `-`
# (unknown); duration is always a non-negative integer, because a pass that
# printed this line necessarily knows how long it ran.
#
# Kept as one string so `regexp` prints exactly what `validate` applies — a
# second copy for documentation is the copy that goes stale.
COUNT='([0-9]+|-)'
GRAMMAR="^tick role=(the-desk|intake-desk|worker-desk|pr-review-desk|verify-desk) outcome=(ok|noop|refused|could-not-check) swept=${COUNT} acted=${COUNT} filed=${COUNT} duration=[0-9]+\$"

usage() {
  sed -n '2,47p' "$0" | sed 's/^# \{0,1\}//'
}

die() { printf 'tick-summary: %s\n' "$1" >&2; return 1; }

# field <line> <key> — echo the value of key=value, or the empty string.
field() {
  printf '%s\n' "$1" | tr ' ' '\n' | while IFS= read -r tok; do
    case "$tok" in
      "$2"=*) printf '%s' "${tok#*=}"; return 0 ;;
    esac
  done
}

# validate_line <line> — the whole grammar: shape first, then the cross-field
# rules. Prints the first reason it failed, so a producer gets a diagnosis
# rather than a bare non-zero.
validate_line() {
  line="$1"

  case "$line" in
    *"$(printf '\t')"*) die "the line contains a tab; fields are single-space separated"; return 1 ;;
  esac

  if ! printf '%s' "$line" | grep -Eq "$GRAMMAR"; then
    die "does not match the published grammar: $line"
    printf '  expected: tick role=<role> outcome=<outcome> swept=<n|-> acted=<n|-> filed=<n|-> duration=<s>\n' >&2
    printf '  roles:    %s\n' "$ROLES" >&2
    printf '  outcomes: %s\n' "$OUTCOMES" >&2
    return 1
  fi

  outcome=$(field "$line" outcome)
  swept=$(field "$line" swept)
  acted=$(field "$line" acted)

  # The cross-field rules. Each one is a distinction the shape alone cannot make.
  case "$outcome" in
    could-not-check)
      # A pass whose sweep did not complete has no count to report. Writing a
      # number here — 0 above all — claims a measurement it never made.
      if [ "$swept" != "-" ]; then
        die "outcome=could-not-check requires swept=- (the sweep did not complete, so its count is unknown); got swept=$swept"
        return 1
      fi
      ;;
    noop)
      # noop is a positive claim about the queue: it was read, and it was empty
      # of actionable work. An unknown swept count cannot support that claim —
      # that line is a could-not-check.
      if [ "$swept" = "-" ]; then
        die "outcome=noop requires a numeric swept (a blind pass is could-not-check, never noop); got swept=-"
        return 1
      fi
      if [ "$acted" != "0" ]; then
        die "outcome=noop requires acted=0 (a pass that acted is ok); got acted=$acted"
        return 1
      fi
      ;;
    ok)
      if [ "$swept" = "-" ]; then
        die "outcome=ok requires a numeric swept; got swept=-"
        return 1
      fi
      case "$acted" in
        -|0) die "outcome=ok requires acted >= 1 (a pass that acted on nothing is noop); got acted=$acted"; return 1 ;;
      esac
      ;;
    refused)
      # A refusal happens before the sweep: there is nothing unknown about it.
      if [ "$swept" != "0" ] || [ "$acted" != "0" ]; then
        die "outcome=refused requires swept=0 and acted=0 (the pass declined before sweeping); got swept=$swept acted=$acted"
        return 1
      fi
      ;;
  esac

  return 0
}

case "${1:-}" in
  regexp)
    printf '%s\n' "$GRAMMAR"
    ;;
  validate)
    if [ "$#" -ne 2 ]; then
      printf 'tick-summary: validate takes exactly one argument, the candidate line\n' >&2
      exit 2
    fi
    validate_line "$2" || exit 1
    ;;
  check)
    # The LAST non-empty line of the input is the summary line. Anything after
    # it means the pass kept printing, which the contract forbids.
    input=$(cat)
    if [ -z "$input" ]; then
      printf 'tick-summary: empty input — no summary line, so the pass did not complete\n' >&2
      exit 1
    fi
    last=$(printf '%s\n' "$input" | sed -e '/^[[:space:]]*$/d' | tail -1)
    validate_line "$last" || exit 1
    ;;
  --version)
    printf 'tick-summary.sh %s\n' "$VERSION"
    ;;
  --help|-h|"")
    usage
    [ -n "${1:-}" ] || exit 2
    ;;
  *)
    printf 'tick-summary: unknown verb %s\n' "$1" >&2
    exit 2
    ;;
esac
