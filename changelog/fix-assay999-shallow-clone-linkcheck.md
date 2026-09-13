### Fixed
- `consumedFragmentIndex.build()` (the `changelog/<slug>.md` consumed-release-fragment
  exemption, #722) now detects a shallow git clone (`git rev-parse --is-shallow-repository`)
  and treats it the same as a `git log` error, instead of reading its truncated history as a
  definitive "never tracked" answer. On CI's default shallow `actions/checkout`, that false
  read spuriously PROBLEMs a genuinely consumed changelog fragment on every PR, unrelated to
  the diff — this makes the failure an honest could-not-check instead of a silent wrong
  verdict. The companion fix — giving the `lint` and `windows-smoke` jobs `fetch-depth: 0` so
  CI actually greens on a real clone — is tracked separately, blocked on the worker App's
  standing no-workflow-write policy. (#999)
