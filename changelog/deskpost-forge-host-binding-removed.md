### Changed
- `deskpost` no longer binds a forge host literal of its own. Its App-token mint and
  its REST reads — the ones with no typed operation on the frozen `Forge` seam
  (repository contents, the head commit's author, the trust-gate GraphQL query and the
  present-label set) — now source their host through the forge module
  (`deskkit.GitHubBaseURLOrDefault`) rather than a package-level `apiBaseURL` bound to
  the host at init. The production host override is EMPTY, exactly as `deskflip`'s is,
  so the concrete literal lives in one place (the forge module) and never in a cmd
  package; the custody minter `deskpost` installs on the resolver returns that same
  empty override, so its writes and its reads share one seam. Transport only — every
  request `deskpost` makes, and the reviewer-App identity it makes them under, is
  unchanged at the wire. Completes forge-neutral/03 row 8 (the host binding is gone,
  not merely unused).
