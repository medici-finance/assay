### Added

- Desk tools can now resolve a cross-repository item's tracking and deliverable repositories with one shared resolver. A deliverable checkout no longer has to be a stream root: name it in `DESK_CHECKOUTS` (same `<owner>/<repo>=<path>` form as `DESK_ROOTS`), or place it beside the tracking checkout. Every answer is checked against the checkout's own origin.

### Fixed

- `deskdispatch` reads a homed-in brief's `decision-issue:` in the repository that tracks the brief. Before, it read the deliverable repository, so the pre-claim hold could miss an open, unruled decision.
- The verifier association and the planner's wake reader find a deliverable repository that has no `docs/streams`. Before, the verifier association stopped at "verifier deliverable root is not configured" and the item stayed could-not-check.
