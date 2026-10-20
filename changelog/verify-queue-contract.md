### Added
- `statusgen --gate-scores` rows carry an additive, versioned queue classification (contract v1: judgment-dispatch, runnable-offline, continuation, human-action, unreadable, paused, parked) taken from the same classifier the board already uses.
- `verifyloop plan --json` renders the same plan value as the text output, with explicit healthy/skipped/unreadable root coverage and diagnostics kept off JSON stdout; `--base-queue` selects the canonical queue base.

### Changed
- `deskboard awaiting` and `throughput` read the queue classification: only judgment work counts toward verify depth and widen advice, and unreadable roots or missing contract fields are reported as coverage gaps instead of zero.
- `verifyloop plan` composes its operational overlays on top of the canonical queue base; the base can only demote a dispatch to the new `queue-held` bucket, and Evidence-only work stays separate from flip authority.
- `deskboard throughput` withholds the JSON `bottleneck` (and says why in `bottleneckWithheld`) whenever a stage's depth is a partial lower bound, and `verifyloop plan --base-queue` exits 6 (unverifiable) instead of 0 when a healthy root's canonical queue is partial or unsupported.

### Upgrade note
- These consumers need a `statusgen` carrying queue contract v1 (the `queue*` fields on `--gate-scores`). Until a release that carries it is pinned, every root reads `unsupported`: `deskboard throughput` reports COVERAGE-GAP for every stage, and `verifyloop plan --base-queue` holds every DISPATCH as could-not-check and exits 6. That is fail-closed and intended, not a regression; pin the paired `statusgen` release alongside these tools.
