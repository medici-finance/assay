# `deskapps` — the GitHub App Manifest-flow installer

`deskapps` is the installer for Assay's GitHub App identities: one command, one browser
sitting, driving GitHub's App Manifest flow (`POST /app-manifests/{code}/conversions`) so the
adopter clicks only what GitHub reserves for a signed-in human — Create, Install, and the
avatar drop. The design of record is
[`docs/streams/apps-installer/design.md`](../streams/apps-installer/design.md); this page is
the per-verb reference plus the measured facts the design asked brief 02 to record before the
conversion timing was coded to them.

This brief (apps-installer/02) ships `deskapps init`. `deskapps resume`, `status` and `avatar`
are later briefs (03, 04, 06) — invoking them today prints the same "not implemented yet"
usage line `init` prints for an unknown verb.

## `deskapps init`

```
deskapps init --tier team|family [--org <login>] [--owner org|me] [--prefix <name>] [--port 41873] [--no-browser] [--dry-run]
```

- `--tier team` — two Apps, `<prefix>-read` and `<prefix>-act`.
- `--tier family` — six Apps, one per desk role: `reviewer`, `worker`, `verifier`, `desk`,
  `issue-loop`, `intake-loop`, named `<prefix>-<role>-app`.
- `--org <login>` — the org an org-owned run creates Apps under; required unless
  `--owner me`.
- `--owner org|me` — org-owned (default) or personal-owned. An App created under a personal
  account leaves with that account; org-owned is the recommended default.
- `--prefix <name>` — the App name prefix. Defaults to `--org`'s value when given, else
  `assay`.
- `--port 41873` — the loopback port to bind. On a bind failure the next free loopback port
  is used instead, and the printed URL reflects the port actually bound.
- `--no-browser` — print the URL instead of opening a browser (also every test in this
  package's mode: `deskapps` never opens a browser in CI or in a dispatched worker's
  offline run).
- `--dry-run` — print the planned URL and App rows, then exit without serving or touching
  the network. Identity (`gh api user`) is **not** resolved on this path — dry-run's whole
  job is a network-free report of what a real run would create.

### `deskapps init --manifest`

```
deskapps init --manifest <file> [--org <login>] [--port 41873] [--no-browser] [--dry-run]
```

Registers a single arbitrary GitHub App from a manifest JSON file, instead of a tier's fixed
App set — for an App that isn't one of the six desk roles (e.g. `assay-leaksweep-app`).
`--manifest` and `--tier` are mutually exclusive.

- The manifest file's fields: `name`, `url`, `description`, `public`,
  `default_permissions`, `default_events`, `hook_attributes`.
- `deskapps` sets its OWN `redirect_url` (the loopback callback) and state nonce, the exact
  machinery `--tier` already uses — a manifest **must not** carry its own `redirect_url`, and
  its `hook_attributes` must not carry a `url`; either is REFUSED with a clear error rather
  than silently stripped.
