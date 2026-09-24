---
brief: assay:assay:harness-portability:06
title: Codex packaging — generated manifest, coverage rule, install path
why: >-
  With neutral skills (04) and generated rule delivery (05) in place, the remaining gap
  is the container: Codex needs its own manifest and install shape, and hand-writing a
  second manifest is a divergence surface (version skew, a skill listed in one and not
  the other). Generating the Codex packaging from the Claude manifest + the bundle
  tree, with a closed coverage rule and a CI byte-compare, ships the second harness
  without shipping a second thing to maintain.
wave: 3
depends: ["harness-portability/03", "harness-portability/04", "harness-portability/05"]
unblocks: ["harness-portability/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "superpowers 6.2.0 precedent: .codex-plugin/plugin.json with skills: ./skills/ and hooks: {} — a Codex plugin manifest over the same skills tree", "harness-portability/01's install-mechanism + skills-discovery matrix rows (the authoritative schema facts; superpowers' shape is re-verified there, not inherited)", "the harness-target ruling (HP/03): ruled target set + channel — what to generate and where it installs from", "the plugin SOURCES coverage rule (every skill pinned or declared, unaccounted = hard error, exit 2) — the rule this brief ports into harnessgen", "freshness-checked 2026-08-07 (no .codex-plugin exists anywhere in this repo)"]
consumers: ["plugins/assay/.claude-plugin/plugin.json: fixed-here (becomes the version/metadata source harnessgen reads; not edited beyond what generation needs)", ".claude-plugin/marketplace.json: out-of-scope (Claude marketplace surface; unchanged by a Codex artifact)", "adopt skill (plugins/assay/skills/adopt/SKILL.md): fixed-here (gains the Codex install scenario)", "docs/adopting-assay.md + PARITY/RELEASE-NOTES: follow-up harness-portability/07", "the publication review: out-of-scope here (the generated artifacts are ordinary repo files; the publication manifest classifies them like everything else)"]
exec-tier: strong
exec-tier-why: >-
  (b): correctness is cross-artifact by definition — manifest vs bundle tree vs
  binding files vs the ruled degradation matrix must agree, and the failure mode is a
  skew no single-file test sees.
version: 1
id: 9a19f81d-f0c4-414c-a6e3-2c14d402cc4c
---

# Brief 06 — Codex packaging

## Context

