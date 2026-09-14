### Fixed
- **`cellctl up --cockpit herdr` now brings up a herdr window when none is open, instead of
  silently doing nothing.** Previously the herdr arm only ever added labelled tabs to whatever
  window herdr already had open (`herdr tab create --label <l>`) — with no window open, those tabs
  had nowhere to land and the operator had to open herdr by hand first (#985). `up` now checks
  first (`herdr workspace list` — herdr's own noun, verified live against herdr 0.8.2, for what
  this cockpit and #961 call a "window") and, finding none, starts one
  (`herdr workspace create --label <cell>-<the first window>`) before any tab create — the same
  trigger point and create-if-absent shape the tmux arm already uses for its `<cell>-cell` session
  (`tmux has-session || tmux new-session`). The new workspace's own auto-seeded default tab is
  dropped once the cell's real tabs exist in it (closing it any earlier closes the whole workspace
  with it — verified live). When a window is already open, behaviour is unchanged: no
  `workspace create` call, and tab create runs exactly as it did before this fix. A build that
  cannot list or create workspaces, or whose `workspace create` fails, refuses up-front naming
  herdr and the exact command tried — never opens no window at all. `--cockpit auto` still falls
  through to tmux when herdr is not installed, as before. `tools/cellctl/tests/herdr-orca-launch.test.sh`
  covers all three cases (no window / window already open / herdr absent or lacking the verb),
  each against fixture stubs shaped from live probing of a real herdr 0.8.2.
