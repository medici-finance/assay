### Fixed
- The `verify-runner` image now ships `jq`, `shellcheck` and `tmux` as pinned Debian bookworm packages, so Verify rows that call them run in the offline container lane instead of failing before they reach the code under test (#4684, #4318). The release capability check runs each tool and stops the release if one is missing.
