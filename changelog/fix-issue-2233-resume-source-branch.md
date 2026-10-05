### Fixed
- Worker resume dispatches now use the open change’s source branch and verified head on GitHub and GitLab, refusing uncertain sources instead of falling back to main.
- The worktree allocator supports a pinned commit plus an explicit remote upstream, creating the checkout and branch tracking separately and rolling back if tracking fails.
