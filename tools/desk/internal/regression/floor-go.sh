#!/usr/bin/env bash
# The one way the floor's runners start the go tool. check-floor.sh and
# testdata/mutate_guard.py both run `go` through this file (TestFloorGoChokePoint
# holds them to it), so every manifest row, reused pre-existing tests included,
# starts with git's environment cleared. An exported GIT_DIR (a hook, or git running
# the floor on a caller's behalf) would otherwise point a row's fixture git calls at
# the caller's repository, and the caller's global or system git config would reach
# the fixtures' own repositories.
set -euo pipefail
scrub() {
  local var
  while IFS= read -r var; do
    case "$var" in [Gg][Ii][Tt]_*) unset "$var" ;; esac
  done < <(compgen -e)
  unset XDG_CONFIG_HOME
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 KUBECONFIG=/dev/null
}
scrub
exec go "$@"
