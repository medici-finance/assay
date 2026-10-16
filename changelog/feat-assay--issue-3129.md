### Fixed
- `desktoken` now publishes the GitHub installation-token cache atomically (temp file in the same directory at 0600, then rename), so a process reading the cache path while another re-mints can no longer see an empty or partial token and silently fall back to the host's stored login.
