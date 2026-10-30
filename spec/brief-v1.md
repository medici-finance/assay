# Brief v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen` v1.0.67

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
| `exec-tier` | string | OPTIONAL | One of `any` or `strong`. `strong` asserts the brief MUST NOT be dispatched to a cheap-tier implementer regardless of `effort` (section 6). The value is DERIVED, not chosen: `strong` when any of four questions is answered yes — (a) Does the Task require design decisions the facts do not fully pre-specify? (b) Does correctness depend on cross-component or cross-artifact reasoning? (c) Is it code where a subtle implementation error survives the brief's own tests (auth, funds, concurrency, safety plumbing)? (d) Is this a design brief raised by an error-class trigger (a recurring defect class, rather than one symptom, has accrued enough counted instances or merged fixes to be owed a design)? — and `any` otherwise. An out-of-set value MUST be flagged. |
| `exec-tier-why` | string | CONDITIONAL | One-line rationale. REQUIRED when `exec-tier` is `strong`; a `strong` brief without it MUST be flagged. |
| `decision-issue` | integer | OPTIONAL | Tracker issue that carries the human sign-off for a `gate: human` brief. A conforming linter SHOULD emit a non-fatal notice when a `gate: human` brief is in flight (`in-progress`, `implemented`, or `verified`) without one, and MUST NOT hard-error on its absence as such. A change that MOVES, relabels or drops a `gate: human` brief is a different matter: without a decision issue and a recorded ruling that change is refused by the decision-gate hold (`lifecycle-v1.md` §4.5). |
| `ruling` | string | OPTIONAL | Link to the comment on `decision-issue` that records the human ruling, as an issue-comment URL `https://github.com/<owner>/<repo>/issues/<N>#issuecomment-<ID>`. A wrong TYPE is a parse error. The link's grammar, and what it must resolve to (an unedited comment by a human login on the brief's own decision issue, naming the brief by its board id), are the decision-gate hold's (`lifecycle-v1.md` §4.5); absence is never flagged on its own. |
| `independence-override` | string | OPTIONAL | A recorded override, naming a human, of the committer-identity independence check, for one brief whose authoring commit and every Evidence commit share one git identity in a repository with several identities (a legitimate post-verification re-touch). Grammar: `human:<name> YYYY-MM-DD <reason>`, the human token first, the date a real calendar date and the reason non-empty. A wrong TYPE is a parse error. It is honored ONLY when `<name>` resolves through the CURRENT human-login map, is not the brief's own author, the brief is not in a declared fixture corpus, the committed line was last written by a commit whose git author identity (the commit object's own email, not a mailmap-resolved one) differs from the one it excuses, the date is no more than one day after that commit's committer date (one day of slack for timezones), and the Evidence section has not changed since that commit. An uncommitted value is not honored. Otherwise the independence failure stands and names why the override was refused; an honored override is reported as a notice, never silently, and the notice says that lint does not corroborate the named human's authorization. These offline conditions read git metadata, which the committer sets: they refuse an override committed under the excused identity's own author email, but they do not tie `<name>` to the person who made the commit. That binding is the network corroboration lane, which checks the `human:<name>` token on an added override line like any other stamp, and it holds only where that lane runs on the change that adds the override. |
| `domain` | string | OPTIONAL | Cynefin classification of the work — one of `clear`, `complicated`, `complex`, or `chaotic`. Absent is equivalent to `complicated` (the safe Ordered default) at read time. An out-of-set value MUST be flagged. |
| `blocked-by` | string | OPTIONAL | Environment-blocked marker; the only defined value is `env` (the brief is blocked on infrastructure or environment). Absent means the brief is not blocked. An out-of-set value MUST be flagged. |
| `homed-in` | string | OPTIONAL | The `<owner>/<repo>` the brief's deliverable was re-homed to (a de-housing) — exactly one `/`, both sides non-empty, no whitespace. Absent means a normal in-repo brief. A malformed shape MUST be flagged; the value MUST NOT be checked against a repo allowlist (the shape is all a linter can validate locally). |
| `measures` | string | OPTIONAL | Name of the process queue this brief instruments (drain-before-instrument). The only wired queue is `verification-debt`. Absent means the brief is not an instrumentation brief. A present value MUST name a wired queue; a present-but-unrecognized name, or a present-but-empty value, MUST be flagged. |
| `satisfies` | array[string] | OPTIONAL | The requirements this brief was written against, as requirement references — `REQ-<slug>` in-repo, or `<alias>:REQ-<slug>` cross-repo through the `docs/streams/graph-repos.yaml` alias registry (`registers-v1.md` §6.5). Absence MUST NEVER be flagged on any brief. A present entry that does not match the grammar MUST be flagged; a wrong TYPE is a parse error. A conforming linter MUST emit a NOTICE for a brief carrying the key. The citation feeds the corpus-wide traceability checks §6.5 defines: a `satisfies:` naming an in-repo requirement that does not exist is a `dangling-satisfies` PROBLEM, and a forward brief in a `traced:` stream that cites nothing is an advisory `untraced-brief` NOTICE (§3.3). |
| `design` | string | CONDITIONAL | The design-decision record this brief was approved against, as a typed reference `DR-<slug>` into the DECISIONS register (`registers-v1.md` §7). REQUIRED for a **risk-gated** brief in the design-approval gate's scope (`lifecycle-v1.md` §4.4) once it is at `in-progress` or later; OPTIONAL — and never flagged when absent — otherwise. A present value that is not a valid `DR-<slug>` reference, or that dereferences to no record in the register, MUST be flagged; a wrong TYPE is a parse error. The gate is grandfathered (§4.4), so absence is flagged only for briefs in the gate's live scope, never for grandfathered legacy briefs. |
| `budget` | scalar | OPTIONAL | What the brief is sized to spend, beside `effort`, WITH its unit: `<amount>[k\|M] <unit>` (the multiplier is case-sensitive: `k` thousand, `M` million), the unit being `tokens` or a three-letter currency code (`400k tokens`, `25 USD`). Absent means no budget checkpoint applies and MUST NOT be flagged. A present value without a unit (`budget: 400`), with an unrecognised unit, or with a non-positive amount MUST be flagged by value; a bare number is read as the scalar it is so it reaches that flag rather than failing as a type error. A non-scalar is a parse error. |
| `outcome` | string | OPTIONAL | What the brief is meant to move: a requirement id from the requirement register (`registers-v1.md` §6) — `REQ-<slug>` in-repo or `<alias>:REQ-<slug>` cross-repo, the same grammar as `satisfies` — or the literal `none`. The register's ids ARE the vocabulary; there is no second one. A value that is neither `none` nor a requirement reference MUST be flagged, and so MUST a well-formed in-repo id the register does not define (an unknown outcome id). A cross-repo id names a register the offline linter cannot read and is a could-not-check NOTICE, never a pass. A wrong TYPE is a parse error. Absence is never a PROBLEM; a conforming linter SHOULD emit an advisory NOTICE for a brief authored after the key's introduction that carries neither an id nor `none` — `none` is a decision, absence is not. |
| `outcome-revision` | string | OPTIONAL | The approved product target revision of the brief's `outcome` that the brief was selected against (statusgen/23): a revision name matching `^[a-z0-9][a-z0-9._-]*$`, such as `r1`. It qualifies `outcome` and never changes its grammar; there is no `REQ@revision` form. A present value MUST be flagged when the brief carries no `outcome` or `outcome: none` (a revision binds no requirement) or when it does not match the revision grammar. A wrong TYPE is a parse error. Absence is never flagged: a brief without it is **unbound**, which a [context-handoff-v3](./context-handoff-v3.md) receipt reports as an unknown product assessment, never as aligned. |
| `parallel-streams` | array[mapping] | OPTIONAL | Declared shards of an intra-brief split. Each entry is a mapping with a REQUIRED `name` (string) and an OPTIONAL `files` (array of path globs the shard owns); no other key is permitted in an entry. Absent means one worker per brief. Only the entry SHAPE is validated in frontmatter — whether a declared split may actually be dispatched is decided by `statusgen shardcheck` against the file tree, not by the frontmatter linter. A declaration that parses is a request, not a permission. |
| `split-from` | string | OPTIONAL | The `<stream>/<NN>` id of the brief this brief was split off from — the explicit parent for the split-flag conservation gate (§3.4), used when the parentage is NOT encoded in the numbering (a split across streams, or a renumbered child). Absent means the brief is not a declared split child. A value that is not a `<stream>/<NN>` in-repo brief reference, or that names the brief itself, MUST be flagged; a wrong TYPE is a parse error. A value that does not resolve to a brief in the tree is a `could-not-check` NOTICE (the parent may have been retired in the same change), never a silent pass. |

