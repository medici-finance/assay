### Fixed
- `deskpost`'s model-capability-floor stamp age-out no longer refuses (verdict) or misreads (flip)
  every risk-classed review on a reviewed PR. The age-out now keys on the REVIEWER's
  review-dispatch claim family (`refs/dispatch/<short>--pr-<N>[--<suffix>]`) in the namespace
  claims actually live in — not the PR body's worker `Brief:` claim (released the moment the
  worker finishes) and not the empty `refs/heads/dispatch/*` namespace the old reader probed. A new
  `Forge.MatchingRefs` op does the prefix listing (GitHub `git/matching-refs`; GitLab CE is a
  could-not-check the review lane never reaches). Reader-side only — the acquire/writer namespace is
  untouched, and full reader+writer convergence remains the issue-708 follow-up.
