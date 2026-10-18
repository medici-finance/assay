### Fixed
- `deskwt prune` reuses mainline ancestry and shared-store reachability data across dead-session candidates, and inventories worktree admin pointers once per sweep. Mutable refs and selected admin pointers are checked again when read; ambiguous or unreadable registrations remain held.