### 3.3 Frontmatter linter requirements

A conforming linter MUST flag any missing required field, any `gate` value inconsistent
with the `risk` answers, any `wave` value inconsistent with `depends`, any empty
`sources` list, any risk-gated brief missing `gate-why`, any out-of-set `value` or
`exec-tier`, and any `exec-tier: strong` brief missing `exec-tier-why`. It MUST also flag
any out-of-set `domain`, `blocked-by`, or `measures` value, any malformed `homed-in`
shape, any `budget` without a unit, any `outcome` that is neither `none` nor a registered
requirement id, any `outcome-revision` that binds no outcome or is not a revision name, and any `parallel-streams` entry that carries a key other than `name`/`files`, is
missing `name`, or gives `name` a non-string value. For every optional field in section
3.2 the rule is the same: an ABSENT field is never flagged, and a wrong *type* is a parse
error, while a present-but-out-of-set *value* is the semantic flag described here.

It MUST also flag any `satisfies:` entry that does not match the requirement-ref grammar,
which is exactly two forms and no others:

```
REQ-<slug>              in-repo requirement; <slug> is 10–20 chars of [a-z0-9-],
                        starting and ending alphanumeric (registers-v1.md §3.4)
<alias>:REQ-<slug>      cross-repo; <alias> MUST resolve in the existing
                        docs/streams/graph-repos.yaml registry (schema graph-repos-v1)
```

