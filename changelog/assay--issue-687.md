### Fixed
- `deskfile` now REFUSES with a named, actionable message (exit 5) when the target repo's
  configured forge (`ASSAY_REPO_FORGES`) is GitLab, instead of shelling `gh` against GitHub
  and surfacing a misleading GraphQL "Could not resolve to a Repository" that reads like a
  token or typo problem. Its issue operations (dedupe search, label-existence probe, issue
  create, issue comment) are GitHub-only until they are routed through the forge backend; the
  refusal names the forge and tells the operator to escalate rather than route around the
  gate with a bare `glab`/`gh` call (#687).

### Added
- `deskkit.ForgeKindFor` resolves which forge serves a repo WITHOUT obtaining a credential or
  constructing a backend — the read a caller makes to branch on or refuse a forge while still
  filing under its own ambient identity, so `deskfile` can name the unsupported forge without
  taking on App-token custody (#687).
