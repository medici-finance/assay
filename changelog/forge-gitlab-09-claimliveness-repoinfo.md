### Added
- **`claimLiveness` is now a typed Forge op with a GitLab backend (#798).** The
  model-capability floor's stamp age-out read a PR's dispatch-claim ref through a GitHub-only
  hand-rolled REST call; it is now the enumerated `Forge.RefExists(repo, ref)` op, with the
  GitHub logic moved onto the seam and a GitLab backend that reads the ref through the Branches
  API (Free tier). Like `DeleteRef`, it validates the ref path before any request and reaches
  only the `heads/` namespace on GitLab CE — a ref outside it is a could-not-check refusal, never
  a guessed "absent" that would report a held claim as released. A 404 is the answer "absent"; a
  403 stays could-not-check.
- **The public-repo trust gate can now run on a GitLab-resolved repo (#798).** A new
  `GitLabRepoInfoFetcher` adapter exposes the GitLab backend's visibility and reaction reads
  under the string-signature `RepoInfoFetcher` the gate consumes, so `PublicRepoGate` enforces
  the same +1-from-an-authorized-human requirement on GitLab that it does on GitHub — reusing the
  backend's existing, tested reads, adding no fall-open path (an unreadable visibility still fails
  closed).

Both unblock deferred blockers named in #800 for the GitLab deskpost-verdict wiring; that wiring
itself remains a separate follow-up. The `RepoInfoFetcher` change touches the public-repo
security gate and needs a separate Security-Review.
