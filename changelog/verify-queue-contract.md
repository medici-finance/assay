### Added
- `statusgen --gate-scores` rows carry an additive, versioned queue classification (contract v1: judgment-dispatch, runnable-offline, continuation, human-action, unreadable, paused, parked) taken from the same classifier the board already uses.
- `verifyloop plan --json` renders the same plan value as the text output, with explicit healthy/skipped/unreadable root coverage and diagnostics kept off JSON stdout; `--base-queue` selects the canonical queue base.

### Changed
- `deskboard awaiting` and `throughput` read the queue classification: only judgment work counts toward verify depth and widen advice, and unreadable roots or missing contract fields are reported as coverage gaps instead of zero.
- `verifyloop plan` composes its operational overlays on top of the canonical queue base; the base can only demote a dispatch to the new `queue-held` bucket, and Evidence-only work stays separate from flip authority.
