### Fixed
- The public-repo self-containment scan no longer refuses a body over a hyphenated compound that merely contains the scratch-worktree prefix word (a finding-block class label, say). A scratch worktree name still refuses at the start of the text, after whitespace or punctuation, and as a `/` path segment (#2080).
