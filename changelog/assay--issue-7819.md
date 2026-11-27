### Fixed

- `muhar` schemata now engages when worker copies sit under a symlinked temp directory, as macOS's default one is. Each child process muhar starts in a directory now gets a `PWD` naming that directory, so the go command and the schemata overlay see the same path; before, the probe build failed with "outside main module" and every mutant rebuilt the suite.
