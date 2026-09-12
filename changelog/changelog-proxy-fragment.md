### Added
- `changelog-check`: a notable fix from a pull request whose branch maintainers cannot commit to (a fork) can be recorded by a **proxy fragment** — a maintainer lands `changelog/pr-<N>-<slug>.md` on the base branch and re-runs the check by adding or removing a label — instead of being dropped from the notes under `changelog:skip`.
