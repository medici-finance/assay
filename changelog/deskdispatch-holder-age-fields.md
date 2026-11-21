### Fixed

- `deskdispatch` reports a held claim using its current age, including after a previous holder expired. A claim's note can no longer supply its age or state, whether the note text arrives as one word or split by a tab or newline, so a live claim is no longer reported stale or counted as a free worker slot.
- `deskclaim-ref` writes a claim note as a single word: every whitespace character in a `steal --reason` becomes `_`. Its own field reader, and the supervisor's owner reader, ignore a field that appears only after the note.
