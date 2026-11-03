### Added
- `deskpr create` now prefixes the PR title with the brief id from a `Brief:` trailer (`<stream>/<NN>: <title>`) or the full value of a `Cross-repo-brief:` trailer, unless the title already names that id as a whole token; `Authors:` and `Issue:` PRs keep their title, and `--check` prints the title it would use.
