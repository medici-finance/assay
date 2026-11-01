### Changed
- `fanoutloop plan` reuses the shared pending-change batch reader for each current candidate frontier, retaining the lazy first-delivery walk and complete native rename/copy ancestry fallback bound to the observed head and file count. Uncertain observations continue to count as representing PRs; trust, authoring, pagination and queue controls remain unchanged.
