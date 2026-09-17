---
brief: assay:assay:desk-supervision:11
title: Budget-driven recycle — retire a healthy worker before it degrades
why: >-
  A worker does not have to die to fail. A session that has spent most of its context window,
  or run for hours, keeps its claim and keeps answering — worse and worse — until it wedges or
  ships something degraded, and the derived-plane observer (briefs 01-03) cannot see it coming
  because every artifact still says "alive." The vitals from desk-supervision/10 make the
  approach measurable; this brief acts on them: recycle a HEALTHY worker at a budget threshold
  by having it hand off to durable state and exit, then respawn it fresh to resume. It is the
  opposite trigger from a reclaim — it fires on a full worker, not a silent one — and it turns
  slow degradation into a clean, logged, minutes-scale handover.
wave: 3
depends: ["desk-supervision/04", "desk-supervision/10"]
unblocks: ["desk-supervision/12"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief adds a NEW autonomous control that stops LIVE, HEALTHY workers on a self-reported
  signal — the derived plane only ever stopped dead or ineligible runs. The four risk answers
  are no (a recycle is reversible: the session hands off to durable state and a fresh session
  resumes), but a human should confirm the threshold policy, the graceful-exit protocol, and
  above all the hard-recycle backstop BEFORE the mechanism exists, not after — pre-empting live
  work automatically is exactly the class of control that gets a sign-off first.
issues: [351]
schema: brief-v2
authored: 2026-09-17 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §8.4 (retry + backoff) and §8.5 (active-run reconciliation; kill semantics) — the reactive-reclaim contrast this brief inverts — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "desk-supervision/10 — the `resource` block (context_pct_used, session_age_seconds, tokens, subagents_spawned, model), three-state, that this brief's evaluator reads."
  - "desk-supervision/04 — the lifecycle hooks (after_run / before_remove) whose run-end moment the graceful recycle fires through; the envelope is configuration, not prose."
  - "desk-supervision/02 — the per-run STOP.run.<key> flag + desk-window stop, reused UNCHANGED as the hard-recycle backstop for a worker that will not exit."
  - "desk-supervision/01 — the observer tick this evaluation hangs off, and its conservative-reclaim rule (could-not-check is never 'no life'); the recycle evaluator mirrors it (could-not-check is never 'over budget')."
  - "plugins/assay/skills/author-drive-plan and standing-note / deskfile — the durable hand-off surfaces a recycled worker writes to before exiting, so a fresh session resumes from record, not memory."
  - "freshness-checked 2026-09-17 @ daaa4b9c — no recycle policy or evaluator exists; `grep -rn -i recycle tools/desk` is empty."
exec-tier: strong
exec-tier-why: >-
  (a) and (c): the graceful-exit protocol and the grace-window-to-hard-recycle escalation are
  design decisions the facts do not fully pre-specify, and the mechanism stops live workers —
  a subtle slip (recycling on a blind vitals reading, or hard-recycling before the hand-off is
  durable) is safety plumbing that would pass its own happy-path tests.
decision-trigger: creation
consumers:
  - "tools/desk/internal/loopengine/recyclepolicy.go (new) RecyclePolicy + DefaultRecyclePolicy: fixed-here (the threshold policy — context_pct max, session-age max — as engine-internal Config data, the liveness.go pattern)"
  - "tools/desk/cmd/desksupervise/recycle.go (new): fixed-here (the recycle-eligibility evaluator, read from the resource block each tick, classifying RECYCLE-ELIGIBLE / GRACE / HARD-RECYCLE)"
  - "schemas/desksupervise-status-v1.json: fixed-here (each claim gains a `recycle` object {state, reason, since} so the decision is visible in the snapshot the console reads)"
  - "tools/desk/hooks.example.yaml (before_remove / after_run): fixed-here (the recycle fires the run-end hooks from desk-supervision/04; the graceful path is a hook-mediated exit)"
  - "the recycle DECISION consumer (deskd, the console): out-of-scope (a private consumer respawns the fresh session; the public contract is the threshold policy + the graceful-exit protocol + the snapshot `recycle` field it reads)"
  - "plugins/assay/skills/worker-desk/SKILL.md (the on-budget graceful-exit step): follow-up desk-supervision/11 (the skill body gains 'on RECYCLE-ELIGIBLE, write the hand-off and exit' once the protocol is proven in the implementation PR)"
version: 1
id: 3fa23219-219a-4f14-bf58-9e83ea4a03a1
---

# Brief 11 — Budget-driven recycle

> The two-planes framing that governs this brief is stated at the top of
> `desk-supervision/10`. In short: the derived plane reclaims a **dead/stalled** worker from
> artifacts; this brief recycles a **healthy-but-full** worker from its self-reported vitals.
> Different subject, different source, different trigger — a recycle can never suppress a
> reclaim, because a could-not-check vital yields no recycle signal at all.

## Context

files:
- `tools/desk/internal/loopengine/recyclepolicy.go` (new) — `RecyclePolicy{ContextPctMax,
  SessionAgeMax}`, `DefaultRecyclePolicy()`, engine-internal Config data (the `liveness.go`
  shape; a nil policy disables recycle entirely, strictly additive).
- `tools/desk/internal/loopengine/recyclepolicy_test.go` (new).
- `tools/desk/cmd/desksupervise/recycle.go` (new) — the evaluator: read the holder session's
  `resource` block, classify against the policy, emit `RECYCLE-ELIGIBLE` / `GRACE` /
  `HARD-RECYCLE` with a reason; integrated into `tick`/`status`.
- `tools/desk/cmd/desksupervise/recycle_test.go` (new).
- `schemas/desksupervise-status-v1.json` — each claim gains a `recycle` object.
- `tools/desk/hooks.example.yaml` — a documented `before_remove` note that the graceful path
  is hook-mediated.
- `tools/desk/cmd/desksupervise/testdata/` — recycle fixtures.
- `docs/desk-tools/` — a short recycle page: the threshold policy + the graceful-exit protocol.

single-point-of-failure: the recycle is defense-in-depth, so there is NO single control — that
is the design requirement, not an accident. The one thing both layers depend on is the vitals
being three-state (desk-supervision/10): a `could-not-check`/`null` reading yields NO recycle
decision, so a blind reading can neither over-recycle (kill a worker on a missing number) nor
be silently swallowed. Behind that: two independent recycle layers (below), which fail for
different reasons in different components.

facts:
- **Trigger.** A claim is `RECYCLE-ELIGIBLE` when its holder session's `resource` block reports
  `context_pct_used >= ContextPctMax` (default **50%**, tunable) OR `session_age_seconds >=
  SessionAgeMax` (default configurable; ships as a conservative wall value in
  `DefaultRecyclePolicy()`), whichever fires first. The reason names which threshold tripped.
- **could-not-check is never over-budget.** If `context_pct_used` and `session_age_seconds` are
  both `could-not-check`/`null`, the claim is `recycle: null` — no decision. This mirrors the
  observer's "could-not-check is never no-life" rule on the derived plane. A recycle is NEVER
  armed on a blind vital.
- **A recycle is a GRACEFUL retirement, not a reclaim.** The subject is HEALTHY. The sequence:
  (1) the session is signalled RECYCLE-ELIGIBLE; (2) it writes its hand-off to durable state
  (a drive-plan entry / standing-note / `deskfile`), so the next session resumes from record,
  not memory; (3) it exits cleanly; (4) it is respawned fresh and resumes. Contrast
  desk-supervision/01-03, which *reclaim* a *dead* run reactively — no hand-off, because the
  subject is already gone.
- **Two INDEPENDENT layers** (defense in depth, rule 10):
  - *Layer A — cooperative graceful exit.* The session, seeing RECYCLE-ELIGIBLE, refuses to
    pick up further work, writes its hand-off, and exits. This layer fails if the session is too
    degraded to cooperate.
  - *Layer B — the hard-recycle backstop.* When a session marked `GRACE` has not exited within
    the grace window, the observer arms the per-run stop of desk-supervision/02
    (`STOP.run.<key>` + the desk-window stop) — the SAME involuntary stop the derived plane
    uses — and the claim is released through the lock-guarded path so a fresh session can
    re-acquire it. This layer fails for a different reason (the worker ignoring a cooperative
    signal) in a different component (the stop flag + harness stop, not the session's own logic).
  The independence test holds: A is the session stopping itself; B is the supervisor stopping
  the session. Bypassing A (the session never cooperates) does not bypass B.
- **State machine on the claim's `recycle` field:** `null` (no signal) → `GRACE` (eligible,
  hand-off requested, grace window running) → cleared on clean exit, OR `HARD-RECYCLE` (grace
  window elapsed, backstop armed). Idempotent by marker: a claim already in `GRACE` is not
  re-signalled; a claim already `HARD-RECYCLE` does not re-arm the stop.
- **The decision consumer is private.** Respawning the fresh session is deskd's job (the
  console); the public contract is the policy, the protocol, and the snapshot `recycle` field.
  This brief ships the classifier and the backstop, not the respawn.
