# boardadd.sh — stage the generated board, or say which ignore rule stops it (#2915).
#
# The regen job's commit step stages the board with a path-scoped `git add`. When
# the repo's ignore rules match one of those paths and git would not stage it (the
# very first STATUS.md on a repo whose template ignores it, or docs/streams under
# an ignored docs/), git prints a generic "Use -f" hint and exits 1 on every push
# to main, and the board is never committed. Force-adding (-f) would override the
# adopter's own ignore rules, so this never does: it names the matching rule and
# the fix, and fails.
#
# The verdict is git's own. `git add --dry-run --ignore-missing` refuses exactly
# the paths the real add refuses (an untracked match, or a directory under an
# ignored one) and accepts what the real add accepts (a tracked file a rule happens
# to match, or a path a later `!` rule re-includes). It writes nothing. `git
# check-ignore -v --no-index` only NAMES the rule: its exit status alone is wrong
# for this, because with -v a matching `!` rule also exits 0. `statusgen --lint`
# and `statusgen init` run the same two commands (statusgen/boardignore.go).
#
# POSIX sh. Embedded verbatim in both CI templates `statusgen init` scaffolds.
# Defines functions only.

# board_ignore_rule PATH — print the ignore rule matching PATH, as
# <source>:<line>:<pattern>, or nothing when no rule names it.
board_ignore_rule() {
  git check-ignore -v --no-index -- "$1" 2>/dev/null | sed -n '1s/	.*//p' || true
}

# board_ignore_message PATH RULE — the one line that says what to do.
board_ignore_message() {
  if [ -z "$2" ]; then
    printf '%s cannot be staged: git add refuses it, but git check-ignore names no rule; run "git add --dry-run -- %s" to see why\n' "$1" "$1"
    return 0
  fi
  case $1 in
    */*)
      printf '%s is ignored by %s; add "!%s" to .gitignore, or narrow that rule if it ignores a parent directory (git cannot re-include a path under an ignored directory)\n' "$1" "$2" "$1"
      ;;
    *)
      printf '%s is ignored by %s; add "!%s" to .gitignore\n' "$1" "$2" "$1"
      ;;
  esac
}

# board_add PATH... — `git add -- PATH...`, unless git would refuse a path for an
# ignore rule. Then print one error line per refused path (a GitHub `::error::`
# annotation under Actions) and return 1 with nothing staged.
board_add() {
  board_add_refused=0
  for board_add_path in "$@"; do
    if git add --dry-run --ignore-missing -- "$board_add_path" >/dev/null 2>&1; then
      continue
    fi
    board_add_refused=1
    board_add_line=$(board_ignore_message "$board_add_path" "$(board_ignore_rule "$board_add_path")")
    if [ "${GITHUB_ACTIONS:-}" = true ]; then
      printf '::error::%s\n' "$board_add_line"
    else
      printf 'ERROR: %s\n' "$board_add_line"
    fi
  done
  [ "$board_add_refused" = 0 ] || return 1
  git add -- "$@"
}
