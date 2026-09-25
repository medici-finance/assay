### Fixed
- The desk secret scan no longer refuses CamelCase identifiers that carry a plural acronym (`PRs`, `IDs`), a numeronym (`K8s`, `I18n`, `L10n`, `A11y`) or a closing acronym on top of one mid-run acronym, nor a git SHA assigned to an ALL-CAPS env var whose name ends in a closed list of digest keys (`SHA`, `SHA1`, `SHA256`, `DIGEST`, `CHECKSUM`, `COMMIT`, or `HEX` directly behind one, as in a Dockerfile `ARG BASE_DIGEST_HEX=<sha256>`) and contains no credential stem (#1642).

### Added
- The desk secret scan refuses GitLab `glpat-` access tokens (#1642).
- Worker kits and the `author-brief` skill ask for identifiers under 32 characters, and for long identifiers to be described rather than quoted in PR bodies (#1642).
