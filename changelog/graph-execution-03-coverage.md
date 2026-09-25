### Added
- `statusgen --coverage [--json]` — the evidence coverage rule (graph-execution/03):
  for every brief, the union of its own Verify rows and (when it is bound to a
  workflow-pattern-v1 node) that node's mandatory evidence must each resolve `pass`
  at the item's revision before the brief is `released`; a missing, errored,
  could-not-check, wrong-revision, or failing claim holds it, with the reason.
- The `observe` evidence kind (`spec/workflow-pattern-v1.md`, `schemas/workflow-pattern-v1.json`):
  a signal watched over a window after a change lands, declared only where a deploy
  exists.
- `statusgen --auto-flip-model` refuses the `verified` → `done` flip when a brief's
  evidence coverage is not released — checked offline, before any live review fetch.

### Changed
- `spec/lifecycle-v1.md` §2.4: `verified` now additionally requires coverage to be
  `released`.
