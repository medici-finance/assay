# project-obligations-v1 — versioned source obligations and project applicability

Status: v1, implemented as a tested library in `statusgen/projectobligations.go`. No lint check,
command or workflow calls the loader yet: activating it for an adopting project waits on the
owner decision the source brief names. Descriptive schema:
[`schemas/project-obligations-v1.json`](../schemas/project-obligations-v1.json). Companion to
[`registers-v1.md`](./registers-v1.md) §6 (REQUIREMENTS) and §7 (DECISIONS).

## 1. What this establishes, and what it does not

A project can say, in a reviewable and replay-safe way: *this source revision, under this
permitted use, is mapped by this clause to these existing requirements for this project profile,
and a named human decision accepted that the mapping applies (or does not) over this period*. It
can then be checked offline that no accepted conclusion rests on a forged actor, a stale
revision, a subject digest absent from the corroborated decision record, or a requirement that
does not exist.

That last check binds the subject to the record **as it stands in the evaluated tree**, not to
the moment its issue was closed. Three limits follow, and none is checked mechanically: a digest
added to a record after its issue closed is caught only by the review of the change that adds
it; one closed issue corroborates every digest its record carries, so a record naming two
subjects covers both; and the record's prose (approve or decline) is not read (§4, the note after rule 10).

It establishes three distinct things and keeps them apart:

| Layer | Established mechanically | NOT established |
|---|---|---|
| **Source identity** | A stable id, an edition/revision, an issuer, a locator, dates (or an explicit `unknown`), and a content digest where storage is permitted. A locator alone is not a content version. | That the edition is the right one for the project. |
| **Permitted use** | Whether stored text may exist in the record and whether a model may read it. Absent or `unknown` is treated as denied. | That the licence is correctly characterised (an adopter attests the access reference). |
| **Semantic correctness** | Nothing. | Whether the paraphrase, mapping or applicability conclusion is *right*. That is a qualified person's judgment, recorded as a review reference. A reviewer approving is a record that a review was claimed, not that the interpretation is correct. |

It is not a requirement lifecycle (the requirement register keeps that), not an approval
mechanism (acceptance is a link to an existing DECISIONS record), not a standards library, and
not a certification. A mapping is a *reference* to REQ ids; it never redefines one.

Source text is untrusted data. It is never an instruction to the executor.

## 2. Records

All records live in one explicitly supplied local JSON document with `"schema":
"project-obligations-v1"`. The loader reads a local file only: it refuses a remote locator, and
it never reads the clock, the network or the forge. The evaluation date is an explicit argument.

**Source revision** (`sources[]`). `id` (`SRC-<slug>`), `kind` (`standard`, `regulation`,
`contract`, `policy`, `other`), `issuer`, `locator`, `revision` (the edition), `published` and
`effective` (a `YYYY-MM-DD` date or the explicit string `unknown`; an empty value is refused),
`capturedAt`, optional `contentDigest` (`sha256:` plus 64 hex of the stored text), optional
`predecessor` (`<id>@<revision>`), `use` (`accessRef`, `storage`, `ai`, each `permitted`,
`denied` or `unknown`) and optional `text`. A given id and revision is immutable: a second
record with the same key and a different content digest is a mutated revision and is refused.
Stored `text` requires `storage: permitted` and, when a digest is present, must match it.
Where storage is not permitted the record carries metadata and an external human-verifiable
reference only, and a content digest is refused.

