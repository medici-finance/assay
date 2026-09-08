### Added
- `statusgen --corroborate`: a THIRD accepted corroboration anchor for a `gate:human`
  `human:<name>` stamp, alongside the existing APPROVED-review and approval-comment
  anchors on the brief's own PR. A stamp now also corroborates when a needs-decision
  issue the brief LINKS was CLOSED by the blessed human (`ASSAY_BLESS_LOGIN`) and
  carries the per-brief marker `<!-- decision-gate: <stream>/<NN> -->` naming that
  exact brief. This aligns the lint with the sanctioned ratification channel — a
  decision recorded by closing its needs-decision issue rather than as a PR approval
  (tracker ruling #2237). The two PR anchors are unchanged; the new path is additive
  and fails closed (no bless login, no link, wrong closer, or a marker for another
  brief all leave the stamp MISSING-CORROBORATION).
