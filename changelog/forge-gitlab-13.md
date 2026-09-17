### Fixed
- `docs/streams/forge-gitlab/README.md`'s Briefs table lists brief 13 (the
  per-row degrade for `classifyPR`'s whole-sweep error returns) as `todo` even
  though its fix, regression tests, and changelog fragment already merged to
  `main` in #1084 — the board-honesty `already-merged-unflipped` class. This
  PR does not hand-flip the Status cell: that table's single writer is
  `statusgen`, never a hand-committed hunk. Neither of `statusgen`'s two write
  paths performs the flip today — `regen --readmes` preserves lifecycle cells
  by design rather than deriving them, and `reconcile --backfill --apply` (the
  verb that would write one) is not wired into this repo's CI — so the row
  stays `todo` pending that follow-up. (The PR's own title previously read
  "flip board row to implemented" — retitled to match: no flip happens here.)
