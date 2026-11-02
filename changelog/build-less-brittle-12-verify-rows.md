### Changed
- The refactor-oracle brief's Verify rows now check their own counts and print one result marker each, so a verifier no longer reads a pass off the last number in the output. The three net-weight and consumers rows now find the delivering change wherever the branch commits landed, so they no longer fail on merged main.
