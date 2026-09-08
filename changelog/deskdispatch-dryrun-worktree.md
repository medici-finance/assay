### Added
- `deskdispatch --dry-run --worktree <path>` renders the previewed prompt against an
  operator-stated home worktree that already exists, at both placeholder sites, instead of
  the not-yet-known placeholder — retiring the by-hand substitution operators ran over
  dry-run prompt batches. The path is validated first, all three checks fail-closed
  (exit 5): it resolves under a sanctioned worktree prefix, it IS a registered git worktree
  of the item's own repo, and it is not the shared checkout. The flag is refused (exit 5) on
  a real dispatch, where the home is `deskwt`'s to name, and a verified path is echoed on the
  PLAN banner as `operator-supplied, verified` so a transcript shows it was checked, not guessed.
