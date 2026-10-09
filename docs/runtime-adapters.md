# Runtime adapters — claimable, branchable state without vendor lock-in

An agent run often needs state of its own: a database branch to migrate against, a snapshot
to restore, an eval corpus to score on, a bucket prefix to write into, a throwaway app
backend to exercise. The quickest way to get one is to have the agent call a provider's CLI
directly. That moves three things outside the methodology at once: **authority** (who
decided the resource should exist, and who may keep it), **cost** (who pays for it, and
until when), and **evidence** (what proves the resource existed, held what it should, and
was cleaned up). Once they are outside, nothing can review them.

This document specifies the contract that keeps them inside. Under it, a runtime resource
is an **external-effect boundary** with a typed record, not an invisible shell call. The
record names the run that owns the resource and the bounds it was provisioned under. It
also lists, in order, every effect performed on the resource, each with the adapter's
receipt.

The contract is vendor-neutral by construction. The normative sections below never name a
provider, and a local in-process mock satisfies all of them. Plain Postgres or SQLite
satisfies them just as well. The last section maps one hosted provider's flow onto the
contract as an informative example only.

**Artifacts:**

- The record schema is [`schemas/runtime-resource-v1.json`](../schemas/runtime-resource-v1.json).
- The reference validator is `statusgen runtime-resource --lint` (`statusgen/runtimeresource.go`).
- Worked fixtures, including one per failure class, are in `statusgen/testdata/runtime-resource/`.

**Ground rules this contract runs under, and expects every adapter to run under:**

- Assay is a methodology repo and creates no infrastructure.
- The validator is pure over the files it is given. It never contacts an adapter, a
  cluster, or a provider API, read-only included.
- Anything that needs live state to answer is reported `could-not-check`, with the reason.
  It is never answered by a probe, and never reported as a vacuous green.

The key words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

## Resource classes

Every record names exactly one class. The class tells a reviewer what kind of state is at
stake. It also tells the registry which adapters may hold the resource.

| Class | What it is | Typical input revision |
|---|---|---|
| `database-branch` | A copy-on-write or full copy of a database, isolated from its parent, that a run may migrate and mutate | the parent's commit or point-in-time |
| `snapshot` | A point-in-time, read-mostly capture of some state, restorable later | the state's revision at capture |
| `eval-corpus` | A versioned dataset an evaluation reads, materialized for one run | the corpus version |
| `object-prefix` | A namespace in object storage (a bucket prefix, a directory) a run may write under | the commit the writes derive from |
| `app-backend` | A disposable instance of an application's backend services, for integration or end-to-end runs | the deployed artifact's revision |

A class is not a vendor product. "A database branch" is the class whether an adapter
implements it with a hosted branching service, a `CREATE DATABASE … TEMPLATE`, or a copied
SQLite file.

## State machine

A record is always in exactly one state.

| State | Meaning | Established by |
|---|---|---|
| `requested` | The control plane accepted a request and assigned the record its id; no adapter has confirmed anything yet | the record existing |
| `provisioned` | The resource exists at the adapter, inside its TTL, unclaimed | a `create` receipt (or a reconcile receipt for an unacknowledged `create`) observing it `present` |
| `claimed` | A human or organization principal has taken ownership, so the resource may outlive its TTL | a succeeded `claim` by that principal, plus an `ownership` block naming them |
| `expired` | The TTL (or a claimed retention) has run out; the resource MUST be released | the clock passing the bound, read against the receipts |
| `released` | The resource is gone | a `release`, then a reconcile receipt observing it `absent` |
| `failed` | The resource was never established | a `create` that failed, or a reconcile observing an unacknowledged `create` `absent` |

Transitions:

```
requested ──create──▶ provisioned ──claim (human/org)──▶ claimed
    │                     │    │                            │
    │                     │    └──ttl elapses──▶ expired ◀──┘ retention elapses
    │                     │                         │
    └──create fails──▶ failed        release + reconcile(absent)
                                               ▼
                         provisioned / claimed / expired ──▶ released
```

The recorded state MUST be the one the receipts support. A record that says `provisioned`
with no receipt showing the resource present fails validation. The same goes for a record
that says `released` with no reconcile showing the resource absent. In both cases the
record and reality have drifted apart, and the record is the half that is wrong.

## Adapter verbs

An adapter is the component that actually holds resources of some class. It implements a
fixed set of verbs. Each verb is an **effect**: it carries a stable effect id and an
idempotency key, and it returns a receipt.