- Verb contract unchanged: kill switch first, one audit line per invocation, exit 0 · 3 · 5 · 6,
  fail closed. The evaluator's only writes are the same read-mostly set brief 01 permits (claim
  release, stop-flag, journal) — a recycle never writes a PR and never deletes a worktree.

## Human decision
<!-- gate: human — lifted verbatim into the decision issue; self-contained, no links/paths. -->
We are about to add an automatic control that stops LIVE, HEALTHY worker sessions when they
report they are running low on headroom (by default, half their context window used) or have
been running past a set wall-clock age. A stopped worker first writes a hand-off record and
exits cleanly, and a fresh worker is started to continue from that record. A worker that is too
degraded to cooperate and will not exit is force-stopped by a separate backstop after a grace
window. This is the first control in this system that pre-empts work that is still healthy and
running, rather than only reclaiming work that has died. It is reversible — no work is lost that
was written to the hand-off record — but a badly chosen threshold either recycles workers too
often (churn, wasted restarts) or too rarely (workers degrade before they are retired), and the
force-stop backstop is a real involuntary kill of a live session.

Options:
1. **Approve the defaults (context 50%, a conservative wall-age cap) with the graceful-exit +
   force-stop backstop.** Recommended. The mechanism ships tunable; the defaults can be adjusted
   from operational data without another sign-off. Recycle is armed only on a real measured
   vital, never on a missing or unreadable one.
