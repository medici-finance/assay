# Review observation v1

This is the advisory value contract for the desk module's internal review classifier
and planner. The wire schema is [review-observation-v1.json](../schemas/review-observation-v1.json).
It is not a snapshot, an authorization token, a dispatch claim, or a ready verdict.
A consumer must recheck consequential facts at its existing effect boundary.

## API and ownership

`reviewflow.BuildObservation(inputs, policy)` validates a complete set of resolved
facts and computes its rows and depths. `DecodeObservation(bytes)` decodes one closed
JSON object, refusing unknown, missing, null, duplicate and trailing fields.
`PlanObservation(observation, policy)` validates the inputs again, recomputes every
row/depth, rejects contradictory supplied output, and returns advisory proposals plus
the reviewer-dispatch idle verdict. Errors have `InputError.Kind` `schema`, `partial`
or `stale`; no failed read returns a usable empty plan.

The same `ClassificationFacts.ClassifyInput` → `BuildClassifyInput` → `Classify`
path serves deskboard. The same action table, coalescer and plan assembly serve
reviewloop. Legacy `PlanBoard` keeps its seven-minute cadence rule. The new observation
path has no universal TTL: the caller supplies `ObservationPolicy.Now`, inclusive
`Oldest`, and the expected policy revision. The supported evaluator is `reviewflow-v1`.
Unsupported schema/evaluator or mismatched policy revisions are schema errors.

The library reads no environment, files, clocks, subprocesses, credentials or provider
clients. `statusgen/evaluation` remains the owner of eligibility/coverage semantics.
This package is internal to `tools/desk`; a different module cannot import it.
Public serving extraction belongs to forge-neutral/39. The library extraction
introduced no CLI flags. Desk-tools/40 now supplies collection and the direct-reader
CLI handoff below; desk-tools/41 delivers dispatcher preparation and its `--observation`
handoff, while desk-tools/43 owns live caller adoption.

## Scope, identity and population

`inputs.requested` and `inputs.admitted` are nonempty sets of exact
`{provider, host, repository}` identities. Provider and host are canonical lowercase
ASCII identifiers; repository is a lowercase slash-separated path (including nested
namespaces). No URL, port, credential, alias resolution, case folding or cross-forge
fallback is performed here. Collectors must canonicalize at their owner before entry.
Duplicate identities, an unrequested admission, or a change outside admitted scope
are schema errors. A requested repository missing from admission is partial coverage,
not proof of zero work.

Every admitted repository has one population record with source coverage, a revision
identifying the enumerated set, and its uncapped change count. `changes` includes each
admitted change exactly once. Counts must agree with that full population. Missing
coverage, incomplete enumeration, a capped prefix, or a missing change array is
partial. A positively enumerated empty scope is represented by nonempty scope,
complete population records with zero counts, and an explicit empty changes array.

Each change identifies a positive number, full lowercase 40- or 64-hex head object
ID, and full base ID when `baseUsed` is true. Base is empty when unused. No abbreviated
head or unresolved sentinel is accepted. Coalescing includes provider and host as well
as repository, number, exact head and proposed verb. Legacy single-forge keys preserve
their existing rendering. Independent observations on two hosts never collapse.

## Time, provenance and applicability

`collection.start` and `collection.end` delimit collection; end is also the stated
population/depth cutoff. Start must not precede the caller cutoff, end must not precede
start or exceed caller Now. These are observations over an interval, not an atomic
provider transaction; a later provider change can invalidate them immediately.

Population and subject/body/reviews/checks/enrichment each carry `source`, `version`,
`revision`, `observedAt`, `freshness`, and `complete`. Source/version identify the
instrument or provider normalization used; revision identifies the observed input
set, including an empty set. They are provenance claims, not authenticated proof.
The envelope's schema/evaluator versions own the normalized field semantics.
Each observedAt must be no later than collection end and no earlier than the caller's
Oldest. Cached inputs can predate collection start if this explicit policy admits
them. Freshness must be positively `fresh`; `stale`/`unknown` produce stale errors;
`complete: false` produces partial errors. No collector failure can be relabeled an
empty complete input. A fully read but uninterpretable check remains an explicit
check fact and follows the existing CI-UNKNOWN classifier behavior.

