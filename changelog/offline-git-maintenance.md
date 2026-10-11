### Added
- `deskwt maintain objects` provides a read-only repository inventory and bounded offline maintenance. Worktree prune coalesces the same tasks daily per common Git directory, retaining every object and normal recovery history while reducing duplicate storage.
### Fixed
- Both offline maintenance commands preserve durable process custody after coordinator crashes or uncertain child cleanup. Pack-directory inventory refuses overflow before task admission.
- Desk merge fetches use individually owned temporary refs and compare-and-delete cleanup, preserving concurrently moved refs and live worktree holders.