An unresolvable alias MUST be flagged: the registry is a closed set, and the same registry
serves the dependency-graph reference grammar — a conforming implementation MUST NOT add a
second one for requirements. The linter MUST NOT flag an absent `satisfies:` (no brief is
required to cite a requirement), and it MUST emit a NOTICE for a brief that carries the key,
so that "parsed and traced" is distinguishable from "silently ignored".

Beyond the grammar, `satisfies:` now feeds the corpus-wide traceability checks that
`registers-v1.md` §6.5 defines (they were reserved at the schema's first version and landed
advisory-first, per §4.5):

- **`dangling-satisfies`** — a `satisfies:` naming an IN-REPO `REQ-<slug>` that this root's
  register does not define is a hard PROBLEM. The register is append-only (§3.1/§3.3), so an
  in-repo id no entry defines can only be a typo or a deleted entry. A cross-repo
  `<alias>:REQ-<slug>` names a register in another repo the offline linter cannot read, so it
  is could-not-check, never dangling.
- **`untraced-brief`** — a brief that is `in-progress` or later, in a stream whose README
  declares `traced: true`, and that names no `satisfies:`, raises an advisory NOTICE. Absent
  the stream opt-in the check never fires — a corpus-wide untraced sweep over legacy briefs
  that predate the register is noise (§4.5).

The companion `orphan-requirement` advisory check lives on the register side (§6.5).

### 3.4 Split-flag conservation

Splitting a brief is authoring, and a split MUST NOT downgrade risk. A brief that is a
split child MUST carry, for `gate` and for each of the four canonical `risk` answers, a flag
at least as strict/high as the brief it was split from:

```
gate  = the stricter of (parent, child)   — human >= model
risk  = MAX(parent, child) per key        — yes >= no, for each of
        regulatory, customer, irreversible, sensitive-data
```

A child that is STRICTER than its parent (an escalation) is always conforming and MUST NOT
be flagged; only a WEAKER flag is a downgrade. A conforming linter MUST flag a downgrade as a
hard PROBLEM. The rule fails safe toward more gating — because risk is a property of what a
change DOES, not of the size of its diff, the small "mechanical" child of a split is exactly
where a human gate is most likely to be dropped.

The parent is resolved from whichever signal is present:

- **Numeric-stem lineage** — the split convention `02` → `02a`, `02b`, `02c`: a lettered
  shard shares the numeric stem of the brief it came from. When the un-lettered parent
  (`02`) is present it IS the floor. When the parent was retired in the same change, the
  strictest SIBLING shard stands in as the floor, so a downgraded shard is still caught by a
  faithful one.
- **A declared `split-from`** (§3.2) — the explicit parent, for lineage the numbering does
  not encode. A `split-from` that does not resolve in the tree is a `could-not-check` NOTICE,
  never a silent pass.

This is a conservation check, not an authorization one: it never licenses lowering a flag,
and a genuinely clean split whose risky work all went to a sibling conserves by keeping the
child's flags UP, not down.

### 3.5 Schema versions

A brief's `schema:` value names the frontmatter contract it is validated against. Each version
is a strict superset of the one before it; no field is removed or changed in meaning.

| Schema | Adds | Machine-readable contract | First `statusgen` release that reads it |
|--------|------|---------------------------|------------------------------------------|
| `brief-v1` | this document's fields (§3.1, §3.2) | `schemas/brief-v1.json` | v0.9.0 (the earliest published release) |
| `brief-v2` | the hierarchical `brief:` id `<cell>:<repo>:<stream>:<NN>`, `version:`, and the identity/graph keys `id`, `supersedes`, `gates`, `feathers`, `verify` | `schemas/brief-v2.json` | v1.0.0 |
| `brief-v3` | the withdrawn-status keys `withdrawn-date`, `withdrawn-reason`, `superseded-by` (§3.6) | `schemas/brief-v3.json` | v1.0.50 |

A `statusgen` release declares the versions it reads, and the first release of each, in
machine-readable form: `statusgen --version --json` prints
`{"version": …, "briefSchemas": […], "briefSchemaFirstRelease": {…}}`. The bare
`statusgen --version` stays a single line holding the tag, because pin checks compare it
verbatim.

Schema evolution fails closed. A file with no `schema:` key at all is a legacy brief and is
exempt (§3.1). A `schema:` value in the brief family (`brief-v<N>`) that the binary does not
recognize MUST NOT fall into that exemption. A conforming linter MUST refuse it loudly,
naming the schema the file needs, the versions the binary supports, where the first
supporting release is listed (this table), and the `statusgen` the repository pins in
`.assay-versions`. A `schema:` value outside the brief family (`contract-v1`,
`placeholder-v1`, …) names a different document kind and stays out of this parser's scope.

### 3.6 Withdrawn briefs (`brief-v3`)

A brief that a later decision retires, or that is merged into another brief, is
**withdrawn** (`lifecycle-v1.md` §2.0a). It is not deleted: the file stays, for history and
links. A withdrawn brief MUST be `schema: brief-v3` and MUST carry:

