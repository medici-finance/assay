### Fixed
- `deskflip` and `deskpost ready` no longer refuse a PR whose reviewer verdict says `Undeclared-desk-decision: none` (or `none found` / `none observed` / `none identified`, any case, optionally followed by a separator and an explanation). A value that only begins with `none`, or an empty value, still holds the flip. The Evidence auto-merge workflow reads the line the same way.
