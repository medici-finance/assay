# shellcheck shell=bash
# private-path.sh — sourced by the cellctl suites, never run on its own.
#
# private_path <dir> is the one way a suite puts its private stub directory in front of the
# inherited PATH. It writes no-op `tmux` and `herdr` stubs into <dir> first (a stub the suite
# already wrote there is kept), so the host's installed cockpits never decide a `check` row or
# the auto cockpit: auto resolves to the herdr stub on every host, and `command -v tmux` holds
# on a runner with no tmux. tools/desk/internal/regression's TestCellctlSuitesUsePrivatePath
# fails a suite that prepends to PATH any other way.
private_path(){
  local dir="$1" c
  mkdir -p "$dir"
  for c in tmux herdr; do
    [[ -e "$dir/$c" ]] && continue
    printf '#!/usr/bin/env bash\nexit 0\n' > "$dir/$c"
    chmod +x "$dir/$c"
  done
  export PATH="$dir:$PATH"
}