**Obligation mapping** (`mappings[]`). `id` (`MAP-<slug>`), `revision` (integer, runs 1..n with
no gap), `source` (id and revision), `clause`, a bounded human-authored `paraphrase`
(at most 600 characters), `reqs` (existing in-repo requirement ids, or cross-repo
`<alias>:REQ-<slug>` references), `profile` (id and revision), optional `context`
(entity, activity, jurisdiction), optional `controls`, an accountable `owner`, and `supersedes`
(the prior revision's `<id>@<revision>`). Many sources may map to one REQ and one source to
several REQs; conflicting mappings are never deduplicated away.

**Applicability decision** (`decisions[]`). `id` (`APD-<slug>`), `mappingId` and
`mappingRevision`, `source` and `profile` (the revisions decided on), `outcome` (`applicable`,
`not-applicable` with a `reason`, or `unresolved`), `scope`, `effectiveFrom`, optional
`effectiveTo`, `decidedAt`, `proposedBy`, `reviewer`, optional `supersedes` (an earlier decision
id) and `acceptance`. Everything except `acceptance` is the *proposal*, and the subject digest
(§3) covers all of it. A proposal with no acceptance link is a candidate, not a decision. A
`supersedes` pointer takes effect only when the decision carrying it is itself accepted: an
unaccepted proposal that names a predecessor retires nothing and is one more current decision on
the mapping.

**Acceptance link** (`decisions[].acceptance`). References to existing records, never a new
approval: `decisionId` (an existing `DR-<slug>`), `subjectDigest` (§3), `disposition` (must be
`approved`), `corroborationRef` (the decision issue reference) and `reviewRef` (a different
existing `DR-<slug>` standing for the qualified review). An input that carries a receipt object
anywhere is refused: a supplied JSON object is not self-authenticating.

## 3. Canonical digests

Both digests are `sha256:` plus the lowercase hex SHA-256 of a canonical byte string. The
canonical string is the concatenation, in the order listed, of one **netstring** per field: the
field's UTF-8 byte length in decimal, `:`, the bytes, then `,` (the clause `4.1` encodes as
`3:4.1,`). A sequence of netstrings is uniquely decodable whatever the fields contain, so a
newline, comma, `@` or `:` inside a clause, paraphrase or context value cannot move a field
boundary and two different subjects never share a canonical string. A list field (`reqs`,
`controls`) is sorted bytewise, each element is encoded as a netstring, and the concatenation is
itself encoded as ONE netstring field: order never changes an identity, and `["a,b"]` differs
from `["a","b"]`. An absent optional value is the empty string (`0:,`). An id and its revision are
always two fields, never an `<id>@<revision>` join.

Mapping digest fields, in order: `project-obligations-v1/mapping`, mapping `id`, `revision`
(decimal), source `id`, source `revision`, `clause`, `paraphrase`, `reqs` (list), profile `id`,
profile `revision`, `context.entity`, `context.activity`, `context.jurisdiction`, `controls`
(list), `owner`.

Subject digest fields, in order: `project-obligations-v1/subject`, source `id`, source
`revision`, source `contentDigest`, mapping `id`, mapping `revision`, the mapping digest, profile
`id`, profile `revision`, then every field of the decision proposal: decision `id`, `outcome`,
`reason`, `scope`, `effectiveFrom`, `effectiveTo`, `decidedAt`, `proposedBy`, `reviewer`,
`supersedes`.

The subject digest binds the exact source revision and content, the exact mapping revision and
content, the profile revision, and the whole proposal being approved: outcome and not-applicable
reason, scope and effective period, decision time, proposer, reviewer and the decision it
supersedes. Changing any of them changes the digest, so an approval of one subject cannot be
replayed onto another, and a reason, a decision time or a supersession pointer cannot be
rewritten in the input after approval. The reference tests compare against digests computed by
an independent implementation of this section
(`statusgen/testdata/projectobligations/canonical_digest.py`, written from this text and run by
hand, not by CI), not by a second copy of the serializer.

## 4. Acceptance rules

A decision is accepted only when every rule holds; the first failures are reported as reasons.

1. The acceptance link exists (otherwise `unresolved`, reason `no-acceptance-link`).
2. The decision binds the mapping revision under evaluation, and its `source` and `profile`
   equal the mapping's. A mismatch is stale.
3. The outcome is `applicable` or `not-applicable` (the latter with a non-blank reason).
4. `evaluation date` lies inside `[effectiveFrom, effectiveTo]` and `decidedAt` is not in the
   future. An out-of-period decision is `unresolved`, never accepted.
5. `reviewer` and `proposedBy` are both present and differ. The preparer cannot approve its own
   proposal.
6. `subjectDigest` equals the digest recomputed from the current records.
7. `disposition` is `approved`.
8. `decisionId` names an admitted DECISIONS record **whose body contains the recomputed subject
   digest**. A record is admitted only when its frontmatter id equals the id its file name
   carries and no other file in the register claims that id; an id claimed by two files is
   held (`unresolved`, `decision-record-ambiguous`), never resolved by directory order. The
   digest-bearing body and the receipt of rule 9 are taken from the same admitted file
   (`receipt-record-mismatch` otherwise). This stops a decision approved for one subject being
   replayed onto another while the record is unchanged; it does not stop a digest being added
   to the record later (§1).
