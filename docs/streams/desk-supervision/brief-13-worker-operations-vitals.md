---
brief: assay:assay:desk-supervision:13
title: Worker-operations vitals — the self-report resource block
why: >-
  The observer built by briefs 01-07 reclaims a DEAD or STALLED worker from artifacts it
  never has to trust the worker for. It has no reading at all on the other way a healthy
  worker fails: it fills up. Context exhaustion, a long-lived session and a growing subagent
  fan-out are things ONLY the session itself can see, and the snapshot leaves the one field
  that would carry them (`tokens`) a reserved could-not-check stub. Filling that stub with a
  small self-reported resource block turns "the worker degraded silently until its answers got
  worse" into a measured signal a consumer can act on before the worker dies — which is what
  the recycle trigger (desk-supervision/14) needs to exist at all.
wave: 2
depends: ["desk-supervision/07"]
unblocks: ["desk-supervision/14", "desk-supervision/15"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [351]
schema: brief-v2
authored: 2026-09-17 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §13.5 (session metrics and token accounting as part of the runtime interface) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "desk-supervision/07 — the runtime snapshot and schemas/desksupervise-status-v1.json, whose per-claim `tokens` field is `const: could-not-check` and documented as 'present so a future harness binding can fill it'. This brief is that binding."
  - "tools/desk/internal/deskkit/ackbeacon.go — the field-preserving read-modify-write onto <StateDir>/roster/<session>.json (deskroster and deskack co-own fields on one beacon; each merges and preserves the other's keys). The vitals write reuses exactly this pattern."
  - "tools/desk/internal/loopengine/liveness.go — the DERIVED plane: 'derive liveness from artifacts, add NO worker-side protocol'. The two-planes framing (below) reconciles this brief's self-report with that invariant."
  - "docs/three-state-instrument-rule.md — measured / could-not-check / null, never a fabricated zero."
  - "freshness-checked 2026-09-17 @ daaa4b9c — schemas/desksupervise-status-v1.json still carries the reserved `tokens` stub; no `resource` block exists; `grep -rn resource_pct tools/desk` is empty."
exec-tier: strong
exec-tier-why: >-
  (b): correctness is cross-artifact — the schema, the self-report emit path, and the
  `desksupervise status` renderer must agree on one contract a private console consumes, and
  the three-state discipline (never a fabricated 0) has to hold identically in all three.
consumers:
  - "schemas/desksupervise-status-v1.json: fixed-here (the reserved `tokens` stub is promoted into a `resource` block and filled; this is the documented intended evolution of the stub, so the schema id stays v1)"
  - "tools/desk/cmd/desksupervise/status.go: fixed-here (the status/JSON renderer surfaces the resource block, joining a claim's holder session to that session's beacon vitals)"
  - "tools/desk/cmd/deskroster/sets.go (the beacon `set` write): fixed-here (gains optional vitals flags; unset ⇒ the field is written null, never 0)"
  - "tools/desk/internal/deskkit/ackbeacon.go (beacon write pattern): fixed-here (a sibling field-preserving merge writes `resource`; it must not drop `acks`/`open_work`)"
  - "an operator console consuming the JSON (deskd's repo): out-of-scope (a private consumer; the versioned schema is the contract it reads. Because `tokens` moves from a claim-level const into `resource`, the console's reader is updated in its own repo when this lands — noted, not owned here)"
version: 1
id: f167b9b6-ec90-4766-87de-da21aa47ee2b
---

# Brief 13 — Worker-operations vitals: the self-report resource block

## The framing principle — two orthogonal planes (governs briefs 13, 14, 15)

This delta and the machinery it sits on are **two orthogonal planes**, and keeping them
separate is what makes the delta safe.

- **The DERIVED plane — desk-supervision as already built (briefs 01-09).** Liveness is
  *reclaimed from artifacts* the worker cannot fake: audit lines, branch-SHA movement, PR
  updates, claim refs. It never asks the worker how it is doing, and `liveness.go` says so
  outright — "derive liveness from artifacts, add NO worker-side protocol." Its subject is a
  **dead or stalled** worker; its action is **reclaim**; its trigger is the absence of any
  observable sign of life.

- **The WORKER-OPERATIONS plane — this delta (briefs 13-15).** Some facts are *only* knowable
  to the session itself: how much of its context window is spent, how many tokens it has
  burned, how long it has been alive, how many subagents it has spawned, which model it runs.
  These are **self-reported vitals**. Their subject is a **healthy-but-full** worker; the
  consuming action (brief 14) is **recycle before it dies**; the trigger is a budget threshold,
  not silence.

**Why they do not collide with the "never self-report" invariant.** That invariant is a
*work-plane / derived-liveness* rule: a worker must never be the source of truth about *the
work* — whether its item is done, whether it is still alive, whether its claim should hold.
This plane never touches that. A session reporting "I have used 60% of my context window"
makes **no claim about the work**: it cannot mark an item done, cannot refute a reclaim, cannot
authorise anything. It is a vital sign about the *session as a process*, consumed only to
retire that process gracefully. Different subject (dead vs full), different source (derived vs
self-report), different trigger (silence vs budget) — so the derived plane keeps deriving
liveness the moment this vital is stale or absent, and nothing this brief adds can be used to
suppress a reclaim. That last property is the load-bearing one, and the three-state discipline
below is what enforces it.

## Context

files:
- `schemas/desksupervise-status-v1.json` — promote the per-claim `tokens` stub into a
  `resource` object and fill it.
- `tools/desk/internal/deskkit/vitals.go` (new) — `ResourceVitals` type + a
  field-preserving merge writer onto `<StateDir>/roster/<session>.json`, mirroring
  `ackbeacon.go`'s `AppendAck` exactly (load raw keys, touch only `resource`+`session`,
  round-trip every other key untouched).
