### Changed
- Re-baselined Verify rows in four briefs (graph-execution/02, derived-board/03, measured-status/02, measured-status/04): the consumers checks are pinned to their delivering commits in a throwaway clone, the reconcile-fold mutation total now follows the spec (36), and two `grep -q` pipelines became `grep -c` so they hold under `pipefail`.
