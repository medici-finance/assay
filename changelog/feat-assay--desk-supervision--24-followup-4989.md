### Fixed
- desk-supervision/24's Verify table no longer depends on whole-repository or ambient state: the register-skip row lints a scratch fixture instead of the whole tree, and the statusgen rows build the binary from the tree under test into a fresh temporary directory. The live-forge mergeability row keeps its `check` class and now says a network-off lane cannot run it.
