# Assay Open Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen/v0.8.0`

## Scope

This directory contains the normative, versioned, third-party-implementable
specification for the Assay methodology — the brief format, the register system,
and the lifecycle and status board. These documents define what a conforming
implementation MUST do.

The specification covers three domains:

| Document | Domain |
|----------|--------|
| [`brief-v1.md`](./brief-v1.md) | The brief-v1 schema: frontmatter fields, body structure, dependency rules, Verify tables, execution tiering, mid-flight change routing, and conformance requirements. |
| [`registers-v1.md`](./registers-v1.md) | The append-only registers (FINDINGS, INTAKE; RETRO informative): per-entry-file storage and generated views, slug IDs, append-only and tombstone rules, deletion detection, and linter conformance. |
| [`lifecycle-v1.md`](./lifecycle-v1.md) | The brief lifecycle (`todo` through `done`), the single-writer STATUS.md board, the Next-up work-queue semantics, review gates, and what the board does and does not measure. |

This specification does not state which repository or licence the Assay artifacts ship
under, and does not describe any particular deployment. That mapping is out of scope
here and is recorded once, elsewhere, in the project's own documentation; restating it
in the specification is what previously put a superseded mapping into circulation.

## What the board measures (normative preamble)

**The board is derived from authored artifacts with consistency linting, not
measured from ground truth.**

The board generator parses status tables, frontmatter, Verified/Reviewed cells,
and Evidence blocks — all documents written by the same identities whose work it
reports. It checks the *internal consistency* of those documents (duplicate ids,
missing evidence, unresolved findings, malformed gates) and is backstopped by
adversarial spot-verification; it does not independently observe that the code
does what a brief claims.

The defensible statement is "status is derived from authored artifacts with
consistency linting and independent re-verification" — not "status is measured,
never self-reported." The strong form is false: the sensors are writable by the
same identities whose work they report. Every conforming consumer of this
specification MUST claim the weaker, true statement.

## Versioning and change policy

### Version numbers

Each document carries its own version (`v1.0-draft`, `v1.0`, `v1.1`, etc.).
The version of the specification as a whole is the version of the most recently
changed document.

### Draft phase

During the `-draft` phase, versions are unstable. Breaking changes MAY be made
without a major-version bump. Once a document reaches `v1.0` (published), the
semantic versioning rules below apply.

### Semantic versioning (post v1.0)

- **Patch** (v1.0.x): clarifications, typo fixes, non-semantic wording changes.
  No conformance impact.
- **Minor** (v1.x.0): new OPTIONAL fields, new SHOULD guidance, expanded
  conformance that does not invalidate existing conforming implementations.
- **Major** (v2.0.0): any change that makes a previously conforming
  implementation non-conforming (new MUST requirement, field redefinition,
  lifecycle state change).

### Conformance across versions

A conforming implementation MUST state which specification version it targets.
An implementation conforming to v1.1 MAY also conform to v1.0 if the v1.1 changes
are additive.

### Conformance is self-declared

There is no conformance test suite, no claim registry, and no arbiter. An
implementation's conformance statement is **self-asserted and unaudited**: anything may
declare itself conforming, and nothing in this specification checks that it is. A
machine-checkable conformance fixture set is the intended remedy; until one exists, a
conformance claim carries exactly the weight of its author's reputation.

## Reference implementation

The reference implementation is `statusgen` — a repo-agnostic Go tool that
generates the `STATUS.md` board from brief-v1 documents and register files,
computes the Next-up work queue, and lints the full document set. It is the
single writer of the generated board.

This specification describes the behaviour of **`statusgen/v0.8.0`**. Semantics that
are version-sensitive (register field parsing and resolution semantics in particular)
are those of that version; an implementation targeting a different version SHOULD say so.

The reference implementation is authoritative for edge-case interpretation but
is not the specification. Where the specification and the reference
implementation conflict, the specification text governs; such conflicts SHOULD
be reported as specification bugs — and the known ones are listed below rather than
left to be discovered.

## Known divergences from the reference implementation

The specification text governs, so each item below is a defect in `statusgen/v0.8.0`
measured against this specification, not a licence to ignore the requirement. They are
disclosed here because a specification that mandates three-state reporting while its own
reference implementation reports in two states, silently, is the exact failure the
requirement exists to prevent.

Where a `Tracked` column below reads "internal", the divergence is tracked in the
reference implementation's own issue tracker, which is not a public one; the entry is
not a resolvable link and a reader is not expected to follow it. A `—` marks a
divergence with no tracker item at all. Everything needed to understand a divergence is
in the row itself.

### Unimplemented normative clauses

