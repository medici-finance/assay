### Fixed
- `docs/adopting-assay-gitlab.md` now documents `GITLAB_API_BASE` — the REST v4 base
  `deskboot` / `deskroster preflight` and `desktoken --forge gitlab <role>` require, with no
  fallback. Covers where/how to set it (a plain environment variable, never a `roster.env`
  key), the expected value shape for self-hosted vs. gitlab.com SaaS, and a short trade-off
  note on whether it should instead be a `roster.env` key.
