### Added

- The forge change library reports the author and draft flag of a change, lists open changes as rows with author, draft flag, state and head in one read (GitLab rows make no pipeline reads), and exposes the incremental updated-changes read with its watermark coverage. Incremental change records now carry the draft flag on GitHub and GitLab.
