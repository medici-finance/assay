### Fixed
- `deskboard` no longer reports a trusted human maintainer's own open PR as
  review-desk neglect. A non-draft PR with no reviewer-App verdict at head used to
  classify `NEEDS-REVIEW` and, after 30 minutes, trip the `UNREVIEWED` neglect
  alarm on every sweep — a false alarm that recurred for every human-gated brief
  closure (the maintainer fills the decision table in their own PR and merges it;
  the review desk deliberately does not dispatch a model reviewer on a human's own
  ratified ruling). Such a PR now classifies as a distinct `HUMAN-OWNED` row ("the
  author owns and merges it") and is kept out of the `NEEDS-REVIEW`/`RE-REVIEW`
  dispatch gate and the `UNREVIEWED` count, while its CI/mergeability columns still
  render. Only the accountable-human set qualifies (the mapped humans of
  `ASSAY_HUMAN_LOGIN_MAP` plus the blessing authority) — App-authored and
  shared-machine-account PRs stay `NEEDS-REVIEW` and remain in the neglect metric.
  The `reviewloop` reactor gives `HUMAN-OWNED` a matching no-op disposition.
