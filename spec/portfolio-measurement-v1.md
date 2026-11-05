# Portfolio-measurement v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen/portfolio/` (statusgen/24)
**Shape:** `schemas/portfolio-measurement-v1.json`

## 1. Scope

This document specifies the `portfolio-measurement/v1` document: one revision of an
allocation policy and the agent and human measurements reported for its scope and window,
and the read-only ledger and comparison computed from them. It implements the "Allocation
and observations" semantics of the product-intent and portfolio planning amendment
(`docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md`).

The evaluation is advisory. It MUST NOT dispatch work, change a target, apply a limit or
write anything. It ships no default weight, share, tolerance or limit. Live collection,
recurring measurement, enforcement and projections (statusgen/25) are out of scope.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. Document

A document is one JSON object with members `schema` (exactly `portfolio-measurement/v1`),
`policy`, `segments`, `decisions` and optional `reconciliations`. A reader MUST refuse an
unknown member, a member name differing only in case, a member given twice, a missing
required member (an absent list is not an empty one) and trailing data. A reader MUST
refuse another `schema` value by name without evaluating its payload.

Times are UTC instants written `YYYY-MM-DDThh:mm:ssZ`. An interval is half-open
`[start, end)` and its end MUST follow its start.

### 2.1 Policy

| Member | Meaning |
|---|---|
| `id`, `revision` | The policy and the revision in force. |
| `scope` | The repository (`owner/name`) the policy covers. |
| `window` | The measurement interval. |
| `sources` | The sources whose reports this policy covers. A segment, decision or reconciliation from any other source MUST be refused. |
| `weights` | The explicit weight table: `id`, `revision` and `models` (`model`, `milli` in thousandths). A model the table does not list has an unknown weight. |
| `tolerance-basis-points` | The explicit band within which an observed share is aligned with its target (0–10000). |
| `buckets` | Each bucket has an `id`, `members` (repository-qualified REQ references), an `agent-target` and an independent `human-target`. |
| `limits` | OPTIONAL already-authorized ceilings: `bucket`, `measure`, `max`, `authorization`. |

A REQ reference MUST belong to at most one bucket; an ambiguous assignment is refused, never
resolved by order. An agent target MUST name an agent measure and a human target a human
measure (§3); no target MAY name the elapsed-time proxy. Each party's targets MUST NOT sum
past 10000 basis points. Neither party's target is derived from the other's or from
consumption.

### 2.2 Agent segments

