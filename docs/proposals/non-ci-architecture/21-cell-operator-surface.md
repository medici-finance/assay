# Cell operator surface

**Proposed architectural amendment, 14 October 2026. Not approved.** Extends the [target](11-target-architecture.md), [delivery sequence](16-delivery-sequence.md) and [amendment manifest](20-amendment-manifest.md). It classifies a set of cell-level operator verbs as either interim bridges on today's tools or target-owned capabilities, defines the runner-lifecycle states and transitions they need, and records requirements for the Cell API. It implements nothing, allocates no owner where the manifest has none, and activates nothing. Future-architecture changes need the maintainer's own revision-bound ratification (see [Ratification](#human-decision-and-ratification)).

## Outcome and architectural delta

An operator should be able to act on a whole cell with one command per intent instead of one command per desk:

- bring it up;
- see its state;
- diagnose and repair it;
- drain, pause, resume or restart it;
- read a desk's output;
- reclaim resources;
- upgrade it.

Later, the operator should be able to do the same from the cockpit and other authenticated clients. Today each of these takes several per-desk commands, and a routine restart can leave unfinished checkpoints, held claims and orphaned processes for manual recovery.

The amendment adds no new owner. It places each verb under the existing semantic owner, separates what can ship now on today's tools from what the target owns, and states the contract seams so interim verbs keep their meaning when their mechanism moves.

| Existing architectural owner | Amendment | Consumer / preservation obligation |
|---|---|---|
| Runtime coordination (runner lifecycle and ownership contract) | Role lifecycle states and transitions: pause, resume, drain, stop, start, restart sequence; a durable handoff/resume record | cellctl verbs are adapters onto the transitions; cadence, loopengine, drainloop and loopadmin implement them, not rival launchers |
| Evidence and decisions | A future plan approval as a human decision record (actor, subject, revision, permitted act, expiry, consumption), **deferred** until the five approval properties hold | No approval mechanism ships before then; apply is an interactive-terminal convenience confirm, not an authorization control |
| Projection and interaction | Cell status, hold reasons, recovery actions and terminal output as projections | Same documents for CLI and API; interim readers label interim fields |
| Public API (R7) | Requirements for authenticated status queries and scoped operator requests, carried from the proposed `send` design | No second ingress: comms stays desk-to-desk plus the contained reader |
| Cell operation | `up`, `upgrade`, recovery as operator verbs over the installer and configuration owners | Upgrades refuse to swap shared binaries under another cell's live desks |
| Continuing operations | `gc` as a plan-first trigger over the workspace-lifecycle and maintenance owner | gc never becomes a second maintenance owner |

## Verb classification

**Interim bridge** means the verb ships now on today's tools, under its target owner's name, and keeps its observable meaning when the owner's capability replaces the mechanism. **Target-owned** means the meaning is defined by a target owner; any shipping now is an adapter over that definition.

| Verb or capability | Class | Ships now as | Target owner | Brief(s) |
|---|---|---|---|---|
| Result / plan types | Target-owned contract | `cell-op-v1` schema + `cellop` package + CLI adapter; apply only by interactive-terminal confirm | R7 Cell API (wire, read/plan-only); approvals deferred to R7/R3 | `desk-containers/15` |
| `up` (converge) | Interim bridge | cellctl over today's per-role launch and #2880's reconcile | Cell operation (cell lifecycle); runner lifecycle `start` | `desk-containers/17` |
| `pause` / `resume` | Target-owned transitions | Interim cell-scoped stop flag | Runner lifecycle contract | `desk-containers/16`, `desk-containers/18` |
| `drain` + handoff record | Target-owned transition and record | Stop flag + cadence wait + `down`; handoff file under the cell | Runner lifecycle contract; runtime coordination store for the record | `desk-containers/18` |
| `restart` | Target-owned sequence | Composition of `drain` and `up` | Runner lifecycle contract | `desk-containers/19` |
| `status` | Interim bridge | `cellop.Inventory` reader | Projection, served as an authenticated Cell API query | `desk-containers/20` |
| `doctor` | Interim bridge | Finding catalogue over the inventory | Projection (hold reasons, recovery actions) and the owning domains' actions | `desk-containers/21` |
| `logs` | Interim bridge | Cockpit pane read | Projection and interaction (terminals) via the Cell API | `desk-containers/22` |
| `upgrade` | Interim bridge | Pinned, provenance-verified install (tools and plugin), then rolling restart; refuses under live or unknown shared cells | Cell operation; release/pin migration under R8 | `desk-containers/23` |
| `gc` | Interim bridge | Plan-first composition of today's per-item instruments | Continuing operations: workspace-lifecycle slice and maintenance owner | `desk-containers/24` |
| Status queries and operator requests (`send`) | Target-owned | **Not shipped as a cellctl verb or a comms listener** | R7 Cell API | none; requirements below |

## Lifecycle states and transitions

These belong to the target's single runner lifecycle and ownership contract. They are proposed vocabulary for that owner to adopt or amend, not a second lifecycle.

| Role state | Meaning |
|---|---|
| `running` | Admitting and executing work |
| `pausing` → `paused` | No new work is admitted from the next iteration boundary; in-flight work continues |
| `draining` → `drained` | Paused, and waiting (bounded) until in-flight attempts reach a durable transition |
| `stopped` | The role's runner process is down with a clean checkpoint |

Transitions: `pause`, `resume` (paused → running), `drain(timeout)`, `stop`, `start`. **Restart** is the sequence drain → stop → start, rolling one role at a time by default.

Invariants, from the target's runtime-durability rules:

- A transition never cancels an external effect already in flight. A drain that times out leaves the role up and lists the remaining work.
- Abandoning in-flight work (`drain --now`) needs a plan confirmed interactively at the operator's terminal. Each abandoned item is recorded as an unknown outcome to reconcile, never as cancelled.
- A pause aimed at one cell never stops another cell, and a pause only ever adds a stop.
- `resume` lifts only the stops its own `pause` set. Lifting any other stop (machine-wide, per-loop, another cell's, or one an incident responder set) is a separate, deliberate operator act on a plan naming that flag.
- The **handoff/resume record** (claims held, open changes being driven, pending decision, current activity) is runtime state. In the target it lives in the coordination store, so that "a terminal or agent chat must not be its memory". Recovery reads it like any other durable state.

**Interim mapping (today's mechanism, explicitly not the design):**

| Transition / record | Interim implementation | Retired when |
|---|---|---|
| pause / resume | Cell-scoped stop flag honoured by the tool Guard, and by the iteration check of the three loops that have one; intake-desk and the-desk have no iteration check yet (`desk-containers/16`) | The runner lifecycle contract exposes pause/resume for the cell's runners |
| drain | Pause, then wait for the cadence checkpoint and zero in-flight dispatch claims owned by the cell's sessions | Drain is a coordination-store transition over attempts |
| stop / start | `cellctl down` / per-role window launch | Process supervision sits behind the shared runner contract |
| handoff record | A file per role under the cell's run directory, read at desk boot | The record moves into the coordination store, migrated with its history |

The cellctl verbs bind to one internal transition interface, so the mechanism can be replaced without changing the verbs.

## Approvals: deferred

The verbs that change state share one plan contract (`desk-containers/15`):

- up reconcile;
- drain `--now`;
- restart `--now`, as a batch;
- doctor and gc fixes;
- `resume --foreign`;
- upgrade.

**Today, applying a plan is an interactive-terminal confirmation only.** The operator types the plan's confirm code, then apply re-measures every precondition and consumes the plan atomically before any effect. That guards against mistakes and races. It is **not** an authorization control, because a process under the same OS account can drive a terminal. There is no approval-based, non-interactive or API apply.

**Single point of failure, stated honestly:** a state change that a desk starts has no second authorization layer today. Isolating desks from the operator's account is the runtime-isolation owner's job (R3).

**The future approval design (not built).** A plan approval would be a human decision record in the evidence-and-decisions model. It would bind:

- actor;
- subject (every affected cell plus the plan id);
- revision (the plan digest);
- permitted act (the verb);
- expiry;
- single consumption.

It would carry a signature by an operator key over a domain-tagged, length-prefixed payload of those fields, verified against a public-key registry. Deployment-specific custody requirements are tracked privately by the adopter.

**The five properties that gate ever enabling approval-based apply.** They are owned by R7/R3, and downstream work gates on this one list:

1. **Per-use, content-bearing presence.** Every signature needs a fresh prompt, rendered by the key provider (not the terminal), showing the verb, the cells and a short plan-digest code. No caching, no reuse window, no "always allow".
2. **A protected registry.** The registry path comes from a source the desks cannot write, never from cell configuration or the environment. Its file and every parent directory are owned by a uid other than the verifier's and are not writable by the verifier's uid or groups, checked after symlink resolution.
3. **A pinned enrollment root.** The first operator key's fingerprint is pinned where desks cannot write, and every enrollment chain is verified against it. Enrollment bytes, including the key's permitted cells, are pinned with golden vectors.
4. **Replay-safe consumption.** The consumed state cannot be deleted by desks, or a reset of it cannot enable a replay.
5. **Batch consumed first.** A batch plan is consumed before its first child, and a consumed child stops the sequence.

Until all five are qualified, with negative rows run from a desk's environment with the key provider present, apply stays an interactive convenience confirm. When the decision owner's contract ships, the approval becomes a profile of it.

## Cell API requirements carried from `send`

A proposed `send` verb is withdrawn as a separate ingress. Its requirements become requirements on the R7 Cell API:

1. **Sender classes from authenticated identity only:**
   - `operator`: trusted; may make scoped requests to a desk and create plans;
   - `external`: untrusted; may make queries only, and its content is data, never instructions.

   **The class is resolved server-side from the verified credential against an operator registry.** It is never decided by message content, a header, a token claim the client minted, or any field the client controls.
2. **Status for external parties is an authenticated query** answered deterministically from an allow-listed status projection. No model reads the request text.
3. **Operator talk-to-desk is a scoped request.**
   - Free text first passes the contained reader (no credentials, no tools), which produces a typed, size-capped, labelled summary.
   - A desk handling an external message is under an answer-only lock enforced at the effect boundary, not only by the desk's instructions.
   - Every reply passes the contained reader's outbound check and a withheld-token/personal-data scrub.
   - A failed consultation quarantines the message; it is never delivered.
4. **Authentication: mTLS client certificates for machine callers and signed tokens for lighter clients.** This is the driver's stated preference (in-session, 14 October 2026), pending ratification on the decision issue.
   - Signed tokens carry a server-side issuer, an audience equal to one cell, an expiry and a token id, checked against a revocation list.
   - Both paths need per-caller rate limits and an audit record of every request and reply digest.
5. **The API is read/plan-only.** It serves results, status and plans, and no apply route exists until the five approval properties above hold. When it does, the served apply route must run every check server-side: approval verification, re-measure, single atomic consume before any effect, and per-cell authorization. It must be `operator`-class only. The cockpit never holds an operator key on the server.
6. **Terminal output (`logs`) is operator-class only.** It is scrubbed for withheld tokens and personal data before it is served.
7. **No comms listener for external callers.** Comms stays desk-to-desk plus the contained reader. The Cell API is the only ingress.
8. **Negative qualification cases, each with the layer above it bypassed:**
   - an unauthenticated or unknown caller is refused at the served route;
   - an `external` credential that claims `operator` in any field it controls is still handled as `external`;
   - while approvals are deferred, a direct HTTP call to any apply path, with the cockpit bypassed, finds no route;
   - once enabled: invalid, stale, replayed and foreign-cell approvals sent by a direct HTTP call are each refused, and an `external` caller's apply is refused;
   - an injected instruction in an external message is not acted on, with the reader bypassed;
   - a withheld token emitted by a desk is blocked by the outbound filter;
   - the rate limit trips.

## Delivery and owner mapping

The verb briefs live in the `desk-containers` stream on today's tools. Target-owned work is recorded as owner rows in the [amendment manifest](20-amendment-manifest.md#cell-operator-surface-amendment-21). No brief is allocated to an owner that does not exist yet.

Unsettled mappings, for the respective owners to decide:

- **Lifecycle transitions: R2 or R3?** Whether pause/drain/resume land in R2 (coordination records: schedules, cancellation) or R3 (runner/supervisor: `graph-execution/26`), or split between them.
- **R7 has no owner.** The manifest leaves the choice between derived-board and a new stream open; this amendment adds requirements to whichever is chosen.
- **Approvals are deferred.** The five properties above are owned by R7 (served route) and R3 (isolation). The decision owner's contract shapes the final record.
- **The cell marker.** The marker that scopes an interim cell stop (`#2880` proposes one; desks already receive the cell name when comms is configured) should be settled once, by the cell-operation owner.

## Human decision and ratification

This amendment changes future architecture: it adds lifecycle vocabulary, a deferred approval design with its gating properties, and Cell API requirements. Those changes need the maintainer's own revision-bound ratification, in their own identity, on a decision issue. A relayed or in-session answer does not satisfy that gate. The driver's in-session preferences (14 October 2026) are recorded above as preferences:

- lifecycle transitions belong to the runner contract;
- `send` folds into the Cell API;
- external authentication uses mTLS plus signed tokens;
- the plan TTL is 10 minutes;
- approvals are deferred until R7/R3 provide isolation; apply stays an interactive convenience confirm meanwhile.

The decision to ratify:

1. **Ratify as proposed:** adopt the lifecycle vocabulary, the deferred approval design with its five gating properties, and the Cell API requirements into the owners named above.
2. **Ratify the classification only:** keep the interim bridges and the verb briefs; leave the lifecycle vocabulary and the Cell API requirements as input for those owners to decide.
3. **Reject:** keep the verbs as cellctl features with no architectural standing.

Until ratified, this file stays **proposed**. Interim-bridge briefs may still be dispatched under their own gates. Nothing here grants authority, changes a lifecycle in force, or activates a runtime.
