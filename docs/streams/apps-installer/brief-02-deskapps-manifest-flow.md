---
brief: apps-installer/02
title: "`deskapps init` — loopback page, tier manifests, the manifest→code→conversion flow, key and record writes"
why: >-
  Creating one GitHub App by hand is eight steps; the recommended adoption needs two Apps and the
  full suite six, against an account throttle. GitHub's App Manifest flow reduces creation to one
  click per App and hands the App ID and private key to a program, but only if a program is
  listening for the code and performs the exchange. This brief is that program: it turns the
  runbook adopters abandon into a page they click through in one sitting, and it keeps the private
  key on their machine.
wave: 1
depends: ["apps-installer/01"]
unblocks: ["apps-installer/03", "apps-installer/04", "apps-installer/06"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §1–§3, §6, §9 — the rules, the screens, the sequence of one App, the command surface, the three facts to measure first."
  - "GitHub App Manifest flow: an HTML form POSTs a `manifest` JSON field to `/settings/apps/new?state=` (personal) or `/organizations/<org>/settings/apps/new?state=` (org-owned); GitHub redirects to the manifest's `redirect_url` with `code` and `state`; `POST /app-manifests/{code}/conversions` returns id, slug, pem, webhook_secret, client_id, client_secret; the code is valid for one hour. Reference implementation: Probot's setup wizard."
  - "`tools/desk/cmd/desktoken/desktoken.go` — credential search path, apps.env, the 0600 discipline the new writes must match."
  - "apps-installer/01 — the `<ROLE>_APP=<app-name>` binding the installer writes, one per role."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no `deskapps` command exists; `tools/desk/cmd/` has no loopback HTTP server; the adoption runbook's App primitive is a hand walkthrough."
exec-tier: strong
exec-tier-why: >-
  Question (a) — design decisions the facts do not fix (HTML form composition, state-nonce lifecycle,
  the exact record schema). Question (c) — a subtle error here leaks a private key to a log or
  serves it in HTML and survives happy-path tests; the negative tests are the point.
consumers:
  - "~/.config/assay/apps.env (format): fixed-here (new `<APP>_APP_ID`, `<APP>_INSTALL_ID`, `<ROLE>_APP` lines; existing readers unchanged)"
  - "tools/desk/README.md: fixed-here (new `deskapps` section)"
  - "docs/desk-tools/deskapps.md: fixed-here (new per-verb doc)"
  - "plugins/assay/skills/install/SKILL.md: follow-up apps-installer/07"
---

# Brief 02 — `deskapps init`: the manifest flow

## Context
files:
- `tools/desk/cmd/deskapps/` (new): `main.go`, `server.go` (loopback HTTP), `manifest.go`
  (tier → manifests), `convert.go` (code exchange), `records.go` (apps.env / state writes),
  `page/` (embedded HTML + CSS, no external assets), `*_test.go`.
- `tools/desk/internal/deskkit/` — reuse `Log`, exit codes, credential search path helpers; add
  nothing role-specific here.
- `tools/desk/README.md` (new § deskapps), `docs/desk-tools/deskapps.md` (planned).
- `tools/desk/cmd/deskapps/mutations.json` (new).

single-point-of-failure: the `state` nonce is the ONE control that keeps a callback from being
accepted by a listener that did not issue it. The second, independent layer: the listener binds
`127.0.0.1` only, so a foreign callback has to originate on the machine; and the conversion is
performed only for a code whose `state` matches a pending row in `apps.state.json`, a third check in
a different component (the record, not the HTTP handler). Verify rows 6 and 7 break the upper layer.

facts:
- Manifest fields used: `name`, `url`, `redirect_url` (`http://127.0.0.1:<port>/callback`),
  `public: false`, `default_permissions`, `default_events: []`, `hook_attributes: {active: false}`.
- Tier manifests (this brief's data; permissions are the desk preflight's required set plus
  CI-read for the roles that read CI):
  - `team`: `<prefix>-read` = metadata, contents:read, issues:read, pull_requests:read,
    checks:read, statuses:read, actions:read. `<prefix>-act` = contents:write, issues:write,
    pull_requests:write, checks:read, statuses:read, actions:read.
  - `family`: six manifests named `<prefix>-<role>-app`; every one carries contents:write,
    issues:write, pull_requests:write (the `requiredDuties` set); reviewer, worker and desk add
    checks:read, statuses:read, actions:read.
- Bindings written by tier (`<ROLE>_APP` lines, brief 01): `team` → all six roles → `<prefix>-act`
  except reads: `deskboard`/index paths use `<prefix>-read` via a `READ_APP=<prefix>-read` line
  (consumer: brief 03 decides which verbs mint the read App; this brief only writes the line).
  `family` → `<role>` → `<prefix>-<role>-app`.
- Records: `apps.env` gains `<APP>_APP_ID`, `<APP>_INSTALL_ID` (filled by brief 03),
  `<APP>_CLIENT_ID`, `<APP>_WEBHOOK_SECRET` (0600), plus the bindings. `apps.state.json` schema
  `deskapps-state-v1` per design §4. PEM at `<credential-search-path-head>/<app>.pem`, 0600.
- Identity: `gh api user` (login, email, avatar_url) and `gh api user/memberships/orgs
  --jq '.[] | select(.role=="admin") | .organization.login'` at start; never a token of its own.
- Default port 41873; on bind failure take the next free loopback port and derive `redirect_url`
  from the port actually bound.
- Timeout for a Create click: 10 minutes without a callback flips the row to `paused` (the
  throttle) — this brief writes the state; brief 04 owns the resume.
- Console: every state change is one line on stdout (`<hh:mm:ss>  <app> → <state> · <detail>`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The PEM, client secret and webhook secret never appear in stdout, the audit log, or any served
  page. Tests assert it (rows 4, 5).
- No external assets in the page: CSS and JS inline, no CDN, no fonts fetched.
- **Measure design §9's three questions on a throwaway account before coding the conversion
  timing**, and record the answers in `docs/desk-tools/deskapps.md` (planned) § "Measured".

## Task
1. `deskapps init --tier team|family [--org L] [--owner org|me] [--prefix assay] [--port N]
   [--no-browser] [--dry-run]`: read identity, list owned orgs, write an initial `apps.state.json` with one
   `pending` row per App in the design's order, start the loopback server, open the browser (or
   print the URL). `--dry-run` prints the URL and the planned App rows and exits without serving.
2. Serve Screen 0 (tier, read-only reflection of `--tier`; switching redraws copy — the chooser
   content is the design §2 table verbatim), Screen 1 (identity strip, owner, names, permissions,
   avatar placeholders until brief 06), Screen 2 (run board; Create cell only in this brief — the
   Install and Verify cells render `after create` until brief 03).
3. Create: an auto-submitting form per row posting `manifest` + `state` to the owner-appropriate
   GitHub URL. `/callback` validates `state` against a pending row, converts immediately, writes
   PEM 0600 and records, flips the row to `keyed`, prints the console line. Conversion 404 → row
   stays `posted` with the "Create again" message.
4. Name collision (GitHub's error page carries "Name has already been taken"): the callback never
   fires; on the person's return the page offers `<name>-<org>` — accept or edit, never silent.
5. Identity mismatch: personal-owned, conversion `owner.login` ≠ `gh` login → red identity strip,
   nothing written, row re-armed.
6. `docs/desk-tools/deskapps.md` (planned) + README section: the verb, the files, the trust boundaries
   (design "Shared conventions"), the measured answers.
7. `mutations.json`: (a) drop the `state` check in `/callback`; (b) log the conversion response
   body. Rows 6 and 5 must go red respectively.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -count=1` | exit 0 |
| 2 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && /tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE -e 'http://127\.0\.0\.1:[0-9]+/' -e 'example-read' -e 'example-act'` | 3 (URL printed, two App rows named) |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestManifest' -count=1 -v 2>&1 \| grep -cE -e 'family.*6 manifests' -e 'team.*2 manifests' -e 'requiredDuties covered'` | ≥ 3 |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInPage' -count=1` | exit 0 — served HTML for every route contains no PEM header, client secret, or webhook secret from the fake conversion |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInLogs' -count=1` | exit 0 — stdout and the deskkit audit line carry `app=`, `state=`, never key material |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestCallbackBadState' -count=1` | exit 0 — a callback with a foreign `state` is 403, no conversion attempted, row unchanged |
| 7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindLoopbackOnly' -count=1` | exit 0 — listener address is `127.0.0.1:<port>`; `0.0.0.0` and `::` never appear |
| 8 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemMode' -count=1` | exit 0 — written key is mode 0600 and byte-equal to the fake conversion's `pem` |
| 9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindingsWritten' -count=1 -v 2>&1 \| grep -cE -e 'REVIEWER_APP=example-act' -e 'WORKER_APP=example-act' -e 'READ_APP=example-read'` | ≥ 3 |
| 10 | `grep -cE -e '^## Measured' docs/desk-tools/deskapps.md && grep -cE -e 'throttle' -e 'org owner' -e 'Enterprise Server' docs/desk-tools/deskapps.md` | 1 then ≥ 3 |
| 11 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | exit 0 — both mutants are caught by rows 5 and 6 |
| 12 | `statusgen --root . --consumers --brief apps-installer/02` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer answers the two
core-system questions: (1) the single control between a foreign callback and a written key is the
state nonce — is the loopback bind plus the record-side match an acceptable second and third layer?
(2) rows 6 and 7 bypass the upper layer; do they prove the lower one catches it?
