### Fixed
- desk-supervision/24's Verify table no longer depends on whole-repository or ambient state: the register-skip row lints a scratch fixture instead of the whole tree, the statusgen rows build the binary from the tree under test into a fresh temporary directory, and the live-forge mergeability row is marked `gate:human` (production-only).
