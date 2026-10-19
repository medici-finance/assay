### Fixed
- Verification wake receipts parse declared bare and inline paths, expand directories, globs and finite braces, and preserve membership/absence so input changes cannot silently remain held. Unsupported declarations and incomplete historical scopes are explicit could-not-check/refusal states; original observation binding and historical receipts remain intact.

- Refuse wrapped unsupported prose and files fields outside Context explicitly, preserve quoted-colon continuations, and bound overlapping resolution work. Correct synthetic scheduler/Evidence fixture declarations without changing their assertions.
- Preserve annotated metadata fields, prose, trailing whitespace and CRLF; reject formatted markers, loose Markdown continuations, unknown boundary keys and malformed fences without losing declared inputs.
- Refuse ambiguous fence-shaped Context lines and honor opener info-string and indentation rules so inline spans and indented code cannot hide declared files; enforce the same indentation bound on closers.
- Refuse ambiguous list/HTML fence containment before it can hide a section or files marker; retain every reviewed example as a fail-closed parser/writer regression.
- Refuse indented section headings within Context, including after a files-list blank, instead of dropping later declared inputs.
- Refuse HTML comments and paused-list raw HTML that could conceal a section boundary and truncate scope.
