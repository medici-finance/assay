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

### Round 3: the floor runner (`f2004-fixture-git-env-escape`, continued)

The fixture repair above isolated the fixtures this floor added. The floor's runners
were still open: `check-floor.sh` and `mutate_guard.py` started `go test` with the
caller's environment unchanged. Three reused manifest rows (the `deskwt`,
`deskdispatch` and statusgen shallow-clone tests) run git with whatever environment
they inherit. With `GIT_DIR` exported, they wrote `user.name`, `user.email` and
`commit.gpgsign` into the config of the repository it named. Separately,
`HostileGitDir` built its victim through `FixtureEnv`. Under the `gitenv` mutation, the
victim's setup commit therefore landed in the caller's repository.

Repair (the runner-side option the reviewer named). `floor-go.sh` is now the only
place the floor's scripts start the go tool. It unsets every `GIT_*` variable and
`XDG_CONFIG_HOME`, then sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`.
Both `check-floor.sh` and `mutate_guard.py` call it, so one scrub covers every
manifest row, reused tests included. The reused tests themselves are unchanged.
`HostileGitDir` now builds its victim from `fixedGitEnv`: PATH, a private HOME and
null git config, written out as a fixed list. It does not read the caller's
environment or call `FixtureEnv`, so the `gitenv` mutation cannot reach the victim.
The shell-fixture wrapper also passes null global and system git config, which
addresses the round-3 advisory.

Fail-first. At the unfixed head, `check-floor.sh` was run with `GIT_DIR`,
`GIT_WORK_TREE` and `GIT_INDEX_FILE` naming a throwaway decoy repository. It exited 1
at the `deskwt` row, and the decoy's tree digest changed: its config gained
`user.name=Test`, `user.email=t@e.st` and `commit.gpgsign=false`. In a separate run
under the same exported variables, unfixed `mutate_guard.py gitenv` added one commit
to a fresh decoy (1 to 2 commits). After the repair, the full runner under the same
hostile variables printed `seed passes=26`, exited 0, and left the decoy's digest
unchanged. `mutate_guard.py gitenv` and `runnerenv` also left a decoy unchanged when
run with those variables exported.

Controls. `TestFloorRunnerGitIsolation` runs `check-floor.sh` over
`testdata/runner-manifest.md` while `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE`
name a second committed repository. That manifest has one planted row whose test,
under `testdata/runnerplant`, runs git with its inherited environment. The test then
requires the second repository to be byte-unchanged. `mutate_guard.py runnerenv`
removes the scrub call from `floor-go.sh`, and the control then fails with
`floor runner let a row write to the GIT_DIR-named repository`. Verify row 8 now
runs seven modes, and row 9 runs the control together with the class guard.

Class guard. `TestFloorGoChokePoint` walks every shell and Python file under this
package, at any depth, and flags any non-comment line that starts the go tool other
than through `floor-go.sh`. Go sources remain `TestFloorExecEnv`'s. Against the unfixed
scripts it named `check-floor.sh:27` and `testdata/mutate_guard.py:75`. A separately
planted `testdata/second-plant.sh` running `go test` was named too and then removed.
In-test planted lines keep the matcher honest.
