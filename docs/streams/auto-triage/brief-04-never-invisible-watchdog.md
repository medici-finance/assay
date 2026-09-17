---
brief: assay:assay:auto-triage:04
title: "Never-invisible watchdog — escalate any red whole-tree gate that no responder acted on within N minutes"
why: >-
  An automated response is only trustworthy if its SILENCE is also caught. If a red gate is
  unclassifiable, a responder crashed, or the classifier had a bug, the red must not sit
  invisible — an alert that never fires is worse than none, because it manufactures false
  confidence. This brief is the fail-safe: a red whole-tree gate with no responder action
  within a bounded window is itself escalated, so no red ever goes silent.
wave: 3
depends: ["auto-triage/02", "auto-triage/03"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief lets the automation ESCALATE (file a needs-human notice) autonomously when no
  responder acted — an autonomous outbound action, so it is `gate: human` by authority
  boundary (cf. desk-tools/17; all four risk answers honestly `no` — an escalation is
  reversible at zero cost to `main`, and escalating-to-a-human is the SAFE direction). The
  human confirms: (1) the watchdog escalates on elapsed-time-with-no-responder-action
  regardless of WHY the responders were silent; (2) it does not double-escalate a red a
  responder already handled; and (3) the watchdog's OWN death is itself surfaced (an absent
  or failed watchdog run is visible via the platform's workflow-failure signal), so the
  fail-safe is not itself a silent single point.
design: DR-auto-triage
issues: []
schema: brief-v2
authored: 2026-09-16 by the-desk auto-triage authoring session
sources:
  - "docs/streams/auto-triage/spec.md — §2 invariant 3 (a red gate never goes invisible); the never-invisible guarantee"
  - "docs/streams/decisions/DR-auto-triage.md — the design record this brief is gated on (PROPOSED)"
  - "medici-finance/assay#611, #1119, #612 — the ambient reds the responders handle; this brief covers the residue they do NOT"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main) — no main-red/alert watchdog workflow exists (grep of .github/workflows/ for main-fail/main-red/alert returns nothing)"
exec-tier: strong
exec-tier-why: >-
  (b) correctness depends on cross-component reasoning — the watchdog must observe the
  responders' (02/03) action records and the gate state together, and (a) the value of N and
  the who-watches-the-watchdog design are decisions the facts do not pre-specify.
consumers:
  - "tools/autotriage/ (the responder action records from briefs 02/03 it observes): follow-up auto-triage/04 (this brief; flips to fixed-here when the watchdog reads those records)"
version: 1
id: 5c1f85bd-ef4f-421e-93c7-2eacbc68f9fd
---

# Brief 04 — never-invisible watchdog

## Context

single-point-of-failure: this brief EXISTS to remove a single point — the assumption that
"a responder always acts". Its own design must therefore not reintroduce one. Two
independent layers: the watchdog escalates on elapsed-time-with-no-action (independent of
WHY the responders were silent, and of the classifier), and the watchdog's own liveness is
surfaced out-of-band — an absent/failed scheduled watchdog run is itself visible through the
platform's workflow-failure notification, a signal the watchdog does not produce for itself.

risk-answer disposition: all four `risk:` answers are honestly `no` — the deliverable is a
scheduled trigger file, reversible at zero cost to `main` — and `gate: human` is carried by
*authority boundary* (arming an autonomous outbound escalation), which is a stricter gate
than the four `no`s alone would derive. This is not a contradiction with brief 02, which
refuses to auto-fix a culprit whose `file` is under `.github/workflows/`: 02's refusal is
about *auto-fixing* a workflow-config culprit unattended, while this brief's own deliverable
is a human-gated *authoring* act that adds a new trigger file — different act, different
control, both intact.

files:
- `.github/workflows/` — the scheduled watchdog trigger (planned). If open PR #1228's
  workflow-only-PR contract (a single PR touching ONLY `.github/workflows/**` plus its own
  changelog fragment) lands first, this trigger file travels as its own PR, separate from the
  `tools/autotriage/` logic below — cheap to split now, a blocked implementer later if not
  noted.
- `tools/autotriage/` — the watchdog logic reading gate state + brief 02/03 action records.
- `tools/autotriage/testdata/` — fixtures: a red gate with no responder record; a red gate
  with a responder record; a red gate of an unclassifiable check.

facts:
- Trigger: a bounded schedule (poll) over the current gate state. For each red whole-tree
  gate, the watchdog looks for a responder ACTION RECORD (an open PR/issue from brief 02 or
  a routed issue from brief 03, keyed on `check`+`file:line`) dated within N minutes of the
  red.
- No action record within N minutes → ESCALATE: file a `needs-decision`/`help wanted`
  notice naming the check and that no responder acted, so a human is pulled in. It escalates
  regardless of cause (crashed responder, unclassifiable red, classifier bug).
- An action record present → do nothing (no double-escalation); dedupe on the same key so a
  red persisting across polls escalates at most once.
- N is configurable; the default is tuned by WIDENING before tightening — a spurious
  escalation costs a human a glance, a missed silent red costs the failure this brief guards.