| Verb | What it does | Mutates? | Required of every adapter |
|---|---|---|---|
| `create` | Brings the resource into existence from its input revision | yes | yes |
| `inspect` | Reads the resource's current state at the adapter | no | yes |
| `snapshot` | Captures the resource's current state as a new, separately recorded `snapshot` resource | yes | no |
| `restore` | Replaces the resource's contents from a snapshot | yes | no |
| `claim` | Transfers ownership to a human or organization principal; MUST rotate or revoke the agent's credential | yes | no |
| `release` | Removes the resource; MUST revoke the agent's credential | yes | yes |
| `reconcile` | Reads the resource's real state to resolve an earlier effect whose outcome is unknown; records that read as an authoritative receipt | no | yes |

The required set is not arbitrary:

- Without `create` there is no resource.
- Without `release` nothing is ever cleaned up.
- Without `inspect` and `reconcile` a lost acknowledgment can never be resolved.

An adapter that cannot reconcile cannot satisfy this contract, however reliable its happy
path.

Adapters MUST deduplicate on the idempotency key. Sending the same verb with the same key
twice MUST have the effect of sending it once. That property makes a retry safe, but only
after a reconcile has said a retry is needed (see "Recovery and reconciliation").

## The resource record

A record is one YAML document conforming to `runtime-resource-v1`. Every field below is
mandatory unless marked otherwise.

| Field | What it holds | Why it is mandatory |
|---|---|---|
| `id` | The record's stable identity, assigned by the control plane before any adapter call | a lost acknowledgment must never lose the record |
| `class` | One resource class | decides which adapters may hold it |
| `adapter` | `id` (MUST be in the adapter registry) and `version` | an unregistered adapter is never trusted on its own say-so |
| `owner` | `brief` and `run`: the brief and the run the resource was provisioned for | a resource with no owning run cannot be reconciled or cleaned up |
| `input-revision` | The revision the resource was created from | evidence produced against the resource is valid only for this revision |
| `ttl` | Time to live from the establishing receipt (`30m`, `4h`, `2d`) | a resource with no TTL is not ephemeral, so no agent may request it |
| `quota` | One or more `{dimension, limit}` bounds | an unbounded resource is not one an agent may request |
| `data-classification` | `synthetic`, `public`, `internal`, `confidential` or `restricted` | decides what data the resource may hold, and so who may claim it |
| `provisioning` | `requested-by` (an `agent:`, `human:` or `org:` principal) and `via: control-plane` | provisioning authority (see the next section) |
| `credentials` | The credential-custody model (see "Credential custody") | durable credentials never reach an agent |
| `cleanup-owner` | `control-plane`, or a `human:`/`org:` principal; never an agent | someone accountable removes the resource |
| `evidence-kind` | `runtime-receipt` | the evidence class every effect produces (see "Receipts") |
| `cost` | `amount`, `unit`, and `basis` (`estimate` at request time, or `metered` from the adapter) | cost stays visible to the people who pay it |
| `recovery` | `reconcile-before-retry` | the recovery semantics, declared rather than assumed |
| `state` | One state from the state machine | the claim the receipts must support |
| `ownership` | *Present once claimed.* `principal`, `claimed-at`, optional `retain-until` | ownership authority, recorded |
| `effects` | Every effect on the resource, in issue order | the evidence itself |

Each `effects[]` entry carries the following fields:

- `id` and `idempotency-key`.
- `verb`.
- `requested-at`.
- `outcome`: `succeeded`, `failed`, or `unknown`.
- `principal`: who authorized the effect.
- On a `reconcile`, `reconciles`: the id of the earlier effect it resolves.
- On every succeeded effect, a `receipt`.

## Provisioning authority and ownership authority

These are two different authorities, and the contract never lets one stand in for the
other.

**Provisioning authority** is the right to ask for a bounded, ephemeral resource. An agent
MAY hold it, under these conditions:

- The agent asks only through the control plane (`provisioning.via: control-plane`), using
  an Assay verb.
- The request carries a TTL and at least one quota bound.
- The TTL MUST NOT exceed the adapter's registered `max-ttl`, where one is declared.

An agent never calls an adapter directly. If it did, there would be no record whose `id`
existed before the call, and nothing to reconcile against when the call's answer is lost.

**Ownership authority** is the right to claim a resource: to retain it past its TTL, to
promote it into something longer-lived, or to pay for it. Only a `human:` or `org:`
principal MAY hold it. An agent MUST NOT claim. A `claim` effect authorized by an `agent:`
principal, or by no principal, fails validation regardless of its outcome. So does an
`ownership` block that no succeeded claim by the same principal backs.

The split matters because the dangerous moment is the moment a resource stops being
ephemeral. An agent that could claim could turn a scratch branch into a standing database
with real data and a recurring bill, with nobody having decided that it should.

## Credential custody

- **Durable credentials stay with the control plane** (`credentials.durable-custody:
  control-plane`). These are the long-lived keys an adapter needs to create and destroy
  resources. They are never handed to an agent and never written into a record.
