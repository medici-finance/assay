### Fixed
- `cellctl` passes each generated role session and the cell's roster and credential home to worktree initialization. Deskd resolves the repository from the cell checkout and uses the cell's custody context instead of the launching shell's.