A segment carries `invocation` (`caller`, `id`: the runner's stable invocation identity),
its own stable `id` and `revision`, `profile` (`id`, `revision`, the pinned `model`), the
`actual-model`, `source`, an OPTIONAL `subject` (REQ reference), an OPTIONAL measured
`active` interval, an OPTIONAL `elapsed-seconds-proxy` and an OPTIONAL `cost-micros`. An
absent optional value is unknown, never zero; a measured zero is a real reading. The
pinned model MUST NOT stand in for an absent actual model.

### 2.3 Human decisions

A decision carries a stable dedupe `id`, a `unit` — `decision` (counts one; no `quantity`)
or `minute` (a positive `quantity` of declared minutes) — a declared `window`, an OPTIONAL
single `primary-subject` and a `source`.

### 2.4 Reconciliations

A reconciliation names one `invocation` (at most one reconciliation per invocation), the
segment reports it `accept`s (`id`, `revision`, each id at most once, at least one), a
`reason` and a `source`. A reconciliation that accepts a report no segment carries, or
more than one distinct report of one segment, leaves the invocation in conflict (§4).

## 3. Measures

| Party | Measure | Value |
|---|---|---|
| agent | `active-seconds` | Measured active interval inside the window. |
| agent | `weighted-active-milliseconds` | Active seconds times the actual model's weight in thousandths. |
| agent | `cost-micros` | Reported cost. |
| agent | `elapsed-seconds-proxy` | Reported elapsed time; a separately labeled proxy. |
| human | `decisions` | Count of `decision`-unit records. |
| human | `decision-minutes` | Sum of `minute`-unit quantities. |

Agent durations and human figures MUST NOT be summed together. A decision count MUST NOT
join decision minutes. The elapsed-time proxy MUST NOT join active time, and an absent
active interval MUST NOT be filled from it.

## 4. Ledger

The ledger tallies each measure per bucket, for records with no bucket (`unattributed`,
which stays visible) and for the whole scope (`total`). Each tally is a `Quantity`: the
known subtotal, the count of known records and the count of unknown records. An unknown
value adds an unknown record and nothing to the subtotal; it affects only its own measure.
Subtotals, weighted products and interval lengths are exact signed 64-bit integers: a value
whose sum or product would leave that range is unknown in every tally it would join (its
bucket or `unattributed`, and `total`), never wrapped, so `total` stays the sum of the
buckets and `unattributed`. Active seconds are the exact difference of the interval's ends
for every interval the time format admits.

1. **Identity.** A report identical to one already seen is a duplicate: it is listed and
   counted once. Reports of distinct invocations — including a retry — count separately.
2. **Conflict.** Two different reports of one `(invocation, segment id)`, or two counted
   segments of one invocation whose active intervals overlap, are a conflict unless a
   reconciliation for that invocation accepts exactly one report per segment and the
   accepted segments do not overlap. An unreconciled conflicting invocation MUST NOT be
   counted on either side: it adds one unknown record to every agent measure of its bucket
   (or of `unattributed` when its reports disagree on the subject) and is listed once, as a
   conflict. When the invocation is not in conflict, the reports its reconciliation does not
   accept are listed as excluded.
3. **Window.** A segment whose measured active interval lies entirely outside the policy
   window is listed as excluded. One whose interval straddles a window boundary cannot be
   apportioned by the window: it adds an unknown record to every agent measure, cost and the
   elapsed proxy included. A segment with no measured interval is not placed against the
   window: its cost and elapsed proxy count as reported and its active time is unknown. A
   conflicting invocation whose every report has an interval entirely outside the window is
   listed as excluded, not as a conflict. A decision whose declared window lies entirely
   outside the policy window is listed as excluded; one straddling it is unknown in its
   measure; a conflicting decision id whose every report's window lies entirely outside is
   listed as excluded, not as a conflict.
4. **Weights.** Weighted active time is known only when active time is known and the
   actual model is listed in the weight table.
5. **Decisions.** Two different reports of one decision id are a listed conflict and add an
   unknown record to each reported unit's measure.

## 5. Shares and comparison

A bucket's observed share of a measure is its known subtotal over the scope total, as an
exact fraction. It is defined only when the total has no unknown record and is not zero;
otherwise it is undefined with the reason, never 0% and never a share of a partial total.

The comparison reports, per bucket and party, the target, the observed share and a state:
`aligned` when `|observed − target| ≤ tolerance`, `divergent` otherwise, `undefined` when
the share is undefined. Each limit is `exceeded` when the known subtotal is above it (a
partial subtotal suffices), `within` when the measure is complete and not above it, and
`unknown` otherwise. A divergence or an exceeded limit is a report: no target is rewritten,
no catch-up work follows and nothing is dispatched.

## 6. Runner reports

A runner request and terminal observation (`loop-admin-runner/v1`, owned by
`spec/loop-admin-runner-v1.md`) translate to a segment only with a supplied attribution:
the segment identity and revision, profile revision, source, subject and, when measured,
the active interval. The runner contract carries no active interval; a translation without
one leaves active time unknown. The result MUST echo the request's caller, id and authority
generation. A negative runner cost is unknown. This translation reads the runner contract;
it does not restate or widen it. Both documents are decoded strictly against that contract:
an unknown, case-variant, duplicate or absent required member is refused, and a refusal
names what failed, never the document's content.
