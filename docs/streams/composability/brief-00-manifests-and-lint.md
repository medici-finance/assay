---
brief: composability/00
title: Component manifests, key catalogue, and the resolve/cycle lint
why: >-
  Assay's units depend on each other through undeclared env keys, verb names, and paths, so
  nothing can tell which component a broken value will stop, and nothing can order an install
  except a hand-written list. Declaring provides/inject per unit, once, is the map every later
  brief reads — the blast-radius fix, the inverses, the reconcile engine, and the harness switch all
  need it — and the lint makes an undeclared dependency a CI failure instead of an outage.
wave: 0
depends: []
unblocks: ["composability/01", "composability/02", "composability/03", "composability/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §2 (manifest shape), §3 (key catalogue), §6.1 (cycle rule)"
  - "arXiv 2608.25512 §3.2 reactive coeffects (a component declares required and provided keys), §6.5 (cycles are detectable from declarations alone), §6.6 (key namespacing against collision)"
  - "docs/adopting-assay.md §2 Component inventory — the prose list this brief makes machine-readable"
  - "docs/adopting-assay.md (roster section) — the ASSAY_* keys, split into fail-closed trust surfaces and adopter extensions with defaults"
  - "Ian's direction (2026-09-07): turn the paper's model into a stream; start with the blast-radius fix"
---

# Brief 00 — Component manifests, key catalogue, and the resolve/cycle lint

## Context

files:
- **create** `components/KEYS.md` (planned) — the authoritative key catalogue (the table in
  `component-model.md` §3, kept here from now on; §3 links to it).
- **create** one `component.yaml` per unit of the `docs/adopting-assay.md` §2 inventory:
  `plugins/assay/skills/<name>/component.yaml` for every skill directory,
  `plugins/assay/hooks/component.yaml` (planned), `tools/desk/component.yaml` (planned), `statusgen/component.yaml` (planned),
  and `components/<name>/component.yaml` for the forge-side / scaffold-side units with no source
  directory: `streams-scaffold`, `registers-scaffold`, `ci-statusgen`, `main-guard`, `labels`,
  `roster`, `reviewer-app`, `forge-github`.
- **create** `tools/desk/cmd/deskmanifest/` — `deskmanifest lint [--root <dir>]`: discovers every
  `component.yaml`, parses it against §2, resolves every `inject` key to a `provides`, checks
  `range` against the provider's `version`, reports cycles among `inject.required`. Three-state
  output per `spec/brief-v1.md` §8.
- **edit** the existing lint CI job so `deskmanifest lint` runs on every push.
- **edit** `docs/streams/composability/component-model.md` §3 — replace the inline table with a
  link to `components/KEYS.md` (planned) once the catalogue exists.

facts:
- **Manifests declare; they do not yet act.** This brief installs nothing and reverses nothing.
  `apply` steps are written down with `boundary:` and a placeholder `inverse:`/`compensation:`
  line; brief 02 makes them real. Getting the declarations wrong is cheap to fix, so this brief
  optimises for coverage of the inventory over perfection of any one manifest.
- **Every unit in the §2 inventory gets a manifest, including the human-gated ones** (reviewer
  App, main guard). A manifest for a unit a human installs still declares what it provides, so
  its dependents can be resolved.
- **Trust keys and extension keys are different `inject` entries.** The five fail-closed surfaces
  are one key, `assay.roster.trust`; each extension is its own `assay.roster.ext.<name>`. Brief 01
  depends on this split being in the manifests, not in code.
- **Key namespace is `assay.<area>.<name>`**; adopter keys use their own top-level namespace.
  The lint MUST reject a manifest that provides a key outside `assay.` from a component whose
  id is in the `assay/` namespace.
- **Cycle detection is static.** A cycle among `inject.required` is a lint PROBLEM; an
  `inject.optional` edge never forms a cycle.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`; a non-implementer runs the Verify
  table on merged main.
- If the inventory in `docs/adopting-assay.md` §2 and the running tree disagree about what a
  unit is, or a key's owner is unclear, raise `NEEDS_CONTEXT` on the PR with the specific
  conflict rather than inventing a component.
- No house-specific value (an App slug, a repo name, a callout path) enters any manifest,
  fixture, or test. Manifests carry keys and shapes; values stay in the roster.

## Task

1. Write `components/KEYS.md` (planned) from `component-model.md` §3, one row per key, with the providing
   component and a one-line meaning. Add the keys the inventory turns out to need that §3 missed;
   note each addition in the PR body.
2. For each unit in the §2 inventory, write its `component.yaml` per `component-model.md` §2:
   `component`, `version` (the current plugin/umbrella tag for bundled units), `provides`,
   `inject` (required vs optional, trust vs extension), `apply` (ordered steps with `boundary:`
   set from §5; reverse lines may say `TODO composability/02`), `intercept` where the unit
   reads a callout.
3. Implement `deskmanifest lint`: discovery, schema check, resolution, range check, cycle
   detection, three-state report, exit 0 / 1 / 2 for clean / problems / could-not-check.
4. Wire it into the existing lint CI job next to `statusgen --lint`.
5. Run the lint on the tree; fix the manifests until it is clean. Record the unresolved keys
   you found on the way in the PR body — each is a dependency that was real and undeclared.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `test -f components/KEYS.md && grep -c '^\| \`assay\.' components/KEYS.md` | exit 0; count ≥ 14 |
| 2 | check | `find . -name component.yaml -not -path './.git/*' \| wc -l` | ≥ 18 (every §2 unit) |
| 3 | check | `deskmanifest lint --root .` | exit 0; last line `checked-clean` |
| 4 | check +mutation | mutation: add `- key: assay.nonexistent` to `inject.required` of one manifest; `deskmanifest lint --root .` | exit 1; names the manifest and the unresolved key; revert |
| 5 | check +mutation | mutation: make two manifests require each other's `provides` key; `deskmanifest lint --root .` | exit 1; reports the cycle with both ids; revert |
| 6 | check +mutation | mutation: set a `range: ">=99.0.0"` on an `inject.required` whose provider is at 0.x; `deskmanifest lint --root .` | exit 1; reports provider out of range; revert |
| 7 | check | `deskmanifest lint --root /nonexistent` | exit 2; last line starts `could-not-check` |
| 8 | check | `grep -rn 'assay.roster.trust' --include=component.yaml . \| wc -l` | ≥ 1; and `grep -rn 'assay.roster.ext.' --include=component.yaml . \| wc -l` ≥ 1 (the trust/extension split is in the manifests) |
| 9 | check | `cd tools/desk && go test ./cmd/deskmanifest/...` | exit 0 |
| 10 | check | neighbour: `statusgen --lint` on the tree | exit 0 (the new files and the CI edit do not disturb the board lint) |

The presence rows (1, 2, 8) gate presence, not quality; whether a manifest's declarations are
the *right* ones is the review gate's judgement.

## Evidence

Implemented on branch `feat/composability-00`. All ten Verify rows run locally (offline,
`KUBECONFIG=/dev/null`); `deskmanifest` and `statusgen` built from the in-tree source.

| # | Result |
|---|--------|
| 1 | PASS — `test -f components/KEYS.md` exit 0; `grep -c '^\| \`assay\.'` = 30 (≥ 14) |
| 2 | PASS — `find . -name component.yaml -not -path './.git/*' \| wc -l` = 23 (≥ 18) |
| 3 | PASS — `deskmanifest lint --root .` exit 0; last line `checked-clean` (23 manifests) |
| 4 | PASS — mutation (add `- key: assay.nonexistent` to `inject.required`) → exit 1, `PROBLEM: … (assay/desk-tools): inject.required key "assay.nonexistent" has no provider (unresolved)`; reverted |
| 5 | PASS — mutation (two manifests require each other's `provides`) → exit 1, `PROBLEM: cycle among inject.required: assay/desk-tools -> assay/forge-github -> assay/desk-tools`; reverted |
| 6 | PASS — mutation (`range: ">=99.0.0"` on a 0.x provider) → exit 1, `PROBLEM: … inject.required key "assay.desk.verbs" needs a provider in range ">=99.0.0"; provider(s) out of range: assay/desk-tools@0.28.0`; reverted |
| 7 | PASS — `deskmanifest lint --root /nonexistent` → exit 2, last line `could-not-check: root /nonexistent does not exist` |
| 8 | PASS — `grep -rn 'assay.roster.trust' --include=component.yaml .` = 9 (≥ 1); `grep -rn 'assay.roster.ext.' --include=component.yaml .` = 12 (≥ 1) |
| 9 | PASS — `cd tools/desk && go test ./cmd/deskmanifest/...` exit 0 |
| 10 | PASS — `statusgen --root . --lint` exit 0 (`LINT: PASS`; the pre-existing NOTICEs are untouched by this change) |

**Fail-first (rule 9).** The lint is the guard, and rows 4/5/6 ARE its mutation entries: each
mutates one manifest into a state the lint must reject, and each was observed RED (exit 1,
with the message quoted above) before being reverted. The unmutated tree lints clean (row 3),
so the three reds are evidence the resolution, cycle, and range checks have teeth — a reviewer
re-runs them by applying the row's mutation to the named manifest.

An independent runner records a second run on merged main before `verified`.

## Review

gate: model — pending.
