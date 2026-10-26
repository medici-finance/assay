### Fixed
- `statusgen --dora-json`: the `change_failure_rate` numerator now counts the defect records dated inside the stated window, resolved or not, instead of every currently-open record. Both sides of the ratio are now windowed, so it no longer grows with the open backlog. The sample key `open_defect_notes` is renamed `defect_notes_in_window`.
