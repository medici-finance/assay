# Cellctl sessions, Codex daemons, and execution hosts

**Status:** draft
**Date:** 2026-10-13
**Decision owner:** human driver; implementation owner: cellctl maintainers
**Scope:** design only. No daemon activation, credential migration, or rollout is authorized by this document.

## 1. Recommendation and outcome

Make `cellctl` manage **owned sessions on explicit execution hosts**, with terminals
as attachable views. Use each harness's supported runtime to execute agents; retain
cellctl's responsibility for role identity, workspaces, admissions, lifecycle and
evidence. Start with an opt-in Codex app-server adapter. Keep the existing process
adapter for harnesses or configurations that cannot satisfy the session contract.

A successful integration lets an operator detach a terminal, reconnect from another
device, see exactly which work still runs where, and drain or stop that work without
affecting another cell. It must refuse to create a replacement when the old owner is
uncertain. The benefit is lifecycle correctness and recoverability; reduced CPU,
memory or startup time is a hypothesis to measure, not a reason to assume sharing is safe.

This changes cellctl's unit of management from a launched client process to a role's
owned runtime session. It does not turn cellctl into an agent engine, transcript
database, vendor cloud scheduler, or transparent cross-host failover system.

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
| Codex owned app server | Server owns threads and turns; cell owns the server binding | First adapter to qualify, local host first |
| Claude CLI / Agent SDK | Owned process hosts a resumable conversation | Retain process adapter; qualify SDK lifecycle before expanding |
| Claude Remote Control | Serving host executes; vendor connection supplies remote UI | Optional operator attachment, not the admission authority |
| Cursor local CLI | Existing cellctl process/cadence integration | Retain current behavior |
| Cursor managed or self-hosted agents | Vendor owns agent/run lifecycle; selected worker executes | Separate adapter and trust review; do not pretend it is a local PID |

Do not normalize all these into a fictitious common `pause` operation or native
standing Goal. Each adapter advertises verified capabilities: observe, attach,
submit, interrupt, enumerate descendants, list/stop background work, preserve
continuation state, and confirm quiescence. Unsupported means unavailable, not
success. A standing desk requires a qualified continuation policy; a persistent
server alone supplies neither a timer nor a permanent-service guarantee.

Three different cross-host operations must remain explicit:

1. **Remote control:** a client on host A observes or steers execution on host B.
   B retains its filesystem, tools, credentials and responsibility for stopping work.
2. **Remote placement:** an authorized new task is assigned to B before it starts.
   Repository access and environment provisioning happen there.
3. **Migration:** an existing task moves from B to C. This is out of the first release.
   Conversation resume is insufficient: worktree changes, commands, approvals,
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
| Exclusive role ownership, desired admission state, recovery binding | Existing cellcadence ownership record, extended compatibly |
| Runtime-specific requests, events and status interpretation | Small harness adapter inside the desk Go module |
| Terminal, browser or cockpit attachment | Presentation client; attachment confers no new authority |
| Placement on another host | Explicit operator-selected execution target; no implicit scheduler |

Separate three configuration choices: **harness**, **execution target** and
**continuation policy**. A remote Codex daemon can still use interactive native
continuation; headless cadence remains a separate mode with native Goals disabled.
Changing the view must not change any of these choices.

Proposed operations are `inspect`, `attach`, `drain`, `stop` and explicit `resume`.
These are behavioral requirements, not claims that new CLI verbs already exist.
Existing `up` should attach to a proven owned session or create one only when no
unfinished owner exists. Existing `down` must report stop evidence before declaring
success. A future `detach` closes only the view. Preserve current command compatibility
through an explicit runtime mode; do not silently change running cells.

Extend `<cell>/run/cadence/<role>/checkpoint.json` with a versioned runtime binding:
backend, stable host identity, endpoint reference, server incarnation, cell/role,
ownership generation, root session ID, known child IDs, worktree identity,
configuration digest, last observation and desired admission state. Keep credentials
out of the record. Vendor IDs and host identities are authoritative only when bound
by the trusted controller; titles, cwd matches and model-written metadata are not.

The OS lock remains host-local. Remote clients must submit lifecycle operations to
the execution host's single owner through an authenticated transport; copying the
checkpoint or placing a file lock on shared storage does not create distributed
ownership. A later multi-controller service needs an explicit fencing design.

Reuse cellcadence leases/checkpoints, cellprocess ownership and existing cockpit
adapters. Retire terminal liveness as completion evidence for daemon mode. Do not
add a second job database or copy transcripts into cellctl. Status may cache bounded
observations, with their age; the runtime remains the source for execution state.

The contained reader's RPC implementation is private and deliberately restrictive.
Both consumers are in `tools/desk`, so extracting a policy-free codec into an
`internal` package is possible, but only with tests preserving the reader's callback
refusal, binary custody and feature restrictions. Otherwise use a small separate
adapter. Sharing transport code must not share tool authority or the comms socket.

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

Report drained only when captured obligations are completed or explicitly handed
off, no new admissions occurred, and no owned turn or background work can continue
admitting work. Freeze continuation at that boundary without marking a standing
objective complete. Prompt-only compliance is insufficient for the admission gate.

**Stop** is a distinct, explicitly requested interruption: close admissions, inhibit
automatic continuation, interrupt owned turns and descendants, and terminate owned
background commands as authorized. Re-read runtime state and retain receipts before
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
| Wrong server/home/version/configuration | Refusal before role execution; no opportunistic fallback or secret values in errors. |
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
- **D2 — Initial boundary:** recommend local, opt-in Codex servers per cell/policy
  domain. Wider operator sharing requires additional isolation evidence; remote/cloud
  rollout is a later decision. Credential provisioning remains an implementation
  prerequisite rather than an assumption that operator state may be copied.
- **D3 — General cellctl direction:** recommend the shared session/host contract with
  capability-specific adapters. Do not require every harness to expose identical
  services, and do not add live migration or automatic cross-host failover.

After review, record rulings through the [spec lifecycle](../spec/lifecycle-v1.md#8-spec-and-scoping-doc-lifecycle).
Only a separately authorized decomposition should create implementation briefs.
