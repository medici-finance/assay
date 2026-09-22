# The deploy model — environments, the deploy transition, rollback, runbooks

Assay's brief lifecycle ([`lifecycle-v1.md`](../spec/lifecycle-v1.md)) stops at `done`: a
brief reaching `done` means a change was reviewed and merged. It says nothing about whether
that change reached a place where users are. Before this document there was no environment
model, no deploy transition, no rollback obligation and no standard for the runbook a deploy
depends on — so "release" in this methodology meant "a tagged artifact exists," never "it
shipped."

This document specifies the three pieces that close that gap, and is deliberately narrow:

- it names the SHAPE an adopter fills in for their own estate, not the estate itself;
- it adds ONE new typed register (the DEPLOYS register, reference implementation:
  `statusgen/deploygate.go`) rather than a sixth brief-lifecycle state; and
- it changes nothing about `docs/distribution.md`'s release story — a RELEASE and a
  DEPLOYMENT are different operations with different rollback questions, and this document
  keeps them distinct rather than quietly merging them (see "Rollback" below).

**Ground rules this document runs under, and expects every deploy record to run under:**
Assay is a methodology repo; it creates no infrastructure. Nothing in the reference
implementation contacts a cluster or a production endpoint, read-only included — a
conforming linter is pure over the tree (`KUBECONFIG=/dev/null` before any command, same as
every other `--lint` check). Anything that needs live state to answer is `could-not-check`
with the reason, never a probe and never a vacuous green.

## Environments

An **environment** is a named place a deploy can target. Assay supplies the shape an
adopter fills in for their own estate — it names no environment beyond one worked example,
because an adopter's estate is not Assay's to model.

A typed environment declaration carries:

| Field | Meaning |
|---|---|
| `name` | The environment's identifier (e.g. `staging`, `prod-us`). Free-form; an adopter's own naming convention. |
| `purpose` | One line: what this environment is for and who/what it serves. |
| `may-deploy` | Who holds deploy authority here — a `human:<name>` stamp, a named role, or a named automation identity. This is the SAME authority a `DEPLOY` record's `authority:` field must name (see "The deploy transition"). |
| `evidence-required` | What a deploy TO this environment must leave behind — at minimum, a `DEPLOY` record (below); an adopter MAY require more (a change ticket, a signed approval). |
| `user-facing` | `yes` / `no`. A `yes` environment is where the rollback obligation below bites hardest — see "Rollback". |

**Worked example** (not a live register — descriptive, so the shape is a filled-in table
rather than a bare column list):

| name | purpose | may-deploy | evidence-required | user-facing |
|---|---|---|---|---|
| `staging` | Pre-production soak; internal traffic only | `human:release-manager` | a `DEPLOY` record | no |
| `prod` | Serves real traffic | `human:release-manager` (change-window only) | a `DEPLOY` record + the change ticket it links | yes |

An environment declaration is descriptive, not a register this repo's linter enforces —
enumerating and validating an adopter's own environment names is exactly the "estate that is
not Assay's to model" scope line above. What Assay DOES enforce is the transition that
targets one: the `DEPLOY` record below.

## The deploy transition

