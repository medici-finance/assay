### Fixed
- `deskclaim-ref` now resolves an scp-like `origin` remote whose host is an SSH config `Host`
  alias (`git@alias:owner/name.git`) by shelling `ssh -G <alias>` and taking its `hostname`
  line, instead of dialing the unresolvable alias literally as an HTTPS host. (#1371)