Requirements this specification states as MUST that `statusgen/v0.8.0` does not check.
An adopter's green lint is silent about all of them.

| Clause | What is not checked | Effect | Tracked |
|---|---|---|---|
| `brief-v1.md` §9.2.5 (body sections) | Only `## Verify`, and `## Evidence` at `verified`/`done`. `Context`, `Ground rules`, `Task`, and `Review` are unchecked; §4.1's `files:`/`facts:`/`consumers:` lines are unchecked. | A brief stripped of four of its six required sections lints clean. | — |
| `brief-v1.md` §9.2.7 (literal commands) | Only that a Verify table exists with one row whose Command and Expect cells are non-empty. | A prose-only row (`the tests should all pass` / `it works`) lints clean. | — |
| `brief-v1.md` §9.2.10 (diagnostic shape) | No diagnostic carries a line number; the rule reference appears on some only. | A diagnostic is not machine-locatable within its file. | — |
| `brief-v1.md` §5.4 (`unblocks` inversion) | Each `unblocks` entry is resolved to a real brief, but mutual inversion with the target's `depends` is never verified. | A one-way dependency edge lints clean, and the wave graph derived from it is wrong in one direction. | — |
| `brief-v1.md` §3.2 / §4.1.1 (`write-scopes`) | The `write-scopes` field is not parsed or validated, and no write-scope set is derived from the Context `files:` line. | The advisory dispatch-time overlap warning has no data from this tool; a dispatch consumer must derive scopes itself, and a malformed `write-scopes` value lints clean. | — |
| `registers-v1.md` §7.2.6 (scoped-but-`new`) | Not implemented; the tool has the *inverse* alarm (it ages entries still marked `new`). | An entry scoped into a stream but left `new` is not flagged. | — |
| `registers-v1.md` §7.2.8 (entry format) for FINDINGS | An entry with an empty `affects:` lints clean, though §4.3 requires one or more brief IDs. Holds for INTAKE (date + disposition key). | A finding that affects nothing passes, and excludes nothing. | — |

`brief-v1.md` §9.2.3 (wave-versus-`depends`) is **not** in this table: the specification
explicitly permits reporting it as a non-fatal notice, which is what the reference
implementation does.

### Three-state instrument invariant

`brief-v1.md` §8 — the reference implementation violates it in at least these places:

| Divergence | Effect | Tracked |
|---|---|---|
| The register tombstone check returns "nothing found" when run outside a repository checkout, and skips on any history-read error. | A check that *could not run* is indistinguishable from one that ran clean. | internal |
| The link checker exempts directory-shaped paths by construction. | A green does not disclose that a subset was never checked. | internal |
| An advisory-check wrapper reports PASS when the underlying tool exits with an error. | A crashed check reads as clean. | internal |
| `--lint` itself terminates in `LINT: PASS` / `LINT: FAIL n problem(s)`. | Two terminal states, not three. | — |

### Scaffolding

The reference implementation's `init` command — the first command an adopter runs —
scaffolds a register in the **retired** dialect: `FINDINGS.md` and `INTAKE.md` as single
files with no per-entry directories (`registers-v1.md` §2.2: not conforming for a new
register), seeded with numeric-counter IDs (§3.4 rejects those on new entries), under a
header asserting that sequence contiguity is enforced (§3.2 forbids claiming it).
Tracked internally.

### Semantics

**Execution witness** (`lifecycle-v1.md` §2.4) — no command, exit code, or output hash is
recorded for a Verify row, so `verified` attests that Evidence was *authored*, not that
it was *executed*. Tracked internally.

**Role separation** (`lifecycle-v1.md` §7.1.2) — the non-implementer requirement is
checked against the Evidence text, not against actor identity; per-role credentials are
not isolated between sessions. It is a convention, not a boundary.

**Merge enforcement** (`lifecycle-v1.md` §3.2) — branch protection is unavailable on the
reference implementation's host plan for private repositories, so CI gates report but do
not block.

## Normative keywords

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in the specification
documents are to be interpreted as described in RFC 2119.

## Conformance summary

A conforming implementation of the full Assay specification MUST:

1. Produce and consume brief-v1 documents per `brief-v1.md`.
2. Maintain FINDINGS and INTAKE registers per `registers-v1.md`. (RETRO is
   informative and carries no conformance requirement.)
3. Implement the lifecycle, generate the board, and compute Next-up per
   `lifecycle-v1.md`.
4. Claim the weaker, true statement ("derived from authored artifacts with
   consistency linting"), never the strong form ("measured ground truth").
5. State which specification version(s) it targets, and disclose known divergences
   from it rather than leaving them to be discovered.
