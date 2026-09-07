### Added
- Windows CI leg (`windows-port/04`): a staged `windows-latest` job
  (`ci/staged-workflows/windows-ci-leg.yml`) that installs Go, builds `statusgen.exe`,
  asserts `statusgen --lint` exits 0 on Windows, and runs an OFFLINE `--version` desk-verb
  smoke — the first check in the repo to run on a Windows runner. The native windows/arm64
  smoke is held BLOCKED pending a `windows-11-arm` runner (never inferred from the amd64
  result), and a `workflow_dispatch` `failfirst` input demonstrates the leg reddens on a
  broken input. Staged for maintainer promotion into `.github/workflows/` (no App holds
  workflow-push permission).
