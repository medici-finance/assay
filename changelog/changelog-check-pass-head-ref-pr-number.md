### Changed
- `changelog-check` now passes `HEAD_REF` and `PR_NUMBER` to `check.sh`: a red run names the exact `changelog/<branch>.md` to create instead of the literal `<slug>`, and the proxy-fragment path (`changelog/pr-<N>-<slug>.md` on the base branch greens PR N) is live in production.
