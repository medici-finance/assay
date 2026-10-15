### Fixed
- The scrubbed-cell cellctl suite no longer calls the host's installed `deskwt` (and so no longer writes refusal rows to the operator's desk audit log): it forces cellctl's own worktree path, shadows `deskwt` on its PATH with a recording stub, and asserts after every case that the stub was never called.
