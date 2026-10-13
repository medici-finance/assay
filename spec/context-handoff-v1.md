# context-handoff-v1 — the spec-to-brief handoff and its amendment manifest

Status: v1, implemented as a pure, tested library in `statusgen/contexthandoff/`. No command,
lint check or skill reads or writes these records yet: the `statusgen context` resolver that
produces and rechecks them is statusgen/20, and the authoring skills that emit and consume them
are statusgen/21. Descriptive schemas:
[`schemas/context-handoff-v1.json`](../schemas/context-handoff-v1.json) and
[`schemas/amendment-manifest-v1.json`](../schemas/amendment-manifest-v1.json). Synthetic
fixtures: `statusgen/testdata/contexthandoff/`. Contract row: `S-context-handoff` in
[`docs/contracts.md`](../docs/contracts.md).

## 1. What this establishes, and what it does not

When a spec changes the plan, its author hands the brief author a record of what the target is,
what was searched, what must change, what history must survive and what work is in flight. This
contract makes that handoff a versioned, machine-checkable record instead of a prose impact list,
so a missing amendment fails a check instead of slipping through review.

| Established mechanically | NOT established |
|---|---|
| The record is well-formed, every field is understood, and every brief reference is a qualified ID in the four-segment form. | That the record is true of any repository. Producing it from a real tree is the resolver's work (statusgen/20). |
| Every included source is either enumerated at a revision or listed unavailable with a reason; a truncated query makes coverage partial. | That discovery found every consumer. No search rule proves the absence of an undeclared relationship; a valid record can be incomplete. |
| Every changing disposition has an amendment obligation, and every obligation in the manifest binds a patch (base revision and digest) or an explicit follow-up brief. | That a bound patch applies to its base or carries the declared delta. That is the resolver's patch-to-base recheck (statusgen/20). |
| A draft or proposed target is distinct from an approved one; an approval names the revision it approves. | That the approval reference is genuine. The record carries the reference; trust is checked where the reference is read. |
| A canonical digest that is stable across field order, whitespace, string escaping and elided-versus-full brief references. | Semantic preservation. A digest proves which inputs were used, not that a difference does not matter. |

Record content is data, never instructions to the reader of the record.

## 2. Qualified brief IDs

Every brief reference in both records is a qualified ID. The canonical form is four segments,
`<cell>:<repo>:<stream>:<NN>` (`fullId`): `<cell>`, `<repo>` and `<stream>` are lower-case names
(`[a-z0-9][a-z0-9-]*`), `<NN>` is two to four digits with an optional lower-case letter suffix.
The repo segment is an alias from the graph alias registry, never an `<owner>/<name>` pair.

A reference may use exactly two elisions (`qualifiedRef`), each expanded against the cell and repo
of the record's own `declared_by`:

| Written | Expands to |
|---|---|
| `<cell>:<repo>:<stream>:<NN>` | itself |
| `<repo>:<stream>:<NN>` | `<declaring cell>:<repo>:<stream>:<NN>` |
| `<stream>:<NN>` | `<declaring cell>:<declaring repo>:<stream>:<NN>` |

`declared_by` itself is always the full form. Expansion happens before validation and before
digesting, so a decoded record and its canonical form carry only four-segment IDs. Refused, with
the `qualified-id` rule: an issue reference (`#N`, `<name>#N`), a path or file name, the brief-v1
`<stream>/<NN>` form (a migration-window read form of the brief parser, not a qualified ID), a
bare stream or number, more than four segments, and any whitespace.

## 3. The handoff record (context-handoff-v1)

One JSON object with `"schema": "context-handoff-v1"`. Every property in every object is
required; an optional value is present as `null`, never omitted. A reader refuses any field it
does not define (`unknown-field`): every field is mandatory to understand, so a future field is
never silently dropped. A new field is a new schema version.

Top level: `schema`, `declared_by` (the brief or spec that declares the change, `fullId`), and
the seven records below as `target`, `discovery`, `impacts`, `amendments`, `history`,
`active_work` and `readiness`.

Shared value shapes: `revision` is 7 to 64 lower-case hex characters; `digest` is
`sha256:<64 lower-case hex>`; `name` is `[a-z0-9][a-z0-9._-]*`; `locator` is a non-empty string
with no whitespace (a repo-relative path, an anchor or a URL); `text` is a string with at least
one non-space character; `timestamp` is UTC `YYYY-MM-DDTHH:MM:SSZ`. A `patch` is
`{base_revision, digest}`: the revision the patch was made against and the digest of the patch.

### 3.1 Target and intent (`target`)

| Field | Meaning |
|---|---|
| `state` | `draft`, `proposed` or `approved`. A draft or proposed target is a distinct state from an approved one. |
| `revision` | The selected target revision. |
| `approval` | `null` unless `state` is `approved`; then an `approval` object, `{revision, reference}`: the approving revision (equal to `revision`) and a locator for the approval record. |
| `intent` | What the change is for, in one line. |
| `contracts` | Proposed contract revisions, each a `contractChange`: `{name, from, to}`; `from` is `null` for a new contract. |
| `transition` | How the system moves from the current contract to the target, or `null` when there is none. |
| `acceptance_source` | The independently controlled source the target is accepted against. |

