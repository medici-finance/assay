# Workflow-instance v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen instance validate` (graph-execution/09)
**Builds on:** [workflow-pattern v1.0-draft](workflow-pattern-v1.md), which this document
does not change.

## 1. Scope

A workflow pattern ([workflow-pattern-v1.md](workflow-pattern-v1.md)) states the shape of a
kind of work. It is not yet an executable contract for one particular change. This document
specifies `workflow-instance-v1`: the record that binds ONE reviewed pattern to ONE canonical
work item in ONE Cell, with every input, acceptance, policy and dependency reference pinned,
so a retry cannot silently pick up new inputs or a different permission set.

This document also specifies the **role packet** — an exported, role-specific view of one
node's latest attempt — and the **handoff** a role returns for that attempt.

Out of scope here: persisting instances and the real coverage-binding adapter
(graph-execution/19), prompt assembly and dispatch from a packet (graph-execution/14), usage
settlement against a budget (graph-execution/16), and qualified-identity / candidate records
(graph-execution/28), which key on the identifiers this document defines and add no second
scheme. No semantic dependency inference and no provider-specific model client is specified.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

- **Cell** — the administrative unit an instance belongs to; its roster binds the actors.
- **Contract** — the immutable part of an instance (§4.1).
- **Records** — the append-only attempts and decisions (§4.2).
- **Projection** — the generation-keyed assignment and execution state (§4.3).

## 2. Identities

The canonical work identifier `<stream>/<NN>` remains the work's identity. An instance adds
three identities, and this document is their single owner:

| Identity | Form | Example |
|---|---|---|
| Instance | `inst-<slug>` | `inst-example-07-a` |
| Node instance | `<instance>.<pattern node id>` | `inst-example-07-a.implement` |
| Attempt | `<node instance>.a<ordinal>` | `inst-example-07-a.implement.a1` |

A conforming document MUST use exactly these forms. Attempt ordinals MUST start at 1 and
increase by one. Later records (evidence, handoffs, qualified-identity and candidate
records) MUST key on these identifiers rather than minting another scheme.

## 3. Document shape

A workflow-instance-v1 document is one YAML or JSON document. `schemas/workflow-instance-v1.json`
is the normative shape; it is closed (`additionalProperties: false`) at every level except
`extensions`.

