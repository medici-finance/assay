### Fixed
- The `evidence-automerge` `enable` job no longer reddens when a ready-flip leaves a PR with
  nothing left for auto-merge to wait on. `enablePullRequestAutoMerge` is a request, not a
  merge: a just-flipped approved PR whose checks are all green already satisfies every merge
  requirement, so GitHub refuses the enable with "Pull request is in clean status". That
  refusal published a failed check — the same self-inflicted loop as the "unstable status"
  case. `tools/evidence-automerge/automerge-refusal.sh` now treats a "clean status" refusal
  as the fifth benign outcome (exit 0 with a `::warning::` reason); nothing here merges, so
  the PR simply awaits a human's merge click. The staged workflow's `Request auto-merge` step
  now also echoes the mutation's answer to the log before deciding, so a benign skip still
  records why it skipped (#586).