- **Agents receive short-lived, resource-scoped credentials**
  (`credentials.agent-credential`, `scope: resource`). Such a credential reaches one
  resource and nothing else. Its `lifetime` MUST NOT exceed the resource's TTL: a credential
  that outlives its resource is a credential to whatever reuses the name.
- **A claim rotates or revokes the agent credential, and a release revokes it.** Ownership
  changing hands means the pre-claim credential must stop working. A resource being removed
  means no credential to it may survive. Each rotation or revocation is recorded in
  `credentials.rotations` as `{after: <effect id>, action: rotated | revoked, at}`. A
  succeeded claim without a recorded rotation or revocation fails validation. So does a
  release without a recorded revocation.

## Receipts

Every effect returns a **runtime receipt**: the adapter's typed statement of what it
observed. A receipt has three fields:

- `adapter-ref`: the adapter's own name for the resource.
- `observed-at`: when the adapter looked.
- `observed-state`: `present` or `absent`.

The effect id ties the receipt to one effect, and `owner.run` ties the effect to one run.
Together they make every receipt addressable as run, effect, receipt.

A runtime receipt is an evidence kind in its own right. An evidence bundle carries runtime
receipts beside command, review and witness evidence (see
[`evidence-bundle.md`](evidence-bundle.md)).

The rules for receipts:

- An effect recorded `succeeded` MUST carry a receipt. An acknowledgment with no receipt is
  only the adapter saying "OK". It is not a statement about the resource, and the record
  cannot reason from it.
- An effect recorded `failed` MAY carry a receipt. One that does tells a reviewer what the
  failure left behind.
- An effect recorded `unknown` carries none. That is what `unknown` means.

## Recovery and reconciliation

The failure this contract exists for is the **lost success acknowledgment**:

1. The control plane sends `create`.
2. The adapter creates the resource.
3. The answer never arrives: the caller crashed, the connection dropped, the run was
   pre-empted.

Retry blindly, and the run creates a second resource. Assume failure, and the first one
leaks with nobody accountable for it. Assume success, and the record claims a resource
that may not exist.

The contract's answer, declared as `recovery: reconcile-before-retry`, has four rules:

1. **Record the unknown.** An effect whose answer never arrived is recorded with
   `outcome: unknown`. It is never silently dropped, and never rounded to `succeeded` or
   `failed`.
2. **Reconcile before anything else.** The next effect the run issues on the resource MUST
   be a `reconcile` whose `reconciles` names the unknown effect. That reconcile succeeds
   and carries a receipt reading the resource's real state at the adapter. Any other
   effect issued while an unknown effect is unresolved fails validation, and a retry of the
   unknown effect is the commonest case. A reconcile attempt that itself loses its answer
   leaves the effect unresolved. Retrying the reconcile is the right move.
3. **Act on the receipt, not on the absence of an error.** The reconcile receipt decides
   what happened:
   - `present` after a `create` means the create took effect. It is not repeated.
   - `absent` after a `create` means it did not take effect. A retry with the same
     idempotency key is now safe.
   - `absent` after a `release` means the release is complete.

   **Success is never inferred from absence**, whether that is the absence of an error, of
   a timeout, or of a complaint.
4. **A release is done when a receipt says so.** Even an acknowledged `release` is
   followed by a reconcile that observes the resource `absent`. Until then the record MUST
   NOT say `released`. An acknowledgment says the adapter accepted the request; only the
   reconcile says the resource is gone.

A resumed run therefore always starts the same way: it reads the record, finds any
`unknown` effect, and reconciles it before doing anything else.

This rule is the second of three independent layers between a lost acknowledgment and a
wrong record:

- The typed record makes the gap representable.
- The reconcile-before-retry rule makes it resolved.
- The verifier's independent receipt check makes it reviewed.

## Adapter registry

The validator checks each record against a registry of the adapters the adopter actually
runs, never against the record's own claims. A registry is one YAML document:

```yaml
schema: runtime-adapters-v1
adapters:
  - id: local-mock
    classes: [database-branch, snapshot, eval-corpus, object-prefix, app-backend]
    verbs: [create, inspect, snapshot, restore, claim, release, reconcile]
    max-ttl: 24h        # optional ceiling on an agent-requested TTL
```

Each entry's `verbs` MUST include `create`, `inspect`, `release` and `reconcile`. A record
fails validation in three cases:

- Its adapter is not registered.
- Its class is not registered for that adapter.
- One of its effects uses a verb the adapter does not implement.

A registry that cannot be read, or that is itself invalid, makes every record
`could-not-check`. A registry nobody can trust cannot vouch for anything.

## Validation

```sh
KUBECONFIG=/dev/null statusgen runtime-resource --lint \
  --adapters path/to/adapters.yaml [--as-of 2026-01-01T02:00:00Z] RECORD_OR_DIR...
```