- `tools/desk/internal/deskkit/vitals_test.go` (new).
- `tools/desk/cmd/deskroster/sets.go` — `set` gains optional vitals flags (the per-tick emit
  path the desk session already calls).
- `tools/desk/cmd/desksupervise/status.go` — the renderer joins each claim's holder session to
  that session's beacon `resource` and emits it (table + JSON).
- `tools/desk/cmd/desksupervise/testdata/` — a beacon fixture carrying a `resource` block.
- `tools/desk/README.md` — note the new `deskroster set` vitals flags.

single-point-of-failure: the three-state discipline is the one control this design depends on —
a missing or unreadable vital must render `null` / `could-not-check`, **never a fabricated 0**.
The layer behind it is on the consumer side and independent: brief 14's recycle evaluator treats
`null`/`could-not-check` as "no recycle signal", never as "0% used ⇒ safe to keep", so even if
the emit path regressed to writing 0 the consumer still would not act on a bare 0 without a
`measured` marker. The two fail for different reasons in different components: the emitter's
"never write 0 for unknown", and the consumer's "never recycle-or-hold on an unmarked value."

facts:
- **The reserved stub.** `schemas/desksupervise-status-v1.json` today carries per claim
  `"tokens": { "const": "could-not-check" }`, described as "present so a future harness binding
  can fill it." This brief supplies that binding: `tokens` moves into a new `resource` object
  and becomes three-state.
