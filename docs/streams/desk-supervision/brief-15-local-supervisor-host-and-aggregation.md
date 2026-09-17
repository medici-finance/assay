---
brief: assay:assay:desk-supervision:15
title: Local supervisor host + multi-cell vitals aggregation
why: >-
  Everything briefs 01-09 and 13-14 build supervises k8s-hosted desks, because that is where a deskd
  runs. The operator's own laptop cell (`--kind house`) has no deskd, so its desks — the ones a
  person actually watches — get no liveness reclaim and no budget recycle at all. This brief
  stands a LOCAL deskd on that cell so the same supervision and recycle apply to non-k8s desks,
  and gives that deskd a way to aggregate its cell's vitals — and, across a multi-cell fleet,
  roll them up into one ops view of who is alive, who is full, who is stopped. It closes the gap
  where the desks closest to the operator were the least supervised, and gives a fleet a single
  answer to "how is everyone doing" that is separate from, and complementary to, the work board.
wave: 4
depends: ["desk-supervision/13", "desk-supervision/14"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief stands a PERSISTENT local daemon on the operator's own machine that runs the
  supervision + recycle controls (briefs 01-09, 13-14) over that machine's live desks. The four risk
  answers are no, but a human should confirm, before it exists, the credential and blast-radius
  surface: a house-cell deskd runs under the operator's own ambient config home (roster, keys
  by symlink) rather than an isolated per-cell App identity, and it inherits authority to stop
  and recycle the operator's local desks. What the human confirms is that this host, under those
  credentials, over that blast radius, is intended — the same class of sign-off brief 04 (a new
  execution surface) and brief 14 (an autonomous stop of healthy work) took.
issues: [351]
schema: brief-v2
authored: 2026-09-17 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §13.3 (runtime snapshot) and §13.7 (optional HTTP server / monitoring endpoint) — the shape a fleet ops view takes — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/cellctl/cellctl — `--kind house` is 'a LOCAL cell on the operator's own laptop … no deskd unless DESKD=1' (lines ~18-23); `DESKD=1` already 'require and stand a deskd as a k8s cell does' (line ~151); `cellctl deskd <cell>` stands the persistent deskd (line ~37); `check_house` already reports the deskd binary/health as n/a when DESKD=0 (lines ~856-860). This brief makes the DESKD=1 house path first-class and supervised."
  - "desk-supervision/07 — `desksupervise status --json` and its status.json written atomically each tick; the per-cell reading a roll-up consumes."
  - "desk-supervision/13 — the `resource` block the aggregation rolls up per cell."
  - "desk-supervision/14 — the recycle the local deskd applies to house-cell desks."
  - "tools/desk/cmd/opmetrics — the house's aggregates-only, three-state, could-not-check-never-zero collector; the fleet roll-up follows the same discipline and stays SEPARATE from the statusgen work board."
  - "freshness-checked 2026-09-17 @ daaa4b9c — no aggregate verb or schema exists; `ls schemas/desksupervise-aggregate*` is empty; cellctl's house-cell deskd path is present but check-only."
exec-tier: strong
exec-tier-why: >-
  (b): correctness spans cellctl (shell), the aggregation schema, and a reference emitter that
  must agree on one contract a private fleet view consumes; and the host runs supervision +
  recycle under the operator's credentials, where a scoping slip is not caught by a happy-path test.
decision-trigger: creation
consumers:
  - "tools/cellctl/cellctl (the `--kind house` + DESKD=1 path and `check_house`): fixed-here (standing + proving a local deskd on a house cell becomes a supported, checked path, not a check-only n/a)"
  - "tools/cellctl/tests/house-cell.test.sh: fixed-here (a DESKD=1 case proves the house deskd is required, stood, and supervises the cell's desks)"
  - "schemas/desksupervise-aggregate-v1.json (new): fixed-here (the aggregation CONTRACT — per-cell vitals roll-up + fleet ops view — as a published schema a console/fleet view consumes)"
  - "tools/desk/cmd/desksupervise/aggregate.go (new): fixed-here (a reference emitter that reads N cells' status.json/beacons and emits the aggregate document, so the contract is executable and testable offline)"
  - "the deskd host that serves the aggregate over HTTP + respawns recycled house-cell desks (deskd, the console): out-of-scope (a private consumer; the public contract is cellctl's house path + the aggregate schema + the reference emitter)"