| Field | Type | Requirement | Description |
|-------|------|-------------|-------------|
| `withdrawn-date` | string | REQUIRED when withdrawn | `YYYY-MM-DD` date of the withdrawal. |
| `withdrawn-reason` | string | REQUIRED when withdrawn, non-empty | One sentence: why, citing the decision, and what (if anything) survives where. |
| `superseded-by` | array[string] | OPTIONAL | The successor brief(s): `<stream>/<NN>` for this repository, or the global brief id `<cell>:<repo-alias>:<stream>:<NN>` (the brief-v2 `brief:` form) for another one. `[]` means won't-fix: no successor. For a duplicate merged into another brief it names the survivor. |

The stream README row carries the bare Status token `withdrawn`; its link text keeps the
brief's original title. Unexecuted work leaves Verified and Reviewed as `—`; any existing stamps, Evidence, receipts and review decisions MUST be preserved. The body keeps a
banner directly under the H1:

```
> **Withdrawn YYYY-MM-DD — superseded by <successor>.** <reason>. Not executable.
```

The withdrawal keys (including optional `withdrawal-context`, below) belong to `brief-v3` only. On a `brief-v1` or `brief-v2` file they are
unknown keys and MUST be tolerated as written — not parsed, not flagged — because boards
that predate brief-v3 already carry `superseded-by:` as free prose, and a linter upgrade
MUST NOT turn a previously valid board red. The one rule that crosses versions is the
status itself: a conforming linter MUST flag a `withdrawn` row whose brief is not
`brief-v3`, or that has no typed brief file at all.

On a `brief-v3` file a conforming linter MUST flag, as hard problems:

- a withdrawn brief without `withdrawn-date` (or with one that is not a `YYYY-MM-DD`
  calendar date) or without a non-empty `withdrawn-reason`;
- a `superseded-by` that is not a list of strings;
- any withdrawal key on a brief whose status is not `withdrawn`;
- a same-repository `superseded-by` entry that does not resolve to an existing brief, an
  entry naming the brief itself, and an entry in neither ref form.

A global `superseded-by` id is syntax-checked, and resolved only when the root it names is
configured in the run (by that root's registry `self:` key, or by the `<cell>:<alias>`
prefix of its own brief-v2 ids); an unconfigured root raises a could-not-check NOTICE (syntax checked only). A successor cycle among configured roots is a hard problem. A chain may pass through another withdrawn brief but must terminate; a terminal chain or empty successor list does not prove that any surviving obligation was delivered. A brief whose
`depends:` names a withdrawn brief MUST get a NOTICE naming the successor, so the edge is
re-pointed: a withdrawn brief never satisfies a dependency.

#### Revision-bound withdrawal context

A brief-v3 withdrawal MAY point at one shared amendment record:

```yaml
withdrawal-context:
  path: docs/changes/example/handoff.json
  digest: "sha256:<64 lower-case hex characters>"
```

`path` MUST be a clean repository-relative regular file within the repository (including
symlink resolution), at most 4 MiB. The object is closed: `path` and `digest` are required,
and no other key is allowed. `digest` is the canonical decoded handoff digest, not a raw
file checksum. Lint strictly decodes context-handoff-v1 or
[context-handoff-v2](context-handoff-v2.md), checks the digest, requires approval of the
record's exact target revision, and finds this brief's retire/replace/split obligation.
Its normalized `replaced_by` list MUST match `superseded-by`. Observed active work on the
brief MUST have an acknowledged treatment, including cancellation/reconciliation. An
executed impact (including in-progress work) or a brief carrying Evidence MUST have a preserved-history entry.

One handoff can cover many briefs. This pointer does not duplicate the target, approval,
claim, history or metric payload into every brief. A valid record is internally consistent;
it is not proof of discovery completeness, a stopped worker, or a delivered successor.
Current claims and PR heads must be re-read at the amendment boundary. The existing
handoff/manifest contract continues to own patch and follow-up binding.

For rollout, missing `withdrawal-context` is advisory on human-gated or historically
executed work, rather than a new failure on existing branches. Supplied contexts MUST
validate. Withdrawal MUST NOT clear Evidence, Verified/Reviewed stamps or merged-work
debt. The witness comparator MUST still surface known execution (including a merged PR
whose current fold is unknown); only an unstarted `todo` witness needs no drift NOTICE.
Use `statusgen withdrawal-audit` and the [rollout runbook](../docs/withdrawal-rollout.md)
to reconcile open implementation and evidence PRs. A missing context never suppresses
those checks, and a supplied context never grants merge, closure or verification authority.

### Product-target binding (statusgen/23)

The [architecture amendment](../docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md)
routed optional `outcome-revision` and versioned context bindings to statusgen/23, which
ships them together: the §3.2 key, the brief reader and linter, and the
[context-handoff-v3](./context-handoff-v3.md) receipt that compares each binding with the
current approved product target. The `outcome` and `satisfies` grammar is unchanged;
missing legacy bindings remain explicitly unbound.
Observation syntax belongs to the continuing-operations impact extension, not a second
brief lifecycle or a universal observation mandate.

### 3.7 Proposed delivery milestone integration (not shipped)

