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

### Non-implementer verifier run — 2026-09-12 sonnet-5-verifier (verify-desk dispatch), FIRST verify pass — **VERIFY: PARTIAL**

Runner ≠ implementer. Own temp worktree off origin/main, `KUBECONFIG=/dev/null`. This brief's Evidence table was completely empty before this pass.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | grep against ci.yml for the three module dirs in the go test case | dirs appear | could-not-check — the CI wiring is not applied to ci.yml; it ships as an unapplied tools/harnesslint/ci.yml.patch (the identity that authors these PRs cannot write .github/workflows). Confirmed the patch applies cleanly and its content does contain the three-module go-test case plus the harnesslint job. statusgen --lint independently flags this row's own grep pattern as using a literal-pipe bug (extended regex reads an escaped pipe as literal, not alternation), so even against an applied ci.yml this row's command matches almost nothing | 2026-09-12 sonnet-5-verifier |
| 2 | run the three modules' go test ./... | zero failing suites | **FAIL — 1 failing suite.** harnessgen and plugindrift real-tree checks fail. Root-caused: NOT this brief's own diff — a later merged PR (desk-skills/04, closes an unrelated stream) added a new skill without regenerating Codex/Cursor packaging or the source-of-truth manifest. The harnesslint module alone passes clean. This is exactly the drift class the new oracle exists to catch — it is currently catching real drift, just from an unrelated stream, not from this brief | 2026-09-12 sonnet-5-verifier |
| 2a | positive control: plant a canary skill, re-run the drift test filtered to the two relevant test names | non-zero exit | **Row command has an authoring bug, not a guard defect.** As literally written the test-name filter uses a backslash-escaped alternation, which Go's -run regexp reads as a literal character, matching zero tests (silent no-op, misleadingly "passes"). Re-run with the correct unescaped alternation confirms the guard itself works: the manifest-drift test genuinely fails when an unaccounted skill is planted in the real tree | 2026-09-12 sonnet-5-verifier |
| 3 | grep ci.yml for the bodies/bindings step count | count >= 2 | could-not-check, same reason as row 1 (staged patch, not applied); same literal-pipe authoring bug independently flagged by statusgen for this row too | 2026-09-12 sonnet-5-verifier |
| 4 | harnesslint bodies on the real skills tree | checked-clean, exit 0 | PASS — checked-clean, no violations, exit 0 | 2026-09-12 sonnet-5-verifier |
| 4a | positive control: banned token replanted in a scratch copy | exit 1 | PASS — exit 1 (report body kept to exit code + count per the security-sensitive-content instruction, not pasted) | 2026-09-12 sonnet-5-verifier |
| 5 | harnesslint bindings on the real references tree | checked-clean, exit 0 | **FAIL, exit 1, 3 violations** — same human-runsheet cross-stream drift as row 2: the three real matrix files each missing a degradation cell for the new skill. The declared non-matrix reference is correctly skipped, which is itself confirmation the skip did not broaden (the failures are only on the real matrix files) | 2026-09-12 sonnet-5-verifier |
| 5a | narrowness control: an undeclared junk reference in a scratch copy | exit 1 | PASS — exit 1, 7 violations on the junk file; the declared non-matrix reference is still correctly skipped in the same run, proving the skip is narrow and does not cover its neighbour | 2026-09-12 sonnet-5-verifier |
| 6 | fail-first suite in the harnesslint module (narrowness + skip-declaration tests) | exit 0 | PASS — exit 0, includes the three targeted narrowness/skip-declaration tests | 2026-09-12 sonnet-5-verifier |
| 7 | statusgen --lint --root . | exit 0, no PROBLEM | PASS — built from this repo's own statusgen source; LINT: PASS, exit 0. Several NOTICE-level lines independently corroborate this pass's own row 1/2a/3/9 command-authoring findings | 2026-09-12 sonnet-5-verifier |
| 8 | changelog aggregate check | exit 0 | PASS — exit 0 | 2026-09-12 sonnet-5-verifier |
| 9 | conflict-marker + diffstat check vs origin/main | 0 markers, narrow diffstat | Adapted — unrunnable as literally written once fast-forwarded to merged main (the base-vs-head comparison is empty by construction); statusgen's own lint independently flags this row as a moving-ref command. A repo-wide search for the actual conflict-marker string returns only legitimate doc/test-fixture prose (this brief's own row text, skill docs, and conflict-detection code plus its tests) — zero real unresolved conflicts. Diffed the actual merge commits instead: touches exactly the plugin manifest version-parity fix, two reference-doc relocations, both affected SKILL.md files, and the harnesslint module — matches the brief's declared touch-set | 2026-09-12 sonnet-5-verifier |

Additional task-correctness spot checks: token relocation confirmed real (moved out of two skill bodies into the shared reference doc, not deleted); the non-matrix reference carries the exact declared marker; the skip mechanism announces every skipped file and keys strictly on that declaration.

