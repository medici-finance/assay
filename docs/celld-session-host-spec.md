# Celld: cell sessions, harness runtimes, and execution hosts

**Status:** draft
**Date:** 2026-10-13
**Decision owner:** human driver; implementation owner: cellctl maintainers
**Scope:** design only. No daemon activation, credential migration, or rollout is authorized by this document.

## 1. Recommendation and outcome

Make `cellctl` the operator client for **owned sessions on explicit execution hosts**,
with terminals as attachable views. Use `celld` as the proposed service name for the
execution-host side of the existing future architecture: the packaging of runtime
coordination (R2) and isolated runner and cell operation (R3) for one cell, with
harness adapters behind the shared runner contract. It hosts model-working sessions
and never holds role effect credentials: the R4 role executors stay separate
processes behind their own credential boundary (section 10). It is a
packaging and interface proposal, not a new scheduler, ingress or authority owner.
Its relation to the other owners is stated in the next subsection. Use each
harness's supported runtime to execute agents; preserve existing role identity,
workspace, admission, lifecycle and evidence contracts. Qualify an opt-in Codex
app-server adapter alongside the already planned process adapter; keep legacy
launch paths until their replacements satisfy the session contract.

A successful integration lets an operator detach a terminal, reconnect from another
device, see exactly which work still runs where, and drain or stop that work without
affecting another cell. It must refuse to create a replacement when the old owner is
uncertain. The benefit is lifecycle correctness and recoverability; reduced CPU,
memory or startup time is a hypothesis to measure, not a reason to assume sharing is safe.

This changes cellctl's unit of management from a launched client process to a role's
owned runtime session. It does not turn cellctl into an agent engine, transcript
database, vendor cloud scheduler, or transparent cross-host failover system.

### Relation to the architecture on main

This revision is reconciled against the architecture on main (`origin/main`
`1948dc087`), in particular [21: cell operator surface](proposals/non-ci-architecture/21-cell-operator-surface.md)
and [23: runtime contract alignment](proposals/non-ci-architecture/23-runtime-contract-alignment.md).

- **Cell API (R7).** Doc 21 allocates R7, the public Cell API and thin cockpit, to the
  active `cell-api` stream and makes the Cell API the only operator ingress. `celld` is
  **not** that API and adds no ingress of its own. Operator clients, including `cellctl`,
  reach `celld` only through the Cell API; its authenticated control endpoint serves
  the Cell API and runner registration. Whether `celld`
  is packaged inside or behind the Cell API is open decision D4 for the R7 owner. This
  spec does not pre-empt it.
- **Lifecycle (R2/R3).** Doc 21 owns the runner lifecycle vocabulary: pause, resume,
  drain(timeout), stop, start and restart. R2 records a requested transition and R3
  executes it. A record is a request, never authority. Section 4 maps every operation in
  this spec onto those transitions and defines no rival one.
- **Interim verbs.** The operator verbs `up`, `down`, `status`, `logs` and the per-role
  stop ship as interim bridges under desk-containers/15-24. A `celld`-backed mode is an
  adapter behind the same verbs and keeps their observable meaning.
- **Runtime contract (doc 23).** PostgreSQL coordination, role executors, the typed forge
  service and the thin-client rule apply unchanged. Section 10 maps every owner.

## 2. Evidence and inherited contracts