2. **Approve graceful exit only, no force-stop backstop for now.** A worker that will not
   cooperate keeps its claim until the existing 120-minute stale-claim backstop frees it. Safer
   against a wrongful kill, weaker against a wedged-but-full worker.
3. **Hold.** Do not add automatic recycle; leave full-worker retirement to a human noticing it.

Default if no answer: none — blocks until answered (this is the gate that authorises an
autonomous stop of healthy work; it does not proceed on a timeout).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `recyclepolicy.go`: `RecyclePolicy{ContextPctMax float64, SessionAgeMax time.Duration}` +
   `DefaultRecyclePolicy()` (50%, a conservative wall age); a nil policy disables recycle.
   `EvaluateRecycle(resource, policy, now) (state, reason)` — three-state input in, no decision
   on all-blind vitals.
2. `recycle.go`: wire the evaluator into the observer tick and `status`; emit the classification
   per claim; on `GRACE`→grace-window-elapsed, escalate to `HARD-RECYCLE` by arming the
   desk-supervision/02 stop and releasing the claim; idempotent by marker.
3. Schema: add the required `recycle` object `{state: null|GRACE|HARD-RECYCLE, reason, since}`
   per claim; JSON validates.
4. Hooks note: the graceful path fires `after_run`/`before_remove` (desk-supervision/04).
5. Tests: context-over-threshold ⇒ RECYCLE-ELIGIBLE reason=context-budget; age-over-threshold ⇒
   reason=age-budget; healthy under-budget ⇒ no recycle; all-blind vitals ⇒ no recycle (the
   safety row); a `GRACE` claim past its window ⇒ HARD-RECYCLE with the stop armed (the
   backstop / negative-path row); idempotency (a second tick does not re-arm).
6. Docs page: threshold policy + graceful-exit protocol.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && GOWORK=off go test ./internal/loopengine/ -run 'Recycle' -count=1` | exit 0; output contains `ok` |
| 2 | check +flow | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/over-context.json --observations-fixture cmd/desksupervise/testdata/over-context-obs.json --beacons-fixture cmd/desksupervise/testdata/over-context-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["recycle"]; print(r["state"], r["reason"])'` | exit 0; output is `GRACE context-budget` |
| 3 | check | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T14:00:00Z --claims-fixture cmd/desksupervise/testdata/over-age.json --observations-fixture cmd/desksupervise/testdata/over-age-obs.json --beacons-fixture cmd/desksupervise/testdata/over-age-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["recycle"]["reason"])'` | exit 0; output is `age-budget` |
| 4 | check +dereference | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/healthy.json --observations-fixture cmd/desksupervise/testdata/healthy-obs.json --beacons-fixture cmd/desksupervise/testdata/healthy-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["recycle"])'` | exit 0; output is `None` (a healthy under-budget worker is not recycled) |
| 5 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestBlindVitalsNeverRecycles -v -count=1` | exit 0; output contains `--- PASS: TestBlindVitalsNeverRecycles` |
| 6 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestGraceWindowElapsedHardRecyclesAndArmsStop -v -count=1` | exit 0; output contains `--- PASS: TestGraceWindowElapsedHardRecyclesAndArmsStop` |
| 7 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestRecycleIsIdempotentAcrossTicks -v -count=1` | exit 0; output contains `--- PASS: TestRecycleIsIdempotentAcrossTicks` |
| 8 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` |
| 9 | check | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "recycle" in item["properties"]; print("ok")'` | exit 0; output is `ok` |
| 10 | check | `statusgen --root . --consumers --brief desk-supervision/11` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "a blind/missing vital is read as 'over budget' and a live worker is
killed" → rows 5, 4; "a worker that ignores the graceful signal keeps its claim forever" → row 6
(the backstop fires); "every tick re-arms the stop / re-requests the hand-off" → row 7; "a
healthy worker is recycled on churn" → row 4; "the decision is invisible to the console" → rows
8, 9. Review-only (the human gate): whether 50% / the wall-age default are the right thresholds,
and whether the grace window is long enough for a real hand-off — the knob, confirmed at
sign-off, not a code defect.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY — the decision issue must be answered before this lands.
