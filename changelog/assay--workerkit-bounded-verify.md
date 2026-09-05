### Changed
- The dispatched-worker prompt kit now binds Verify runs to be BOUNDED inside the
  agent: a worker runs targeted (`go test -run '<TestName>' ./<pkg>/...`) or
  single-package tests with an explicit `-timeout`, and never the whole-module
  `go test ./...` — a full-module run can overrun the agent's watchdog, which kills
  the agent mid-row and strands the work. The full suite is left to CI, which has no
  such watchdog. The same clause requires a worker to PUSH before starting a long
  Verify row, so a row that overruns the watchdog costs the row and not the branch.
  `deskdispatch`'s emitted-prompt test pins both halves of the new clause.
