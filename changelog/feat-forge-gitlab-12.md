### Added
- `repohardenguard` now checks a GitLab project: the hardening-read operation serves five
  GitLab kinds — `project`, `protected-branches`, `protected-tags`, `push-rules`,
  `approvals` — each a fixed endpoint returning GitLab's own settings document (the two lists
  are walked page by page and refuse at the ceiling rather than hand back a partial array).
  A Premium-only route on Community Edition (`push-rules`, and `approvals` on some
  self-managed instances) arrives as a typed could-not-check naming the tier, never an empty
  document; the GitHub backend refuses the GitLab kinds by name and GitLab refuses the GitHub
  kinds, with zero requests either way. The guard's preflight reads the resolved forge's own
  document (`project` on GitLab) instead of the GitHub `repo` kind, and a checklist Field may
  index into a list (`push_access_levels.0.access_level`).
- `docs/adopting-assay-gitlab.md` §5a: the GitLab hardening-checklist template (Community
  Edition rows, the `not available — Premium` / `— Ultimate` divergences, the Premium and
  Ultimate swaps) and the auditor's minimum project role per kind.
