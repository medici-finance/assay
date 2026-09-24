### Fixed
- The desk secret scan no longer refuses CamelCase identifiers that carry a plural acronym (`PRs`, `IDs`), a numeronym (`K8s`, `I18n`, `L10n`, `A11y`) or a closing acronym on top of one mid-run acronym, nor a Dockerfile `ARG …_HEX=<sha256>` line whose ALL-CAPS key sits in front of an already-exempt value (#1642).

### Added
- The desk secret scan refuses GitLab `glpat-` access tokens (#1642).
- Worker kits and the `author-brief` skill ask for identifiers under 32 characters, and for long identifiers to be described rather than quoted in PR bodies (#1642).
