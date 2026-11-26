### Fixed
- Protected verification can now read private approval records when opening and closing a container run. Each host step obtains and releases its own repository-scoped, read-only verifier credential. The verifier checks the assignment again before reading approvals at close.
