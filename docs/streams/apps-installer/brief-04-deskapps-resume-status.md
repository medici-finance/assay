---
brief: apps-installer/04
title: "`deskapps resume` / `status` — per-App state machine, throttle pause, expired-code re-arm, page verbs"
why: >-
  GitHub pauses App creation on an account after roughly four Apps, silently and for an
  unstated time. Measured by hand on the six-App runbook, that turned every full-suite setup into a
  restart with half the records already written. A per-App state record and a resume verb turn the
  throttle into a wait, and the same verb, exposed as a button on the page and as a line to the
  launching console, means the person and the Claude Code session driving the install see and
  drive one state machine.
wave: 2
depends: ["apps-installer/02"]
unblocks: ["apps-installer/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-05 by apps-installer authoring session
sources:
  - "./design.md §4 (state machine and its edges), §2 Screen 2 (throttle banner, Resume creation, console strip), §6 (`deskapps resume`, `status`), §8 (throttle, code expired, port in use)."
  - "Operator observation 2026-09-05: about four Apps can be created before the account is throttled; the throttle's duration is not documented by GitHub."
  - "apps-installer/02 — writes `apps.state.json` (`deskapps-state-v1`) and flips a row to `paused` after 10 minutes without a callback; this brief owns everything that reads that state back."
  - "freshness-checked 2026-09-05 @ 38e96f7 (origin/main) — no state record or resume verb exists."
exec-tier: any
consumers:
  - "~/.config/assay/apps.state.json: fixed-here (read/write; schema owned by apps-installer/02)"
  - "docs/desk-tools/deskapps.md: fixed-here (§ Resume, § Status, § Page verbs)"
---

# Brief 04 — Resume and status

## Context
files:
- `tools/desk/cmd/deskapps/state.go` (planned) (transitions), `resume.go`, `status.go`, `verbs.go` (page →
  CLI verbs), tests.
- `tools/desk/cmd/deskapps/page/` — the throttle banner, the console strip, the Resume / Close
  buttons, the Retry and Cancel controls.
- `docs/desk-tools/deskapps.md` (planned).

single-point-of-failure: the state file is the ONE record resume trusts. The independent layer:
before acting on a `keyed` or later row, resume re-reads GitHub (`GET /app` with the row's PEM)
and reconciles — a row the file calls `keyed` whose PEM is missing or whose App no longer exists
is demoted to `pending` with a line saying why, so a stale or hand-edited file cannot make the tool
skip a step. Row 5 proves it.

facts:
- Transitions and causes: design §4 table, verbatim.
- `paused` is not an error: `deskapps init` exits 0 when every remaining row is `paused`, printing
  `N created, M paused by GitHub's creation throttle — run: deskapps resume`.
- `deskapps resume`: reconcile every row against GitHub; re-arm `paused` and expired `posted`
  rows to `pending`; start the server; open the run board. Idempotent.
- `deskapps status`: print the board as text (one line per App, state, ids, avatar confirmed),
  exit 0 when all rows are `verified`, 5 otherwise; `--json` emits the state file.
- Page verbs: `POST /verb` with `{verb: resume|retry|confirm-avatar|cancel, app, state}` — the
  four verbs only; unknown verb → 400; missing or wrong nonce → 403. Each verb prints the console
  line `page → <verb> requested · <detail>` before acting.
- Port: on bind failure use the next free loopback port; the manifest `redirect_url` is derived
  from the port actually bound, so a resumed run on a new port re-renders forms, never reuses the
  old URL.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl.
- Stop at `implemented`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never retry a Create on the person's behalf; the throttle is waited out by them, not polled by us.

## Task
1. `state.go`: the transition table as data, with a single `Apply(row, event)` and a test per edge.
2. Throttle handling: the 10-minute timer from brief 02 emits the `paused` event here; banner and
   buttons render from state; `init` exit semantics as in facts.
3. `resume` with reconciliation; `status` (+ `--json`).
4. `/verb` handler with nonce check, the four verbs, console lines.
5. Port fallback and redirect derivation.
6. Docs sections.
7. `mutations.json`: remove the reconciliation step → row 5 red; accept a fifth verb → row 6 red.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./cmd/deskapps/ -run State -count=1 && go test ./cmd/deskapps/ -run Resume -count=1 && go test ./cmd/deskapps/ -run Verb -count=1` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestStateTable' -count=1 -v 2>&1 \| grep -cE -e 'posted->paused' -e 'paused->pending' -e 'posted->posted.*expired' -e 'verified->installed'` | ≥ 4 |
| 3 | `cd tools/desk && go build -o /tmp/deskapps ./cmd/deskapps && HOME=$(mktemp -d) sh -c 'mkdir -p $HOME/.config/assay; cp tools/desk/cmd/deskapps/testdata/state-two-paused.json $HOME/.config/assay/apps.state.json; /tmp/deskapps status; echo rc=$?' 2>&1 \| grep -cE -e 'paused' -e 'rc=5'` | ≥ 2 |
| 4 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestInitExitsZeroWhenPaused' -count=1 -v 2>&1 \| grep -cE -e 'paused by GitHub' -e 'deskapps resume'` | ≥ 2 |
| 5 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestResumeReconcilesMissingPem' -count=1` | exit 0 — a `keyed` row with no PEM on disk is demoted to `pending` and the reason printed (independent layer over the state file) |
| 6 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestVerbHandler' -count=1 -v 2>&1 \| grep -cE -e 'unknown verb.*400' -e 'bad state.*403' -e 'page → resume requested'` | ≥ 3 |
| 7 | `cd tools/desk && go test ./cmd/deskapps/ -run 'TestPortFallbackRedirect' -count=1` | exit 0 — with the default port occupied, the bound port differs and every rendered manifest's `redirect_url` carries the bound port |
| 8 | `grep -cE -e '^## Resume' -e '^## Status' -e '^## Page verbs' docs/desk-tools/deskapps.md` | 3 |
| 9 | `cd tools/desk && go test ./cmd/deskapps/ -run 'Mutation' -count=1` | exit 0 — both mutants caught |
| 10 | `statusgen --root . --consumers --brief apps-installer/04` | exit 0 (routing claims corroborated against the diff) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
