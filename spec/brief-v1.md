# Brief v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen` v0.22.0

## 1. Scope

This document specifies the `brief-v1` schema: a self-contained scope-and-DoD unit for
agent-executable software work, with typed dependencies, derived risk gates, and
executable verification. A conforming implementation MUST produce and consume briefs
that satisfy every MUST in this document.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. File location and naming

A brief-v1 file MUST reside at `docs/streams/<stream>/brief-<NN>-<slug>.md`.

- `<stream>` is the owning stream's directory name.
- `<NN>` is a zero-padded, two-digit integer that MUST be unique within the stream.
- `<slug>` is a hyphenated, human-readable identifier.

## 3. Frontmatter (YAML)

A brief-v1 file MUST begin with a YAML frontmatter block delimited by `---`.
Every REQUIRED field listed in section 3.1 MUST be present; an empty value MUST still
carry the key. Section 3.2 lists fields that are OPTIONAL: a conforming implementation
MUST accept a brief that omits them, and MUST NOT treat their presence as an error.

### 3.1 Required and conditional fields

| Field | Type | Requirement | Description |
|-------|------|-------------|-------------|
| `brief` | string | REQUIRED | Typed ID `<stream>/<NN>`. MUST match the stream README table row. MUST NOT be a prose name. |
| `title` | string | REQUIRED | One-line human-readable title. |
| `wave` | integer | REQUIRED | 0 for no dependencies; N for depending only on briefs in waves `< N`. MUST be consistent with `depends`. |
| `depends` | array[string] | REQUIRED | Typed IDs only (`<stream>/<NN>`). MUST NOT contain prose arrows or narrative descriptions. MUST be the inverse of another brief's `unblocks`. |
| `unblocks` | array[string] | REQUIRED | Typed IDs this brief enables. MUST be the inverse of another brief's `depends`. |
| `effort` | string | REQUIRED | One of `S`, `M`, or `L`, indicating estimated effort and execution tier. |
| `gate` | string | REQUIRED | One of `model` or `human`. MUST be derived from `risk`: if ANY risk answer is `yes`, `gate` MUST be `human`; if all four are `no`, `gate` MUST be `model`. MUST NOT be chosen independently of `risk`. |
| `risk` | mapping | REQUIRED | Four boolean answers: `regulatory`, `customer`, `irreversible`, `sensitive-data`. Each MUST be present and MUST be `yes` or `no`. |
| `issues` | array[string] | REQUIRED | Tracker/issue IDs this brief closes. The key MUST be present; an empty list is valid when the brief closes no tracker item. |
| `schema` | string | REQUIRED | MUST be `brief-v1` for this specification version. Files without this field are exempt from validation (legacy opt-in). |
| `authored` | string | REQUIRED | ISO-8601 date (`YYYY-MM-DD`) and author identity. |
| `sources` | array[string] | REQUIRED | Provenance: scoping doc, finding (`F-<slug>`), or intake (`I-<slug>`) IDs. An empty `sources` list is an error. |
| `gate-why` | string | CONDITIONAL | Prose stating what makes this brief risky. REQUIRED when `gate` is `human` (i.e. any `risk` answer is `yes`); OPTIONAL otherwise. A risk-gated brief without it MUST be flagged. |
| `why` | string | RECOMMENDED | One to three lines a non-engineer could justify the work from. A conforming linter SHOULD emit a non-fatal notice when it is absent, and MUST NOT hard-error on its absence. |

### 3.2 Optional fields

These fields are OPTIONAL. Each is parsed by the reference implementation, so a brief
that omits the whole group is conforming, and a brief that carries one MUST have it
validated against the value set given.

