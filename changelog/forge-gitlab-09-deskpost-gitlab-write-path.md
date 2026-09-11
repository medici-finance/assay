### Added
- `deskpost review` / `security-review` / `comment` / `ready` now resolve the repo's forge and
  read their preconditions + land the verdict/comment/ready-flip through the typed Forge surface,
  so a GitLab-resolved repo no longer fails closed with `deskpost has no gitlab write backend`
  (the #772 follow-up). On GitLab an approve/request-changes verdict lands through `Forge.PostReview`
  — its first shipping consumer — mapping to a head-pinned approval + verdict note the read path
  sees at head; the GitHub path is unchanged.
