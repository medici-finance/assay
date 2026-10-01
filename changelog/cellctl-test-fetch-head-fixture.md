### Fixed
- The `cellctl` model-policy test suite passes again. Its offline stand-in for `git fetch` now writes a `FETCH_HEAD` the way a real fetch does, so the launcher's refusal to boot on a fetch that wrote none (#1853) no longer fails the launch test. A new check in the suite fails if any test's git stub exits from its fetch branch without writing `FETCH_HEAD`.