A deploy is **not** a sixth brief-lifecycle state (see
[`lifecycle-v1.md` §9](../spec/lifecycle-v1.md)). It is a transition on its own typed
record — a **DEPLOY** entry in the DEPLOYS register, `docs/streams/deploys/DEPLOY-<slug>.md` —
so that a brief's own five-state sequence (`todo → in-progress → implemented → verified →
done`) stays exactly what it already was, and "deployed" is answered by a second, separate
artifact rather than by overloading the first.

### Preconditions

A DEPLOY record's transition MUST NOT be considered to have fired until:

1. **The brief it carries is `verified` (or `done`)** — never merely `implemented`. This
   reuses the existing attribution rule rather than inventing a new one:
   `implemented` is the implementer's own self-report (`lifecycle-v1.md` §2.3); `verified`
   means a NON-implementer independently re-ran the Verify table on merged main and filled
   Evidence (`lifecycle-v1.md` §2.4). A deploy gated on `implemented` would let a model-gated
   implementer's own self-report authorize a deploy, which is exactly the "narrator grading
   their own exam" problem the lifecycle already refuses at `implemented`.
2. **A named authority exists for the target environment** — the record's `authority:` field
   names a human (`human:<name>`), matching the environment's own `may-deploy` value. A
   model-gated identity MUST NOT hold deploy authority: this is the same boundary
   `lifecycle-v1.md` §2.5 draws for `gate: human` review, applied to the deploy transition.
3. **A rollback obligation is stated** — see "Rollback" below. A deploy declaration with no
   reverse path does not pass the gate.

### The record

A DEPLOY record is the artifact the transition leaves. A deploy that leaves no record did
not happen for the purposes of an audit pack: `docs/streams/deploys/DEPLOY-<slug>.md`,

```
---
id: DEPLOY-<slug>
kind: deploy
date: "YYYY-MM-DD"
title: "<one line: what was deployed, where>"
environment: <name>              # an environment declared above
brief: <stream>/<NN>             # the brief this deploy carries — MUST resolve to verified/done
authority: "human:<name>"        # the deploy authority for this environment
rollback: <the reverse path> | none-accepted
rollback-approver: "human:<name>"  # REQUIRED when rollback: none-accepted
blocked-by: env                  # OPTIONAL — see "Board handling" below
---

<body: what shipped, any operational notes>
```

`brief:` is the same `<stream>/<NN>` shape a brief's own frontmatter uses. The reference
implementation (`statusgen/deploygate.go`) resolves it against the loaded stream set and
enforces precondition 1 as a hard PROBLEM (`--lint` exit non-zero) naming the brief and its
actual status when the precondition is not met; a reference that resolves to no brief is
flagged as a dangling reference, never silently ignored.

**What this precondition does and does not attest.** Like the design-approval gate before
it (`lifecycle-v1.md` §4.4), the check proves an approved-basis record EXISTS and
dereferences to a brief at the required status. It does not mechanically prove the deploy
authority is a different identity from the brief's author or its verifier — that is the same
attribution-not-identity limit `lifecycle-v1.md` §7.1.2 already declares, restated here
rather than silently assumed away.

## Rollback

A deploy declaration with no reverse path does not pass the gate. `rollback:` MUST be one of:

- **a stated reverse path** — free text naming how the deploy is undone (a re-point, a
  previous artifact re-deployed, a feature flag flipped back); or
- **`none-accepted`** — the reverse path genuinely does not exist (a schema migration
  already applied, a published artifact already consumed downstream), stated as an
  ACCEPTED CONSEQUENCE with a NAMED approver (`rollback-approver: "human:<name>"`), never
  omitted. A record carrying `rollback: none-accepted` with no `rollback-approver` is
  rejected — the point of the field is that "we can't roll this back" is a decision someone
  is named as having accepted, not a silence.

### This is a DEPLOYMENT rollback, not a RELEASE rollback

[`docs/distribution.md`](distribution.md) states plainly that **there is no rollback** for a
released Assay umbrella version: the platform has no downgrade verb, and moving to an older
named version is a re-point, never a rollback. That statement is about the METHODOLOGY'S OWN
distribution mechanism (the `.assay-versions` pin and the umbrella tag) and stays exactly as
true after this document as before it — this document does not touch it and does not
reconcile it away.

The rollback this section specifies is a different operation on a different axis: it is the
reverse path for a DEPLOYMENT — an adopter shipping THEIR OWN change to THEIR OWN
environment, tracked by a DEPLOY record. Rolling back a deployment (re-pointing an
adopter's environment at the previous artifact) and rolling back a release (moving Assay
itself to an older umbrella tag) are unrelated operations that happen to share a word:

| | Rolling back a RELEASE (`distribution.md`) | Rolling back a DEPLOYMENT (this document) |
|---|---|---|
| What moves | The Assay umbrella version an adopter's OWN repo is pinned to | An adopter's OWN environment's running artifact |
| Mechanism | None — `upgrade-assay` re-points to another named version; the platform has no downgrade verb | The `rollback:` reverse path named on the DEPLOY record, or the accepted-consequence exception |
| Who is affected | The adopter's own tooling/pin | The environment's users (`user-facing: yes` in the "Environments" table) |
| This document's claim | Unchanged: still no rollback, still honest about that limit | A deploy with no reverse path does not pass the gate, unless that absence is a named, accepted consequence |

A deploy model that silently reversed `distribution.md`'s honesty statement — implying
Assay itself gained a release rollback because deployments now have one — would be the
wrong-but-well-formed failure this table exists to rule out.

## Runbooks

A **runbook** is a typed recovery artefact: a **RUNBOOK** entry in the same DEPLOYS
register, `docs/streams/deploys/RUNBOOK-<slug>.md`:

```
---
id: RUNBOOK-<slug>
kind: runbook
date: "YYYY-MM-DD"
title: "<one line: what this runbook recovers>"
trigger: "<what condition starts this runbook>"
cadence: "<how often it must be drilled, e.g. quarterly>"
---

