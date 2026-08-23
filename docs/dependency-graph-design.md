# The dependency graph — design spec

**Status**: adopted for authoring. This document is the public definition the brief
template and `brief-rules.md` point at when they document the reserved graph keys
(`gates:`, `feathers:`, the `id:`/`supersedes:` identity keys) and the reference grammar.
It defines what those keys mean; the *gating behaviour* they will eventually drive is
reserved, not yet live (§3.6).

## 0. Summary

Every ordering, behavioural, and cross-repo dependency between briefs is a **typed
frontmatter field**, not a prose sentence in a README. The graph those fields describe
is a **derivation** — `union(all briefs' frontmatter)` — never a hand-maintained file.
Under `schema: brief-v2` the new edge keys are **reserved**: parsed, type-checked, and
validated by `statusgen --lint`, so a tree can carry them and a tool can read them, while
the behaviour that consumes them (holding a brief out of the ready queue because an edge
is unsatisfied) lands later. Reserving the keys now costs one parser stanza; adding them
after a release costs another schema bump and a second migration.

## 1. The problem, stated plainly

A board expresses three kinds of edge between units of work:

1. **build-dep** — this brief consumes another brief's deliverable.
2. **ordering / behavioural** — nothing is consumed, but the other work must be in force
   first (a migration order; a process or skill change that must land before this work is
   safe).
3. **cross-repo / cross-stream** — the other work lives in a different stream, or a
   different repository, whose state this repository's own history does not contain.

Only the first is machine-read today (`depends:`). The other two survive as prose in a
README's critical-path section or as rows in a hand-maintained table — which means a tool
cannot see them, cannot explain why a brief is held, and cannot notice when the prose and
the frontmatter disagree. This spec makes all three typed.

## 2. Design principles

1. **No prose-only edges.** Every ordering, behavioural, or cross-repo dependency is a
   typed frontmatter field. Prose survives as the human explanation — a caption attached
   to a structured edge (`reason:`), never the source of truth.
2. **Every gate has an owner.** A behavioural gate must point at a brief (or a tracking
   issue) that owns clearing it. "That change must land first" is not an edge until
   something owns landing it.
3. **Distributed source of truth; a single hand-maintained graph file is forbidden.** The
   edges live in per-brief frontmatter, each brief a separate file edited on its own
   branch. The graph is a *derivation*, never a document — there is no file whose merge
   conflict, lint freeze, or regen failure can take the graph down, because the graph is
   recomputed from the same files the work already lives in.
4. **Derive on demand, in git, current on every branch.** The graph is computed from the
   branch's committed content (plus the working tree for a local run). Checking out a
   branch *is* checking out its graph. Any materialised view that exists is generated,
   single-writer, and lives outside the merge path.
5. **Hierarchical namespace, composed the same way at every level.** Nodes nest
   cell → repo → stream → brief; unions compose the same way at each level, under one
   ownership rule (a node lives in exactly one parent).
6. **Could-not-check fails closed, bounded, and loud.** An unresolvable or stale
   cross-repo edge holds back only the briefs that declared it, and names them — bounded,
   loud, self-selected beats unbounded, silent, universal.
7. **Incremental, view-preserving landing.** Fields land as optional-but-known keys. A
   repo that has not migrated loses nothing it has today.

## 3. The typed edge schema

### 3.1 The edge-type taxonomy

Every edge carries exactly one `type`. The type determines what "satisfied" means and
what an unsatisfied edge does to whether a brief is ready to dispatch:

| type | Meaning | Satisfied when | Effect while unsatisfied |
|---|---|---|---|
| `build-dep` | the target's deliverable is consumed by this brief | target brief `done` or `verified` | brief ineligible for the ready queue |
| `ordering-gate` | no artifact consumed, but the target must land first (sequencing, migration order) | target brief `done` or `verified` | brief ineligible for the ready queue |
| `behavioural-gate` | a behaviour / process / skill change must be in force before this work is safe | the **owning brief** `done` or `verified` | brief ineligible for the ready queue |
| `human-gate` | a human act clears it — a ruling, a sign-off, a granted scope | the referenced **issue closed**, or a dated human assertion on the edge | brief ineligible for the worker pool; it surfaces on the awaiting/gate queue instead — a human's work item, not a dispatch candidate |
| `external-env` | an infrastructure / environment precondition | the referenced tracking issue closed, or an operator assertion | brief ineligible; rendered in the env-blocked awaiting segment |
| `external-repo` | a resolution class, not a semantic of its own: the target lives in another repository. Combines with one of the above | per the underlying type, read from the target repo's derived view | per the underlying type; if that view is missing or stale → could-not-check, held closed and named (§4.6) |

