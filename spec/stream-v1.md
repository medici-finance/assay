# Stream ownership and milestone integration

**Version:** v1.0-draft
**Status:** draft — proposed integration contract; existing stream behavior is unchanged.

## 1. Scope and existing contracts

A stream owns a coherent area of work, its canonical scoping source and its briefs.
A delivery milestone states a supported outcome within that area. It may draw on other
streams. A wave orders implementation dependencies; a drive focuses attention for a
bounded period. None is interchangeable with the others.

This document adds a proposed milestone integration boundary. It is not a replacement
specification for every existing README field. Existing lifecycle, source approval,
priority, parking, board generation and dispatch semantics remain controlled by
[`lifecycle-v1.md`](./lifecycle-v1.md) and the current implementation. Discovery at
Assay main `866a169aecab730e1acdaad7cbe57f9c41af8553` found these stream fields in
`statusgen/parse.go`: stream, status, priority, track, issues, external, tiering,
max-concurrent, serves, theme, owner, repo, board, traced, spec and mission.
No milestone field is supported there today. This draft MUST NOT change legacy validation.

## 2. Canonical ownership

The stream README at `docs/streams/<stream>/README.md` remains the stream index.
The owning repository identity and stream slug MUST resolve uniquely using existing
graph alias registry and existing root/cell identity facilities. The scoping source follows lifecycle §8; a milestone
proposal does not approve its source or authorize a parked stream to run.

For an opted-in future stream, canonical milestone records live beneath `milestones/`
per [`milestone-v1.md`](./milestone-v1.md), with full IDs `<cell>:<repo>:<stream>:m-<NN>`
distinct from brief references `<cell>:<repo>:<stream>:b-<NN>[@<version>]`
(brief §3.8). Colon-only elisions resolve against the declaration; legacy aliases
remain compatibility inputs, not canonical writes. Legacy
`<stream>/<NN>` labels remain readable and are deprecated only for new authoring
after coordinated reader support; canonical internal brief fullIds are unchanged. Discover records there rather than hand-maintain
an additional membership list in README frontmatter. Each milestone has one owning
stream; cross-stream contributors and prerequisite milestones are references to their
canonical owners. Moving work across repositories must preserve identity, provenance,
brief risk gates and acceptance history.

## 3. Authored and generated information

Authors maintain scope, milestone promises, contribution lists, capability requirements,
criterion coverage and the existing brief definitions. There MUST be one authored owner
for each relationship, as milestone §4 specifies.

Generated README views SHOULD show milestone title/version, supported outcome, contributor
wave span, remaining briefs, prerequisite holds, acceptance state and evidence links.
Do not hand-maintain milestone status, reverse membership or expanded dependency lists.
An implementation MUST identify generated regions and its single writer, reconcile them
with existing `board: generated` handling, and preserve authored narrative and evidence.
Missing source records are diagnostics, not permission to erase previous evidence.

Milestones MAY be independent tracks. Numbering is an identifier, not an implicit chain;
only declared prerequisites establish ordering. A broad maintenance stream MUST NOT
force unrelated capabilities behind a whole-stream completion gate. Streams with prose
outlines or no executable briefs MUST show uncovered planning work rather than invented
briefs, waves or completion percentages. Fixtures MUST be excluded from delivery totals.

## 4. Completion, parking and reporting

Stream status does not establish milestone acceptance; milestone acceptance does not
complete the entire stream. Existing archive eligibility and lifecycle rules continue.
Retained versions, identities and evidence MUST remain resolvable after archive or rename.
An unresolved archived source yields unknown, not satisfied.

Required work in a parked/paused stream remains visible in an outcome's prerequisite
view with the dispatch hold and decision needed. Reporting it does not activate it.
Unrelated streams are excluded from that outcome's work totals. A cross-stream plan MUST
report which configured roots were actually read and their revisions; partial coverage
cannot produce an unqualified readiness or critical-path claim.

## 5. Capability negotiation and migration

New required milestone semantics MUST be explicitly negotiated before use, including
when a consumer reads only the README. A tolerant legacy parser ignoring an unknown
frontmatter key is not compatibility. The implementation must select a recognized
stream capability envelope and prove old-reader refusal before writing operational
milestone declarations. This draft does not reserve a live README key or require legacy
streams to migrate.

Begin with an explicitly selected pilot; use existing spec approval and brief-authoring
routing. Author-spec supplies useful outcomes and criteria. Author-brief reconciles
existing work and maps contributors and actual start requirements. Generated stream
views and trace reports share the same resolver and semantics. Historical evidence,
finished work and active brief versions MUST be preserved through migration; unexplained
scope or prerequisite additions require a decision, not automatic bulk adoption.

The pilot MUST check independent outcomes sharing one wave, one outcome spanning waves,
cross-root references, parked dependencies, missing coverage, archived identities and
legacy streams unchanged. The milestone contract's §9 cases also apply.

## Allocated requirements envelope

[reader-capabilities-v1](reader-capabilities-v1.md) allocates the top-level
`reader-requirements` envelope and implements its pure assessment and named tree
preflights. No operational stream opts in through this change. This does not make
old binaries safe: coordinated readers, operation support and adoption qualification
remain prerequisites for operational milestone declarations under §5.
