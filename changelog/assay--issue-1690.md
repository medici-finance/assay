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
  unrelated content, possibly a local, site-specific rule, that happens to
  equal the earlier text's own tail; the bytes alone cannot tell — `--sync` now
  **refuses by default**: could-not-check, naming the file and the exact
  line-range span the longest-match rule would have removed, nothing written.
  An earlier version of this fix took the longest match anyway and only
  printed a `note:`, and claimed `git add` right after each sync closed the
  window; neither held up (the tie comes from committed history, not from
  anything staging affects, and the same note fired on every ordinary edit
  too). The longest-match rewrite is still reachable, deliberately, via the
  new `--allow-ambiguous-extent` flag, and is still recorded as a `note:` when
  taken that way. This default (refuse, with an explicit opt-in) is this
  project's own choice among the review's options, not a settled
  cross-project ruling. A shrink of an uncommitted, unstaged edit that is
  itself trimmed again before ever being committed or staged is still
  could-not-check only when nothing at the anchor matches (README §4).
- `--sync` used to prove a copy's removal extent against one read of the site
  file, then splice that proof into a second, later read taken only at write
  time, with no check that the two reads still agreed, and wrote in place
  (truncate-then-write). Concurrent `--sync` runs could corrupt a file this
  way. Each site file is now read at most once per run, that same read backs
  both the proof and the write, the file is re-read and compared against it
  immediately before writing (refusing rather than guessing if it changed),
  and the write itself goes through a temp file plus atomic rename.
