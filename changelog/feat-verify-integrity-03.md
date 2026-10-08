### Added
- `statusgen verifyrun --fail-first [--base <rev>]` runs each risk-bearing Verify row twice. It first runs the row on the merge-base, checked out in a temporary worktree, then at head. The witness gains a `Base` column (`base=<sha> red|green|unproven · head=<sha>`). A risk-bearing row that is green at base and passes at head is `non-discriminating` and folds the exit code to 1.
- New lint gate for a `verified`/`done` closure that the branch makes: a risk-bearing row is a PROBLEM when it has no fail-first witness, its witness is `unproven`, it is non-discriminating, or its witness names a base that is not an ancestor of HEAD. A non-risk-bearing row that is non-discriminating is a NOTICE.
- Three new advisory Verify-row strength rules, all NOTICE only: R11 `trivially-green`, R12 `no-output-assertion` and R13 `table-touches-no-files`. When `files:` cannot be parsed, R13 reports could-not-check.
- `docs/verify-row-strength.md` lists the thirteen rules, each with the incident that earned it.
- The verify-desk skill gains the fail-first step.

### Changed
- The unfailable-row rules R1–R10 are now a lint PROBLEM on a closure the branch makes (verified/done, and not so at the merge-base). Every other brief, including closures already on main, keeps the NOTICE. When `origin/main` cannot be resolved, the gate runs degraded and says so.
