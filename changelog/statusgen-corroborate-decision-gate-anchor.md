### Added
- `statusgen --corroborate`: a THIRD accepted corroboration anchor for a `gate:human`
  `human:<name>` stamp, alongside the existing APPROVED-review and approval-comment
  anchors on the brief's own PR. A stamp now also corroborates through the
  needs-decision channel — but ONLY when all of the following hold together: a
  needs-decision issue is CLOSED by the blessed human (`ASSAY_BLESS_LOGIN`), that
  issue carries the per-brief marker `<!-- decision-gate: <stream>/<NN> -->` naming
  THIS exact brief, AND the brief LINKS that issue. This aligns the lint with the
  sanctioned ratification channel — a `gate:human` decision recorded by closing its
  needs-decision issue rather than as a PR approval. The two PR anchors are
  unchanged; the new path is additive and fails closed (no bless login, no link,
  wrong closer, an open issue, or a marker for another brief all leave the stamp
  MISSING-CORROBORATION).
