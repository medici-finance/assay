### Fixed
- `skillslint --sync` (`make guardrail-sync`) now rewrites a guardrail copy only
  when the lines at its anchor match a known text of that block byte-for-byte:
  either the current canonical text (already synced, so no write) or its text in
  an earlier committed or staged revision of `.claude/guardrails/GUARDRAILS.md`.
  It removes exactly the matched lines. A copy it cannot match, such as a
  hand-edited one or one in a tree with no git history, is reported as
  could-not-check and left alone. Before this change a grown block swallowed
  trailing content that was never part of it (the #1687 incident), a shrunk block
  left stale lines behind, and a second sync or a sync after committing the source
  edit did the same (#1690).
