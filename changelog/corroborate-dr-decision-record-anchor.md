### Added
- **`statusgen --corroborate` now accepts a decision record (`DR-<slug>.md`) through the
  decision-issue anchor.** The third human-stamp corroboration anchor — a linked, blessed-human-CLOSED
  `needs-decision` issue carrying the per-record `<!-- decision-gate: <id> -->` marker — previously
  fired only for a `human:<name>` stamp found in a `brief-<NN>.md` file. It now fires for a stamp in a
  `DR-<slug>.md` design-decision record under `docs/streams/decisions/` too, corroborating the
  record's `decided-by:` name against the CLOSER of the decision issue the record links. A DR's
  approving human ratifies by closing the `needs-decision` issue, not by signing the DR's own PR, so a
  concrete `decided-by: "human:<name>"` on a DR used to come back MISSING-CORROBORATION and the only
  sanctioned notation was the literal `human:<name>` placeholder that names nobody. The addition is
  strictly ADDITIVE: the two PR anchors (an APPROVED review, an explicit approval comment) and the
  original brief-file anchor are unchanged, and all three of the anchor's conditions — closed by the
  blessed login, marker naming THIS exact record, and the record linking the issue — remain
  independently required. The two record-id namespaces never collide (a brief id carries a `/`, a DR
  id never does).
