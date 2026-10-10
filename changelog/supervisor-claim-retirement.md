### Fixed
- Supervisor claim retirement compares the exact assessed claim version, preserving replacement holders, and reads immutable payloads from the active claim namespace. Live ticks use bounded pages with explicit continuation and hold malformed claims for inspection.
- Keep healthy claims and the standing supervisor running when a claim has an unknown owner, a future timestamp, or a refused stale release; report those claims blind. Already-released claims skip deletion.
