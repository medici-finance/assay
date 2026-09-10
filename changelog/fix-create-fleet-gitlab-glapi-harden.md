### Fixed
- **`create-fleet-gitlab.sh` no longer exposes the group-owner token on the process table, and
  fails closed when the GitLab API is unreachable (#786).** The shared `gl_api` helper — which every
  settings step calls — passed the owner PAT to `curl` as a `PRIVATE-TOKEN:` header on the command
  line, where any local user could read it from the process table, and it captured only the HTTP
  status, so a transport failure (DNS/TLS/connection refused) could pass through undiagnosed rather
  than surfaced. `gl_api` now mints the token into a 0600 `curl -K` config file under `umask 077` and
  passes `curl -K` — never argv — the same credential custody the label/avatar step already used, and
  it captures curl's exit status so a transport failure fails closed with the cause reported instead
  of reading as an empty success. Pre-existing and fleet-wide (present since the helper was
  introduced, not a regression of the label change).
