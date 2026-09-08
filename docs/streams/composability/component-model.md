---
stream: composability
title: Component model — draft-0 of component-v1
status: draft
---

# Component model — draft-0 (candidate `spec/component-v1.md` (planned))

**Version:** draft-0 (stream-local; `composability/05` promotes it to `spec/component-v1.md` (planned))
**Status:** DRAFT — every MUST below is a design target, not a description of the reference
implementation. §12 lists what exists today.
**Source model:** *A Programming Paradigm for Spatiotemporal Composability*, arXiv 2608.25512
(2026-08-26): revertible effects, reactive coeffects, the context paradigm, a declarative
component loader with configuration reconciliation. This document is the mapping of that
model onto Assay's actual substrate — a git tree, a forge, and an agent session — and it
adopts the model's *discipline*, not its TypeScript runtime.

The key words MUST, MUST NOT, SHOULD, MAY are to be read as in RFC 2119.

## 1. Scope and terminology

| Term | Meaning here | Paper term |
|---|---|---|
| **component** | an installable unit of Assay (a skill, a hook set, a tool bundle, a CI workflow, a label set, a forge App binding, a scaffold) | component |
| **key** | a named thing one component provides and others require; namespaced (§3) | coeffect key |
| **manifest** | the `component.yaml` declaring a component's keys and apply steps (§2) | the triple (𝑑, 𝑝, 𝑒) |
| **apply step** | one effect the component makes on the adopter repo, host, or forge, paired with its reverse (§4) | revertible effect |
| **boundary** | what the system can exclusively modify and restore (inside) vs. what it cannot (outside) (§5) | system boundary |
| **ledger** | the append-only record of what a component created outside the boundary, so removal can compensate (§5) | acquisition record |
| **entry** | one line of the desired-state record: which component, which version, which config, enabled or not (§7) | loader entry |
| **reconcile engine** | the tool that diffs entries against the ledger and applies the least disruptive operation (§7) | loader reconciliation |
| **intercept** | metadata consulted when a key is *used*, changeable without reload — Assay's callouts (§8) | intercept |
| **ACTIVE / INACTIVE** | whether a component's required keys are all provided by ACTIVE components (§6) | fiber state |

## 2. The manifest

Every component MUST carry a `component.yaml`. It lives at the unit's own root when the unit
has one (`plugins/assay/skills/<name>/`, `plugins/assay/hooks/`, `tools/desk/`,
`statusgen/`), and at `components/<name>/component.yaml` for units that are forge-side or
scaffold-side and have no source directory (labels, reviewer App binding, main guard, the
statusgen CI workflow, the roster). A lint discovers manifests by filename.

```yaml
component: assay/pr-review-desk        # id; namespace/name; MUST be unique in the tree
version: 0.3.0                         # semver; the plugin/umbrella tag for bundled units
provides:
  - assay.desk.role.pr-review          # keys this component installs (§3)
inject:
  required:
    - key: assay.harness               # a key with no ACTIVE provider → this component INACTIVE
    - key: assay.desk.verbs
      range: ">=0.27.0 <1.0.0"         # optional semver range on the provider's version
    - key: assay.roster.trust          # trust surface: MUST stay fail-closed (§6.2)
  optional:
    - key: assay.roster.ext.risk-callout   # absent → the component runs with its default
apply:                                 # ordered; each step is one effect and its reverse (§4)
  - id: skill-body
    effect: install skill directory into the harness plugin cache
    boundary: inside
    inverse: remove the directory
  - id: review-labels
    effect: create labels review-request, raised-by:reviewer on the forge
    boundary: outside
    ledger: label                      # record what was created, by id
    compensation: delete the label if no issue carries it; else list for a human
intercept:                             # metadata this component reads at use time (§8)
  - assay.roster.ext.risk-callout
config: {}                             # schema of the entry's config payload, if any
```

Rules:

- `component`, `version`, `provides`, `inject`, `apply` are REQUIRED; `intercept` and
  `config` are OPTIONAL. An empty `provides` or `apply` MUST still carry the key.
- A key MUST NOT be installed by a component that does not list it in `provides`.
- A component MUST NOT read a key it does not list in `inject` (required or optional). This
  is the capability rule of the paper's §6.3: the full set of things a component can touch is
  known before it runs, so an adopter can review it at install time.
- `inject.required` entries with a `range` MUST be checked against the provider's `version`.
  A provider outside the range is *not a provider* for activation purposes (§6).

## 3. Keys and the catalogue

