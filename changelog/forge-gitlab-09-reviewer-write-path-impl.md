### Added
- **`desktoken` / `deskfile` reviewer-role (and every role) auth now works on a GitLab-resolved
  repo (#798 §2).** `desktoken <role> --repo <gitlab-slug>` with no explicit `--forge` now
  RESOLVES the forge from the repo and, on a definite GitLab resolution, takes the GitLab PAT
  custody path — instead of falling through to the GitHub App mint and dying with `no App ID for
  App "<role>-app"` (exit 6), the credential a PAT-backed GitLab bot never provisions (#772). So
  `deskfile check` on a GitLab repo reaches its dedupe search rather than a bare App-ID exit 6. A
  GitHub or could-not-check resolution still falls through to the App mint unchanged, and an
  absent custody PAT is a refusal — never an ambient-identity fallback.

### Changed
- **The GitLab reviewer verdict-WRITE control is now proven end-to-end (#798 §1).** The GitLab
  backend's `PostReview` mapping — approve → verdict note + `/approve`; request-changes →
  `/unapprove` + a head-SHA verdict note — is covered by a round-trip test: a verdict written
  through the Forge is read back by `ReviewsAtHead` at head (approve and request-changes both
  visible to the read path), and a permission/tier 403 on the write surfaces could-not-check
  rather than a clean or laundered verdict.
