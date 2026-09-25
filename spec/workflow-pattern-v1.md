# Workflow-pattern v1.0-draft — Specification

**Version:** v1.0-draft
**Status:** DRAFT — published for review. v1.0-draft is unstable: breaking changes MAY be
made without a major-version bump; no stability commitment.
**Describes reference implementation:** `statusgen patterns` (graph-execution/02)

## 1. Scope

This document specifies the `workflow-pattern-v1` schema: a reviewed, versioned artifact
that states the shape of a piece of work — its nodes, their execution contracts, and the
gates a risk class requires — as something a reviewer can approve and a tool can refuse,
rather than procedure text scattered across desk skills and routing code. A conforming
implementation MUST produce and consume patterns that satisfy every MUST in this document.

This document does not state how a pattern is *instantiated* against a specific brief or
run (that is the concern of a future specification building on this one), nor does it
change any existing desk procedure — a pattern file DESCRIBES the procedure a role already
follows; it does not alter it.

### 1.1 Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## 2. File location and naming

A workflow-pattern-v1 file MUST reside at `spec/workflow-patterns/<pattern>-v1.yaml`, where
`<pattern>` is the pattern's own name (`implementation`, `research`, …). The `-v1` suffix is
the file's own schema-version marker in its name, independent of the `pattern:` field.

## 3. Document shape

A workflow-pattern-v1 file is a single YAML document with these top-level keys, exactly as
`schemas/workflow-pattern-v1.json` encodes:

| Key | Type | Requirement |
|---|---|---|
| `schema` | string | REQUIRED. MUST be the literal `workflow-pattern-v1`. |
| `pattern` | string | REQUIRED. The pattern's own name. |
| `version` | integer >= 1 | REQUIRED. Bumped on every node/edge/evidence edit made after the pattern's first review. |
| `risk-input` | mapping | REQUIRED. Risk-class verdict to mandatory gate-node-id list; see §5. |
| `nodes` | array (>= 1) | REQUIRED. The node set; see §4. |
| `edges` | array | REQUIRED. Directed `{from, to}` pairs between node ids. MAY be empty for a single-node pattern, though no shipped pattern is. |
| `join` | string | REQUIRED. The id of the node carrying the integration check; see §6. |

## 4. Nodes — the execution contract

A node is an execution contract, not a shell step. Every node MUST sit at a durable
boundary: an artifact, an independent check, a human decision, or an external effect. A
step that produces none of these — a transient action with no output a later node can
depend on, no independent check, and no external consequence — is NOT a node and MUST NOT
be represented as one.

### 4.1 Fields

| Field | Type | Requirement |
|---|---|---|
| `id` | string | REQUIRED. Unique within the pattern. |
| `kind` | string | REQUIRED. Exactly one of `artifact`, `check`, `decision`, `effect`. |
| `role` | string | REQUIRED. Exactly one of `desk`, `reviewer`, `verifier`, `worker` — a `topology.yaml` `apps:` role name. A node names a role, never a login or an id: binding a role to an identity is the roster's job, not the pattern's. |
| `inputs` | array of `{artifact, revision?}` | OPTIONAL. What the node needs before it runs. |
| `outputs` | array of string | OPTIONAL. The durable artifacts the node produces, by name. |
| `evidence` | array of `{kind, claim, mandatory?, signal?, band?, window?, source?}` | OPTIONAL. The claims the node owes. `kind` MUST be one of `command`, `review`, `witness`, `observe`. Each `claim` MUST be stated in words a verifier can check — not "it works". `signal`, `band`, `window` and `source` are meaningful ONLY for `kind: observe`; see §4.1.1. |
| `effects` | array of `{kind, target}` | OPTIONAL. External-consequence actions the node performs. |
| `budget` | `{attempts?}` | OPTIONAL. |
| `outcomes` | `{wait?, fail?}` | OPTIONAL. Named non-success dispositions. |

### 4.1.1 The `observe` evidence kind

`observe` is the one evidence kind that is not resolved from an execution witness,
a review object or a Verify row: it is a **signal watched over a window after a
change lands** — the evidence class the other three kinds cannot express, because
none of them observes anything past the moment the check ran. It carries four
fields beyond the base `{kind, claim, mandatory}` shape, all OPTIONAL in the
schema but load-bearing together once `kind: observe` is used:

| Field | Meaning |
|---|---|
| `signal` | The named signal being watched (e.g. an error-rate metric, an alert name). |
| `band` | The acceptable range/threshold the signal MUST stay within for the claim to resolve `pass` (e.g. "error rate < 1%"). |
| `window` | How long after the change lands the signal is watched (e.g. `PT1H`, `24h`). |
| `source` | Where the claim is filled FROM. A coverage rule (`docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`) resolves this claim by reading `source`; an unreadable source resolves the claim to `could-not-check`, never `pass` — a claim this document cannot corroborate is never treated as satisfied. |

