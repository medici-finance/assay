### Added
- The deskkit exit-code table (`ExitOK`/`ExitDisabled`/`ExitRateLimited`/`ExitRefused`/`ExitUnverifiable`) now carries its derivation of record — codes start at 3 to leave 1/2 to the shell's own error codes, with one code per refusal class — pinned by `TestExitCodeTableMatchesDerivation` and a mutation-spec entry that fails if a value drifts.
