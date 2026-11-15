### Fixed
- `writeguard` no longer refuses `git -C <dir> status [flags]` or `unset NAME...` beside a listed `gh` read that mentions `status`. The dir must be a literal path after one `-C`, and `unset` clears only with plain variable names; a computed dir, another git option, an `unset` flag and every `gh auth` form stay refused.
