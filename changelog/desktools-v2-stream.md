### Added
- New **desktools-v2** stream (proposed — `status: parked`, citing a `**Status:** draft`
  scoping doc): the architectural rebuild of the desk tools' forge access, on three first-class
  principles — **custody** (explicit minted-token only, key-presence as the custody boundary,
  the desktop made to behave like a locked container), **the read path covers statusgen**
  (promote `deskkit` to an importable shared library, then migrate statusgen's `gh`-shelling
  reads onto it under minted custody — the `scanloop`-in-container fix), and **purpose-built
  queries** (typed access-pattern operations, one tuned query per backend: N+1 → one consistent
  snapshot). Scoping doc plus nine briefs in three waves; ban-lint extended to cover
  `statusgen/**`. Authored-only; no tool changed. Boundaries with `forge-neutral`,
  `desktools-go-git`, and `desk-tools` are stated in the scoping doc.