`RISK-VALUE: DERIVED` — the one literal this diff introduces: a packaging-manifest version field regenerated as a side effect of this brief's own tooling, mechanically derived from the source-of-truth manifest's version field (the same value a dedicated equality test pins) — not a picked value, previously silently stale because nothing exercised the test that catches it, and reversible via a re-run of the generator. Everything else in the diff is structural (skip logic, doc relocation, CI step wiring) with no new numeric threshold, timeout, ratio, or authority binding.

**VERIFY: PARTIAL.** Rows 1/3 could-not-check by design (already documented in the stream README — App can't write .github/workflows, wiring ships as a staged patch), not a defect in this brief. Rows 2/5 are a genuine FAIL on current merged main, but caused by an unrelated later-merged PR — not a defect in this brief's own diff; worth a separate filing against that drift. Row 2a's own command has a shell/regex-escaping bug (matches zero tests, so its "pass" would be hollow) — corrected, the underlying guard genuinely works. Row 9 unrunnable as literally written post-merge; adapted to the merge-commit diff, which is clean and narrow. Everything else (4, 4a, 5, 5a, 6, 7, 8) is a clean, real PASS including both designed positive/narrowness controls and the fail-first suite.

Per frontmatter `gate: model`: this verifier does not sign off and status does not change. Evidence-only, for the dispatching session to route per the gate:model path.

**Additional findings worth relaying (not Verify-row failures):** (1) this brief's own Verify rows 1, 2a, 3, and 9 have command-authoring bugs (an escaped-pipe-as-literal issue in two different regex dialects, and a moving-ref comparison) — statusgen's own lint independently flags all of these as NOTICEs; worth a small brief-hygiene fix. (2) statusgen also flags a risk-files-crossread NOTICE: this brief answers all four risk questions "no" but its files: list names a security-path-triggering CI workflow file — worth a human/desk glance even though the actual edit currently lands only as a sidecar patch rather than a live workflow change. (3) The unrelated drift causing rows 2/5's FAIL (a skill added without regenerating packaging/bindings) is a real, currently-live defect worth its own filing.
### Non-implementer verifier re-run — VERIFY: FAIL (CI-wiring deliverable never landed, filed) — sonnet-5-verifier (verify-desk dispatch), @ merged main `ee79ed43e`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main (HEAD already at origin/main, no fetch needed). Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | grep for harnessgen/harnesslint/plugindrift in ci.yml | 3 module dirs appear in build-test's go test case | **FAIL** — 0 matches, confirmed by an independent direct grep of the whole file for all three tool names too. Genuinely absent, not a regex artifact | 2026-09-18 | sonnet-5-verifier |
| 2 | `go test ./...` in all 3 tool modules | suites=0 | checked-clean, suites=0 — all three modules pass; unrelated drift from the 2026-09-12 pass (skill added without repackaging) no longer present | 2026-09-18 | sonnet-5-verifier |
| 2a | mutation: plant a canary skill, re-run harnessgen filtered check | non-zero exit | checked-failed as expected once the row's own regex-escaping bug is corrected (same pre-existing authoring quirk flagged in the 2026-09-12 pass, reproduced identically); guard genuinely catches the canary | 2026-09-18 | sonnet-5-verifier |
| 3 | grep for harnesslint bodies/bindings invocation in ci.yml | ≥2 | **FAIL** — 0, confirmed by row 1's direct grep too; the neutrality-gate CI leg is not present on main | 2026-09-18 | sonnet-5-verifier |
| 4 | `harnesslint bodies` on real skills | exit 0 | checked-clean, no violations | 2026-09-18 | sonnet-5-verifier |
| 4a | mutation: banned token replanted | exit 1 | checked-failed as expected | 2026-09-18 | sonnet-5-verifier |
| 5 | `harnesslint bindings` on real references | exit 0 | checked-clean, no violations; 3 declared non-matrix files correctly skipped, all real matrices fully checked | 2026-09-18 | sonnet-5-verifier |
| 5a | mutation: undeclared junk reference | exit 1 | checked-failed as expected, 8 violations — proves the skip is keyed on declaration not filename | 2026-09-18 | sonnet-5-verifier |
| 6 | harnesslint's own NonMatrix/Skip/Bindings test suite | suite=0 | checked-clean, all named subtests pass | 2026-09-18 | sonnet-5-verifier |
| 7 | `statusgen --lint --root .` | exit 0, no PROBLEM | checked-clean, LINT: PASS | 2026-09-18 | sonnet-5-verifier |
| 8 | changelog aggregate check | exit 0 | checked-clean | 2026-09-18 | sonnet-5-verifier |
| 9 | conflict-marker sweep + touch-set narrowness | 0 markers, narrow diffstat | checked-clean — 14 repo-wide `<<<<<<<` hits all confirmed legitimate (prose quoting the string, detection code/tests); merge-commit diffstat touches exactly the brief's declared files, .github/workflows/ci.yml itself untouched (only the sidecar .patch file) — confirms the CI wiring was never pushed, not that it drifted. `git apply --check` on the sidecar patch: exit 0, still applies cleanly against current main | 2026-09-18 | sonnet-5-verifier |

Scope traceability: rows map 1:1 to the brief's Deliverables items (a: CI wiring, rows 1/3/9; b: skill-body scrubs, rows 4/4a; c: harnesslint bindings declared-skip, rows 5/5a/6; plus board/changelog hygiene rows 7/8, and drift-oracle proof rows 2/2a).

RISK-VALUE: DERIVED — version="1.0.14" @ plugins/assay/.codex-plugin/plugin.json:3 — mechanically regenerated to equal the Claude manifest's version (confirmed identical this pass); not an authored/picked value, reversible via a harnessgen re-run.
RISK-VALUE: N/A — enumeration over the checkBindings/nonMatrixDeclaration diff found no literal threshold/timeout/ratio/authority-binding; it is pure string-marker parsing.

VERIFY: FAIL — held at implemented. Items (b) skill-body scrubs and (c) harnesslint bindings declared-skip are fully landed and verified-clean (every row incl. positive/narrowness controls). Item (a) — wiring the three tools into public CI — has NOT landed: `.github/workflows/ci.yml` carries zero references to any of them. The fix exists as an unapplied sidecar patch (`tools/harnesslint/ci.yml.patch`, confirmed not stale, applies cleanly) but the authoring identity cannot push `.github/workflows/**` (App tokens lack workflows scope). This is exactly the blocker three sibling briefs (harness-portability/04, 06, 12) independently cited as their own unlanded CI-wiring dependency — hp/15 itself is still blocked on the same constraint. Filed medici-finance/assay#1332 (help wanted) asking for a human push via the human:<name> path.

### Non-implementer verifier run — VERIFY: FAIL — 9/12 pass, 0 could-not-check, 3 fail — 2026-09-23 claude-opus-4-8-verifier

Runner ≠ implementer. Own detached temp worktree cut off `refs/remotes/origin/main` at merged head `438dd26a3e95`; offline envelope observed (`KUBECONFIG=/dev/null`). No PR, no push, no status flip, no landing. This brief's Verify table has no `check:ci`-classed rows. Rows 1/3/5 FAIL: rows 1/3 because item (a) — the CI wiring — was never pushed (App tokens lack workflow scope; tracked #1332), and row 5 on a real-tree bindings drift (#1488). Execution witness (`statusgen verifyrun`) was run and its rows are left uncommitted in the brief file in the worktree for the desk; where the witness's captured exit is masked by a trailing `; echo` in the row command (rows 4/5/7 end in `echo`, so the witness records echo's exit 0, not the tool's), the Observed cell below records the tool's OWN exit read directly.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `grep -E 'tools/harnessgen\|tools/harnesslint\|tools/plugindrift' .github/workflows/ci.yml \| grep -c 'go test' \|\| true; sed -n '/build-test:/,/plugin-shell-suites:/p' .github/workflows/ci.yml \| grep -cE 'harnessgen.*\|harnesslint.*\|plugindrift'; grep -nE 'harnessgen\|harnesslint\|plugindrift' .github/workflows/ci.yml; echo "wholefile-grep-rc=$?"` | the three module dirs appear in the build-test job's go test case | FAIL — ci-wiring-absent; exit 0; 0 ⏎ 0 ⏎ wholefile-grep-rc=1 | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go test ./... -timeout 120s ) \|\| rc=1; done; echo "suites=$rc"` | suites=0 | PASS — suites-green; exit 0; suites=0 ⏎ github.com/medici-finance/assay/tools/harnessgen ⏎ github.com/medici-finance/assay/tools/harnesslint ⏎ github.com/medici-finance/assay/tools/plugindrift | 2026-09-23 | claude-opus-4-8-verifier |
| 2a | `mkdir -p plugins/assay/skills/zzz-canary-skill && printf '%s\n' '---' 'name: zzz-canary-skill' 'description: canary' '---' > plugins/assay/skills/zzz-canary-skill/SKILL.md; ( cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... -v -timeout 120s ); echo "drift-exit=$?"; rm -rf plugins/assay/skills/zzz-canary-skill` | non-zero drift-exit | PASS — canary-reddens; exit 0; drift-exit=1 ⏎ --- FAIL: TestCodexCommitted ⏎ --- FAIL: TestCursorCommitted | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `grep -A30 -E 'harnesslint' .github/workflows/ci.yml \| grep -Ec 'bodies\|bindings'` | >= 2 | FAIL — ci-step-absent; exit 1; 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills; echo "bodies=$?"` | checked-clean then bodies=0 | PASS — bodies-clean; exit 0; checked-clean: bodies — no violations ⏎ bodies=0 | 2026-09-23 | claude-opus-4-8-verifier |
| 4a | `t=$(git rev-parse --show-toplevel); d=$(mktemp -d); cp -R "$t/plugins/assay/skills" "$d/skills"; printf '\n`${CLAUDE_PLUGIN_ROOT}/x`\n' >> "$d/skills/ask-decision/SKILL.md"; cd "$t/tools/harnesslint" && go run . --vocab "$t/docs/streams/harness-portability/README.md" bodies "$d/skills" > /dev/null 2>&1; echo "planted=$?"; rm -rf "$d"` | planted=1 | PASS — canary-caught; exit 0; planted=1 | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references; echo "bindings=$?"` | checked-clean then bindings=0 | FAIL — bindings-drift; exit 0; bindings=1 ⏎ ../../plugins/assay/references/claude-code.md: no degradation cell for skill "system-demo" ⏎ checked-failed: bindings — 1 violation(s) | 2026-09-23 | claude-opus-4-8-verifier |
| 5a | `t=$(git rev-parse --show-toplevel); d=$(mktemp -d); cp -R "$t/plugins/assay/references" "$d/refs"; printf '# undeclared junk reference\n\nno capability bindings here\n' > "$d/refs/zzz-undeclared.md"; cd "$t/tools/harnesslint" && go run . --vocab "$t/docs/streams/harness-portability/README.md" bindings "$d/refs" > /dev/null 2>&1; echo "undeclared=$?"; rm -rf "$d"` | undeclared=1 | PASS — undeclared-caught; exit 0; undeclared=1 | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... -v -timeout 60s` | exit 0, named skip/narrowness subtests --- PASS (not "ok" alone) | PASS — bindings-suite-green; exit 0; --- PASS: TestNonMatrixDeclaration ⏎ github.com/medici-finance/assay/tools/harnesslint | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `statusgen --lint --root .; echo $?` | exit 0, no PROBLEM | PASS — lint-pass; exit 0; LINT: PASS | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `awk '/^## v1\.0\.0 /{f=1;next} /^## /{f=0} f' CHANGELOG.md \| grep -qE '^- .*harnesslint'; echo "changelog=$?"` | changelog=0 | PASS — changelog-present; exit 0; changelog=0 | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | `git grep -n '<<<<<<<' -- . \| wc -l; git diff --stat origin/main...HEAD -- ':(exclude)docs/streams/harness-portability' ':(exclude)changelog'; echo '--- git grep -n read ---'; git grep -n '<<<<<<<' -- .` | 0 real markers, narrow diffstat | PASS — no-real-markers; exit 0; 15 ⏎ brief-14-code-dehouse.md ⏎ brief-15-ci-wiring-harnesslint.md ⏎ pr-shepherd/SKILL.md ⏎ deskmerge/merge.go ⏎ deskpreflight/testdata/markers/conflict.txt ⏎ verifyoutcomes_union_test.go | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: N/A — enumeration over this brief's diff scope (CI-workflow test-case + neutrality-gate wiring, two skill-body prose token scrubs, harnesslint checkBindings string-marker declared-skip + its Go tests, and the desk-shell.md HTML-comment declaration marker) found no literal threshold, tolerance, ratio, timeout, limit, or authority binding introduced or changed. The changes are structural: string-marker parsing, prose relocation, and CI step wiring.
RISK-VALUE: N/A (adjacent, checked) — the one numeric literal prior passes named, `.codex-plugin`/`.claude-plugin` `version`, is `1.0.27` in both @ plugins/assay/.codex-plugin/plugin.json:3 and plugins/assay/.claude-plugin/plugin.json:5 (equal this pass); it is regenerated by harnessgen and is NOT introduced or changed by this brief's own diff — this brief only wires the test that guards its parity. Named for completeness, not owed a derivation by this brief.

