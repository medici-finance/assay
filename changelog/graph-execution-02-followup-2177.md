### Changed
- Verify rows in graph-execution/02, derived-board/03 and measured-status/04 are re-baselined so each can run, and fail, on merged main: graph-execution/02 row 10 pins its consumers check to the delivering commit, derived-board/03 row 10 pins the exact mutation count it must catch, and measured-status/04 rows 2 and 4 hold under `pipefail`.
