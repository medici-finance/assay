---
brief: assay:assay:harness-portability:15
title: Public CI wiring + harnesslint clean-up for the de-housed tools
why: >-
  Brief 14 landed `tools/harnessgen`, `tools/harnesslint` and `tools/plugindrift` in the public
  tree, but CI's Go-module walk only `go build`s and `go vet`s them — it does not run their test
  suites. Those suites are the stream's actual drift oracles: `harnessgen`'s tests assert the
  committed Codex/Cursor manifests and resident payload still match this tree's skills roster
  (`--check --root ../..`) and that the `.codex-plugin` version equals the `.claude-plugin`
  version, so with them un-wired a skill added without regenerating packaging, or a version
  skew, lands GREEN. Two loose ends were also flagged on #631 and ruled out-of-scope for hp/14:
  `harnesslint bodies` reports four banned-harness-token violations in shipped skill bodies
  (`ask-decision`, `install`), and `harnesslint bindings` reports nineteen violations against
  `plugins/assay/references/desk-shell.md` — a file that is by its own first paragraph "not a per-harness
  capability binding" and should never have been in the matrix. This brief closes all three.
wave: 7
depends: ["harness-portability/14"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-08 by harness-portability follow-up authoring dispatch (assay-worker-app)
sources: ["the-desk ruling recorded on #631 (hp/14, merged): the three items below were flagged during hp/14 as out-of-scope-for-14 and ruled into one follow-up brief — (a) wire the three de-housed modules' test suites into public ci.yml, (b) scrub the banned harness tokens from ask-decision/install SKILL.md, (c) declare references/desk-shell.md a non-matrix reference the bindings lint skips", "brief-14-code-dehouse.md read in full 2026-09-08 as the format template, and its facts: 'CI needs no edit for the new modules' — TRUE for build+vet, which is all ci.yml's module walk does; its no-new-workflow/glob rule (Task step 5) was hp/14-scoped and does not bind this brief", ".github/workflows/ci.yml read 2026-09-08: the build-test job walks `git ls-files '*go.mod'` and runs `go build ./... && go vet ./...` per module, `go test ./...` ONLY for `tools/desk` (special-cased because two of its guard tests went latent under build+vet alone, #547/#550); no job runs harnessgen/harnesslint/plugindrift tests, and no job runs harnesslint against the real plugins/assay tree", "measured 2026-09-08 at branch head off origin/main: `go test ./...` passes in all three modules (harnessgen 28 tests, plugindrift ~39, harnesslint 15) — harnessgen checks the real tree via `--root ../..`, plugindrift stubs `gh` and is hermetic, harnesslint is fixture-based; `harnesslint bodies plugins/assay/skills` = exit 1, 4 violations (ask-decision SKILL.md lines 48/52/149 `CLAUDE_PLUGIN_ROOT`, install SKILL.md line 246 `SessionStart`); `harnesslint bindings plugins/assay/references` = exit 1, 19 violations ALL on desk-shell.md (7 unresolved capabilities + 12 missing degradation cells)", "plugins/assay/references/desk-shell.md read 2026-09-08: its own opening states it is 'the first that is not a per-harness capability binding ... this file is harness-neutral', which is the standing justification for declaring it non-matrix", "tools/harnesslint/lint.go read 2026-09-08: checkBindings globs refsDir/*.md and demands every capability resolve and every skill have a degradation cell in EVERY reference file; the tool already uses in-file HTML-comment markers (`<!-- assay:capability-vocabulary`, `<!-- assay:banned-tokens`) as its declaration convention", "changelog/README.md read 2026-09-08: this repo enforces a per-PR changelog fragment; brief-adds carry one (harness-portability-13.md, harness-portability-14-code-dehouse.md)"]
consumers: ["docs/streams/harness-portability/README.md: fixed-here (status row 15, wave 7, notes, dependency-wave block)", ".github/workflows/ci.yml: fixed-here (the module-test wiring and the neutrality-gate step are this brief's primary deliverable)", "tools/harnesslint (lint.go + lint_test.go): fixed-here (the non-matrix-reference declaration is a tool change with its own fail-first test)", "plugins/assay/skills/ask-decision/SKILL.md, plugins/assay/skills/install/SKILL.md: fixed-here (the four token scrubs)", "plugins/assay/references/desk-shell.md: fixed-here (the non-matrix declaration marker, if the in-file-marker mechanism is chosen)"]
exec-tier: strong
exec-tier-why: >-
  (b) cross-component correctness — the deliverable spans a CI workflow, three Go modules, two
  skill bodies and one reference. The two hazards are both correctness-of-a-guard: a CI leg that
  is added but does not actually redden on the drift it claims to catch (so it reads green while
  guarding nothing), and a bindings-skip that is written so broadly it silences real violations in
  other references rather than only the one declared non-matrix file. Both are caught only by the
  positive-control rows below, which is why every absence-assertion here is paired with a planted
  failure.
version: 1
id: fcd37132-10d0-46b6-a094-adf6b493b669
---

# Brief 15 — Public CI wiring + harnesslint clean-up for the de-housed tools

## Context

This is the follow-up to **harness-portability/14** (PR **#631**, merged): the code de-house that
landed `tools/harnessgen`, `tools/harnesslint`, `tools/plugindrift` and the packaging/provenance
files in the public tree. Three items were flagged on #631 as out-of-scope for hp/14 and ruled
into this one brief:

1. **CI runs the de-housed modules' suites nowhere.** `.github/workflows/ci.yml`'s `build-test`
   job discovers every module by walking for `go.mod` and runs `go build ./... && go vet ./...`
   from each module's root. It runs `go test ./...` for **`tools/desk` only** (a deliberate
   special-case, #547/#550). So the three new modules build and vet in CI but their tests never
   run — and `harnessgen`'s tests are the stream's drift oracle: `TestCodexCommittedManifestMatchesSource`
   and `TestCommittedArtifactsMatchSource` run `--check --root ../..` against **this** tree, and
   `TestCodexManifestVersionEqualsClaude` compares the `.codex-plugin` and `.claude-plugin`
   versions. With the suite un-wired, a skill added without regenerating the Codex/Cursor/resident
   packaging, or a `.codex-plugin` version left behind, lands **green**.

2. **`harnesslint bodies` is red on shipped skill bodies.** Measured 2026-09-08:
   `plugins/assay/skills/ask-decision/SKILL.md` names the Claude-specific env var `CLAUDE_PLUGIN_ROOT` three times
   (lines 48, 52, 149) and `plugins/assay/skills/install/SKILL.md` names the Claude-specific hook event `SessionStart`
   once (line 246) — four banned-harness-token violations. These are exactly the neutrality
   regressions the hp/04 lint exists to catch; they were shipped because nothing runs the lint.

3. **`harnesslint bindings` is red on `plugins/assay/references/desk-shell.md`.** The bindings check demands that
   every capability resolve and every skill have a degradation cell in **every** `references/*.md`.
   `desk-shell.md` is a harness-**neutral** shell/transport-mechanics reference — its own opening
   paragraph says it is "the first that is not a per-harness capability binding" — so it produces
   nineteen violations (7 unresolved capabilities + 12 missing degradation cells). The fix is to
   declare it a non-matrix reference the bindings check **skips by declaration**, not to bury
   nineteen per-line suppressions.

**This brief authors the SPEC. It does not implement (a)/(b)/(c)** — that is a later dispatch
against this brief.

files:
- **amend** `.github/workflows/ci.yml` — run `go test ./...` for `tools/harnessgen`,
  `tools/harnesslint`, `tools/plugindrift` (extend the existing `tools/desk` special-case), and add
  a step that runs the real-tree neutrality gate (`harnesslint bodies` + `harnesslint bindings`).
- **amend** `plugins/assay/skills/ask-decision/SKILL.md` — neutralise the three `CLAUDE_PLUGIN_ROOT`
  references (Task step b).
- **amend** `plugins/assay/skills/install/SKILL.md` — neutralise the one `SessionStart` reference
  (Task step b).
- **amend** `tools/harnesslint/lint.go` (+ `main.go` if a flag is chosen) — teach `checkBindings`
  to skip a **declared** non-matrix reference, announcing the skip (never silent).
- **amend** `tools/harnesslint/lint_test.go` — the fail-first test(s) for the skip: a declared file
  is skipped; an **un**declared file is still fully checked (Task step c, and §Fail-first).
- **amend** `plugins/assay/references/desk-shell.md` — add the non-matrix declaration marker (if the
  in-file-marker mechanism is chosen; see Task step c).
- **add** `docs/streams/harness-portability/brief-15-ci-wiring-harnesslint.md` — this brief.
- **amend** `docs/streams/harness-portability/README.md` — status row 15, wave 7, the notes and
  dependency-wave block.
- **add** the required per-PR changelog fragment under the `changelog/` directory, named for the
  branch. **Delivered, and no longer present as a file**: the v1.0.0 release roll aggregated every
  fragment into `CHANGELOG.md` and cleared the directory, which is the designed end state for a
  fragment — not a deletion and not a pending deliverable. The delivered content is the
  harnesslint/CI-wiring block in `CHANGELOG.md` § **v1.0.0 — 2026-09-09**. The path is deliberately
  not backticked above: a backticked path to a rolled-away fragment is a lint PROBLEM that reddens
  the whole board (the instance this line used to be), and `(planned)` would be a lie — the file is
  gone by design, not owed.

facts:
- `ci.yml's module walk is build+vet, test only for tools/desk`: read 2026-09-08. The per-module
  loop sets `extra=true` and only `case "$dir" in */tools/desk|tools/desk) extra="go test ./..." ;;`
  turns on the suite. Extending that case is the minimal wiring for the three new modules.
- `the three suites pass in THIS repo`: measured 2026-09-08 at the branch head. `harnessgen`'s
  `--root ../..` checks resolve because ci.yml runs in the **source** repo (`medici-finance/assay`),
  where the real tree is present; `plugindrift` stubs `gh` in its tests and is hermetic;
  `harnesslint`'s suite is fixture-based (`testdata/`). None reaches the network.
- `harnessgen's suite is the roster/version oracle, on the real tree`: `--check --root ../..`
  compares the committed `.codex-plugin`/`cursor`/`resident` artifacts against this tree's
  `plugins/assay/skills/**`, so an unaccounted skill or a stale manifest reddens the suite. This is
  why wiring `go test` — not merely adding a lint step — is what closes item (1).
- `harnesslint's own suite is fixture-based`: it exercises `checkBodies`/`checkBindings` against
  `testdata/`, so `go test ./tools/harnesslint` catches tool-logic regressions but does NOT check
  the real `plugins/assay` tree. The real-tree neutrality property (items 2/3) is enforced only by
  invoking `harnesslint bodies`/`bindings` against the live tree — which is why item (a)'s CI step
  includes that invocation and is what makes the item-(b)/(c) scrubs durable rather than cosmetic.
- `the four banned-token sites are prose/code-fence references, not structural`: the
  `CLAUDE_PLUGIN_ROOT` hits are `${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh` invocations in fenced
  examples; the `SessionStart` hit names the Claude hook event in an honesty caveat. No schema key or
  test fixture depends on the literal, so neutral phrasing suffices (the hp/04 pattern: the neutral
  body names the mechanism, the Claude reference file keeps the harness-specific token).
- `desk-shell.md self-declares non-matrix`: its opening paragraph names `claude-code.md`,
  `codex.md` and `cursor.md` as the three per-harness bindings and states it is "the first that is
  not a per-harness capability binding ... this file is harness-neutral." The declaration marker
  makes that prose machine-checkable.
- `the tool already has a marker convention`: `lint.go` reads `<!-- assay:capability-vocabulary`
  and `<!-- assay:banned-tokens` blocks via `blockLines`. An in-file `<!-- assay:harnesslint …`
  marker for the non-matrix declaration is symmetric with what the tool already parses.
- `this repo enforces a changelog fragment`: `changelog/README.md` is present; brief-adds carry a
  fragment (`harness-portability-13.md`, `harness-portability-14-code-dehouse.md`).
- `nothing here weakens a security control`: the bindings-skip is a declaration mechanism for a
  neutrality lint, not a leak-sweep/RBAC/auth control; item (c) is a `needs-decision` STOP only if
  an implementer finds it would blanket-silence other references, which Verify row 5a exists to
  forbid.

## Read first

- `.github/workflows/ci.yml` — the `build-test` module walk and its `tools/desk` special-case, the
  `skillslint` job's `cd tools/skillslint && go run . --root ../..` invocation shape (the pinned
  "run the in-tree Go tool from its own source" pattern this brief reuses).
- `tools/harnesslint/lint.go` and `main.go` — `checkBindings`, `blockLines`, and the marker
  convention; `banned-tokens.md` for what `bodies` flags.
- `plugins/assay/references/desk-shell.md` (its opening paragraph) and the three real binding
  matrices `claude-code.md` / `codex.md` / `cursor.md`.
- `plugins/assay/references/claude-code.md` — where the neutralised `CLAUDE_PLUGIN_ROOT` /
  `SessionStart` tokens legitimately live, so the skill-body scrub relocates rather than deletes the
  mechanism.
- brief-14-code-dehouse.md — the format template and the Defense-in-depth framing.

## Ground rules

- **`ci.yml` MAY be edited by this brief.** hp/14's Task step 5 ("Do not add a workflow, a Makefile
  target, or a `.assay-surfaces` glob … an unmeasured CI addition is a separate change with its own
  review") was **scoped to hp/14**, where the point was that the module walk already covered
  build+vet. This brief IS that separate, measured change: it adds test coverage and the neutrality
  gate to `ci.yml`, with positive-control rows proving each new leg reddens on the drift it guards.
  Do not self-block on the hp/14 rule.
- **Stop at `implemented`** — you do not set verified/done, and you do not flip the PR ready.
- **A neutralisation must relocate, not delete.** The `CLAUDE_PLUGIN_ROOT`/`SessionStart` mechanisms
  must still be reachable to a Claude Code reader — the neutral body names the mechanism and the
  Claude reference file (`plugins/assay/references/claude-code.md`) carries the harness-specific token. Deleting
  the guidance passes the lint and destroys what the body is for; that is the failure mode item (b)
  forbids.
- **The bindings-skip is by explicit declaration only.** A reference with no declaration still gets
  the full closure + degradation-cell check. If the only way you can make `bindings` green is a
  change that also stops other references being checked, STOP and report `NEEDS_CONTEXT` — that is a
  broadening of a guard, not this brief.
- **CI wiring is proven by a red, not by a green.** For each new CI leg, show it going red on the
  drift it claims to catch (Verify rows 2a, 4a, 5a) before claiming it passes clean.
- If anything is unclear or contradicts repo state: report `NEEDS_CONTEXT`, do not guess.

## Task

The three parts may land as one PR; author them so each has its own commit and its own Verify rows.

### (a) Wire the three modules into public CI

1. **Run the suites.** In `ci.yml`'s `build-test` job, extend the per-module `case` so
   `tools/harnessgen`, `tools/harnesslint` and `tools/plugindrift` run `go test ./...` (as
   `tools/desk` already does), keeping `go build ./... && go vet ./...` for every other module. Do
   not remove the build+vet default or the `tools/desk` case; only add the three dirs. The comment
   block that explains why the default is build+vet stays true for the other modules — annotate the
   new case with why these three are safe to run in the source repo (harnessgen's `--root ../..`
   checks resolve here; plugindrift stubs `gh`; harnesslint is fixture-based).
2. **Run the real-tree neutrality gate.** Add a step (or a small job, mirroring `skillslint`) that
   installs Go and runs, from the harnesslint module source:
   `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills`
   and `… bindings ../../plugins/assay/references`, failing the job on a non-zero exit. This is the
   leg that keeps item (b)/(c) from silently regressing: the harnesslint *unit* suite is
   fixture-based and never reads the real tree. Do not add a `.assay-surfaces` glob — this is an
   explicit, named invocation, not a discovery surface.

### (b) Scrub the banned harness tokens from the two skill bodies

3. **`plugins/assay/skills/ask-decision/SKILL.md`** — neutralise the three `${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh`
   references (lines 48, 52, 149 at authoring time; re-locate by content, not line number). Name the
   inbox script by a harness-neutral mechanism and defer the `CLAUDE_PLUGIN_ROOT` expansion to
   `plugins/assay/references/claude-code.md` per the hp/04 neutral-core convention. The reader must still be able
   to run the inbox — relocate the harness-specific path, do not drop the command.
4. **`plugins/assay/skills/install/SKILL.md`** — neutralise the one `SessionStart` reference (line 246 at authoring time).
   Name the resident-rules injection channel by its neutral mechanism (per hp/05) rather than the
   Claude hook event; the honesty caveat about the `bash`+`jq` workaround must still say what it says.
   Move `SessionStart` to `plugins/assay/references/claude-code.md` if it is not already there.

### (c) Declare `desk-shell.md` a non-matrix reference

5. **Teach `harnesslint bindings` a declared skip.** Give `checkBindings` a way to recognise a
   reference file **explicitly declared** non-matrix and exclude it from both the capability-closure
   and degradation-cell checks. The recommended mechanism, symmetric with the tool's existing marker
   convention (`<!-- assay:capability-vocabulary`, `<!-- assay:banned-tokens`), is an in-file HTML
   comment carrying a reason, e.g.
   `<!-- assay:harnesslint non-matrix-reference — harness-neutral shell/transport mechanics, not a per-harness capability binding -->`,
   which `checkBindings` reads from each file's body (it already reads the body). The tool must
   **print which files it skipped** — a skip is announced on stdout/stderr, never silent (three-state
   discipline: a file the check chose not to look at is a could-not-check for that file, reported as
   itself). Moving the file out of `references/` is an accepted alternative the ruling permits, but
   the in-file declaration keeps desk-shell.md discoverable next to the bindings it complements and
   is preferred.
6. **Add the declaration** to `desk-shell.md` (if the marker mechanism is chosen). Its opening prose
   already carries the justification; the marker makes it machine-readable.

### Fail-first (guard change)

The `checkBindings` change is a guard change, so `lint_test.go` gains, and the PR body shows failing
first (§9): (i) a test asserting a declared non-matrix reference is skipped — red before the skip
logic exists; (ii) a test asserting an **un**declared reference in the same directory is still fully
checked and still reddens on a missing capability/cell — this is the narrowness guarantee, and the
mutation that reddens it is "remove the declaration marker / add an undeclared junk reference." The
token scrubs (b) and the CI-wiring (a) are proven by Verify rows 2a/4a/5a rather than a Go mutation.

## Verify (executable — no prose-only DoD items)

Run from the repository root of a checkout of this branch; the Go toolchain is on `PATH`. The three
tool modules are separate Go modules (no root `go.work`), so tool invocations `cd` into the module
and use `../..`-relative paths — the same shape `ci.yml`'s `skillslint` job uses.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -E 'tools/harnessgen\|tools/harnesslint\|tools/plugindrift' .github/workflows/ci.yml \| grep -c 'go test' \|\| true; sed -n '/build-test:/,/plugin-shell-suites:/p' .github/workflows/ci.yml \| grep -cE 'harnessgen.*\|harnesslint.*\|plugindrift'` | the three module dirs appear in the `build-test` job's `go test` case — the wiring exists in the file, not only in a local run |
| 2 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go test ./... -timeout 120s ) \|\| rc=1; done; echo "suites=$rc"` | `suites=0` — all three suites pass here, which is exactly what the new CI legs run |
| 2a | **Positive control — roster drift reddens harnessgen (the CI leg catches it):** `mkdir -p plugins/assay/skills/zzz-canary-skill && printf '%s\n' '---' 'name: zzz-canary-skill' 'description: canary' '---' > plugins/assay/skills/zzz-canary-skill/SKILL.md; ( cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... -run 'Codex\|Committed' -timeout 120s ) > /tmp/hp15r2a.out 2>&1; echo "drift-exit=$?"; rm -rf plugins/assay/skills/zzz-canary-skill` | `drift-exit` is **non-zero** — an unaccounted skill makes harnessgen's real-tree `--check` suite fail, so the wired `go test` leg would redden CI. Without this row, row 2's `0` proves only that the suite ran, not that it guards anything |
| 3 | `grep -A30 -E 'harnesslint' .github/workflows/ci.yml \| grep -Ec 'bodies\|bindings'` | `>= 2` — `ci.yml` invokes `harnesslint bodies` and `harnesslint bindings` against the real tree (the neutrality gate step of item (a)); this is the leg that keeps items (b)/(c) from regressing |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills; echo "bodies=$?"` | `checked-clean` then `bodies=0` — the four banned-token violations in `ask-decision`/`install` are gone |
| 4a | **Positive control — a banned token still reddens `bodies`:** `t=$(git rev-parse --show-toplevel); d=$(mktemp -d); cp -R "$t/plugins/assay/skills" "$d/skills"; printf '\n`${CLAUDE_PLUGIN_ROOT}/x`\n' >> "$d/skills/ask-decision/SKILL.md"; cd "$t/tools/harnesslint" && go run . --vocab "$t/docs/streams/harness-portability/README.md" bodies "$d/skills" > /dev/null 2>&1; echo "planted=$?"; rm -rf "$d"` | `planted=1` — with a banned token replanted in a scratch copy the same invocation goes red, proving row 4's `0` is a real read of a clean tree, not a matcher that matched nothing |
| 5 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references; echo "bindings=$?"` | `checked-clean` then `bindings=0` — with `desk-shell.md` present **and** declared non-matrix, the nineteen violations are gone; the three real binding matrices are still fully checked |
| 5a | **Narrowness control — the skip is by declaration only:** `t=$(git rev-parse --show-toplevel); d=$(mktemp -d); cp -R "$t/plugins/assay/references" "$d/refs"; printf '# undeclared junk reference\n\nno capability bindings here\n' > "$d/refs/zzz-undeclared.md"; cd "$t/tools/harnesslint" && go run . --vocab "$t/docs/streams/harness-portability/README.md" bindings "$d/refs" > /dev/null 2>&1; echo "undeclared=$?"; rm -rf "$d"` | `undeclared=1` — an **un**declared reference with no bindings still reddens, proving the skip excludes only the declared file and did not blanket-disable the check |
| 6 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... -run 'NonMatrix\|Skip\|Bindings' -timeout 60s; echo "hl-suite=$?"` | `hl-suite=0` — the new fail-first test(s) for the declared-skip and its narrowness pass (their red-on-unfixed evidence is in the PR body under `## Fail-first`) |
| 7 | `statusgen --lint --root .; echo $?` | `0` — PASS, no PROBLEM: the board row 15 is a valid lifecycle status and the frontmatter is schema-clean. Build `statusgen` from this repo's `statusgen/` rather than trusting a `PATH` binary older than the pinned tag |
| 8 | `awk '/^## v1\.0\.0 /{f=1;next} /^## /{f=0} f' CHANGELOG.md \| grep -qE '^- .*harnesslint'; echo "changelog=$?"` | `changelog=0` — **amended 2026-09-09**: the row used to assert the fragment file existed and carried a bullet. The v1.0.0 release roll aggregated every fragment into `CHANGELOG.md` and cleared the directory, so the file-existence form now asserts something that is false by design and is unrunnable forever after. The row asserts the delivered content instead: the aggregated highlight bullets are present in the `v1.0.0` section. `changelog/README.md` still governs how a *future* fragment is added |
| 9 | `git grep -n '<<<<<<<' -- . \| wc -l; git diff --stat origin/main...HEAD -- ':(exclude)docs/streams/harness-portability' ':(exclude)changelog'` | `0` conflict markers; the diffstat outside this stream dir + changelog touches only `.github/workflows/ci.yml`, the two `SKILL.md` bodies, `tools/harnesslint/*`, and `plugins/assay/references/desk-shell.md` — no incidental edit rode along |

## Evidence

<!-- appended at implementation/verification time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this section filled by someone who did NOT implement.
     Rows 4a/5a/2a: record the exit code and, for 4a, that the report body was
     NOT pasted (harnesslint bodies prints file:line, never a token value, so it
     is safe — but keep to exit code + count regardless). -->

## Review

Gate: **model** (from frontmatter; no `irreversible`/`sensitive-data`). The reviewer records the
verdict + date in the stream README table. This is not a publication and carries no leak surface of
its own — the moved content already lives in the public tree (hp/14). The reviewer's focus is the
two guard-correctness hazards this brief's `exec-tier-why` names:

1. **Does each new CI leg actually redden on the drift it claims to catch?** Verify rows 2a
   (harnessgen suite on a planted unaccounted skill) and 4a/5a (harnesslint on planted violations)
   are the intended proofs. A table that only shows the green rows has verified that the legs run,
   not that they guard.
2. **Is the bindings-skip narrow?** Row 5a plus the fail-first test (ii) must show an undeclared
   reference still fully checked. A skip written as "ignore `desk-shell.md`" by name, or one that
   silences any reference lacking bindings, is a broadening of the guard and a finding — the skip
   must key on an explicit declaration and announce every file it skips.
