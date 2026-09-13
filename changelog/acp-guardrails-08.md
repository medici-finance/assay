### Added
- `commsloop` gains an executor dispatch leg (`dispatch_native.go`, `roleprofile.go`):
  behind a `Native` flag (zero value = today's interim behavior, the rollback position —
  no production call site sets it), a dispatchable tier reaching `Dispatch` fires a real,
  role-fenced ACP session instead of the interim placeholder result. Each target desk
  role (`the-desk`/`intake-desk`/`worker-desk`/`pr-review-desk`/`verify-desk`) gets its
  own compiled `PermissionPolicy` + `FileAccessPolicy`; a role outside that closed set
  refuses dispatch outright — there is no permissive default profile.
- Every spawn is gated by the kill switch (checked before each spawn, not once per drain
  cycle) and a per-hour firing budget (`deskkit.AllowWrite`, scoped per dispatch target);
  an exhausted budget fires zero sessions and leaves the item to the engine's own
  retry/backoff rather than fabricating a result. Every spawn and every permission
  decision is one audit line, for the daily lane-violation sweep.
