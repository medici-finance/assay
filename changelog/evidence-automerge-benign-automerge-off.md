### Changed
- **evidence-automerge — repository auto-merge OFF is a benign skip.** The staged
  `evidence-automerge` workflow's "Request auto-merge" step now treats GitHub's
  "Auto merge is not allowed for this repository" response as a benign no-op
  (`exit 0`), exactly like the existing "already enabled" carve-out, instead of
  reddening the run. Enabling auto-merge is an optimisation, not the merge itself —
  the required review and status checks still gate the actual merge, and a human can
  merge directly — so a repository with the setting off is a skip, not a failure. (#579)
