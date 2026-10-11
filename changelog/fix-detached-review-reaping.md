### Fixed
- The opt-in dead-session worktree reaper recognizes detached review heads preserved by remote refs after squash merges, requiring a named inactive owner and retaining unpublished commits and local review evidence.
- Combining stale-lock reclamation with dead-session reaping is refused before mutation so named owner evidence is retained.
