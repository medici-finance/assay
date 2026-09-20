### Fixed
- `deskclaim-ref` now resolves an scp-like `origin` remote whose host is an SSH config `Host`
  alias (`git@alias:owner/name.git`) by shelling `ssh -G <alias>` and taking its `hostname`
  line, instead of dialing the unresolvable alias literally as an HTTPS host. (#1371)
- The alias is always passed to `ssh -G` after an explicit `--` end-of-options marker, and a
  leading `-` on the alias is refused outright before any resolution is attempted, so a
  flag-shaped host segment parsed from a remote URL can never be read as an `ssh` option.
