### Added
- `statusgen init` now detects the target's forge from its `origin` remote (or an explicit `--forge github|gitlab`) and scaffolds the matching CI half: a GitLab remote gets a `.gitlab-ci.yml` running the same two single-writer halves (lint on merge requests, board regen + commit on the default branch) instead of the inert `.github/workflows/assay-statusgen.yml`, and the closing next-steps text names whichever file was actually written (#349).

### Fixed
- The dead-claim decay pass no longer reports its GitHub-only nature as a transient "unavailable this run" NOTICE on a GitLab remote. On a definitively non-GitHub (`gitlab`) `origin` it emits a distinct "NOT APPLICABLE on this forge" message and skips the `gh` shell-out entirely, so a GitLab adopter's lint no longer reads green while implying a CLI authentication that would never help (#349).
