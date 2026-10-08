### Changed

- build-less-brittle briefs 05, 09, 11, 12 and 13: the `statusgen --consumers` Verify row now pins the check to the delivering change (resolved from its `Brief:` trailer on first-parent main, diffed against its parent in a throwaway clone; the PR head against its merge-base before the merge), so it runs green after the merge instead of reporting COULD-NOT-CHECK. Two sibling rows were fixed for the execution witness: build-less-brittle/09 row 6 no longer dies of SIGPIPE under `pipefail`, and build-less-brittle/05 row 6 states its Expect as an exact output line (#1915).
