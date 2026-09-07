### Added
- **Activate the staged CI workflows.** Promote two workflows from `ci/staged-workflows/`
  into `.github/workflows/`: the `evidence-automerge` leg (with the tolerant
  auto-merge-not-allowed skip, #579) and the `windows-ci-leg` — the first check in this repo
  to run on a Windows runner, asserting `statusgen --lint` exits 0 and an offline `--version`
  smoke passes on `windows-latest` (windows-port/04). The Windows leg runs against an LF
  checkout via the repo `.gitattributes` (#584). (#583)
