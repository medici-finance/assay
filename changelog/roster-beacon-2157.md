## Fixes
- Refuse roster-beacon and lock leaf symlinks, Windows reparse points and non-regular files; use strict beacon parsing for supervision resource reads.
- Keep valid roster table rows visible when a beacon read or prune fails, return exit 6 and label uncertain ownership explicitly.
- Document preservation-first retention and quiescent archive requirements without deleting receipts, stable locks or abandoned publication files.