version: 1
id: b394892e-6e4b-48aa-94f0-1167f195cd86
---

# Brief 15 — Local supervisor host + multi-cell vitals aggregation

> Two-planes framing: see the top of `desk-supervision/13`. This brief extends BOTH planes to a
> new class of host — the operator's local, non-k8s cell — and adds the roll-up that reads the
> worker-operations plane (vitals) across many cells.

## Context

files:
- `tools/cellctl/cellctl` — make the `--kind house` + `DESKD=1` path first-class: stand the
  local deskd (the existing `cellctl deskd` path applied to a house cell) and have
  `check_house` PROVE (not report n/a) that the deskd is up and supervising the cell's desks
  when `DESKD=1`.
- `tools/cellctl/tests/house-cell.test.sh` — a `DESKD=1` case.
- `schemas/desksupervise-aggregate-v1.json` (new) — the aggregation contract.
- `tools/desk/cmd/desksupervise/aggregate.go` (new) — the reference emitter + `aggregate` verb.
- `tools/desk/cmd/desksupervise/aggregate_test.go` (new).
- `tools/desk/cmd/desksupervise/testdata/cells/` — multi-cell fixtures.
- `docs/desk-tools/` — a short page: the local deskd host (console callout) + the aggregate.

single-point-of-failure: the host itself is the single point for a house cell — one local deskd
supervises that cell's desks, and if it is down the cell is unsupervised. The layer behind it is
`check_house` under `DESKD=1`, which fails LOUDLY (the cell reads unsupervised, not silently
clean) when the deskd is absent or unhealthy — an independent signal (a precondition check) from
a different component (cellctl, not deskd) that catches exactly the "supervisor silently not
running" fault. The aggregation itself carries no single point beyond the three-state discipline
it inherits: an unreadable cell renders `could-not-check`, never dropped and never zero.

facts:
- **cellctl already knows the house/deskd shape.** `--kind house` is the operator's laptop cell,
  reusing the operator's config home by symlink, with **no deskd unless `DESKD=1`**. `DESKD=1`
  already means "require and stand a deskd as a k8s cell does"; `cellctl deskd <cell>` stands the
  persistent deskd; `check_house` today reports the deskd binary/health as **n/a** when
  `DESKD=0`. This brief turns the `DESKD=1` house path from present-but-check-only into a
  supported, proven path.
- **deskd is the console (private) — never named by repo.** The `bin/deskd` binary is built and
  owned in the console's repo; cellctl (public) stands it, checks it, and puts it on PATH. The
  host is a **console callout**: the public tree names `deskd`, `cellctl`, and the aggregate
  schema, and stops there.
- **The aggregation CONTRACT is public; the serving is private.** deskd aggregates its cell's
  vitals and, in a multi-cell fleet, rolls up to a fleet ops view served over its existing local
  endpoint (the `/healthz` neighbour). This brief ships that contract as a schema PLUS a
  reference emitter (`desksupervise aggregate`) so it is executable and testable offline; deskd
  wraps the same logic to serve it live.
- **The aggregate document** (`desksupervise-aggregate-v1`): a list of cells, each carrying its
  `desksupervise-status-v1` snapshot (or `could-not-check` when the cell is unreachable), plus a
  fleet roll-up — counts by liveness class, by recycle state, armed stops, and blind cells —
  across every cell. Three-state throughout: an unreadable cell is `could-not-check`, never
  dropped, never zero.
