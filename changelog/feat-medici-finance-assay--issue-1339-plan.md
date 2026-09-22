### Fixed

- `fanoutloop plan` now reconciles every fresh Next-up row against the repo's open+merged PRs and
  routes by state instead of blindly offering the row (#1339): a row whose brief already MERGED is
  listed under a new `LANDED-UNRECONCILED` heading (with its PR number) and never dispatched — its
  board cell just never reconciled after the merge — a row with an OPEN PR is routed to the resume
  lane, and only unrepresented rows are dispatched. The match is keyed on each PR's `Brief:`
  trailer, never a branch name. A could-not-check read (or an unresolvable repo) HOLDS the fresh
  lane with a `FRESH LANE HELD:` line rather than offering rows on an unverified forge. `plan` takes
  an optional `--repo <owner/name>` (defaults to the configured-roots map for `--root`, else the
  checkout's origin remote). The PR-list transport is a typed forge op deferred to the cutover — the
  closed forge surface ships no forge-CLI call — so until it is wired the shipped `plan` performs no
  forge read; the classification, repo resolution and one-read reduction are in place.