The draft [`milestone-v1.md`](./milestone-v1.md) defines a separate, versioned outcome
contract. It does not extend the supported schema table above. A future negotiated
brief schema (provisionally `brief-v4`, not allocated or implemented by this change)
would add an optional `requires-milestones: array[string]` containing exact versioned
milestone references per milestone §2: `<stream>:m-<NN>@<version>` for the current
root, `<repo>:<stream>:m-<NN>@<version>` within the declaring cell, or
`<cell>:<repo>:<stream>:m-<NN>@<version>` for explicit qualification.
These use the new typed milestone-reference grammar, not the existing brief-only
`qualifiedRef` parser; brief §3.8 defines `:b-<NN>[@<version>]` and compatibility
with the existing internal brief `fullId` `<cell>:<repo>:<stream>:<NN>`. Its presence would mean the capability is required before
the consuming brief starts; absent/empty means no additional milestone start requirement.
Unknown, unreadable or unaccepted required capabilities cannot be treated as ready.
The brief Context must identify the capability it uses and the relevant supported
environment/profile; an acceptance for a different subject cannot satisfy that need.

`depends` and `unblocks` continue to name briefs; do not insert milestone IDs into their
existing grammar or into `outcome`/`satisfies`, which name requirements. Existing brief
waves continue to describe brief dependencies. The additional capability gate is shown
separately and participates in mixed-graph cycle checks; a wave alone is not readiness.
The implementation must recheck eligibility at dispatch/pickup using current capability
observations, without retroactively rewriting completed execution or evidence.

Milestone contribution membership is authored only in the milestone's `contributes`
list; reverse brief membership is generated. Contributing to a milestone does not
inherit that milestone's final-acceptance prerequisites as brief start dependencies.
A brief may serve several milestones. Existing Task/Verify amendment, version, risk,
design and review obligations still apply to edits prompted by a milestone change.

This proposed field MUST NOT be added to a legacy operational brief merely because its
parser tolerates extra keys. Schema negotiation, dispatch refusal, all graph readers and
report consumers must ship together before operational use (milestone §10).

### 3.8 Proposed colon-only brief references and deprecation (not shipped)

For new authoring with negotiated reader support, the canonical brief ID is
`<cell>:<repo>:<stream>:b-<NN>`, for example `example-cell:tools:build:b-11`.
A brief definition's `brief` field uses that full, unversioned ID; its existing
positive integer `version` field records the Task/Verify revision separately.
References may optionally pin that revision as `:b-<NN>@<version>`.
Milestone references use `:m-<NN>@<contract-version>` instead. The prefixes name
different work types, and the two version counters have different meanings.

The future typed brief adapter accepts these colon-only forms:

| Written reference | Expanded identity |
|---|---|
| `<cell>:<repo>:<stream>:b-<NN>[@<version>]` | Explicit owning cell, repo and stream. |
| `<repo>:<stream>:b-<NN>[@<version>]` | Declaring cell; explicit repo and stream. |
| `<stream>:b-<NN>[@<version>]` | Declaring cell/repo; explicit stream. |

Brackets above indicate an optional suffix; they are not literal syntax. Names,
repo-alias lookup and brief-number grammar follow context-handoff §2. Expansion uses
the declaration's actual cell/root, never a fuzzy search. A present version is a
positive integer and MUST resolve to that exact authored brief revision. A missing
or unreadable requested revision is unknown; a newer revision cannot silently replace
it. With no suffix, resolve the brief at the selected source snapshot and record its
observed version/commit before execution or assessment. No implicit global latest.
The revision selector is not part of logical work identity and MUST NOT allocate a
second task, claim or workflow. Conflicting revision requirements must be surfaced.

### Compatibility and canonical writes

Existing `<stream>/<NN>`, `<stream>:<NN>`, `<repo>:<stream>:<NN>`,
`<cell>:<repo>:<stream>:<NN>`, `<repo>:<stream>/<NN>` and
`<cell>:<repo>:<stream>/<NN>` references remain compatibility inputs under their
existing resolution rules. Earlier prefixed draft slash labels (`<stream>/b-<NN>`
and `<cell>:<repo>:<stream>/b-<NN>`, optionally versioned) may also be read by the
new negotiated adapter. New writes emit colon-only prefixed IDs/references; legacy
unprefixed and slash forms are **deprecated for new authoring after adoption**, not
invalid historical records. No compatibility read may reinterpret a milestone as a brief.

All aliases normalize to the same existing permanent UUID/work identity. The current
internal brief `fullId` `<cell>:<repo>:<stream>:<NN>` used by context-handoff and
verification-domain remains an internal compatibility representation, not a second
work item. Existing `qualifiedRef` and verification-domain APIs are not changed by
this draft: they still reject `b-`, `m-`, slash and revision suffixes. The future
adapter resolves type, identity and revision, then supplies the existing normalized
ID plus separately bound version/subject to those APIs. Never feed new syntax to an
old parser and assume it understood it. Unknown/ambiguous aliases refuse or remain
unknown; aliases resolving to different UUIDs are a collision, not duplicates to merge.

