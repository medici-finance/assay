---
brief: assay:assay:windows-port:07
title: deskinstall --harness cursor — place the skills/references tree and write the AGENTS.md bindings
why: >-
  On Claude Code, installing Assay into a repo is a marketplace command. On Cursor it is a
  five-step manual copy an adopter performs by hand — and the documentation for it exists in three
  places that each state it slightly differently (`docs/adopting-assay.md:1087-1098`,
  `plugins/assay/references/cursor.md:15-25`, `plugins/assay/cursor/packaging.md:12-17`). Step 2 of
  that copy is the one adopters get wrong: copying only `skills/` leaves the skills'
  `../../references/*.md` includes dead, and nothing reports it — the install looks finished and is
  quietly broken. Cursor's install mechanism IS file placement, which is precisely the kind of work
  a tool should do idempotently and then be able to re-check. This brief makes step 2 of the
  three-command install one command on any OS, and gives it a `--check` so drift is reportable
  rather than discovered.
wave: 3
depends: ["windows-port/03"]
unblocks: ["windows-port/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-11 by windows-port authoring session (driver ask, 2026-09-11)
sources:
  - "driver's ask (2026-09-11): step 2 of a three-command Windows install is `deskinstall --harness cursor --forge gitlab --repo <path>` — a harness mode that places skills + references and writes the AGENTS.md bindings, idempotent, with a --check that reports drift"
  - "plugins/assay/cursor/packaging.md:12-15 — 'Cursor reads SKILL.md skills, AGENTS.md, and .cursor/rules/*.mdc directly from the repo tree (HP/12 §2.10 install-mechanism), so there is no per-harness plugin manifest — the packaging IS the instruction files'"
  - "plugins/assay/references/cursor.md:11-25 — the five manual steps, including 'Skills-only copies leave those links dead' (step 2) and 'harnessgen cursor output is optional' (step 5)"
  - "docs/adopting-assay.md:1085-1098 — the same five steps, a third time, in the adopter runbook"
  - "tools/harnessgen/cursor.go:39,45-52 — cursorPathsFor: the generator's outputs are BUNDLE-relative (bundle/cursor/assay.mdc, bundle/skills); it generates the bundle's own .mdc rule and never writes into an adopter repo"
  - "tools/harnessgen/cursor.go:103-157 — the coverage rule over plugins/assay/cursor/packaging.md (every skills/*/SKILL.md packaged or excluded-with-reason, exit 2 otherwise) and the --check drift verdict (exit 1 on DRIFT, 0 on clean)"
  - "tools/harnessgen/main.go:48-57 — harnessgen's three verbs (resident, codex, cursor); there is no adopter-repo placement verb today"
  - "tools/desk/cmd/deskinstall/main.go:47-66 — deskinstall's current flag surface is --manifest/--dest/--platform only; exit 0 installed & verified, exit 5 refused"
  - "plugins/assay/skills/ — 12 skill directories at the freshness head (adopt, ask-decision, author-brief, install, intake-desk, pdfingest, pr-review-desk, pr-shepherd, the-desk, upgrade-assay, verify-desk, worker-desk), all 12 listed `packaged` in cursor/packaging.md's assay:cursor-packaging block"
  - "plugins/assay/references/ — 4 files at the freshness head (claude-code.md, codex.md, cursor.md, desk-shell.md); the only ../../references/ include in the skills tree today resolves to desk-shell.md"
  - "plugins/assay/codex/AGENTS-assay.md — the shared AGENTS.md fragment Cursor reads natively (cursor.md:66-68), i.e. the bindings text this mode writes rather than invents"
  - "freshness-checked 2026-09-11 @ 35316469 (origin/main): deskinstall has no --harness flag; harnessgen has no adopter-repo placement mode; nothing in the tree writes .cursor/skills or .cursor/references"
consumers:
  - "tools/desk/cmd/deskinstall/: follow-up windows-port/07 (this brief; flips to fixed-here when the implementation adds the harness mode)"
  - "plugins/assay/cursor/packaging.md: out-of-scope (this brief READS the coverage roster as the packaged-set source; it never edits the roster — a skill added to the bundle is the bundle's change, not this mode's)"
  - "tools/harnessgen/cursor.go: out-of-scope (the .mdc generator stays bundle-scoped; this mode CONSUMES its output rather than re-implementing it — see Task 4)"
  - "plugins/assay/references/cursor.md: follow-up windows-port/09 (the doc collapse owns pointing the five manual steps at the one command)"
  - "docs/adopting-assay.md: follow-up windows-port/09 (same)"
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/09 (§Scope's acquisition-only wording is 09's edit)"
exec-tier: strong
exec-tier-why: >-
  Question (b): correctness is cross-artifact. The placed set must agree with the packaging
  coverage roster, the skills' `../../references/*.md` includes must still resolve from their NEW
  location, and the AGENTS.md bindings must be the shared fragment rather than a fourth
  paraphrase — three artifacts that a per-file-correct implementation can still put out of
  agreement, and a wiring error here survives any test that only checks that files appeared.
version: 1
id: 43f518d4-b344-4a88-b8b6-9aca53b99988
---

# Brief 07 — `deskinstall --harness cursor`

## Context

files:
- **create/edit** under `tools/desk/cmd/deskinstall/` — a harness mode (`--harness cursor`) and its
  tests. It is a SECOND mode of the same command, not a second command: the existing
  acquire→verify→place flow (`--manifest`/`--dest`) is untouched and keeps its exit contract.
- **create** `changelog/<branch-slug>.md` — this repo enforces a per-PR fragment.
- **do NOT** edit `plugins/assay/cursor/packaging.md`, `plugins/assay/references/cursor.md`,
  `docs/adopting-assay.md`, or `plugins/assay/skills/install/SKILL.md` — the first is the roster
  this mode reads; the rest are `windows-port/09`'s doc collapse.

facts:
- **Cursor's install mechanism IS file placement.** `plugins/assay/cursor/packaging.md:12-15`:
  "Cursor reads `SKILL.md` skills, `AGENTS.md`, and `.cursor/rules/*.mdc` directly from the repo
  tree (HP/12 §2.10 `install-mechanism`), so there is no per-harness plugin manifest — the
  packaging IS the instruction files." There is no marketplace to call and nothing to register;
  the whole install is: put these files there, and write the bindings.
- **The five steps, and where they are written three times.**
  `plugins/assay/references/cursor.md:15-25` and `docs/adopting-assay.md:1087-1098` both enumerate:
  (1) copy `plugins/assay/skills/*` to `.cursor/skills/` (or `.agents/skills/`); (2) copy
  `plugins/assay/references/*.md` so the skills' `../../references/*.md` includes resolve;
  (3) bindings live in the adopter `AGENTS.md`/`CLAUDE.md`, optionally `.cursor/rules/*.mdc`;
  (4) desk binaries on PATH, `glab`/`--forge gitlab` on GitLab; (5) `harnessgen cursor` output is
  optional. This mode automates (1)-(3); (4) is `windows-port/06`'s PATH write plus `deskinstall`'s
  existing binary install.
- **Step 2 is the failure mode, stated in the source:** "Copying **only** `skills/` leaves those
  includes dead" (`docs/adopting-assay.md:1091-1092`); "Skills-only copies leave those links dead"
  (`cursor.md:17`). At the freshness head the skills tree contains exactly one such include,
  resolving to `plugins/assay/references/desk-shell.md` — so the relative depth the placed tree must preserve is
  `<skills-root>/<skill>/SKILL.md` → `../../references/`, i.e. `references/` must be a SIBLING of
  the skills root, not a child of it.
- **What `harnessgen cursor` does and does not do.** `tools/harnessgen/cursor.go:45-52` resolves
  every path relative to the BUNDLE (`bundle/cursor/assay.mdc`, `bundle/skills`); it generates the
  bundle's own `.cursor/rules`-shaped `assay.mdc` and diffs it under `--check`
  (`cursor.go:150-157`: exit 1 on DRIFT, 0 on clean, exit 2 on could-not-check). It has no notion
  of an adopter repo. This mode therefore CONSUMES `plugins/assay/cursor/assay.mdc` as a file to
  place; it must not re-implement the generation.
- **The packaged set has a machine-readable single source.** `plugins/assay/cursor/packaging.md`'s
  `<!-- assay:cursor-packaging … -->` block lists each skill as `packaged` (bare name) or
  `excluded` (`<name> :: EXCLUDED: <reason>`), and `harnessgen cursor` treats a skill on disk in
  neither list as a hard error (exit 2). All 12 skills at the freshness head are `packaged`, with
  no exclusions. The harness mode must read that roster rather than globbing `skills/*` — globbing
  would silently ship a skill the roster excludes.
- **The bindings text already exists; do not write a fourth copy.**
  `plugins/assay/codex/AGENTS-assay.md` is the shared `AGENTS.md` fragment, and
  `plugins/assay/references/cursor.md:66-68` records that Cursor reads `AGENTS.md` natively and
  consumes that SAME shared fragment. Writing bindings means emitting/merging that fragment into
  the adopter's `AGENTS.md`, delimited so a re-run replaces the block instead of appending a
  second one.
- **`deskinstall`'s existing contract to preserve:** `main.go:47-66` — `--manifest` and `--dest`
  are both required TODAY and the command refuses without them; exit 0 = installed & verified,
  exit 5 = refused. A harness mode that needs neither must be a distinguishable mode, not a
  loosening of that requirement for the install mode.
- **`--forge gitlab` is a binding value, not a placement value.** On GitLab the adopter's bindings
  must say `glab` / `--forge gitlab` rather than `gh` — `cursor.md:22-24` calls `gh`-shaped skill
  examples "GitHub-shaped; they are not a Cursor requirement and they are wrong on GitLab".
  `statusgen init` already takes `--forge github|gitlab`, so the vocabulary is fixed; reuse it.
- **Idempotence is the property under test, not a nice-to-have.** The three-command install is run
  once by a new adopter and re-run by every upgrade; `--check` is what makes the second run
  informative. A re-run must converge (no duplicate `AGENTS.md` block, no orphaned skill left
  behind when the roster drops one) and `--check` must report drift WITHOUT writing.

single-point-of-failure: the packaging coverage roster
(`plugins/assay/cursor/packaging.md`'s `assay:cursor-packaging` block) is the ONE source deciding
which skills reach an adopter repo, so a mode that ignored it could ship an excluded skill with
nothing to catch it. Two independent layers behind it: (1) `harnessgen cursor`'s existing coverage
rule (`tools/harnessgen/cursor.go:103-111`) fails the BUNDLE build, in CI, on any
`skills/*/SKILL.md` that is in neither list — so a skill can never enter the roster's blind spot
in the first place; (2) this mode's own `--check`, run against a placed tree, reports a placed
skill the roster does not list — a different component, on a different signal (what is on the
adopter's disk vs what is in the bundle), catching the case where the roster changed AFTER a
placement. Row 9 proves layer 2 with layer 1 bypassed.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- **Do not re-implement `harnessgen cursor`.** The `.mdc` rule is generated, coverage-checked and
  drift-checked in one place already; this mode places its output. A second generator is the
  drift this stream exists to avoid.
- **Do not glob `skills/*`.** The roster decides the packaged set; a glob is a silent bypass of a
  written exclusion.
- **Never write outside `--repo`.** Every path this mode creates is under the resolved repo root.
  A placement that escapes it (a `..` in a roster name, a symlink in the source tree) is a
  REFUSAL, not a normalisation.
- **`--check` writes nothing.** If the implementation cannot guarantee that, the mode is not done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Add the mode.** `deskinstall --harness cursor --forge <github|gitlab> --repo <path>`. It is
   mutually exclusive with the acquire→verify→place mode: supplying `--harness` alongside
   `--manifest`/`--dest` is a refusal naming both, not a silent precedence.
2. **Place the skills tree from the roster.** Parse the `assay:cursor-packaging` block in
   `plugins/assay/cursor/packaging.md`; for each `packaged` name, copy
   `plugins/assay/skills/<name>/` to `<repo>/.cursor/skills/<name>/`. A name in the roster with no
   directory on disk, or a directory with no roster entry, is a REFUSAL naming which — the same
   bidirectional coverage rule `harnessgen cursor` enforces for the bundle.
3. **Place `references/` as a SIBLING of the skills root** — `<repo>/.cursor/references/*.md` from
   `plugins/assay/references/*.md` — so `<skills-root>/<skill>/SKILL.md`'s `../../references/x.md`
   resolves. Do not flatten, and do not nest it under `skills/`.
4. **Place the generated Cursor rule if it exists.** Copy `plugins/assay/cursor/assay.mdc` to
   `<repo>/.cursor/rules/assay.mdc`. Absent bundle artifact = a NOTICE and a continue, never a
   failure — `cursor.md:25` and `docs/adopting-assay.md:1097-1098` both say the copy path stands
   without it.
5. **Write the bindings into `<repo>/AGENTS.md`.** Emit `plugins/assay/codex/AGENTS-assay.md`
   between stable delimiters (an `assay:bindings` begin/end pair, the same shape the repo already
   uses for `statusgen:briefs:begin/end`), forge-substituted per `--forge`: `gh` vocabulary on
   `github`, `glab` / `--forge gitlab` on `gitlab`. An existing block is REPLACED in place; an
   `AGENTS.md` with content outside the block keeps that content untouched.
6. **Make it idempotent, and prove convergence not just success.** A second identical run changes
   no byte: same file set, same `AGENTS.md`, one block.
7. **Add `--check`.** Re-derive what a run WOULD place and diff it against what is on disk; write
   nothing. Exit contract, matching `harnessgen cursor`'s so the two read alike: `0` clean, `1`
   drift (name every drifted path and the kind of drift — missing / extra / content-differs),
   `2` could-not-check (unreadable roster, unresolvable repo root).
8. **Refuse rather than half-place.** A failure partway through leaves no partial tree the adopter
   would mistake for an install: stage into a temp location under the repo and move into place, or
   validate the whole plan before the first write. Say which you chose in the PR body.
9. **Add the changelog fragment** (`changelog/<branch-slug>.md`).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./cmd/deskinstall/ && go vet ./cmd/deskinstall/` | exit 0 | `check` |
| 2 | The mode exists and is documented in the usage: `cd tools/desk && go run ./cmd/deskinstall --help 2>&1 \| grep -c -e '--harness' -e '--repo'` | `>= 2` | `check` |
| 3 | **Placement into a scratch repo** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorPlaces' -count=1 -timeout 120s` | exit 0; the test asserts `.cursor/skills/<name>/SKILL.md` exists for EVERY `packaged` name in the roster and for no other name | `check` |
| 4 | **The dead-includes failure is caught** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorReferencesResolve' -count=1 -timeout 120s` | exit 0; the test resolves each `../../references/*.md` include found in a placed `SKILL.md` against the placed tree and asserts the target file EXISTS | `check +flow` |
| 5 | **Idempotence** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorIdempotent' -count=1 -timeout 120s` | exit 0; the test runs the mode twice into one scratch repo and asserts byte-identical trees and exactly ONE bindings block in `AGENTS.md` | `check` |
| 6 | **`--check` reports drift and writes nothing** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorCheckDrift' -count=1 -timeout 120s` | exit 0; the test places, mutates one placed file, asserts `--check` exits 1 naming that path, and asserts the mutated file is UNCHANGED after `--check` ran | `check +mutation` |
| 7 | **`--check` is clean right after a place** — same test binary, `-run 'TestHarnessCursorCheckClean'` | exit 0; `--check` exits 0 on an unmutated placement | `check` |
| 8 | **Fail-first for rows 4 and 6** — run the row-4 test against a build that places only `skills/` (drop the references copy), and the row-6 test against a build whose `--check` always returns 0; capture both RED | both tests observed FAILING, pasted under `## Fail-first` in the PR body with the mutation they ran against | `check +mutation` |
| 9 | **The roster is authoritative, not the glob** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorRosterExcludes' -count=1 -timeout 120s` | exit 0; with a fixture roster marking one on-disk skill `EXCLUDED: <reason>`, that skill is NOT placed and the run still succeeds; with a fixture roster naming a skill absent from disk, the run REFUSES naming it | `check +neighbour` |
| 10 | **Escape refusal** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorRefusesEscape' -count=1 -timeout 120s` | exit 0; a roster entry resolving outside `--repo` is REFUSED and nothing is written | `check +mutation` |
| 11 | **Forge vocabulary lands in the bindings** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorForgeBindings' -count=1 -timeout 120s` | exit 0; `--forge gitlab` produces an `AGENTS.md` block naming `glab`/`--forge gitlab` and NOT bare `gh` as the desk transport; `--forge github` is the converse | `check` |
| 12 | **Mode exclusivity** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestHarnessCursorRefusesWithManifest' -count=1 -timeout 120s` | exit 0; `--harness cursor` supplied together with `--manifest`/`--dest` REFUSES naming both modes | `check` |
| 13 | **The existing install mode is untouched** — `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestWindowsInstall' -count=1 -timeout 120s` | exit 0 — the pre-existing acquire→verify→place tests, including `RefusesOnHashMismatch`, still pass | `check +neighbour` |
| 14 | **Dereference the packaged set against the roster** (catches a well-formed placement of the wrong set): `sed -n '/assay:cursor-packaging/,/-->/p' plugins/assay/cursor/packaging.md \| grep -vE '^\s*(#\|<!--\|-->\|assay:cursor-packaging)' \| grep -vE '^\s*$' \| grep -v 'EXCLUDED' \| sort` and compare to the placed `.cursor/skills/` directory listing from row 3 | the two sets are identical — no extra, no missing | `gate:model +dereference` |
| 15 | Consumers routing corroborated by the diff (run on the implementer's branch): `statusgen --root . --consumers windows-port/07; echo $?` | `0` | `check` |
| 16 | Board lint stays clean: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Every row here is
     OS-neutral Go — a POSIX verifier can discharge all of them; there is no
     Windows-runtime row to defer. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no). The reviewer's two questions:
(1) does the placed tree actually WORK, or does it merely exist? Row 4 is the discriminating row —
it resolves the includes from their new location rather than counting files, which is the exact
failure both source documents warn about. (2) Is the packaged set decided by the roster or by a
glob? Rows 9 and 14 answer that from evidence; a green row 3 alone cannot tell the two apart.
