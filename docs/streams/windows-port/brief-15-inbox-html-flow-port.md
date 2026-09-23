---
brief: assay:assay:windows-port:15
title: deskinbox html + flow — the self-contained page renderer and the pipeline-flow model (split from windows-port/13)
why: >-
  windows-port/13 ported the oracle's shared engine and its two most-used renderings
  (`table`, `walk` — the `ask-decision` skill's entry point) and split off the remaining two
  renderings as too large for one PR (dispatch's own pre-authorization: "if it's too big for
  one PR, STOP and split ... keeping only the piece you were mid-implementing"). Those two —
  `--html` (the self-contained decision-queue page, PLUS the Flow section) and `--flow`/
  `--flow --html` (the pipeline flow model: statusgen/deskboard readers, a terminal table, and
  an inline-SVG stage diagram) — are a materially different, larger piece of work: a new
  reader (statusgen --bottleneck/--intake-debt/--net-flow, deskboard throughput) and an
  inline-SVG diagram builder, neither of which table/walk touch. windows-port/14's Windows
  leg (`deskinbox flow medici-finance/assay`) depends on THIS brief's deliverable, not
  windows-port/13's.
wave: 5
depends: ["windows-port/13"]
unblocks: ["windows-port/14"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1435]
schema: brief-v2
authored: 2026-09-22 by a windows-port/13 worker session, on a STOP-and-split (dispatch's own pre-authorization) — driver ask 2026-09-21 (the original brief-13 authoring session)
sources:
  - "plugins/assay/scripts/assay-inbox.sh (assay-inbox.sh:594-1233 — write_html_program, the flow reader collect_flow/write_flow_program, render_flow_text) — the engine for these two modes; its `--help` and plugins/assay/commands/inbox.md are the behavioural spec"
  - "tools/desk/cmd/deskinbox/{query.go,forge.go,format.go,detail.go} (windows-port/13, done) — the shared engine and format builder this brief REUSES rather than re-implements; walk.go is the pattern a html.go follows for card rendering"
  - "tools/desk/cmd/deskinbox/testdata/spec.md (windows-port/13) — the mode/flag contract and the split rationale, written at 13's pickup"
  - "plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md (windows-port/13 re-pointed the table/walk invocations; the --html and --flow invocations there still name the bash oracle — this brief re-points those)"
  - "docs/streams/windows-port/brief-14-windows-leg-proves-desk-role-paths.md:56 — the Windows leg step `deskinbox flow medici-finance/assay` this brief's deliverable must satisfy"
  - "freshness-checked 2026-09-22 (this worktree, mid windows-port/13): deskinbox table+walk implemented; no html/flow mode on the verb"
exec-tier: strong
exec-tier-why: >-
  Question (b): the flow model is DERIVED from four separate JSON readers
  (statusgen --bottleneck/--intake-debt/--net-flow, deskboard throughput) whose per-cell and
  fleet-aggregation rules (assay-inbox.sh:857-1231) must be reproduced exactly — including the
  could-not-check-vs-n/a distinction and the fleet-row "AT LEAST" partial-sum flag — and the
  html renderer must produce a self-contained page (no url(), no external asset) that the
  oracle's own test suite already asserts as a property, which this port must keep proving.
domain: complicated
parallel-streams:
  - {name: html, files: ["tools/desk/cmd/deskinbox/html.go", "tools/desk/cmd/deskinbox/html_test.go"]}
  - {name: flow, files: ["tools/desk/cmd/deskinbox/flow.go", "tools/desk/cmd/deskinbox/flow_test.go"]}
consumers:
  - "tools/desk/cmd/deskinbox/{html.go,flow.go} (new modes: html | flow | flow --html): follow-up windows-port/15 (this brief)"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up windows-port/15 (this brief — kept as the parity oracle until windows-port/14 retires it)"
  - "plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md (name the verb for html/flow; drop the 'not yet ported' notes windows-port/13 left): follow-up windows-port/15 (this brief)"
version: 1
---

# Brief 15 — `deskinbox html` + `deskinbox flow` (split from windows-port/13)

## Context
files: tools/desk/cmd/deskinbox/{html.go,flow.go,flow_parity_test.go,html_parity_test.go,testdata/**}, plugins/assay/commands/inbox.md, plugins/assay/skills/ask-decision/SKILL.md, changelog/windows-port-15-deskinbox-html-flow.md
facts:
- `html` reuses windows-port/13's fetchQueue + buildRendered for the cards (the SAME format builder walk.go already renders — the oracle shares write_format_program between its own `--walk` and `--html` and this port must keep that property, per windows-port/13's testdata/spec.md), then adds the self-contained-page wrapper (inline CSS, no `<script>`, no `src=`, no `@import`, no `url()`) plus the Flow section
- `flow` is a SEPARATE reader (assay-inbox.sh:857-1233): per-cell `statusgen --bottleneck/--intake-debt/--net-flow --json` (one invocation per root — statusgen refuses multi-root for these) plus ONE fleet-wide `deskboard throughput --json`; a reader that fails leaves its stage `could-not-check`, NEVER `0`
- the stage model (7 stages: intake/todo/in-progress/review/implemented/verified/done), the COUNT-vs-QUEUE distinction, and the fleet-row "AT LEAST" partial-sum flag are DERIVED rules from the oracle's write_flow_program jq (assay-inbox.sh:1017-1232) — port the rules, not just the shape
- `flow --html` renders the SAME flow model as an inline-SVG stage diagram (assay-inbox.sh:598-748) — an equivalent `<table>` must follow it with the same numbers (a diagram a screen reader cannot read is half the audience not served, per the oracle's own comment)
- readers are invoked via `os/exec` on `statusgen`/`deskboard` binaries (the oracle's own `ASSAY_STATUSGEN`/`ASSAY_DESKBOARD` env overrides, default: on PATH) — this is the ONE place in `deskinbox` that legitimately shells a subprocess, because the flow model's readers are OTHER desk binaries, not `gh`/`jq`/`make`; the no-shell-outs Verify row from windows-port/13 must be re-scoped here to name statusgen/deskboard specifically, not banned outright
single-point-of-failure: the flow-model parity test against the extracted oracle jq program (write_flow_program), run over canned reader-output fixtures (no live statusgen/deskboard call needed for that test, mirroring windows-port/13's format_parity_test.go approach); the independent layer is windows-port/14's live Windows-leg run of `deskinbox flow` against a real repo with real statusgen/deskboard binaries.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do not delete or edit the script (oracle); do not change the skill PROCEDURE, only the
  command it names.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Read assay-inbox.sh:594-1233 end to end; extend `tools/desk/cmd/deskinbox/testdata/spec.md`
   with the html/flow contract (page structure, the 7-stage model, the reader invocations) —
   checked by the reviewer against the script.
2. Port `html` (reuses windows-port/13's query+format engine; adds the page wrapper + Flow
   section).
3. Port `flow` and `flow --html` (the reader, the stage-model interpreter, the terminal table,
   the inline-SVG diagram).
4. Re-point `plugins/assay/commands/inbox.md` and `plugins/assay/skills/ask-decision/SKILL.md`'s
   remaining `--html`/`--flow` invocations to the verb; drop windows-port/13's "not yet ported"
   notes.
5. Changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go vet ./cmd/deskinbox/ && go test -count=1 ./cmd/deskinbox/` | exit 0 | `check` |
| 2 | **Parity flow**: `cd tools/desk && go test -count=1 -run 'TestParityFlow' -v ./cmd/deskinbox/ \| grep -c -- '--- PASS'` | `>= 1` (needs `jq` on the runner; could-not-check with reason otherwise) | `check +dereference` |
| 3 | **Parity html** (timestamp-normalised): `cd tools/desk && go test -count=1 -run 'TestParityHTML' ./cmd/deskinbox/` | PASS | `check +dereference` |
| 4 | Self-contained page: `deskinbox html /tmp/inbox-check.html medici-finance/assay && grep -c -e 'url(' -e '<script' -e ' src=' /tmp/inbox-check.html` | `0` | `check` |
| 5 | No unscoped shell-outs: `grep -rn 'exec\.Command' tools/desk/cmd/deskinbox/*.go \| grep -v _test.go \| grep -vc -e statusgen -e deskboard` | `0` (every exec.Command site names statusgen or deskboard, nothing else) | `check` |
| 6 | Windows build: `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskinbox/` | exit 0 | `check` |
| 7 | Command + skill re-pointed for html/flow: `grep -c 'deskinbox' plugins/assay/commands/inbox.md plugins/assay/skills/ask-decision/SKILL.md` | `>=` windows-port/13's own counts (strictly more references, not fewer) | `check` |
| 8 | **Flow — the skill's own example runs**: `deskinbox flow medici-finance/assay; echo rc=$?` and `bash plugins/assay/scripts/assay-inbox.sh --flow medici-finance/assay; echo rc=$?` on the same instant | the bottleneck line and the per-stage counts agree, modulo the two tools' different exit-code taxonomies (windows-port/13's testdata/spec.md divergence 4); needs live statusgen/deskboard binaries and a live minted token — could-not-check with reason otherwise | `check +flow` |
| 9 | Consumers routing corroborated: `statusgen --root . --consumers windows-port/15; echo $?` | `0` | `check` |
| 10 | Board lint: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: **model**. Reviewer's questions: (1) does the flow-model port reproduce the
could-not-check-vs-n/a distinction and the fleet-row "AT LEAST" partial-sum flag, or does a
blind reader silently read as zero anywhere? (2) is the html page genuinely self-contained
(row 4), and does it render through the SAME format builder walk.go uses, or has a second copy
of the five-part logic drifted in? (3) is every `exec.Command` site in this package scoped to
`statusgen`/`deskboard` only (row 5), with nothing that reaches `gh`/`jq`/`make`?
