---
stream: shared
status: active
priority: P1
repo: example-org/repo-a
mission:
  version: 1
  outcome: A returning reader can tell what this stream is for and how success is shown.
  success:
    - criterion: The acceptance report records a passing run at a pinned revision.
      evidence:
        - docs/evidence/acceptance.md
        - example-org/repo-a#12
    - criterion: The follow-up report is published.
      evidence:
        - {path: docs/evidence/follow-up.md, planned: true}
        - https://example.org/streams/shared
  commitments:
    - Keep the stream readable without the chat history.
  exclusions:
    - No overall progress percentage.
---

# Shared Stream — fixture stream with an authored mission

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | First brief | 0 | S | done | — | — |
| 02 | Second brief | 1 | M | in-progress | — | — |
