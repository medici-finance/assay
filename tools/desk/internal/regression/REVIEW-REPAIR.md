## Review repair receipt

`f2004-ci-directory-coverage` (regression-ci-descendant-trigger-coverage):
the old predicate checked registry directory names as if they were changed files.
The repair walks every registered directory and checks every descendant file.
Both the public floor entrypoint and the legacy registry loop use this check.
The class sweep found these two coverage loops; branch filters use the same glob
matcher on a branch name and are outside this directory-reader class.

Before the repair, the compiler-valid new control failed all six combinations:
PR/push filters containing only `tools/desk`, `statusgen`, or `tools/cellctl`
reported `directory-only filter admitted descendant read`. A separate
`new-reader/deep/second.txt` plant reported `second directory-only plant was missed`.
After the repair those controls pass, name the uncovered descendant, and retain
healthy `directory/**` coverage. `mutate_guard.py directories` replays the guard's
failure when enumeration is bypassed. No live workflow was changed.

`f2004-shell-time-budget` (regression-shell-execution-budget): the reviewer
retains repeated full-suite and package timeout failures at 25 seconds; those
runs are not green. The unchanged targeted suite passed in 23.22 seconds and
the direct suite passed 49 assertions in 22.70 seconds. A fresh unchanged
wrapper run passed in 20.31 seconds. The cause of variation remains unproven.
The 25-second cap leaves only 1.78 seconds above the observed 23.22-second pass.

The wrapper now has a finite 60-second budget; the bounded named runner and new
regression Verify row allow 90 seconds. Every shell assertion is unchanged.
The deadline control executes a one-second sleeping fixture under a 20ms
context: it must fail with cancellation. Replacing the context-aware command
with an ordinary command produced the assertion failure `deadline fixture
completed without cancellation: err=<nil> context=context deadline exceeded`.
The restored command passes. `mutate_guard.py deadline` retains that control.
A one-second pipe wait bound also prevents descendants retaining output pipes
from holding the wrapper indefinitely after cancellation.

These are implementer repair receipts, not reviewer resolution. The statusgen
CI activation hold remains on issue #1836, and its staged patch is unchanged.