Preserve brief versions, lifecycle, Evidence, source locators and historical aliases.
Label conversion alone does not bump Task/Verify version or establish completion.
The `brief-<NN>-<slug>.md` filename convention is unchanged. No bulk rename, UUID
reminting or historical Evidence rewrite is required.

The new references require a future negotiated brief schema (provisionally `brief-v4`,
subject to allocation alongside §3.7). Update generator/scaffolding, graph/dependency
readers, board/eligibility, PR trailers/claims, workflow/handoff adapters, the
human-gate ruling-link/board-ID check (lifecycle §4.5), and history/export/import
consumers together. Old/new aliases must bind the same brief in human rulings;
normalization must not bypass or weaken a human gate. Advertise support and refuse required
unsupported semantics. This draft adds no supporting release or operational YAML.

Authoring skills MUST write the new colon-only form once the pinned generator and all
required consumers support it. Until then capture proposed IDs in scoping prose;
executable briefs retain the pinned reader's supported legacy form with the limitation
disclosed. The pilot must show old/new aliases deduplicate, optional revision pins bind
exactly, unversioned reads bind a source snapshot, unknown versions stay unknown,
unsupported readers refuse, cross-root elisions resolve, and `m-` never becomes brief work.

## 4. Body structure

### 4.1 Context section

The body MUST contain a `## Context` section. It MUST include:

- A column-zero `files:` field listing repository-relative paths the implementer touches, as
  bare or backtick-quoted list items or inline comma-separated declarations.
  The following grammar governs verification input receipts, coverage, admission
  binding and wake scheduling; other coordination-tag readers are outside this contract.
  Literal files/directories, segment globs (`*`, `?`, character classes) and
  bounded finite brace alternatives are supported. A declaration ends before
  its em-dash description or a colon after a quoted path; descriptions and
  parenthesized annotations do not declare additional paths. The field ends at
  the next blank line, known column-zero Context field key or column-zero section
  heading. An indented `## ` line inside Context MUST refuse, including while a
  files list is paused after a blank. A field key may carry a parenthesized
  annotation, such as `facts (date):`. HTML comments within a declaration or its
  description, and raw HTML blocks or comments while awaiting the next field,
  MUST refuse ambiguous section containment. Trailing whitespace and CRLF MUST
  NOT count as indentation. Unknown keys, loose list/quote/code
  continuations after the blank, and unsupported markers inside Context MUST
  refuse rather than drop declarations. Outside Context, field-shaped `files:`
  declarations refuse; ordinary prose and inline references remain prose.
  Hyphen-minus/en-dash description separators are unsupported. Examples inside fences do not
  declare paths. A region skipped as fenced MUST NOT contain a section heading
  or files marker; such examples refuse conservatively. Inside Context, an
  indented opener or one within/after the files field before its next known field
  MUST refuse. This bounded grammar does not infer list/HTML container ends.
  Fence openers/closers MUST have at most three leading ASCII spaces;
  a backtick opener's info string MUST contain no backtick. Ambiguous fence-shaped
  Context lines refuse. A closing fence MUST use the same character and be at least as
  long as its opener; an unclosed fence refuses. Duplicate Context sections/fields
  are ambiguous and refused.
  Abbreviations, placeholders, recursive `**` and malformed/escaping paths MUST
  be reported explicitly, never interpreted as absent deliverables. See the
  [declaration and observation contract](../docs/declared-input-scope.md).
- An OPTIONAL column-zero `inputs:` field narrowing a Verify row's witness scope to
  the literal repository-relative files and directories (a trailing `/`) its Commands read. It
  reads a strict subset of the `files:` grammar: bare or backtick-quoted entries as an inline
  value or `- ` list items, separated by commas, semicolons or spaces, ended by an em dash;
  globs, braces, backslashes, absolute paths and `..` make the whole declaration unreadable,
  and an unreadable or absent `inputs:` means the witness scope is the whole tree. A Command
  that names a repository path outside the declared set is refused by the witness reader
  (`witness-input-undeclared`) and flagged by `--lint`. STATUS.md and `docs/streams/**` are
  never inputs. See [declared-input-scope](../docs/declared-input-scope.md).
- A `facts:` block containing 3-5 project facts as `key: value` pairs (no narrative prose).

If the brief changes a SHARED VALUE (a party, identity, environment variable name,
configuration key, field meaning, wire/JSON format, or default — anything another
component reads), the brief MUST carry a `consumers:` frontmatter field (a schema field in
`schemas/brief-v1.json` and `schemas/brief-v2.json`) that greps every reader and lists each
with a disposition: `fixed-here`, `follow-up <stream>/NN`, or `out-of-scope <reason>`.

If the brief's Task creates a new component, service, or tool — or substantially
changes where an existing component's logic lives (extracting or dissolving a domain
layer, adding an external adapter or entrypoint, moving logic across a process or
trust boundary) — the Context section MUST include a `layering:` line. In one or two
lines, name the chosen structure, the meaningful rules and external effects it
separates (or why no extracted boundary is warranted), and the current reason for
that choice. Reference the Task and Verify rows that implement and check the boundary;
put detailed contracts there rather than expanding the Context into a design document.

