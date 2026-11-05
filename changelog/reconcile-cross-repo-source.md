### Fixed
- `statusgen reconcile` now says why it cannot read a run's brief version: the refusal carries git's own reason, and a cross-repository run with no `tracking-source=` is told that this commit is the one it must name (its `source=` sha is never re-pointed or read in the tracking repository).
