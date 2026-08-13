// Command desktoken — mint or reuse a per-role GitHub App installation token.
//
// A key-parameterized token minter: one tool, one audit
// path, every desk role. Usage:
//
//	desktoken <role> [--repo <slug>] [--ttl]
//
// Where <role> ∈ {reviewer, verifier, worker, desk, issue-loop, intake-loop}. Reads
// ~/.config/assay/<role>-app.pem (0600) for the App's RSA private key. App ID
// comes from <ROLE>_APP_ID env (e.g. REVIEWER_APP_ID; set it from per-deployment config,
// see ~/.config/assay/apps.env — App IDs are never baked into source). The installation is
// resolved at runtime via GET /app/installations, matching the repo owner against
// account.login (defaults to "example-org" when --repo is absent); <ROLE>_INSTALL_ID
// overrides. The token is cached at ~/.config/assay/<role>-token-<installID>
// (0600) and reused if < 50 min old. The token SECRET is never printed to stdout,
// audit, or logs — only the cache file path.
//
// This tool provides attribution (which App name appears on GitHub) plus an audit
// trail — it does NOT enforce authorization (which session may act as each role).
// The caller holds the key and controls the env; every key is readable by any
// session of the same OS user. File permissions (0600) protect from other users
// only, not from other sessions of the same user.
//
// Inherits deskkit: audit (one line per mint), kill-switch (Guard first),
// fail-closed. Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.
package main

import "os"

const usage = `desktoken — mint or reuse a per-role GitHub App installation token.

USAGE:
  desktoken <role> [--repo <slug>] [--ttl]
  desktoken --version

<role> ∈ {reviewer, verifier, worker, desk, issue-loop, intake-loop}

Reads ~/.config/assay/<role>-app.pem (0600) for the App's private key.
App ID comes from <ROLE>_APP_ID env var (e.g. REVIEWER_APP_ID; see ~/.config/assay/apps.env).
Installation resolved at runtime via GET /app/installations, matching the
repo owner against account.login (defaults to "example-org" when --repo is
absent); <ROLE>_INSTALL_ID overrides.
Caches the token at ~/.config/assay/<role>-token-<installID> (0600).
Reuses the cached token if < 50 min old; otherwise mints a fresh one.
The token SECRET is never printed to stdout/audit/logs — only the path.

Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.`

func main() {
	os.Exit(run(os.Args[1:]))
}
