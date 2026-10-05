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

### Non-implementer verifier run — VERIFY: BLOCKED (rows 1-8 and 11 checked-clean; row 9 meets its stated expectation; row 10 could-not-check) — 2026-09-27 opus-5.5[1m]-verifier (verify-desk dispatch), merged main `9585b4b`
Runner ≠ implementer. Isolated detached worktree off origin/main at the merged head. Offline (KUBECONFIG=/dev/null, GOWORK=off); desk rows module-scoped from tools/desk. `gate: model`, all risk `no`, `irreversible: no`. Impl commit `d8552d9` (#1350, squash; parent `f4cb66a`). The witness table below was written by `statusgen verifyrun` (v1.0.27); observed output per row follows it.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | pass exit=0 | sha256:f9ce087c78e7 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | pass exit=0 | sha256:cf9af6045351 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | pass exit=0 | sha256:d708489def97 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | pass exit=0 | sha256:1b4630643f29 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | pass exit=0 | sha256:7614458c86b2 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | pass exit=0 | sha256:a3061c96a360 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | pass exit=0 | sha256:f3bf79d43c6d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | fail exit=0 | sha256:10159baf262b | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | fail exit=2 | sha256:d42bd9e6b4b2 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | pass exit=0 | sha256:7f856e7a9ebc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

Observed output per row (2026-09-27 opus-5.5[1m]-verifier):
- Row 1: exit 0 — "ok github.com/medici-finance/assay/tools/desk/internal/deskkit".
- Rows 2, 3, 4: exit 0 — "--- PASS: TestMergeResourceVitalsPreservesAcks", "--- PASS: TestUnsetVitalIsNullNotZero", "--- PASS: TestMeasuredZeroSubagentsRoundTrips".
- Row 5: exit 0 on both pipe stages — "60 0 example-model".
- Row 6: exit 0 — "could-not-check"; all five resource fields read could-not-check for the absent beacon (none null, none 0).
- Row 7: exit 0 — "--- PASS: TestStatusJSONValidatesAgainstSchema".
- Row 8: exit 0 — "ok".
- Row 9: exit 0 — real output "7", which meets the Expect "1 or more". The witness `fail` is a matcher artifact: verifyrun reads the Expect prose as the literal line "1" ("no output line equals 1"). Not a product defect.
- Row 10: could-not-check, exit 2 — "COULD-NOT-CHECK: … is not in the diff against 9585b4b…, so this run carries no evidence about its claims — no entry was corroborated and none was disproved". Re-run on the implementing commit d8552d9 with --base f4cb66a (its parent, also the PR head's merge-base with main): same COULD-NOT-CHECK, exit 2. The implementing diff never touched this brief, which is the post-merge `--consumers` class tracked at medici-finance/assay#1281. No DISPROVED anywhere. Manual comparison of the consumers block against the d8552d9 diff: the schema and cmd/desksupervise/status.go are in the diff as declared; the `deskroster set` vitals flags landed in cmd/deskroster/roster.go, not the declared cmd/deskroster/sets.go (untouched); the resource merge landed as the new sibling internal/deskkit/vitals.go, and the declared internal/deskkit/ackbeacon.go is untouched, which matches that entry's own "a sibling field-preserving merge" wording. So two `fixed-here` entries name the wrong file (brief-text drift, not a code defect).
- Row 11: exit 0 — "--- PASS: TestSetRefusesMultiSegmentSessionName".

Risk-bearing values (trigger FIRED on the path clause: risk metadata is all `no` and `irreversible: no`, but the diff adds tools/desk/internal/deskkit/vitals.go, which matches this repo's security-path trigger for tools/desk/internal/deskkit/ — `statusgen --lint` reports it as a risk-files-crossread NOTICE on this brief. Enumerated over the d8552d9 diff to the status schema, internal/deskkit/vitals.go and cmd/deskroster/roster.go):
- `RISK-VALUE: DERIVED — ValidSessionSegment rejects "" / "." / ".." / any "/" or "\" / any ".." substring @ tools/desk/internal/deskkit/vitals.go:94-100 — these are exactly the names that make a filepath.Join under the roster directory collapse onto or escape that directory (empty, self, parent, a separator on either OS); the blanket ".." substring reject is strictly more conservative. Row 11 exercises the refusal (exit 5). Reversible.`
- `RISK-VALUE: DERIVED — beacon file mode 0o600 / dir mode 0o700 @ tools/desk/internal/deskkit/vitals.go:147 and :140 — the brief's stated trust boundary is same-uid, same-host; owner-only read/write is that boundary expressed as a mode, and it matches deskroster's own beacon write (roster.go:161 / :149). Reversible.`
- `RISK-VALUE: DERIVED — sentinel "unknown" ⇒ could-not-check @ tools/desk/cmd/deskroster/roster.go:291, :307, :325 — the brief's Task 2 names this exact sentinel; a numeric flag that fails to parse is refused rather than coerced to could-not-check, so a typo cannot hide a real reading. Reversible.`
- Observation (no literal, so not a risk-value entry): context_pct_used is described as 0-100 (schema line 95, flag help roster.go:357) but neither the parser nor the schema enforces the range, and strconv.ParseFloat also accepts NaN/Inf. Outside this Verify table; a review note for desk-supervision/14, which reads this field as a recycle threshold.

**VERIFY: BLOCKED** — rows 1-8 and 11 checked-clean; row 9 meets its Expect (7 ≥ 1) and its witness fail is a matcher artifact; row 10 could-not-check (medici-finance/assay#1281), nothing disproved. The item stays `implemented` until the desk rules on row 10 or the brief is amended so rows 9 and 10 are machine-checkable after merge. Because the diff touches a security-path trigger while the brief answers every risk question `no`, whether a model may sign this item off (versus routing it to the human gate) is also the desk's call.

### Non-implementer verifier re-run: 2026-10-02T21:45:18Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

Runner is not the implementer. Isolated detached worktree at the merged head. Offline (KUBECONFIG=/dev/null, GOWORK=off). Every `go test` / `go build` row under tools/desk ran with a throwaway HOME and a scratch TMPDIR (the real Go build and module caches kept), so nothing was written to a live audit log or to the system temp directory. Rows 5 and 6 need a configured roster: under an empty throwaway HOME `desksupervise status` refuses (exit 6, "could-not-check: assay/desk-tools inactive") and the pipe exits 1; with a read-only copy of the roster placed in the throwaway HOME both rows run as authored. The results below are from that configuration.

Re-run reason: the prior record (2026-09-27, blocked, #1281) woke on a changed declared input. Of the thirteen declared input files, twelve are byte-identical to the recorded hashes (the brief, the status schema, the vitals and roster sources and tests, the status renderer and its test, all four fixtures); only the tools/desk README changed, and it is not exercised by any Verify row. Tool version moved v1.0.27 to v1.0.31.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | exit 0; output contains `ok` | exit 0 — "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 0.388s" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | exit 0; output contains `--- PASS: TestMergeResourceVitalsPreservesAcks` | exit 0 — "--- PASS: TestMergeResourceVitalsPreservesAcks (0.00s)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | exit 0; output contains `--- PASS: TestUnsetVitalIsNullNotZero` | exit 0 — "--- PASS: TestUnsetVitalIsNullNotZero (0.00s)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | exit 0; output contains `--- PASS: TestMeasuredZeroSubagentsRoundTrips` | exit 0 — "--- PASS: TestMeasuredZeroSubagentsRoundTrips (0.00s)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | exit 0; output is `60 0 example-model` | exit 0 (both pipe stages) — "60 0 example-model" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | exit 0; output is `could-not-check` | exit 0 (both pipe stages) — "could-not-check"; all five resource fields read could-not-check for the absent beacon (none null, none 0) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` | exit 0 — "--- PASS: TestStatusJSONValidatesAgainstSchema (0.00s)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | exit 0; output is `ok` | exit 0 — "ok" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | output is `1` or more | exit 0 — "7" (7 is 1 or more: meets the Expect) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | exit 0; output does not contain `DISPROVED` | exit 2 — "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:13 is not in the diff against cf31c32418ba…, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." Zero occurrences of DISPROVED. could-not-check, reported as itself: neither a pass nor an observed failure | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | exit 0; output contains `--- PASS: TestSetRefusesMultiSegmentSessionName` | exit 0 — "--- PASS: TestSetRefusesMultiSegmentSessionName (0.01s)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief <this brief> --dry-run`, v1.0.31, nothing written to the brief): 9 of 11 pass. Rows 1-8 and 11 pass (exit 0). Row 9: fail, exit 0, "no output line equals "1"". Row 10: fail, "exit 2, expected 0". The witness stamped the tree "+dirty" because row 5's own `go build` leaves the built desksupervise binary untracked in tools/desk; no tracked file was modified.

Findings — has the verdict changed since 2026-09-27? No.
- Row 10 (previously could-not-check): still could-not-check, same exit 2, same message. Nothing landed that changes it: #1281 is still OPEN (last updated 2026-09-30), the brief text is byte-identical to the 2026-09-27 input hash, and on merged main the brief is by construction not in the diff against the head it is checked out at. The row's own Expect says it is meant for the implementing branch; the prior pass already showed it is could-not-check there too (the implementing diff never touched the brief). No DISPROVED was observed anywhere. The prior pass's manual comparison still stands, since the same files are unchanged: two `fixed-here` consumers entries name files the implementation did not touch (the vitals flags landed in the deskroster roster source, not the declared sets source; the merge landed as a new sibling vitals source, not in the declared ackbeacon source).
- Row 9 (previously a witness fail): by hand it still meets its Expect (7 is 1 or more). The witness still fails it for the same reason — the matcher reads the Expect prose as the literal line "1". A check-definition artifact, not a product defect; it needs an Expect the matcher can evaluate.
- Rows 1-8 and 11: checked-clean, as before.
- New observation: rows 5 and 6 are environment-sensitive — `desksupervise status` with fixture flags still requires a configured roster and refuses without one, so on a host with no roster these two fixture-only rows exit 1. Not counted against the item (they pass as authored on a configured host), but it is a portability gap in the row definitions.

Risk-bearing values (trigger fires on the path clause: all risk metadata reads `no` and `irreversible: no`, but the implementing diff adds a file under the tools/desk deskkit path, a security-path trigger). Enumerated over the implementing diff to the status schema, the deskkit vitals source and the deskroster roster source; all three files are byte-identical to the 2026-09-27 inputs, and the literals were re-read at the cited lines in this pass:
- `RISK-VALUE: DERIVED — ValidSessionSegment rejects "" / "." / ".." / any "/" or "\" / any ".." substring @ tools/desk/internal/deskkit/vitals.go:93-100 — exactly the names that make a path join under the roster directory collapse onto or escape that directory (empty, self, parent, a separator on either OS); the blanket ".." substring reject is strictly more conservative. Row 11 exercises the refusal. Reversible.`
- `RISK-VALUE: DERIVED — beacon directory mode 0o700 / file mode 0o600 @ tools/desk/internal/deskkit/vitals.go:140 and :147 — the brief's stated trust boundary is same-uid, same-host; owner-only access is that boundary expressed as a mode, and it matches deskroster's own beacon write (roster.go:149 / :161). Reversible.`
- `RISK-VALUE: DERIVED — sentinel "unknown" ⇒ could-not-check @ tools/desk/cmd/deskroster/roster.go:291, :307, :325 — the brief's Task 2 names this exact sentinel; a numeric flag that fails to parse is refused rather than coerced, so a typo cannot hide a real reading. Reversible.`
- Carried observation (no literal, so not a risk-value entry): context_pct_used is described as 0-100 but neither the parser nor the schema enforces the range. Outside this Verify table; a review note for desk-supervision/14.

VERIFY: BLOCKED — rows 1-8 and 11 checked-clean; row 9 meets its Expect by hand (7 ≥ 1) but the witness cannot evaluate its prose Expect; row 10 is could-not-check on merged main (#1281, still open), nothing disproved. Blocker kind: check-definition. Unchanged from 2026-09-27: the item stays `implemented` until rows 9 and 10 are amended to be machine-checkable after merge or the desk rules on row 10; and because the diff touches a security-path trigger while the brief answers every risk question `no`, sign-off routing is the desk's call.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner is not the implementer: assay-verifier-app[bot] (claude-opus-5-5[1m], on-behalf-of human:ian), verify-desk dispatch, run 2026-10-04T09:12Z (UTC). It ran in an isolated detached worktree at merged main ade741f44372ccdc0986c2b0973f621745fbced7, offline (KUBECONFIG=/dev/null, GOWORK=off). Desk rows are module-scoped from tools/desk, and every `go test` uses a targeted `-run` with `-timeout 300s`. The hand-run rows used the session's own HOME. All `go test` rows set HOME to a t.TempDir, but rows 5 and 6 (`desksupervise status`) each appended one `result=ok` line to the operator's desk-tools audit log at 2026-10-04T09:10:49Z. That append is the verb's normal one-line-per-invocation behaviour, with no forge or cluster contact. The witness run below used a throwaway HOME holding a read-only copy of the roster, with the real Go caches and a scratch TMPDIR.

Why this re-run: the 2026-10-02 receipt (BLOCKED at cf31c3241, assay#1281) went stale because five declared inputs changed under tools/desk. Four commits touched them, and only one touches vitals behaviour: medici-finance/assay#2155 (704711ad3, "Serialize and atomically publish roster beacon updates").
- That PR moves `MergeResourceVitals` (internal/deskkit/vitals.go) off its own read/merge/WriteFile code and onto a new shared `MutateRosterBeacon` transaction (internal/deskkit/rosterbeacon.go). The transaction takes a cross-process lock file, reads the raw keys, and replaces the beacon atomically through a same-directory temp file.
- The same PR moves deskroster's role/work writes (cmd/deskroster/roster.go `mutateBeacon`) onto that transaction. They now copy back only `session`/`role`/`updated`/`open_work` and preserve every other key, `resource` included.
- `drop` now deletes a beacon only when every key on it belongs to deskroster. Before, a beacon holding only `resource` with no work, role or acks would have been deleted, losing its vitals.
- The beacon parse is now stricter. It rejects a non-object, duplicate keys and trailing data, and fails closed ("refusing to overwrite it or treat corruption as missing state").
- The session-path check is also stricter: `AckBeaconPath` → `validRosterSession` adds Windows-reserved characters, trailing dot or space, control characters and device names on top of `ValidSessionSegment`.
- The vitals_test.go change only adds `t.Setenv("USERPROFILE", home)`, a Windows-portability fix with no change to any assertion.
- The cmd/deskroster/sets.go change adds a `repos --scope roots` printer, unrelated to vitals. The README gains a "Roster beacon writes and recovery" section, and its `deskroster set` vitals-flags bullet is still present.

Net effect on the brief's claims: field preservation (Task 1, row 2), null-not-zero and measured-zero (rows 3 and 4), fail-closed on a malformed beacon, and the exit-5 refusal of a multi-segment session name (row 11) all still hold, and the refactored write path enforces them more strictly. The renderer (cmd/desksupervise), its fixtures and the status schema are byte-identical to cf31c3241. Nothing in the change weakens the three-state discipline. Supplementary targeted tests on the new transaction all pass at ade741f44 (13 in internal/deskkit and 17 in cmd/deskroster, all `--- PASS`, both packages `ok`). They cover concurrent-process field preservation, a killed writer leaving committed JSON and releasing the lock, malformed state preserved, lock timeout not running the callback, portable session names, `TestRosterMutationsPreserveForeignFields`, `TestSetVitalsPreservesRoleAndOpenWork` and `TestSetVitalsUnknownSentinelIsCouldNotCheck`.

Expectations were derived from the brief's Task and Verify text before the prior Evidence was read: rows 1-9 and 11 checked-clean, and row 10 could-not-check on merged main because its own Expect scopes it to the implementing branch.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | exit 0; output contains `ok` | exit 0 — "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 0.498s". The regex selects TestMergeResourceVitalsPreservesAcks and TestMergeResourceVitalsMalformedBeaconFailsClosed, both PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | exit 0; output contains `--- PASS: TestMergeResourceVitalsPreservesAcks` | exit 0 — "--- PASS: TestMergeResourceVitalsPreservesAcks (0.02s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | exit 0; output contains `--- PASS: TestUnsetVitalIsNullNotZero` | exit 0 — "--- PASS: TestUnsetVitalIsNullNotZero (0.02s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | exit 0; output contains `--- PASS: TestMeasuredZeroSubagentsRoundTrips` | exit 0 — "--- PASS: TestMeasuredZeroSubagentsRoundTrips (0.01s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | exit 0; output is `60 0 example-model` | exit 0 on every pipe stage — "60 0 example-model" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | exit 0; output is `could-not-check` | exit 0 on both pipe stages — "could-not-check" (absent beacon reads as blind, not 0) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` | exit 0 — "--- PASS: TestStatusJSONValidatesAgainstSchema (0.00s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | exit 0; output is `ok` | exit 0 — "ok" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | output is `1` or more | exit 0 — "7" (7 is 1 or more: meets the Expect by hand). The witness marks it fail because its matcher reads the Expect prose as the literal line "1" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | exit 0; output does not contain `DISPROVED` | exit 2 — "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:13 is not in the diff against ade741f44372…, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." Zero occurrences of DISPROVED. Reported as could-not-check: neither a pass nor an observed failure (statusgen v1.0.31) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | exit 0; output contains `--- PASS: TestSetRefusesMultiSegmentSessionName` | exit 0 — "--- PASS: TestSetRefusesMultiSegmentSessionName (0.00s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness, written by `statusgen verifyrun --brief <this brief> --timeout 300s` (v1.0.31) into this worktree's copy of the brief. The run exited 1 and nothing was committed.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | pass exit=0 | sha256:8bc17d67503f | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | pass exit=0 | sha256:c5efc91b91a4 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | pass exit=0 | sha256:2a5e9849c742 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | pass exit=0 | sha256:2b8e1c176281 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | pass exit=0 | sha256:793cac7f9506 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | pass exit=0 | sha256:5daf05682473 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | pass exit=0 | sha256:27c64f5ebc2c | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | fail exit=0 | sha256:10159baf262b | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | fail exit=2 | sha256:a8677ebe7662 | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | pass exit=0 | sha256:ac9cc4ba028d | 2026-10-04 | assay-verifier-app[bot] @ ade741f44372 (on-behalf-of human:ian) (forge-identity) |

`statusgen verifyrun --check --brief <this brief>` (v1.0.31), exit 1, summary line: "docs/streams/desk-supervision/brief-13-worker-operations-vitals.md: 9 pass, 2 fail, 0 could-not-run/missing (of 11 Verify rows)". Rows 1-8 and 11: "witness matches the row and passed". Rows 9 and 10: "the witness records a failure".

Risk-bearing values. The trigger fires on the path clause: all risk metadata reads `no` and `irreversible: no`, but the implementing diff adds a file under the tools/desk deskkit security path. I enumerated over the implementing diff plus the #2155 refactor of the vitals write path (internal/deskkit/vitals.go, internal/deskkit/rosterbeacon.go, cmd/deskroster/roster.go). The prior pass cited the beacon modes in vitals.go:140/:147, but those literals moved into rosterbeacon.go, so they are re-located below.
- `RISK-VALUE: DERIVED — ValidSessionSegment rejects "" / "." / ".." / any "/" or "\" / any ".." substring @ tools/desk/internal/deskkit/vitals.go:87-95 — these are exactly the names that make a path join under the roster directory collapse onto or escape it (empty, self, parent, a separator on either OS). The blanket ".." substring reject is stricter still. It is called at cmd/deskroster/roster.go:459 (exit 5, row 11) and again inside the path builder. Reversible.`
- `RISK-VALUE: DERIVED — validRosterSession additionally rejects the characters <>:"|?*, a trailing "." or " ", any rune < 32, and the device names CON/PRN/AUX/NUL/CONIN$/CONOUT$/COM1-9/LPT1-9 @ tools/desk/internal/deskkit/rosterbeacon.go:30-52 — these are the Windows filename rules under which a name could alias another file, an alternate data stream or a device. It is a strict superset of the brief's S-1 single-segment rule, and TestBeaconStorePortableSessionNames passes. Reversible.`
- `RISK-VALUE: DERIVED — roster dir mode 0o700 @ tools/desk/internal/deskkit/rosterbeacon.go:135; lock file mode 0o600 @ :138; beacon file 0600 via os.CreateTemp + atomic replace @ :195 (CreateTemp's documented mode; the replace also narrows an old over-broad mode) — the brief's stated trust boundary is same uid, same host, and owner-only access is that boundary expressed as a mode. Reversible.`
- `RISK-VALUE: DERIVED — sentinel "unknown" ⇒ could-not-check @ tools/desk/cmd/deskroster/roster.go:325, :341, :359 — Task 2 names this exact sentinel. A numeric flag that fails to parse is refused rather than coerced (TestSetVitalsRejectsUnparseableNumber passes), so a typo cannot hide a real reading. Reversible.`
- Ranked last, no derivation needed (reversible operational knobs): rosterBeaconLockWait = 10 * time.Second @ tools/desk/internal/deskkit/rosterbeacon.go:26; lock poll 10 * time.Millisecond @ :155.
- Carried observation (no literal, so not a risk-value entry): context_pct_used is described as 0-100, but neither the parser nor the schema enforces the range. This is outside the Verify table and is a review note for desk-supervision/14.
- Minor, not counted: the cmdSet comment at cmd/deskroster/roster.go:486-489 still says "see vitals.go for why a second, dedicated writer is the safer property", but #2155 removed that rationale from the vitals.go header. The comment is stale and the code is unaffected.

Has the verdict changed since 2026-10-02? No. The real code change (#2155) leaves every vitals property this brief verifies intact or stricter. Row 10 is unchanged: it is could-not-check on merged main by construction, and the prior pass showed it is could-not-check on the implementing commit d8552d9 too. Row 9 is unchanged: it meets its Expect by hand, but the witness cannot evaluate the prose Expect.

VERIFY: BLOCKED — rows 1-8 and 11 checked-clean at ade741f44. Row 9 meets its Expect by hand (7 ≥ 1), but the witness records a matcher-artifact fail. Row 10 is could-not-check (exit 2), with nothing disproved. Blocker: medici-finance/assay#1281 (OPEN, last updated 2026-09-30T02:27:52Z), kind check-definition. The item stays `implemented` until rows 9 and 10 are amended to be machine-checkable after merge or the desk rules on row 10. Because the diff touches a security-path trigger while the brief answers every risk question `no`, sign-off routing stays the desk's call.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner is not the implementer: assay-verifier-app[bot] (claude-opus-5-5[1m], on-behalf-of human:ian), verify-desk dispatch, run 2026-10-04T10:40Z (UTC). Brief frontmatter: `gate: model`; `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`. Every row ran in a fresh throwaway clone detached at merged main afe5fe94c0e16ec8753c1b7095681ffdcfcbd0d4, with the clone's origin/main pinned to that same sha. It ran offline (KUBECONFIG=/dev/null, GOWORK=off) under a throwaway HOME that held only a read-only copy of the roster, with a scratch TMPDIR and the real Go build and module caches. Nothing was appended to the operator's audit log. Desk rows are module-scoped from tools/desk, and every `go test` used a targeted `-run` with `-timeout 300s`. No row is classed check:ci, so the Linux container recipe was not needed: the witness was written on the darwin host and no docker command was run. The clone's own local git config carries the verifier identity (assay-verifier-app[bot], the noreply address keyed to its user id). Without it the witness attributed every row to the driver, which would be a misattribution for a model run. That clone is a standalone repo, not a linked worktree.

Why this re-run: the 2026-10-04T09:17Z receipt (BLOCKED at ade741f44, 10/11 rows by hand, assay#1281) went stale because four declared inputs changed on main between ade741f44 and afe5fe94c. The brief hash is unchanged as landed (41471f1e…), and so are the schema, both vitals sources and tests, the four fixtures and the deskroster sets source.
- tools/desk/README.md (the expected wake) changed in medici-finance/assay#2162 (09212ca13). The change is a new "Beacon file boundary and retention" section, which says the supervisor's resource join now uses the strict beacon reader and that malformed or duplicate-key state yields `could-not-check` vitals. The same diff range also carries #2135's deskpr usage edits. The `deskroster set` vitals-flags bullet (README line 1460) is still present. No Verify row runs or cites the README.
- tools/desk/cmd/desksupervise/status.go also changed in #2162, and this change does touch what the brief verifies. `liveResourceSource` dropped its own `ValidSessionSegment` check plus `os.ReadFile`, and now calls `deskkit.ReadRosterBeacon`. That call validates the holder through `AckBeaconPath` → `validRosterSession`, a strict superset of the S-1 single-segment rule. It also opens with O_NOFOLLOW|O_NONBLOCK, accepts only a regular file, and rejects duplicate keys, trailing data and non-objects. Every failure, including a missing or null `resource`, still returns `blindResource()` (all five fields `could-not-check`). Task 4's "absent or unreadable beacon is blind, never null, never 0" therefore still holds and is now stricter. Rows 5 and 6 and the supplementary tests below confirm it.
- tools/desk/cmd/desksupervise/status_test.go changed in #2162. It adds `TestLiveResourceSourceStrictBeaconBoundary` and a `USERPROFILE` setenv. The schema-validation test behind row 7 is untouched.
- tools/desk/cmd/deskroster/roster.go changed in #2162, but only in `cmdList`. An unreadable beacon or a failed prune is now collected and reported (exit 6, "ownership unverified"). Before, the read error was skipped silently and the prune error aborted the command. The `set` vitals flags, the "unknown" sentinel parse and the exit-5 session refusal are untouched.
- One undeclared supporting file changed too: internal/deskkit/rosterbeacon.go plus its new unix and windows variants (`openRosterBeaconFile`). This is the read and lock path that the vitals writer and the renderer both go through.

Net: nothing in the change weakens the three-state discipline or S-1. The read side of S-1 moved from an inline guard in status.go into the shared reader and became stricter.

Expectations were derived from the brief's Task and Verify text before the prior Evidence was read. Rows 1-9 and 11 should be checked-clean. Row 10 should be could-not-check on merged main, because its own Expect scopes it to the implementing branch.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | exit 0; output contains `ok` | exit 0 — "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 0.460s". The regex selects TestMergeResourceVitalsPreservesAcks and TestMergeResourceVitalsMalformedBeaconFailsClosed, and both PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | exit 0; output contains `--- PASS: TestMergeResourceVitalsPreservesAcks` | exit 0 — "--- PASS: TestMergeResourceVitalsPreservesAcks (0.02s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | exit 0; output contains `--- PASS: TestUnsetVitalIsNullNotZero` | exit 0 — "--- PASS: TestUnsetVitalIsNullNotZero (0.01s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | exit 0; output contains `--- PASS: TestMeasuredZeroSubagentsRoundTrips` | exit 0 — "--- PASS: TestMeasuredZeroSubagentsRoundTrips (0.01s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | exit 0; output is `60 0 example-model` | exit 0 (pipefail, every stage) — "60 0 example-model" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | exit 0; output is `could-not-check` | exit 0 (pipefail, both stages) — "could-not-check". Printing the whole resource block shows all five fields as could-not-check for the absent beacon (none null, none 0) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` | exit 0 — "--- PASS: TestStatusJSONValidatesAgainstSchema (0.00s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | exit 0; output is `ok` | exit 0 — "ok" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | output is `1` or more | exit 0 — "7". Seven is 1 or more, so the row meets its Expect when checked by hand. The witness marks it fail because its matcher reads the Expect prose as the literal line "1" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | exit 0; output does not contain `DISPROVED` | exit 2 — "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:13 is not in the diff against afe5fe94c0e16ec8753c1b7095681ffdcfcbd0d4, so this run carries no evidence about its claims — no entry was corroborated and none was disproved." DISPROVED appears zero times. This is could-not-check (the vacuous class: merged main has no diff for this check), so it is neither a pass nor an observed failure (statusgen v1.0.31) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | exit 0; output contains `--- PASS: TestSetRefusesMultiSegmentSessionName` | exit 0 — "--- PASS: TestSetRefusesMultiSegmentSessionName (0.00s)" | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

Supplementary checks on the changed read path, not Verify rows. All were targeted runs at afe5fe94c with exit 0:
- In cmd/desksupervise, `-run TestLiveResourceSource`: TestLiveResourceSourceRejectsHolderTraversal and TestLiveResourceSourceStrictBeaconBoundary both `--- PASS`.
- In internal/deskkit, `-run 'Beacon|RosterBeacon'`: "ok" (11.8s).
- In cmd/deskroster, `-run 'Vitals|Beacon|Roster'`: "ok".

`statusgen verifyrun --check --brief <this brief>` (v1.0.31), exit 1, summary line: "docs/streams/desk-supervision/brief-13-worker-operations-vitals.md: 9 pass, 2 fail, 0 could-not-run/missing (of 11 Verify rows)". Rows 1-8 and 11 read "witness matches the row and passed". Rows 9 and 10 read "the witness records a failure".

Execution witness, written by `statusgen verifyrun --brief <this brief> --timeout 300s` (v1.0.31, darwin host) into the throwaway clone's copy of the brief. The run exited 1 and nothing was committed. The table is copied byte-for-byte from the tool's output:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Vitals' -count=1` | pass exit=0 | sha256:0210c70660d2 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMergeResourceVitalsPreservesAcks -v -count=1` | pass exit=0 | sha256:2ab0affc7fa2 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestUnsetVitalIsNullNotZero -v -count=1` | pass exit=0 | sha256:e12b39d81ee1 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestMeasuredZeroSubagentsRoundTrips -v -count=1` | pass exit=0 | sha256:890c28e3c156 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/vitals-beacons.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["claims"][0]["resource"]; print(r["context_pct_used"], r["subagents_spawned"], r["model"])'` | pass exit=0 | sha256:793cac7f9506 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 6 | `cd tools/desk && ./desksupervise status --json --now 2026-09-17T12:00:00Z --claims-fixture cmd/desksupervise/testdata/vitals.json --observations-fixture cmd/desksupervise/testdata/vitals-obs.json --beacons-fixture cmd/desksupervise/testdata/no-beacon.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["claims"][0]["resource"]["tokens"])'` | pass exit=0 | sha256:5daf05682473 | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 7 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | pass exit=0 | sha256:fd6836b2b2fe | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 8 | `python3 -c 'import json; s=json.load(open("schemas/desksupervise-status-v1.json")); item=s["properties"]["claims"]["items"]; assert "resource" in item["properties"], "no resource block"; assert "resource" in item["required"], "resource not required"; print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 9 | `grep -c 'could-not-check' schemas/desksupervise-status-v1.json` | fail exit=0 | sha256:10159baf262b | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 10 | `statusgen --root . --consumers --brief desk-supervision/13` | fail exit=2 | sha256:8d4b586c53fa | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |
| 11 | `cd tools/desk && GOWORK=off go test ./cmd/deskroster/ -run TestSetRefusesMultiSegmentSessionName -v -count=1` | pass exit=0 | sha256:bd6d421c258b | 2026-10-04 | assay-verifier-app[bot] @ afe5fe94c0e1 (on-behalf-of human:ian) (git-config) |

Risk-bearing values. The trigger fires on the path clause: all risk metadata reads `no` and `irreversible: no`, but the implementing diff adds a file under the tools/desk deskkit security path, and #2162 changed the beacon read and lock path under it. I enumerated over the implementing diff plus the vitals read and write path as it stands at afe5fe94c: internal/deskkit/vitals.go, rosterbeacon.go and rosterbeacon_unix.go, cmd/deskroster/roster.go, and cmd/desksupervise/status.go. Every literal was re-read at the cited line in this pass.
- `RISK-VALUE: DERIVED — ValidSessionSegment rejects "" / "." / ".." / any "/" or "\" / any ".." substring @ tools/desk/internal/deskkit/vitals.go:87-95 — these are exactly the names that make a path join under the roster directory collapse onto or escape it (empty, self, parent, a separator on either OS). The blanket ".." substring reject is stricter still. It is called at cmd/deskroster/roster.go:460 (exit 5, row 11). Reversible.`
- `RISK-VALUE: DERIVED — validRosterSession additionally rejects the characters <>:"|?*, a trailing "." or " ", any rune < 32, and the device names CON/PRN/AUX/NUL/CONIN$/CONOUT$/COM1-9/LPT1-9 @ tools/desk/internal/deskkit/rosterbeacon.go:30-52 — these are the Windows filename rules under which a name could alias another file, an alternate data stream or a device. It is a strict superset of S-1. Since #2162 it is also the renderer's read-side holder check, through ReadRosterBeacon → AckBeaconPath (ackbeacon.go:43), and TestLiveResourceSourceStrictBeaconBoundary exercises it on COM1, "holder.", "holder " and "holder:stream". Reversible.`
- `RISK-VALUE: DERIVED — roster dir mode 0o700 @ tools/desk/internal/deskkit/rosterbeacon.go:140; lock file mode 0600 with O_NOFOLLOW|O_NONBLOCK|O_CLOEXEC @ tools/desk/internal/deskkit/rosterbeacon_unix.go:15-18; beacon file 0600 via os.CreateTemp + atomic replace @ rosterbeacon.go:200 — the brief's stated trust boundary is same uid, same host. Owner-only access is that boundary expressed as a mode. Refusing to follow a leaf symlink and refusing a non-regular file keep a same-host substitution from redirecting a control-bearing write or read, and the #2162 README section states plainly that this does not defend against parent-directory replacement or hard links. Reversible.`
- `RISK-VALUE: DERIVED — sentinel "unknown" ⇒ could-not-check @ tools/desk/cmd/deskroster/roster.go:326, :342, :360 — Task 2 names this exact sentinel. A numeric flag that fails to parse is refused rather than coerced, so a typo cannot hide a real reading. Reversible.`
- Ranked last because they are reversible operational knobs, so no derivation is needed: rosterBeaconLockWait = 10 * time.Second @ tools/desk/internal/deskkit/rosterbeacon.go:26; lock poll 10 * time.Millisecond @ :160.
- Carried observation (no literal, so not a risk-value entry): context_pct_used is described as 0-100, but neither the parser nor the schema enforces the range. This is outside the Verify table and is a review note for desk-supervision/14.

Has the verdict changed since the 09:17Z receipt? No. #2162 hardens the beacon boundary that the vitals read and write share, and every vitals property this brief verifies still holds. Row 10 is unchanged: it is could-not-check on merged main by construction, and an earlier pass showed it is could-not-check on the implementing commit d8552d9 too. Row 9 is unchanged: it meets its Expect by hand, but the witness cannot evaluate the prose Expect.

VERIFY: BLOCKED. Rows 1-8 and 11 are checked-clean at afe5fe94c. Row 9 meets its Expect by hand (7 ≥ 1), but the witness records a matcher-artifact fail. Row 10 is could-not-check (exit 2), with nothing disproved. Blocker: medici-finance/assay#1281 (OPEN, last updated 2026-09-30T02:27:52Z), kind check-definition. The item stays `implemented` until rows 9 and 10 are amended to be machine-checkable after merge or the desk rules on row 10. Because the diff touches a security-path trigger while the brief answers every risk question `no`, sign-off routing stays the desk's call.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