Two consequences worth stating flatly:

- `depends:` entries are `build-dep` edges by definition — nothing changes for them.
- A `human-gate` edge **requires an issue target.** A human gate with no tracking issue
  is an invisible wait, so the lint makes it a PROBLEM on the edge form: file the issue,
  or the edge does not parse as a human gate.

### 3.2 `depends:` / `unblocks:` — existing fields

Unchanged in form: in-repo `<stream>/<NN>` references, `checkRef`-validated against the
current root's known streams, same-wave lint, and the existing satisfied-when rule. A
`depends:` entry is a `build-dep` edge. `unblocks:` is its inverse — the same edge
declared from the enabling end.

### 3.3 `feathers:` — cross-repo / cross-stream edges, and the reference grammar

`feathers:` is a reserved optional-but-known frontmatter field for edges whose target is
in another stream or another repository — the edges a README "feathering table" carries
today, made typed.

**Reference grammar** (also used by `gates:` targets and by graph queries):

```
<stream>/<NN>                        in-repo brief        (existing form, unchanged)
<repo-alias>:<stream>/<NN>           cross-repo brief
<repo-alias>#<issue>                 cross-repo issue
#<issue>                             in-repo issue
<cell>:<repo-alias>:<stream>/<NN>    cross-cell brief     (reserved form)
```

The grammar is deliberately **hierarchical and elision-based**: each omitted prefix
segment means "same as the declaring brief" — omit the cell and you mean your own cell,
omit the repo and you mean your own repo. A cross-cell reference needs no schema change
later, only a longer reference.

Repo aliases resolve through a checked-in registry, `docs/streams/graph-repos.yaml`
(`schema: graph-repos-v1`), which maps each short alias to its cell. Deliberate choices:

- **Canonical node ids in emitted output always use the fully qualified form.** Short
  aliases are an authoring convenience only — ambiguous short names are a known trap, so
  emitted output never relies on one.
- **The registry is a validated file of its own schema**, not an entry in any operational
  config the write path reads. The graph must never be able to take the write verbs down,
  so its registry is isolated from them.
- **An alias may reserve a namespace without publishing its target.** In a public tree,
  an entry for a repository that is not publicly named carries `repo: null` and
  `unpublished: true`: the alias still parses and lint-validates, but resolving a
  reference past it is a could-not-check from this tree, never a silent answer. The full
  mapping lives in that repo's own registry.

**Entry shape** — a scalar reference (defaults: `type: build-dep`, no reason) or a
mapping that carries the why:

```yaml
feathers:
  - <repo-alias>:<stream>/<NN>              # scalar: build-dep, no reason
  - ref: <repo-alias>#<issue>
    type: external-env
    reason: "<the machine-attached why>"
```

**Why a separate field rather than widening `depends:`.** The `depends:` validator hard-
fails any reference whose stream is not in the current root's known set — by design, and
that check is load-bearing (it catches typos against a closed set). Cross-repo references
are resolvable only against a view that may be absent or stale, so they need three-state
treatment (satisfied / unsatisfied / could-not-check) that `depends:` deliberately does
not have. Different failure semantics → different field.

### 3.4 `gates:` — behavioural / ordering edges

`gates:` is a reserved optional-but-known field for must-precede edges that are not
build-deps. Every entry is a **mapping** — `type` and `reason` are required (unlike
`feathers:`, where a scalar defaults sensibly), because the entire point of the field is
to make the "why" machine-attached:

```yaml
gates:
  - on: <stream>/<NN>                        # any reference from the §3.3 grammar
    type: behavioural-gate
    reason: "<why this work is unsafe until the owning brief is in force>"
  - on: "#<issue>"
    type: human-gate
    reason: "<the decision this work needs first>"
```

`on:` accepts the full §3.3 grammar, so a gate may be cross-repo
(`on: <repo-alias>:<stream>/<NN>` with `type: ordering-gate`).

### 3.6 Schema versioning — reserved under brief-v2

