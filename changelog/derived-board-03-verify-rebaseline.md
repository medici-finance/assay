### Changed
- derived-board/03 brief: Verify rows 4 and 5 re-baselined after three
  consecutive verifier cycles tripped on stale anchors — row 4's reconcile
  lookup is now id-shape-tolerant (ids went hierarchical
  `assay:assay:<stream>:<NN>` at the brief-v2 flag-day), and row 5 expects the
  post-#1251 eligibility-evaluator wording now that `gates:` is actively
  gating instead of reserved.
