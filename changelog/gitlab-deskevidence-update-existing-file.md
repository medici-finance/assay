### Fixed
- `deskevidence` on a GitLab-hosted repo now lands Evidence into an **existing**
  brief. The GitLab file-write path probed the not-yet-created target branch for
  the file, always missed it, and issued a create (`POST`) that GitLab rejects
  `HTTP 400` when the path already exists on the base — so post-merge verify could
  produce a PASS but never land the Evidence row or the `implemented → verified`
  flip. The existence probe now reads the branch the write is based on (the start
  branch, for the inline side-branch lane), so an existing path is updated with a
  `PUT` and only a genuinely new path is created with a `POST`. (#1412)
