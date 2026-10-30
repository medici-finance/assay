### Fixed
- `clusterguard`: a read-only-tier allowlist entry no longer admits a flag that mutates the cluster. Allowlisted verbs can now be flag-restricted: for such a verb the read-only tier refuses (exit 5) any flag it does not list, written before or after the verb, and any positional or empty argument.
