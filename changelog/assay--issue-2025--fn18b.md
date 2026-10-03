### Fixed
- A plain `statusgen --lint` (no `--forge`) no longer reaches the forge: before this change it ran the claim read (`git ls-remote --heads origin`) and dead-claim decay (`gh pr list`, or the GitLab API). Offline, the claim set now reads could-not-check, and `--require-claims` fails closed on it. The offline-lint test now records every forge/network process and HTTP request, so a read that starts and then fails is caught.

### Changed
- `statusgen --lint` starts about 70% fewer git processes (276 to 79 on this repository) with identical lint output: blob reads share one `git cat-file --batch` per run, repeated merge-base questions are memoised, and the per-flag `git log -S` age check is one `git log -p`.
- `statusgen/bench/lintbench.sh` generates a 400+ brief fixture and times an offline `--lint` for two binaries, best of three.
