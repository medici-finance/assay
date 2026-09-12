### Added
- **The standing truth suite is live.** `truth-suite.yml` was promoted from `ci/staged-workflows/` to `.github/workflows/`: the baseline test corpus plus the release mutation gate now run on every push to the default branch and on a daily schedule, reporting three-state. Closes #740; completes sdlc/03 rows 6-7.
