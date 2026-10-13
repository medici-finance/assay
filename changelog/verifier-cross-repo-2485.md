### Fixed

- Bind cross-repository verifier runs to separate immutable tracking and deliverable sources across dispatch packets, pre-work admission, execution, Evidence/outcome landing and same-record recovery (#2485). Tracking content stays outside the deliverable tree; failed outcomes bind a repository-qualified deliverable commit for offline wake evaluation. Same-repository admission and the existing source, plan, model and publication checks remain in force.
