---
stream: serial
status: active
priority: P0
track: platform
max-concurrent: 1
---

# Serial

A stream whose briefs edit the same files, so they must SERIALIZE. The
`max-concurrent: 1` above is the declaration the board reads; without it the
serialization mandate would exist only as prose while the board offered both
briefs at once.

| # | Brief | Wave | Status | Verified | Reviewed |
|---|-------|------|--------|----------|----------|
| 01 | [First](brief-01.md) | 0 | todo | — | — |
| 02 | [Second](brief-02.md) | 0 | todo | — | — |
