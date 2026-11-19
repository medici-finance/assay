### Fixed
- `deskmigrate`'s refusal on an unmigrated brief-v1 tree now says what to do when the migrations are already vendored but the requested span excludes the brief-v1 to brief-v2 step: run `statusgen migrate brief-v1-to-v2 --root .` after writing `docs/streams/graph-repos.yaml`. The vendor-and-re-run hint stays for a tree with no `migrations/`. Exit codes are unchanged.
