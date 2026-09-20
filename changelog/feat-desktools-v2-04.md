### Fixed
- `deskclose` derives the authorizing comment's kind from its own permalink (`/pull/` → change,
  `/issues/` → issue, falling back to change only on could-not-check) instead of always reading
  the change/pull-request thread — a human ruling recorded on an ISSUE now authorizes instead of
  coming back could-not-check every time (#1019).
