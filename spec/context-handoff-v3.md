# Context handoff v3

`context-handoff-v3` is the [v2 contract](context-handoff-v2.md) with one required addition:
a `product` section that records product-target binding beside the architecture target.
Its schema is [context-handoff-v3.json](../schemas/context-handoff-v3.json). Every v2 shape,
refusal, approval, coverage, history and active-work rule is unchanged, and `withdrawn`
remains a lifecycle. v1 and v2 refuse a `product` key as an unknown field; v3 requires it.
Unknown versions fail closed, and canonical digests include the schema identifier, so every
existing v1 and v2 receipt keeps its digest.

The resolver emits v3 only when the snapshot carries a `product` input. A snapshot without one
still gives an architecture-only receipt, v1, or v2 when an impact is `withdrawn`, whose
prerequisites say product alignment is unchecked.

## 1. Two targets, two inputs

The architecture target (`$.target`) and the product target are different inputs. The
architecture baseline comes only from the snapshot's `target`. Product targets come only from the
snapshot's `product.targets` and from each brief's own `outcome` and `outcome-revision`
frontmatter ([brief-v1](brief-v1.md) §3.2). A change to one never moves the other: a new
approved product revision does not change `$.target`, an impact, an impact disposition or the
next act.

## 2. The product section

| Field | Meaning |
|---|---|
| `targets[]` | Each product target revision the snapshot supplies, in snapshot order: `outcome` (a qualified requirement reference `<cell>:<repo>:REQ-<slug>`), `revision`, `approval` (`approved`, `unapproved` or `unavailable`) and `reference`. The last approved revision of an outcome is its **current** target. |
| `bindings[]` | One per subject (the declaring brief, every impact and every subject of a declared relationship): `state` (`bound`, `unbound`, `none` or `could-not-check`), `outcome`, `declared` (the brief's `outcome-revision`), `selected` (the revision the work was first selected against), `locator` and `reason`. |
| `assessments[]` | The exact comparison of each binding with its outcome's current target: `relation` (`primary` for the brief's own outcome, `secondary` for a declared relationship), `bound`, `current`, `result` (`aligned`, `mismatch`, `unknown` or `not-applicable`), `coverage` and `reason`. |
| `dispositions[]` | Append-only decisions on a mismatch, or on an `unbound` or `undiscovered` gap with a current approved target (§5): `subject`, `outcome`, `expected_revision`, `decision` (`continue`, `adapt` or `stop`), `rationale`, `decided_by` and `basis` (the digest of the receipt the decision was made on). |

A brief's `outcome` in the in-repo form `REQ-<slug>` is qualified with the brief's own cell and
repository. A cross-repo alias (`<alias>:REQ-<slug>`) names a register the offline snapshot cannot
resolve, so that binding is `could-not-check`.

## 3. Coverage: an absent binding is never aligned

Only `covered` coverage can carry `aligned` or `mismatch`. Every gap is `unknown`, with the gap
named:

| Coverage | When |
|---|---|
| `covered` | An exact comparison of a bound revision with a current approved one, or `outcome: none` (`not-applicable`). |
| `unbound` | The brief names no outcome, or names one without an `outcome-revision`. Every legacy brief is unbound. |
| `undiscovered` | The snapshot declares that work relates to an outcome its own binding does not name. |
| `approval-unavailable` | An approval for the outcome could not be read by this run. |
| `target-unknown` | The snapshot supplies no approved target for the outcome. |
| `could-not-check` | The brief, its frontmatter or its outcome reference could not be read. |

An unapproved rival target is recorded but can never become current, so it never stales approved
work.

The decoder enforces these rules. An `aligned` or `mismatch` result needs covered coverage, an
outcome, a bound revision and a current revision, and for a primary assessment a `bound`
binding declaring that revision. An `unknown` result states its gap. A non-null `current` is the
last approved revision of the assessment's outcome in the section's own `targets[]`. The refusal
code is `coverage`. Duplicate targets, bindings or assessments are refused as `duplicate-id`. A
snapshot that lists one product target, or one subject's relationship to one outcome, twice is
refused as a snapshot error.

## 4. A projection, never a veto

The receipt's prerequisites replace the architecture-only line with a product-assessed scope line
and add `product:` lines. A mismatch gets one line: the decision when a disposition is in force
for it, otherwise a line saying it needs one. An `unbound` or `undiscovered` gap with a
disposition in force gets one line that names both the decision and the gap, for example
`product: continue: <subject> <outcome> (secondary, undiscovered) current r1 (decided by <role>)`.
Every other coverage gap gets its plain gap line. An `aligned` or `not-applicable` assessment
gets no line. A disposition is keyed on its revision, not on the state it was made on (§5), so it
can stay in force after its assessment becomes `aligned` or a gap of another kind, such as
`approval-unavailable`. That disposition is kept in `dispositions[]`, but no line names it: an
aligned assessment prints no line, and the other gap prints only its plain gap line. These lines
inform the deciding role. They gate nothing:
no product result changes a lifecycle, an impact, the next act or a recheck outcome, and a stale
product binding is never a veto.

A recheck reports product changes in the `assess` class. A new or changed approved product target
or an unavailable approval is `assess`, an unapproved rival is `ignored`, and `assess` never
forces reconciliation or a hold. A recheck carries the prior `selected` revision forward while
the binding names the same outcome, so rebinding a brief to a newer revision keeps the original
selection as history. Every prior disposition is carried forward.

## 5. Dispositions

A disposition is recorded with `statusgen context dispose`. It is refused with:

| Refusal | When |
|---|---|
| `stale-disposition` | `basis` is not the digest of the receipt being changed (a concurrent change), or `expected_revision` is not the outcome's current approved target (a stale rebinding). |
| `disposition-scope` | The receipt has no product section; the subject has no assessment for the outcome or no current approved target; the assessment is aligned, not-applicable or has a coverage gap other than unbound or undiscovered; or `decided_by` is not the receipt's deciding role. |

A disposition is in force only for the revision it names. When the current target moves on,
the earlier disposition stays as history and the mismatch or gap is open again. `stop` is a
recorded decision, not a cancellation.

A disposition is keyed on its subject, outcome and expected revision. It records no relation,
coverage or bound revision, so it covers every state of that subject and outcome at that
revision, not only the state it was made on. A decision recorded on an `unbound` gap stays in
force when the brief later binds to another revision and the assessment becomes a mismatch, and
the mismatch line then names that decision (§4). The reverse also holds. To decide the new state
again, the deciding role records another disposition. The latest one for the subject, outcome
and revision is in force.

A carried disposition is checked again whenever a receipt is decoded or rechecked: one whose
`decided_by` is not the receipt's deciding role is refused as `disposition-scope`. A recheck whose
snapshot names a different deciding role than a prior receipt that carries dispositions is refused
as a snapshot error, because those dispositions cannot be carried under another role.

## 6. What this does not establish

- That the snapshot's product targets are the true approvals. The snapshot is the authorized
  universe, as in v1.
- Who recorded a disposition. `decided_by` is a role name, not an authenticated identity, and a
  carried disposition's `basis` names a receipt the record no longer holds, so it cannot be
  checked after the fact. A disposition is trusted only as far as the custody of the receipt that
  carries it, as every other receipt field is.
- A `REQ@revision` grammar or any change to `outcome` and `satisfies`. `outcome-revision` is a
  separate optional key.
- Product observation or measurement. Observed outcomes belong to the continuing-operations
  impact extension.

`amendment-manifest-v1` still binds the selected handoff's canonical digest without change.
Consumers that only accept v1 or v2 must be upgraded before they read a v3 receipt.
