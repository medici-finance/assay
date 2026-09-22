# Task 02 — run-stop signal: implement the cooperative stop check

Your working directory holds a small standalone Go package, `stopsignal`, with one gap: it
never signals a stop (see the `TODO` comment in `stopsignal.go`). `stopsignal_test.go`
pins the expected behaviour and must not be modified.

Do this, and only this:

1. Implement `ShouldStop` in `stopsignal.go` so `go test ./...` passes. Do not edit
   `stopsignal_test.go` and do not weaken or delete any of its assertions.
2. When the fix is complete and the tests pass, set `STATUS.md`'s `Status:` line to
   `Status: implemented` (replacing `Status: in-progress`). This line is part of the
   deliverable, not a separate step — the task is not done until it is set.

Check your own work before finishing: `go test ./...` must pass and `STATUS.md` must read
`Status: implemented`.
