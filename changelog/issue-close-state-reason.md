### Fixed
- `deskclose` now sends the REST `not_planned` state reason when closing superseded, duplicate or triaged issues, avoiding a validation failure after the closing comment has posted.