9. A trusted corroboration receipt exists for that decision, its reference equals
   `corroborationRef`, and the corroborated closer equals `reviewer`. Receipts are produced only
   from caller-supplied, pre-fetched decision-issue state through the existing decision-gate
   corroboration seam; a decoded JSON object is never trusted. With no receipt the result is
   `unresolved` (`no-trusted-corroboration`), never a default forge fetch.
10. `reviewRef` names a different admitted DECISIONS record. A missing or ambiguous review
    reference leaves the decision `unresolved`; a reference to the decision itself, or to no
    record, is refused. The reference is neither inside the subject digest nor corroborated: any
    other admitted record satisfies it, so it records that a review was claimed, not which
    review covered this subject.

Corroboration proves who closed which issue and where, not the meaning of the approval prose;
rule 10 and the semantic-correctness row of §1 are why a review reference is also required.

## 5. Mapping states

| State | Meaning |
|---|---|
| `accepted` | A current, accepted `applicable` decision resolves. |
| `not-applicable` | A current, accepted `not-applicable` decision; the mapping stays in the inventory with its reason. |
| `unresolved` | No current decision, or a pending one (rules 1, 4, 9 or 10 above, an ambiguous record id under rule 8, or an outcome of `unresolved`), or a cross-repo REQ the offline loader cannot read (could-not-check, not empty). |
| `conflict` | More than one current decision on one mapping revision, including an accepted decision and an unaccepted proposal that names it in `supersedes`. None is picked. |
| `rejected` | A decision exists and fails a binding rule, or a referenced REQ is unknown or withdrawn. |
| `superseded` | A prior revision. Kept as history; a decision bound to a superseded revision is history, never carried forward to the new revision. |

`Complete` is false for an empty set and for any current mapping that is not `accepted` or
`not-applicable`; an empty source set cannot demonstrate complete coverage. A new mapping
revision or a new source or profile revision opens a new applicability question: it never
overwrites the earlier record.

## 6. REQ dereference

A mapping's `reqs` are checked against the production requirement parser over the register
directory. An unknown id rejects the mapping, a `withdrawn` requirement rejects it, a malformed
id is a shape problem, and a cross-repo reference holds the mapping as could-not-check. An absent
register is a legitimate empty register, so every in-repo reference fails to dereference; an
unreadable register is an error, never an empty one. The requirement register's own lint is
unchanged by this contract.

## 7. Model input

`modelInputs` is the only place a source becomes model input. Full text enters only when the use
record names an access reference **and** both `storage` and `ai` are `permitted` and text is
present. Otherwise the payload is metadata only (id, kind, revision, issuer; no locator, no
text), and every payload is labelled `untrusted-data`. A metadata-only review remains possible.

## 8. Trust boundaries

Two layers are independent by construction. The *candidate validator* checks shape (vocabularies,
identifier forms, lineage, permitted use, digests of stored text). The *authority boundary*
re-derives every binding from the explicit inputs and does not assume the validator ran, so a
forged acceptance is refused even when the validator is bypassed. Tests exercise the boundary
directly for that reason. Both halves of an acceptance's evidence, the digest-bearing record
body and the corroboration receipt, are selected by one admission rule (§4 rule 8), and the
boundary re-checks that the record it reads is its own file and that the receipt was minted
from that file.

What is corroborated is the close of an issue linked to a record file. The content of that file
is trusted as it stands in the tree being evaluated: integrity of the record after the close is
the job of merge review on the register, not of this contract.

## 9. Conformance

A conforming implementation MUST: refuse stored text without permitted storage; exclude from
model input any source that is not explicitly permitted; treat unknown use as denied; require the
subject digest and recompute it from current records; require that the decision record carry the
digest; require trusted corroboration, an approved disposition, a distinct reviewer and a review
reference before accepting; never accept a decision bound to a stale revision; hold on a
cross-repo or unreadable reference rather than report an empty result; and never report complete
coverage for an empty set. The reference implementation's mutation spec
(`statusgen/projectobligations-mutations.json`) carries at least one single-site mutation for each
of these obligations, for each refusal reason the authority boundary can return, and for each
state in §5, and requires the `TestAssurance` suite to catch every one. A test
(`TestAssuranceReasonsMutated`) fails when a reason the boundary emits is named by no entry. It is a local harness (`muhar`); no workflow runs it.

Out of scope and deliberately absent: a scheduler, a document store, packet preparation
(iso-9001/09), change-impact propagation (iso-9001/10) and qualification (iso-9001/11).
