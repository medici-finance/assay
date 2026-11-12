### Fixed
- The brief-v4 mutation manifest's milestone-gate case is re-anchored in `milestonegraph.go` and split into one case per path (held could-not-check and resolved), so a whole-manifest run of `04-mutate.py` completes with every case killed instead of stopping on a stale anchor.