**Declare `observe` only where a deploy exists.** A pattern node MUST NOT carry an
`observe` evidence entry unless the work it governs actually deploys somewhere a
signal can be watched. Where no deploy exists, the kind is **omitted entirely** —
never recorded with `signal`/`band`/`window`/`source` present but empty, which
would read as "declared and inapplicable" rather than "not applicable at all".
Neither pattern this document's reference implementation ships
(`spec/workflow-patterns/implementation-v1.yaml`, `spec/workflow-patterns/research-v1.yaml`)
carries an `observe` entry, because neither pattern's own nodes assert a deploy —
`implementation`'s `merge` node produces a merge commit, not a running deploy. The
shape below is illustrative only, under a placeholder `example-org` deploy:

```yaml
  - id: deploy
    kind: effect
    role: worker
    outputs: [live-change]
    evidence:
      - kind: observe
        claim: "error rate stays within band for one hour after the example-org deploy lands"
        mandatory: true
        signal: error-rate
        band: "< 1%"
        window: PT1H
        source: "example-org/monitoring#error-rate-dashboard"
    effects:
      - {kind: push, target: live-change}
```

### 4.2 Normative rules (MUST)

1. Every node MUST have exactly one `kind` and exactly one `role` (§4.1's shape already
   forces this: both are scalar fields, not lists).
2. A node that declares `effects` MUST be `kind: effect`, OR MUST declare every one of its
   effects' `target` among its own `outputs`. A node that is not an effect node performing
   an action outside its own declared output boundary is a node claiming a consequence it
   cannot be held to.
3. A node whose `evidence` includes a `kind: review` entry (a **review node**) MUST have a
   `role` that differs from the `role` of every node that produced one of its `inputs`
   (i.e. every node whose `outputs` names one of this node's `inputs[].artifact`). This is
   `docs/enforcement-model.md`'s implementer↔reviewer separation, made machine-checkable in
   the pattern itself: a review node cannot share the role of whoever it is reviewing.
4. `risk-input` MUST map every one of the four risk-class verdicts (`low`, `standard`,
   `elevated`, `human`) to a (possibly empty) list of gate-node ids.
5. `join` MUST name a node whose `kind` is `check`.

## 5. `risk-input` — risk class as a declared input

`risk-input` maps a brief's risk-class verdict to the node ids that become mandatory gates
for that brief. A generator instantiating this pattern against a brief of a given risk
class MUST require every node `risk-input` names for that class before the pattern's `join`
node may run. This is how the risk class the brief already declares (`risk:` /
`gate:` in brief-v1/v2 frontmatter) becomes an input to which nodes are mandatory, rather
than a separate routing decision made again by hand.

## 6. `join` — the integration check

The `join` node is the node carrying the pattern's integration check — the point at which
the graph's separate branches (if any) and the accumulated evidence are checked together,
as one thing, before the pattern's outcome is final. `join` MUST name a `check`-kind node
(§4.2 rule 5); a pattern whose `join` points at an `artifact`, `decision`, or `effect` node
has no independent integration check and is refused.

## 7. Effect permissions

A node's `effects[].kind` MUST be permitted for the node's `role`, derived from what each
`topology.yaml` role does today:

| Role | Permitted effect kinds |
|---|---|
| `worker` | `push`, `pr-open`, `comment` |
| `reviewer` | `review`, `comment` |
| `verifier` | `evidence-commit`, `comment` |
| `desk` | `file-issue`, `comment`, `dispatch` |

The `pattern-effect-exceeds-role` lint rule refuses a node whose effect kind is not
permitted for its role. This is the single point of failure this schema depends on for
permission safety (see `docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md`
§Context's single-point-of-failure note): **a generated instance carries no permission the
pattern lacks.** A second, independent layer enforces the same boundary at instantiation
time (a future brief's harness validates an instance against its pattern, so an edited
instance fails for a different reason in a different component than an edited pattern
file); a third, out-of-band layer is that the roster's role binding is what actually
authorises a write on the forge — a node naming `worker` never itself mints a token, so a
wrong pattern cannot act on its own.

## 8. Conformance

A conforming implementation of this specification MUST:

1. Validate every `spec/workflow-patterns/*.yaml` file against `schemas/workflow-pattern-v1.json`
   and against the MUST rules in §4.2, §5 and §6.
2. Refuse (exit non-zero, naming the file, node and rule) a pattern that violates any MUST
   rule; report a schema violation and a MUST-rule violation as distinct classes (a schema
   violation is a shape defect; a MUST-rule violation needs the resolved node graph).
3. Never grant a generated instance of a pattern a permission the pattern itself does not
   grant its nodes.

## 9. Known limitations

This version of the schema does not express: cyclical retry sub-graphs (a node's `budget`
states an attempt count, not a retry topology); cross-pattern references (a node cannot
name a node in another pattern file); or evidence *aggregation* rules (how "every mandatory
claim passes" is computed across an instance's Evidence rows and witnesses) — that
coverage rule is `docs/streams/graph-execution/brief-03-evidence-coverage-rule.md`'s scope,
building on the `evidence[].kind`/`mandatory` vocabulary this document fixes.
