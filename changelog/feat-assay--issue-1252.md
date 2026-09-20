### Fixed
- The `On-behalf-of:` composite-identity trailer no longer stamps a human's forge login
  onto writes to a **public** repo. The one shared resolver every write verb calls
  (`deskpost`, `deskreply`, `deskpr`, `deskfile`, `deskevidence`, `deskflip`) now chooses
  the form from the target repo's configured visibility: the roster's neutral name for
  that human on a public target, the login on a repo the roster states is `:private`. The
  split fails closed towards the neutral name — an unstated visibility, a repo admitted
  only by an `owner/*` pattern, and an unnamed target all take it — and the target repo is
  now a mandatory argument to the resolver, so no verb can resolve an identity without
  saying where the write lands. A public-target write refuses (exit 5) rather than fall
  back to the login when the roster carries no neutral name for the principal.
