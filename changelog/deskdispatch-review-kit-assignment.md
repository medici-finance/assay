### Fixed
- `deskdispatch --kit review` now emits a REVIEW-shaped Assignment section instead of the
  implementer scaffold. The top of the emitted prompt previously told every dispatched
  reviewer to "Open the draft PR", run `deskpr create`, "Stop at `implemented`", self-register
  a PR number, and release its dispatch claim once its branch was pushed — directly
  contradicting the read-only review clauses that follow. A reviewer handed both could open a
  spurious draft PR for a PR that is already open, or review the fresh branch cut off `main`
  rather than the PR's head. The review Assignment is now read-only: it names the PR under
  review, says the reviewer opens no PR and pushes no branch, points the worktree at
  `pull/<N>/head`, and releases the claim once the verdict is posted. The `--kit worker`
  (implementer) Assignment is unchanged.
