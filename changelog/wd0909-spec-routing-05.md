### Added
- `author-brief` skill now states the authoring-time consumer-routing rule: a brief-authoring PR
  declares future consumers but does not edit them, so a path the brief's own implementation will
  later touch routes to the deferred disposition (`follow-up <stream>/<NN>` at the brief itself),
  never `fixed-here` — with a worked wrong/right example.

### Changed
- The `statusgen --consumers` gate's DISPROVED-`fixed-here` messages now name the deferred
  disposition as the fix, so an author whose authoring PR reddens the routing gate is pointed at
  the correct routing token instead of only being told the claim is contradicted.