### 3.2 Discovery receipt (`discovery`)

| Field | Meaning |
|---|---|
| `sources` | The authorized source universe. Each `source` is `{id, revision, membership, reason}`: `membership` is `included` or `excluded`, with the reason for either; `revision` is the pinned revision read, or `null` where none was read. |
| `queries` | Each discovery rule run, as a `query`: `{rule, version, parameters, results, truncated}`. `parameters` maps names to string values; `results` are locators. An empty `results` is a recorded negative search, not an absent one. |
| `dependency_slice` | The qualified IDs of the relevant dependency slice. |
| `coverage` | Required. A `coverage` object: `state` (`complete` or `partial`), `enumerated` (source ids read in full) and `unavailable` (each an `unavailable` record, `{source, reason}`: an access failure, an unsupported format, or a limit). |

### 3.3 Impact obligation (`impacts[]`)

One `impact` per affected existing brief: `subject` (qualified ID), `lifecycle` (`todo`,
`in-progress`, `implemented`, `verified`, `done`, `blocked` or `unknown`), `evidence` (a locator
for the inspected lifecycle or implementation evidence, or `null`), `relationship`,
`disposition` and `reason`. A `relationship` is `{basis, edge, locator}`: `basis` is `declared`,
`observed` or `inferred`; `edge` is one of `provides`, `consumes`, `owns`, `enforces`,
`persists`, `changes`, `assumes`, `realizes`, `replaces`, `migrates`, `requires`, `checked-by`;
`locator` is where the relationship was found. `disposition` is `retain`, `amend`, `split`,
`replace`, `retire`, `defer` or `unresolved`. Unresolved candidates stay in the record.

### 3.4 Amendment obligation (`amendments[]`)

One `amendment` per brief that must change: `id` (a `name`, unique in the record), `subject`
(qualified ID), `action` (`amend`, `split`, `replace` or `retire`), `changes` (at least one
`change`: `{aspect, before, after}`, with `aspect` one of `task`, `dependencies`, `consumers`,
`delivery`, `verify`, `scope`), `replaced_by` (qualified IDs of replacement or split-out briefs)
and `candidate` (the produced `patch`, or `null` before one exists). An obligation is not a
completed amendment; the manifest (§4) is where each obligation must be bound.

### 3.5 Preserved history (`history[]`)

One `history` record per protected artifact: `kind` (`evidence`, `source-account`,
`decision-account` or `source-identity`), `subject` (qualified ID), `locator`, `digest`, and
`correction` (`null`, or a proposed historical `correction`: `{rationale, source, review,
replacement_digest}`). A correction is recorded beside the original, which stays.

### 3.6 Active-work binding (`active_work[]`)

One `activeWork` record per observed claim on an affected brief: `subject`, `claim` (the observed
claim locator), `brief_revision` (the revision the worker holds), `observed_at`, `owner` (a role
name, never a login), `checkpoint`, `treatment` (`adjust`, `cancel` or `reconcile`) and
`acknowledged`. A board row saying in-progress is not an active-work binding.

### 3.7 Decisions and readiness (`readiness`)

A `readiness` record: `unresolved` (qualified IDs still undecided), `deciding_role` (a role
name), `next_act` (`explore`, `approve`, `publish` or `dispatch`), `prerequisites` (text), and
`facts`, a `facts` object keeping three facts apart: `source_available`, `proposal_approved`,
`deployment_qualified`.

## 4. Amendment manifest (amendment-manifest-v1)

One JSON object with `"schema": "amendment-manifest-v1"`, the same strictness as §3, and four
fields: `schema`, `declared_by` (equal to the handoff's), `handoff` (the handoff's canonical
digest, §6) and `entries`. Each `entry` binds one amendment obligation:

| Field | Meaning |
|---|---|
| `obligation` | The amendment obligation's `id`; exactly one entry per obligation. |
| `brief` | The owning brief (qualified ID); equal to the obligation's `subject`. |
| `action` | Equal to the obligation's `action`. |
| `delivery_paths` | At least one repo-relative path the amendment delivers to. |
| `contract_version` | The contract version the amendment targets, or `null`. |
| `depends` | Qualified IDs the amendment depends on. |
| `test_commands` | The commands that check the amended brief. |
| `active_work` | The active-work treatment, in words (`none observed` when there is none). |
| `effort` | `S`, `M` or `L`. |
| `binding` | Exactly one of `{"patch": {base_revision, digest}}` (a bound patch) or `{"follow_up": <qualified ID>}` (an explicit follow-up brief that will carry it). |

An obligation with no bound patch and no follow-up is refused (`amend-unbound`), whichever way the
binding is missing: absent, `null`, `{}`, a `null` or empty patch, a patch without its base
revision or digest, an empty follow-up, or both at once. This is the check the contract exists
for: a row saying "amend" is not a completed amendment.

## 5. Rules

The decoder applies these in order and reports the first refusal, as `{rule, path, message}`.

