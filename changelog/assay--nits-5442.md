### Fixed
- The GitLab board push no longer pins a `url.*.insteadOf` rewrite that points the board remote at a different scheme or host: the pin would have sent the board credential to that host. Such a push fails as the rewrite says and prints a NOTICE; a rewrite that keeps the destination's host is pinned as before (#5004).
- `statusgen migrate gitlab-anchors` refuses a `.gitlab-ci.yml` that is not a regular file (a symbolic link, for example) before reading it and again before writing it, exiting 5 with one line naming it, so the refresh never writes through a link (#5004).

### Changed
- The `statusgen --lint` drift NOTICE and the `statusgen init` STALE line now say the embedded anchor differs from the running statusgen's copy and name that statusgen's version, instead of saying the pipeline runs an older script. `statusgen --lint` also prints a NOTICE for a `.gitlab-ci.yml` scaffolded before the anchors existed (the `statusgen-regen` job with neither anchor key), naming a re-scaffold with `statusgen init --forge gitlab` as the remedy. The exit code is unchanged (#5004).