- **Separate from the work board.** This is an OPS view (who is alive / full / stopped), not the
  statusgen work board (what is queued / in review / done). The two never merge; a desk being
  supervised says nothing about brief status and vice-versa. The roll-up follows opmetrics'
  aggregates-only discipline.
- **`sensitive-data: no` is about regulated/customer data, not about "nothing worth protecting"
  (security review finding S-3).** The aggregate concentrates per-session operational telemetry —
  session identities, claim keys, models, token counts, liveness class, armed stops — across a
  whole fleet, which is a real information-concentration risk even though none of it is
  regulated or customer data (the four risk questions above answer correctly for what they ask).
  The serving side is out of scope for this brief, but the reference emitter's document carries at
  least the same access control as the `/healthz` endpoint it is served beside — never a wider
  audience than the health check that already exposes the cell's identity.
- **A house cell can name `example-cell-a`, `example-cell-b`** in fixtures — placeholders, not
  real cell names (house config: the real cell/roster values live in the operator's cell.env and
  never in this public tree).
- Verb contract unchanged: kill switch first, one audit line per invocation, exit 0 · 3 · 5 · 6,
  fail closed; `aggregate` is read-only.

## Human decision
<!-- gate: human — lifted verbatim into the decision issue; self-contained, no links/paths. -->
We want to run a small persistent background service on the operator's own machine so that the
desks running there — the ones a person watches directly, which today have no automatic
supervision at all — get the same liveness reclaim and budget-driven recycle that the
cloud-hosted desks already get. On the operator's machine this service runs under the operator's
own configuration and credentials (rather than a separate, isolated identity): concretely, it
inherits ambient access to the operator's roster file, the operator's forge (GitHub/GitLab) keys
by symlink, and whatever else lives in that configuration home — it does not get its own scoped
App identity the way a cloud-hosted desk does. What it is allowed to DO with that inherited access
is bounded to exactly three writes: release a claim, set a per-run stop flag, and append an audit
journal entry — the same read-mostly write set brief 01's observer already has. It gains no forge
write authority beyond that set: it never opens, comments on, or merges a PR, and never pushes a
branch, even though the credentials it runs under could technically do so. It is allowed to stop
and restart the operator's local desk sessions on that basis. Separately, this service can gather
a "how is everyone doing" view — which sessions are alive, which are running low on headroom,
which are stopped — across one or many machines, kept apart from the existing work-status board.

Options:
1. **Approve standing a local supervisor on the operator machine, under the operator's own
   config home, with authority to recycle that machine's desks, plus the cross-machine ops
   view.** Recommended. It is off by default and only turned on per machine by an explicit
   setting; when off, nothing changes. It closes the gap where the least-isolated desks were the
   least supervised.
2. **Approve the aggregation / ops view only, no local recycle authority yet.** The local service
   would report vitals and liveness but not stop or restart any local desk; retiring a full local
   worker stays manual.
3. **Hold.** Keep local operator desks unsupervised for now; supervise only cloud-hosted desks.

