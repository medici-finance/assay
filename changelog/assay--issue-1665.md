### Fixed
- `deskroster liveness` no longer reports every trusted bot login as DELETED: it now probes a bot identity at its `"<slug>[bot]"` REST rendering, matching how GitHub's `GET /users/{login}` actually resolves a GitHub App's bot account (#1665).
