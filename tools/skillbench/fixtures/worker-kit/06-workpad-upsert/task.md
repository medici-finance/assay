# Task 06 — workpad: implement the upsert-in-place rule

Your working directory holds a small standalone Go package, `workpad`, with one bug: it
always appends a new comment instead of editing the agent's own existing one in place (see
the `BUG` comment in `workpad.go`). `workpad_test.go` pins the expected behaviour and must
not be modified.

Do this, and only this:

1. Fix `Upsert` in `workpad.go` so `go test ./...` passes. Do not edit `workpad_test.go`
   and do not weaken or delete any of its assertions.
2. When the fix is complete and the tests pass, set `STATUS.md`'s `Status:` line to
   `Status: implemented` (replacing `Status: in-progress`). This line is part of the
   deliverable, not a separate step — the task is not done until it is set.

Check your own work before finishing: `go test ./...` must pass and `STATUS.md` must read
`Status: implemented`.
