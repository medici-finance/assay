### Fixed
- `fanoutloop plan` and `verifyloop plan` now print the reason on stderr when they refuse. Before, both exited non-zero with no explanation: a missing `STATUS.md` exited 6 and printed nothing that said "cannot read the Next-up queue". Exit codes are unchanged.
