### Changed
- `verifyloop plan` no longer wakes a held verify receipt when the tool version is the only input that changed; the hold reason names the tool-only change. A tool change together with any other changed input still wakes, and unreadable inputs stay could-not-check. Use an explicit recheck to re-run after a release. (#2310)