The validator first checks the schema, then the rules below. It reports in three states,
on the same contract as `statusgen conform` and `statusgen patterns`:

- `0`: every record is checked-clean.
- `1`: at least one record is checked-failed.
- `2`: could-not-check, with the reason, or a usage refusal.

`--as-of` fixes the instant against which TTLs are read, so a verdict is reproducible. It
defaults to now.

| Rule | Fails when |
|---|---|
| `runtime-resource-schema-violation` | the record breaks the schema: a mandatory field missing (a missing `ttl` is reported by name), an agent as cleanup owner or ownership principal, no quota, durable credentials outside the control plane, a duplicated effect id |
| `runtime-resource-unknown-adapter` | the adapter is unregistered, or not registered for the class or for a verb used |
| `runtime-resource-ttl-exceeds-bound` | the TTL exceeds the adapter's registered `max-ttl` |
| `runtime-resource-expired` | an unclaimed resource is past its TTL, or a claimed one past its `retain-until`, yet still recorded as live |
| `runtime-resource-claim-without-principal` | a claim is authorized by anything but a `human:`/`org:` principal, a `claimed` record has no `ownership`, or `ownership` is not backed by a succeeded claim by that principal |
| `runtime-resource-unreconciled-effect` | an `unknown` effect has no resolving reconcile, an effect was issued while one was unresolved, or a reconcile names no earlier effect |
| `runtime-resource-unreconciled-release` | a succeeded release has no reconcile observing `absent`, or the record says `released` without one |
| `runtime-resource-receipt-missing` | a succeeded effect carries no receipt |
| `runtime-resource-state-unsupported` | the recorded state is not the one the receipts support |
| `runtime-resource-credential-not-rotated` | a succeeded claim has no recorded rotation or revocation, or a release no recorded revocation |
| `runtime-resource-credential-outlives-ttl` | the agent credential's lifetime exceeds the resource's TTL |

## Implementing an adapter

An adapter conforms when it implements the required verbs with idempotent effects and
returns a receipt from each. Three implementations show the range the contract is designed
for:

- **A local mock.** An in-process map from resource id to state. This is the adapter the
  reference fixtures are written against (`local-mock` in
  `statusgen/testdata/runtime-resource/adapters.yaml`). Every rule above can be exercised
  against it, including lost acknowledgments, which a mock can simulate by dropping its
  answer after applying the effect.
- **Plain Postgres.** A `database-branch` is a database created from a template. `release`
  drops it, and `inspect`/`reconcile` query the catalog for it. A per-resource role whose
  password the control plane rotates is the resource-scoped credential.
- **SQLite.** A `database-branch` or `snapshot` is a copied file. `reconcile` is a stat of
  the path, and the credential is the file permission the control plane grants and
  withdraws.

None of these needs a managed service, an account, or a network. A hosted provider is one
more adapter, not a dependency of the contract.

## What this document does not do

- It does not integrate any provider, and it adds no authentication, billing, or data-copy
  path. Production data copying in particular is out of scope.
- It does not run adapters. The validator reads records the control plane already wrote.
  Producing those records is the adopter's control plane's job.
- It does not add a brief-lifecycle state. A runtime resource is a typed record beside a
  brief's run, as a deploy record is (see [`deploy-model.md`](deploy-model.md)), not a new
  state of the brief.

## Informative Neon mapping

*This section is informative. Nothing in it is required by the contract, and nothing above
depends on it.*

Neon's claimable-project flow is a public example of the lifecycle this contract
generalizes: provision inside a bounded scope, use it, record what happened, then either
let it expire or have a human claim it. The flow maps onto the contract like this:

| Neon concept | Contract term |
|---|---|
| A temporary project provisioned for an agent before any human account exists | a record in `provisioned`, `requested-by: agent:…`, with a `ttl` and `quota` |
| A Neon branch of a project | class `database-branch`; the parent branch's point in time as `input-revision` |
| The temporary project expiring if nobody claims it | `expired`, then `release` and a reconcile observing `absent` |
| A human claiming the project into their account | a `claim` effect by a `human:` principal, and an `ownership` block |
| Pre-claim tokens revoked and database credentials rotated on claim | `credentials.rotations` entries `after:` the claim effect |
| The project's connection string handed to the agent | the resource-scoped `agent-credential`; Neon's account-level API key stays with the control plane as the durable credential |
| Neon's API response for a create or delete | a runtime receipt; when that response is lost, a `reconcile` that reads the project's state back from the Neon API |

A Neon adapter would be registered like any other (`id: neon`, with the classes and verbs it
implements). It would be held to exactly the rules above. Its records would validate with the
same `statusgen runtime-resource --lint`, and nothing in the contract would change to admit
it.
