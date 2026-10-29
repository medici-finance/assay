### Changed
- The declared-scope parser behind `deskevidence --outcome-record` keeps refusing every `## Context`
  line that starts with "files" followed by whitespace and is not the accepted `files:` form,
  hard-wrapped prose included, at both the Context-line and the fenced-region marker checks. New
  regression tests pin that refusal. A brief refused this way is fixed by reflowing the paragraph so
  no line starts with "files", not by loosening the parser.
