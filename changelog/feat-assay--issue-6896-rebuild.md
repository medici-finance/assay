### Fixed

- `deskclaim-ref` refuses a value flag whose value is missing, empty, only whitespace, or another `--` option, in the spaced and the `=` form alike, and names that flag (`refused: --repo requires a value`). Before, the flag took the next option as its value and shifted the rest of the command: `--repo --branch -` failed as `unknown flag -`, `--repo --branch=-` looked up a work reference for a repository named `--branch=-`, and a trailing `--branch --owner` held a claim on branch `--owner`. A single `-` is still a value, the branch placeholder deskdispatch prints for a branchless claim. A `--perf-*` flag refuses a missing value or another `--` option in its place, but an empty value still clears the inherited performance value, as the shared performance contract requires.

### Added

- `deskclaim-ref` value flags also accept `--flag=value`, so `--branch=-` is the same as `--branch -`.
