# Context handoff: the offline resolver

`statusgen context` resolves a change's planning context **offline**, from an
explicit snapshot directory. It answers three questions a spec author and a
brief author need answered before a plan moves:

- **discover**: who owns and who consumes the target's contracts? It emits a
  context-handoff receipt.
- **recheck**: does a prior receipt still hold against a newer snapshot?
- **check**: are the amendment obligations safe to apply?
- **dispose**: record a continue, adapt or stop decision on a product-binding
  mismatch, or on an unbound or undiscovered gap (statusgen/23).

The record shapes are owned by statusgen/19: [`spec/context-handoff-v1.md`](../spec/context-handoff-v1.md),
[`schemas/context-handoff-v1.json`](../schemas/context-handoff-v1.json) and
[`schemas/amendment-manifest-v1.json`](../schemas/amendment-manifest-v1.json).
This page covers the resolver that produces and checks those records, which is
statusgen/20. Its pure library is `statusgen/contexthandoff`, and the CLI is a
thin adapter over that library.

## What the resolver reads

The resolver reads **only the snapshot directory** and any files named on the
command line. It never reads:

- a git repository, a forge or the network;
- environment variables or the clock.

Each command opens the snapshot through `os.OpenRoot`, so nothing outside the
directory is reachable through it. Two runs over equal snapshots give equal
receipts and equal digests.

```
<snapshot>/
  snapshot.json              the declaration (below)
  sources/<id>/**.md         the files of each available source
  <proposal paths>           rival handoff records listed in snapshot.json
```

`snapshot.json` is strict JSON, and unknown fields are refused. Its fields:

| Field | Meaning |
|---|---|
| `declared_by` | Four-segment identity `<cell>:<repo>:<stream>:<NN>` of the brief that declares the change |
| `target` | The context-handoff-v1 target record: state, revision, approval, intent, contracts, transition, acceptance source |
| `terms` | Extra discovery terms. The target's contract names are always added |
| `deciding_role` | Role name recorded on the receipt |
| `sources[]` | The authorized source universe. Each entry has `id`, `cell`, `repo`, `role` (`plan` or `guideline`) and `required` |
| `edges[]` | Declared semantic edges (owns, produces, replaces and so on). Each has `subject`, `edge`, `term`, `locator` and an optional `version` |
| `claims[]` | Observed dispatch claims. Each has `subject`, `claim`, `brief_revision`, `observed_at`, `owner` and `checkpoint`. They are read from the snapshot, never from a forge |
| `proposals[]` | Paths of rival handoff records inside the snapshot. Only an approved rival can block or invalidate approved work |
| `product` | Optional product-target input (statusgen/23): `targets[]` (`outcome` as `<cell>:<repo>:REQ-<slug>`, `revision`, `approval` of `approved`, `unapproved` or `unavailable`, and `reference`) and `relationships[]` (`subject`, `outcome`) declaring an outcome work relates to besides its own binding. When present, the receipt is [context-handoff-v3](../spec/context-handoff-v3.md) |

A source whose directory is missing is **unavailable**, and so is a source that
exceeds a read limit. An unavailable source is never read as empty. It becomes
a scoped could-not-check, and the receipt names only the source id: no file
names, no counts.

Only regular `.md` files are read. Symbolic links, other non-regular entries
and dot-directories are skipped.

## Discovery

`discover` runs fixed, versioned queries and records each one on the receipt,
with its parameters and results:

| Query | What it finds |
|---|---|
| `declared-edge` | Semantic owners, producers and planned replacements that the snapshot declares |
| `plan-edge` | `depends`/`unblocks` frontmatter that points to or from the declaring brief |
| `consumers-field` | `consumers:` entries naming a discovery term |
| `directory-enumeration` | Every brief file, independent of README rows |
| `prose-candidate` | Deterministic mentions of a term in brief and plan prose. This is how a prose-only consumer with no typed edge is still found |
| `guideline-enumeration` | Applicable engineering guidelines, by blob |
| `approved-proposals` | Approved rival handoffs, by digest |
| `read-set` | Every file the run read, by blob |
| `coverage-scope` | What kind of alignment the receipt covers |

