### Fixed
- `deskpr create`/`update`/`edit` now ask the public-repo gate's visibility read through the
  SAME resolved forge backend used for every other operation on the change, instead of a
  hardcoded GitHub-only client — a GitLab-resolved repo's visibility is now read from GitLab's
  own API rather than failing closed on a GitHub 401 for a project GitHub has never heard of
  (#1054).
