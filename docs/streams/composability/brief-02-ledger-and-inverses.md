---
brief: assay:assay:composability:02
title: Install ledger, paired inverses, and the `disable` verb
why: >-
  There is no way to remove Assay from a repo, or to back out a half-failed install: labels,
  hooks, workflows, refs, a pinned binary, and forge-side Apps are left for a human to find and
  hand-scrub. Pairing every apply step with its reverse, and ledgering what is created outside
  the boundary, gives adopters a real "turn it off" and makes "try it on one repo for a week"
  a safe offer. It is also the precondition for a reconcile engine: you cannot rebuild an entry you
  cannot reverse.
wave: 1
depends: ["composability/00"]
unblocks: ["composability/03", "composability/05"]
effort: L
gate: human
gate-why: >-
  The `disable` verb deletes things: files and hooks in an adopter's repo, and — through
  compensations — labels, rulesets, and App installations on a forge that other people's
  issues and branches may depend on. A wrong inverse destroys adopter state that Assay did not
  create; a wrong compensation removes a control (a ruleset bypass, a guard hook) from a live
  repo. Irreversible in the forge case; the tree case is recoverable only if the adopter has a
  clean checkout.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §4 (effects and reverses), §5 (system boundary and ledger), §10 rows C2/C3"
  - "arXiv 2608.25512 §3.1 (every context transformation carries an inverse the runtime holds; inverses compose LIFO), §6.1 (system boundary: acquisition vs emission; withholding and compensation), §5.3 (removal is effortless for the author because inverses are composed by the abstraction)"
  - "docs/adopting-assay.md §3 — the install primitives whose reverses this brief writes; the current tree has no uninstall verb (survey 2026-09-07: the only removal text is the clusterguard shim-off-PATH note)"
  - "composability/00 — the manifests whose apply steps carry the `boundary:` field this brief pairs reverses with"
consumers:
  - "plugins/assay/skills/install/SKILL.md: follow-up composability/03 (install writes the ledger from this brief on; the skill body gains the ledger step when the reconcile engine lands)"
  - "docs/adopting-assay.md §3: follow-up composability/05 (the runbook gains the removal path in the adopter-doc delta)"
  - "tools/desk/cmd/deskinstall: fixed-here (the binary installer writes its ledger line)"
  - "tools/desk/cmd/clusterguard shim-off-PATH note: fixed-here (superseded by the shim's inverse)"
version: 1
id: 3c7ab933-8f9d-4de7-831f-dabc170095dc
---

# Brief 02 — Install ledger, paired inverses, and the `disable` verb

## Context

files:
- **edit** every `component.yaml` from brief 00: replace each `TODO composability/02` reverse
  line with a real `inverse:` (inside) or `ledger:` + `compensation:` (outside) per
  `component-model.md` §4.
- **create** `tools/desk/internal/deskkit/ledger.go` (planned) — append-only writer/reader for
  `.assay/ledger.jsonl` (`{component, version, step, kind, id, created, by}`).
- **edit** `tools/desk/cmd/deskinstall/` and the scaffold paths `assay:install` drives so that
  every outside step writes its ledger line at the moment of creation (label created, ruleset
  entry added, App installation observed).
- **create** `tools/desk/cmd/deskdisable/` — `deskdisable <component> [--dry-run] [--yes]`:
  replays the component's reverses LIFO; inside steps run; outside steps run their compensation
  only when it is marked `unattended: true` in the manifest, otherwise they are printed as a
  human checklist. `--dry-run` prints the plan and exits 0 without touching anything.
- **edit** `tools/desk/component.yaml` (planned) — provides `assay.desk.verbs` now includes `deskdisable`.

facts:
- **Inside the boundary** (`component-model.md` §5): `docs/streams/**`, `STATUS.md`,
  `.githooks/`, `.assay/`, the plugin cache, installed binaries under the install prefix,
  `refs/dispatch/*`. Reverses here MUST leave the tree byte-identical to the pre-install
  snapshot; the mutation row proves it.