`appliesTo` binds all classification facts to head, used base, body revision, review
set revision and check set revision. These must exactly match the identity and input
provenance. Body edits, new reviews and rerun checks can invalidate applicability
without changing head. Each independent lane conclusion carries the same binding;
mismatches are stale errors. A collector must create new revision tokens when those
sets change. Merely repeating an old token cannot prove freshness to an effect gate.

## Resolved facts and lanes

The adapter supplies the existing `ChangeFact`, resolved `ReviewState`, CI policy,
zero-CI result, own-file delta, risk and non-commit-resolution facts. Existing legacy
struct field names inside `change`, `reviews` and checks retain their Go JSON spelling;
the surrounding new envelope uses the explicit lower-camel schema names. Labels and
checks are explicit arrays, including for measured zero. Required false/zero fields
must be present in JSON, not silently defaulted.

Operational owners still resolve authenticated reviewer identity, remove stale
re-anchored grants, select newest checks with `deskkit.LatestRunPerName`, and verify
body-edit exemptions with `deskkit.EvaluateBodyEditReverification`. This library does
not duplicate those owners. Their resolved output is bound to the observed revisions;
contradictory flags, future verdict timestamps and missing full review revisions are
refused. The input format never turns caller-supplied booleans into trusted evidence.

Correctness and security are two explicit independent lane conclusions: approved,
blocked, missing, stale or unknown, each applicable to the input revisions. Neither
is inferred from the other. The security conclusion must agree with the resolved
security fact. The legacy board's aggregate `Approved` deliberately is **not** used
as independent correctness approval: its historical advisory reducer admits security
verdicts into the aggregate. That documented divergence is preserved, rather than
creating a second ready predicate here. Lane output records required/satisfied values
separately; the full conclusion and applicability remain in inputs. A classification
proposal can therefore coexist with an unsatisfied canonical lane. Actual ready gates
must re-read and require their canonical independent verdicts and exemptions.

## Rows, ranks, holds and proposals

All action rows are computed with the existing classifier and action table. Every row
retains identity, applicability, title, note, score, one-based rank, lanes and holds.
Score is the adapter's resolved ranking input, covered by enrichment provenance;
planner order is descending score then qualified change ID, matching legacy plan
ordering within one scope. The observation does not claim to reproduce a collector's
additional display tie-breaks. WAIT, SURFACE and NO-OP rows remain visible and carry
the action table's explanation even when they propose no outward verb.

Depths include total, per-action and per-disposition counts over the same complete
population and collection-end cutoff, before any caller display limit. The planner
recomputes them rather than trusting serialized counts, ordering, lanes or holds.
Idle means measured zero NEEDS-REVIEW and RE-REVIEW within this qualified scope; it
never means every surfaced/waiting obligation is complete. Only review and ready
proposals come from the existing table; no merge proposal or write authority is added.

The JSON schema defines complete successful wire values. Relational constraints
(scope membership, count equality, applicability equality, caller freshness and
recomputed output equality) require `PlanObservation`; JSON-schema validation alone
is insufficient. No cost record is embedded here. Cost accounting remains with the
existing shared instrumentation and its coverage contract.

## Compatibility and qualification

`TestReviewContractFlow` uses independently authored action expectations and the
existing literal command oracle. It covers head advances, body edits, check states,
independent lanes, holds, rank, more than one display page, qualified coalescing and
explicit idle. Production-source overlays prove wrong body applicability and ignored
reviewer depth are detected. `TestReviewBoundary` checks pure execution with credentials
and command lookup unavailable, a closed production import allowlist, schema/Go field
parity, malformed and partial observations, and an external Go-module import refusal.
Its production mutation proves incomplete-as-empty cannot evade the behavioral test.

Legacy command flags, wire payloads, output ordering, error wrapping and idle cadence
are retained. No generated command documentation changes are applicable. The existing
`statusgen regen --readmes` generator owns stream board regions; this change does not
branch-commit generated board state or alter verification/acceptance gates.

