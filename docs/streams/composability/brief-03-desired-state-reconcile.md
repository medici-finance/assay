---
brief: composability/03
title: Desired-state record and the reconcile engine behind deskmigrate / upgrade-assay
why: >-
  Install order and upgrade steps are hand-written lists today, and the only installed-state
  record is a version pin. With manifests (00) and reverses (02) the install can be derived:
  a record of entries says what should be installed, the reconcile engine diffs it against the ledger
  and applies the least disruptive operation per changed field, and an upgrade is an edit to a
  version. Cycles and ordering mistakes become lint failures before anything runs.
wave: 2
depends: ["composability/00", "composability/02"]
unblocks: ["composability/05"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §7 (desired-state record; per-field reconcile table; end state a function of the final record alone), §11 (umbrella = a group entry)"
  - "arXiv 2608.25512 §5.2.1 Declarative Configuration (entry fields id/url/isolate/intercept/config/disabled; per-field least-disruptive dispatch; Theorem 80: the quiescent state depends on the final configuration only, whatever the order)"
  - "docs/distribution.md and docs/streams/distribution — the umbrella version, .assay-versions, deskversion, deskmigrate, upgrade-assay as they stand"
  - "composability/02 — the ledger the reconcile engine reads and the reverses it replays on rebuild"
consumers:
  - ".assay-versions: fixed-here (becomes a derived view of .assay/config.yaml for the pinned-binary channel; deskversion reads the record first and falls back to the pin)"
  - "plugins/assay/skills/upgrade-assay/SKILL.md: fixed-here (drives the reconcile engine; dry-run shows the per-entry operations)"
  - "plugins/assay/skills/install/SKILL.md: fixed-here (install = write the record, then reconcile; the hand-ordered primitive list becomes the derived order)"
  - "tools/desk/cmd/deskversion: fixed-here (reads `umbrella` from the record)"
  - "tools/desk/cmd/deskmigrate: fixed-here (becomes the reconcile engine driver; existing migrations run as pre-reconcile steps)"
  - "docs/adopting-assay.md: follow-up composability/05"
---

# Brief 03 — Desired-state record and the reconcile engine behind deskmigrate / upgrade-assay

## Context

files:
- **create** the schema and a reference `.assay/config.yaml` (planned) (`schema: assay-config-v1`,
  `umbrella`, `entries[]` with `id`, `component`, `version`, `config`, `intercept`, `disabled`)
  — documented in `component-model.md` §7, checked by `deskmanifest lint`.
- **edit** `tools/desk/cmd/deskmigrate/` — becomes the reconcile engine driver: `deskmigrate plan`
  (diff record vs ledger + tree, print operations) and `deskmigrate apply` (run them). Existing
  migrations run as pre-reconcile steps keyed on the `umbrella` change.
- **edit** `tools/desk/cmd/deskversion/` — read `umbrella` from the record; fall back to
  `.assay-versions` when no record exists (pre-record adopters).
- **edit** `plugins/assay/skills/upgrade-assay/SKILL.md` — an upgrade is: bump `umbrella` and
  the entry versions it pins, `deskmigrate plan`, show release notes, `deskmigrate apply`.
- **edit** `plugins/assay/skills/install/SKILL.md` — install is: write the record from the
  chosen scenario, then `deskmigrate apply`; the primitive order is derived from manifests.
- **edit** `docs/distribution.md` — the record is the source of truth; `.assay-versions` is a
  derived view.

facts:
- **Per-field dispatch** (`component-model.md` §7): `component`/`version` → reverse old, apply
  new; `intercept` → in place, no reverse; `config` → hand to the component; `disabled` → replay
  reverses / re-apply; entry added/removed → apply/reverse. Nothing else is a valid operation.
- **The end state is a function of the record alone.** Two different orders of the same
  operations MUST leave the same tree and ledger; the test suite checks this with a randomised
  order over a fixture.
- **Dependency order is derived from `inject`**: a component's apply runs after its required
  providers'; reverses run in the opposite order. No hand-written order survives this brief.
- **Pre-record adopters** have `.assay-versions` and no `.assay/config.yaml` (planned); the first
  `deskmigrate plan` on such a tree synthesises a record from the pin and the manifests present,
  prints it, and asks for it to be committed before applying anything.
- **Outside steps still go through 02's compensation rules**: a rebuild whose reverse is
  `list-for-human` stops the plan at that step and prints the checklist; `apply` never skips
  past a listed item.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`.
- `deskmigrate apply` runs only against a throwaway fixture repo during implementation and
  verification.
- If an existing migration cannot be expressed as a pre-reconcile step, keep it as-is and raise
  `NEEDS_CONTEXT` naming it; do not drop a migration to make the model fit.

## Task

1. Define the record schema; extend `deskmanifest lint` to validate it (unknown component,
   version not in range of dependents, duplicate id, `intercept` keys not in the component's
   `intercept:` list).
2. Implement `deskmigrate plan`: load record, manifests, ledger; compute the per-entry
   operation from the changed fields; order by derived dependencies; print.
3. Implement `deskmigrate apply`: run the plan; stop at any `list-for-human` compensation; write
   ledger lines as 02 specifies; exit three-state.
4. Synthesise a record for pre-record trees; make `deskversion` read the record.
5. Rewrite the `upgrade-assay` and `install` skill bodies to drive the reconcile engine; keep the
   human gates those skills already carry.
6. Order-independence test: fixture record with five entries; apply the same operation set in
   ten shuffled orders; assert identical tree hash and ledger.

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `deskmanifest lint --root <fixture>` with a valid `.assay/config.yaml` (planned) | exit 0 |
| 2 | `deskmigrate plan` on a fixture whose record matches its ledger | exit 0; prints `no operations` |
| 3 | edit one entry's `version`; `deskmigrate plan` | exit 0; exactly one `rebuild` operation for that entry, preceded by none and followed by its dependents' re-activation lines |
| 4 | edit one entry's `intercept`; `deskmigrate plan` | exit 0; one `update-in-place`; no rebuild |
| 5 | set `disabled: true` on one entry; `deskmigrate apply`; then `tar df` against the pre-install snapshot for that component's inside paths | empty diff for that component; other entries untouched |
| 6 | order-independence: `cd tools/desk && go test -run TestReconcileOrderIndependent ./cmd/deskmigrate/...` | exit 0 (ten shuffles, identical hash + ledger) |
| 7 | mutation: introduce a dependency cycle between two fixture entries; `deskmigrate plan` | exit 1; reports the cycle; no apply possible; restore |
| 8 | pre-record tree (only `.assay-versions`): `deskmigrate plan` | exit 2; prints a synthesised record and `could-not-check: no .assay/config.yaml — commit the record above, then re-run` |
| 9 | `deskversion` on a tree with a record | prints the record's `umbrella`; on a tree without, prints the pin (unchanged behaviour) |
| 10 | neighbour: `upgrade-assay --dry-run` (skill body) on the fixture | shows per-entry operations + release notes; changes nothing |
| 11 | flow: fixture install via the rewritten `install` body → `deskmigrate plan` | `no operations` (install and reconcile agree) |
| 12 | `cd tools/desk && go test ./cmd/deskmigrate/... ./cmd/deskversion/... ./cmd/deskmanifest/...` | exit 0 |

## Evidence

Pending — the implementer records its run here on reaching `implemented`; an independent
runner records a second run on merged main before `verified`. Evidence MUST name the fixture
repo path.

## Review

gate: model — pending.