- The watchdog NEVER fixes, merges, ready-flips, or edits a gate. It observes and escalates.
- Who-watches-the-watchdog: the scheduled run's own failure/absence is surfaced by the
  platform (a failed/failing workflow run is itself visible), so the fail-safe is not a
  silent single point.
- **Escalation bodies are sanitized before they are published**, the same rule briefs 02/03
  apply: any CI-derived text the watchdog names (the `check`, not the withheld detail of an
  opaque red) is truncated, fenced, and rendered inert before it enters the escalation
  artifact.
- **Arming mechanism.** Same repository-tracked config surface as briefs 02/03 (never an
  environment variable or a value the watchdog could set for itself) — the same path
  brief 02's refused-path allow-list protects.

## Ground rules
- NEVER fix, merge, ready-flip, self-approve, or edit a gate — escalate only.
- File the escalation ONLY through the sanctioned desk write path; never a hand-rolled mint;
  live writes only when armed (default `--dry-run`) via the repository-tracked config.
- NEVER echo a withheld/opaque-red token — the escalation names the CHECK, not the detail.
- **The forge-write identity this watchdog posts under holds NO merge authority, NO
  review-submission authority, NO branch-protection-bypass authority, and NO
  workflow-write/workflow-dispatch authority** — the same credential-layer control briefs
  02/03 state, applied here too since this brief also posts autonomously.
- Stop at `implemented`. Feature branch + draft PR only. Never commit `STATUS.md`/`FINDINGS.md`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the watchdog: enumerate red whole-tree gates, join each to any brief-02/03 action
   record on the `check`+`file:line` key within the N-minute window.
2. Escalate any red with no action record in-window (file a needs-human notice naming the
   check), regardless of why the responders were silent.
3. Suppress escalation when a responder acted, and dedupe so a persistent red escalates once.
4. Make N configurable; wire the scheduled trigger; ensure the watchdog's own run
   failure/absence is surfaced out-of-band.
5. `--dry-run` prints intended escalations and writes nothing (test default).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/autotriage && go build ./... && go vet ./...` | exit 0 | check |
| 2 | `cd tools/autotriage && go test ./...` | exit 0; all green | check |
| 3 | `cd tools/autotriage && go test -run TestEscalatesRedWithNoResponderAction -v` | exit 0; a red gate with no action record within N minutes produces exactly one escalation naming the check — dereferences the core behaviour | check +dereference |
| 4 | `cd tools/autotriage && go test -run TestNoDoubleEscalationWhenResponderActed -v` | exit 0; a red gate WITH a responder action record in-window produces NO escalation (negative row: proves it observes responder action, not just gate redness) | check |
| 5 | `cd tools/autotriage && go test -run TestEscalatesUnclassifiableRedRegardlessOfCause -v` | exit 0; a red of an unclassifiable check with no responder record still escalates — proves cause-independence (the failure mode #611-class automation would miss) | check |
| 6 | `cd tools/autotriage && go test -run TestEscalationNamesCheckNotWithheldDetail -v` | exit 0; a leak-sweep-red escalation body names the check and contains NO withheld token/detail (public-repo safety) | check |
| 7 | `cd tools/autotriage && go test -run TestWatchdogAbsenceIsSurfaced -v` | exit 0; asserts the watchdog run emits a failure/heartbeat signal such that its own absence is detectable out-of-band (who-watches-the-watchdog layer) | check |
| 8 | `cd tools/autotriage && go test -run TestScheduledTriggerJoinsGateStateAndResponderRecordsThenEscalates -v` | exit 0; drives the path end to end on fixtures — a simulated scheduled-trigger invocation reads the current gate-state fixture AND the brief-02/03 action-record fixture through the same entry point the real trigger calls, and produces exactly one escalation naming the check. Fails if the trigger's entry point is stubbed, or if the join is exercised only through direct calls into the escalation function (rows 3–7) rather than through the wiring the schedule actually invokes | check +flow |
| 9 | `cd tools/autotriage && go test -run TestEscalationBodySanitizesParsedText -v` | exit 0; an escalation for a check whose diagnostic text is CI-derived wraps that text in a fenced block with no live markdown link or directive content passed through | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     gate: human — green rows are INPUT to the human review (escalate-only; cause-independent;
     self-surfacing), not a substitute. Status stays `implemented` until the human confirms
     and DR-auto-triage is approved. -->

## Review
Gate: human (autonomous outbound action; authority widening — see gate-why; design:
DR-auto-triage). Reviewer answers both core-control questions: (1) the single control this
brief removes is "a responder always acts"; the layers making the removal safe are
cause-independent escalation and out-of-band watchdog-liveness — confirm both; (2) rows 4/5
prove the lower layer catches the fault when the happy path (a responder acted) is bypassed,
and row 7 proves the fail-safe is not itself a silent single point. Reviewer also confirms
row 9 closes the sanitize-before-publish gap raised on security review, that the arming
config and credential-layer ground rules are stated, and that the `.github/workflows/`
trigger note about #1228's contract still holds. Verdict + date in the stream README table.
