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

# private_deskwt <dir> closes the separate worktree-tool boundary: the launcher
# resolves deskwt on its caller's PATH, never in DESK_TOOLS_BIN. Normal cases use
# cellctl's own worktree implementation; an owned recorder still intercepts any
# accidental lookup. The log path is embedded because launch environments scrub
# test-only variables. This does not change deliberate deskwt integration suites.
private_deskwt(){
  local dir="$1"
  DESKWT_CALLS="$dir/deskwt.calls"
  : > "$DESKWT_CALLS"
  cat > "$dir/deskwt" <<EOF_DESKWT
#!/usr/bin/env bash
printf '%s\n' "deskwt \$*" >> "$DESKWT_CALLS"
exit 5
EOF_DESKWT
  chmod +x "$dir/deskwt"
  export CELLCTL_DESKWT=0
}

# Run the positive control before normal cases. It is a fresh real launch, so a
# shadowed recorder cannot make an empty log look like isolation. Clear only this
# control's calls; the EXIT check retains all subsequent calls, including on abort.
private_deskwt_check(){
  local cellctl="$1" repo="$2" cell="deskwt-control"
  if [[ "$(command -v deskwt)" != "$(dirname "$DESKWT_CALLS")/deskwt" ]]; then
    assert "deskwt resolves to the owned recorder" false
    return 1  # Never launch through an unexpected host executable.
  fi
  assert "deskwt resolves to the owned recorder" true
  : > "$DESKWT_CALLS"
  "$cellctl" new "$cell" --kind house --repo "$repo" --roots "fixture/repo=$repo" >/dev/null
  CELLCTL_DESKWT=1 "$cellctl" desk "$cell" worker-desk </dev/null >/dev/null 2>&1 || true
  assert "real launch with deskwt enabled records its probe" 'grep -qx "deskwt role-init --help" "$DESKWT_CALLS"'
  : > "$DESKWT_CALLS"
}

# Called before the suite's existing cleanup, even when a normal case exits early.
private_deskwt_finish(){
  [[ -n "${DESKWT_CALLS:-}" ]] || return 0
  assert "normal cases never called deskwt" '[[ ! -s "$DESKWT_CALLS" ]]'
  [[ "$fails" -eq 0 ]]
}