files:
- **create** `plugins/assay/.codex-plugin/plugin.json` — GENERATED (path contingent on
  01's measured schema + 03's ruling; superpowers' shape is the working hypothesis)
- **amend** `tools/harnessgen` — new verb `codex` (+ `--check`), coverage rule
- **amend** `plugins/assay/skills/adopt/SKILL.md` — Codex install scenario
- **amend** the tools CI workflow — extend the regenerate-and-diff check to the new verb

facts:
- **Everything derivable is generated**: name, version, description, author, homepage
  come from `.claude-plugin/plugin.json` (single metadata source); the skills roster
  comes from the bundle tree. Hand-edits to the generated file are overwritten and CI
  `--check` makes them fail loudly.
- **Coverage rule (ported from the plugin SOURCES discipline)**: every `skills/*/SKILL.md`
  must be either packaged for Codex or excluded in harnessgen's config with a written
  reason (e.g. a skill 03 ruled `refuses` on Codex may still ship — refusal text is
  method — vs a skill that genuinely cannot exist there). A skill in neither set → exit 2.
  "Every packaged skill is valid" means nothing until the packaged set is known to be the
  whole set.
- **The manifest points at the SAME `skills/` tree** the Claude plugin uses — the
  neutral core (04) is what makes one tree servable to both. No copied skill files, no
  per-harness skills dir.
- If 01 measured that Codex CLI (as distinct from the App) has no plugin/skill
  mechanism, the CLI install path is file-placement per 01's `install-mechanism` row
  (documented in adopt), and the manifest serves the App only — the brief covers
  whichever set 03 ruled in.
- Version skew is the classic failure: the two manifests must carry the same version
  forever, which is why one is generated from the other (row 3).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **No marketplace submission, no external PR** — distribution beyond the bundle is
  post-publication and out of this stream (03's ruling).
- If 01's matrix contradicts superpowers' packaging shape, follow the matrix and record
  the contradiction in the PR body — never ship the hypothesis over the measurement.

## Task

1. Extend `tools/harnessgen` with verb `codex`: emit the manifest from the metadata
   source + bundle tree; implement the coverage rule (packaged / excluded-with-reason /
   else exit 2); `--check` regenerate-and-diff. Table-driven tests: version skew
   planted → red; skill added to tree without roster/exclusion entry → exit 2; excluded
   entry with empty reason → parse error.
2. Generate and commit the manifest; wire CI `--check`.
3. Add the Codex install scenario to the adopt skill (install steps per 01's
   `install-mechanism` row + 03's channel ruling, including the AGENTS.md fragment
   installation from 05 and the `multi_agent` config note from the binding file).
4. Cross-artifact consistency: harnessgen asserts every skill in the manifest has a
   degradation cell in the codex binding file (04's `plugins/assay/references/` deliverable) —
   skew between packaging and binding is a build error, not a doc bug. Exercised via the
   same `--bundle` override used for coverage (row 5/6) — no separate flag.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | `0` — includes the skew/coverage red tests (task 1) |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | `0` — the manifest exists, parses, and carries the required fields (adjust path/fields to 01's measured schema in the same commit as the ruling, never silently) |
| 3 | Version skew impossible: `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | `0` |
| 3a | **Mutation — skew detected**: `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | non-zero naming the manifest; after checkout, `--check` passes again |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | `0` |
| 5 | **Mutation — coverage closed**: `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | exit `2`, output names `probe-skill` — an unaccounted skill is a hard error, the coverage discipline held (built binary so the three-state exit `2` is observable; `go run` collapses non-zero to `1`) |
| 6 | **Mutation — binding consistency**: `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | exit `2` naming `worker-desk` — packaging-vs-binding skew is a build error |
| 7 | Adopt path present: `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | `0` — two independent greps ANDed (install scenario + fragment step both present) |
| 7a | **Positive control for row 7** — `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | `1` — the probe reports absence for an absent token |
| 8 | **Neighbour row** — `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | `0` — the 05 verb still passes beside the new one (shared generator plumbing) |

## Evidence

<!-- The delivery + independent verification below ran in the stream's source tree
     before this public re-home; the harnessgen `codex` verb, the generated manifest and
     the coverage rule are part of the sequenced tool de-house follow-on (see the stream
     README re-home note). The Verify table above is the executable spec. -->

**Delivery & verification (in the source tree, before this re-home).** The `codex` verb
landed in `tools/harnessgen` with the coverage rule (packaged / excluded-with-reason /
else exit 2), the generated `.codex-plugin/plugin.json` (version derived from the single
metadata source, kept equal by generation), the adopt Codex install scenario, and the CI
`--check`. An independent non-implementer verifier re-ran all rows — **VERIFY: PASS**
(`gate: model`, all risk answers `no`): the table-driven tests (skew / coverage / parse /
binding-skew) pass; the three-state exit gate was observed at all three values —
`clean→0`, `drift→1` (planted version skew), and `could-not-check→2` (an unaccounted
`probe-skill`, and a packaging↔binding skew when a skill's degradation cell is removed),
the exit-2 observable only via a built binary (`go run` collapses non-zero to `1`).

Two Verify commands as originally authored were re-baselined, not defects in the work:
a skew-probe token (`batch-fanout`) had been renamed to `worker-desk` by a later skill
rename post-dating the brief (the property re-runs green with the current name — rows 5/6
above use `worker-desk` and a built-binary invocation).

**Risk-bearing value.** `exitCouldNotCheck = 2` (with `exitClean=0`, `exitDrift=1`) — the
three-state gate; a wrong value would let unaccounted skills ship silently, and the
mutation rows prove it fires. Manifest version is derived-and-equality-bound to the single
metadata source, not independently set; the exclusion list is empty (no exclusion to
justify).
### Verify run — 2026-09-11, non-implementer dispatched verifier (opus-4.8[1m]-verifier) — VERDICT: FAIL (held at `implemented`)

Ran the Verify table against public medici-finance/assay merged main `553dc2ae530f00e861a14536af7cc884ab77ccf5` (two-protocol head confirmed), offline in an isolated worktree; all mutation controls restored, worktree left clean. Non-implementer. tools/harnessgen is its own Go module (no repo-root go.mod), so the literal `go run ./tools/harnessgen …` rows fail go.mod-not-found; the real properties were run module-aware via a built binary (non-blocking command-string note).

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | go test in tools/harnessgen | 0 | ok tools/harnessgen | PASS |
| 2 | jq name+version+skills on plugins/assay/.codex-plugin/plugin.json | 0 | true (all present) | PASS |
| 3 | version equality vs plugins/assay/.claude-plugin/plugin.json | 0 | both 1.0.7 — equal | PASS |
| 3a | mutate manifest version to 9.9.9 → codex --check → revert | 1 | DRIFT (committed manifest differs); recheck after revert clean; tree clean | PASS |
| 4 | harnessgen codex --check (module-aware) | 0 | clean — the manifest matches the metadata source | PASS |
| 5 | built binary + planted undeclared skill | 2 | coverage rule failed — skill "probe-skill" on disk but in neither the packaged roster nor the excluded list | PASS |
| 6 | built binary + removed a degradation cell | 2 | packaging↔binding skew — packaged skill "worker-desk" has no degradation cell in references/codex.md | PASS |
| 7 | grep codex AND AGENTS-assay in plugins/assay/skills/adopt/SKILL.md | 1 | both greps 0 hits — the file is a 58-line thin pointer to docs/adopting-assay.md and carries neither token | FAIL |
| 7a | positive control — grep an absent token | 1 | absent token reports absence | PASS |
| 8 | harnessgen resident --check (module-aware) | 0 | clean — committed artifacts match the source | PASS |

**Why FAIL — split-delivery gap.** The Codex packaging backend all landed and passes (rows 1–6, 8): the generated `.codex-plugin/plugin.json` with version bound equal to the `.claude-plugin` manifest, the coverage rule failing closed at exit 2, the binding-skew check at exit 2, and the resident verb intact. But Verify row 7's deliverable — the Codex install scenario + the AGENTS-assay fragment step in `plugins/assay/skills/adopt/SKILL.md`, which this brief's own `consumers` frontmatter marks `fixed-here` (distinct from `docs/adopting-assay.md`, scoped as `follow-up harness-portability/07`) — did not land in the public tree. `plugins/assay/skills/adopt/SKILL.md`'s git history carries only the open-core drop and a guardrails consolidation; the task-3 amendment is absent. `docs/adopting-assay.md` carries a Codex row but not the AGENTS-assay token, and the adopt SKILL the row targets has neither. Filed as #872 (bug, →worker). Brief stays at `implemented`; re-run row 7 after the adopt-skill amendment lands (or after Verify row 7 + the consumers frontmatter are retargeted, if the pointer-only design is intended — a spec call).

**Risk-bearing value:** `RISK-VALUE: DERIVED — the three-state exit gate exitCouldNotCheck=2 / exitDrift=1 / exitClean=0 (tools/harnessgen/main.go) is the top risk-bearing literal; a wrong value would let an unaccounted or binding-skewed skill ship silently. Observed live at all three: clean=0 (rows 4/8), drift=1 (row 3a), could-not-check=2 (rows 5 & 6, built binary). Manifest version equality (1.0.7==1.0.7) is equality-bound to the single metadata source, not independently set.`
### Non-implementer verifier re-run — VERIFY: FAIL (row 7 pre-existing content gap, already tracked) — sonnet-5-verifier (verify-desk dispatch), @ merged main `5fbf75834e1d2e5a80b44524649b4030f50e80f1`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted. Second independent verify pass (prior: 2026-09-11).

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/harnessgen && go test ./...` | exit 0 | exit 0, ok | 2026-09-18 | sonnet-5-verifier |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json` | exit 0 | exit 0, true | 2026-09-18 | sonnet-5-verifier |
| 3 | version compare both manifests | exit 0 | exit 0, both 1.0.14 | 2026-09-18 | sonnet-5-verifier |
| 3a | mutation: plant version skew, check, revert | exit 1 naming manifest, clean after | checked-failed as expected, then checked-clean after revert | 2026-09-18 | sonnet-5-verifier |
| 4 | `harnessgen codex --check` | exit 0 | exit 0, clean — matches metadata source | 2026-09-18 | sonnet-5-verifier |
| 5 | mutation: plant undeclared skill under --bundle | exit 2 naming it | checked-failed (could-not-check per tool's own contract) as expected, skill named | 2026-09-18 | sonnet-5-verifier |
| 6 | mutation: strip a degradation cell under --bundle | exit 2 naming it | checked-failed as expected, cell named | 2026-09-18 | sonnet-5-verifier |
| 7 | grep for Codex mentions + AGENTS-assay step in adopt/SKILL.md | exit 0 | **FAIL** — zero hits; file unchanged since 2026-09-11, 58 lines, no Codex install scenario or AGENTS-assay step | 2026-09-18 | sonnet-5-verifier |
| 7a | control: grep an absent token | exit 1 | exit 1, correctly absent | 2026-09-18 | sonnet-5-verifier |
| 8 | `harnessgen resident --check` | exit 0 | exit 0, clean — committed artifacts match source | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all rows map 1:1 to Verify rows; no invented scope.

RISK-VALUE: DERIVED — exitClean=0, exitDrift=1, exitCouldNotCheck=2 @ tools/harnessgen/main.go:22-24 — top-ranked (a wrong value silently ships an unaccounted/skew skill); all three states observed live this pass (rows 3a, 4/8, 5/6).

VERIFY: FAIL — held at implemented. Same real, unresolved content gap as the 2026-09-11 pass, confirmed unchanged: rows 1-6,8 all pass (harnessgen codex verb, manifest version binding, coverage + binding-skew checks all sound); row 7 fails because adopt/SKILL.md still lacks a Codex install scenario and the AGENTS-assay step this brief's own consumers frontmatter marks fixed-here. Not a stale-anchor case — a genuine unresolved gap, already tracked at medici-finance/assay#872 (confirmed still OPEN). No new issue filed.

### Non-implementer verifier run — VERIFY: PASS — 2026-09-24 claude-opus-4-8-verifier

Runner not the implementer; own detached temp worktree cut off origin/main at merged head;
offline envelope observed (KUBECONFIG=/dev/null); no PR, push, or status flip. Third
independent verify pass. Runner cells copy the form statusgen verifyrun printed. This pass
is the first to observe row 7 PASS: the adopt-skill Codex install scenario + AGENTS-assay
fragment step landed on main (commit 1ef5edeac, PR-referenced #1357), closing the gap the
2026-09-11 and 2026-09-18 passes held FAIL on (tracked at #872).

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | 0 — includes skew/coverage red tests | exit 0 — ok github.com/medici-finance/assay/tools/harnessgen | 2026-09-24 | claude-opus-4-8-verifier |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | 0 — manifest parses, required fields present | exit 0 — jq printed true | 2026-09-24 | claude-opus-4-8-verifier |
| 3 | `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | 0 — versions equal | exit 0 — both manifests 1.0.27 | 2026-09-24 | claude-opus-4-8-verifier |
| 3a | `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | non-zero naming the manifest; clean again after checkout | exit 1 — "DRIFT — committed manifest plugins/assay/.codex-plugin/plugin.json differs from the metadata source"; after checkout row 4 re-runs clean; worktree clean | 2026-09-24 | claude-opus-4-8-verifier |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | 0 — manifest matches metadata source | exit 0 — "clean — plugins/assay/.codex-plugin/plugin.json matches the metadata source" | 2026-09-24 | claude-opus-4-8-verifier |
| 5 | `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | exit 2, output names probe-skill | exit 2 — "coverage rule failed … skill \"probe-skill\" is on disk but appears in neither the packaged roster nor the excluded list" | 2026-09-24 | claude-opus-4-8-verifier |
| 6 | `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | exit 2 naming worker-desk | exit 2 — "packaging↔binding skew … packaged skill \"worker-desk\" has no degradation cell in references/codex.md" (also names pr-shepherd, the-desk — the grep -vF stripped every line containing the substring) | 2026-09-24 | claude-opus-4-8-verifier |
| 7 | `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | 0 — install scenario + fragment step both present | exit 0 — both greps hit; 12 codex mentions, AGENTS-assay at line 98 (plugins/assay/codex/AGENTS-assay.md install step); file now 120 lines | 2026-09-24 | claude-opus-4-8-verifier |
| 7a | `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | 1 — probe reports absence of an absent token | exit 1 — absent token correctly reported absent | 2026-09-24 | claude-opus-4-8-verifier |
| 8 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | 0 — brief 05 verb still passes beside the new one | exit 0 — "clean — committed artifacts match the source" | 2026-09-24 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — the three-state exit gate exitClean = 0, exitDrift = 1, exitCouldNotCheck = 2 @ tools/harnessgen/main.go:22-24 is the top-ranked risk-bearing literal introduced by this item's diff; a wrong could-not-check value would let an unaccounted or binding-skewed skill ship silently. Derivation: it is the canonical three-valued instrument contract (0 = checked-clean, 1 = checked-failed/drift, 2 = could-not-check), each state a distinct process exit so a caller and CI can branch on which of the three occurred; the mutation rows observe all three live this pass — clean = 0 (rows 4, 8), drift = 1 (row 3a), could-not-check = 2 (rows 5, 6, built binary). Secondary: the manifest version = 1.0.27 @ plugins/assay/.codex-plugin/plugin.json:3 is NOT an independently-set constant — it is generation-derived and equality-bound to plugins/assay/.claude-plugin/plugin.json:5 (row 3), so it carries no independent-value risk. The exclusion list is empty (no exclusion to justify). Enumeration found no other introduced literal.

Row 7 now passes: the Codex install scenario landed in #1357, resolving the gap tracked at #872.


## Review

Gate: **model** (from frontmatter). Review focus: the exclusion list — every skill
excluded from Codex packaging must cite 03's ruling or 01's matrix, never convenience;
and the adopt scenario's install steps must match the measured `install-mechanism` row,
not superpowers' 2026-03 shape.
