### Fixed

- Obtaining an App installation token no longer costs a process and an API round trip per
  forge read. `deskkit.RoleTokenForOwner` now memoises per `(role, account)` for the life of
  one process, bounded at 45 minutes — strictly inside `desktoken`'s own 50-minute reuse
  window — so a board read over N repositories forks `desktoken` once per ACCOUNT rather than
  once per read, and `deskflip` mints once per run instead of twice (#1036).
- `desktoken` consults its token cache before it resolves the installation id, so a warm cache
  hit makes no network call at all. Previously every "reuse cached token" still read the App
  key, signed a JWT and called `GET /app/installations` to rediscover an installation that
  changes only on install or uninstall.

### Added

- `desktoken` records an owner sidecar (`<token cache>.owner`) naming which App and which
  account a cached token belongs to, and caches a resolved installation id per (App, account)
  for 24 hours. Every fast path is a positive match on both halves; absence, ambiguity, a
  non-0600 file or a malformed account name falls through to full resolution. `--fresh` and
  `<PREFIX>_INSTALL_ID` are unaffected, and a 404 from the token exchange invalidates a cached
  installation id instead of failing for the rest of its TTL.
