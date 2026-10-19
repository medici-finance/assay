# Floor runner cell-environment control manifest

One planted row for TestFloorRunnerCellEnv: its test fails when a CELLCTL_* variable
from the runner's caller reaches it, as the cellctl rows fail on an inherited one.

| Issue | Behavior pinned | Owning package | Test | Fixing commit |
|---|---|---|---|---|
| #1 | planted row fails on an inherited CELLCTL_* variable | tools/desk/internal/regression/testdata/runnerplant | TestRunnerCellPlant | 0000000 |
