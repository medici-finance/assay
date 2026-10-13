### Fixed
- `cellctl desk` no longer fails a routine multi-role boot with a bare "advisory lock is held by another process". The cell-wide scratch lock that sibling launches share now waits, with backoff, up to two minutes for a sibling's sweep instead of failing at once; the shared Go cache keeps its one-minute wait with backoff.
- When a launch lock stays busy, the error names the lock and its path, says the wait was bounded and retryable, names the holder (pid, command, cell, role) or says "unknown", says whether the holder is current, and states the action taken or available.

### Added
- Lock holders write an owner record beside the lock. A waiter checks it against the live process (pid alive, start time matches, same command, holds the file) before trusting it. A record naming a dead or reused pid reports the holder as unknown and is never acted on.
- `cellctl desk --end-stale-lock-holder` sends SIGTERM to a role-lease holder only when it is proven to be this cell's own `cellctl` for the same role and its role child has exited. The launch then waits a bounded time and re-checks the lock. SIGKILL is never sent, a current holder is a correct refusal, and on Windows a holder is reported but never ended.
