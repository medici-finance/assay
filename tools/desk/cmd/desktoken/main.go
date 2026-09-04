// Command desktoken — mint or reuse a per-role GitHub App installation token.
//
// A key-parameterized token minter: one tool, one audit
// path, every desk role. Usage:
//
//	desktoken <role> [--repo <slug>] [--ttl] [--fresh]
//
// Where <role> ∈ {reviewer, verifier, worker, desk, issue-loop, intake-loop}.
//
// # Where the credentials live (#794)
//
// The App private key, apps.env and the token cache are resolved across the
// App-CREDENTIAL SEARCH PATH: $ASSAY_CONFIG_HOME first when set, then the
// shipped default ~/.config/assay. The key is READ from the first directory that
// has it and the cache is WRITTEN to the head of the path, so one setting puts
// all four (key, apps.env, cache, provisioning) in the same directory. A
// deployment whose walkthrough provisions elsewhere sets
// ASSAY_CONFIG_HOME=~/.config/<its-dir> once; the not-found refusal names every
// directory searched AND the directory this repo's walkthrough provisions into,
// so a fresh-shell mint failure is self-diagnosing instead of a bare
// "private key not found".
//
// The knob covers the App-credential plane ONLY — roster.env is deliberately not
// moved by it (see deskkit/confighome.go).
//
// App ID comes from <ROLE>_APP_ID env (e.g. REVIEWER_APP_ID) else apps.env on the
// same search path — App IDs are never baked into source. The installation is
// resolved at runtime via GET /app/installations, matching the repo owner against
// account.login (defaults to "example-org" when --repo is absent); <ROLE>_INSTALL_ID
// overrides. The token is cached as <role>-token-<installID> (0600) and reused if
// < 50 min old, alongside a <cache>.perms sidecar recording what the installation
// was GRANTED (the grant is visible only in the mint response; `deskroster
// preflight` checks it against the role's duties — #571). The token SECRET is
// never printed to stdout, audit, or logs — only the cache file path.
//
// This tool provides attribution (which App name appears on GitHub) plus an audit
// trail — it does NOT enforce authorization (which session may act as each role).
// The caller holds the key and controls the env; every key is readable by any
// session of the same OS user. File permissions (0600 on a workstation; 0440
// for a Secret-mounted key read via a pod's fsGroup) protect from other users
// only, not from other sessions of the same user.
//
// Inherits deskkit: audit (one line per mint), kill-switch (Guard first),
// fail-closed. Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.
package main

import "os"

const usage = `desktoken — mint or reuse a per-role forge credential.

USAGE:
  desktoken <role> [--repo <slug>] [--ttl]           # GitHub: mint/reuse App token
  desktoken --forge gitlab <role>                     # GitLab: rotate PAT in place
  desktoken coverage <role> [--repo <slug>] [--json]  # GitHub: list repos the role's App sees
  desktoken --version

<role> ∈ {reviewer, verifier, worker, desk, issue-loop, intake-loop}

coverage — read-only enumeration. Lists every installation of the role's App and
the repositories each can see, one block per installation (stable order, by
account login). It mints a token per installation into MEMORY only: it writes no
token cache and no .perms sidecar, and prints no token or JWT. --repo <slug>
prints only the installation that sees that repository and exits 0 if one does,
5 if none (the slug is matched on owner/name, not the bare name). --json emits
the same enumeration as one object. A repository-page read that fails is exit 6
naming the installation — never a short list read as complete. GitHub-only:
--forge gitlab is refused (a PAT has no installation to enumerate).

--forge selects the backend: empty or github (default) mints a GitHub App
installation token as below; gitlab rotates the role's PAT in place.

GitLab (--forge gitlab) — rotate-on-mint token custody:
  Reads the role's current PAT from gitlab-<role>.token (0600) on the
  App-credential search path, calls the GitLab self-rotation endpoint (which
  returns a NEW token and atomically INVALIDATES the current one), write-verifies
  the new value 0600 back to the same file, and prints the PATH only — never the
  token value. At most one credential per role is ever valid; a captured token
  dies at the next mint. The new token's expiry is set by the GROUP
  token-lifetime policy (7 days RECOMMENDED, configured on the group, not here) —
  the expiry backstop that retires an idle fleet's credential on its own.
  Roles are single-window: a second concurrent rotation for the same role
  invalidates the first's token BY DESIGN — give parallel actors per-actor
  service accounts, never a shared token. A missing file, a non-regular-file
  custody, or a wrong file mode each refuses with a named remedy. GITLAB_API_BASE
  is REQUIRED and sets the REST v4 base (self-hosted:
  https://gitlab.example.com/api/v4; gitlab.com SaaS:
  https://gitlab.com/api/v4). There is no default: with it unset the command
  refuses BEFORE any network contact rather than transmit the role's live PAT
  to a guessed host.

Reads <role>-app.pem for the App's private key, and apps.env for the
App ID, from the App-credential SEARCH PATH: $ASSAY_CONFIG_HOME (when set),
then ~/.config/assay. <ROLE>_APP_ID / <ROLE>_PEM override. The key file must
not be readable by others or writable by group/others: 0600 or 0400 on a
workstation; 0440 is accepted because a Secret-mounted key read through a
pod's fsGroup is necessarily root-owned and group-readable.
Installation resolved at runtime via GET /app/installations, matching the
repo owner against account.login (defaults to "example-org" when --repo is
absent); <ROLE>_INSTALL_ID overrides.
Caches the token as <role>-token-<installID> (0600) at the HEAD of the search
path, plus a <cache>.perms sidecar recording the installation's granted scopes.
Reuses the cached token if < 50 min old; otherwise mints a fresh one.
--fresh deletes the cached token and its .perms sidecar first, forcing a
fresh mint — use it after a GitHub-App permission change, whose new grant the
cached token would otherwise mask for the rest of the ~50-min reuse window (#571).
The token SECRET is never printed to stdout/audit/logs — only the path.

If the key is not found, the refusal names every directory searched and the
directory this repo's App-provisioning walkthrough uses (#794).

Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.`

func main() {
	os.Exit(run(os.Args[1:]))
}