Default if no answer: none — blocks until answered (this authorises a persistent service under
the operator's own credentials; it does not proceed on a timeout).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **cellctl house-deskd path.** Make `--kind house` + `DESKD=1` stand and require a local
   deskd: `up` opens the deskd; `check_house` under `DESKD=1` PROVES the deskd binary exists, is
   up on its address, and lists the cell's supervised desks (fail LOUD, not n/a). `DESKD=0`
   behaviour is unchanged (n/a, no deskd).
2. **Aggregate schema** (`desksupervise-aggregate-v1.json`): a `cells` array (each `{cell, status:
   <status-v1 doc | "could-not-check">}`) and a `fleet` roll-up (`by_liveness`, `by_recycle`,
   `armed_stops`, `blind_cells`). `additionalProperties: false`; three-state throughout.
3. **Reference emitter** (`aggregate.go`): `desksupervise aggregate [--cells-fixture DIR]
   [--json]` reads N cells' status.json (or beacons) and emits the document; an unreadable cell
   renders `could-not-check` and appears in `blind_cells`, never dropped. Table + JSON; JSON
   validates against the schema.
4. **Tests**: add a `DESKD=1` case INSIDE `tools/cellctl/tests/house-cell.test.sh`, alongside the existing
   `DESKD=0` case (which asserts the opposite — n/a, no deskd), printing an identifiable PASS
   line for the new case (e.g. `PASS: DESKD=1 house cell requires + supervises a local deskd`);
   the DESKD=1 case asserts `check_house`'s OBSERVED OUTPUT proves the deskd (up + supervising),
   not the DESKD=0 `n/a — not required` line. Aggregate over
   3 cells where one is unreadable ⇒ that cell is `could-not-check` and in `blind_cells`, the
   other two roll up; the roll-up counts match the per-cell snapshots; JSON validates.
5. **Docs page**: the local deskd host (console callout) + the aggregate contract; state the
   ops-view / work-board separation.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/cellctl && bash tests/house-cell.test.sh` | exit 0; output contains a `PASS` line for the new `DESKD=1` case (e.g. `PASS: DESKD=1 house cell requires + supervises a local deskd`) — the whole suite, incl. the existing `DESKD=0` case, stays green |
| 2 | check +flow | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise aggregate --json --cells-fixture cmd/desksupervise/testdata/cells \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["schema"], len(d["cells"]))'` | exit 0; output is `desksupervise-aggregate-v1 3` |
| 3 | check +dereference | `cd tools/desk && ./desksupervise aggregate --json --cells-fixture cmd/desksupervise/testdata/cells \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["fleet"]["blind_cells"])'` | exit 0; output contains `example-cell-c` (the unreadable cell is named blind, not dropped) |
| 4 | check +dereference | `cd tools/desk && ./desksupervise aggregate --json --cells-fixture cmd/desksupervise/testdata/cells \| python3 -c 'import json,sys; d=json.load(sys.stdin); c=[x for x in d["cells"] if x["cell"]=="example-cell-c"][0]; print(c["status"])'` | exit 0; output is `could-not-check` |
| 5 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run 'Aggregate' -count=1` | exit 0; output contains `ok` |
| 6 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestAggregateJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestAggregateJSONValidatesAgainstSchema` |
| 7 | check | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestFleetRollupMatchesPerCellSnapshots -v -count=1` | exit 0; output contains `--- PASS: TestFleetRollupMatchesPerCellSnapshots` |
| 8 | check | `test -f schemas/desksupervise-aggregate-v1.json && python3 -c 'import json; s=json.load(open("schemas/desksupervise-aggregate-v1.json")); assert s["properties"]["fleet"]["properties"]["blind_cells"]; print("ok")'` | exit 0; output is `ok` |
| 9 | check | `cd tools/cellctl && bash tests/house-cell.test.sh 2>&1 \| grep -cF 'deskd up + supervising'` | output is `1` or more — a RUNTIME line from the `DESKD=1` case's `check_house` proves the deskd is up + supervising (observed output, not the `DESKD=0` `n/a — not required` line, and not a grep of a source comment) |
| 10 | check | `statusgen --root . --consumers --brief desk-supervision/15` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "an unreachable cell is silently dropped and the fleet looks all-green"
→ rows 3, 4 (blind cell named, not dropped); "the ops view leaks into / is confused with the
work board" → row 7 + the docs separation; "a house cell reads 'clean' while its deskd is not
actually running" → rows 1, 9 (check_house proves, not n/a); "the aggregate JSON drifts from the
schema a fleet view reads" → rows 6, 8. Review-only (the human gate): whether a persistent local
deskd under the operator's own credentials, with recycle authority over local desks, is the
intended host and blast radius.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY — the decision issue must be answered before this lands.
