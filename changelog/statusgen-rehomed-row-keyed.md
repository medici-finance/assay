### Fixed
- **board-honesty `re-homed` phantom no longer fires on a stream re-homed INTO
  the repo.** The `NON-DISPATCHABLE (re-homed)` notice matched the word
  "re-homed" anywhere in a stream's README, so a stream re-homed *into* this repo
  — whose README describes its own arrival — had every LIVE `todo` row falsely
  flagged non-dispatchable, telling the dispatcher to skip real work. The class
  now keys on the ROW's own record: it fires only on a POINTER row (the README
  marks it re-homed AND the row's brief file is gone), never on the stream's
  history. A present brief file is the live-work signal that overrides the
  README narrative. (#581)
