### Fixed
- desk-containers/15 Verify row 11 now passes under `verifyrun`'s `bash -o pipefail`: the expected exit 1 from `go run` on the unknown `cellop-fixture` verb is asserted inside the pipeline instead of failing it. Command-only correction; Expect, class and every other row unchanged (brief version 2 to 3).
