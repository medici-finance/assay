### Added
- **`cellctl up` is cockpit-aware: herdr, orca or tmux, chosen by what is on PATH.** A cell's role
  windows were stood one way only — a tmux window per role — so an operator running a cockpit with
  labelled tabs and a semantic agent state, or one with scheduled automations, still got tmux.
  `cell.env` now carries `CELL_COCKPIT` (default `auto`; `tmux` | `herdr` | `orca`), scaffolded by
  `cellctl new` and overridable per run with `cellctl up --cockpit <value>`. `auto` resolves by
  presence on PATH — herdr first, then orca, then tmux — never a flag someone has to remember; an
  `orca` binary whose desktop app does not answer a cheap, time-bounded probe **falls through** to
  tmux rather than failing, because its CLI is a thin client of that app. An **explicit** cockpit
  that is not available is a refusal naming exactly what is missing, never a silent fall-through.
  Herdr opens one labelled tab per window (`<cell>-<role>`), each started as a `claude`-kind agent
  under that label so the cockpit's agent state drives its sidebar per desk; orca opens one
  terminal per role running the same `cellctl desk` command, or — with the opt-in
  `cellctl up --automate '<cron>'` — one scheduled automation per role fronted by the exit-code
  precheck `cellctl check <cell>`, so a tick on a cell that is not fit to boot launches no model.
  Every `up`, `down` and `check` prints the resolved cockpit and the reason
  (`[cockpit] herdr (auto: on PATH)`, `[cockpit] tmux (orca on PATH but app unreachable)`), and
  `DRY_RUN=1 cellctl up <cell>` prints that plus the per-role commands and launches nothing.
  Nothing else about a cell changes with the cockpit: the per-role locked worktree, the roster
  beacon, the pinned model and the shim `PATH` are identical in all three, and tmux behaviour is
  unchanged.
### Changed
- **`cellctl down` and `cellctl check` follow the cockpit.** `down` takes `--cockpit`, always tears
  the tmux session down, and closes what a non-tmux cockpit opened where that cockpit offers a verb
  for it — **naming what to close by hand where it does not**, never leaving it unsaid (orca's
  scheduled automations outlive `down` on purpose and are named rather than deleted). `check` gains
  a cockpit precondition row carrying the same resolution and reason, plus an orca-reachability row
  whenever orca is installed, so which surface a boot will use is answerable before booting. These
  cockpit CLIs move fast, so every verb and flag `cellctl` cannot see is probed from `--help` at run
  time: a herdr build with no `tab create` still gets labelled windows with a notice, and an orca
  build whose terminal-create verb or command flag is absent gets the exact per-role commands
  printed to run by hand rather than a guessed spelling. `tools/cellctl/tests/cockpit.test.sh`
  covers the selection matrix offline with stub cockpit binaries on a private PATH.
