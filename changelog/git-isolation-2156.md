### Fixed
- The regression floor's hostile Git repository setup now disables automatic maintenance
  and garbage collection for its own setup commands. Isolation failures report every
  changed relative path, mode and content hash, including all `.git` metadata, without
  printing file bodies or symlink targets. Both shell and floor-runner checks still fail
  on any tree change.

The earlier intermittent CI failure remains unreproduced; detached maintenance was a
hypothesis, not a confirmed root cause. These changes remove that setup race candidate
and preserve evidence if a future failure has another cause.
