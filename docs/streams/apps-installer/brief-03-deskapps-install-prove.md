---
brief: apps-installer/03
title: "`deskapps` install + prove — installation poll, fresh mint, scopes-vs-duties, roster write"
why: >-
  A created App with no installation is a key that reaches nothing, and an installed App whose
  grant misses a duty is a role the desk preflight refuses to boot — today the adopter discovers
  both at the first desk boot, hours after the setup they thought was done. Closing the run with
  the same check the preflight runs, on the same freshly minted token, means the install is proven
  before the browser closes.
wave: 2
depends: ["apps-installer/01", "apps-installer/02"]
unblocks: ["apps-installer/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §2 Screen 2 (Install, Verify cells), Screen 3 (Prove), §3 (poll, mint, check), §8 (installed on the wrong account; scopes ≠ duties; cross-org cell)."
  - "`tools/desk/internal/deskkit/preflight.go` — `CheckAppScopes` (`app-scopes-vs-duties`), `requiredDuties` (pull_requests:write, issues:write, contents:write), the `.perms` sidecar written on a FRESH mint, `desktoken <role> --fresh`."
  - "`tools/desk/cmd/deskroster/preflight.go` — the check is already surfaced by `deskroster preflight`; this brief invokes the same code, never a copy."
  - "`docs/adopting-assay.md` roster section: `ASSAY_TRUSTED_BOT_SLUGS=[role=]slug:<bot-user-id>`; the bot USER id is what commits and trust matching key on."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no code polls `/app/installations` with an App JWT; the roster is hand-written by the adopter."
exec-tier: strong
exec-tier-why: >-
  Question (b) — cross-artifact: the bot USER id (for the roster) is a different number from the
  App id (for apps.env) and from the installation id; the three are read from three endpoints and a
  swap passes every local test and breaks trust matching in production.
consumers:
  - "~/.config/assay/roster.env: fixed-here (writes `ASSAY_TRUSTED_BOT_SLUGS` role bindings; refuses to overwrite an existing file — appends a `# deskapps` block only when the keys are absent)"
  - "~/.config/assay/apps.env: fixed-here (`<APP>_INSTALL_ID` filled)"
  - "tools/desk/internal/deskkit/preflight.go: out-of-scope (called, not changed)"
---

# Brief 03 — Install and prove

## Context
files:
- `tools/desk/cmd/deskapps/install.go` (new), `prove.go` (new), `roster.go` (new), tests.
- `tools/desk/cmd/deskapps/page/` — Install and Verify cells, Screen 3.
- `docs/desk-tools/deskapps.md` (planned) — § Install, § Prove.

single-point-of-failure: the scopes check is the ONE control that says the App can do its duties.
The independent lower layer is the desk preflight itself, which runs the identical check at every
role boot from a different process on the token it mints then — a wrong pass here is caught at
first boot, and a wrong fail here cannot mask a real pass there. Row 6 proves the lower layer with
the upper one bypassed (a `--skip-prove` run followed by `deskroster preflight`).

facts:
- Installation URL: `https://github.com/apps/<slug>/installations/new`. After the click, poll
  `GET /app/installations` authenticated with the App JWT (RS256 over the PEM, 9-minute expiry —
  `desktoken` already builds this JWT; reuse, do not re-implement) every 5 s for up to 10 min; an
  installation whose `account.login` equals the target org completes the row; any other login
  renders "installed on `<login>`" with an uninstall link and keeps waiting.
- Bot USER id: `GET /users/<slug>%5Bbot%5D` → `.id` (the roster's `<bot-user-id>`); App id is
  from the conversion; installation id from the poll. Three numbers, three sources.
- Roster block written (only when `ASSAY_TRUSTED_BOT_SLUGS` is absent in `roster.env`; otherwise
  print the block and do not write): `team` → `reviewer=<p>-act:<uid>,worker=<p>-act:<uid>,…`
  for all six roles plus `read=<p>-read:<uid>`; `family` → one `role=<slug>:<uid>` per role.
  Mode 0600 preserved; a group- or world-writable file is refused (the tools refuse it anyway).
- Prove: `desktoken <role> --fresh` per bound role (two mints for `team`, six for `family`),
  then `deskkit.CheckAppScopes` per role. Tile values per design §2 Screen 3. Exit 0 on a full
  pass; exit 5 with the missing grant named when any role fails; exit 6 when the grant could not
  be read.
- Installation scope tile: `GET /installation/repositories` with the minted token →
  `repository_selection` (`all` or `selected`) and count.
- Cross-org: `--org` may repeat; the install step runs once per org and the bindings gain a
  per-org install id (`<APP>_INSTALL_ID_<ORG>`); a single `<APP>_INSTALL_ID` remains for the
  first org so existing readers keep working.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never widen a grant, never edit an installation's permissions, never call `gh workflow run`.
- Never overwrite an existing `roster.env` value.

## Task
1. Install cell: open the installation URL for the row's App; poll; render the found org, the
   wrong-org case, and the timeout ("still waiting — click Install, then come back").
2. Records: `<APP>_INSTALL_ID` (and per-org variant), bot user id, into `apps.env` and the state
   file; row → `installed`.
3. Verify cell + Screen 3: fresh mint per bound role via `desktoken … --fresh`, `CheckAppScopes`
   per role, tiles, and the console line per row.
4. Roster write per the facts; print the block when it cannot write.
5. `deskapps init` exit code follows the prove result; `--skip-prove` exists only for row 6 and
   says so in its help text.
6. Docs: § Install, § Prove in `docs/desk-tools/deskapps.md` (planned).
7. `mutations.json`: swap App id and bot user id in the roster write → row 4 red.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -run Install -count=1 && go test ./cmd/deskapps/ -run Prove -count=1 && go test ./cmd/deskapps/ -run Roster -count=1` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestInstallPollWrongOrg' -count=1 -v 2>&1 \| grep -cE -e 'installed on other-org' -e 'still waiting'` | ≥ 2 |
| 3 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestProveMissingGrant' -count=1 -v 2>&1 \| grep -cE -e 'contents=read \(want write' -e 'exit 5'` | ≥ 2 |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRosterBlock' -count=1 -v 2>&1 \| grep -cE -e 'reviewer=example-act:4242' -e 'read=example-read:4343'` | ≥ 2 (4242/4343 are the fake bot USER ids, not the fake App ids 1/2) |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestRosterNeverOverwrites' -count=1` | exit 0 — an existing `ASSAY_TRUSTED_BOT_SLUGS` line is untouched and the block is printed instead |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPreflightCatchesSkippedProve' -count=1` | exit 0 — with `--skip-prove` and a fake grant missing `issues:write`, a subsequent `deskroster preflight` (in-process) reports `app-scopes-vs-duties` FAILED (the independent lower layer) |
| 7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPerOrgInstallIDs' -count=1 -v 2>&1 \| grep -cE -e 'EXAMPLE_ACT_INSTALL_ID=' -e 'EXAMPLE_ACT_INSTALL_ID_ORG2='` | ≥ 2 |
| 8 | `grep -cE -e '^## Install' -e '^## Prove' docs/desk-tools/deskapps.md` | 2 |
| 9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | exit 0 — the id-swap mutant is caught by row 4 |
| 10 | `statusgen --root . --consumers --brief apps-installer/03` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
