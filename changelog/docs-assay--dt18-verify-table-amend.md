### Changed
- The desk-tools brief 18 Verify table can now pass in a verifier run: row 8 tests the gate and its call-site packages, each with its own timeout, instead of the full module suite; row 10 is the plain repo-wide lint that rule 51 credits, without the `; echo $?` that hid its result; and row 11's mutation names the rank comparison the public-repo write gate uses today.