- This flow never sets a webhook URL, so a url-less `hook_attributes` (e.g. `{"active": true}`)
  is **accepted by the loader but then dropped** — it is not posted, because GitHub's manifest
  schema rejects a `hook_attributes` object with no `url` (assay#1260). If you need a webhook,
  set it on the App in GitHub's UI after creation; `deskapps` does not carry one through this
  flow.
- `--org <login>` selects org-owned (its presence) vs. personal-owned (its absence) — there
  is no separate `--owner` flag for this mode.
- Runs the SAME callback → conversion → PEM-write path as `--tier`, including the design.md
  §8 identity-mismatch check. The one structural difference: the `apps.state.json` row (and
  every `apps.env` write) is keyed by the manifest App's own **name**, not a desk role — no
  `<ROLE>_APP=`/`READ_APP=` binding line is written for it.

### Files written

- `~/.config/assay/apps.env` — `<APP>_APP_ID`, `<APP>_CLIENT_ID`, `<APP>_WEBHOOK_SECRET` per
  App (0600), plus the `<ROLE>_APP=` role→App bindings (brief 01) and, for the team tier,
  `READ_APP=<prefix>-read`. Writes MERGE: an existing file's unrelated lines are never
  clobbered.
- `~/.config/assay/apps.state.json` — schema `deskapps-state-v1`, one row per App: state
  (`pending → posted → keyed → avatar_ok → installed → verified`, with `posted → paused` on
  the creation throttle), the per-row state nonce, timestamps, App id, client id.
- `~/.config/assay/<app>.pem` — the App's private key, written once, mode 0600.

### Trust boundaries (design.md "Shared conventions")

- **Loopback only.** The page binds `127.0.0.1`; `bind_test.go`'s `TestBindLoopbackOnly`
  asserts `0.0.0.0`/`::` never appear on the listener, on both the direct-bind and the
  fallback-port paths.
- **The state nonce** keeps a `/callback` request from being accepted for a row it was not
  issued for: `server.go`'s callback handler looks the incoming `state` up against a pending
  row and refuses (403) before anything else happens — no conversion attempted, no row touched
  — on a miss. `callback_test.go`'s `TestCallbackBadState` is the negative case;
  `TestCallbackGoodStateConverts` is the positive control proving the check isn't refusing
  everything. The nonce is consumed once a row is keyed, so a replay carrying it changes
  nothing (`TestCallbackReplayDoesNotOverwriteKey`).
- **The genuinely independent second layer is the owner check on the conversion result.** The
  App's real owner as GitHub reports it must equal the owner the operator named — their `gh`
  login (personal-owned) or `--org` (org-owned) — before any key is written; a mismatch writes
  nothing and re-arms the row (`pem_test.go`'s `TestPemNeverWrittenOnMismatch` for the personal
  path, `TestPemNeverWrittenOnOrgOwnerMismatch` for the org path). It trips on a different
  signal (the forge's reported owner, not the local state record) in a different component, so
  it catches exactly the fault the nonce cannot: a callback carrying a valid `state` and a
  foreign App's `code`. The loopback bind is a **precondition, not an independent layer** —
  `GET /run` serves each pending row's live nonce to any local process, so "reached the
  listener" and "knows the nonce" are one capability, not two.
- **The private key is written once, mode 0600, and never printed, logged or rendered.**
  `secrets_test.go`'s `TestNoSecretInPage` drives a real (fake) conversion and checks every
  served route's HTML for the PEM, client secret and webhook secret; `TestNoSecretInLogs`
  does the same for stdout and the `deskkit` audit line, while also asserting both carry
  `app=`/`state=` for every state change. `convert.go`'s conversion-failure path deliberately
  omits the response body from its error string for the same reason.
- **Nothing here asks for a password or a token of its own.** Identity comes from `gh auth`
  once (`identity.go`'s `ghIdentity`, a registered forge-surface exception per #1260's
  ruling — `tools/desk/internal/forgeban/allowlist.go`), never called for `--dry-run`.
- **Public tree.** This file and the code it documents are self-contained: no private repo
  names, no private issue references.

## Measured

Design.md §9 asks brief 02 to measure three facts on a throwaway account before coding the
conversion timing, and to record the answers here.

**These three could not be measured in this session.** The implementing session runs under
an offline envelope that forbids contacting any live/production endpoint, GitHub's API
included — every test in this package therefore drives a fake `httptest.Server` standing in
for `api.github.com`, never the real one. Answering §9 requires a real signed-in browser
click through GitHub's live App-creation flow on a throwaway account, which this session
cannot perform. This is filed as **BLOCKED-ON-HUMAN** on the brief's PR: a human (or a
session with live GitHub access) needs to run the three checks below once and update this
section with the real answers. Until then, `deskapps init`'s behaviour follows the
conservative reading of each question — see "Assumed, pending measurement" after each one.

1. **Does the throttle count conversions, or only the create form?** This decides whether
   codes are converted the instant they arrive (the plan this brief implements) or may be
   batched. *Assumed, pending measurement:* the **throttle** is charged against the create
   step (GitHub's account-level App-creation limit), so `deskapps` converts every code the
   moment its callback lands — `handleCallback` never queues or batches a conversion.
2. **Does the org-owned manifest URL require org owner, or does an App-manager role
   suffice?** *Assumed, pending measurement:* `deskapps` requires **org owner** per
   design.md §8 ("Not an org owner") and offers personal ownership with the custody trade
   stated as the fallback; it does not attempt to detect an App-manager role.
3. **Is a redirect to a loopback URL accepted on GitHub Enterprise Server?** *Assumed,
   pending measurement:* `deskapps` assumes yes — nothing in this brief special-cases
   **Enterprise Server**, and `githubAPIBase`/`newAppURL` are the only two seams a future
   brief would need to repoint at a GHES host.
