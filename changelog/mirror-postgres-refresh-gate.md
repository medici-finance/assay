- Add the canonical PostgreSQL read-only refresh Gate and per-attempt authority, with shared generation fencing, transaction-bound observation completion, explicit finite accounting and conservative retry holds.

- Keep completion data time in an explicit trusted application clock domain, fence held owners on populated scopes, and document the lifetime reservation cap.