| Field | Type | Requirement | Description |
|-------|------|-------------|-------------|
| `value` | string | OPTIONAL | Coarse worth signal — one of `low`, `med`, `high`. Absent is equivalent to `med`. Feeds the Next-up score (`lifecycle-v1.md` §5.2). An out-of-set value MUST be flagged. |
| `exec-tier` | string | OPTIONAL | One of `any` or `strong`. `strong` asserts the brief MUST NOT be dispatched to a cheap-tier implementer regardless of `effort` (section 6). An out-of-set value MUST be flagged. |
| `exec-tier-why` | string | CONDITIONAL | One-line rationale. REQUIRED when `exec-tier` is `strong`; a `strong` brief without it MUST be flagged. |
| `decision-issue` | integer | OPTIONAL | Tracker issue that carries the human sign-off for a `gate: human` brief. A conforming linter SHOULD emit a non-fatal notice when a `gate: human` brief is in flight (`in-progress`, `implemented`, or `verified`) without one, and MUST NOT hard-error on its absence. |
| `domain` | string | OPTIONAL | Cynefin classification of the work — one of `clear`, `complicated`, `complex`, or `chaotic`. Absent is equivalent to `complicated` (the safe Ordered default) at read time. An out-of-set value MUST be flagged. |
| `blocked-by` | string | OPTIONAL | Environment-blocked marker; the only defined value is `env` (the brief is blocked on infrastructure or environment). Absent means the brief is not blocked. An out-of-set value MUST be flagged. |
| `homed-in` | string | OPTIONAL | The `<owner>/<repo>` the brief's deliverable was re-homed to (a de-housing) — exactly one `/`, both sides non-empty, no whitespace. Absent means a normal in-repo brief. A malformed shape MUST be flagged; the value MUST NOT be checked against a repo allowlist (the shape is all a linter can validate locally). |
| `measures` | string | OPTIONAL | Name of the process queue this brief instruments (drain-before-instrument). The only wired queue is `verification-debt`. Absent means the brief is not an instrumentation brief. A present value MUST name a wired queue; a present-but-unrecognized name, or a present-but-empty value, MUST be flagged. |
| `parallel-streams` | array[mapping] | OPTIONAL | Declared shards of an intra-brief split. Each entry is a mapping with a REQUIRED `name` (string) and an OPTIONAL `files` (array of path globs the shard owns); no other key is permitted in an entry. Absent means one worker per brief. Only the entry SHAPE is validated in frontmatter — whether a declared split may actually be dispatched is decided by `statusgen shardcheck` against the file tree, not by the frontmatter linter. A declaration that parses is a request, not a permission. |

### 3.3 Frontmatter linter requirements

A conforming linter MUST flag any missing required field, any `gate` value inconsistent
with the `risk` answers, any `wave` value inconsistent with `depends`, any empty
`sources` list, any risk-gated brief missing `gate-why`, any out-of-set `value` or
`exec-tier`, and any `exec-tier: strong` brief missing `exec-tier-why`. It MUST also flag
any out-of-set `domain`, `blocked-by`, or `measures` value, any malformed `homed-in`
shape, and any `parallel-streams` entry that carries a key other than `name`/`files`, is
missing `name`, or gives `name` a non-string value. For every optional field in section
3.2 the rule is the same: an ABSENT field is never flagged, and a wrong *type* is a parse
error, while a present-but-out-of-set *value* is the semantic flag described here.

## 4. Body structure

### 4.1 Context section

The body MUST contain a `## Context` section. It MUST include:

- A `files:` line listing exact paths the implementer touches.
- A `facts:` block containing 3-5 project facts as `key: value` pairs (no narrative prose).

If the brief changes a SHARED VALUE (a party, identity, environment variable name,
configuration key, field meaning, wire/JSON format, or default — anything another
component reads), the Context section MUST include a `consumers:` line that greps
every reader and lists each with a disposition: `fixed-here`, `follow-up <stream>/NN`,
or `out-of-scope <reason>`.

### 4.2 Ground rules

The body MUST contain a `## Ground rules` section. At minimum, it MUST state:

- A prohibition on git push, workflow triggers, and mutating infrastructure commands
  unless explicitly instructed.
