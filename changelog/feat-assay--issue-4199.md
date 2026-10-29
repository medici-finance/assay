### Fixed
- `deskevidence --outcome-record` no longer refuses a brief whose `## Context` prose wraps onto a
  line starting with "files " followed by ordinary words; a missing-colon declaration such as
  `files tools/a.go` is still refused.
