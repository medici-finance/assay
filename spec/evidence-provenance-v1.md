# Evidence provenance v1

**Contract:** `evidence-provenance-v1`. Reference implementation: `statusgen/domain`
(`provenance.go`). Normative shape:
[`schemas/evidence-provenance-v1.json`](../schemas/evidence-provenance-v1.json).
It extends the [verification domain](verification-domain-v1.md) records and has the
same semantic owner, `S-verification-domain`.

This is a pure record and duty-applicability contract. It authenticates no actor,
verifies no signature, mints no role, key, grant or credential, waives no duty and
activates no consumer. No admission, publication or export path reads it yet.
The maintainer selected corroborated provenance (option 1) in
[decision #4800](https://github.com/medici-finance/assay/issues/4800#issuecomment-6481737472).
Admission and publication consumers (/14 and /25) must require trusted corroboration
when they implement this contract. This records the selected direction, not activation
or qualification of a deployed consumer. The delivered design is transcribed in
[DR-evidence-provenance](../docs/streams/decisions/DR-evidence-provenance.md);
that record still needs its own identifier-bound human ratification.

## Why

Different session keys, provider Apps or role identities do not prove that two
duties were performed independently. One person can hold many keys and direct
many agents. This record names who is accountable for an evidence item and who
delegated to them, so independence is compared on accountable principals rather
than on credentials. It also keeps each kind of assurance separate, so a
declared label cannot stand in for proof that was never checked.

## Record

Every field is mandatory. Unknown JSON fields fail schema validation, including
any `waiver` or self-declared `independent` field. Unknown schema versions,
required capabilities, actor kinds, proof classes and corroboration states are
could-not-check; they must not be ignored. The supported capability is
`provenance@1`. A wire adapter validates the schema before decoding to normalized
Go records, then calls `Provenance.Validate`.

| Schema record | Meaning and rules |
|---|---|
| `work-identity` | The same definition as verification-domain-v1. `node` and `attempt` name the node where this duty acted, so they may differ from the assessed record's node; `brief` and `instance` must match it. |
| `candidate` | The same definition as verification-domain-v1. It must equal the assessed record's candidate exactly. |
| `contract-binding` | The accepted definition's positive `version` and SHA-256 `digest` that the evidence was produced for. It must equal the assessed record's acceptance binding. |
| `principal` | An opaque, pseudonymous handle: a lower-case kind prefix, a colon and an id (`principal:p-17`, `app:example-worker`, `key:k-3`). Names, email addresses and provider account numbers are not needed and do not fit the grammar. Mapping a handle to a person stays with the existing identity owner and access policy. |
| `actor-kind` | `human`, `provider-app`, `agent-session` or `service`. A provider App or agent session acting for a person is still that kind; it is never recorded or read as a human act. |
| `actor` | An `id` and `kind` as normalized by a trusted adapter. The record carries three: `requester` (the authenticated requester), `represented` (the actor the request was made on behalf of) and `executor` (the provider identity that executed). |
| `delegation` | The delegating-principal lineage of the `responsible` principal, outermost first. Entries are unique and never repeat `responsible`. An empty list means the responsible principal acted on its own authority. |
| `capture` | The `actor` that captured the evidence, the `source` kind it was captured from, and `observed-at`, a UTC RFC3339 timestamp with whole seconds. |
| `environment` | The `manifest-digest` of the execution environment. It makes evidence attributable and comparable. It does not prove the run is reproducible. |
| `proof-class` | The declared class, ordered weakest first: `self-report`, `authenticated`, `attested`, `signed`, `custody-qualified`. The declared class is a claim only. |
| `corroboration-state` | `corroborated`, `claimed`, `missing`, `withheld` or `could-not-check`. Only `corroborated` counts. `withheld` means an access policy redacted the fact; it counts as uncorroborated. |
| `corroboration` | One state per assurance dimension, kept separate: `attribution` (authenticated actor), `lineage` (responsible principal and delegation), `execution` (attested execution), `signature` (verified signature) and `custody` (custody or isolation evidence). Duty independence is not a dimension: it is computed, never declared. |
| `duty-state` | The applicability result vocabulary: `satisfied`, `held` or `unsatisfied`, ordered by severity. It is not part of the record. |

The remaining top-level fields are:

- `responsible`: the principal accountable for the act.
- `sponsor`: the principal accountable for the work as a whole. A sponsor is not
  automatically anyone's delegator and is not part of the lineage.
- `issuer` and `key`: the issuing infrastructure and the credential key id. They
  are recorded for audit and never take part in an independence comparison.
- `duty`: the duty this evidence performs, such as `author` or `review`.
- `evidence`: the SHA-256 digest of the evidence item described.
- `declared-proof`: a `proof-class`.

`Validate` refuses a shape error as checked-failed. Shape errors include a
`responsible` principal inside its own delegation, a duplicate lineage entry and a
timestamp that is not a real calendar instant.

## Effective proof

`EffectiveProof` climbs the ladder one rung per corroborated dimension:
`attribution` gives `authenticated`, then `execution` gives `attested`,
`signature` gives `signed`, and `custody` gives `custody-qualified`. It stops at
the first uncorroborated dimension. The declared class never raises it. A
declared class above the effective class holds with `declared-proof-unsupported`,
so a forged or merely claimed signature is visible rather than silently accepted.

Nothing in this contract qualifies hardware-backed human approval, threshold
signing or legal non-repudiation. A `custody-qualified` result means only that a
trusted adapter reported custody corroborated. That claim needs separately
qualified evidence before any managed use.

## Trusted selection

Provenance is accepted only from a trusted composition point, never from model
text or a caller's payload. `Selector` is that port. `Corroborate(claim, selector)`
looks up the item by work, candidate, duty and evidence digest and returns the
selector's record. A claim that differs in any field, such as a caller-chosen
principal, a shortened delegation or a raised proof label, is refused with
`provenance-not-selected`. An item the selector does not know is could-not-check.
`EvaluateDuty` takes the selector as an argument and calls `Corroborate` on every
claim itself before reading any field of it, so the duty check never reads a value
the selector did not supply. The selector is the trust root. This package cannot
tell a trusted selector from one that echoes its input, so composing it from trusted
adapters, never from model text, stays the consumer's obligation. This package
installs no selector. Production adapters belong to the existing identity and
evidence owners.

This package does not relate `requester` or `represented` to the lineage. When an
actor acts on behalf of a principal, the trusted adapter that builds the record must
put that principal in `responsible` or `delegation`. A principal that appears only
as the represented actor is not compared for independence.

## Duty applicability

`EvaluateDuty(record, assessment, policy, selector, claims)` applies one
caller-selected existing duty policy. It first requires
`assessment.CheckCurrent(record)` and a selector; without one the call is
could-not-check (`provenance-uncorroborated`). Every claim then goes through
`Corroborate`, so an unselected claim is refused (`provenance-not-selected`) and an
unknown item is could-not-check. Every selected item must bind to the record's
brief, instance, exact candidate and acceptance binding, or the call is refused with
`provenance-subject`. Two items for one duty are refused as `duty-ambiguous`.
The function does not read the assessment's support state: evidence support and
duty provenance are separate claims.

A `DutyPolicy` names the `duty`, the duties it must be independent `against`, whether
it requires a `human-act`, and the minimum proof class. This package defines no
policy and accepts no exception input.

| Condition | Result | Reason |
|---|---|---|
| No evidence for the duty or a duty it is compared against | held | `duty-evidence-missing` |
| Effective proof below the policy minimum | held | `proof-uncorroborated` |
| Declared class above the effective class | held | `declared-proof-unsupported` |
| Human act required, but the executor is not a human, or the requester or represented actor differs from it | unsatisfied | `not-human-act` |
| Human act required and shaped correctly, but attribution is not corroborated | held | `attribution-uncorroborated` |
| Either side's lineage is not corroborated | held | `lineage-uncorroborated` |
| The two lineages share any principal | unsatisfied | `principal-overlap` |

The lineage is the responsible principal plus its delegation. Two items with
different keys, Apps or issuers but one shared principal overlap. Two items from
distinct principals that share an issuer do not. A sponsor shared by both sides
does not create overlap. When the only human authors a change and also sits in
the reviewer agents' delegation, the review is not independent and the duty is
unsatisfied. Results only move toward more severe: an overlap stays unsatisfied
even if other facts are also held.

No result is waived here. An exception to a required independent duty can only
be an explicitly applicable, retained human decision recorded by its existing
owner. This contract creates none and approves none, and a waiver label in a
record fails the schema.

## Consumers

The planned consumers read corroborated records with their own existing effect
and role controls. None is wired by this contract.

- Graph-execution/14: admission duty checks.
- Graph-execution/25: the publication review bridge's independent verdict binding.
- Graph-execution/15: control exports report the effective proof class and each
  omitted or withheld dimension. They make no certification claim.
