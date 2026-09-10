### Fixed

- **`create-fleet-gitlab.sh` no longer exposes the group-owner token on the process table, and a
  GitLab API transport failure is now recorded in the run summary instead of silently aborting the
  run (#786).** The shared `gl_api` helper — which every settings step calls — had two defects, both
  pre-existing and fleet-wide (present since the helper was introduced, not a regression of the label
  change):
  - It passed the owner PAT to `curl` as a `PRIVATE-TOKEN:` header on the command line, where any
    local user could read it off the process table. It now mints the token into a `0600` `curl -K`
    config file under `umask 077` and passes `curl -K` — never argv — the same credential custody the
    avatar step already uses.
  - It captured only curl's HTTP status, so a transport failure (DNS / TLS / connection refused, where
    curl exits non-zero) made the command substitution non-zero and, under `set -euo pipefail`,
    **hard-aborted the whole run before the failure ledger or the summary was written** — no
    diagnostic, no recorded step. It now uses `|| echo "000"` (the same pattern the avatar step uses),
    so the transport failure surfaces as the `000` status the caller's `record_failure` branch already
    handles: the failure is written to the ledger, the remaining settings steps still run, and the
    run reaches its summary and exits non-zero — recorded, not fatal.