- A stop-at-`implemented` rule: the implementer does not set `verified` or `done`.
- A NEEDS_CONTEXT escalation rule for unclear or contradictory instructions.

### 4.3 Task section

The body MUST contain a `## Task` section defining what to build or change. It SHOULD
describe scope and intent rather than a keystroke-by-keystroke script.

### 4.4 Verify section

The body MUST contain a `## Verify` section with an executable table:

| # | Command | Expect |
|---|---------|--------|

- Every row MUST contain a literal command a non-implementer can run and an expected
  exit code or output match.
- Rows MUST NOT be prose-only assertions without a command.
- Prose deliverables (docs, articles) MUST use PRESENCE gates: checks that required
  elements exist (file, section, token). The Verify section MUST state that
  it gates presence, not quality; quality is owned by the review gate.
- A brief that adds a CHECK MUST include a mutation-test row: revert the fix or break
  the guarded condition, run the check, and confirm it fails.
- A brief that touches a shared lister, flag, or query MUST include a neighbour row:
  exercise the adjacent consumer of the shared code path (not the deliverable).
- A brief that changes a shared value MUST include a flow-level row that verifies
  the cross-component flow still completes end-to-end.

### 4.5 Evidence section

The body MUST contain a `## Evidence` section, recording actual runs of the Verify
table. Each Verify row MUST receive an entry: `(command, exit code, output line(s) or
hash, date, runner)`.

The section is written twice, by different parties, and `lifecycle-v1.md` §2.3 and §2.4
own the sequencing: the implementer records its own run on reaching `implemented`, and
an independent runner records a re-run on merged trunk before the brief may be marked
`verified`. A conforming linter MUST require Evidence content at `verified` and `done`,
and MUST require at least one entry attributed to a runner who is not the brief's
author — subject to the attribution-not-identity limit in `lifecycle-v1.md` §7.1.2.

### 4.6 Review section

The body MUST contain a `## Review` section recording the gate type (from frontmatter)
and the reviewer's verdict and date.

## 5. Structural rules

### 5.1 Load-bearing facts in islands

Facts a script or reviewer depends on MUST reside in frontmatter, tables, or Verify
rows — not solely in prose. Prose motivates but MUST NOT be the sole carrier of a
dependency, identifier, or machine-checkable constraint.

### 5.2 Typed IDs

References to briefs, findings, intake entries, and retro entries MUST use typed IDs
(`<stream>/<NN>`, `F-<slug>`, `I-<slug>`). References MUST NOT use fuzzy names (e.g.,
"the pricing brief") or arrow notation (`←12a`).

### 5.3 Self-containment

A brief MUST be self-contained: executing it MUST NOT require reading another brief.
If knowledge from another brief is needed, it MUST be linked in `facts:` or `depends:`.

### 5.4 Dependency and wave consistency

`depends` and `unblocks` entries MUST be typed IDs and MUST be mutual inverses.
`wave` MUST be derivable as `max(dependency wave) + 1`, with wave 0 for no dependencies,
and MUST be consistent with `depends` at all times.

**Enforcement status of these two rules.** Both are authoring rules that the reference
implementation checks only partly, and section 9.2 says which:

- *Mutual inversion* has no linter clause at all in section 9.2. The reference
  implementation resolves each `unblocks` entry to a real brief but never checks that
  the named brief lists this one in its `depends`. An implementation MUST NOT report
  inversion as checked.
- *Wave-versus-depends* is a document MUST here, but the reference implementation
  reports a dependency pointing at an equal-or-later wave as a non-fatal notice rather
  than an error — a deliberate downgrade, so that a wave mis-numbering does not block a
  whole board. Section 9.2.3 records the licence to do this.

### 5.5 Data-first decomposition

A brief implementing two distinct roles SHOULD be split into two. A brief whose Task
section exceeds approximately five steps or touches more than two subsystems SHOULD
be split at authoring time.

## 6. Execution tiering

Effort keys the execution tier:

