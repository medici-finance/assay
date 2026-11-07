### Fixed
- The release workflow's changelog roll is now pinned to the release tag: it aggregates and clears only the fragments present at the tag, so a fragment merged after the tag waits for the next cut instead of being filed under the wrong version and deleted. A retry that finds the version's section already rolled is treated as done.