- **Outside the boundary**: forge Apps and installations, branch rulesets and bypass lists,
  labels that issues carry, org/repo Actions variables, anything merged. Each such step MUST
  ledger; compensations default to *listed for a human*. Only a compensation the manifest
  marks `unattended: true` runs on its own, and this brief marks NONE of the forge-side ones so.
- **Reverses are local.** A component reverses only its own effects. `deskdisable` refuses to
  disable a component that an ACTIVE component still `inject.required`s, unless `--cascade`
  lists the dependents to disable first (in reverse dependency order).
- **The ledger is the only memory of outside effects.** An outside step with no ledger line is
  a lint PROBLEM in `deskmanifest lint` (extend brief 00's lint, one check).
- **The reviewer-App and main-guard components are human-installed today**; their manifests'
  reverses are human checklists, not code, and `deskdisable` prints them.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`.
- **`deskdisable` MUST NOT run against any repo other than a throwaway fixture repo during
  implementation and verification.** No forge-side compensation runs unattended in this brief.
- If a step's reverse cannot be written without deleting something Assay did not create,
  the step gets `compensation: list-for-human`, never a best-effort delete; raise
  `NEEDS_CONTEXT` if the boundary classification of a location is unclear.

## Task

1. For each manifest, write the reverse of each apply step. Classify by boundary; inside steps
   get an `inverse:` that restores the pre-effect state; outside steps get `ledger:` and a
   `compensation:` (default `list-for-human`).
2. Implement the ledger writer/reader; make every outside step in `deskinstall` and the
   scaffold paths write its line at creation.
3. Implement `deskdisable`: read manifests + ledger, compute the LIFO plan, honour
   dependents (refuse or `--cascade`), run inside reverses, run only `unattended` compensations,
   print the human checklist for the rest. `--dry-run` never touches anything.
4. Extend `deskmanifest lint` with the "outside step without ledger" check.
5. Fixture: a throwaway repo with a scripted install of the inside-only components; snapshot,
   install, `deskdisable`, diff. Record the diff in Evidence.

## Verify

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -rn 'TODO composability/02' --include=component.yaml . \| wc -l` | 0 |
| 2 | in a throwaway fixture repo: `git stash -u; tar cf /tmp/pre.tar .` then scripted install of `assay/streams-scaffold`, `assay/main-guard` (inside steps only), then `deskdisable assay/main-guard --yes && deskdisable assay/streams-scaffold --yes`, then `tar df /tmp/pre.tar` | empty diff (tree byte-identical) |
| 3 | after row 2's install step, before disable: `wc -l .assay/ledger.jsonl` and `grep -c '"kind":"label"' .assay/ledger.jsonl` | one ledger line per outside step executed (labels created in the fixture) |
| 4 | `deskdisable assay/labels --dry-run` in the fixture | exit 0; prints a plan; every label line says `list-for-human`; nothing deleted (`gh label list` unchanged) |
| 5 | `deskdisable assay/desk-tools --yes` while `assay/pr-review-desk` is ACTIVE and requires `assay.desk.verbs` | non-zero; refuses naming the dependent; nothing changed |
| 6 | mutation: remove the `ledger:` line from one outside step; `deskmanifest lint --root .` | exit 1; names the step; restore |
| 7 | mutation: in the fixture, edit one inside `inverse` to a no-op; run row 2 | `tar df` reports a leftover file; the test suite fails; restore |
| 8 | `cd tools/desk && go test ./internal/deskkit/... ./cmd/deskdisable/... ./cmd/deskmanifest/...` | exit 0 |
| 9 | neighbour: `deskinstall --help` and a normal `deskinstall` dry-run against the fixture | exit 0; installer still works and now prints the ledger path |
| 10 | flow: fixture install → `deskdisable` → re-install → `deskmanifest lint` | all exit 0; second install identical to first (idempotent after a full reverse) |

## Threat model

| Failure mode | Caught by |
|---|---|
| An inverse deletes a file Assay did not create (adopter's own `docs/streams` content) | row 2 (byte-identical diff includes adopter files placed in the fixture before install) |
| A compensation deletes a label that issues carry | row 4 (forge-side compensations are `list-for-human`; dry-run shows no deletion); design: no `unattended: true` on any forge-side step in this brief |
| Disabling a provider strands its dependents | row 5 |
| An outside effect is created but never ledgered, so removal cannot find it | row 6 (lint) and row 3 (count) |
| A no-op inverse passes silently | row 7 |
| `deskdisable` run against a real repo during development | no row; ground rule + review-only — the reviewer checks the PR's Evidence names only the fixture path |

single-point-of-failure: the `boundary:` classification in each manifest — an outside step
mis-labelled inside gets a destructive `inverse` instead of a listed compensation. Layer
behind it: `deskdisable` refuses to act on any forge object it cannot find in the ledger, so a
mis-classified forge step has no id to act on and falls through to the human checklist.

## Evidence

**Implementer run — 2026-09-12.** All fixture rows are Go tests under
`tools/desk/cmd/deskdisable` using a fresh `t.TempDir()` per test (never a real repo — ground
rule honoured); each test name below stands in for "the fixture repo path" the row asks for,
since a `t.TempDir()` path is ephemeral by construction and the test itself is the reproducible
artifact.

| # | Command | Result |
|---|---------|--------|
| 1 | `grep -rn 'TODO composability/02' --include=component.yaml . \| wc -l` | `0` — checked-clean |
| 2 | `go test -run TestDisable_InsideOnly_ByteIdenticalAfterDisable ./cmd/deskdisable/... -timeout 60s` | PASS — fixture installs `assay/streams-scaffold` + `assay/main-guard` (plus pre-existing "adopter" content in `docs/streams/` and at the repo root), disables `assay/main-guard` then `assay/streams-scaffold` (row 2's literal order), and the Go-native tree-diff (`treeDiff`, equivalent to `tar cf`/`tar df`) is empty — byte-identical, adopter content untouched |
| 3 | `go test -run TestFixture_LedgerLineAfterOutsideInstall ./cmd/deskdisable/... -timeout 60s` | PASS — after `assay/labels`' outside step is recorded, `.assay/ledger.jsonl` has exactly 1 line and `grep -c '"kind":"label"'` = 1 |
| 4 | `go test -run TestDisable_LabelsDryRun_ListsForHuman ./cmd/deskdisable/... -timeout 60s` | PASS — `--dry-run` exits clean, every step's detail is prefixed `list-for-human:`, names the ledgered id (`review-request`), and touches neither the tree nor the ledger |
| 5 | `go test -run TestDisable_RefusesActiveDependent ./cmd/deskdisable/... -timeout 60s` | PASS — disabling `assay/desk-tools` while `assay/pr-review-desk` requires `assay.desk.verbs` refuses (`ExitRefused`), names the dependent, and changes nothing; `TestDisable_CascadeDisablesDependentFirst` additionally proves `--cascade` disables the named dependent BEFORE the target |
| 6 | manual mutation against the real tree: removed `ledger: label` from `components/labels/component.yaml`, ran `go run ./cmd/deskmanifest lint --root ../..` → exit 1, `PROBLEM: components/labels/component.yaml (assay/labels): outside apply step "label-set" has no ledger: value ...`; restored the file (`diff` against the pre-mutation copy is empty) and re-ran → `checked-clean: 23 manifest(s)...` | PASS |
| 7 | `go test -run TestDisable_MutationNoOpInverseIsCaughtByTreeDiff ./cmd/deskdisable/... -timeout 60s` | PASS — with `main-guard`'s registered inverse swapped for a no-op, the disable call itself does not error, but the tree-diff check catches the leftover `.githooks/pre-commit` + `core.hooksPath`; the executor is then restored and the same flow leaves a byte-identical tree |
| 8 | `go test -timeout 180s ./internal/deskkit/... ./cmd/deskdisable/... ./cmd/deskmanifest/...` | exit 0 — `ok  .../internal/deskkit  ~34-52s`, `ok .../internal/deskkit/untrustcorpus`, `ok .../cmd/deskdisable  ~1-5s`, `ok .../cmd/deskmanifest` (includes the pre-existing house-wide `TestRegistryCoversCmdBinaries` and `TestCorpusHasNoWithheldStreamPaths` guards, both green after registering `deskdisable` in `audittoolkey.go` and scrubbing the stream slug from every new/edited tools/desk file's prose) |
| 9 | `go test -run TestWindowsInstallVerifiesCorrectHash ./cmd/deskinstall/... -timeout 30s` and `go build ./cmd/deskinstall/...` | PASS — `deskinstall`'s success output now contains `ledger: .assay/ledger.jsonl (this install has only inside-boundary effects; nothing to record)`; `--help` documents the same; the installer's existing acquire→verify→place behaviour is unchanged (all prior deskinstall tests still pass) |
| 10 | `go test -run TestDisable_FullReverseThenReinstallIsIdempotent ./cmd/deskdisable/... -timeout 60s` | PASS — install → disable both components → re-install produces a tree byte-identical to the first install |

**Extra row (Review's invitation, §"which Verify row proves..."):**
`TestDisable_MisclassifiedOutsideStepRefusesRatherThanExecute` — a fixture component claims
`boundary: inside` for a step that is really a forge/outside effect. `insideExecutors` is a
closed allowlist keyed by `(component, step-id)`, verified safe one at a time; the mis-labelled
step has no entry, so `deskdisable` refuses before touching anything rather than execute a
destructive "inverse" against something it never actually created locally. This is the row the
Review section asks for — not "no row".

**Scope notes, recorded rather than silently assumed:**
- `insideExecutors` (real, executed reverses) covers exactly `assay/streams-scaffold`,
  `assay/main-guard`, and `assay/registers-scaffold` — the components whose install target is a
  concrete, adopter-repo-relative path today. The plugin skills, the hook wiring, the
  `assay/ci-statusgen` workflow file, and the `assay/statusgen`/`assay/desk-tools` binaries have
  real, non-TODO `inverse:` prose (Task 1) but no registered executor yet: their targets are
  either harness-defined (component-model.md §9, not landed until a later brief) or determined
  by installer flags the manifest does not carry. `deskdisable` refuses by name rather than
  guess (proven by `TestDisable_UnregisteredInsideStepRefusesBeforeTouchingAnything`) — an
  honest could-not-check, not a rounded-up success.
- The `tools/desk/cmd/clusterguard shim-off-PATH note` consumer listed in this brief's
  frontmatter does not apply: no `component.yaml` exists for `clusterguard` anywhere in this
  tree (brief 00 never created one — it is not one of the 21 manifests brief 00 shipped), so
  there is nothing here to supersede `docs/adopting-assay.md`'s manual shim-off-PATH note. That
  doc's removal-path delta is explicitly this stream's brief 05, not this one.
- `deskinstall`'s own apply steps are all `boundary: inside` (installed binaries are explicitly
  inside per component-model.md §5), so it never had an outside step to ledger; Task 2's mention
  of it is honoured as "name the ledger path" (row 9), not as a ledger write it has nothing to
  make.
- The `plugins/assay/skills/install/SKILL.md` and `docs/adopting-assay.md` §3 consumers are
  unchanged in this PR, exactly as their own entries specify ("follow-up composability/03" and
  "follow-up composability/05" respectively).

## Review

gate: human — pending. The reviewer answers (1) which single control stands between a wrong
reverse and adopter damage, and whether it is acceptable; (2) which Verify row proves the lower
layer (ledger-id refusal) catches a mis-classified step with the upper layer (manifest
classification) wrong — row 4 with a deliberately mis-labelled fixture step, if the implementer
adds it; otherwise the answer is "no row" and must be recorded as such.
