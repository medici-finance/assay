# Task 03 — eligibility reconciliation: stop on a merged/closed PR

Your working directory holds a small standalone Go package, `eligibility`, with one bug:
it reports every PR state as eligible (see the `BUG` comment in `eligibility.go`).
`eligibility_test.go` pins the expected behaviour and must not be modified.

Do this, and only this:

1. Fix `IsEligible` in `eligibility.go` so `go test ./...` passes. Do not edit
   `eligibility_test.go` and do not weaken or delete any of its assertions.
2. When the fix is complete and the tests pass, set `STATUS.md`'s `Status:` line to
   `Status: implemented` (replacing `Status: in-progress`). This line is part of the
   deliverable, not a separate step — the task is not done until it is set.

Check your own work before finishing: `go test ./...` must pass and `STATUS.md` must read
`Status: implemented`.
