### Fixed
- **evidence-automerge — an `enable`-only "unstable" refusal is a benign skip.** The staged
  `evidence-automerge` workflow's auto-merge request could not recover from its own failure:
  a refused `enablePullRequestAutoMerge` reddened the run, the red check run made the pull
  request UNSTABLE, and UNSTABLE made the next request refuse — so re-running the job could
  never clear it and only a head-advancing push did. The step now reads the pull request's
  status-check rollup on an "unstable" refusal and, when the only failing check is this
  workflow's own job, logs a `::warning::` and exits 0, so the run greens itself and the next
  pull-request or review event enables auto-merge. A refusal while any OTHER check is failing
  still reddens, and a rollup that cannot be read or parsed is could-not-check and also
  reddens. (#586)

### Changed
- **evidence-automerge — the refusal decision is extracted and unit-tested.** The
  four benign outcomes (accepted, already enabled, repository auto-merge off, `enable`-only
  unstable) and the everything-else-reddens rule now live in
  `tools/evidence-automerge/automerge-refusal.sh`, proved offline by
  `tools/evidence-automerge/automerge-refusal_test.sh` against fixture rollups — including a
  committed pre-fix reference impl that reds on the new cases. (#586)
