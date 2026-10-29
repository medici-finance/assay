### Fixed
- `deskevidence --outcome-record` no longer refuses a brief whose `## Context` prose hard-wraps a
  plain sentence onto a line starting with "files " — only when the previous line is unfinished
  prose, the next line is prose or blank, and the "files" line carries no path-shaped word, colon
  or backtick. Anything else starting with "files", including a missing-colon declaration such as
  `files tools/a.go`, is still refused.
