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

`f2004-fixture-git-env-escape` (regression-fixture-git-env-isolation): the #727
fixture built its git environment from the full inherited environment, so an
exported `GIT_DIR` sent its commit, remote, config and worktree writes to the
repository it named. The shell wrapper had the same shape: both wrapped suites
run git, and with `GIT_DIR` exported the #1145 suite failed instead of using its
own temporary repositories. `FixtureEnv` in `fixtureenv.go` now drops every
`GIT_*` variable for fixture children, and `IsolateGit` clears them from the test
process for the in-process origin readers. Both fixture sites use them.

Controls. `TestGitDirFixtureIsolation` runs the #727 fixture with `GIT_DIR`,
`GIT_WORK_TREE` and `GIT_INDEX_FILE` naming a second committed repository and
requires that repository to be byte-unchanged. `TestShellGitIsolation` does the
same for a planted shell fixture that creates and commits to its own repository.
Before the repair both failed with `fixture wrote to the GIT_DIR-named
repository` and `shell fixture wrote to the GIT_DIR-named repository`. After the
repair both pass, and `TestReg727WorktreeOrigin`, `TestReg786FleetHardening` and
`TestReg1145ShimCredential` pass with those three variables exported while the
named repository stays unchanged. `mutate_guard.py gitenv` replays the shell
control's failure with the filter bypassed.

Class guard. `TestFloorExecEnv` parses every Go source in this package and each
file declaring a manifest `TestReg*` entry point. Each `exec.Command` or
`exec.CommandContext` result must have its own `Env` assigned from `FixtureEnv`
in the same function, and `os.Environ()` may appear only in `fixtureenv.go`. A
planted source holds five unsafe sites and one healthy site, and the guard must
flag exactly the five. With the previous environment spelling restored, it named
both fixture sites. A separate planted `exec.Command("git", "status")` in a
scratch test file was named too. `mutate_guard.py execenv` replays that failure
with the guard disabled. Reused pre-existing tests keep their own fixtures and
are outside this guard.

`mutate_guard.py` now keeps a backup beside the mutated file and restores any
leftover backup before it starts. An interrupted control run therefore cannot
leave a guard disabled in the tree.
