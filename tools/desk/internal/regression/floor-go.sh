#!/usr/bin/env bash
# The one way the floor's runners start the go tool. check-floor.sh and
# testdata/mutate_guard.py both run `go` through this file (TestFloorGoChokePoint
# holds them to it), so every manifest row, reused pre-existing tests included,
# starts with git's environment cleared. An exported GIT_DIR (a hook, or git running
# the floor on a caller's behalf) would otherwise point a row's fixture git calls at
# the caller's repository, and the caller's global or system git config would reach
# the fixtures' own repositories.
#
# Every inherited CELLCTL_* variable is cleared too (#1631). A cellctl shim exports
# them into the shell that runs the floor, CELLCTL_GH_AMBIENT carrying the operator's
# ambient forge credential. The cellctl rows build their own cell and read these names
# as the cell they are testing, so an inherited value fails them for a reason unrelated
# to the change under test. CELLCTL itself, which picks the implementation a suite
# tests, is kept.
set -euo pipefail
scrub() {
  local var
  while IFS= read -r var; do
    case "$var" in
      [Gg][Ii][Tt]_*) unset "$var" ;;
      CELLCTL_*) unset "$var" ;;
    esac
  done < <(compgen -e)
  unset XDG_CONFIG_HOME
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 KUBECONFIG=/dev/null
}
scrub
exec go "$@"
