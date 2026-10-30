### Fixed
- `clusterguard`: a read-only-tier allowlist entry no longer admits a flag that mutates the cluster. Allowlisted verbs can now be flag-restricted: after such a verb the read-only tier refuses (exit 5) any flag it does not list, and any positional argument.
