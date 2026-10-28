# Verification domain v1

**Contract:** `verification-domain-v1`. Reference implementation: `statusgen/domain`.
Normative shape: [`schemas/verification-domain-v1.json`](../schemas/verification-domain-v1.json).
This is a pure record and assessment contract. It activates no runtime, store, runner,
queue, publisher, acceptance authority or human decision gate.

## Records and identity

The top-level record contains `schema`, `requires`, `work`, `candidate`, optional
`acceptance`, `facts`, `generation` and `progress`. Every field is mandatory to
understand. Unknown schema versions, required capabilities and mandatory state values
are could-not-check; they must not be ignored. The supported capability is `domain@1`.
An empty `requires` list is valid for the base v1 shape. Unknown JSON fields fail schema
validation. A wire adapter MUST validate the schema before decoding to normalized Go
records, then call `Record.Validate`; the library accepts normalized values and supplies
no filesystem, wire parser, environment, forge or process adapter.

| Schema record | Meaning and rules |
|---|---|
| `work-identity` | `brief` is the canonical four-segment `<cell>:<repo>:<stream>:<NN>` ID. `NewWork` expands two- and three-segment references against a fully qualified declaring brief, using `contexthandoff.ResolveQualifiedID`. Slash references, issues and file paths are refused. `instance`, `node` and `attempt` use [workflow-instance-v1 §2](workflow-instance-v1.md#2-identities): `inst-<slug>`, `<instance>.<node>`, `<node-instance>.a<ordinal>`. Their prefixes must agree; ordinals start at one. This contract generates no new instance or attempt identity scheme. |
| `candidate` | `repository` is a normalized owner/name; `base-commit`, `base-tree`, `commit` and `tree` are full lower-case 40- or 64-hex object IDs of the same object format. `manifest-digest` is `sha256:` plus 64 lower-case hex digits. All six fields identify the subject together. Branches and paths are not candidate identities. Different repository, base, commit, tree or manifest means a different candidate. |
| `acceptance-binding` | The accepted definition's positive integer `version` and SHA-256 `digest`, one exact `candidate`, and its fixed `required-facts` list. This is a definition binding, not an acceptance decision. Required fact IDs are unique; `coverage` and `eligibility` cannot be removed. Additional requirements are supplied by the definition's existing policy/authority owners. A binding for A is refused for B, even on the same branch. |
| `fact` | Unique `id` and explicit completeness `state`: present, missing or could-not-check. Present means the adapter supplied the observation, never that its result passed. |
| `fact-set` | `entries` and `observed-at`, a UTC RFC3339 timestamp with whole seconds. An omitted required fact is treated as missing; an empty or partial list cannot shrink the binding's requirements. Unsupported completeness values are could-not-check. |
| `reevaluation-generation` | Positive integer `requested` and `recorded`, independent of workflow assignment generation. Structural validation permits historical unequal values so they can be retained; `CheckCurrent` and `Assess` refuse any unequal generation, including future results. The caller advances requested generation for relevant fact/event changes and passes the current value explicitly. |
| `components` | Separate bind, admit, execute, assess, decide and publish progress, each using `component-state`. These are observations of logical responsibilities, not six services or a scheduler. |
| `hold` | Typed `kind`, responsible `owner`, the `required` fact/event, and `observed-at`. No default owner, retry permission or routing authority is inferred. |
| `progress` | Independent stage, component progress, execution outcome, acceptance disposition, publication receipt state, retirement state, `observed-at` and `holds`. A held stage, acceptance or retirement requires an explicit hold. Complete requires accepted plus confirmed publication; retirement may still be held. |

`ValidateLocations` rejects two supplied file declarations claiming the same qualified
brief ID. Renaming a file preserves its ID. It validates a caller-supplied inventory,
never scans a repository or checks file existence. Alias-to-repository resolution,
instance uniqueness across stores, commit/tree correspondence, snapshot cleanliness,
artifact content verification and accepted-definition authenticity belong to adapters
and their existing semantic owners. Neither a digest nor a valid record proves any of
those observations. Importers must bind their importing cell/repository and authenticated
source before using a record; this package performs no authorization.

The schema's remaining definitions are the closed state vocabularies:
`stage-state`, `component-state`, `execution-state`, `acceptance-state`,
`publication-state`, `retirement-state` and `hold-kind`. Their enums are the normative
spellings. Execution success never becomes acceptance, publication never grants
acceptance, and resource retirement never implies acceptance. Accepted with pending
publication remains distinct from Complete with a retirement hold. This projection is
not a second queue classifier; existing lifecycle, decision and publication owners
provide the observations.

## Assessment seam

`Assess(record, observations)` first validates the record and requires current
reevaluation generation. `Observations` binds the existing evaluator input types to
that exact work, candidate, acceptance version/digest and recorded generation. A
mismatch is refused. Observers supply already normalized, frozen facts; this envelope
is a consistency check, not authentication of those facts.

1. No acceptance binding yields a held assessment with a missing acceptance fact.
2. Every fact required by the binding must be present. Missing or could-not-check
   returns held with the exact completeness state and no misleading evaluator result.
3. Exactly one eligibility subject and one coverage brief must match the qualified
   work's local stream/number. Cross-repository eligibility still uses the evaluator's
   existing registry/sibling semantics; callers supply observations scoped to the
   candidate repository. No new cross-cell resolver is introduced.
4. The adapter calls the real `evaluation.EvaluateEligibility` and
   `evaluation.EvaluateCoverage`. Coverage receives the candidate **tree**, overriding
   any ambient revision in the supplied input. The brief, Verify rows, witnesses,
   patterns and dependency manifests are forwarded unchanged. `Repo` and `Siblings`
   are the existing evaluator ports and must answer from frozen observations for a
   pure run. A missing port retains the evaluator's existing uncertainty semantics.
5. The assessment carries `evaluation.CurrentIdentity()`, the bound subject and
   definition, generation, observation time, and both original evaluator results.
   State is supported only when coverage is released and eligibility is eligible or
   eligible-with-notice. Otherwise it is held. Actual failure, wrong-revision and
   could-not-check remain distinguishable in the evaluator results; completeness
   holds do not overwrite them.

An assessment is evidence support, **not acceptance**. It cannot prove independence,
record review or human approval, make a decision, publish, or advance any field in
Progress. `Assessment.CheckCurrent` checks work, candidate, definition, generation
and evaluator identity against the caller's current record before reuse; it is not a
signature check. Missing context cannot be reconstructed from file non-overlap or
content hashes. Complete applicability, time validity and context invalidation remain
graph-execution/30's scope; v1 supplies no substitute applicability fingerprint.

## Canonicalization and conformance

`NewWork`, `NewCandidate`, `BindAcceptance` and `NewRecord` return records plus typed
validation errors. Exported records remain plain Go values: callers own their lifetime
and must not mutate them concurrently with assessment. Constructors do not freeze a
workspace or acquire a lease. Acceptance is optional only when no definition is known;
accepted progress without a binding is invalid.

`Record.Canonical` validates, then serializes JSON with object keys sorted by byte order,
no insignificant whitespace, standard `encoding/json` escaping and decimal integers.
Array order is preserved; nil required arrays are invalid. Omitted optional acceptance
stays absent. `Digest` hashes those bytes with SHA-256 and prefixes `sha256:`. It is an
unkeyed content identifier and confers no authenticity. No clock is read; observation
timestamps are supplied. Timestamp calendar validity and cross-field consistency are
library checks beyond JSON Schema. Schema and library validation are both required;
JSON Schema cannot compare two arbitrary candidate objects or successive generations.

Refusals expose a stable code plus checked-failed or could-not-check and omit input
values. Invalid shapes, subject mismatch and stale generations are checked-failed;
unsupported schema/capability/state is could-not-check. A well-formed held assessment
is a normal result, not a pass or a malformed record.

`TestDomainSchemaSurface` compares Go fields/required fields and production state sets
to the schema and checks the instance grammar against its existing owner.
`TestDomainSchemaFixtures` validates every fixture in `statusgen/testdata/domain`:
valid and historical records pass the schema, invalid shape records fail it, and
semantic-invalid records pass shape but fail the production cross-field validator.
Every canonical positive output is also checked against the schema. The named behavior
checks and `domain/mutate.py` exercise the real evaluator seam and planted failures.
The schema testing helper is shared with the existing context-handoff tests.

Consumers are the planned candidate workspace (/23), regression adapters (/29), durable
store (/19), runtime/controller and projections. They import these types; they must not
create a second candidate or binding record. No existing evaluator semantics change.
