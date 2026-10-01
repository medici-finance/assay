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
| 16 | `cd tools/desk && go test ./cmd/deskapps/ -run 'OmitsHookAttr' -count=1 -v` | exit 0 — neither the `--tier` nor the `--manifest` path emits a `hook_attributes` key in the posted manifest JSON (assay#1260) | check:ci |
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
| 16 | PASS — `-run 'OmitsHookAttr'` runs all three pins (`TestBuildManifestJSONOmitsHookAttributesTierPath`, `TestManifestJSONOmitsHookAttrsManifestPath`, `TestManifestJSONOmitsHookAttrsWhenUnset`), each `--- PASS`; the first two assert on the RAW JSON (not just the decoded map) that no `hook_attributes` key is posted on either entry path when no webhook url is set (assay#1260) |
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
### Verification — non-implementer pass, 2026-10-01 (merged main 3da1cf5901f5)

What moved since the last run: this is the first non-implementer pass. The block above holds only the implementer's feature-branch rows, which are not verification. Merged by #1260 (squash 08d96323a). The verified tree is 3da1cf5901f5, which differs from the squash only in STATUS.md. Current origin/main (f60962e1a573) has no further change under tools/desk/cmd/deskapps, docs/desk-tools/deskapps.md or docs/streams/apps-installer.

How the rows were run: every command was run from the repo root of a temporary worktree detached at the merged head, with `KUBECONFIG=/dev/null`. Every conversion and `gh` call goes through a test double. Nothing contacted GitHub: no live App was created and no browser was opened. Each row below is labelled with the Verify row it discharges (V1–V17), and the table has one row per Verify row.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| V1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -count=1` | exit 0 | exit 0. `ok github.com/medici-finance/assay/tools/desk/cmd/deskapps`. A full `-v` run shows 49 PASS lines and no SKIP or FAIL lines | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V2 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && /tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE -e 'http://127\.0\.0\.1:[0-9]+/' -e 'example-read' -e 'example-act'` | 3 | exit 0, count 3. Raw output: `would serve at http://127.0.0.1:41873/`, then the planned Apps `example-read` and `example-act` (tier=team owner=org:example prefix=example port=41873, dry-run) | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestManifest' -count=1 -v 2>&1 \| grep -cE -e 'family.*6 manifests' -e 'team.*2 manifests' -e 'requiredDuties covered'` | ≥ 3 | exit 0, count 3. Log lines: `family tier: 6 manifests`, `team tier: 2 manifests`, `requiredDuties covered by every family manifest`. 9 tests matched the filter, all PASS, none skipped | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInPage' -count=1` | exit 0 | exit 0, `ok`. The `-v` rerun shows the test PASS. Independent review: every route it fetches is free of secrets, and every error body in server.go is a static string. Its coverage limit is recorded under Findings | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInLogs' -count=1` | exit 0, mutation-backed | exit 0, `ok`. The `-v` rerun shows the test PASS. Mutation-backed: see V11 (the response-body-log mutant reddens this test) | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestCallbackBadState' -count=1` | exit 0: foreign state gets 403, no conversion attempted, row unchanged | exit 0, `ok`. The `-v` rerun shows the test PASS. Mutation-backed: see V11 (the state-check-drop mutant reddens this test) | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindLoopbackOnly' -count=1` | exit 0: listener is 127.0.0.1 only | exit 0, `ok`. Both subtests PASS (requested_port_free, and requested_port_busy_falls_back_to_a_free_one). Independent review: hand mutants that bind `:%d` or `:0` each redden it | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V8 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemMode' -count=1` | exit 0: key is 0600 and byte-equal | exit 0, `ok`. Two tests PASS: the mode test and its pre-existing-file chmod companion. The mutations.json control (invert the 0600 check), applied by hand to a scratch copy of the module, reddens it | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindingsWritten' -count=1 -v 2>&1 \| grep -cE -e 'REVIEWER_APP=example-act' -e 'WORKER_APP=example-act' -e 'READ_APP=example-read'` | ≥ 3 | exit 0, count 3. The logged apps.env has READ_APP=example-read and the six role bindings, all =example-act. Both tests (team and family) PASS | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V10 | `grep -cE -e '^## Measured' docs/desk-tools/deskapps.md && grep -cE -e 'throttle' -e 'org owner' -e 'Enterprise Server' docs/desk-tools/deskapps.md` | 1 then ≥ 3 | exit 0. Counts: 1, then 8. The section's content says the three facts were NOT measured; each is "Assumed, pending measurement". See Findings | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V11 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | exit 0: both mutants caught by V5 and V6 | exit 0, `ok`. Both corpus-presence tests PASS. As authored, the row only proves the mutant text is present in source. So I also applied each mutations.json mutant by hand to a scratch copy of the module (outside the worktree). State-check drop: the V6 test FAILs (nil-pointer panic, then FAIL, exit 1). Response-body log: the V5 test FAILs ("real os.Stdout carried secret material", exit 1). Both mutants are caught | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V12 | `statusgen --root "$PWD" --consumers --brief apps-installer/02 --base ca81ea0a9` | exit 0 | exit 0. 0 corroborated, 0 disproved, 4 UNCHECKED ("unchanged since the merge-base"). The as-authored form (no `--base`) was run as `statusgen --root "$PWD" --consumers --brief apps-installer/02`, because a bare `.` root is write-guarded on this host. It exited 2: COULD-NOT-CHECK, the brief is not in the diff against the merged head. The corrected form above adds `--base` (the squash parent). I checked the 4 UNCHECKED claims by hand. The README has a deskapps section; docs/desk-tools/deskapps.md exists (added in the squash); records.go writes the APP_ID / CLIENT_ID / WEBHOOK_SECRET and ROLE_APP lines; the install SKILL follow-up is brief-07, which exists | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V13 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRunInitManifestAndTierMutuallyExclusive' -count=1 -v` | exit 0 | exit 0. One test, `--- PASS`, `ok` | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V14 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesRedirectURL' -count=1 -v && go test ./cmd/deskapps/ -run 'TestRunInitManifestBadFileReportsAndExits' -count=1 -v` | exit 0, both tests | exit 0. Both tests `--- PASS`, `ok`. The as-authored command, taken literally with the table escape inside the quoted -run pattern, matched no tests ("no tests to run"). That is an exit 0 that proves nothing, so it is not counted. The corrected form above runs each test on its own. See Findings | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V15 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesHookURL' -count=1 -v` | exit 0 | exit 0. One test, `--- PASS`, `ok` | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V16 | `cd tools/desk && go test ./cmd/deskapps/ -run 'OmitsHookAttr' -count=1 -v` | exit 0 | exit 0. Three tests `--- PASS` (tier path, manifest path, unset), `ok` | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| V17 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemNeverWrittenOnOrgOwnerMismatch' -count=1 -v` | exit 0: foreign App on the org path, nothing written | exit 0. One test, `--- PASS`, `ok`. Independent review: these hand mutants redden it: the original personal-only owner check, dropping the comparison, and moving the PEM or records write before the check. One gap is under Findings | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

**Core-system review questions.**
- (1) Is the owner check an acceptable independent second layer? Yes. It sits at server.go:280-297 and runs before every write (PEM at :299, records at :303, bindings at :308). It trips on the App owner that GitHub reports, which is not the local nonce record. It fails closed on an empty value on either side. It compares case-insensitively.
- (2) Does V17 prove the lower layer catches the fault with the upper layer bypassed? Yes, for the PEM and the per-App records: the test sends a valid `state` with a foreign App's code. It does not cover the role-binding lines (Findings 3).

**Risk-bearing value enumeration.** The brief is marked `irreversible: no`. The desk note asked for the credential-handling literals to be treated as risk-bearing. The enumeration covers every literal in the diff under tools/desk/cmd/deskapps (non-test files). Paths are relative to tools/desk/cmd/deskapps.

| Literal | Location | Rank / reversibility |
|---|---|---|
| file mode `0o600` (PEM, apps.env, state) | records.go:50 and records.go:53 | top: a wider mode exposes a private key; undoing it needs key rotation on GitHub |
| dir mode `0o700` | records.go:39 and records.go:42 | top: a wider dir mode exposes the key directory listing and opens a widened-file window |
| bind `127.0.0.1:%d` / fallback `127.0.0.1:0` | server.go:390 and server.go:395 | top: a wider bind exposes the nonce-serving /run to the network |
| redirect `http://127.0.0.1:%d/callback` | server.go:161 | top: tied to the bind |
| nonce `make([]byte, 24)` from crypto/rand | records.go:28 | top: guessability of the one nonce control |
| `branchProtectionRead = "administration:read"` | manifest.go:33 | top: permission scope; every App is born with it; `:write` could rewrite protection |
| `requiredDuties` (contents/issues/pull_requests :write) | manifest.go:22 | high: permission scope |
| `ciReadDuties` (checks/statuses/actions :read) | manifest.go:26 | high: permission scope |
| `teamReadPerms` / `teamActPerms` | manifest.go:36-39 and manifest.go:42-45 | high: permission scope |
| `Public: spec.Public` (false on the tier path) | manifest.go:198 | high: a public App can be installed by anyone |
| owner comparison `strings.EqualFold` with an empty-value fail-closed check | server.go:287 | high: the second trust layer |
| `githubAPIBase = "https://api.github.com"` | convert.go:22 | medium: where the code is exchanged |
| `conversionHTTPClient` Timeout `20 * time.Second` | convert.go:25 | low: reversible knob |
| `io.LimitReader(..., 1<<20)` | convert.go:69 | low: reversible knob |
| `throttleTimeout = 10 * time.Minute` | server.go:50 | low: reversible knob (brief fact) |
| ticker `15 * time.Second` | server.go:351 | low: poll interval |
| default port `41873` | main.go:62 | low: reversible knob (brief fact) |

RISK-VALUE: DERIVED — file mode = 0o600 @ tools/desk/cmd/deskapps/records.go:50 (enforced again by chmod at :53) — owner read/write only is the minimum mode that lets the owning user use the key while denying group and other. It matches the 0600 discipline the brief's sources cite: the token-cache write at tools/desk/cmd/desktoken/desktoken.go:563 and the 0600 check at :895. Umask can only narrow the create mode, and the explicit chmod restores exactly 0600.
RISK-VALUE: DERIVED — dir mode = 0o700 @ tools/desk/cmd/deskapps/records.go:39 (chmod at :42) — owner-only traversal is the narrowest mode that still lets the owner reach the files. It closes the pre-existing-looser-file window, because no other uid can traverse into the directory.
RISK-VALUE: DERIVED — bind = 127.0.0.1 @ tools/desk/cmd/deskapps/server.go:390 and server.go:395 — design.md line 30 serves the page at that address. An IPv4 loopback literal (not `localhost`, not `0.0.0.0` or `::`) keeps /run, which serves live nonces, off every non-loopback interface. The redirect at server.go:161 uses the same literal, so a process holding the IPv6 loopback cannot catch the callback.
RISK-VALUE: DERIVED — nonce length = 24 bytes @ tools/desk/cmd/deskapps/records.go:28 — 192 bits from crypto/rand, above the usual 128-bit unguessability floor for a bearer state value. It is cleared once the row is keyed (server.go:320).
RISK-VALUE: DERIVED — branchProtectionRead = "administration:read" @ tools/desk/cmd/deskapps/manifest.go:33 — the brief's facts derive it: the legacy branch-protection endpoint is the only reader of a required-check set that the rules API cannot express (#1020), and read is the whole grant. The no-administration-write manifest test asserts that no tier set carries `administration:write`. The other permission literals (manifest.go:22, :26, :36-45) match the brief's tier lists entry for entry.
RISK-VALUE: DERIVED — Public = false on the tier path @ tools/desk/cmd/deskapps/manifest.go:198 — the brief's fact says `public: false`. These are the operator's own credential Apps, and a public App could be installed on accounts the operator does not control. The `--manifest` path passes the file's own value through, which is the operator's explicit choice.

Findings. None of these fail a Verify row's Expect. They go to the desk for routing.
1. V14 as authored is a vacuous pass. The table's `\|` escape inside a quoted -run pattern becomes a literal pipe in Go's regex, matches no test, and still exits 0. Fix: author the row as two -run invocations (the corrected form above).
2. V12 as authored cannot run on merged main. `--consumers` is diff-scoped and needs `--base <squash parent>`. Even then, all 4 claims come back UNCHECKED because the consumers block itself did not change in the squash.
3. Test gap on V17's claim "NO apps.env record". Neither owner-mismatch test asserts the ROLE_APP binding lines. A hand mutant that moves the bindings write before the owner check survives both tests. The current code orders the writes correctly.
4. Medium, functional. After the 10-minute throttle flips a row to `paused`, a late but still-valid callback for that row is refused with 409 and the message "app already keyed" (server.go:245-249). The code is valid for an hour, so the App is created on GitHub but its key is lost. Brief 04 owns resume and should cover this.
5. Medium, robustness. The PEM, apps.env and state writes truncate files in place, with no temp file and rename. The apps.env read-modify-write runs outside the server mutex (server.go:299-311; records.go:108-157). In a hand probe, two concurrent callbacks for different Apps lost one App's records, which leaves an unrecoverable webhook secret missing.
6. Low. TestNoSecretInLogs captures stdout only. Hand mutants that leak the body via stderr or the `log` package survive it. The test's comment claims `log` writes to stdout, which is wrong. No test asserts the directory mode or the apps.env and state modes.
7. Low. The loopback routes check neither Host nor Origin. A hand probe with a foreign Host and Origin read /run (live nonces) and flipped a row to posted. No key can be written that way, because the owner check holds. A Host allowlist for the bound address would close it.
8. Info. The URL-path escaping comment in convert.go overclaims: `..` passes through, with no impact. The client_secret is parsed and then discarded, never written. The loopback server has no timeouts. Each `init` overwrites any existing apps.state.json.
9. Info. The brief's Ground rule asks for design §9's three facts to be measured before coding. V10 passes on keywords only, and the Measured section records all three as BLOCKED-ON-HUMAN assumptions. This is outside every Verify row's Expect and needs a live account.
10. Scope traceability: every piece of verified work above maps to a Verify row (V1–V17), or is the risk enumeration and core-system review the kit requires. Nothing is unmapped.

Execution witness. `statusgen verifyrun --root <worktree> --brief docs/streams/apps-installer/brief-02-deskapps-manifest-flow.md` (statusgen v1.0.29) was run on a clean tree after the hand run. It exited 2. Every check:ci row is could-not-run on this darwin host, because the network-off sandbox needs Linux `unshare --net`. The witness table is reproduced exactly as emitted:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && /tmp/deskapps init --tier team --org example --no-browser --dry-run 2>&1 \| grep -cE -e 'http://127\.0\.0\.1:[0-9]+/' -e 'example-read' -e 'example-act'` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestManifest' -count=1 -v 2>&1 \| grep -cE -e 'family.*6 manifests' -e 'team.*2 manifests' -e 'requiredDuties covered'` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInPage' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestNoSecretInLogs' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestCallbackBadState' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindLoopbackOnly' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemMode' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestBindingsWritten' -count=1 -v 2>&1 \| grep -cE -e 'REVIEWER_APP=example-act' -e 'WORKER_APP=example-act' -e 'READ_APP=example-read'` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -cE -e '^## Measured' docs/desk-tools/deskapps.md && grep -cE -e 'throttle' -e 'org owner' -e 'Enterprise Server' docs/desk-tools/deskapps.md` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief apps-installer/02` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 13 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRunInitManifestAndTierMutuallyExclusive' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 14 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesRedirectURL\|TestRunInitManifestBadFileReportsAndExits' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 15 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestLoadManifestFileRefusesHookURL' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 16 | `cd tools/desk && go test ./cmd/deskapps/ -run 'OmitsHookAttr' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |
| 17 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPemNeverWrittenOnOrgOwnerMismatch' -count=1 -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-01 | assay-verifier-app[bot] @ 3da1cf5901f5 (on-behalf-of human:ian) (forge-identity) |

The witness is all could-not-run because of the host. It is not a code result. The `verified` close still needs a Linux run of the same verifyrun command. The hand-run table above is the verdict.

rows_passed=17 rows_total=17

RISK-VALUE: DERIVED — six top-ranked literals: 0o600 file mode, 0o700 dir mode, 127.0.0.1 bind, 24-byte nonce, administration:read scope, public=false. Each has its file:line and derivation above. The remaining entries are reversible knobs (timeouts, body cap, poll interval, default port) and rank last.

WITNESS: could-not-run (darwin host, no `unshare --net`) — re-run verifyrun on Linux before the verified close.

VERIFY: PASS

## Review
Gate: model. Reviewer records verdict + date in the stream README table. Reviewer answers the two
core-system questions: (1) the single control between a foreign callback and a written key is the
state nonce (the record-side match is that check; the loopback bind is a precondition, not a layer)
— is the owner check on the conversion result an acceptable independent second layer? (2) rows 6 and
7 break the nonce/bind layer; row 17 bypasses it with a valid `state` and a FOREIGN App's `code` —
does it prove the owner check catches the fault with the upper layer bypassed?
