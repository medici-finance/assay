### Fixed
- Statusgen modes that hydrate brief frontmatter (`--gate-scores`, `--eligibility`, `--launch`, `--next-up`) no longer compute the git-history stale-FAIL notice for every waiting brief, work whose result they discard. On this repository's tree `--gate-scores` drops from about 15 s to under half a second with byte-identical output; `--lint` and the STATUS build still compute the notice.

### Added
- A hydration budget test with a planted fixture that proves it can fail, a profile harness (`ASSAY_HYDRATE_TREE`), and a git-backed `real-shape` profile in the synthetic workload so the benchmark would have caught this cost.