| Key | Layer | Requirement |
|---|---|---|
| `schema` | — | REQUIRED. The literal `workflow-instance-v1`. |
| `requires` | contract | OPTIONAL. Reader capabilities the document depends on (`<name>@<N>`); see §6. |
| `extensions` | contract | OPTIONAL. Fields a capability in `requires` defines. |
| `instance` | contract | REQUIRED. `id`; optional `supersedes {id, contract-digest}`. |
| `work` | contract | REQUIRED. `id` (canonical `<stream>/<NN>`), `cell`, `intent`. |
| `pattern` | contract | REQUIRED. `name`, `version`, `digest` (sha256 of the pattern file). |
| `acceptance` | contract | REQUIRED. `version` (the brief's `version:`), `ref` (`<path>@<40-hex commit>`), `rows` (the acceptance scope), optional `exclusions`. |
| `policy` | contract | REQUIRED. `revision` (`<path>@<40-hex>`), `control-profile` reference. |
| `budget` | contract | REQUIRED. `ref` — a reservation reference, never an amount of authority. |
| `inputs` | contract | REQUIRED. Immutable input references: `name`, `ref`, `digest`, `origin`, `trust`. |
| `dependencies` | contract | REQUIRED. The dependency manifest: `ref`, `kind` (`source`, `api`, `schema`, `build`, `policy`, `environment`), `fingerprint`, `supports` (acceptance rows it supports). A `ref` appears once; a repeated ref is refused (`instance-identity-mismatch`). |
| `nodes` | contract + records | REQUIRED. One node instance per pattern node: `id`, `node`, `role`, `actor {binding, cell, capabilities}`, `effects`, `evidence`; `attempts` are records. |
| `decisions` | records | OPTIONAL. Human decisions as references (§8). |
| `assignment` | projection | REQUIRED. `backend`, `generation` (>= 1), `runner-profile` reference. |
| `execution-state` | projection | REQUIRED. `projection-version`, `generation`, `state`. |
| `digest` | — | OPTIONAL. The export seal (§7). |

## 4. Three layers, three change rules

### 4.1 Contract — immutable

The **contract digest** is the sha256 of the canonical JSON of the document with `digest`,
`assignment`, `execution-state`, `decisions` and every node's `attempts` removed.

1. Two documents with the same `instance.id` MUST have the same contract digest. A changed
   binding — a different policy revision, acceptance version, input, dependency pin, actor
   or pattern version — MUST be a NEW instance whose `instance.supersedes` names the prior
   instance's `id` and contract digest (`instance-rewrite-in-place`).
2. A superseding instance MUST keep the work identity and Cell, MUST NOT name itself, and
   MUST NOT carry the prior instance's decisions as its own; they stay in the prior
   instance's history (`instance-supersession-broken`).

### 4.2 Records — append-only

Between two revisions of one instance, every prior attempt and decision MUST be present and
unchanged; new ones MAY be appended (`instance-record-rewritten`).

### 4.3 Projection — versioned by generation

The assignment and execution state are a projection of where the work is running, keyed by
`generation`.

1. `execution-state.generation` MUST equal `assignment.generation` (`instance-stale-generation`).
2. A change of backend or runner profile MUST advance the generation. The same generation
   with a different backend or runner profile is a substitution, and a generation that goes
   backwards is a substitution (`instance-assignment-substituted`).
3. A projection change never changes the contract digest.

## 5. Rules against the pattern, the actor and the repository

1. **Binding.** The named pattern MUST resolve under `spec/workflow-patterns/` and pass the
   pattern lint; its version and file digest MUST equal the pinned ones
   (`instance-pattern-unbound`, `instance-pattern-invalid`, `instance-pattern-drift`). A
   declared instance whose binding is missing MUST be refused; it MUST NOT degrade to
   no-pattern behaviour.
2. **Node set.** Every pattern node MUST be instantiated exactly once, with the same role
   and the same evidence claims (`instance-node-mismatch`).
3. **Effects ⊆ pattern ∩ actor.** Every effect MUST be declared by the pattern node
   (`instance-effect-not-declared`) AND be among the actor's resolved capabilities
   (`instance-effect-exceeds-actor`).
4. **No minting.** An actor's recorded capabilities MUST be a subset of what its role holds
   in the pattern effect-permission table ([workflow-pattern-v1.md §7](workflow-pattern-v1.md))
   (`instance-capability-minted`). An instance records capability a roster binding already
   holds; it never grants one, and an instantiator MUST refuse rather than widen an actor to
   fit a pattern.
5. **One Cell.** Every actor's `cell` MUST equal `work.cell` (`instance-cross-cell`).
6. **Acceptance currency.** `acceptance.version` MUST equal the work item's current brief
   `version:` (`instance-acceptance-stale`); an unresolvable work item is could-not-check.
7. **Truncation.** An artifact whose `included-bytes` is less than its `bytes` MUST carry a
   coverage `index` and SHOULD carry a matching omission; an excerpt without an index is
   silent truncation (`instance-silent-truncation`). Raw artifacts remain addressable by
   reference and digest.
8. **Eligibility is reported, not folded.** A validator MAY report the work item's
   eligibility verdict beside its result; that verdict MUST NOT change the instance verdict.

## 6. Reader capabilities and old-reader refusal

A document that depends on something an older reader would misread lists it in `requires`.
A reader MUST refuse — as could-not-check, never by ignoring the field — a document whose
`requires` names a capability it does not implement, or whose
`execution-state.projection-version` it does not understand (`instance-unsupported-capability`,
`instance-unsupported-projection`). An unknown top-level field with no `requires` entry is a
schema violation. This reference reader implements `packet-view@1`, `assignment@1` and
`execution-state@1`.

## 7. Export and import

Export writes the canonical document (an explicit, field-by-field copy; nothing recomputed
or defaulted) with `digest` set to the sha256 seal of the canonical document without the seal.
The seal is an UNKEYED content digest: it detects accidental alteration in transit or at
rest, and nothing more. Anyone holding the document can edit it and recompute a seal that
verifies, so the seal authenticates neither the document's origin nor any field in it — not
the Cell, not trust provenance, not the contract. Authenticity comes from the store or
transport the document crosses (an authenticated store, a signed transport — graph-execution/19's
scope), never from the seal.

Import is the lower boundary every consumer crosses, and it MUST:

1. apply §6 refusal, the schema and the credential scan (§9);
2. require the seal and recompute it — a document altered after export WITHOUT the seal
   being recomputed is refused (`instance-seal-mismatch`); an alteration that recomputes the
   seal is not detected here;
3. refuse an instance whose `work.cell` is not the importing Cell (`instance-cross-cell`),
   independently of rule §5.5. This compares the Cell the document states with the importing
   Cell; it does not prove the document came from that Cell;
4. re-apply every rule the document alone can decide.

An export → import round trip MUST return the same identity, contract digest, acceptance,
assignment and records, and re-exporting the import MUST be byte-identical.

## 8. Human decisions are external authenticated acts

A human decision is recorded as `decisions[]`: `id` (`dec-<slug>`), the node instance it
concerns, an `outcome`, and `act {kind: external-authenticated-act, ref}` — a reference to
an act the human performed in their own authenticated identity on the forge or another
system of record. It is NOT a model role, it is NOT a node actor, and it carries no token:
no workflow component acts as a human, and a relay of a decision by a bot identity is not
the decision.

## 9. Credentials and trust provenance

1. A document, packet or handoff MUST NOT carry credential material. A field named like a
   credential, or a value shaped like a token, key, authorization header or URL with
   embedded credentials, is refused (`instance-credential-material`); the finding names the
   path, never the value. A field NAME shaped like credential material is refused the same
   way, and its path segment is reported as `[redacted]`. Carry an authorized reference to
   the credential instead.
2. No finding, refusal or error — schema findings included, which otherwise quote the
   offending value — carries a credential-shaped value or field name: every message a
   decode, an import, a packet projection or a handoff check produces is redacted before it
   leaves, and the packet and handoff boundaries refuse credential material before any
   message quotes a field. A YAML/JSON parse failure is reported by position only; the
   parser's own text quotes the offending scalar and is never forwarded.
3. Only the trust gate decides an input's disposition. An input whose `trust.source` is
   `reporter` MUST have disposition `unresolved`; a reporter-supplied label is kept in
   `reported-label` and is never a disposition (`instance-trust-not-authoritative`). Import
   re-applies this rule, so a document that promotes a reporter-sourced input is refused
   whether or not it was re-sealed. The rule reads `trust.source` as the document states it:
   a document whose `trust.source` itself was rewritten is not detected by this rule or by
   the seal (§7) — provenance is authenticated by the store or transport, not the document.

## 10. The role packet and the handoff

### 10.1 The packet is a view

A role packet is projected from a canonical instance for one node instance's latest attempt.
It is NOT a new run record: it holds no identifier of its own, and projecting the same
instance with the same observations MUST yield the same packet. It carries:

- the instance id, contract digest, work id, Cell, node instance, pattern node, attempt and role;
- the intent, the acceptance version, reference, rows and exclusions, and the policy revision;
- the inputs, with every input not decided `trusted` by the trust gate listed as unresolved
  (reporter labels named as labels);
- the dependency manifest as observed now — an unobserved dependency has the explicit
  fingerprint `unknown`, never an omitted entry;
- coverage: rows supported by at least one dependency, and rows with no support listed as
  `unknown` explicitly;
- the node's required evidence, each `present`, `omitted` or `missing`;
- the node's artifacts by reference and digest, and the attempt's omissions;
- completed and remaining rows, and the delta since the prior attempt (or since
  instantiation): changed dependencies and the acceptance rows they support. An affected row
  is never reported completed, and the next safe action becomes revalidation of those rows;
- references — not authority — to the backend, generation, runner profile, control profile
  and budget reservation.

A packet MUST NOT be projected from an instance that fails §4–§9, and MUST NOT carry a
silently truncated artifact or credential material.

### 10.2 The handoff answers the packet

A handoff is bound mechanically to the packet it answers: the same instance id, contract
digest, Cell, work id, node instance and attempt, the same acceptance version and reference,
the exclusions, the packet's dependency fingerprints, and the omissions and unresolved inputs
carried forward. Its scope MUST be within the acceptance rows.

Acceptance re-checks the binding at the moment of acceptance, without relying on how the
handoff was built, and MUST refuse a handoff that: answers another instance, contract, Cell,
attempt or acceptance revision; has a scope row outside the acceptance rows; does not carry
the acceptance exclusions, the attempt's omissions or the unresolved inputs forward; does
not bind every dependency of the instance's manifest exactly once at the fingerprint
observed now (a moved or unobserved dependency, or one trimmed from the handoff's list or
the whole list emptied, is refused — the manifest, not the handoff's list, decides what is
checked); lacks evidence for a mandatory claim backed by an artifact the handoff returns;
or carries credential material. A packet is current under the same manifest-driven
dependency test: a packet projected before a dependency moved is no longer current and
cannot stand in for the re-projected one. A packet's required evidence reads `present` only
when it cites an artifact the attempt holds.

## 11. Conformance

A conforming implementation MUST:

1. validate documents against `schemas/workflow-instance-v1.json` and every MUST in §2–§9,
   reporting three states — checked-clean, checked-failed (a rule finding) and
   could-not-check (a refusal or an unreadable input) — and failing closed;
2. refuse an in-place contract rewrite, a rewritten record and a same-generation
   substitution when given the prior document;
3. implement export and import per §7 and the packet and handoff per §10.

The reference implementation is `statusgen instance validate --file PATH [--root DIR]
[--prior PATH] [--cell CELL] [--json]`, exiting 0 (checked-clean), 1 (checked-failed) or
2 (could-not-check). Fixtures: `statusgen/testdata/instances/`.

## 12. Known limitations

- The reference validator reads acceptance currency and eligibility from a local
  `docs/streams/` tree; persisted instances and the real coverage-binding adapter are
  graph-execution/19's scope.
- Dependency fingerprints are supplied by the caller; this version infers no semantic
  dependency and computes no fingerprint itself.
- The credential scan is pattern-based and catches common formats; it is one layer. The
  roster binding remains what authorizes a write, and no instance or packet ever carries
  the credential that would. Message redaction removes what the scan recognises; a value it
  does not recognise as credential-shaped is quoted like any other value.
- The export seal is an unkeyed digest (§7). It does not authenticate a document: a party
  able to edit an exported document can re-seal it, and import then checks only what the
  document states — its Cell, its trust provenance, its contract. Authenticating where a
  document came from is the authenticated store and transport's job (graph-execution/19).
- A handoff's dependency fingerprints are its own statement of what it was bound to; they
  are not tied to a packet digest. Acceptance refuses a list that does not match the
  manifest at the current observations, but a handoff rewritten to the current observations
  passes — binding a handoff to the exact packet it answers is likewise the authenticated
  transport's job.