For that same scope, the Task MUST identify the affected components and cite their
existing architectural and semantic contracts. An existing cell architecture map
(a cell-level inventory linking components to their contracts) MAY aid discovery;
no cell taxonomy (a set of classification categories) or new registry is required.
The brief MUST reuse the existing owners of facts, policy and operations, or explicitly
propose an ownership migration with its compatibility and consumer routing. The `design-fit.owner` and
`design-fit.contract` declarations below remain the ownership record; classification
is a discovery aid, never a grant of ownership, authorization, eligibility or execution
authority. A stream can span categories while its implementation must still respect
each component's declared interfaces.

Choose the simplest structure that keeps meaningful rules independent of changing
infrastructure. These are defaults, not an exhaustive set of permitted architectures:

- `domain-core` — prefer an isolated core when there are meaningful rules that should
  remain unchanged as external systems change. Adapters for the forge, database,
  transport or UI depend inward; infrastructure types and effects stay outside the
  core. Core behavior is determined by explicit inputs: time, randomness and external
  state are supplied as values rather than read implicitly. A pure third-party
  calculation library can be appropriate; standard-library-only imports do not prove
  purity, because the standard library also exposes I/O, clocks and mutable global state.
- `flat tool` — prefer a simple package for bounded orchestration with little independent
  domain logic. Keep decisions separate from effects where useful, without requiring an
  extracted package or a layer of forwarding interfaces. A small copy command can remain
  flat even though it talks to two external systems; a substantial calculation engine can
  justify an isolated core with only one external adapter.

Adapter/entrypoint count, expected lifespan, complexity and change patterns inform the
choice; none alone dictates it. Additional adapters or entrypoints trigger reconsideration,
not automatic extraction. An interface MUST have a concrete current purpose, such as
isolating an unreliable dependency, testing important rules without external services,
or supporting existing alternative implementations. A second implementation is not a
prerequisite, and hypothetical future implementations alone do not justify an abstraction.
Other structures are allowed when the same `layering:` record explains their boundaries
and present justification. A later extraction or dissolution records the changed decision
in its own brief.

A structure label is not evidence. The Task MUST name the rules, external effects,
allowed dependency directions and forbidden shortcuts at any claimed boundary, citing
the existing contracts they preserve or explicitly propose to change. Route affected
consumer compatibility and migration through `consumers:` and the flow verification
requirements, rather than creating a second owner or assuming an atomic rollout.
The Verify table MUST check any claimed boundary, new or changed, with focused tests
and/or mechanical dependency/effect checks, naming the properties covered and their
limits. It MUST also include a discriminating case: a plausible forbidden dependency,
bypassed adapter or unauthorized effect must fail the appropriate check. Import tests alone do not enforce a cross-process
authority boundary; test that boundary at its enforcement point with isolated fixtures.
For an isolated core, include tests of its important
rules without external services. An import allowlist or identifier scan alone MUST NOT be
presented as proof of freedom from side effects. A new mechanical check also carries the
negative control required by §4.4. For a flat tool with no extracted boundary, verify its
observable behavior and relevant failure paths; do not invent a package boundary just to
satisfy this requirement.

For existing briefs, apply the added contract and discriminating-case guidance at the
next authoring/review pass on unfinished work, using the scope triggers above in full
(including new components). This is an amendment policy, not a narrower boundary scope.
Do not rewrite completed implementation or evidence to fit the new description; route
any required implementation migration as separate work.

Every NEW brief's Context section MUST include a `design-fit:` block. It makes the author
answer where the change belongs, what it replaces and how much weight it adds before any
implementer starts, and gives the reviewer something to hold the diff to. Five keys:

```
design-fit:
  owner: <the one module that owns the meaning this brief touches, or n/a>
  contract: <an S-<slug> row id of the project's semantic-owner index, or none — <why>>
  retires: [<mechanism/refusal/flag/test this brief removes>, ...]   # [] is an answer
  weight: <signed delta per ratcheted dimension: verbs, flags, refusals, rule-text lines>
  why-add: <REQUIRED when any weight delta is positive: why the capability cannot live
           in the owner, and what removal was considered instead; n/a otherwise>
```

`contract` names a row of the project's semantic-owner index (in this repository,
`docs/contracts.md` §"Semantic owners"), or says `none` and why. Like `layering:`, the
block is Context text, not frontmatter: no schema field, conformance check or lint reads it
while its grammar settles. (`consumers:` differs: it is a frontmatter schema field, read by
`statusgen --lint` and `statusgen --consumers`.) Legacy briefs are not back-filled. Two
rules keep it from becoming a delete-everything bias:

1. **Consolidate meaning; preserve independent enforcement.** A second *owner* of one
   meaning is a finding. A second *enforcement point* at a different trust boundary, failing
   for a different reason, is a legitimate layer, not a duplicate.
2. **Retiring a control at a trust boundary** names the layer that still refuses the same
   threat, and carries a Verify row proving that layer refuses it with the retired layer
   absent. Tests that pinned a retired refusal retire with it; the reviewer checks that the
   invariant is still covered at its owner, not that every old test survives. This rule adds
   an obligation and grants no authority: retiring a security or access control is still
   subject to the `risk`/`gate` derivation (§3.1) like any other change.