**Coverage is architecture only** unless the snapshot carries a `product`
input. Such a receipt carries the prerequisite "architecture-only coverage:
product alignment unchecked". A missing product binding means *unchecked*,
never *aligned*.

## Product-target binding

With a `product` input, `discover` emits a
[context-handoff-v3](../spec/context-handoff-v3.md) receipt. The architecture
target and the product target stay separate inputs: a product change never
moves `$.target`, an impact or the next act.

Each impacted brief (and the declaring brief) is bound from its own frontmatter:

```yaml
outcome: REQ-nightly-export   # the requirement the brief moves
outcome-revision: r1          # the approved product target revision it was selected against
```

The receipt compares that binding with the outcome's current target, the last
approved revision in snapshot order:

| Result | Coverage | When |
|---|---|---|
| `aligned` | `covered` | The declared revision is the current approved one |
| `mismatch` | `covered` | It is an older or different revision |
| `not-applicable` | `covered` | `outcome: none` |
| `unknown` | `unbound` | No outcome, or an outcome with no `outcome-revision`. Every legacy brief is here |
| `unknown` | `undiscovered` | A declared relationship to an outcome the binding does not name |
| `unknown` | `approval-unavailable`, `target-unknown`, `could-not-check` | The current target or the brief cannot be read |

An unapproved rival is recorded but never becomes current, so it cannot stale
approved work. The prerequisites list each mismatch and each gap as `product:`
lines. They are a projection for the deciding role, never a veto.

`recheck` reports product changes in a fifth class, `assess`. It never forces
`reconcile` or `hold`. When a brief rebinds to a newer revision, the receipt
keeps the revision the work was first selected against (`selected`) and every
recorded disposition.

## Recheck

`recheck --snapshot DIR RECEIPT` compares a prior receipt with the snapshot and
puts every change into one class:

| Class | When |
|---|---|
| `preserved` | An input changed, but no conclusion depends on it (an unrelated edit) |
| `reconcile` | A conclusion depends on a changed input. Examples: a guideline added or changed, an approved owner, producer version or planned replacement changed, or a new or deleted consumer outside the prior read set |
| `hold` | An input could not be checked. Either it is unavailable, which is a could-not-check scoped to that input, or the coverage on either side is partial, for example because a query was truncated at its result limit |
| `ignored` | An input that cannot affect approved work, such as an unapproved (draft) rival or an unapproved product target |
| `assess` | A product-binding change in a v3 receipt: a new approved product target, an unavailable product approval, or a changed product assessment. The deciding role assesses it. It never forces `reconcile` or `hold` |

Discovery is re-run over the whole new snapshot. It does not just re-hash the
files that were read before, so a new consumer outside the old read set forces
`reconcile` even when every previously read file is unchanged.

Partial coverage is never confirmed current. If either the prior receipt or the
new snapshot has a query truncated at its result limit, or a source that is not
available, recheck raises a `hold` for that side. Each impacted brief is
compared at its fresh revision. If the prior read set was truncated before that
brief, its revision cannot be compared, so the brief is a scoped `hold` rather
than a silent match. When a `reconcile` and a `hold` are both present,
`reconcile` decides the outcome (exit 1).

Two brief files that share one identity are not merged silently. The first in
path order holds the identity, and the receipt's prerequisites name the
collision.

## Check

`check --snapshot DIR HANDOFF [MANIFEST] [PATCH...]` checks each amendment
obligation. A file whose `schema` is the amendment-manifest-v1 schema is the
manifest; there can be at most one. Every other file is a candidate patch, keyed
by its digest.

A patch is a strict single-file unified diff. It applies exactly at the lines it
names or it does not apply: there is no fuzz and no offset search. Every
context line carries its leading space, blank ones included. A blank context
line with no leading space is refused.

| Refusal | When |
|---|---|
| `amend-unbound` | A manifest obligation has no bound patch or follow-up |
| `patch-apply` | The patch bytes are missing, the base revision does not match, or the patch does not apply exactly |
| `history-rewrite` | The patch removes or rewrites populated Evidence or a dated source account |
| `silent-rebind` | An active claim on the brief has neither an acknowledged adjustment nor a cancellation, whatever revision it holds |
| `incompatible-delta` | An approved rival changes the same aspect of the same brief differently, even when the two texts merge cleanly |
| `stale-receipt` | The handoff's receipt no longer matches the snapshot, so it must be reconciled first |

