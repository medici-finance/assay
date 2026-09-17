### Added
- New **desktools-v2** stream (proposed — `status: parked`, citing a `**Status:** draft`
  scoping doc): the architectural rebuild of the desk tools' forge access, on three first-class
  principles — **custody** (explicit minted-token only, key-presence as the custody boundary,
  the desktop made to behave like a locked container), **the read path covers statusgen across
  the `deskread` verb boundary** (the migration stays with the sibling `forge-neutral` brief
  that owns it; this stream brings `statusgen/**` under the ban and then holds the zero), and
  **purpose-built queries** (typed access-pattern operations, one tuned query per backend:
  N+1 → one consistent snapshot).
- The same stream scopes **one outbound-write check at the forge write seam**, keyed on the
  target repository's configured visibility, with a deployment-supplied callout on the existing
  callout plumbing — so what may be written to a forge is enforced by the tools rather than by
  skill prose. The scoping doc tabulates which outward verb runs which check today.
- Scoping doc plus ten briefs. Authored-only; no tool changed. Boundaries with `forge-neutral`,
  `desktools-go-git`, and `desk-tools` are stated in the scoping doc.