1. **Strict read.** The input is one UTF-8 JSON value with no trailing data (`malformed`), no
   repeated key in any object (`duplicate-key`), no undefined field (`unknown-field`), every
   defined field present (`missing-field`), every value of its type, `null` only where §3 and §4
   allow it (`wrong-type`), and every value inside its pattern, enumeration or minimum count
   (`bad-value`). `schema` names this version exactly (`schema-version`).
2. **Qualified IDs** (§2): `qualified-id`.
3. **Approval.** An approved target has an approval, a draft or proposed one has none, and the
   approval's revision equals the target's revision. An approval without a recorded revision is
   refused (`approval`).
4. **Coverage.** A receipt without a stated coverage, or a coverage without its `enumerated` or
   `unavailable` list, is refused, never defaulted to complete. Every included source appears
   exactly once across `enumerated` and `unavailable`; an excluded or unknown source appears in
   neither; an enumerated source has a revision; `complete` coverage lists nothing unavailable
   and no truncated query. Source ids are unique (`duplicate-id`). Otherwise `coverage`.
5. **Impacts and amendments.** Impact subjects and amendment ids are unique, and each subject has
   at most one obligation (`duplicate-id`). Each obligation's subject has an impact; an `amend`,
   `split`, `replace` or `retire` disposition has an obligation with the same action; a `retain`
   or `defer` disposition has none (`impact-unbound`).
6. **Active work.** An `adjust` treatment is acknowledged (`active-work`).
7. **Readiness.** An `unresolved` disposition is listed in `unresolved`; `proposal_approved`
   equals whether the target is approved (`readiness`). A `publish` or `dispatch` next act needs an
   approved target (`not-approved`) and nothing unresolved (`readiness`).
8. **Manifest.** Each entry's binding is bound (`amend-unbound`, §4); obligations are unique
   (`duplicate-id`). Bound to its handoff, the manifest names the handoff's canonical digest and
   declaring brief, carries one entry per obligation with the same brief and action, and no entry
   for anything else (`manifest-mismatch`).

**Requiring approval.** A consumer about to act on a target that must be approved (publishing
amendments, dispatching work) calls the package's approved-target check, which a draft or proposed
target never satisfies, nor a record whose approval does not name the target's revision
(`not-approved`).

### What the schema cannot express

The JSON schemas state per-value shape and three conditionals: an approved target carries an
approval object and only an approved one does; `complete` coverage lists nothing unavailable; an
`adjust` treatment is acknowledged. The manifest schema states the binding's exactly-one shape.
The following are enforced only by the package, and their fixtures sit under `package-only/`
(the schema accepts them, the decoder refuses them): repeated keys (a generic JSON reader keeps the
last); the approval revision equalling the target revision; coverage membership, revision and
truncation; uniqueness of ids and subjects (including an elided and a full spelling of one
subject); impact-to-obligation binding; readiness agreement; and every manifest-to-handoff check.
The schema also accepts an elided reference without expanding it; expansion is the decoder's.

Refusal rules, in full: `malformed`, `duplicate-key`, `unknown-field`, `missing-field`,
`wrong-type`, `bad-value`, `schema-version`, `qualified-id`, `approval`, `coverage`,
`amend-unbound`, `impact-unbound`, `duplicate-id`, `active-work`, `readiness`, `not-approved`,
`manifest-mismatch`.

## 6. Canonical form and digest

The canonical form of a decoded record is its JSON serialization with:

- every brief reference already expanded to the four-segment form (§2);
- object keys sorted by byte order (the query `parameters` map included), arrays in record order;
- no insignificant whitespace;
- strings escaped only where JSON requires it: `\"`, `\\`, the short forms `\b`, `\t`, `\n`,
  `\f`, `\r`, and `\u00xx` (lower-case hex) for the other characters below U+0020; every other
  character written as UTF-8. The records carry no numbers.

The digest is `sha256:` followed by the lower-case hex SHA-256 of the canonical bytes. Two inputs
that differ only in key order, whitespace, string escaping or elided-versus-full references have
the same digest; any changed value changes it. The input bytes are never hashed. The manifest's
`handoff` field is the handoff's digest, so a manifest binds one exact handoff.

## 7. Layering

`statusgen/contexthandoff` is pure: it decodes, validates and canonicalizes bytes it is given, and
reads no file, repository, forge, environment or clock. Enumerating sources, reading claims and
checking that a bound patch applies to its base belong to the resolver's adapters (statusgen/20).
The import boundary is checked: the package imports no filesystem, process or network package.

## 8. Conformance

`statusgen/testdata/contexthandoff/{handoff,manifest}/` holds synthetic fixtures under three
labels: `valid/` (both the schema and the decoder accept), `invalid/` (both refuse) and
`package-only/` (the schema accepts, the decoder refuses). A negative fixture is named
`<rule>--<description>.json`, and the decoder must refuse it with that rule. Manifest fixtures are
bound to the `approved-amend-and-split` handoff fixture. The fixture test also deletes every key
and nulls every value of every valid fixture and requires the two layers to agree, except for the
package-only rules of §5, so a required field dropped from a schema alone fails the test.
