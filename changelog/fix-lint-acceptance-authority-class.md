### Fixed
- `deskevidence` no longer refuses a verified outcome record because of a lint PROBLEM that names a different brief. Such PROBLEMs are printed as notices. A PROBLEM that names the brief being closed, or names no brief, still refuses, so a protected-table PROBLEM still blocks its own brief's outcome (#4823).