<body: preconditions, the steps, the verification that the recovery worked>

## Drill rows

| # | Action | Expect | Evidence |
|---|--------|--------|----------|
| 1 | <what the drill runner does> | <what proves the recovery worked> | <filled by whoever ran the drill: date + runner + result> |
```

A runbook has a trigger, a preconditions list, the steps, the verification that the
recovery worked, and **drill rows** — a Verify-table-shaped table with a cadence, whose
Evidence is filled by whoever ran the drill (the same Evidence discipline
`spec/brief-v1.md` uses for a Verify row, applied to a rehearsal instead of a one-time
check).

**An undrilled runbook reports `could-not-check`, never "ready".** A drill row with an
empty Evidence cell means nobody has rehearsed that step; the reference implementation
(`statusgen/deploygate.go`) reports a runbook carrying any such row as an advisory
`could-not-check` NOTICE naming the record and the undrilled count. This is deliberately
NEITHER a pass (nothing proved the recovery works) NOR a hard `--lint` failure (an
unrehearsed drill is a gap to close, not itself a defect in the record) — the same
three-state discipline every other instrument in this repo keeps
(`docs/three-state-instrument-rule.md`).

## Board handling

The board already has a `blocked-by: env` marker (`spec/lifecycle-v1.md`, brief-v1
frontmatter) whose only legal value is `env`, for a brief that cannot proceed until an
environment exists. A DEPLOY record waiting on an environment that does not yet exist
reuses this SAME marker — `blocked-by: env` on the DEPLOY record itself — rather than
minting a second one; the Awaiting board already knows how to file `blocked-by: env`
separately and the desk already knows not to re-triage it.

`statusgen`'s handling, reference implementation `statusgen/deploygate.go`:

- The DEPLOYS register (`docs/streams/deploys/`) is a reserved register directory
  (`spec/registers-v1.md` §2.1 shape) — stream discovery skips it, the same way it skips
  `decisions/`, `requirements/`, `findings/` and `intake/`.
- **Register shape** — every record's `id`, `date`, `title`, and kind-specific required
  fields — is validated the same way the DECISIONS register's shape is validated
  (`spec/lifecycle-v1.md` §4.4's `DR-<slug>` sibling), producing a hard `--lint` PROBLEM on
  a malformed record.
- **The deploy-transition precondition** (a DEPLOY record's `brief:` reference must resolve
  to a `verified`-or-later brief) is a hard `--lint` PROBLEM naming the brief and its actual
  status when unmet, and a dangling reference (a `brief:` that resolves to nothing in the
  loaded stream set) is flagged the same way a dangling `design:` reference already is.
  A register that cannot be read is a `could-not-check` NOTICE, never a silent pass and
  never a blanket failure of every citing record.
- **An undrilled runbook** is the NOTICE described in "Runbooks" above — advisory, never a
  hard failure, always visible.

## What this document does not do

- It does not integrate with any specific CI/CD system or cloud provider — the DEPLOY and
  RUNBOOK record shapes are the contract; how an adopter's own pipeline authors one is
  their own integration.
- It does not change the release/versioning story beyond the one reconciliation in
  "Rollback" above — `docs/distribution.md`'s "no rollback" statement stands unmodified.
- It does not model an adopter's own environments as a register Assay validates — the
  "Environments" table is descriptive shape plus a worked example, not a corpus this
  repo's linter enforces membership against.