- Effort S MAY run inline at the session's tier.
- Effort M and L MUST be planned at the author's tier and SHOULD be dispatched to
  cheap-tier implementers behind the verify and review gates.

The optional `exec-tier` field (section 3.2) overrides the effort-derived default.
`exec-tier: strong` asserts that the brief MUST NOT be dispatched to a cheap-tier
implementer whatever its `effort` — the work needs judgment the tier cannot supply.
A session running below that tier MUST hand the brief back rather than execute it.
`exec-tier` only ever *tightens* the effort-derived default; it MUST NOT be used to
widen a brief down-tier.

## 7. Mid-flight changes

A change to a brief after authoring MUST be routed by one test: does the brief's
Verify table change?

- If the Verify table does NOT change: the change MAY proceed without ceremony.
- If the Verify table DOES change: the brief MUST be amended in the same commit and
  demoted if it was past `implemented`.
- If no owning brief exists: a one-line intake entry or a new small brief MUST be
  created; untracked drive-by changes MUST NOT be enacted.

## 8. Three-state instrument invariant

Every desk instrument MUST report in three states, never two:

- `checked-clean` — the check passed.
- `checked-failed` — the check found a problem.
- `could-not-check` — the check could not run.

A conforming implementation MUST NOT produce a binary pass/fail instrument. In
particular, an instrument that could not execute MUST NOT report the same result as one
that executed and found nothing.

The reference implementation does not yet satisfy this requirement everywhere; the known
violations are listed in `spec/README.md` § "Known divergences from the reference
implementation" rather than left for an adopter to discover.

## 9. Conformance

### 9.1 Brief-v1 conformance

A conforming brief-v1 file:

1. MUST reside at a path matching `docs/streams/<stream>/brief-<NN>-<slug>.md`.
2. MUST have a YAML frontmatter block with every field listed in section 3.
3. MUST have `schema: brief-v1`.
4. MUST derive `gate` from `risk` answers.
5. MUST have `wave` consistent with `depends`.
6. MUST have non-empty `sources`.
7. MUST contain `Context`, `Ground rules`, `Task`, `Verify`, `Evidence`, and `Review`
   sections.
8. MUST use typed IDs for all references.
9. MUST have an executable Verify table with literal commands and expected outputs.

### 9.2 Linter conformance

A conforming linter MUST:

1. Flag any missing required frontmatter field.
2. Flag any `gate` value inconsistent with `risk` answers.
3. Flag any `wave` value inconsistent with `depends`. This one MAY be reported as a
   non-fatal notice rather than an error; the reference implementation does so
   deliberately, so that one mis-numbered wave does not fail a whole board.
4. Flag any empty `sources` list.
5. Flag any missing required body section (the six of section 4). **Diverges** — the
   reference implementation checks only `## Verify` (always) and `## Evidence` (at
   `verified`/`done`); a brief stripped of `Context`, `Ground rules`, `Task`, and
   `Review` passes its lint. See `spec/README.md` § "Known divergences from the
   reference implementation".
6. Flag any prose reference where a typed ID is expected.
7. Flag any Verify row lacking a literal command. **Diverges** — the reference
   implementation checks only that a Verify table exists with at least one row whose
   Command and Expect cells are non-empty, and says so in its own diagnostic ("a
   structure/presence check, not a judgement of the table's content"). A prose-only
   row passes. See the divergence section.
8. Flag any brief claiming `verified` or `done` status with an implementer-authored
   Evidence section.
9. Flag any brief with an unresolved finding against it.
10. Report every flag as a machine-parseable diagnostic naming the file and the rule.
    A diagnostic SHOULD additionally carry the line number. **Diverges** — the
    reference implementation emits no line numbers at all, and carries a rule
    reference on only some diagnostics. See the divergence section.

Items 5, 7, and 10 are the clauses where the reference implementation is presently
non-conforming. They are stated as MUSTs because the specification governs; they are
labelled here so an adopter is not surprised by a green lint that did not check them.
