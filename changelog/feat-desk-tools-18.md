### Fixed
- The `pr-review-desk` skill no longer lists a per-item public-repo `+1` reaction among the human gates before a verdict post. That check was retired when the public-repo write gate became repository-scoped: an explicit `:public` allowed-repos entry that agrees with the live visibility. The `deskpr` trailer parser's doc comment no longer says the trailer's issue number feeds that gate.