### 4.1.1 Implementation pickup assessment

For work within §4.1's architecture scope, the executor MUST assess the selected
approved target/transition, current semantic owners and applicable engineering
guidelines against the brief and actual producer/consumer implementations before
implementation effects, on resume and after a material change. Record source
revisions, coverage gaps and a reasoned continue/adapt/hold disposition in the
implementation plan or PR, using the existing context receipt where supported.
The [pickup procedure](../plugins/assay/references/implementation-pickup.md) is the
single procedural reference. Missing required context holds only the affected scope;
a newer unapproved proposal or unrelated edit MUST NOT veto authorized work.

A material contract/owner change requires reconciled Task, dependencies, consumers
and Verify plus existing active-work acknowledgement and approval where applicable.
Preserve dated sources and completed Evidence. Brief authors MUST cite canonical
contract/guideline entry points in Read first rather than freeze copied rule text.
This execution obligation adds no frontmatter key or current lint/runtime gate.
Automated discovery/recheck and dispatch integration are delivered by statusgen/20–21;
until supported, use the explicitly bounded manual bootstrap.

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
- The command is written as an inline code span in the Command cell. A cell whose
  first code span is its command (a cell that is exactly one code span, optionally
  followed by prose or a parenthetical) needs no marker. A cell whose first code span
  is NOT its command (a prose cell that mentions a function, a file or a word before
  the real command) MUST mark its command with the explicit command marker: a code
  span whose content starts `cmd:`, e.g. `` `cmd: go test ./pkg/ -count=1` ``. A
  tool that executes Verify rows MUST prefer the first honoured `cmd:`-marked span
  over any other text in the cell; with no honoured marker, each tool keeps its
  unmarked lift (the first code span, or the unwrapped cell). An executor MAY decline
  to run an unmarked first span that is a mention rather than a command (a file, an
  identifier, a word ahead of the command span) and record the row could-not-run,
  since running the mention measures nothing and can exit 0. A cell SHOULD NOT carry
  more than one marked span, and a marked command SHOULD be able to fail (`true`,
  `:` or `exit 0` passes whatever the tree holds). (The marker is `cmd:`, not `run:`, because `run:` is a
  CI workflow step key that Verify prose quotes; a quoted workflow line must never
  become the command.)
- A marker is honoured only where the rendered brief shows it as code, so a reader of
  the rendered table sees the command that runs. Code spans are found as CommonMark
  renders them: a backslash-escaped backtick is literal text, and a run of N
  backticks closes only on the next run of exactly N. A cell whose prose (outside
  code spans) carries an unescaped `<` or `[` (raw HTML, an HTML comment, a link or
  an image, any of which can hide text from the rendered table), or a dollar in any
  spelling — bare `$`, escaped `\$`, or a character reference such as `&#36;`
  (GitHub renders dollar-wrapped text as math, not code, and decodes character
  references first, so any `&…;` reference in prose counts) — has no honoured
  marker. Neither does a cell where a code span's opening backticks are fused to the
  text before them (an autolink, a `~~` strikethrough or a dollar can swallow them):
  only whitespace, the start of the cell or a `(` may lead a span. The marker span
  itself MUST follow whitespace or the start of the cell, and MUST end the cell or be
  followed by whitespace or plain punctuation (`.` `,` `;` `:` `!` `?` `)`). Write
  `\<` or `\[` to use those characters in prose; keep dollars out of a marked
  Command cell.
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

### 4.7 Threat model (risk-gated briefs)

A **risk-gated** brief — `gate: human`, or any `risk` answer `yes` — MUST carry a
recorded threat model. This is the pre-mortem of `docs/mistake-proofing.md` **B5** made
REQUIRED and RECORDED rather than an optional authoring habit: at authoring time the
author names the ways this change could ship and be wrong, and MAPS each named failure
mode to the `## Verify` row that would catch it. A failure mode with no row MUST carry an
explicit line saying so — "no row; review-only" — rather than being silently omitted; an
unmapped failure mode the reader cannot see is the defect this rule closes.

The threat model plugs into the brief's existing defense-in-depth obligation rather than
duplicating it: where the project layer requires a single-point-of-failure note (the one
control the design leans on and the layer(s) behind it), the threat model's failure modes
and their detection rows are the evidence for that note. A conforming implementation MUST
NOT stand up a second pre-mortem concept parallel to **B5**.

This is a body-content requirement checked for PRESENCE, not quality — the same posture
section 4.4 takes for the Verify table. Whether the enumerated failure modes are the
*right* ones is the review gate's judgement, not the linter's.

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

## Allocated reader negotiation contract

[reader-capabilities-v1](reader-capabilities-v1.md) defines the versioned advertisement
and `reader-requirements` envelope used by the shared preflight. JSON version discovery
adds `readerCapabilities`; the existing fields and bare version output are unchanged.
This does not allocate brief-v4, add milestone-start keys to legacy schemas, or enable
prefixed operational writing. The §3.7–3.8 rollout holds remain in force.
