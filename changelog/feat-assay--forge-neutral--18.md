### Added
- `deskread` gains a `reviews` kind: a change's whole review array, oldest first, one row per submission (`authorLogin`, `authorId`, `state`, `commitId`, `submittedAt`), addressed with `--change owner/name#N`. It is transport over the existing `ReviewsAtHead` operation, so the seam gains no operation. An empty array is a real answer, a change it could not read lands in `partial`, and the review body is not carried.

### Changed
- `statusgen --review-rework`, `--first-pass-yield` and the `--assayscore` quality dimension read their merged-PR list and per-PR reviews through `deskread changes` and `deskread reviews` instead of `gh api`, so they run wherever `deskread` does and no longer need an authenticated `gh`. Their merged-PR list was one 100-row page with no sign of truncation. It is now the verb's bounded walk, and a window it could not read in full is reported as could-not-check rather than as a shorter list.
