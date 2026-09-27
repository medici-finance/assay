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
- When more than one known length matches at the anchor and the longest is not
  the current canonical text — the copy could genuinely still be at that
  longer, earlier text, or it could already be at the current text followed by
  unrelated content that happens to equal the earlier text's own tail; the
  bytes alone cannot tell — the rewrite still applies the longest match (the
  ordinary case, needed so a still-unsynced copy at an earlier revision keeps
  working), but the tie is now reported as a `note:` on `--sync` rather than
  applied silently. A shrink of an uncommitted, unstaged edit that is itself
  trimmed again before ever being committed or staged is still could-not-check
  only when nothing at the anchor matches; `git add` right after each sync
  avoids both windows (README §4).
