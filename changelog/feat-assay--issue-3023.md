### Fixed
- statusgen claim decay no longer decays a branch whose per-branch pull-request window came back full with none open: an open pull request behind the window would be invisible, so the branch stays a live claim and the decay says so as a could-not-check. The window is raised from five to ten pull requests per branch.

### Added
- Tests and a mutation spec (`statusgen/claimdecay-mutations.json`) pin three refusals in the claim decay that no test reached: the newest closed pull request decides rather than the oldest, a page with no repository and no `errors` is refused, and the query window equals the one the corpse test treats as full.
