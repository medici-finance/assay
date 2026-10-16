### Fixed
- The review-depth-by-tier brief's consumers-corroboration Verify row now pins the change that implemented the brief and that change's parent, so it reaches a real verdict after the merge. Before, it ran against the runner's own head, where a merged brief is no longer in the diff, and could only report could-not-check.
