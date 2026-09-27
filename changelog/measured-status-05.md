### Changed
- Bookkeeping only: the same-identity attribution hard-reject (a brief whose
  authoring commit and every commit that touched its Evidence section share one
  git identity now fails the committer-identity cross-check as a hard PROBLEM
  instead of a NOTICE) is already announced under `CHANGELOG.md`'s `v1.0.24`
  entry for `statusgen`'s git-committer-identity cross-check. This fragment adds
  no new behavior; it exists only because this PR's own `Brief:` trailer is the
  board's delivery witness for that landed change.
- Corrected framing (review F2): merging this PR does **not** flip board row
  `measured-status/05` from `todo` to `implemented`. The push-to-main regen job
  (`.github/workflows/assay-statusgen.yml`) only runs `statusgen --root .` and
  commits the regenerated `STATUS.md`; it never invokes `reconcile`. The
  reconcile step that derives a brief's lifecycle cell from its `Brief:`-trailer
  PR history is staged, not applied — `.github/assay-statusgen.reconcile.patch`.
  The row moves to `implemented` in a follow-up, once that reconcile step is
  wired into CI (or via the carve-out B same-repo path), using this PR's merged
  `Brief:` trailer as the witness.
