### Fixed
- Seven cellctl shell fixtures now isolate worktree launches from an installed `deskwt`, with shared recording/refusing stubs, real-launch positive controls and checks that run even when a case aborts.
- Deliberate house-cell worktree-tool tests explicitly select their owned stub while normal cases use cellctl's own worktree path.
