---
stream: auto-triage
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: [611, 1119, 612, 880, 536]
board: generated
spec: docs/streams/auto-triage/spec.md
---

# auto-triage Stream

**Keep the all-stop gate; automate the RESPONSE to it.** When a whole-tree gate reddens —
a red `main`/post-merge check, or an ambient `main` defect every open PR inherits — the
desired behaviour is that work stops: the red is the signal that something somewhere is
wrong. What is manual today is everything that happens *after* the red: reading the log,
naming the culprit check and the failing `file:line`, deciding mechanical-vs-judgement,
filing the bug, and either opening a fixing PR or routing it. This stream automates that
response so a human/desk no longer hand-triages every ambient red — while never weakening,
narrowing, or silencing the gate, and never merging (the human still merges every fix).

Provenance — instances where an unrelated ambient defect reddened gates and someone
hand-triaged it: medici-finance/assay#611 and #1119 (gofmt nits tripping unrelated
Verify-table rows), #612 (a load-induced timing flake failing the whole-module test
first-attempt), #880 (stale remediation text), #536 (a duplicated reducer). See
[spec.md](spec.md) for the principle, the four-step response, what the culprit identifier
can and cannot see, the non-goals, and the human-gate posture.

**This stream is `parked` (not yet approved).** Its [spec.md](spec.md) is `**Status:**
draft`; approval is a human act — the merge that flips the spec to `approved` and this
README to `status: active`. Parked keeps the briefs authored and reviewable without
claiming an approval that has not happened.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Culprit identifier — parse a failing whole-tree gate into check + file:line + mechanical/judgement class](brief-01-culprit-identifier.md) | 1 | M | todo | — | — |
| 02 | [Mechanical responder — file the bug and open a fixing draft PR, autonomously, for the mechanical class](brief-02-mechanical-responder.md) | 2 | M | todo | — | — |
| 03 | [Judgement responder — file the bug and route it with a recommended default, for the judgement and opaque classes](brief-03-judgement-responder.md) | 2 | M | todo | — | — |
| 04 | [Never-invisible watchdog — escalate any red whole-tree gate that no responder acted on within N minutes](brief-04-never-invisible-watchdog.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`auto-triage/01` (culprit identifier — parse the failing gate into `check` + `file:line`
+ mechanical/judgement class) → `auto-triage/02` (mechanical responder — file + open a
fixing draft PR) and `auto-triage/03` (judgement responder — file + route with a default),
which fan out in parallel → `auto-triage/04` (never-invisible watchdog — escalate a red
gate no responder acted on within N minutes).

**Smallest unblocking move:** land `auto-triage/01`. It is the real head — every responder
consumes its `check`/`file:line`/class output, and it is where the mechanical/judgement
boundary and the *leak-sweep-is-never-mechanical* rule are drawn. Its head was verified
against the live workflows @ `e9fa19d3`: the readable jobs (`ci` → `build-test` /
`plugin-shell-suites` / `skillslint`, `truth-suite`, `changelog-check`, `pin-consistency`,
`plugin-drift`, `assay-statusgen`) expose the failing `file:line`, while the one opaque
gate (`leak-sweep`) withholds its detail by design and so has a defined non-guessing
fallback (always route to judgement). 01 therefore has no hidden upstream blocker.

**Tempting-but-wrong first step:** wiring the auto-fixer (02) first. It is dead on arrival
without 01's classifier — an auto-fixer with no reliable culprit + class would either do
nothing or attempt to fix reds it must never touch (leak-sweep). Build the classifier, then
the responders.

## Dependency waves

- **Wave 1** — `auto-triage/01` (culprit identifier; depends on nothing; `gate: model`).
- **Wave 2** — `auto-triage/02` (mechanical responder) and `auto-triage/03` (judgement
  responder), both depend on 01 and are parallelizable; both `gate: human` (autonomous
  outbound action; cite `DR-auto-triage`).
- **Wave 3** — `auto-triage/04` (never-invisible watchdog; depends on 02 + 03 — it observes
  whether a responder acted; `gate: human`, cites `DR-auto-triage`).

Path: `01 → {02, 03} → 04`.

## Shared conventions

- **Never weaken a gate.** Every brief carries the explicit Definition-of-Done line: the
  deliverable must not narrow, silence, or edit any gate to make a red go green.
- **Never merge / ready-flip / self-approve.** Autonomous write authority is bounded to
  filing an issue, opening a *draft* PR, posting a routing default, and escalating.
- **Never auto-fix an opaque red.** A red whose culprit cannot be resolved to a concrete
  `file:line` + a deterministic fix routes to judgement; `leak-sweep` is the standing case.
- Feature branch + draft PR per brief; never commit `STATUS.md` / `FINDINGS.md` on a branch.
- The three autonomous-action briefs (02/03/04) cite `DR-auto-triage`
  ([../decisions/DR-auto-triage.md](../decisions/DR-auto-triage.md)) via their `design:`
  key and may not advance past `todo` until that record is approved.
