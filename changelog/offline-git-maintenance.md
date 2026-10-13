### Added
- `deskwt maintain objects` provides a read-only repository inventory and bounded offline maintenance. Worktree prune coalesces the same tasks daily per common Git directory, retaining every object and normal recovery history while reducing duplicate storage.
### Fixed
- Both offline maintenance commands preserve durable process custody after coordinator crashes or uncertain child cleanup. Pack-directory inventory refuses overflow before task admission.
- Desk merge fetches use individually owned temporary refs and compare-and-delete cleanup, preserving concurrently moved refs and live worktree holders.
- Desk merge verdicts no longer turn could-not-check when temporary-ref cleanup is refused by the hook policy; the retained ref is reported and left for maintenance. A worktree on an unborn branch no longer blocks temporary-ref cleanup.
- Temporary-ref recovery retains receipts for unreadable or malformed destinations, including after a failed compare-and-delete. Fixture packages isolate managed caches, and cleanup warnings distinguish retained receipts from confirmed producer completion.
