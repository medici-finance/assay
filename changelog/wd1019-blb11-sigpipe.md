### Fixed
- Verify rows 3 and 4 of the regression-test-ratchet brief (build-less-brittle/11) no longer fail with exit 141 under the verifier's pipefail shell: the first-match selection now reads its whole input (`sed -n 1p`) instead of closing the pipe early (`head -1`). The assertion and expected output are unchanged, and the brief's version is bumped by one.