## Candidate-plus-priority selection

`PlanCandidates(observation, freshness, selectionPolicy, capacity)` is the review role's
pure selection entrypoint added by desk-tools/39's merged candidate-plus-priority amendment.
It first calls `PlanObservation`: source schema, complete requested/admitted population,
per-input freshness, exact-head/body/review/check applicability and recomputed output remain
mandatory. Classification, lane facts, action legality, coalescing and reviewer-idle semantics
keep their existing owners. This is a Go-library extension; observation-v1 JSON and existing
command behavior remain compatible. Collection/CLI and actual caller migration remain with
/40 and /43; graph-execution/27 later proves the discrete/prose role bindings.

`SelectionPolicy` names the approving owner, opaque approved revision, supported
`review-selection-v1` evaluator, complete priority entries and an explicit unmapped policy.
The caller establishes that the selected revision is approved; these values do not authenticate
an owner or convey authority. Priority subjects are provider/host/repository plus change number;
rank never authenticates a new head. The observed full head and each lane's applicability stay
on the candidate decision. Higher existing-domain scores precede lower scores, with the same
qualified identity tie-break as the observation; the coalescer's key order is not capacity
priority. A nil priority list is unavailable; an explicit empty list is valid. Duplicate or
noncanonical priority subjects, unsupported evaluator, absent owner/revision or negative
capacity are schema errors. Policy changes need not alter the source's classification-policy
revision or recollect unchanged applicable observations.

Every observed candidate appears exactly once, including deferred, held and watch rows.
Unmapped candidates explicitly retain the existing observed-score default (`observed-score`)
or are held (`hold`); they never disappear in an inner join. Policy entries with no candidate
are returned as `NotObserved` gaps, never fabricated work or proof that an out-of-scope
repository is empty. Their reason distinguishes an absent subject in admitted scope from
a priority subject outside that scope. Both remain visible; neither implies collection
coverage beyond the admitted repositories. Domain holds/non-operation dispositions are
independent of score. Capacity is a nonnegative aggregate limit on advisory proposals, not a
reviewer-lane grant or permission to act. Zero capacity preserves every row and source depth;
it does not make backlog idle. An otherwise idle source with not-observed priority entries
becomes could-not-check until those gaps are reconciled. Selected decisions preserve the proposed verb, qualified exact
head, independent lanes and existing holds. Downstream adapters retain their admission,
claims, custody and final mutable-state checks.

The plan binds role, evaluator, policy owner/revision, source schema/evaluator/policy,
collection interval, requested/admitted coverage, explicit current time/cutoff and capacity.
Selection order is separate from the immutable source row's stored score/rank. It performs
zero provider, filesystem, process, credential or clock reads, including on policy/slot-only
reselection. For R admitted repositories, P candidates and Q priorities, time is
O(R + P log P + Q log Q) and memory O(R + P + Q), including legacy validation/coalescing and deterministic unobserved-entry ordering.
No cross-window cache, universal score, TTL, service, priority register or telemetry is added.

`TestReviewPriorityJoin` checks literal expected selections that disagree with identity order,
source-score order and a reversed-comparator production mutation. An equal-priority,
cross-host case with capacity one rejects removed/reversed identity tie-breaks even when
the source/input ordering disagrees with the selected order. It covers changed priorities
on the same source, zero capacity, 111 candidates, second-root-only work, host-qualified joins,
unmapped and unobserved entries, high-ranked holds/watch rows, shuffled inputs, stale/partial
and contradictory observations, repeated pure execution and the closed import boundary.

## Direct-reader command adapters

Desk-tools/40 delivers `deskboard review-queue --out FILE` and
`reviewloop plan --snapshot FILE --selection JSON --now RFC3339` over this contract.
The [operator contract](../docs/deskboard.md) specifies scope/source-policy binding,
explicit freshness/capacity and compatibility. Both planning adapters reuse the
shared library; no independent sorter or live consumer migration is introduced.
