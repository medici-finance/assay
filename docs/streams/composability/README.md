---
stream: composability
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [624]
board: generated
---

# Composability Stream

Make **Assay a set of declared components with paired install/remove effects and declared
dependencies**, so that one broken key downs one component instead of the fleet, an adopter
can turn a component off and get their repo back, and the install/upgrade path is derived
from declarations rather than hand-ordered.

Today Assay is composed by convention. `docs/adopting-assay.md` §2 lists the units in prose;
`assay:install` orders the primitives by hand; the units depend on each other through
undeclared `ASSAY_*` keys, verb names, and file paths; and **nothing has an inverse** — the
only "uninstall" text in the tree is "take the shim directory off PATH". Two house incidents
in 2026-09 showed the cost: a single roster key one tool refused took a whole desk lane down,
and one unregistered `ASSAY_*` key fail-closed every desk verb at once. The dependency was
real; it was just not declared, so nothing could contain it.

The model comes from *A Programming Paradigm for Spatiotemporal Composability*
(arXiv 2608.25512, 2026-08-26), which formalises exactly these two dimensions — **temporal**
(every effect a component makes carries an inverse the runtime holds, so removal is
replay of inverses) and **spatial** (a component declares what it requires and provides,
and the runtime activates and deactivates it as those keys appear and disappear). Its
implementation is an in-process TypeScript runtime; Assay's "runtime" is a git tree, a
forge, and an agent session, so this stream borrows the **discipline**, not the library.
The full mapping and the normative draft are in [`component-model.md`](./component-model.md).

## End state — what "done" means

Every installable unit of Assay carries a `component.yaml` declaring the keys it
**provides**, the keys it **injects**, and an ordered list of **apply steps each paired with
an inverse or a ledgered compensation**. A lint proves every injected key has a provider and
the graph is acyclic. A missing or rejected extension key deactivates **only the components
that inject it**, with a `could-not-check`-style report, while the trust-surface keys stay
fail-closed exactly as today. An adopter repo carries a desired-state record the reconcile engine
diffs against an install ledger; `disabled: true` on an entry runs that component's inverses
and the repo is left as it was, with anything outside the system boundary (forge Apps,
rulesets, labels in use) listed for a human to compensate rather than silently left behind.
`upgrade-assay` becomes an edit to an entry's version.

## Scope — the six units, and what each owns

0. **Manifests + key catalogue + lint (brief 00).** One `component.yaml` per unit in the
   `docs/adopting-assay.md` §2 inventory; namespaced keys; a lint that resolves every
   `inject` to a `provide` and reports cycles from declarations alone. **Head of the
   critical path** — everything else reads the manifests.
1. **Reactive activation — contain the blast radius (brief 01).** Desk tools consult the
   manifests: a component whose required *extension* key is missing or malformed goes
   INACTIVE with a report; its neighbours keep running. Trust-surface keys remain
   fail-closed; the change is *which* components refuse, never *whether* trust refuses.
2. **Install ledger + paired inverses + `disable` (brief 02).** Every apply step in every
   manifest gets its inverse (inside the boundary) or its ledger entry and compensation
   (outside it). A `disable` verb replays them LIFO. Human-gated: it deletes things.
3. **Desired-state record + reconcile engine (brief 03).** `.assay/config.yaml` (planned) entries; the
   reconcile engine applies the least disruptive operation per changed field; `deskmigrate` and
   `upgrade-assay` drive it instead of a hand-ordered primitive list.
4. **Harness as an exclusively-bound key (brief 04).** The Claude Code, Codex and Cursor
   adapters become components that *provide* `assay.harness`; the skills *inject* it. One
   binding at a time, switched by the reconcile engine, not by three parallel manifests.
5. **Promote to `spec/component-v1.md` (planned) + adopter doc delta (brief 05).** The draft in this
   stream becomes a versioned spec document alongside `brief-v1`, `registers-v1`,
   `lifecycle-v1`; `docs/adopting-assay.md` gains the component view and the removal path.

**Out of scope:** adopting Cordis or any TypeScript runtime; hot-reloading a running desk
process (Assay components are files and forge state, not live objects — the existing
`SYNC-FROM` / paired-versions checks already play the stale-entry role); rewriting the desk
verbs' internals beyond the activation and ledger seams; structural interface typing across
components (the paper itself lists it as open — we use namespaced keys plus version ranges).

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 00 | [Component manifests, key catalogue, and the resolve/cycle lint](brief-00-manifests-and-lint.md) | 0 | M | implemented | — | — |
| 01 | [Reactive activation — a missing extension key downs one component, not the fleet](brief-01-reactive-activation.md) | 1 | M | todo | — | — |
| 02 | [Install ledger, paired inverses, and the `disable` verb](brief-02-ledger-and-inverses.md) | 1 | L | todo | — | — |
| 03 | [Desired-state record and the reconcile engine behind deskmigrate / upgrade-assay](brief-03-desired-state-reconcile.md) | 2 | L | todo | — | — |
| 04 | [Harness as an exclusively-bound key — adapters as components](brief-04-harness-as-key.md) | 1 | M | todo | — | — |
| 05 | [Promote the draft to spec/component-v1.md + adopter doc delta](brief-05-promote-spec.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

```
[No external-environment head. Everything here is source, docs, and CI on this repo;
 the one human gate (02, deletion in adopter repos and on the forge) is a review gate,
 not a procured environment.]
                      |
   00 manifests + lint ──┬──► 01 reactive activation ─────────────────────────┐
                         ├──► 02 ledger + inverses ──► 03 desired-state reconcile engine ──┼──► 05 promote spec
                         └──► 04 harness as key ──────────────────────────────┘
```

**In-stream head: 00.** Longest chain is `00 → 02 → 03 → 05`. 00 is at the head because
every later brief reads the manifests: 01 needs declared keys to know what to deactivate,
02 needs declared apply steps to pair inverses with, 04 needs a key for the adapters to
provide. 01 is deliberately wave 1 and small: it is the unit that would already have
prevented two outages, and it lands without waiting for the ledger or the reconcile engine.