Keys are namespaced `assay.<area>.<name>` so that two components cannot collide on a local
name (the paper's §6.6 key-collision problem, solved by namespacing). Adopter-defined keys
use their own top-level namespace and MUST NOT start with `assay.`.

Initial catalogue (brief 00 owns the authoritative list, at `components/KEYS.md` (planned)):

| Key | Provided by | Meaning |
|---|---|---|
| `assay.streams` | streams scaffold | `docs/streams/` exists with a README and registers |
| `assay.board` | statusgen | `STATUS.md` is generated and linted |
| `assay.registers` | registers scaffold | FINDINGS / INTAKE / RETRO per-entry files |
| `assay.ci.statusgen` | CI workflow component | the lint runs on every push |
| `assay.main-guard` | main-guard component | `core.hooksPath` + pre-push guard installed |
| `assay.labels` | labels component | the `review-request` / `raised-by:*` label set exists |
| `assay.roster.trust` | roster component | the fail-closed trust surface (`ASSAY_BLESS_LOGIN`, `ASSAY_TRUSTED_LOGINS`, `ASSAY_TRUSTED_BOT_SLUGS`, `ASSAY_ALLOWED_REPOS`, `ASSAY_HUMAN_LOGIN_MAP`) |
| `assay.roster.ext.<name>` | roster component | one key per adopter extension (`risk-callout`, `writeguard-callout`, `repo-aliases`, `release-repo`, `scan-repos`, …) |
| `assay.desk.verbs` | desk-tools bundle | the pinned desk binaries on PATH |
| `assay.desk.role.<role>` | each desk-role skill | the role's procedure is installed |
| `assay.forge` | forge adapter | a forge (GitHub, GitLab) reachable with the roster's identities |
| `assay.harness` | harness adapter (exclusive, §9) | the agent harness the skills and hooks are shaped for |
| `assay.reviewer-identity` | reviewer App binding | the App whose review is a verdict |
| `assay.hooks.session-start` | hooks component | resident rules and board state injected at boot |
| `assay.hooks.pre-tool` | hooks component | the write guard on tool calls |

## 4. Effects and their reverses

Every `apply` step MUST carry exactly one of:

- **`inverse`** — when the step's target is *inside* the boundary (§5). Running the inverse
  MUST leave the target as it was before the effect, and the pair MUST be mutation-tested:
  install → disable → the tree is byte-identical to before install (§10 gives the row).
- **`ledger` + `compensation`** — when the target is *outside* the boundary. The step MUST
  write a ledger line naming what it created (§5); the compensation MUST restore the target
  up to an equivalence the manifest states ("the label is gone" / "the ruleset entry is
  removed"), or, when that is not safe to do unattended, MUST list the item for a human.

`disable` (§7) replays reverses in LIFO order of the ledger. A component MUST NOT rely on
another component's reverse to undo its own effect: reverses are local to the component
that made the effect. That the reverse actually reverts the effect is an obligation on the
component author which the tooling does not verify beyond the mutation row — the same
limit the paper places on its runtime.

## 5. The system boundary and the ledger

A location is **inside** the boundary when Assay can modify it exclusively and restore it:
files under `docs/streams/`, `STATUS.md`, `.githooks/`, `.assay/`, the plugin cache, the
installed binaries, `refs/dispatch/*`. A location is **outside** when either ability fails:
forge Apps and their installations, branch rulesets, labels already carried by issues,
org/repo Actions variables, anything merged to a branch, anything another party reads or
writes concurrently.

The ledger is `.assay/ledger.jsonl` in the adopter repo, append-only, one line per outside
effect: `{component, version, step, kind, id, created, by}`. The reconcile engine reads it to
know what to compensate; a human reads it to know what Assay left on the forge. An outside
effect with no ledger line is a defect the lint MUST flag.

Removing a forge App, a ruleset entry, or a label that issues carry is `gate: human` work.
The ledger makes it *listable*; it does not make it automatic.

## 6. Activation

### 6.1 The rule

A component is ACTIVE when, and only when, every `inject.required` key has an ACTIVE
provider within range. Otherwise it is INACTIVE and MUST be reported in the three-state
form of `brief-v1` §8: `could-not-check: assay/<name> inactive — assay.<key> has no provider`.
An INACTIVE component's verbs and hooks MUST refuse with that message rather than run with
a missing input. A component whose provider *arrives* later (a key set, a tool installed)
MUST become ACTIVE with no further step.

Cycles among `inject.required` are detectable from manifests alone and the lint MUST report
them; a cycle leaves all its members INACTIVE.

### 6.2 What this changes and what it does not

Today an unknown or malformed `ASSAY_*` key fail-closes every desk verb (the tools share one
config loader). Under this model the trust surface stays fail-closed — a component injecting
`assay.roster.trust` MUST refuse when that surface is unset or malformed, exactly as today —
but the *extension* keys are per-component: a rejected `assay.roster.ext.repo-aliases`
deactivates the components that inject it and no others. The blast radius becomes the
declared dependency set instead of the process. This never weakens a security control; it
narrows which components a broken non-security value can stop.

## 7. The desired-state record and the reconcile engine

`.assay/config.yaml` (planned) in the adopter repo is the authoritative record of what is installed:

```yaml
schema: assay-config-v1
umbrella: v0.27.0                # the release the entries below were pinned from
entries:
  - id: streams
    component: assay/streams-scaffold
    version: 0.27.0
  - id: pr-review
    component: assay/pr-review-desk
    version: 0.3.0
    config: {pool-size: 3}
    intercept: {assay.roster.ext.risk-callout: /path/to/callout}
    disabled: false
  - id: dailies
    component: assay/dailies
    version: 0.3.0
    disabled: true               # installed record kept; effects reversed
```

The reconcile engine diffs the record against the ledger and the tree, and dispatches on which
field of an entry changed, applying the least disruptive operation:

| changed field | operation |
|---|---|
| `component`, `version` | rebuild the entry: reverse the old, apply the new |
| `intercept` | update in place; no reverse, no re-apply (§8) |
| `config` | hand to the component, which decides whether the change is material |
| `disabled` | set → replay reverses (LIFO); cleared → apply again |
| entry added / removed | apply / reverse |

The end state MUST be a function of the final record alone: whatever order the reconcile engine
applies operations in, the tree quiesces where a fresh install of that record would have
left it. `deskmigrate` becomes the reconcile engine's driver; `upgrade-assay` becomes an edit to
`umbrella` and the entry versions it pins. `.assay-versions` MAY remain as a derived view
of the record for the pinned-binary channel; it MUST NOT be a second source of truth.

## 8. Intercept — the callouts

House-specific behaviour is supplied as *intercept metadata*: a value resolved at the moment a
key is used, from `~/.config/assay/roster.env` or org/repo Actions variables, never from
component source. This is Assay's existing callout mechanism (`ASSAY_RISK_CALLOUT`,
`ASSAY_WRITEGUARD_CALLOUT`, …) given its place in the model:

- an intercept value MUST NOT appear in any component's source, tests, or fixtures;
- changing an intercept value MUST NOT trigger a rebuild or re-apply of any entry;
- a component MUST list in `intercept:` every metadata key it consults, so a reader can
  see where house behaviour enters.

## 9. The harness as an exclusively-bound key

`assay.harness` is provided by exactly one ACTIVE adapter at a time (the paper's *exclusive
binding*). The Claude Code, Codex, and Cursor adapters are components whose `apply` installs
the harness-specific shape (plugin manifest, hook wiring, generated rules file) and whose
`provides` is `assay.harness` with a `flavour` field. Skills and hooks inject `assay.harness`
and read the flavour where they must branch. Switching harness is an entry change the
reconcile engine performs: reverse one adapter, apply the other, and every dependent re-activates
against the new provider. Three parallel hand-kept manifests become one declaration.

## 10. Conformance rows (for every component brief)

A brief that adds or changes a component MUST carry these Verify rows, in addition to the
brief-v1 rules:

| # | Command | Expect |
|---|---------|--------|
| C1 | lint manifests: every `inject` key resolves to a `provides`, no cycles | exit 0 |
| C2 | install the component, `disable` it, diff the tree against the pre-install snapshot (inside the boundary) | empty diff |
| C3 | for each outside step: a ledger line exists naming the created item | one line per step |
| C4 | unset one `inject.required` extension key, run a verb of the component | refuses with the `could-not-check … inactive` message |
| C5 | with C4's key unset, run a verb of a component that does NOT inject it | runs normally |
| C6 | unset a trust-surface key, run any verb | refuses (fail-closed, unchanged) |

## 11. Versioning

Component versions are semver. `inject.range` is the compatibility statement between
independently released components; a provider outside the range is not a provider. The
umbrella version names a *set* of entry versions — it is the paper's group entry — so an
umbrella upgrade is a reconcile over every entry it pins. This document's own version
follows `spec/README.md`'s draft policy once promoted.

## 12. What exists today (known divergences)

- No manifests, no key catalogue, no lint. The nearest thing is the prose inventory in
  `docs/adopting-assay.md` §2 and the `ASSAY_*` list in `docs/adopting-assay.md` (roster section).
- No inverses and no `disable` verb; install is idempotent and refuse-not-clobber, with no
  reverse. The only removal text in the tree is the shim-off-PATH note.
- No ledger. What Assay creates on a forge (Apps, rulesets, labels, Actions variables) is
  recoverable only by reading the adoption runbook.
- One shared config loader: an unknown or rejected `ASSAY_*` key fail-closes every verb.
- `.assay-versions` + `deskversion` + `deskmigrate` are a version marker and a migration
  runner, not a desired-state record and a reconcile engine.
- Callouts already behave as §8 requires. `SYNC-FROM`, `paired-versions.yaml`, and the
  `PARITY` check already play the paper's stale-entry-detection role for the file layer.
- Three harness manifests are kept in parallel (`.claude-plugin`, `.codex-plugin`, a
  generated `cursor/*.mdc`).

## 13. Non-goals

Adopting Cordis or any in-process runtime; hot module replacement of a running desk
process; structural (as opposed to nominal-plus-range) typing of keys across components;
sandboxing untrusted components — the paper's §6.3 places that outside the model, and Assay
places it in the forge's permission layer already.
