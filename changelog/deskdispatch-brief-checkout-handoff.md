### Fixed

- A worker resuming its own brief as a pull request can keep the registered checkout, staged edits and in-progress merge. Dispatch confirms the earlier session, delivery and claims and records a new run for the pull request.
