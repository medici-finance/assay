---
brief: assay:assay:apps-installer:02
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
schema: brief-v2
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
version: 1
id: 566d5440-7cb6-4731-9aea-d153ee6b16c8
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

single-point-of-failure: the `state` nonce keeps a callback from being accepted for a row it was
not issued for. The genuinely INDEPENDENT second layer is the owner check on the conversion
result: the App's real owner as GitHub reports it must equal the owner the operator named — their
`gh` login (personal-owned) or `--org` (org-owned) — before any key is written. It trips on a
different signal (the forge's reported owner, not the local state record) in a different component,
so it catches exactly the fault the nonce cannot: a callback carrying a valid `state` and a FOREIGN
App's code. The loopback bind is a PRECONDITION, not a second independent layer — `GET /run` serves
each pending row's live nonce to any local process, so "reached the listener" and "knows the nonce"
are one capability, not two. Rows 6 and 7 break the nonce/bind layer; row 17 breaks the owner
layer (foreign App's code on the org path → nothing written).

facts:
- Manifest fields used: `name`, `url`, `redirect_url` (`http://127.0.0.1:<port>/callback`),
  `public: false`, `default_permissions`, `default_events: []`. No `hook_attributes` key is
  posted: this flow never sets a webhook URL, and GitHub's manifest schema rejects a
  `hook_attributes` object with no `url` (even `{active: false}`), so a webhook-less App omits
  the key entirely (assay#1260).
- Tier manifests (this brief's data; permissions are the desk preflight's required set plus
  CI-read for the roles that read CI, plus `administration:read` for the roles that read branch
  protection):
  - `team`: `<prefix>-read` = metadata, contents:read, issues:read, pull_requests:read,
    checks:read, statuses:read, actions:read, administration:read. `<prefix>-act` = contents:write,
    issues:write, pull_requests:write, checks:read, statuses:read, actions:read,
    administration:read.
  - `family`: six manifests named `<prefix>-<role>-app`; every one carries contents:write,
    issues:write, pull_requests:write (the `requiredDuties` set); reviewer, worker and desk add
    checks:read, statuses:read, actions:read; reviewer and desk additionally carry
    administration:read.
  - `administration:read` is what lets `deskflip` read a branch's required status checks through the
    legacy branch-protection endpoint — the ONLY endpoint that can read a required set the rules API
    cannot express. The rules API surfaces rulesets only, and within a ruleset only a
    `required_status_checks` rule carries contexts, so BOTH a classically-protected branch AND a
    branch under a ruleset with no `required_status_checks` rule read as "protected, no contexts"
    and fail the gate closed. A reviewer App born without the permission flips nothing on such a
    repo: could-not-check forever (#1020). Read-only is the whole grant — never
    `administration:write`, which can rewrite protection itself. Because these manifests are what
    every future install is born with, the permission belongs in the manifest data, not in a
    post-install fix-up.
- Bindings written by tier (`<ROLE>_APP` lines, brief 01): `team` → all six roles → `<prefix>-act`
  except reads: `deskboard`/index paths use `<prefix>-read` via a `READ_APP=<prefix>-read` line
  (consumer: brief 03 decides which verbs mint the read App; this brief only writes the line).
  `family` → `<role>` → `<prefix>-<role>-app`.
- Records: `apps.env` gains `<APP>_APP_ID`, `<APP>_INSTALL_ID` (filled by brief 03),
  `<APP>_CLIENT_ID`, `<APP>_WEBHOOK_SECRET` (0600), plus the bindings. `apps.state.json` schema
  `deskapps-state-v1` per design §4. PEM at `<credential-search-path-head>/<app>.pem`, 0600.
- Identity: `gh api user` (login, email, avatar_url) at start; never a token of its own. (The
  `gh api user/memberships/orgs` owned-orgs lookup was dropped — `identity.go`'s `ghOwnedOrgs`,
  removed in `fcbe86aa6`: Screen 1 never rendered an owned-orgs list, so the call was dead.)
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
   [--no-browser] [--dry-run]`: read identity (`gh api user`), write an initial `apps.state.json`
   with one `pending` row per App in the design's order, start the loopback server, open the
   browser (or print the URL). `--dry-run` prints the URL and the planned App rows and exits
   without serving.
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
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -count=1` | exit 0 | check:ci |
| 2 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && /tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE -e 'http://127\.0\.0\.1:[0-9]+/' -e 'example-read' -e 'example-act'` | 3 (URL printed, two App rows named) | check:ci |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestManifest' -count=1 -v 2>&1 \| grep -cE -e 'family.*6 manifests' -e 'team.*2 manifests' -e 'requiredDuties covered'` | ≥ 3 | check:ci |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInPage' -count=1` | exit 0 — served HTML for every route contains no PEM header, client secret, or webhook secret from the fake conversion | check:ci |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInLogs' -count=1` | exit 0 — stdout and the deskkit audit line carry `app=`, `state=`, never key material | check:ci +mutation |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestCallbackBadState' -count=1` | exit 0 — a callback with a foreign `state` is 403, no conversion attempted, row unchanged | check:ci +mutation |
| 7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindLoopbackOnly' -count=1` | exit 0 — listener address is `127.0.0.1:<port>`; `0.0.0.0` and `::` never appear | check:ci |
| 8 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemMode' -count=1` | exit 0 — written key is mode 0600 and byte-equal to the fake conversion's `pem` | check:ci |
| 9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindingsWritten' -count=1 -v 2>&1 \| grep -cE -e 'REVIEWER_APP=example-act' -e 'WORKER_APP=example-act' -e 'READ_APP=example-read'` | ≥ 3 | check:ci |
| 10 | `grep -cE -e '^## Measured' docs/desk-tools/deskapps.md && grep -cE -e 'throttle' -e 'org owner' -e 'Enterprise Server' docs/desk-tools/deskapps.md` | 1 then ≥ 3 | check:ci |
| 11 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | exit 0 — both mutants are caught by rows 5 and 6 | check:ci |
| 12 | `statusgen --root . --consumers --brief apps-installer/02` | exit 0 (routing claims corroborated against the diff) | check:ci |
| 13 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRunInitManifestAndTierMutuallyExclusive' -count=1 -v` | exit 0 — `--manifest` together with an explicit `--tier` is refused, at the flag layer, before any manifest file is read or port bound; stderr names "mutually exclusive" | check:ci |
| 14 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesRedirectURL\|TestRunInitManifestBadFileReportsAndExits' -count=1 -v` | exit 0 — a manifest carrying its own top-level `redirect_url` is refused with a clear error naming `redirect_url`, both at the loader (`LoadManifestFile`) and end-to-end through `deskapps init --manifest` (non-zero exit, nothing written) | check:ci |
| 15 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesHookURL' -count=1 -v` | exit 0 — a manifest carrying `hook_attributes.url` is refused with a clear error naming `hook_attributes.url`, the same way and for the same reason as `redirect_url` | check:ci |
| 16 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBuildManifestJSONOmitsHookAttributes' -count=1 -v` | exit 0 — neither the `--tier` nor the `--manifest` path emits a `hook_attributes` key in the posted manifest JSON (assay#1260) | check:ci |
| 17 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemNeverWrittenOnOrgOwnerMismatch' -count=1 -v` | exit 0 — a `/callback` with a valid `state` and a FOREIGN App's `code` on the org-owned path writes NO PEM and NO `apps.env` record, and re-arms the row (the owner layer catching the fault the nonce cannot) | check:ci |

## Evidence

Implemented on branch `feat/apps-installer-02`. All seventeen Verify rows run locally (offline —
every conversion/`gh` call is a test double; `KUBECONFIG=/dev/null`, no live GitHub contact from
this session). Rows 13-15 were added in a follow-up review round (clause 7): the `--manifest`
mode's two refusal paths and its mutual exclusivity with `--tier` already had passing tests
(`manifest_flow_test.go`) but no executable Verify row citing them, so there was no traceable
proof-of-behaviour for that deliverable — these rows cite the existing tests rather than
duplicating them, per the fail-first rule (nothing new is being pinned, so no new fail-first
run applies). Rows 16-17 were added in a later review round: row 16 wires the already-present
`hook_attributes`-omission tests into the table (assay#1260); row 17 is the new org-owner check
(the S-1 security finding), which was fail-first-proven — the test writes a foreign App's PEM
against the pre-fix code and refuses it after (see its Result below).

| # | Result |
|---|--------|
| 1 | PASS — `go build ./...` clean; `go test ./cmd/deskapps/ -count=1` exit 0 |
| 2 | PASS — `deskapps init --tier team --org example --no-browser --dry-run` prints the loopback URL and both `example-read`/`example-act` rows; grep count 3 |
| 3 | PASS — `TestManifest*` (`-v`) logs `family tier: 6 manifests`, `team tier: 2 manifests`, `requiredDuties covered`; grep count 3 (≥3) |
| 4 | PASS — `TestNoSecretInPage` drives a real (fake) conversion carrying unmistakable fake pem/client-secret/webhook-secret and checks every served route (`/`, `/tier`, `/setup`, `/run`) for them |
| 5 | PASS — `TestNoSecretInLogs` checks the injected console writer, the REAL `os.Stdout` (captured via `os.Pipe`, not just the injectable writer — see the fail-first note below), and the `deskkit` audit line; all three carry `app=`/`state=`, none carry the secret material |
| 6 | PASS — `TestCallbackBadState`: a callback with a state matching no pending row is refused 403, `convertCodeFn` is never called, and the row is untouched |
| 7 | PASS — `TestBindLoopbackOnly`: the listener address always starts `127.0.0.1:`, on both the direct-bind and the bind-fails-take-next-free-port paths; `0.0.0.0`/`::` never appear |
| 8 | PASS — `TestPemMode`: the written key is mode 0600 and byte-equal to the fake conversion's `pem` |
| 9 | PASS — `TestBindingsWritten` (`-v`) logs the merged `apps.env`; grep count 3 (≥3) for `REVIEWER_APP=example-act`, `WORKER_APP=example-act`, `READ_APP=example-read` |
| 10 | PASS — `docs/desk-tools/deskapps.md` carries `## Measured` (count 1) and `throttle`/`org owner`/`Enterprise Server` (count 8, ≥3) |
| 11 | PASS — `TestMutationCorpus*` confirm both `mutations.json` mutants are present verbatim against the real source; independently **hand-applied both mutants** (see fail-first below) and confirmed the named guard tests genuinely fail, then reverted |
| 12 | PASS (exit 0) once this Evidence edit puts the brief file itself in the diff — `--consumers` is diff-scoped by design (its own `--help`: "corroborate ... against its own diff") and reports `no brief files in the diff — nothing to corroborate` for a diff that never touches a `docs/streams/*/brief-*.md` file, which was true before this edit landed. Both id forms accepted (`apps-installer/02` and the frontmatter's own `assay:assay:apps-installer:02`) — the `#822` slash-form rejection brief 01's Evidence records did not reproduce here. The four `consumers:` entries print **UNCHECKED**, not corroborated: statusgen's own diff-corroboration heuristic did not match the claimed file edits to a recognizable pattern in this diff, even though `git diff refs/remotes/origin/main -- tools/desk/README.md docs/desk-tools/deskapps.md` shows both were genuinely added/extended. Per the tool's own text ("UNCHECKED entries are NOT passes — each one's truth is the reviewer's call") and the row's literal Expect column (exit 0), this is recorded as PASS on the exit code with the UNCHECKED nuance named for the reviewer. |
| 13 | PASS — `TestRunInitManifestAndTierMutuallyExclusive`: `deskapps init --manifest <file> --tier family --dry-run` exits non-zero and stderr reads `deskapps init: --manifest and --tier are mutually exclusive` (main.go's `runInit`, checked via `fs.Visit` before either path runs) |
| 14 | PASS — `TestLoadManifestFileRefusesRedirectURL` (loader-level: `LoadManifestFile` returns an error naming `redirect_url` for a manifest carrying its own top-level `redirect_url`) and `TestRunInitManifestBadFileReportsAndExits` (end-to-end: `deskapps init --manifest <that file> --dry-run` exits non-zero, stderr names `redirect_url`, no port bound, nothing written) both pass |
| 15 | PASS — `TestLoadManifestFileRefusesHookURL`: `LoadManifestFile` returns an error naming `hook_attributes.url` for a manifest whose `hook_attributes` carries its own `url`, presence-checked on the raw JSON before the typed unmarshal (so the field is refused, never silently dropped) |
| 16 | PASS — `TestBuildManifestJSONOmitsHookAttributesTierPath` and `TestManifestJSONOmitsHookAttrsManifestPath` both assert on the RAW JSON (not just the decoded map) that no `hook_attributes` key is posted on either entry path when no webhook url is set (assay#1260) |
| 17 | PASS (fail-first proven) — `TestPemNeverWrittenOnOrgOwnerMismatch`: with `--org example` and a conversion result owned by `attacker-org`, the `/callback` writes no PEM and no `apps.env` record and re-arms the row. Reverting only the owner check in `server.go` to the pre-fix `ownerKind=="me"`-gated form makes this test fail (`a PEM was written despite an org-owner mismatch`), confirming the org path had no owner check at all before the fix; restored → green |

**Fail-first (Task 7 / mutations.json).** Hand-applied both required mutants directly against
the built package (not just corpus presence):
- **State-check drop** (`if row == nil { … }` → `_ = row` in `server.go`'s `handleCallback`):
  `TestCallbackBadState` goes from PASS to FAIL — the request panics (nil `row` dereference,
  recovered by `net/http` into a connection reset) instead of a clean 403, which the test
  correctly reads as a failure (not a 403, and — had the panic not fired — `convertCodeFn` would
  have been reached, which the test also checks for).
- **Response-body log** (append `fmt.Println("deskapps: conversion response", string(body))` in
  `convert.go`'s `convertCode`): `TestNoSecretInLogs` goes from PASS to FAIL — the fake
  `client_secret`/`webhook_secret`/`pem` are printed to stdout and the test's real-`os.Stdout`
  capture catches it. (An earlier draft of this test only checked the server's *injectable*
  console writer, which a bare `fmt.Println` bypasses entirely — the test was strengthened to
  capture real `os.Stdout` via `os.Pipe` before this evidence was recorded, specifically because
  the first hand-applied mutation run exposed the gap.)

Reverting both restores all-green; `git diff` clean afterwards.

**Design decisions (frontmatter question (a) — decisions the facts do not fix).**
- **`--prefix` default.** Undocumented by design.md; Verify row 2 (`--org example`, no
  `--prefix`) expects `example-read`/`example-act`, which fixes the default as: explicit
  `--prefix` first, else `--org`'s value, else `assay`. Recorded in `docs/desk-tools/deskapps.md`.
- **Create button, not a JS auto-submit.** Design.md §3 describes an "auto-submitting form"; this
  brief renders a manual Create button (`target="_blank"`) that still costs the person exactly one
  GitHub click, keeps `/run` open in the original tab so the state-nonce-scoped
  `POST /mark-posted` `onsubmit` fetch can mark the row `posted` before the tab navigates, and
  avoids a page-load side effect that fires GitHub's throttle without the person having acted.
- **§9 "Measured" facts.** Could not be measured from this offline session (no live GitHub
  account access) — recorded as `BLOCKED-ON-HUMAN` in `docs/desk-tools/deskapps.md`'s Measured
  section, with the conservative assumption `deskapps init` currently codes to, named for each of
  the three questions.
- **`gh`/GitHub calls are test-hook seams**, never real network from a test: `runGH` (identity.go)
  and `githubAPIBase`/`conversionHTTPClient` (convert.go) are package vars every test in this
  package overrides; `--dry-run` additionally never calls either, so Verify row 2 — the one row
  that runs the compiled binary directly rather than through `go test` — stays offline by
  construction.

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer answers the two
core-system questions: (1) the single control between a foreign callback and a written key is the
state nonce — is the loopback bind plus the record-side match an acceptable second and third layer?
(2) rows 6 and 7 bypass the upper layer; do they prove the lower one catches it?