- **The beacon is the emit surface.** `<StateDir>/roster/<session>.json` is a shared,
  field-preserving object — `deskroster` owns `session/role/updated/open_work`, `deskack` owns
  `acks`, and `ackbeacon.go` shows the merge pattern (load raw `map[string]RawMessage`, write
  only your key, preserve the rest, fail closed on a parse error rather than blank another
  writer's fields). Vitals become one more such co-owned key, `resource`.
- **The desk session is the only writer of its own vitals.** The per-tick emit is a
  `deskroster set` call the desk window already makes each cadence tick; it gains the optional
  vitals flags. The cadence IS the tick (tunable via the loop's existing interval); no new
  timer is introduced.
- **The `resource` block fields**, each three-state (`measured value` | the string
  `could-not-check` | `null`):
  - `tokens` — cumulative tokens the session has consumed (integer when measured).
  - `context_pct_used` — percent of the context window in use, 0–100 (number when measured).
  - `session_age_seconds` — wall seconds since the session booted (integer when measured).
  - `subagents_spawned` — count of subagents this session has launched (integer when measured).
  - `model` — the model id the session runs (string when measured).
- **Three-state, restated because it is the safety property:** a flag not passed ⇒ the field is
  written `null` (not collected this tick); a flag whose source read failed ⇒ `could-not-check`;
  a real reading ⇒ the value. `0` is a legitimate MEASURED value (e.g. `subagents_spawned: 0`)
  and therefore may NEVER be used to mean "unknown."
- **This is the only strictly-new collection in a vitals report.** Everything else an operator
  would want in a "how is this worker doing" view is already derivable from existing instruments
  — liveness from the observer (brief 01), activity from the roster beacon and metrics-harvest,
  aggregates from opmetrics, decisions/concerns from filed issues (deskfile). This brief adds the
  one plane none of them can see: the session's own resource state.
- Verb contract unchanged: deskkit kill switch first, one audit line per invocation, exit
  0 · 3 · 5 · 6, fail closed. The status renderer stays read-mostly (brief 07's property).
- **The beacon's trust boundary, stated explicitly (security review finding S-1).** Nothing in
  `deskroster set` today binds a beacon to the session it names: the caller passes an optional
  `--session NAME`, or the name resolves from the session environment, and either way it is a
  caller-chosen string joined unvalidated into `roster/<session>.json`. Once desk-supervision/14
  makes a GRACE/HARD-RECYCLE reading on this beacon trigger an involuntary stop, that write
  becomes a control input, not just a status display, so the boundary has to be named rather than
  left implicit. **The intended boundary is: same uid, same host, all desk sessions on that host
  mutually trusted** — a house-cell operator's own windows, not a multi-tenant surface — mirroring
  the trust domain the rest of this tree already runs under (ambient config home, shared roster).
  Given that boundary, `deskroster set` does not need to authenticate a beacon write beyond that
  domain; it does, however, need to fail predictably outside it: a session-name argument that does
  not resolve to a single path segment (no `/`, no `..`, no empty string) is refused (exit 5),
  never silently joined. A read of a malformed or unreadable beacon renders every vital
  `could-not-check`, never `measured` — an absent or corrupt binding is a blind reading, not a
  trusted one, so it can arm a recycle GRACE/HARD-RECYCLE decision under desk-supervision/14 no
  more than a missing vital can (that brief's own three-state rule already covers this case; this
  bullet is what makes the input to that rule well-defined).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **`ResourceVitals` + writer** (`vitals.go`). Define the five-field type (each field a
   pointer / optional so "unset" is distinguishable from a measured 0). `MergeResourceVitals(session, v)`
   loads the beacon as raw keys, sets only `resource` (marshalling unset fields as JSON `null`,
   never `0`) and `session`, preserves every other key, and fails closed on a parse error —
   the `AppendAck` shape. A field whose source the caller could not read is written as the
   string `could-not-check`, distinct from `null`.
2. **Emit flags** (`deskroster set`). Add `--tokens`, `--context-pct`, `--session-age-seconds`,
   `--subagents`, `--model`, each optional; a flag present with the sentinel value `unknown`
   writes `could-not-check`; a flag absent leaves the field `null`. `set` continues to preserve
   `acks`/`open_work` (regression test). **Session-name shape check** (security review finding
   S-1, worth-fixing-here): `--session`/the resolved session id must resolve to a single path
   segment (no `/`, no `..`, non-empty) before it is joined into the beacon path; a name that
   fails the check is refused (exit 5), never silently joined — this brief is what makes the
   session-keyed path newly writable with control-bearing fields, so it is the moment to close it,
   even though the unvalidated join itself pre-dates this brief.
3. **Schema** (`desksupervise-status-v1.json`). Replace the `tokens` const with a required
   `resource` object on each claim item (`additionalProperties: false`), each field
   `oneOf` [its measured type, `{const: could-not-check}`, `null`]. Keep the schema id
   `desksupervise-status-v1` (filling the reserved stub is the documented intended evolution) and
   update the top-of-file description to say `tokens` now lives in `resource` and is fillable.
4. **Renderer** (`status.go`). For each claim, resolve the holder session's beacon, read its
   `resource`, and emit it in the table and the JSON. A claim whose holder session has no beacon,
   or an unreadable beacon, renders every resource field `could-not-check` (never `null`, never 0)
   — an absent reading is a blind reading, not a zero.
5. **Tests**: writer preserves `acks`; unset field serialises `null` not `0`; a measured
   `subagents_spawned: 0` round-trips as `0` and reads back as measured; the JSON validates
   against the updated schema; the flow row below.
6. **README**: the new `deskroster set` vitals flags.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | exit 0; output contains `ok` |
| 2 | check | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | exit 0; output contains `--- PASS: TestMergeResourceVitalsPreservesAcks` |
| 3 | check | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | exit 0; output contains `--- PASS: TestUnsetVitalIsNullNotZero` |
| 4 | check | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | exit 0; output contains `--- PASS: TestMeasuredZeroSubagentsRoundTrips` |
| 5 | check +flow | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | exit 0; output is `60 0 example-model` |
| 6 | check +dereference | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | exit 0; output is `could-not-check` (an absent beacon is blind, never 0) |
| 7 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` |
| 8 | check | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | exit 0; output is `ok` |
| 9 | check | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | output is `1` or more |
| 10 | check | `statusgen --root . --consumers --brief desk-supervision/13` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |
| 11 | check | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | exit 0; output contains `--- PASS: TestSetRefusesMultiSegmentSessionName` (a `--session` value containing `/` or `..` is refused, exit 5, never joined into the beacon path) |

Pre-mortem → detection: "an unset or unreadable vital renders 0 and a consumer reads it as
'plenty of headroom'" → rows 3, 6 (unset ⇒ null; absent beacon ⇒ could-not-check; never 0);
"a measured 0 (no subagents) is mistaken for 'unknown' and dropped" → row 4; "the vitals write
clobbers the ack/open_work fields the beacon co-owns" → row 2; "the JSON drifts from the schema
the console reads" → rows 7, 8; "tokens stays a dead const stub" → row 8; "a malformed session
name is joined into the beacon path unvalidated" (security review finding S-1) → row 11.
Review-only: whether the chosen field set is the right vitals to collect (a knob for brief 14 to
consume, not a defect here).

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

Run against merged origin/main 50989dbc58f2cd95276dc8fe27314d3a2e15e3f8 from an isolated
detached worktree. Runner form as printed by statusgen verifyrun.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1 | exit 0; output contains ok | exit 0; ok github.com/.../tools/desk/internal/deskkit 0.449s | 2026-09-23 | opus-5.5-verifier |
| 2 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitals-PreservesAcks -v -count=1 | exit 0; --- PASS line | exit 0; --- PASS: TestMergeResourceVitals-PreservesAcks (0.00s); ok | 2026-09-23 | opus-5.5-verifier |
| 3 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1 | exit 0; --- PASS line | exit 0; --- PASS: TestUnsetVitalIsNullNotZero (0.00s); ok | 2026-09-23 | opus-5.5-verifier |
| 4 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagents-RoundTrips -v -count=1 | exit 0; --- PASS line | exit 0; --- PASS: TestMeasuredZeroSubagents-RoundTrips (0.00s); ok | 2026-09-23 | opus-5.5-verifier |
| 5 | cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 (parse resource fields) | exit 0; output is 60 0 example-model | exit 0; output: 60 0 example-model | 2026-09-23 | opus-5.5-verifier |
| 6 | cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 (print resource.tokens) | exit 0; output is could-not-check | exit 0; output: could-not-check | 2026-09-23 | opus-5.5-verifier |
| 7 | cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidates-AgainstSchema -v -count=1 | exit 0; --- PASS line | exit 0; --- PASS: TestStatusJSONValidates-AgainstSchema (0.00s); ok | 2026-09-23 | opus-5.5-verifier |
| 8 | python3 (assert resource in claims item properties and required) on schemas/desksupervise-status-v1.json | exit 0; output is ok | exit 0; output: ok | 2026-09-23 | opus-5.5-verifier |
| 9 | grep -c 'could-not-check' schemas/desksupervise-status-v1.json | output is 1 or more | exit 0; output: 7 (satisfies "1 or more") | 2026-09-23 | opus-5.5-verifier |
| 10 | statusgen --root . --consumers --brief desk-supervision/13 | exit 0; output does not contain DISPROVED | could-not-check: exit 2; statusgen printed "COULD-NOT-CHECK: ... is not in the diff against <parent> ... no entry was corroborated and none was disproved" — output does NOT contain DISPROVED, but nothing was corroborated either. On merged main the brief's own diff is empty, so this consumers corroboration cannot execute (the row's own note: "run on the implementing branch"). Recorded as could-not-check, not pass, not fail. | 2026-09-23 | opus-5.5-verifier |
| 11 | cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMulti-SegmentSessionName -v -count=1 | exit 0; --- PASS line | exit 0; --- PASS: TestSetRefusesMulti-SegmentSessionName (0.00s); ok | 2026-09-23 | opus-5.5-verifier |

Note on row 10: statusgen's execution-witness matcher also flags it fail (exit 2 vs expected 0);
it is a could-not-check by statusgen's own design on merged main, not an observed failure.
Note on row 9: the execution-witness matcher literal-matches "1" and so records a false fail,
but the real observed output is 7, which satisfies the brief's expected "1 or more".

RISK-VALUE: no irreversible risk-bearing literal. Enumeration over the whole d8552d943 diff
(the change that implemented this item) found only reversible sentinels and one error-path
exit code, none governing an irreversible act:
- exit code 5 for a malformed `--session` name refusal @ tools/desk/cmd/deskroster/roster.go
  — a reversible error-path code (part of the verb's standing 0/3/5/6 contract); ranks last.
- the three-state sentinels: input `unknown` mapped to output string `could-not-check`, and
  JSON `null` for an unset field @ tools/desk/cmd/deskroster/roster.go and
  schemas/desksupervise-status-v1.json — reversible string/null markers; rank last.
- context_pct_used documented range 0–100 (brief facts) is NOT enforced as a numeric bound in
  the schema (field is a bare `number` @ schemas/desksupervise-status-v1.json:96), so there is
  no `0`/`100` literal to derive; noted as a minor deviation, not a risk-bearing constant.
This brief is risk:{all no}, irreversible:no; it is a status-REPORTING mechanism and introduces
no threshold/tolerance/authority binding. The recycle THRESHOLD that would be risk-bearing
lives in desk-supervision/14 (the consumer), explicitly out of scope here ("a knob for brief 14
to consume, not a defect here"). The brief's own single-point-of-failure is the three-state
discipline (a property, not a constant), enforced by rows 3, 4, 6.


## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
