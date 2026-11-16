### Fixed
- `scanloop` records the `placeholder` exit only for a batch member whose placeholder the scan actually wrote. A member the scanner skipped (an excluded label, a hold) is no longer ledgered against a carrier that does not carry it: it gets no exit, no audit line and no intake-exit record, and the pass report names it under EMITTED with its `scanloop land` command, so its real disposition can still be recorded.
- A scan that writes nothing under the scan directory opens no carrier and no longer fails the dispatch on an empty commit.
- A member whose placeholder is excluded because another open scan carrier already carries it now names that carrier as its artifact, not the new carrier.
- `HasPlaceholder` stems a repo with the last path segment, as `statusgen --scan-issues` names the file, so an existing placeholder for a GitLab nested-group project (`group/sub/project`) is found and the issue is routed as an update instead of being sent through the scan again.
