### Fixed
- `statusgen --lint` no longer reddens a brief for citing its own changelog
  fragment after a release consumed it. A backticked `changelog/<slug>.md` path
  resolves when the repo runs the fragment convention, has cut a release, and the
  fragment is present in git history — so the release that correctly clears
  `changelog/` no longer turns every brief that listed its fragment PROBLEM-red.
  A mistyped or never-committed fragment path is still reported, and a tree with
  no readable git history reports the problem as could-not-check rather than
  silently exempting the path.