A new obligation on a **delivered** brief (one with populated Evidence) is not
patched into that brief. It routes to an unfinished owner or to a linked
follow-on, and the delivered brief is left unchanged. The follow-up must be
linked: it depends on the delivered brief, the delivered brief unblocks it, or
it owns a term. Any other follow-up is refused as `amend-unbound`.

## CLI

```
statusgen context discover --snapshot DIR [--json]
statusgen context recheck  --snapshot DIR RECEIPT [--json]
statusgen context check    --snapshot DIR HANDOFF [MANIFEST] [PATCH...] [--json]
statusgen context dispose  RECEIPT --subject ID --outcome REF --decision continue|adapt|stop
                           --expected-revision REV --rationale TEXT --by ROLE --basis DIGEST [--json]
```

`dispose` reads no snapshot. It appends one disposition to a v3 receipt and
writes the updated receipt. It refuses with `stale-disposition` when `--basis`
is not the receipt's digest or `--expected-revision` is not the current
approved target, and with `disposition-scope` when there is nothing to decide
or `--by` is not the receipt's deciding role. A decision on an unbound or
undiscovered gap is projected as one prerequisite line naming the decision and
the gap. A decision covers every state of its subject and outcome at the
revision it names, so it stays in force if that assessment later becomes a
mismatch, aligned, or another kind of gap. On a mismatch its line names the
decision. On an aligned assessment or another kind of gap, no line names it,
although the receipt still records it. Every command that reads a v3 receipt refuses one carrying a
disposition from another role.

- `--snapshot` is required.
- `--json` prints the machine output:
  - **discover**: the canonical context-handoff record (v1, v2 when an impact
    is withdrawn, v3 when the snapshot carries product input), which the
    decoder decodes and digests unchanged;
  - **recheck**: `{outcome, prior, receipt, findings}`;
  - **check**: the full report;
  - **dispose**: the updated canonical receipt.
- Text output names the coverage state, every could-not-check scope and the
  coverage scope line, and for a v3 receipt every product target, binding,
  assessment and disposition.
- Every input file is read with a 16 MiB bound.

| Exit | Meaning |
|---|---|
| 0 | Pass: discovery complete, receipt current with complete coverage on both sides, or every obligation passes |
| 1 | Refusal, reconcile, or invalid input |
| 2 | Usage error, such as a missing `--snapshot` or an unknown verb |
| 3 | Hold: could-not-check, including partial coverage on either side, such as a query truncated at its result limit |

## Limits

Default read limits:

| Limit | Default |
|---|---|
| Files per source | 20000 |
| Bytes per file | 1 MiB |
| Bytes per source | 256 MiB |
| Results per query | 2000 |

- The byte limit counts each source separately; it is not a per-run total.
  Proposal files are read outside it, at up to 1 MiB each. Their number, and
  the number of sources, is bounded only by what the 1 MiB `snapshot.json` can
  list.
- A source over a read limit is unavailable, which is a scoped hold.
- A query over its result limit is truncated, which makes coverage partial.
- Neither case is ever reported as complete.

## Fixture coverage limits

The fixtures under `statusgen/testdata/contexthandoff/journeys/` are a neutral
synthetic corpus: two example repositories and one guideline source, with
withheld owners and consumers. They exercise each query, each recheck class and
each refusal above.

The fixtures do **not** establish recall beyond that corpus. In particular:

- **Prose discovery.** `prose-candidate` is term matching. It finds a consumer
  that names a term, not one that describes the contract in other words.
- **Patch shape.** Patches are single-file unified diffs. Multi-file or binary
  patches are refused rather than approximated.
- **Snapshot authority.** The resolver trusts that the snapshot contains the
  authorized universe. Assembling that snapshot, for example from a mirror or a
  live forge, is out of scope.

`statusgen/contexthandoff-mutations.json` is a `muhar` spec. Each test that
guards one of the behaviours above has a mutation that removes that behaviour,
and running the spec shows the test going red:

```
cd statusgen && muhar -spec contexthandoff-mutations.json
```
