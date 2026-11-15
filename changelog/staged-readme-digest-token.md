### Fixed
- `ci/staged-workflows/README.md` no longer says the reconcile job's sign-off digest step holds no token. It now states the step's one credential: the job's own read-only `github.token`, passed as `GITHUB_TOKEN` for `--acceptance-forge`, never an App token.
