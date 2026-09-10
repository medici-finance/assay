### Added

- **Scoped brief `forge-gitlab/09` — the GitLab reviewer write path.** Planning-only: the brief
  wires the review desk's verdict-and-escalation path (`deskpost review`/`security-review`/`comment`
  then `ready`, plus `deskfile`/`desktoken` reviewer auth) onto the typed Forge surface for a
  GitLab-resolved repo, using the provisioned role PAT rather than a GitHub App mint, with parity
  proven against the GitHub backend. It is the head of the field-check critical path (#795 §1 and §2).
