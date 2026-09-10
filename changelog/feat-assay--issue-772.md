### Fixed
- **`deskpost` and `deskflip` resolve the repo's forge BEFORE minting a GitHub App token, so
  a GitLab adopter no longer hits a misleading `set REVIEWER_APP_ID` error.** Both verbs used
  to mint a GitHub App installation token unconditionally as their first step, so on a
  GitLab-configured repo (`ASSAY_REPO_FORGES=…=gitlab`, authenticating with a PAT file rather
  than an App PEM) a review verdict or ready-flip died with `no App ID for role "reviewer":
  set REVIEWER_APP_ID` — a GitHub credential error that sent the operator hunting `apps.env`
  for a credential the GitLab lane never uses (medici-finance/assay#772).
  - `deskflip` now authenticates a GitLab flip from the GitLab PAT custody path (via the forge
    resolver) instead of the GitHub App mint; its reads and writes were already forge-neutral,
    so this was the last GitHub-shaped step in the flip.
  - `deskpost` (review / security-review / comment / ready) now fails **closed** with an honest
    could-not-check that names the resolved forge — its verdict/flip precondition reads still
    run through a GitHub App-authenticated client, so the GitLab write path is the follow-up —
    rather than the GitHub App-ID mint error.
  - A forge that cannot be **positively** resolved (no roster binding and no known origin host)
    is treated as could-not-check, never as GitLab, so every GitHub repo keeps its exact prior
    behaviour. Part of the GitLab-adopter portability family (siblings #773–#776).
