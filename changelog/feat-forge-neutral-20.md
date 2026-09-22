### Changed
- `docs/streams/forge-neutral/reviewer-write-boundary.md` §3.1 gains a reviewer-role forge-write
  inventory (repository write is the dispatch claim only — every other reviewer site is read,
  PR write or issue write) and a claim-reader inventory covering every site that reads, lists
  or releases a dispatch claim outside the claim tool. §3.3's S2 and S4 rows move from
  could-not-check to measured results: the local-disk race probe is established (exactly one
  of 16 winners), a network filesystem could not be reached under this agent's isolation floor
  and stays could-not-check, and filesystem-type / container detection are measured on darwin
  and (via a local container) on linux.
