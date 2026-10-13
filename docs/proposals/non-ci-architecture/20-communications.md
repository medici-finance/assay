# Future-state communications: reusable service, thin clients, broker-owned delivery

**Status: draft specification, 13 October 2026.** This is the communications target
within the future-state architecture, not a description of deployed behavior. The
requested direction is reusable client/server packages and reconciliation of existing
unfinished work. Review and approval bind this document's revision; broker selection,
trust changes and deployment activation are separate decisions. Existing evidence and
recorded approvals are not superseded by a new diagram or package name.

## Outcome and scope

An authenticated tool or agent sends a typed message to an authorized mailbox, within
or between cells. Required filtering happens before recipient access. The recipient
may be offline; delivery resumes when it reconnects. Acknowledgment records delivery
processing, not acceptance of a brief, permission to execute, or a human decision.
No agent-specific transport is required. Agents and deterministic tools are clients
or filter workers of the same messaging service.

Keep Assay's semantic verbs and policy above a generic messaging contract. Use an
established broker for eventual durable storage, delivery state and redelivery. Do not
build another distributed queue, workflow engine or agent framework. Broker selection
is not a prerequisite to extracting the existing implementation safely.

## Observed implementation and limitations

Source snapshot: public Assay
[`a435badbd1d245402407acd2a82a6abe5f37f637`](https://github.com/medici-finance/assay/tree/a435badbd1d245402407acd2a82a6abe5f37f637),
fetched 2026-10-13. These are source observations, not deployment or test-pass claims.

| Source | Existing responsibility | Extraction consequence |
|---|---|---|
| `tools/desk/internal/comms/{envelope,identity,laneacl}.go` | cellmsg-v1 parsing, assertions, fixed Assay vocabulary and lane ACL; laneacl imports deskkit | Separate generic contract from Assay policy; moving the directory alone is insufficient |
| `tools/desk/cmd/commsgw` | Local listener, A2A inbound, admission checks, mailbox operations and filtering | Service logic is package main; extract before another host can reuse it |
| `tools/desk/cmd/commsloop` | Drain, routing, mailbox delivery and optional native dispatch | Messaging routing/filtering is reusable; session execution belongs to the existing runtime owner |
| `tools/desk/internal/commsqueue` | File-backed accepted/held records, mailbox and acknowledgments | Retain as a transition backend; atomic file replacement alone is not a distributed durability guarantee |
| `tools/desk/cmd/deskcomms` and `internal/commstransport` | send/poll/ack and local Unix-socket/Windows-pipe transport | Extract a narrow client; preserve custody and command behavior |
| `tools/desk/cmd/cellctl/comms*.go`, `docs/cellctl-comms.md` | init/check/run, manifest, service supervision and client environment | Extend this configuration owner; do not invent a parallel operator CLI |

The documented host profile is local-only interim mailbox delivery. A2A inbound code
is not evidence of complete outbound cross-cell forwarding. The existing trust store
has one public key per cell; role names do not establish independent per-client
cryptographic identity. Current assertions do not establish end-to-end binding of every
payload/routing field. Extraction must preserve and describe these limits, not silently
advertise the stronger future contract.

Quality evidence is structural: internal imports block external consumers and command
packages mix transport, policy and lifecycle. Existing bypass, identity, mailbox and
native-dispatch tests are the regression input. No fresh complexity/churn measurement
or comparative reliability benchmark was run for this spec. Remediate those package
seams only; a Rust rewrite, generic workflow engine or unrelated CLI cleanup is outside
this extraction. Improvement is demonstrated by an external consumer and one shared
implementation with equivalent observable behavior, not line-count reduction.

## Target ownership and package boundary

Initial implementation remains Go. The module is
`github.com/medici-finance/assay/comms`, rooted at `comms/` in the Assay repository,
with its own `go.mod`. Desk-tools is a consumer, not the comms library's parent module.
`commsgw`, `deskcomms` and a separately built service-host fixture are named consumers.
An independent repository is optional later; the module boundary is part of the first
extraction. Public packages must build with `GOWORK=off`. The comms module must not
import tools/desk, deskkit, forge adapters or desk runtime packages, even transitively.
Assay-specific composition stays in desk-tools and depends inward on comms ports.
Use explicit module versions for consumers; qualify with an offline module proxy before
claiming release readiness. A local replace may aid development but is not release proof.

| Package / composition | Owns | Must not own |
|---|---|---|
| `comms/contract` | Versioned envelope, typed addresses, delivery/error/receipt types, canonical parsing | Assay role lists, credentials, environment reads, network calls |
| `comms/client` | Typed Send, Poll and Ack API; version negotiation/refusal | Server credentials, queue filesystem access, grants or policy overrides |
| `comms/server` | Admission and mailbox use cases; typed auth, policy, filter and backend ports | Forge credentials, native session spawning, CLI parsing |
| `comms/backend/file` | Existing queue implementation behind the service port during transition | A newly generalized distributed queue |
| `comms/config` | Pure parsing/validation and resolved configuration/provenance types | Hidden environment search, secret minting, automatic activation |
| Assay composition under `tools/desk/internal` | Current verb/role allow-set, prose/risk policy, stop/budget checks and runtime handoff | A second generic transport or mailbox engine |
| Selected broker adapter, later | Broker-native persistence, consumption and redelivery mappings | Assay workflow acceptance or human authorization |

The exact Go signatures land in comms-core/01. Required logical operations are
`Send(context, message) -> receipt`, `Poll(context, mailbox, cursor/limit) -> deliveries`
and `Ack(context, deliveryToken) -> receipt`. The authenticated caller is supplied by
trusted transport middleware, never accepted from a payload's role string. A versioned
wire contract carries equivalent operations for non-Go consumers. Initially preserve
the existing wire through a compatibility adapter; a stronger identity/delivery version
is a separate migration with explicit version rejection.

`commsgw` hosts this service standalone. `deskd` may compose the same Go service once its
public service boundary and custody profile permit it. A Rust deskd uses the client/wire
boundary to the standalone Go service first; no FFI or second server implementation is
implied. Logical packages do not each require a process. The broker normally runs as a
separate managed process. Existing `commsloop` becomes an adapter around shared filtering
and delivery work; runtime execution remains outside the generic comms library.

## Identity, hierarchy and filtering

A future address identifies cell, mailbox and client/principal where required. Distinct
client credentials or a trusted delegating gateway bind attribution. The authenticated
principal must match its permitted sender identity; payload fields cannot impersonate
another client. A broker credential authenticating a cell does not by itself authenticate
an individual agent inside that cell. Preserve legacy cell-wide trust as an explicitly
limited profile until the stronger profile is implemented and qualified.

Parent/child connections describe topology. Permissions independently determine allowed
sender, destination, verb, subscription and administrative operations. Parent status
never implies read-all or write-all. Leaf-to-leaf forwarding is allowed only under an
explicit route and policy; bound hop count/loop detection prevents forwarding cycles.

Future flow: admission → durable pending message → required filter → durable approved
mailbox or held disposition → recipient delivery → acknowledgment. Broker permissions
must prevent producers publishing directly to approved mailboxes and recipients reading
pending/held data. A filter may be a deterministic tool or an agent, but receives only
its filtering capability, has bounded time/resources, and cannot bypass deterministic
authorization. Timeout, unavailable policy, malformed result or filter failure holds;
no fallback delivers unchecked content. Release from held state is separately authorized,
attributed and revalidated; a model's opinion cannot grant a human-only action.

Keep content/routing immutable under the message ID. Changed content is a new message
with a correlation reference. For delegated forwarding, persist an authenticated origin
record bound to content hash, route and policy version; reauthenticate each hop. Short-lived
transport credentials may expire while a message is stored: forwarding uses current
service credentials while retaining original admission provenance, rather than replaying
an expired client assertion as fresh authorization. Recheck current destination policy
and revocation before delivery. No universal cross-cell trust is inferred.

## Delivery and failure contract

- A successful submit means durable acceptance under the selected backend profile;
  return message ID and acceptance stage. On timeout the result is unknown: retry the
  same ID and identical content or query its status, not a new identity.
- Deduplicate within a documented scope and retention window. Reusing an ID with changed
  content refuses. Specify ordering per mailbox/key only where required; no global order.
- Delivery is at least once. Ack is bound to the authenticated recipient and current
  delivery token. Another client cannot acknowledge it; stale tokens refuse. Recipient
  business effects require their own idempotency/reconciliation.
- The filter must persist output/disposition before acknowledging input. Crash between
  these steps can repeat processing; stable derived identities prevent duplicate delivery
  effects. A backend-specific transaction may optimize this, but is not assumed.
- Bound payload sizes, retention, retry/backoff, poison-message handling and per-cell
  quotas. Full storage refuses durable acceptance explicitly; never acknowledge then
  silently discard. Expiry and operator discard produce observable dispositions.
- Disconnected cells need local durable storage and configured forwarding, not merely a
  persistent connection or broker leaf link. Retain local responsibility until the next
  durable hop acknowledges; account for expiry, duplicates, routing loops and revocation.
- Separate accepted, filtered, delivered/acked, held, expired and rejected states. None
  means the requested work succeeded. Expose metadata and lag only to authorized viewers.

## Storage boundary and broker decision

Amend the broader architecture's state table: the selected broker owns message transport
state; PostgreSQL continues to own Assay attempts, leases, business effect intents and
query projections. When a SQL transaction must emit a message, use its transactional
outbox and a retrying relay with stable message identity. The receiver uses a durable
inbox/idempotency record for business effects. These are application transaction bridges,
not a second broker implementation. SQL/broker acknowledgments cannot form an assumed
atomic transaction. Broker loss is not automatically reconstructible from Git/forge:
accepted messages need their own backup/replication and declared loss policy.

Select one initial supported broker after a bounded disposable qualification:

| Candidate | Why evaluate | Discriminating question |
|---|---|---|
| RabbitMQ | Durable mailbox/task semantics and established inter-broker forwarding | Can the selected queue/forwarding profile meet disconnected-cell recovery and operator cost? |
| NATS/JetStream | Per-cell topology, live messaging plus durable streams/consumers | Can configured local domains/forwarding retain and deliver safely across outages? |
| Kafka | Existing operational investment, retained event history/replay; modern queue capabilities | Do those benefits justify per-cell or shared-cluster and replication complexity for this workload? |

First inventory infrastructure already operated. Pin versions, license, client support,
disk/memory footprint, startup/recovery, permissions, packaging, upgrade and backup burden.
Exercise the same failure cases below. Do not implement all three adapters to make a
comparison, and do not claim broker independence beyond the qualified contract. A2A can
remain an external interoperability adapter when an actual consumer requires it.

## Migration and existing backlog

The initial path is behavior-preserving refactoring, not a broker switch:

1. **comms-core/01:** contract and Assay policy separation; characterize legacy behavior.
2. **comms-core/02:** shared service and existing file backend; standalone gateway/drain adapters.
3. **comms-core/03:** client SDK; deskcomms delegates; prove external consumer packaging.
4. **comms-core/04:** shared config validation; extend existing cellctl configuration management.

02 follows 01; 03 follows 02; 04 follows 02 and desktools-v2/16. Broker qualification can
proceed alongside extraction after the delivery contract is reviewed. deskd integration
waits for the actual host API/custody seam, not a wholesale rewrite of deskd.

| Existing work at source snapshot | Disposition applied or required |
|---|---|
| desktools-v2/42, todo | Amend now: depends on 02/03 and existing /16; parser changes call the shared service/client; no domain implementation in Cobra handlers |
| desktools-v2/16 | Keep its scope/status; add inverse edge for config consumer 04 only |
| measured-status/03, implemented | Preserve Task/Evidence and Option 2 history; #1722 remains a risk-signal follow-up, not fulfilled by extraction; its delivered regression must survive moves |
| Existing comms implementation and released tests | Reuse and preserve; do not reopen completed briefs as extraction work |
| R1/R2/R7/R8 architecture packages | Clarify message identities, outbox ownership, service hosting and broker migration obligations in the existing delivery sequence |
| Adopter mode-enforcement and activation plans | Amend in their own repositories: use shared service/config, preserve their decision history and gates; no private identifiers or policy copied here |
| Unrelated CLI, forge, graph and Windows work | No blanket dependency on this stream; only actual imported interfaces justify an edge |

This inventory covers tracked public brief sources matched for comms/mailbox/gateway and
the relevant architecture packages. Source-board status does not establish active-claim
or open-PR status. Refresh those before dispatch/landing; do not overwrite an active
implementation's scope without an explicit checkpoint. A private companion reconciliation
records adopter-specific changes without making public delivery depend on private briefs.

After extraction, author bounded broker-qualification/selection, chosen-adapter,
service-host integration and cutover briefs. Each names test commands, versions, policy
changes and migration ownership. They must not be treated as already dispatchable work.
No broker choice or stronger client identity is silently included in the four refactors.

At cutover pause producers/drains for the selected scope; inventory accepted, held,
unacknowledged and acknowledged IDs; import or drain exactly once with a reconciliation
manifest; switch one delivery authority; resume and verify. Never dual-write two queue
owners without a proven reconciliation protocol. Rollback is trivial only before new
writes; afterwards reconcile new IDs and dispositions before switching back. Preserve
quarantine/audit history, release pins, old-client failure messages and a bounded adapter
retirement condition. Urgent fixes on supported legacy paths remain independently shippable.

## Acceptance and falsification

Initial extraction acceptance: old command/wire/config fixtures and security refusals
remain equivalent; direct calls bypassing CLI checks still refuse; no public client
imports server/custody/runtime packages; independent module consumer builds without a
workspace/relative replace and sends, polls and acknowledges through the service. Old
file state remains readable. Each extracted behavior has one owner, not two copied bodies.

Before selecting/activating the durable profile, prove in disposable environments:

1. Two cells exchange allowed messages; forged sender, unauthorized verb/destination,
   direct approved-mailbox publication and another recipient's ack all refuse.
2. Recipient and upstream link disconnect; locally accepted messages survive service and
   broker restarts and arrive after reconnection within the declared retention profile.
3. Kill filter after output persistence but before input ack; processing resumes without
   a second business effect. Filter failure holds; only authorized release can resume.
4. Retry after lost submit/ack reply; same ID/content converges, changed content refuses.
   Include expired transport assertion during a long outage and revoked destination policy.
5. Full disk/quota, expiry, poison message, routing cycle and unsupported wire version yield
   explicit bounded outcomes; no silent success and no cross-cell metadata disclosure.
6. Both standalone and composed hosts enforce the same contract through actual mounted
   routes. Disabled mode admits/drains nothing; interim never starts a session; full still
   requires separate admitted runtime capability and all existing safety gates.
7. Reconcile real legacy accepted/held/acked fixtures through cutover and rollback. Qualify
   SQL-outbox relay crash/retry independently from broker delivery.

Failure of the isolation or disconnected-delivery cases falsifies a candidate profile,
regardless of a successful demo. No live production access is needed for these checks.

## Decisions and completion boundary

The driver approves this target and explicit compatibility/authority deltas. The broker
and operational profile remain open until comparative evidence exists. Retain Go for the
first extraction with the independent `assay/comms` module; defer a separate repository
and a Rust server until a named consumer justifies them. Prefer existing cellctl commands over a second config executable.
The same source remains under `docs/proposals/` for link stability; “future-state” identifies
its architectural role and never claims the target is implemented or all prior proposals
have been ratified. This document and brief amendments authorize no live activation.
