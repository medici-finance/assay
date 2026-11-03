### Added
- `statusgen` builds the combined milestone and brief dependency graph (`evaluation.BuildMilestoneGraph`). Briefs point to the briefs they depend on and to the milestone versions they require; milestones point to their contributors and prerequisite milestones. Membership is derived and counted once per canonical brief. A pinned contributor revision that is not the observed one stays unknown and never binds another revision. Withdrawn work never satisfies a milestone, and only `done` completes one.
- A cycle through a milestone is a lint PROBLEM. This covers a brief requiring its own containing milestone and an indirect cycle across roots.
- A logical brief key claimed by more than one scanned brief (two roots resolving to one cell and alias) never picks a winner: every claimant's start requirements are held could-not-check, naming the collision. A milestone version declared by more than one loaded record stays unknown.
- A milestone's transitive blockers and unknowns are carried once each, so a lattice of shared prerequisites stays linear in the records. A start gate's hold reason lists at most 20 entries and counts the rest.

### Changed
- A `brief-v4` `requires-milestones` entry no longer holds the brief as could-not-check regardless of the graph. It now holds the brief as unsatisfied when the milestone has known remaining work (the notice names the blockers) or forms a cycle with it, and as could-not-check when something is unknown. It is never satisfied, because milestone acceptance is not observed yet. Only the declaring brief is held: a milestone's prerequisites are never inherited by its contributors.
- Eligibility edges can carry a pre-resolved state, which the evaluator uses as given. The evaluator version is now `statusgen-evaluation/5`.
