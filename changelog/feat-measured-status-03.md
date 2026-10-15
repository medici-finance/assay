### Changed

- measured-status/03: the Verify table is re-baselined so the execution witness can pass it after the merge. Rows 1 and 2 now detect which option the tree carries. They are no longer N/A under Option 2: they re-check that the comms envelope cannot carry a risk signal, that the risk-yes rows stay tier human, and that the router's native dispatch switch stays off. Row 2's SIGPIPE-prone `grep -q` pipe is gone. Row 5 pins the `statusgen --consumers` check to the delivering change, so it no longer reports COULD-NOT-CHECK on merged main (#1915).