CI wiring still unpushed (#1332); rows 1/3 fail on its absence and row 5 is the same system-demo bindings drift noted on #1332.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on the darwin host (no row needs a Linux-only facility), statusgen built from a `--no-hardlinks` clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -E 'tools/harnessgen\|tools/harnesslint\|tools/plugindrift' .github/workflows/ci.yml \| grep -c 'go test' \|\| true; sed -n '/build-test:/,/plugin-shell-suites:/p' .github/workflows/ci.yml \| grep -cE 'harnessgen.*\|harnesslint.*\|plugindrift'` | pass exit=0 | sha256:30fee5333b80 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go test ./... -timeout 120s ) \|\| rc=1; done; echo "suites=$rc"` | pass exit=0 | sha256:9d9f53d8d943 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2a | `mkdir -p plugins/assay/skills/zzz-canary-skill && printf '%s\n' '---' 'name: zzz-canary-skill' 'description: canary' '---' > plugins/assay/skills/zzz-canary-skill/SKILL.md; ( cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... -run 'Codex\|Committed' -timeout 120s ) > /tmp/hp15r2a.out 2>&1; echo "drift-exit=$?"; rm -rf plugins/assay/skills/zzz-canary-skill` | pass exit=0 | sha256:d1efb28396c0 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -A30 -E 'harnesslint' .github/workflows/ci.yml \| grep -Ec 'bodies\|bindings'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills; echo "bodies=$?"` | pass exit=0 | sha256:1076d844154e | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4a | `bodies` | could-not-run exit=- — prose-led-command: not executed; the first span bodies is a lone word mentioned ahead of the command span, not a command. Mark the command with a cmd: code span | sha256: | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references; echo "bindings=$?"` | pass exit=0 | sha256:194e90424d59 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5a | `t=$(git rev-parse --show-toplevel); d=$(mktemp -d); cp -R "$t/plugins/assay/references" "$d/refs"; printf '# undeclared junk reference\n\nno capability bindings here\n' > "$d/refs/zzz-undeclared.md"; cd "$t/tools/harnesslint" && go run . --vocab "$t/docs/streams/harness-portability/README.md" bindings "$d/refs" > /dev/null 2>&1; echo "undeclared=$?"; rm -rf "$d"` | pass exit=0 | sha256:828242b59978 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... -run 'NonMatrix\|Skip\|Bindings' -timeout 60s; echo "hl-suite=$?"` | pass exit=0 | sha256:ec59dc5ac76b | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --lint --root .; echo $?` | pass exit=0 | sha256:e10a3fd8fed4 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 8 | `awk '/^## v1\.0\.0 /{f=1;next} /^## /{f=0} f' CHANGELOG.md \| grep -qE '^- .*harnesslint'; echo "changelog=$?"` | pass exit=0 | sha256:e93edb8e4dc3 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 9 | `git grep -n '<<<<<<<' -- . \| wc -l; git diff --stat origin/main...HEAD -- ':(exclude)docs/streams/harness-portability' ':(exclude)changelog'` | could-not-run exit=- — unsubstituted placeholder(s) ... — the row cannot run as written | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -E 'tools/harnessgen\|tools/harnesslint\|tools/plugindrift' .github/workflows/ci.yml \| grep -c 'go test' \|\| true; sed -n '/build-test:/,/plugin-shell-suites:/p' .github/workflows/ci.yml \| grep -cE 'harnessgen.*\|harnesslint.*\|plugindrift'` | the three module dirs appear in build-test's go test case | exit 0; printed 3 then 8. The 3 go-test lines are the case arms at ci.yml:78-80 (tools/desk arm kept at :77, build+vet default kept) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `rc=0; for d in tools/harnessgen tools/harnesslint tools/plugindrift; do ( cd "$d" && GOFLAGS=-buildvcs=false go test ./... -timeout 120s ) \|\| rc=1; done; echo "suites=$rc"` | suites=0 | exit 0; ok harnessgen 14.3s, ok harnesslint 9.8s, ok plugindrift 33.1s; suites=0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2a | `mkdir -p plugins/assay/skills/zzz-canary-skill && printf ... > plugins/assay/skills/zzz-canary-skill/SKILL.md; ( cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... -run 'Codex\|Committed' -timeout 120s ) > <out> 2>&1; echo "drift-exit=$?"; rm -rf plugins/assay/skills/zzz-canary-skill` | drift-exit non-zero | drift-exit=1; --- FAIL: TestCodexCommittedManifestMatchesSource and --- FAIL: TestCursorCommittedRuleMatchesSource (with -v added; output file redirected under <scratch> instead of the row's /tmp path). Canary removed, tree clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `grep -A30 -E 'harnesslint' .github/workflows/ci.yml \| grep -Ec 'bodies\|bindings'` | at least 2 | exit 0; printed 3 (the job's two go run invocations plus its comment) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bodies ../../plugins/assay/skills; echo "bodies=$?"` | checked-clean then bodies=0 | checked-clean: bodies — no violations; bodies=0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4a | `t=$(git rev-parse --show-toplevel); d=<scratch dir>; cp -R "$t/plugins/assay/skills" "$d/skills"; printf ... >> "$d/skills/ask-decision/SKILL.md"; cd "$t/tools/harnesslint" && go run . --vocab ... bodies "$d/skills" > /dev/null 2>&1; echo "planted=$?"` | planted=1 | planted=1; one violation line on the planted file (count only, report body not kept). Run with a literal scratch dir in place of mktemp and a literal-path removal after (see Notes). SUBSTANCE PASS; as authored the witness could not run it (prose-led first code span) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | `cd tools/harnesslint && go run . --vocab ../../docs/streams/harness-portability/README.md bindings ../../plugins/assay/references; echo "bindings=$?"` | checked-clean then bindings=0 | checked-clean: bindings — no violations; bindings=0. Four files announced on stderr as declared non-matrix (desk-common.md, desk-shell.md, standing-note.md, tick-contract.md); claude-code.md, codex.md, cursor.md fully checked | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5a | `t=$(git rev-parse --show-toplevel); d=<scratch dir>; cp -R "$t/plugins/assay/references" "$d/refs"; printf '# undeclared junk reference ...' > "$d/refs/zzz-undeclared.md"; cd "$t/tools/harnesslint" && go run . --vocab ... bindings "$d/refs" > /dev/null 2>&1; echo "undeclared=$?"` | undeclared=1 | undeclared=1; 8 capability-closure violations, all on zzz-undeclared.md, none on the four declared files or the three matrices. Witness ran the row exactly as authored and recorded the same output (hash match) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 6 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... -run 'NonMatrix\|Skip\|Bindings' -timeout 60s; echo "hl-suite=$?"` | hl-suite=0 | hl-suite=0; with -v 12 tests PASS, among them the declared-file-excluded test, TestCheckBindings_UndeclaredReferenceStillChecked, the declared-neighbour narrowness test, TestCheckBindings_NonMatrixDeclarationRequiresReason, TestCheckBindings_AllDeclaredIsCouldNotCheck and TestNonMatrixDeclaration | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 7 | `statusgen --lint --root .; echo $?` | 0, no PROBLEM | LINT: PASS; 0 PROBLEM lines; printed 0. statusgen built from the clone's statusgen/ at b0088804 (--version dev) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 8 | `awk '/^## v1\.0\.0 /{f=1;next} /^## /{f=0} f' CHANGELOG.md \| grep -qE '^- .*harnesslint'; echo "changelog=$?"` | changelog=0 | FAIL AS AUTHORED under the witness shell: in zsh and plain bash it prints changelog=0 (two harnesslint bullets present in v1.0.0), but under bash -o pipefail, which is how verifyrun runs every row, it prints changelog=141 on 3 of 3 runs (grep -q exits at the first match, awk takes SIGPIPE). The witness recorded pass only because the trailing echo exits 0. Check-definition failure; substance passes by hand | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 9 | `git grep -n '<<<<<<<' -- . \| wc -l; git diff --stat origin/main...HEAD -- ':(exclude)docs/streams/harness-portability' ':(exclude)changelog'` | 0 markers; narrow diffstat | FAIL AS AUTHORED: printed 16, and an empty diffstat by construction (HEAD equals origin/main). Substance passes by hand: all 16 hits in 11 files are prose, detection code or test fixtures; the delivering commits' touch-set outside the stream dir and changelog is ci.yml, the two SKILL.md bodies, tools/harnesslint files, desk-shell.md, plus references/claude-code.md (the relocation target Task steps 3 and 4 require) and the .codex-plugin version (parity regen). Check-definition failure | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — nonMatrixMarker = "<!-- assay:harnesslint non-matrix-reference" @ tools/harnesslint/lint.go:75 — the brief's Task step 5 names this exact form, symmetric with the existing assay:capability-vocabulary and assay:banned-tokens markers; a declaration needs a closing --> and a non-empty reason (lint.go:185, :192), every excluded file is printed on stderr, an all-declared directory is could-not-check, and row 5a plus the fail-first tests show an undeclared neighbour is still fully checked.
RISK-VALUE: DERIVED — version = "1.0.30" @ plugins/assay/.codex-plugin/plugin.json:3 — not a chosen value: harnessgen regenerates it from plugins/assay/.claude-plugin/plugin.json:5 (also 1.0.30) and TestCodexManifestVersionEqualsClaude pins equality (PASS in row 2a's -v run); the 0.5.1 to 1.0.0 change in c87fd7619 was that parity repair.
RISK-VALUE: DERIVED — extra = "go test ./..." @ .github/workflows/ci.yml:78-80, runs-on "medici-builder-public" @ ci.yml:179, checkout pin 3d3c42e5aac5 @ ci.yml:181 — copied from the existing tools/desk arm and the sibling skillslint job; the unauthenticated public read of ci run 36693857646 at b0088804 shows build-test and harnesslint (step "harnesslint bodies + bindings (real tree)") both concluded success.

Notes:
- BLOCKED (check-definition), not a product failure. The CI wiring is on main (#1813) and green at this SHA; hand rows 1, 2, 2a, 3, 4, 4a, 5, 5a, 6 and 7 pass. Row 4a (the Command cell opens with a prose code span) and row 9 (as written it scans the whole tree and diffs HEAD against itself) could not run in the witness; row 8 prints changelog=141 under `bash -o pipefail` (grep -q exits early and awk takes SIGPIPE) and the witness scores it pass only because the trailing echo exits 0. `statusgen brief --check-verified` with a hypothetical flip exits 1 (rows 4a and 9; it does not catch row 8). Advancing needs rows 4a, 8 and 9 re-authored, tracked with the other row re-authors at #1927.
- Deliverable substance: every item (a), (b), (c) is present on main and behaves as the brief says; the two prior FAIL causes (CI wiring missing; a real-tree bindings drift) are both gone. The block is solely check-definition on rows 4a, 8 and 9.
- Row 8 is a new finding, not seen in earlier passes: under the witness's bash -o pipefail the command prints changelog=141, so the recorded witness pass is masked by the trailing echo. Suggested re-author: grep the file without a pipe, e.g. `awk '...' CHANGELOG.md > <tmp>; grep -qE '^- .*harnesslint' <tmp>`, or make grep read the whole stream (drop -q, count lines).
- Row 4a: prose-led first code span; verifyrun records could-not-run. Suggested fix: a cmd: code span, or move the prose to Expect (rows 2a and 5a already work because their first span is the command).
- Row 9: verifyrun treats the three-dot range as a placeholder (could-not-run); as authored the marker count is 16 legitimate hits, not 0, and the diffstat is empty once merged (lint moving-ref NOTICE). Suggested fix: pin the base to the delivering merge's first parent and grep for markers at line start only.
- check-verified: with a hypothetical README flip of row 15 to verified plus a dated Verified cell, `statusgen brief --root . --check-verified harness-portability/15` exits 1 on both throwaway clones: on a fresh clone every row lacks a witness; on the witness copy it names row 4a (no witness) and row 9 (could-not-run). It does not catch row 8, whose witness reads pass.
- Lint and verifyrun disagree about `\|` in these rows: lint NOTICEs (ere-literal-pipe) read it as a literal backslash-pipe on rows 1 and 3, while verifyrun unescapes it to alternation (output hashes match 3 and 8 for row 1, 3 for row 3, drift-exit=1 for row 2a). Under the raw reading rows 1 and 3 print 0 and rows 2a and 6 select no tests (row 6 would still exit 0, vacuously). Re-authoring with separate -e patterns would remove the ambiguity.
- Lint also raises a risk-files-crossread NOTICE: all four risk answers are no while files: names .github/workflows/ci.yml. The change adds read-only test and lint steps under inherited contents: read and the existing action pin; whether the answers stand is for the desk to route, not decided here.
- Row 2a side note: TestCommittedArtifactsMatchSource stays green with the canary planted; the catch comes from the Codex and Cursor committed-manifest tests.
- Row 2a as authored writes /tmp/hp15r2a.out; the witness did so (left in place, a test log only). The hand run redirected into <scratch>.
- Rows 4a and 5a hand runs: the local harness refused a removal whose target was a command substitution, so the hand runs used literal scratch dirs and removed them by literal path afterwards; the go run invocations and printed exit codes are unchanged. The witness executed 5a exactly as authored.
- tools/harnesslint/ci.yml.patch (the staged sidecar from c87fd7619) is still on main now that the real wiring has landed; it is redundant and could be removed in a follow-up.
- The declaration is found anywhere in a file's body (strings.Index), not only as a header line, so a binding matrix that quoted the marker in prose would leave the matrix with only a stderr line and a green job. Hygiene observation on a neutrality lint, not a security control.

VERIFY: BLOCKED

### 2026-10-02 desk dispatch — Verification — 2026-10-02T22:58Z (non-implementer hand run on merged main e1d99484ffd9)

Runner is not the implementer. Detached worktree at merged main e1d99484ffd9, gate: model, all four risk answers no. Every row was run by hand from the repository root exactly as authored (the table-escaped pipe read as a pipe), under bash with a clean environment and a throwaway home directory; statusgen was built from this tree's own statusgen source (version dev). Commands are abbreviated in the Command column; the authored text is the Verify table above. No row is classed check:ci.

| # | Command | Exit | Observed | Date | Runner |
| --- | --- | --- | --- | --- | --- |
| 1 | two greps of ci.yml for the three module dirs in the go test case | 0 | printed 3 then 8; the three go test arms are at ci.yml lines 78 to 80, beside the kept tools/desk arm at 77. Against the ci.yml that preceded #1813 the same row prints 0 and 0 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | go test of all packages in harnessgen, harnesslint, plugindrift | 0 | ok for all three modules; suites=0 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2a | plant a canary skill, run the harnessgen Codex and Committed tests, remove the canary | 0 | drift-exit=1; the captured log shows two FAIL lines, the Codex committed-manifest test and the Cursor committed-rule test; canary removed and tree clean afterwards | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | grep of ci.yml for bodies and bindings near harnesslint | 0 | printed 3 (expect at least 2). Against the ci.yml that preceded #1813 it prints 0 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | harnesslint bodies over the shipped skills tree | 0 | checked-clean: bodies — no violations; bodies=0. With the two skill bodies restored to their pre-#714 text in a scratch copy the same call reports 4 violations and exits 1 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4a | positive control: banned token appended to a scratch copy of ask-decision, then harnesslint bodies | 0 | planted=1; an unsilenced repeat shows exactly one violation, on the planted line, checked-failed: bodies — 1 violation(s). Count only, report body not kept | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | harnesslint bindings over the shipped references tree | 0 | four files announced as declared non-matrix (desk-common, desk-shell, standing-note, tick-contract); checked-clean: bindings — no violations; bindings=0. With the declaration line removed from desk-shell in a scratch copy the same call reports 8 violations, all on desk-shell, and exits 1 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5a | narrowness control: undeclared junk reference added to a scratch copy, then harnesslint bindings | 0 | undeclared=1; an unsilenced repeat shows 8 capability-closure violations, all on the junk file, none on the declared files or the three matrices | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | harnesslint go test filtered to NonMatrix, Skip, Bindings | 0 | ok; hl-suite=0. With -v added, 12 tests PASS, among them the declared-file-skipped test, the undeclared-still-checked test, the neighbour-narrowness test, the reason-required test and the all-declared could-not-check test | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | statusgen lint of the repo root, then echo of its exit | 0 | LINT: PASS; zero PROBLEM lines; printed 0 | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | awk of the v1.0.0 changelog section piped to a quiet grep for a harnesslint bullet | 0 | changelog=0 under plain bash (two matching bullets in the v1.0.0 section). Under bash with pipefail, the shell the execution witness uses, it printed changelog=141 on 2 of 2 runs: the quiet grep exits at the first match and awk takes SIGPIPE. The trailing echo makes the row exit 0 either way. Check-definition defect; substance passes | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 9 | whole-tree count of conflict-marker strings, then a diffstat of main against HEAD | 0 | FAIL as authored: printed 18 where the Expect cell says 0, and an empty diffstat by construction (HEAD is main). All 18 hits, in 11 files, are prose, detection code or test fixtures (5 of them in this brief); no real unresolved conflict. Check-definition defect; substance passes | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (dry run, nothing written): 10 of 12 rows proven, exit 2. Rows 1, 2, 2a, 3, 4, 5, 5a, 6, 7 and 8 recorded pass exit=0. Row 4a: could-not-run, prose-led command (the first code span in the Command cell is a lone word ahead of the real command). Row 9: could-not-run, the three-dot range is read as an unsubstituted placeholder. The witness pass on row 8 is exit-status only and is masked by the trailing echo (see row 8 above).

What changed since the 2026-09-30 pass: nothing in the brief's Verify table; rows 4a, 8 and 9 are still authored as they were, so the same three check-definition defects hold. The deliverable is unchanged and still present: CI wiring on main since #1813, skill-body scrubs and the declared skip since #714. The marker count on row 9 moved from 16 to 18 as unrelated files were added. #1332 (the request for a human push of the CI wiring) is still open although #1813 delivered that push; it can be closed citing #1813. The 2026-09-30 note says the row re-authors are tracked at #1927; that issue is open but neither its body nor its comments name this brief, so no open issue found by this pass tracks the re-author of rows 4a, 8 and 9 here.

Vacuity:
- Rows 1, 3, 4 and 5 discriminate: each fails against the pre-change input (shown in the rows above).
- Rows 4a, 5a and 2a are controls and each went red on its planted fault with the fault visible in unsilenced output.
- Row 6 discriminates on the tests this brief added (five of the twelve selected tests exercise the declared skip).
- Row 8 discriminates in substance (both matching bullets are this brief's), but its exit status cannot fail: the row ends in echo.
- Row 2 is non-discriminating on its own: the three suites passed before this brief; it shows only that what the new CI arms run is green. Row 2a carries the proof.
- Row 7 is non-discriminating for this brief: a repo-wide lint that passes with or without the work.
- Rows 4, 5, 6, 7 and 8 all end in echo, so their exit status is always 0; only the printed value decides them, and the witness scores exit status only.
- Row 9 cannot pass as authored on merged main.

Lint NOTICE lines that name this brief (not PROBLEM): ere-literal-pipe on rows 1 and 3, verify-row-portability on row 2a, prose-led-command on row 4a, moving-ref and unsubstituted-metavar on row 9, a consumers list with no consumers row, a one-sided depends edge to brief 14, and risk-files-crossread (all four risk answers are no while the files list names a CI workflow path). The last one is for the desk or a human to route; it is not decided here.

Risk values (enumerated over #714 and #1813; all are reversible by an edit, none is a threshold, tolerance, ratio or timeout):

RISK-VALUE: DERIVED — nonMatrixMarker (the `assay:harnesslint non-matrix-reference` comment marker) @ tools/harnesslint/lint.go:75 — the form Task step 5 of this brief names, symmetric with the two markers the tool already parsed; a declaration must be closed and carry a reason (lint.go:185 and :192), every skipped file is announced, and row 5a plus the row 5 mutation show the skip keys on the declaration alone.
RISK-VALUE: DERIVED — extra = "go test ./..." @ .github/workflows/ci.yml:78-80 — Task step 1 of this brief asks for exactly this, copied from the existing tools/desk arm at line 77.
RISK-VALUE: NAMED, NOT DERIVED — runs-on = "medici-builder-public" and the checkout action pin 3d3c42e5aac5 in the harnesslint job @ .github/workflows/ci.yml:179 and :181 — both are identical to the other three jobs in the same file (4 of 4 each), so they follow the file's standing convention; this pass did not independently confirm the pin resolves to the tagged release it is commented as.
RISK-VALUE: N/A (adjacent, checked) — the plugin version is 1.0.32 in both the Claude and the Codex plugin manifest; it is regenerated, not chosen, and is not introduced by this brief.

Other observations: the staged sidecar patch under tools/harnesslint is still on main and is redundant now that the wiring has landed. The relocation in item (b) holds: the two harness-specific tokens are absent from both skill bodies and present in the Claude reference file.

Suggested re-authors, unchanged from the prior pass: row 4a, start the Command cell with the command (move the prose to Expect); row 8, drop the quiet flag or avoid the pipe, and drop the trailing echo; row 9, pin the base to the delivering merge and match markers at line start only.

VERIFY: BLOCKED — check-definition. Rows 1, 2, 2a, 3, 4, 4a, 5, 5a, 6 and 7 pass by hand and every deliverable (a), (b), (c) is on main and behaves as the brief says; rows 4a and 9 cannot be witnessed as authored (witness 10 of 12, exit 2), row 9 prints 18 against an Expect of 0, and row 8 prints 141 under the witness shell. Status stays implemented until rows 4a, 8 and 9 are re-authored.

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