Repository observations below are pinned to
[`bcc02252e52ffe596d7b16f06df28e9e11e3806b`](https://github.com/medici-finance/assay/tree/bcc02252e52ffe596d7b16f06df28e9e11e3806b).
Vendor documentation was read on 2026-10-13; availability must be qualified against
the deployed version and account, not inferred from these pages.

| Current source | Observed behavior and implication |
| --- | --- |
| [launch.go](../tools/desk/cmd/cellctl/launch.go), [plan.go](../tools/desk/cmd/cellctl/plan.go) | Codex launch argv does not explicitly select daemon attachment or `--no-daemon`. Client defaults can therefore affect the runtime boundary. |
| [codex_environment.go](../tools/desk/cmd/cellctl/codex_environment.go), [env.go](../tools/desk/cmd/cellctl/env.go) | Host Codex retains operator configuration/authentication while command subprocesses receive cell context. Scrubbed cells have different home handling. Command environment isolation alone does not prove isolation of server credentials, MCP processes or plugins. |
| [cellcadence](../tools/desk/internal/cellcadence/cadence.go), [interactive.go](../tools/desk/internal/cellcadence/interactive.go) | Interactive and scheduled roles share a durable checkpoint and OS ownership lock. The caller must prove child cleanup before marking completion. Preserve that single-owner boundary. |
| [down.go](../tools/desk/cmd/cellctl/down.go) | Host shutdown stops comms and role supervisors before closing cockpit surfaces. It has no recorded Codex thread binding or native Goal operation. This is an integration gap, not proof of a particular orphan incident. |
| [Host desk cadence](cellctl-cadence.md) | Native Goals are the interactive Codex default; explicit headless cadence disables them. The launcher currently does **not** own or mutate native Goals. Paused or cleared Goals must remain so. |
| [Role context](cellctl-role-context.md) | Operator custody and per-role policy remain requirements. Existing harness-specific checks do not establish daemon equivalence. |
| [Contained reader](../tools/desk/internal/cellreader/reader.go) | A tools-stripped Codex app-server client already exists, with isolated configuration and rejection of server callbacks. It is not a general desk runtime client. |
| [Model policy](cellctl-model-policy.md), [cursor.go](../tools/desk/cmd/cellctl/cursor.go) | Policy can route roles to Claude or Codex today. Cursor supports an explicit host launch but refuses `CELL_MODEL_POLICY` and non-house cells. Mixed Cursor/Codex daemon deployment is not established by existing mixed-model configuration. |
| [loopadmin runner](../loopadmin/runner/adapter.go) | `Capabilities`, `Start`, `Observe`, `Cancel`, and `Reconcile` already form a checked adapter contract. Only a fake adapter and offline conformance kit ship here; a real provider adapter and durable supervisor remain planned. |

Local protocol inspection used CLI **0.162.1**, including
`codex app-server generate-json-schema --experimental`. Its schema exposes thread
start/resume configuration, Goal operations, descendant queries and background
terminal enumeration. Schema presence does not prove account access, isolation or
successful execution. The CLI and running daemon can have different versions; qualify both.

Recent changes at this source pin include native Goal defaults (#2697), role-lock
recovery (#2714), configuration custody (#2433), and Goal recovery guidance (#2783).
This is evidence of coupling and active change in the launch/lifecycle seam, not a
measured complexity score or proof that a rewrite is needed. The bounded improvement
here is to replace client-process completion assumptions with owned-session evidence,
while retaining the existing ownership and recovery machinery.

## 3. Harness capabilities and cross-host meaning

Codex exposes thread and turn operations, events, Goals and experimental background
terminal controls through its app-server protocol. Disconnecting a subscriber is
distinct from interrupting a turn. Its documented remote connections include
remote-host execution through SSH. These are useful adapter surfaces, subject to
version qualification. [App-server protocol](https://learn.chatgpt.com/docs/app-server),
[remote connections](https://learn.chatgpt.com/docs/remote-connections).

Claude Code has a Remote Control server, including multiple sessions and worktree
creation. Execution remains on the serving machine, which must keep running;
web/mobile clients connect through Anthropic. This is not evidence of a public
Codex-compatible management API. The Agent SDK separately supplies a programmable
Claude Code runtime in a process the integrator operates. Use documented SDK/CLI
interfaces for a cellctl adapter; do not automate private Remote Control endpoints.
[Remote Control](https://code.claude.com/docs/en/remote-control),
[Agent SDK](https://code.claude.com/docs/en/agent-sdk/overview).

Cursor exposes durable agents and individual runs through its Cloud Agents API.
A cancelled run is terminal; continuing creates another run on that agent. Cursor
also supports self-hosted workers on personal machines and team pools, connected
outbound to Cursor's service. Pool availability and credential requirements differ
from personal workers. Thus Cursor can execute on another host, but its service,
worker and run boundaries differ from a local Codex daemon.
[Cloud Agents API](https://cursor.com/docs/cloud-agent/api/endpoints),
[self-hosted machines](https://cursor.com/docs/cloud-agent/self-hosted).

| Backend candidate | Session/execution owner | Initial cellctl disposition |
| --- | --- | --- |
| Codex owned app server | Server owns threads and turns; cell owns the server binding | First daemon adapter to qualify, local host first; existing Claude process-adapter plan remains |
| Claude CLI / Agent SDK | Owned process hosts a resumable conversation | Retain process adapter; qualify the SDK control mapping below (S4 Claude lane) before expanding |
| Claude Remote Control | Serving host executes; vendor connection supplies remote UI | Optional operator attachment, not the admission authority |
| Cursor local CLI | Existing cellctl process/cadence integration | Retain current behavior |
| Cursor managed or self-hosted agents | Vendor owns agent/run lifecycle; selected worker executes | Separate adapter and trust review; do not pretend it is a local PID |

Do not normalize all these into a fictitious common `pause` operation or native
standing Goal. Each adapter advertises verified capabilities: observe, attach,
submit, interrupt, enumerate descendants, list/stop background work, preserve
continuation state, and confirm quiescence. Unsupported means unavailable, not
success. A standing desk requires a qualified continuation policy; a persistent
server alone supplies neither a timer nor a permanent-service guarantee.

### Claude Agent SDK control mapping

The Agent SDK's documented interfaces cover most of that capability list from inside a
process cellctl owns. No vendor patch or private endpoint is needed. Names below are
from the TypeScript SDK, and the Python SDK lacks some of the hooks. Every row is a
candidate for the S4 Claude lane to qualify at a pinned version. None of them has been
verified yet. [Hooks](https://code.claude.com/docs/en/agent-sdk/hooks),
[sessions](https://code.claude.com/docs/en/agent-sdk/sessions),
[permissions](https://code.claude.com/docs/en/agent-sdk/permissions),
[TypeScript reference](https://code.claude.com/docs/en/agent-sdk/typescript).

| celld capability | Agent SDK surface | Qualification note |
| --- | --- | --- |
| Bind session | `query()` with a caller-chosen `sessionId`, or the `session_id` of the init system message | The SDK process owns the conversation. If the process exits, the in-flight turn ends with it. |
| Submit / steer | streaming input (`prompt` as an `AsyncIterable`) and `streamInput()` | Streaming mode only. |
| Observe / attach | the message stream, plus lifecycle events from `includeHookEvents` | Clients attach through celld, never to the SDK process directly. |
| Interrupt | `Query.interrupt()` | Streaming mode only. The result message confirms the interrupt; the request alone does not. |
| Stop background work | `Query.stopTask(taskId)` | Documented. Whether every owned task can be enumerated is unproven. |
| Admission gate (drain) | `PreToolUse` hook that consults celld admission and returns `permissionDecision: "deny"` or `"defer"` | Hooks run before the permission rules, and a hook deny holds even under `bypassPermissions`. `defer` ends the turn with `stop_reason: "tool_deferred"` so the call can be resumed later. |
| Continuation policy | `Stop` hook that returns `decision: "block"` with a `reason` | Replaces a native standing Goal. celld owns the policy, so freezing it changes no vendor state. A timed-out Stop callback lets the agent stop. |
| Waiting for approval | `canUseTool` callback or `PermissionRequest` hook; `Notification` of type `permission_prompt` | celld reports the wait and never approves it automatically. |
| Descendants / quiescence | `SubagentStart`/`SubagentStop` and `SessionStart` (with `source`)/`SessionEnd` hooks | The `SessionStart`/`SessionEnd` callbacks are TypeScript-only. |
| Session rebind | `resume: <id>` (or `forkSession`); a `sessionStore` adapter for another host | Restores the conversation only. Files, running commands and pending approvals are not restored. This is the vendor's conversation resume, a session-level operation, and is not the lifecycle transition `resume` of doc 21. |

This keeps Claude an owned-process adapter, and the drain gate becomes mechanical
rather than prompt-only. The existing [model-policy](cellctl-model-policy.md) hooks
are the in-repo precedent: settings-injected hooks that call back into the launcher.
The admission hook must follow the same rules: fail closed when its binary is
missing, and never hang. Timeout handling differs by hook event and by version, and
the model-policy record notes that Claude Code can let an action through when a hook
times out. S4 therefore records the observed timeout behaviour and must include two
refusal cases: the admission hook binary removed, and the hook stalled past its
deadline. In both, a tool call made while admission is closed must still be refused. A
gate that passes a call on timeout is not mechanical.

The limits stay explicit:

- A controller crash loses the in-flight turn and its background commands.
- There is no remote placement other than running the SDK process on the target host.
- Remote Control remains an optional operator attachment. If input can arrive through
  it, that input passes the same admission hook while drained, or the path is disabled
  during drain. S4 verifies which one holds at the pinned version.

Three different cross-host operations must remain explicit:

1. **Remote control:** a client on host A observes or steers execution on host B.
   B retains its filesystem, tools, credentials and responsibility for stopping work.
2. **Remote placement:** an authorized new task is assigned to B before it starts.
   Repository access and environment provisioning happen there.
3. **Migration:** an existing task moves from B to C. This is out of the first release.
   Conversation rebind is insufficient: worktree changes, commands, approvals,
   claims, credentials and execution ownership also require a handoff.

No observed feature establishes transparent live migration across all three vendors.
On loss of contact with B, cellctl reports **unknown**, preserves its claim and does
not start the same task on C. Future failover requires proof of fencing the old
executor, not merely a missed heartbeat or an expired controller connection.

## 4. Ownership and adapter boundary

| Responsibility | Owner |
| --- | --- |
| Model execution, conversation history, native tools and native child agents | Harness runtime |
| Cell/role identity, worktree custody, model policy and dispatch authorization | Existing cellctl/desk policy |
| Exclusive role ownership, desired admission state, recovery binding | Existing authority during transition; the approved runtime-coordination owner in the target architecture |
| Runtime-specific requests, events and status interpretation | Shared `loopadmin` runner contract and qualified adapters; cellctl supplies resolved profiles |
| Terminal, browser or cockpit attachment | Presentation client; attachment confers no new authority |
| Placement on another host | Explicit operator-selected execution target; no implicit scheduler |

Separate three configuration choices: **harness**, **execution target** and
**continuation policy**. A remote Codex daemon can still use interactive native
continuation; headless cadence remains a separate mode with native Goals disabled.
Changing the view must not change any of these choices.

This spec defines no lifecycle vocabulary of its own. Operator intents map onto doc 21,
and a harness-specific step is an implementation of a doc-21 transition, never a rival:

| This spec's operation | Doc 21 meaning | Owner and constraint |
| --- | --- | --- |
| `inspect` | `status` projection | Read-only projection through the Cell API and `cellctl status` |
| `attach` / `detach` | Not a lifecycle transition | Presentation view only; confers no authority and changes no state |
| `drain` (section 5) | `drain(timeout)`: paused, then a bounded wait for in-flight attempts to reach a durable transition | R2 records, R3 executes. A drain that times out leaves the role up and lists the remaining work |
| `stop` (section 5) | `stop`, legal for a role already paused or drained | R2 records, R3 executes after re-measuring idleness. Interrupting in-flight work is the abandon path described there |
| session rebind (conversation resume) | None. Re-binding to a proven vendor session after a restart | Never lifts a stop or pause. Only doc 21 `resume` (paused to running) changes admission |
| `pause` / `resume` | `pause` and `resume` (paused to running) | `resume` lifts only the stops its own `pause` set |

These are behavioral requirements, not claims that new CLI verbs already exist.
Existing `up` should attach to a proven owned session or create one only when no
unfinished owner exists. Existing `down` must report stop evidence before declaring
success. A future `detach` closes only the view. Preserve current command compatibility
through an explicit runtime mode; do not silently change running cells.

For a bounded legacy-path qualification, extend the existing
`<cell>/run/cadence/<role>/checkpoint.json` rather than inventing another local
registry. The target service stores runtime facts through the planned PostgreSQL
coordination/journal owners; this local checkpoint is transitional, not a competing
distributed authority. A versioned runtime binding needs:
backend, stable host identity, endpoint reference, server incarnation, cell/role,
ownership generation, root session ID, known child IDs, worktree identity,
configuration digest, last observation and desired admission state. Keep credentials
out of the record. Vendor IDs and host identities are authoritative only when bound
by the trusted controller; titles, cwd matches and model-written metadata are not.

The OS lock remains host-local. Remote operator clients submit lifecycle operations through the Cell API, which
reaches the execution host's single owner over an authenticated transport; copying the
checkpoint or placing a file lock on shared storage does not create distributed
ownership. A later multi-controller service needs an explicit fencing design.

Reuse cellcadence ownership semantics, cellprocess behavior and existing cockpit
adapters through the planned shared-runner migration. Retire terminal liveness as
completion evidence for daemon mode. Do not add a second job database or copy
transcripts into cellctl. Status may cache bounded
observations, with their age; the runtime remains the source for execution state.

The contained reader's RPC implementation is private and deliberately restrictive.
The target adapter lives across a Go module boundary in `loopadmin`; it cannot import
`tools/desk/internal/cellreader` or `cellprocess`. The existing plan moves the process
runner into `loopadmin/process` and expands the launch audit. A reusable protocol
codec would likewise need an explicit shared package and dependency, with tests
preserving the reader's callback refusal, binary custody and feature restrictions.
Otherwise use a small independent wire client behind the shared runner interface.
Sharing transport code must not share tool authority or the comms socket.

## 5. Codex integration contract

### Server scope and launch

Default sharing boundary: one **cell and compatible credential/policy domain** on
one host. Share between roles only after proving their per-thread environment,
model, plugins, MCP and approval policies stay distinct. If a setting is process-wide,
split servers by the incompatible domain. A separate home or daemon is operational
separation, not a security sandbox against processes running as the same OS user.

Do not adopt or stop the operator's general-purpose daemon implicitly. Select an
owned endpoint and explicit runtime mode. Process mode must explicitly disable
daemon attachment on qualified Codex versions; daemon mode must not silently fall
back to another endpoint or process after a connection failure.

Cell-specific configuration must account for CODEX_HOME's authentication, packages,
plugins and session state. Do not copy a personal auth directory wholesale. Preserve
the existing host authentication contract until a documented credential-provisioning
mechanism has been qualified and approved. Endpoint permissions and remote transport
must bind to that same domain; use authenticated local transport or an authenticated
remote channel, never a newly exposed unauthenticated listener.

Under the existing role lock, launch is a transaction:

1. Read stop/drain state and unfinished ownership; resolve trusted host, worktree,
   model and configuration. Refuse unknown or incompatible state.
2. Connect to the intended server, initialize the protocol and check required
   methods plus both client/server versions. Readback must match the intended home
   and server identity. Presence of a schema field alone is not qualification.
3. Persist a launch intent and generation before creating a session. Record the
   returned session ID before starting its first turn.
4. Verify effective role context and actual model using non-secret probes; test MCP
   and subprocess context separately. Preserve current shell and login restrictions.
5. Start the authorized role only after its ordinary claim/model/admission gates.
   Native Goal bootstrap retains its exact-objective and active-readback requirement.

A lost create response is ambiguous. Do not replay creation or a user prompt
blindly. Reconcile through a proven binding, or leave an unfinished record requiring
recovery. A client request ID is not assumed to be a vendor idempotency guarantee.

### Drain, stop and recovery

Admission state and runtime state are separate. `running`, `waiting-for-approval`,
`idle`, `disconnected` and `unknown` do not themselves establish completion.

**Drain** first closes new admissions mechanically at the existing claim/dispatch
boundary. Capture the already accepted work and its authorized completion obligations,
including fixes and re-review of those items. Finishing an item does not refill its
slot. Send steering to the existing session and retain comms until handoff completes.
Do not interrupt active work merely to satisfy drain. Approval waits remain visible
and are never automatically approved.

Drain is bounded: it takes a timeout, as `drain(timeout)` in doc 21 does. Report drained
only when captured obligations are completed or explicitly handed
off, no new admissions occurred, and no owned turn or background work can continue
admitting work. A drain that reaches its timeout reports the remaining sessions, turns
and commands, leaves the role up and paused, and is neither drained nor stopped. Freeze continuation at that boundary without marking a standing
objective complete. Prompt-only compliance is insufficient for the admission gate.
On the Claude lane the gate is the `PreToolUse` admission hook (section 3), which
refuses or defers each tool call while admission is closed. Continuation freezes
because the celld-owned `Stop` hook stops blocking.

**Stop** is a distinct, explicitly requested transition, legal only for a role already
paused or drained. Close admissions, inhibit automatic continuation, re-measure that the
role is idle, and then close the owned session. R2 records the request and R3
executes it. A recorded request is never authority: while approvals are deferred,
only stop and no-new-work transitions execute from a record.

Interrupting an owned turn or descendant, or terminating an owned background command, is
**abandoning in-flight work**. It maps to doc 21's abandon path (`drain --now`) and needs
a plan confirmed interactively at the operator's terminal. Each abandoned item is
recorded as an unknown outcome to reconcile, never as cancelled. No transition cancels
an external effect already in flight. Re-read runtime state and retain receipts before
closing comms/cockpit surfaces. Never translate unsubscribe, client exit or loss of
connection into successful stop. Do not use unrestricted server shell commands as
a teardown shortcut. Shutdown of an entire server requires exclusive proven
ownership and no unrelated clients; stopping a cell must never kill a global daemon.

This proposes new Goal control authority and conflicts with the current cadence
contract that the launcher does not mutate Goals. **Decision D1 below is required.**
If approved, explicit drain/stop may pause continuation while preserving objective,
budget and usage. It may never clear, recreate, complete or automatically reactivate
a Goal. A paused or cleared Goal encountered during reconnect remains paused or
cleared. If the installed API cannot enforce this contract, daemon standing-desk
mode is unavailable; report could-not-check rather than a false successful drain.

Controller or server failure leaves an unfinished record. Reconnection observes and
reattaches to the same proven session. Recovery never kills an arbitrary recorded PID,
matches sessions merely by cwd, deletes the lock to force progress, or restarts a
paused Goal. Partial shutdown reports the remaining session/command IDs and host.
No response from a remote host remains unknown until observed or explicitly fenced.

## 6. Compatibility, alternatives and rollout

| Alternative | Assessment |
| --- | --- |
| Keep process mode only | Lowest integration cost; retain as fallback, with explicit daemon opt-out. Does not deliver shared runtime attachment. |
| Use one operator-wide daemon for every cell | Convenient, but unsuitable as the default without proven policy separation and ownership-scoped shutdown. |
| Own a server per cell/policy domain | Recommended first daemon deployment: bounded sharing and clear lifecycle custody. Adds endpoint/configuration management. |
| Build a universal agent server inside cellctl | Duplicates vendor execution and history machinery; reject. A thin adapter is sufficient. |
| Move all desks to managed cloud agents | Changes credential, network, artifact and approval boundaries; separate deployment decision, not a daemon optimization. |

First qualify one isolated host cell with synthetic work and two distinct role
identities. No production probes, global daemon restart, or automatic vendor update
is part of qualification. Measure aggregate process-tree CPU/RSS, startup/reattach
latency and stop/recovery correctness against explicit process mode using the same
workload. Report failed isolation and lifecycle cases even if sharing saves resources.

Then offer opt-in host daemon mode with an exact tested version/capability range.
Upgrade only after draining owned sessions; preserve bindings and stopped/paused
state. Rollback to process mode requires confirmed quiescence of the old runtime,
not simply a changed configuration flag. Historical conversation import/resume is
qualified separately and cannot reset claims or usage.

Keep scrubbed/container entrypoints, Windows behavior and non-Codex adapters unchanged
until separately qualified. Add remote Codex attachment next, reusing the execution
host's owner. Claude SDK and Cursor worker adapters follow only when their lifecycle
proofs satisfy the same outcomes. Cursor must retain its agent/run distinction;
Claude Remote Control must not silently replace current role settings or admission.

Affected consumers are direct `desk`/`up` launches, `down`, cadence status/recovery,
cockpit attachment, dispatch admission, comms shutdown, role context/model policy,
and operator status views. Implementation briefs must trace each consumer; a change
only to Codex launch argv cannot satisfy this spec.

## 7. Acceptance and decisions

| Scenario | Required observation |
| --- | --- |
| Two roles share a server | Effective model, command environment, MCP/plugin access and approval policy match each role; no cross-role credential exposure. |
| Terminal closes and another client attaches | Same bound session and worktree; no duplicate turn, role claim or new Goal. |
| Drain during accepted work | Existing work finishes; attempted new dispatch is refused; final status proves quiescence and preserved objective/usage. |
| Stop with native child agents and background commands | Owned descendants and commands cease; unrelated cell and personal sessions remain running. |
| Paused or cleared Goal on reconnect | No automatic resume/recreation; actual native state is reported. |
| Waiting for human approval | Visible pending gate; neither successful completion nor automatic approval. |
| Lost create/submit response or controller crash | Unfinished binding blocks duplicate creation; recovery reconciles or reports unknown. |
| Wrong server, configuration home, version or settings | Refusal before role execution; no opportunistic fallback or secret values in errors. |
| Host B disconnected while controlled from A | B remains the execution owner; no replacement on C; status records observation age. |
| Upgrade or rollback with surviving work | Refused until prior execution is stopped or explicitly handed off. |
| Contained reader after transport extraction | All forbidden callbacks still refused; its original custody and isolation tests pass. |
| Alternative harness lacks stop/continuation capability | Explicit unsupported result; no invented equivalent to native Goal pause. |

Current evidence supports the proposed seam and protocol candidates. It does **not**
yet prove daemon role isolation, effective MCP credentials, complete descendant
cleanup, standing Goal preservation during external control, or remote fencing.
Those are acceptance work, not conclusions of this document. Failure of the first
or fourth acceptance case rejects shared daemon deployment even if basic launch works.

The human driver owns these consequential decisions:

- **D1 — Goal authority:** recommend narrowly authorizing pause for explicit cell
  drain/stop after preservation tests. Alternative: keep native Goal control solely
  with the user; then cellctl must expose the remaining human action and cannot
  claim unattended daemon drain/stop. Update the cadence contract only with the ruling.
  D1 concerns native Goals only. Claude has no native Goal, and its continuation is
  the celld-owned `Stop` hook, so the Claude lane neither needs nor implies D1's
  authority. Its policy still has to pass S4.
- **D2 — Initial boundary:** recommend local, opt-in Codex servers per cell/policy
  domain. Wider operator sharing requires additional isolation evidence; remote/cloud
  rollout is a later decision. Credential provisioning remains an implementation
  prerequisite rather than an assumption that operator state may be copied.
- **D3 — General cellctl direction:** recommend the shared session/host contract with
  capability-specific adapters. Do not require every harness to expose identical
  services, and do not add live migration or automatic cross-host failover.
- **D4 — Cell API relation:** recommend that `celld` exposes no operator ingress and is
  reached only through the R7 Cell API owned by the active `cell-api` stream. The R7
  owner decides whether `celld` is packaged inside or behind that API. Until then this
  spec adds no listener for operators or external callers.

After review, record rulings through the [spec lifecycle](../spec/lifecycle-v1.md#8-spec-and-scoping-doc-lifecycle).
Only a separately authorized decomposition should create implementation briefs.

## 8. Distance from today's implementation

The architectural distance is smaller than the operational distance. The intended
owners already exist in the [future architecture](proposals/non-ci-architecture/11-target-architecture.md):
a cell API/controller, runtime coordination, isolated runners, separate effect
executors and human clients. Its initial deployment is one host with isolated cells,
not an already qualified cluster. No `celld` implementation was found in the inspected
`tools`, `docs` or `spec` trees at the source pin above.

| Slice | Available now | Remaining work |
| --- | --- | --- |
| Role launch and policy | Worktrees, harness argv, role/model resolution and process ownership | One declarative per-role route covering Cursor and qualified remote runtimes; preserve current refusal cases until supported |
| Invocation contract | `loopadmin/runner` and fake-adapter conformance tests | Actual harness adapters, persistent receipts, session attachment and native background-work semantics |
| Long-running supervision | Local cellcadence leases and process cleanup | Durable shared supervisor/journal, runtime binding after client exit, authenticated control endpoint |
| Service and client | Operator CLI and cockpit adapters | Celld API, shared client contract, per-cell authorization and session/event projection |
| Containers and hosts | Local Docker engine with explicit socket and ownership inspection | Authenticated runner registration, host identity, placement, reconnection and pod lifecycle qualification |
| Recovery | Local unfinished-owner refusal; runner unknown/cancel contracts | PostgreSQL restart/restore proof, effect-boundary fencing and independently confirmed cross-host stop |

This is a substantial integration increment, not a rename of `cellctl` or one daemon
flag. A percentage-complete or calendar estimate would be false precision before the
real-runtime qualification below. A useful local prototype is much nearer than a
reliable multi-host service. Existing `deskd` is a forge-read/index service launched
by [deskd.go](../tools/desk/cmd/cellctl/deskd.go); it is not this execution controller.

## 9. Experiment that decides whether this works

Test the smallest complete journey first: one Cursor worker produces a candidate,
one independent Codex reviewer examines that exact candidate, and an operator can
detach, reconnect, drain and stop both through the proposed control contract. Use a
synthetic repository with a planted defect and a local handoff manifest in place of
forge writes. A candidate hash, author/reviewer identity and actual served model must
survive the journey. Passing a model prompt alone is insufficient.

| Stage | Environment and experiment | Pass evidence / what it cannot prove |
| --- | --- | --- |
| A: current foundations | Run existing runner unknown-launch, stale-generation, cancellation, unsupported-resume and model-fallback controls; run interactive/cadence ownership controls | Named test results establish reusable local contracts. They prove no daemon or remote deployment. |
| B: service contract | One local celld prototype, disposable PostgreSQL, two separately supervised fake runners and two clients. Inject lost launch replies, duplicate registration, expired ownership and controller restart | One invocation per accepted identity, same session after reconnect, stale owner cannot publish, pause/drain survive restart. Fake runners prove control logic only. |
| C: real harnesses | Repeat the candidate/review journey with pinned Cursor and Codex versions on one test host, separately provisioned credentials and isolated workspaces | Actual model/session readback, exact candidate review, background-child stop and role separation. Native CLI output/capabilities may reject the proposal; record that rather than simulate parity. |
| D: remote execution | Move one runner to a disposable second host. Disconnect clients, interrupt the transport, restart the runner agent and attempt duplicate admission | Work remains with its owner; the second client reattaches; network loss becomes unknown and prevents replacement. This proves selected remote paths, not cluster HA. |
| E: cluster packaging | After separate authorization, repeat in a disposable cluster with pod deletion, termination grace and lost-volume/retained-volume cases | A replacement pod cannot revive stale authority or reset budget/stop state; forced termination is reported accurately. No production cluster is needed. |

Stage A can run offline now. B is new prototype implementation, not an existing
command. C needs a separately authorized model-backed trial; D/E need explicitly
selected disposable infrastructure. Do not count schema generation, container startup
or a mock session as real-harness qualification.

### Required spikes and their decision outputs

These are bounded experiments to resolve design uncertainty, not implementation
briefs or authorization to activate a service. Apart from the eight existing Stage A
tests recorded below, **none of these spikes has been executed**. The runtime
maintainers own the prototypes; an independent verifier checks their retained
evidence; the human driver owns activation and the consequential decisions in section 7.

Each spike delivers a reproducible fixture, pinned versions, exact commands and
results, a capability/limitation manifest, and a short decision report. Use synthetic
repositories and non-secret transcripts. Reports must separate checked-clean,
checked-failed and could-not-check. Stop at the named decision; do not turn a spike
into a production implementation or silently add another service/database.

| Spike | Question and bounded experiment | Required evidence and decision |
| --- | --- | --- |
| **S1 — Codex server boundary** | Can one owned server preserve two roles' distinct model, command environment, MCP/plugin access and approval settings? Use two synthetic roles, two clients and one pinned server; detach/reattach each and inspect children. Compare with separate servers. | Actual effective settings and served-model records, endpoint/home identity, denied cross-role access, and the same bound session after reconnect. Decide shared-per-cell versus separate-per-policy-domain servers. Any process-wide setting that widens a role rejects sharing. CLI schema presence is insufficient. |
| **S2 — Mixed-harness candidate handoff** | Can a Cursor worker produce a candidate that a Codex reviewer independently checks through the shared runner contract? Use one planted defect, immutable candidate hash and isolated author/reviewer workspaces. Characterize a Claude process fixture through the same contract; real Claude qualification remains owned by the existing first-adapter brief. | Preserve role, actual harness/model, candidate and attempt identity end to end; the reviewer detects the planted defect. A changed candidate invalidates review currency. Record supported and unsupported operations separately per harness. Decide the minimal adapter/profile extension, including how Cursor joins per-role policy without bypassing its present refusals. |
| **S3 — Durable controller and supervisor** | Can the existing owners support a celld client/service split? Use disposable PostgreSQL, one cell owner, two fake runners and two clients. Inject a lost Start reply, duplicate invocation/registration, controller restart, unavailable database and backup rollback. | No duplicate accepted launch; unknown survives restart; stale generation/incarnation cannot produce an accepted effect even when bypassing the upper controller. Database failure refuses new admission; rollback starts paused. Decide the required changes to the runner contract, supervisor journal and ownership interfaces. Fake-provider success proves no vendor fencing. |
| **S4 — Drain, stop and native continuation** | Can real runtime state prove quiescence? Exercise a synthetic Codex Goal with child agents/background commands and a Cursor run, including approval wait, lost cancellation reply and a client disconnect. **Claude lane:** with one owned Agent SDK process, drain through the `PreToolUse` admission hook (deny and defer), continue and freeze through the `Stop` hook, interrupt a turn, stop a background task with `stopTask`, wait on an approval, and rebind by session ID after a controller restart. Test drain during existing work and attempts to refill after completion, and test interrupting in-flight work only through the confirmed-plan abandon path, with each abandoned item recorded as unknown. | Drain is bounded and rejects new admission while accepted work finishes; a timed-out drain leaves the role up with its remaining work listed. Stop is accepted only for a paused or drained role, and confirms the owned execution tree ceased; unrelated sessions survive. Goal objective, budget and usage remain intact and paused/cleared state never revives. Decide D1 and each harness's stop capability. Unconfirmed cancellation stays unknown. Changes to real session Goal state require the driver's explicit trial authorization. For Claude, also record hook-timeout behaviour, which owned tasks can be enumerated, and what a session rebind does not restore, each as verified, unavailable or could-not-check. |
| **S5 — Remote registration and attachment** | Does the same ownership contract work on two disposable hosts? Register an operator-bound runner, connect two clients, cut the transport, replay an old registration and restart the runner. | Name who owns any cross-host session store, how access to it is bound, and when its data is retained or deleted, as for host identity and transport; transcripts can carry sensitive tool output. Unauthorized/cross-cell registration is rejected; host identity and owner generation remain bound; reconnect finds the original session; contact loss cannot cause a duplicate on another host. Decide transport, credential provisioning and supported recovery. This does not qualify automatic failover. |
| **S6 — Kubernetes lifecycle** | Does cluster packaging preserve the qualified contract? In a separately authorized disposable cluster, run one celld owner and two runner pods; delete a runner, exhaust termination grace, restart the controller and test both retained and lost workspace/session volumes. | Ready is not reported before registration and reconciliation; termination receipts distinguish confirmed stop from unknown; replacement cannot revive an old owner or reset budget/drain/Goal state. Retained artifacts identify the exact candidate after replacement; lost state causes a visible hold. Decide the pod/PVC/probe contract and whether a pilot is supportable. No production cluster access or HA claim. |

Run S1 and S3 first; their protocol/ownership decisions constrain the service shape.
S2 follows S1's runtime characterization and S3's fixture contract. S4 follows S2
and explicit authorization for synthetic Goal changes; its evidence informs the
production authority decision D1. S5 requires S3/S4, and S6 requires
S5 plus separate cluster-trial authorization.

The S4 Claude lane qualifies **this spec's** section-3 SDK control mapping. It is not
a lane of [brief 22](streams/graph-execution/brief-22-local-harness-adapter.md), which
has no SDK mapping. Brief 22 defines the Claude adapter as a wrapper over the resolved
Claude Code CLI argv/env profile that cellctl already builds, and it adds no second argv
builder or process runner. An owned Agent SDK host process is a different runtime shape.
The S4 Claude lane is therefore a separate qualification lane and test fixture only. It
does not amend or supersede brief 22 and does not wait on it. Adopting SDK hosting for
production would need a later amendment to brief 22, decided by its owner, that
reconciles the single-argv-builder and single-process-runner constraint.

The first useful delivery milestone is **S1–S4 locally**, with a thin client rather
than a new desktop UI. A failure to preserve role policy or prove scoped stop blocks
daemon standing-desk adoption. S5/S6 decide remote deployment separately; local
success must not be reported as a completed cluster design. Record unresolved gaps
and their owning contracts before estimating the production implementation.

Author checks on 2026-10-13 at the pinned source passed all eight selected existing
tests: runner `UnknownLaunch`, `LateResultDenied`, `CancelStillRunning`,
`ResumeUnsupported`, `ModelFallback`, and `BothModesWithoutGraph`; cellcadence
`InteractiveOwnershipAndCadenceTransition` and `InteractiveUncertainRefusesBothModes`.
The runner cases use fake adapters. These results establish only Stage A's selected
foundations, not an implemented celld, a complete qualification suite, or independent
verification of this proposal.

Retain a machine-readable receipt for each transition: cell/role, invocation and
attempt, owner incarnation/generation, runtime/session/child IDs, harness version,
candidate hash, requested versus observed model, observation time, requested action,
confirmed result and outstanding unknowns. Retain non-secret event transcripts and
independent filesystem/process observations for cancellation tests. A recorded cancel
request alone never proves that execution stopped.

Measure service CPU/RSS separately from harness descendants, startup and reattachment
latency, duplicate launches, stop-confirmation time, outstanding work at drain,
recovery time and unresolved observations. Establish operational thresholds with the
owner after a baseline; correctness bars are no duplicate admission, no stale-owner
effect and no false stopped/drained result in the prescribed cases. If either real
harness cannot meet a required capability, keep its process path and report partial
support; do not broaden permissions to make the demonstration pass.

## 10. Consequences for the future architecture

`celld` should be the service packaging of the existing R2/R3 domain owners, reached
only through the Cell API, with `cellctl` and the desktop/cockpit as clients of that
API rather than of `celld` directly. Start with one service owner per logical cell; several
cells may share a host. A shared ingress may route to those cells without becoming a
cluster-wide authority holding every role's credentials. A cell is a policy and
ownership boundary, not a synonym for one pod: a controller and its isolated runner
pods can belong to the same cell.

Runner registration means authenticated identity/capability/liveness reporting; it
does not mint permission to claim arbitrary work. The operator provisions a cell
binding, celld validates it, and a registered runner receives only an admitted,
profile-bound invocation. Kubernetes may place or restart a pod; it does not decide
work eligibility, accept a review, fence a provider write or resume a paused Goal.

The [amendment manifest](proposals/non-ci-architecture/20-amendment-manifest.md), the
[cell operator surface](proposals/non-ci-architecture/21-cell-operator-surface.md),
the [runtime contract alignment](proposals/non-ci-architecture/23-runtime-contract-alignment.md)
and existing graph-execution briefs already name the relevant owners. `celld` relates to
each as follows:

| Owner | Relation of `celld` |
| --- | --- |
| R7 Cell API and thin cockpit (doc 21; active `cell-api` stream; doc 23 owner table) | `celld` is not the Cell API and has no operator ingress of its own. The Cell API is the only ingress; `celld` serves it and registered runners (D4) |
| R2 coordination and R3 isolated runner (doc 21 lifecycle; doc 23 owner table) | `celld` packages the execution-host side. R2 records a requested transition, R3 executes it; section 4 maps every operation onto doc 21 and adds no transition |
| [desk-containers 15-24](streams/desk-containers/README.md) (doc 21 interim verbs) | The interim `up`, `down`, `status`, `logs` and per-role stop keep their meaning. A `celld` mode is an adapter behind them, not a replacement |
| R4 role executors and R6 forge service (doc 23) | Effect credentials and provider effects stay with them; `celld` hosts sessions, not effects |

The graph-execution briefs name these further owners:

| Existing owner | Required daemon/service reconciliation |
| --- | --- |
| [20: runner contract](streams/graph-execution/brief-20-runner-contract.md) | Extend versioned capabilities for attachment, descendants and native continuation where justified; retain Start/Observe/Cancel/Reconcile and unknown semantics |
| [26: shared supervisor](streams/graph-execution/brief-26-shared-loop-admin.md) | Bind daemon sessions and registrations to invocation ownership; use its PostgreSQL journal and existing process-runner migration, not a new scheduler |
| [22: first adapter](streams/graph-execution/brief-22-local-harness-adapter.md) | Specifies a Claude Code CLI-profile adapter with one argv builder and one process runner. The Codex daemon experiment and the SDK lane (S4) are separate qualification lanes; neither amends it, replaces its scope or moves Cursor onto an unqualified adapter |
| [27: standing desk clients](streams/graph-execution/brief-27-standing-desk-loop-admin.md) | Route current desk callers through the shared supervisor while preserving their scheduling and admission semantics |
| [16: ownership and budgets](streams/graph-execution/brief-16-cell-ownership-budgets.md) | Reuse incarnation/generation, cumulative budget and restoration fencing; registration and heartbeat are not a second authority |
| [19: durable store](streams/graph-execution/brief-19-durable-instance-store.md), [21: controller](streams/graph-execution/brief-21-controller-host.md) | Keep work state and workflow decisions with their existing owner; session status cannot substitute for evidence or acceptance |

The durable target is PostgreSQL coordination, Git/forge-retained canonical work and
evidence, and harness-owned conversation state. A vendor session database is not the
work ledger; the transitional cellcadence checkpoint is not an alternative distributed
store. Restore follows the existing
[storage and operator-incarnation contract](proposals/non-ci-architecture/14-storage-forge-graph.md).
Native harness continuity supplies execution convenience, never restored authority.

Effect credentials remain outside model-working sessions under the target design;
moving an existing desktop credential arrangement into a pod does not establish that
boundary. Retain the separate role-bound effect executors and their final admission
checks. Likewise, the comms gateway carries messages; it must not become a parallel
launch scheduler or substitute for acknowledged celld operations.

The resulting change is primarily a client/service and runtime-adapter cutover within
the approved direction. Authenticated multi-host registration, cluster placement and
automatic recovery expand the currently host-first deployment scope and require their
own qualification. Update the canonical future-architecture documents and affected
unfinished briefs together after review; this draft does not amend their authority.

## Execution-environment contract follow-through — 16 October 2026

The [future-state execution amendment](proposals/non-ci-architecture/27-agent-execution-environments.md)
and [agent-execution stream](streams/agent-execution/README.md) own the newly routed profile,
preparation and preservation extensions. They retain the Cell API, doc 21 lifecycle and
R2/R3/R4 authority boundaries described here. S1–S6 still explore vendor/runtime, daemon,
remote and cluster behavior under their existing authorization; agent-execution/06 only
compares AX source fixtures offline. It is not an alternative S5/S6 trial. Reuse a qualified
profile/receipt where applicable; schema support alone does not qualify a harness or host.
