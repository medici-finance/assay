# verification-handoff-v1 — accountable routing observations

Status: source library contract; operational writers and activation remain held.
Canonical owner: `deskcore/issueflow`. It accepts caller-supplied values in-process;
no process protocol, provider read, clock, credential, write or new lifecycle store.
The internal desk-tools facade preserves supported callers.

## Authority and identity

Forge issues/PRs and existing dispatch claims retain assignment authority. Intake
exits and committed projections are evidence. An investigated comment, label,
mechanical placeholder, issue closure, repair merge or local log grants neither
assignment nor acceptance. `AssessHandoff` returns an advisory observation only.
It cannot dispatch, claim, resolve, grant permissions or mark a brief verified/done.

The adapter supplies the existing canonical case/obligation ID, including
`deskkit.RepairObligationID` where applicable, unchanged. The library creates no
alternate case identity. One brief can have multiple cases/receipts/held row sets.
Rows compare as a set; duplicate/nonpositive row numbers are incomplete. Existing
wake/outcome source bindings, independent actor checks and exact repaired-revision
acceptance remain with their current owners. A merge only wakes reverification.

## Companion shape

`Handoff` uses exactly these top-level JSON keys. Missing information is preserved
as unknown by assessment; a successful structural decode is never completeness.

| Key | Meaning |
|---|---|
| `schema` | Exactly `verification-handoff-v1`. |
| `source` | Object with `repo` (owner/repo), `brief` (supported stream/NN), `source_version`, `source_revision`, `receipt_id`, `case_id`, `rows` (held Verify row numbers), `blocker_ref` (canonical #N or owner/repo#N). |
| `disposition` | One of the dispositions below. |
| `deciding_role` | Existing intake-exit closed role vocabulary; role text is not identity proof. |
| `successor` | A supported stream/NN or canonical issue/PR reference; empty for external work. |
| `external_role` | human, operator, brief-author or verifier for external work; otherwise empty. |
| `required_action` | Exact external prerequisite action; otherwise empty. |
| `observed_at` | RFC3339 observation time. |
| `fresh_until` | RFC3339 adapter-established freshness boundary, inclusive. No default TTL. |

Adapters normalize existing URL references through their existing typed forge
reference owner before mapping to canonical refs. No milestone or b-prefixed
operational ID is introduced. The source version/revision is the original
acceptance source, not a substitute repair revision.

`DecodeHandoff` uses the existing strict domain decoder: unknown, duplicate and
case-variant keys, trailing values and unsupported schemas are refused. Readers
retain the unavailable/unsupported observation; refusal is never an empty queue.

## Dispositions and assessment

| Disposition | Accountable observation required |
|---|---|
| `needs-scoping` | A typed successor for scoping/brief-authoring work. |
| `repair` | A typed successor for directly implementable repair. |
| `external-prerequisite` | An external owner role and exact action. No worker dispatch inferred. |
| `duplicate` | A typed existing owner/successor, not a free-text duplicate claim. |
| `unresolved` or absent | Explicit unknown; retain the blocker. |

`HandoffFacts` carries a complete admitted source/successor/disposition read,
the expected original source binding, caller time, and existing human/activation
holds. Assessment requires all source fields and held rows to match, complete
facts and current observation freshness. Missing source, stale/future observation,
unreadable input, unknown deciding role, absent successor/external action or
unsupported disposition returns `unknown`. Existing human/activation holds return
`held`. A valid accountable observation returns `observed`, never ready or accepted.
Trust, label history, admission, eligibility, claims, custody and independent
acceptance must still be checked separately at effect boundaries.

## Version negotiation and rollout

The Go API and companion schema are separate from intake-exit-v1. Both existing
intake writers retain their exact keys and five exits. `SupportsHandoff` advertises
only this exact companion schema. Operational writers must establish consumer
support before sending required fields; no writer is activated here. An older
reader/absent record is unknown, with original evidence retained; do not retrofit
legacy exits as dispositions. Future incompatible versions require explicit
negotiation and reader-first deployment. A local log is never fleet-wide authority.

The source module dependency uses the repository's existing deskcore source.
`GOWORK=off` consumer builds and a standalone local module-proxy fixture qualify
packaging; the placeholder in-repository requirement is not a published version
or installation receipt. Release, consumer pins, operational sink activation and
human approval remain separate. Rollback restores previous consumer pins and
leaves additive observations intact; it never erases held work or evidence.

## Qualification

The two brief Verify entrypoints exercise the public owner, actual scanner, full/
intake/work-ready/inbox readers, worker inbox ordering, existing trust/history
holds, strict version/source/freshness assessment, real consumer builds and an
external consumer without a relative replacement. The module's existing
architecture checks cover the new pure package. The committed mutation manifest
plants addressed selection bypasses, wrong source/freshness/completeness behavior,
and an effectful second caller; these must fail their actual checks. No production
endpoint, live consumer installation or acceptance is certified by fixtures.