`gates:`, `feathers:`, and the identity keys (`id:`, `supersedes:`, and the per-Verify-row
`id`/`target`) land **reserved under `schema: brief-v2`**: parsed, type-checked, and
lint-validated, with the gating behaviour deferred. The alternative to reserving them —
adding them as loose optional keys under the current schema — has a fail-open hazard: an
old pinned tool silently ignores an edge it does not understand and dispatches past it,
which is the exact edge class this design exists to close. A schema bump makes that
fail-closed instead: a tool pinned below the release that understands brief-v2 **refuses**
a brief-v2 tree rather than misreading it. That refusal is the property worth the one-time
flag-day, so the keys are reserved at the bump rather than retrofitted after it.

## 4. The derived graph

### 4.1 Source of truth: distributed frontmatter — there is no graph file

The graph has no storage of its own. Its entire content is the typed frontmatter of the
brief files (`depends:` / `unblocks:` / `feathers:` / `gates:`), the stream READMEs'
generated status tables, and the small `graph-repos.yaml` registry. Every one of those is
an existing, independently-edited file on the normal branch/PR path. Consequences, each
load-bearing:

- **No merge collisions**: two branches adding edges touch two different brief files.
- **No freeze class**: there is no graph-write step that a single lint PROBLEM can abort
  into staleness. A malformed edge PROBLEMs its own brief; every other edge is unaffected.
- **No drift**: the graph cannot disagree with the briefs, because it *is* the briefs,
  re-read.

### 4.2 Derivation: per-branch, on demand

The graph is computed from the branch's committed content — nothing graph-shaped is
committed to the default branch by default, no out-of-band store that can drift from git.
Any materialised view that exists (a generated feathering table in a README, §4.7) is
single-writer and regenerated, never hand-edited.

### 4.4 The hierarchy: cell → repo → stream → brief

A **cell** is a configured scope of service — today there is exactly one; a multi-tenant
tier serves many. The graph must not hard-code "repo" as its top level, or the day a
second cell exists becomes a schema migration across every brief. So the containment
hierarchy is **cell → repo → stream → brief**, uniform at every level:

- **One ownership rule, applied recursively.** A brief lives in exactly one stream, a
  stream in exactly one repo, a repo in exactly one cell, a cell in one fleet. Every union
  at every level rejects double-ownership rather than merging it.
- **One composition operator.** `graph(stream) = union(briefs)`;
  `graph(repo) = union(streams)`; `graph(cell) = union(repo graphs)`;
  `graph(fleet) = union(cell graphs)` — the same operator by induction, not new
  machinery.
- **Stable identity across renames.** Each brief carries an `id:` — a uuid minted once at
  authoring and never reused. It is the key a fact log or an executor references when a
  brief is renamed, renumbered, or re-homed to another repo, and the reason the template
  says to mint one at authoring even while nothing gates on it yet: a uuid added later is
  a uuid with no history. `supersedes:` records object lineage — the briefs a given brief
  splits from or re-baselines — so a split or a re-baseline is a traceable edge, not a
  silent replacement.

**YAGNI boundary, stated explicitly.** Cell-ready by design now (cheap): the node-id
grammar carries the cell segment; the registry names each repo's cell; the union operator
is level-agnostic. Deferred until a second cell exists (not cheap, no instance today):
cross-cell reference resolution and trust, per-cell view discovery, and any cell-level
dispatch or visibility semantics. Every reference written today elides the cell and means
the sole cell; nothing written today has to change later.

### 4.6 Staleness and the could-not-check arm

A cross-repo edge is resolved against the target repository's derived view. When that view
is absent or older than a freshness bound, the edge is **could-not-check**: it holds back
only the briefs that declared it, and the board names them and says why. A cross-repo edge
never renders a confident satisfied/unsatisfied answer from a view the instrument could
not actually read — the three-state instrument invariant applied to the graph.

### 4.7 Generated views

A README feathering table, where one exists, becomes a **generated view** of the `feathers:`
fields — single-writer, regenerated between markers, the same shape a generated status
table has. Hand-editing a generated view is a lint PROBLEM.

## 5. Relationship to the board

This graph and the derived board (a stream README's generated status table) share one
substrate: the brief frontmatter and the stream READMEs. The board derives a brief's
**lifecycle cell** from its witnesses; the graph derives a brief's **edges** from its
typed fields. Neither is a field a human hand-asserts, and neither is a file committed to
the default branch that a merge conflict or a regen failure can take down.
