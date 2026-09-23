### Fixed
- The GitHub App token minter is now reached only after the forge serving the repo is resolved. `deskwt role-init` selects its credential through the shared forge-aware resolver (`deskkit.ResolveRoleCredential`) instead of its own forge branch. `deskgit push/fetch --as` refuses a GitLab-served origin before any token is minted, where it previously minted a GitHub App token for that origin. `scanloop`'s poller identity records a GitLab-served owner as a named keyring fallback instead of minting for it. GitHub repos behave as before.

### Added
- A structural class guard (`TestGitHubMinterReachedOnlyFromForgeArms`) walks `tools/desk` and fails on any reference to `RoleTokenForOwner` / `RoleTokenForRepo`, whether a call or a function value, outside a two-entry reviewed allow-list: the resolver's GitHub arm and `cellctl deskd`'s forge-switched GitHub arm.
