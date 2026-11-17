### Fixed
- `deskpr`'s branch-diff secret-scan refusal now names the file, the line in that file and the side (`<path>:<line> (added|removed|context)`), and `--explain` adds `path=` and `side=`, instead of a line number counted inside the stripped diff text that matched no line anyone could open. What the scan refuses is unchanged.
- A path that carries control or bidi characters is printed Go-quoted in that refusal and in `--explain`, and the location is added only after the refusal has been classified, so a file name cannot change which override path a refusal takes.
