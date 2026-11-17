### Fixed
- `deskpr`'s branch-diff secret-scan refusal now names the file, the line in that file and the side (`<path>:<line> (added|removed|context)`), and `--explain` adds `path=` and `side=`, instead of a line number counted inside the stripped diff text that matched no line anyone could open. What the scan refuses is unchanged.
