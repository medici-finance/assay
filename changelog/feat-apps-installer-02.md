### Added
- `deskapps init` — the loopback web app that drives GitHub's App Manifest flow: it posts
  the chosen tier's manifest (`team`: read+act; `family`: one App per desk role) to GitHub's
  own new-App page, exchanges the returned code for the App's credentials, and writes the
  private key (0600, never printed or logged), `apps.env` and the role→App bindings. Binds
  `127.0.0.1` only, and refuses a `/callback` whose state does not match a pending row.
- `deskapps init --manifest <file>` — registers a single arbitrary GitHub App (e.g. a
  leak-sweep gate App) from a manifest JSON file instead of a tier's fixed App set, through
  the same loopback bind, state-nonce, callback/conversion/PEM-write and identity-mismatch
  machinery as `--tier`, keyed by the App's manifest name rather than a desk role. Mutually
  exclusive with `--tier`; refuses a manifest that specifies its own `redirect_url` or
  `hook_attributes.url`.
