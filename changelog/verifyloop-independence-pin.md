### Changed
- `verifyloop` now has a regression test for its independence rule on pending Evidence. If the verifier App wrote the Evidence change for a brief that also names that App as its implementer, the brief is reported `could-not-check` instead of being held as `awaiting-merge`.
