### Fixed
- Resolve cell reset directory comparisons consistently across symbolic links. Hard reset refuses nested configuration, repositories, and comms storage; soft reset retains its owned-worktree selection through execution. Unusable directory paths fail closed while absent optional storage remains supported (#2718).
