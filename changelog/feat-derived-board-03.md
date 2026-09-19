### Fixed
- `derived-board/03`'s Verify table rows 4 and 5 re-baselined against current `statusgen`
  behaviour (assay#1305): row 4's id lookup now matches the post-flag-day hierarchical
  `id` shape (`assay:assay:derived-board:02`) instead of the retired flat form, and row 5's
  expected `--lint` substring now matches the `gates:` eligibility-evaluator NOTICE that
  superseded the old "reserved, not gating" wording. No `statusgen/` engine code changed.
