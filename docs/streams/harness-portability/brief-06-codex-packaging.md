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

### Non-implementer verifier run — VERIFY: PASS — 10/10 pass, 0 could-not-check, 0 fail — 2026-09-24 claude-opus-4-8-verifier

Runner not the implementer; own detached temp worktree cut off origin/main at merged head;
offline envelope observed (KUBECONFIG=/dev/null); no PR, push, or status flip. Third
independent verify pass. Runner cells copy the form statusgen verifyrun printed. This pass
is the first to observe row 7 PASS: the adopt-skill Codex install scenario + AGENTS-assay
fragment step landed on main (commit 1ef5edeac, PR-referenced #1357), closing the gap the
2026-09-11 and 2026-09-18 passes held FAIL on (tracked at #872).

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | 0 — includes skew/coverage red tests | PASS — exit 0 (ok github.com/medici-finance/assay/tools/harnessgen, includes the skew/coverage red tests) | 2026-09-24 | claude-opus-4-8-verifier |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | 0 — manifest parses, required fields present | PASS — exit 0 (jq printed true; manifest parses, required fields present) | 2026-09-24 | claude-opus-4-8-verifier |
| 3 | `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | 0 — versions equal | PASS — exit 0 (the .codex-plugin and .claude-plugin manifest versions compared equal) | 2026-09-24 | claude-opus-4-8-verifier |
| 3a | `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | non-zero naming the manifest; clean again after checkout | PASS — exit 1 (expected non-zero: "DRIFT — committed manifest plugins/assay/.codex-plugin/plugin.json differs from the metadata source"); after checkout row 4 re-runs clean; worktree clean | 2026-09-24 | claude-opus-4-8-verifier |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | 0 — manifest matches metadata source | PASS — exit 0 ("clean — plugins/assay/.codex-plugin/plugin.json matches the metadata source") | 2026-09-24 | claude-opus-4-8-verifier |
| 5 | `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | exit 2, output names probe-skill | PASS — exit 2 as expected ("coverage rule failed … skill \"probe-skill\" is on disk but appears in neither the packaged roster nor the excluded list") | 2026-09-24 | claude-opus-4-8-verifier |
| 6 | `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | exit 2 naming worker-desk | PASS — exit 2 as expected ("packaging↔binding skew … packaged skill \"worker-desk\" has no degradation cell in references/codex.md"; also names pr-shepherd, the-desk — the grep -vF stripped every line containing the substring) | 2026-09-24 | claude-opus-4-8-verifier |
| 7 | `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | 0 — install scenario + fragment step both present | PASS — exit 0 (both greps hit — the Codex install scenario and the AGENTS-assay fragment step are both present in plugins/assay/skills/adopt/SKILL.md). First pass to observe row 7 PASS: the gap closed on main | 2026-09-24 | claude-opus-4-8-verifier |
| 7a | `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | 1 — probe reports absence of an absent token | PASS — exit 1 as expected (the absent token is correctly reported absent) | 2026-09-24 | claude-opus-4-8-verifier |
| 8 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | 0 — brief 05 verb still passes beside the new one | PASS — exit 0 ("clean — committed artifacts match the source"; the brief-05 verb still passes beside the new one) | 2026-09-24 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — the three-state exit gate exitClean = 0, exitDrift = 1, exitCouldNotCheck = 2 @ tools/harnessgen/main.go:22-24 is the top-ranked risk-bearing literal introduced by this item's diff; a wrong could-not-check value would let an unaccounted or binding-skewed skill ship silently. Derivation: it is the canonical three-valued instrument contract (0 = checked-clean, 1 = checked-failed/drift, 2 = could-not-check), each state a distinct process exit so a caller and CI can branch on which of the three occurred; the mutation rows observe all three live this pass — clean = 0 (rows 4, 8), drift = 1 (row 3a), could-not-check = 2 (rows 5, 6, built binary). Secondary: the manifest version = 1.0.27 @ plugins/assay/.codex-plugin/plugin.json:3 is NOT an independently-set constant — it is generation-derived and equality-bound to plugins/assay/.claude-plugin/plugin.json:5 (row 3), so it carries no independent-value risk. The exclusion list is empty (no exclusion to justify). Enumeration found no other introduced literal.

Row 7 now passes: the Codex install scenario landed in #1357, resolving the gap tracked at #872.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ 45d4f34a59de (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verify on merged main 45d4f34a59de53b58e9516624609c014d4ff0ec8, which equals the forge's `commits/main`. No harness-portability or harnessgen path changed between that SHA and the landing main. The Linux machine witness below is the statusgen verifyrun record. It ran on a fresh copy of the tree in a network-off container (golang 1.25 bookworm, jq 1.6 mounted read-only). The dry-run and the writing run agree.


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | pass exit=0 | sha256:542e6e399ba3 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 3 | `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 3a | `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | pass exit=0 | sha256:f556ad938565 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 5 | `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 6 | `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 7 | `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 7a | `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |
| 8 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | pass exit=0 | sha256:3baad508f1f3 | 2026-09-30 | assay-verifier-app[bot] @ 45d4f34a59de (on-behalf-of human:ian) (git-config) |


Every row ends with `echo $?` or a cleanup command, so the witness exit is always 0. What proves the printed code is the output hash. Decoded on the host: `0` + newline = 9a271f2a916b, `1` + newline = 4355a46b19d3, `2` + newline = 53c234e5e847. So rows 1, 3, 7 printed 0; rows 3a and 7a printed 1; rows 5 and 6 printed 2. Each is the Expect value.

Verifier detail (harness-portability/06 — NON-implementer, merged main 45d4f34a59de, 2026-09-30). Every row was also run by hand on the host, with the rows' scratch prefix moved to a private temp dir:

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | row 1 command as written | 0 | printed 0; ok for the harnessgen package | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 2 | row 2 command as written | true, exit 0 | printed true, exit 0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 3 | row 3 command as written | 0 | printed 0; both manifests at version 1.0.30 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 3a | row 3a command as written | 1, drift named | printed 1; DRIFT, the committed Codex manifest differs from the metadata source; tree clean after the restore | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 4 | row 4 command as written | 0 | printed 0; clean, the manifest matches the metadata source | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 5 | row 5 command as written | 2, names probe-skill | printed 2; could-not-check, coverage rule failed, skill probe-skill in neither the packaged roster nor the excluded list | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 6 | row 6 command as written | 2, names worker-desk | printed 2; packaging-binding skew, worker-desk has no degradation cell (the grep also strips the pr-shepherd and the-desk cells, so all three are named) | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 7 | row 7 command as written | 0 | printed 0; the adopt skill section 5 carries the Codex install arm with the AGENTS-assay fragment step | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 7a | row 7a command as written (positive control) | 1 | printed 1; the absent token is reported absent | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |
| 8 | row 8 command as written | 0 | printed 0; clean, committed artifacts match the source | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 45d4f34a59de |

Risk-bearing values. Risk metadata is present, all four flags are `no`, and no risk-classed path is touched. Enumerated anyway:

RISK-VALUE: DERIVED — exit codes clean 0 / drift 1 / could-not-check 2 @ tools/harnessgen/main.go:22-24 — the repo's three-state instrument contract; all three states were observed live in this pass (rows 4, 3a, 5 and 6).
RISK-VALUE: DERIVED — Skills = "./skills/" @ tools/harnessgen/codex.go:217 — the brief says the manifest points at the same skills tree; the 14 skill directories match the 14 roster entries in plugins/assay/codex/packaging.md.
RISK-VALUE: DERIVED — manifest version 1.0.30 @ plugins/assay/.codex-plugin/plugin.json:3 — generated from and equality-bound to the Claude manifest version; row 3a shows a planted skew goes red.
RISK-VALUE: NAMED, NOT DERIVED — the 32 KiB composition cap @ plugins/assay/skills/adopt/SKILL.md:101 — confirming it needs upstream Codex sources outside the offline envelope; reversible doc text in a gate: model, irreversible: no brief. The desk routes this as a `question` issue before the flip; until that issue exists the brief stays `implemented` with this Evidence recorded.

Findings (none block): (F1) the witness alone cannot judge these rows because each ends in `echo $?`; the hash decode above and the host run carry the proof. (F2) row 6 names three skills, not one, because its grep strips three cells. (F3) the Codex check runs in the release workflow (the tag-cut gate), not in per-PR CI; the brief's CI task is met at release. (F4) lint NOTICEs only: the consumers field has no consumers row, and the Verify rows declare no dereference or flow obligation.

VERIFY: PASS

### Non-implementer verifier run: 2026-10-02T22:38:23Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Runner is not the implementer. Own detached worktree at the merged head named above, which equals the forge's `commits/main` at run time. Offline envelope observed (`KUBECONFIG=/dev/null`). Every row was run by hand. Two disclosed deviations from the authored text, neither changing what a row measures: (a) in the hand run the rows' `/tmp/hp06*` scratch paths were redirected into a private scratch directory; (b) row 3a's mutation edits a tracked file, so it was run verbatim inside a scratch clone checked out at the same sha, not in the verifier worktree. Go commands ran under a throwaway HOME. Worktree clean after the pass.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | `0`, includes the skew/coverage red tests | printed 0; "ok github.com/medici-finance/assay/tools/harnessgen". A separate `-v` pass shows 32 tests, all `--- PASS`, including TestCodexCheckDetectsVersionSkew, TestCodexCoverageCatchesUnaccountedSkill, TestCodexExcludedEmptyReasonIsParseError, TestCodexBindingSkewCaught | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | `0` | printed true then 0; name "assay", version "1.0.32", skills "./skills/" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | `0` | printed 0; both manifests read 1.0.32 (a real value on each side, so the equality is not null-equals-null) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3a | `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | non-zero naming the manifest; clean again after checkout | printed 1; "harnessgen codex --check: DRIFT — committed manifest ../../plugins/assay/.codex-plugin/plugin.json differs from the metadata source"; after the checkout the same check printed "clean" and exit 0, clone status clean (run in a scratch clone at the same sha) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | `0` | printed 0; "harnessgen codex --check: clean — ../../plugins/assay/.codex-plugin/plugin.json matches the metadata source" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | exit `2`, output names `probe-skill` | printed 2; "could-not-check: coverage rule failed … skill \"probe-skill\" is on disk but appears in neither the packaged roster nor the excluded list" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | exit `2` naming `worker-desk` | printed 2; "could-not-check: packaging↔binding skew … packaged skill \"worker-desk\" has no degradation cell" (pr-shepherd and the-desk are named too: the row's grep strips every line containing the substring) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | `0` | printed 0; the adopt skill's section 5 "Codex CLI — the second-harness install arm" carries the fragment step at line 98. History check: both tokens entered this file in commit 1ef5edeac (#1357), the change made for this brief | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7a | `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | `1` | printed 1; the absent token is reported absent | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | `0` | printed 0; "harnessgen resident --check: clean — committed artifacts match the source" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Result by hand: 10 of 10 rows pass (1, 2, 3, 3a, 4, 5, 6, 7, 7a, 8). No row failed and every row could be executed on this host.

Execution witness (`statusgen verifyrun --brief … --dry-run`, statusgen v1.0.31, rows as authored, nothing written to the brief): exit 0, 10 of 10 rows `pass exit=0`. Per row, with the output hash decoded (`0`+newline = 9a271f2a916b, `1`+newline = 4355a46b19d3, `2`+newline = 53c234e5e847):

- row 1: pass, sha256:9a271f2a916b (printed 0)
- row 2: pass, sha256:542e6e399ba3 (printed true then 0)
- row 3: pass, sha256:9a271f2a916b (printed 0)
- row 3a: pass, sha256:4355a46b19d3 (printed 1)
- row 4: pass, sha256:f556ad938565 (clean line then 0)
- row 5: pass, sha256:53c234e5e847 (printed 2)
- row 6: pass, sha256:53c234e5e847 (printed 2)
- row 7: pass, sha256:9a271f2a916b (printed 0)
- row 7a: pass, sha256:4355a46b19d3 (printed 1)
- row 8: pass, sha256:3baad508f1f3 (clean line then 0)

The hashes are identical to the 2026-09-30 container witness for every row, and each decoded value is the row's Expect value.

Findings (none block):

- F1. The witness judges exit status only, and every row ends in `echo $?` or a cleanup, so its `pass exit=0` cannot fail on its own. The hash decode above and the hand run carry the proof.
- F2. No row passes vacuously. Row 1 has no `-run` filter and the `-v` pass shows the ten TestCodex tests executing. Row 7's tokens were introduced by #1357 (checked with `git log -S`), not present before it. Row 3 compares two real version strings.
- F3. Row 6 names three skills, not one (see its cell). The Expect still holds.
- F4. Per-PR CI runs `go test ./...` for the harnessgen module, which includes TestCodexCommittedManifestMatchesSource; the explicit `codex --check` invocation is in the release workflow. The brief's CI task is met by the two together.
- F5. Since the 2026-09-24 outcome record (sha 2a5c230e): the brief file changed (four Evidence-only commits, the last being #1873); statusgen moved v1.0.26 to v1.0.31; both plugin manifests moved in lockstep 1.0.27 to 1.0.32 with the generated resident fragment; a smoke-roster test file was added to harnessgen (#1963). No change to the harnessgen codex code, the adopt skill, or the Codex binding file.
- F6. #872 (the row 7 gap) is closed. The 2026-09-30 run said a `question` issue would be routed for the 32 KiB cap; no such issue was found, and this pass resolves that value from the repo's own record (below), so none is needed.

Risk-bearing values. Risk metadata is present, all four answers are `no`, the brief is `gate: model`. Enumerated over the codex verb in tools/harnessgen, the generated manifest, and the adopt-skill change (#1357):

RISK-VALUE: DERIVED — exitClean = 0, exitDrift = 1, exitCouldNotCheck = 2 @ tools/harnessgen/main.go:22-24 — the three-state instrument contract (clean / drift / could-not-check as distinct exits so CI can branch); a wrong value would let an unaccounted or binding-skewed skill ship. All three observed live (rows 4 and 8; row 3a; rows 5 and 6).
RISK-VALUE: DERIVED — Skills = "./skills/" @ tools/harnessgen/codex.go:217 — the brief's Context requires the manifest to point at the same skills tree the Claude plugin serves; row 4 proves the committed manifest equals the generated one and row 5 proves the tree is closed under the coverage rule.
RISK-VALUE: DERIVED — version = "1.0.32" @ plugins/assay/.codex-plugin/plugin.json:3 — generated from and equal to plugins/assay/.claude-plugin/plugin.json:5; not independently set (row 3, row 3a).
RISK-VALUE: DERIVED — composition cap 32 KiB @ plugins/assay/skills/adopt/SKILL.md:101 — matches the stream's own capability ground truth (docs/research/codex-harness-capabilities.md, AGENTS.md row: 32 KiB cap on the combined total, `project_doc_max_bytes`) and the adoption guide's Codex section. That source is documentation-measured; the ground-truth file lists a live truncation test as an open item. Reversible doc text, ranked last.

Status note: the stream README row for brief 06 reads `implemented` with empty verifier and reviewer cells. Nothing in the Verify table or the risk metadata holds the row there; the remaining step is the status change itself.

VERIFY: PASS

### Non-implementer verifier re-run — VERIFY: PASS — 2026-10-04 claude-opus-5-5-verifier

Runner is not the implementer. Merged main bed1a31ba875549c069fe49276d5b6eeaa0d147c, run in a throwaway clone detached at that sha with its origin/main pinned to the same sha. The verifier worktree's tracked tree was not modified. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR, push, comment, issue or status change. Brief frontmatter: `gate: model`, risk regulatory no, customer no, irreversible no, sensitive-data no.

Why this re-run: the 2026-09-24 receipt (sha 2a5c230e) went stale because its declared inputs moved. The brief file changed by Evidence-only additions (four Evidence commits plus the 2026-10-02 batch re-verify, #2068; no line of Task, DoD or Verify changed). The statusgen tool moved from v1.0.26. Beyond the receipt's declared inputs, the change that does touch what this brief verifies is #2146 (commit 17458cf08), which added a new bundle skill, cut-release, and registered it in the Codex coverage roster (plugins/assay/codex/packaging.md) and the Codex binding file (plugins/assay/references/codex.md). That is exactly the surface of rows 4, 5 and 6: a new skill missing from either file would turn row 4 red. Other changes since the receipt: both plugin manifests moved in lockstep to 1.0.32, the generated resident fragment was restamped, and a smoke-roster test was added to harnessgen (#1963). The harnessgen codex code and the adopt skill did not change.

Blocker check: the receipt's blocker_ref is the note "verified flip is a separate single-brief PR", not an issue number, so there is no issue to check. The one issue this brief's Evidence names, #872 (the row 7 gap), is CLOSED (2026-09-20).

Hand run. Every row was run by hand on the darwin host inside the throwaway clone, under a throwaway HOME, with GOPROXY=off and -count=1. Disclosed deviation: the rows' `/tmp/hp06*` scratch paths were redirected into a private scratch directory under the verifier worktree. Row 3a's mutation edits a tracked file, so it ran in the clone.

| # | Verify row discharged | Expect (from the brief text) | Observed (exit + key output line) | Date / runner |
|---|---|---|---|---|
| 1 | row 1, harnessgen go test | `0`, includes the skew and coverage red tests | printed 0; "ok github.com/medici-finance/assay/tools/harnessgen". A separate -v -run Codex pass shows ten TestCodex tests all PASS, among them TestCodexCheckDetectsVersionSkew, TestCodexCoverageCatchesUnaccountedSkill, TestCodexExcludedEmptyReasonIsParseError and TestCodexBindingSkewCaught | 2026-10-04 claude-opus-5-5-verifier |
| 2 | row 2, jq required fields | `0` | printed true then 0; name "assay", version "1.0.32", skills "./skills/" | 2026-10-04 claude-opus-5-5-verifier |
| 3 | row 3, version equality | `0` | printed 0; both manifests read 1.0.32, real values on both sides | 2026-10-04 claude-opus-5-5-verifier |
| 3a | row 3a, planted skew mutation | non-zero naming the manifest; clean again after checkout | printed 1; "harnessgen codex --check: DRIFT — committed manifest ../../plugins/assay/.codex-plugin/plugin.json differs from the metadata source". After the checkout, row 4 printed clean and 0, and the clone status was clean | 2026-10-04 claude-opus-5-5-verifier |
| 4 | row 4, codex --check | `0` | printed 0; "harnessgen codex --check: clean — ../../plugins/assay/.codex-plugin/plugin.json matches the metadata source". This includes the new cut-release skill (on the coverage roster at packaging.md line 38, with a degradation cell at codex.md line 49) | 2026-10-04 claude-opus-5-5-verifier |
| 5 | row 5, coverage mutation (built binary) | exit `2`, names probe-skill | printed 2; "could-not-check: coverage rule failed … skill \"probe-skill\" is on disk but appears in neither the packaged roster nor the excluded list". Control: the same built binary on an unmutated copy of the bundle printed clean and 0, so the exit 2 comes from the planted skill | 2026-10-04 claude-opus-5-5-verifier |
| 6 | row 6, binding mutation (built binary) | exit `2`, names worker-desk | printed 2; "could-not-check: packaging↔binding skew … packaged skill \"worker-desk\" has no degradation cell". It also names pr-shepherd and the-desk, because the row's grep removes every line containing the substring | 2026-10-04 claude-opus-5-5-verifier |
| 7 | row 7, adopt skill greps | `0` | printed 0; both tokens present in the adopt skill | 2026-10-04 claude-opus-5-5-verifier |
| 7a | row 7a, positive control | `1` | printed 1; the absent token is reported absent | 2026-10-04 claude-opus-5-5-verifier |
| 8 | row 8, resident --check | `0` | printed 0; "harnessgen resident --check: clean — committed artifacts match the source" | 2026-10-04 claude-opus-5-5-verifier |

Result by hand: 10 of 10 rows pass. None failed, none was left unrun, and none passes vacuously: row 1 has no -run filter, rows 5 and 6 have an unmutated control, and row 3 compares two real version strings.

Execution witness. `statusgen verifyrun --brief docs/streams/harness-portability/brief-06-codex-packaging.md --root /work`, then `statusgen verifyrun --check` on the same brief. Both ran in a Linux container: golang:1.25-bookworm (cached, `--pull never`), `--network none`, and `--security-opt seccomp=unconfined`. That option was used only inside this throwaway, network-off container. Mounts: the throwaway clone at /work, the host Go module cache read-only, and the roster read-only. No credentials, tokens or PEMs were mounted. Inside the container: git safe.directory `*`, an in-container git identity of assay-verifier-app[bot] with its noreply email, statusgen built from the clone's own source (`go build`, reporting version "dev"), and GOFLAGS=-count=1 GOPROXY=off GOTOOLCHAIN=local. Disclosed deviation: the image has no jq, so gojq v0.12.15 was built inside the container from the read-only module cache (`GOPROXY=file:///go/pkg/mod/cache/download`, still no network) and linked as jq. Exact command:

`docker run --rm --pull never --network none --security-opt seccomp=unconfined -v <home>/.v/clone:/work -v $(go env GOMODCACHE):/go/pkg/mod:ro -v ~/.config/assay/roster.env:/root/.config/assay/roster.env:ro golang:1.25-bookworm bash -c "<witness script>"`

verifyrun --check summary: `docs/streams/harness-portability/brief-06-codex-packaging.md: 10 pass, 0 fail, 0 could-not-run/missing (of 10 Verify rows)` (exit 0).

Witness table, copied byte-for-byte from the tool's output:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp06r1.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 2 | `jq -er '.name and .version and .skills' plugins/assay/.codex-plugin/plugin.json; echo $?` | pass exit=0 | sha256:542e6e399ba3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 3 | `test "$(jq -r .version plugins/assay/.claude-plugin/plugin.json)" = "$(jq -r .version plugins/assay/.codex-plugin/plugin.json)"; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 3a | `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..) > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 4 | `(cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | pass exit=0 | sha256:f556ad938565 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 5 | `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | pass exit=0 | sha256:53c234e5e847 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 6 | `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hp06gen . && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | pass exit=0 | sha256:53c234e5e847 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 7a | `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 8 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..); echo $?` | pass exit=0 | sha256:3baad508f1f3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |

Every row ends with `echo $?` or a cleanup command, so the witness exit is always 0. The proof of the printed code is the output hash. Decoded on the host: `0`+newline = 9a271f2a916b, `1`+newline = 4355a46b19d3, `2`+newline = 53c234e5e847. So rows 1, 3 and 7 printed 0; rows 3a and 7a printed 1; rows 5 and 6 printed 2. Each is the row's Expect value, and each hash is identical to the 2026-09-30 and 2026-10-02 witnesses.

Risk-bearing values. Risk metadata is present and all four answers are `no`; the brief is `gate: model`. Enumerated over the codex verb in tools/harnessgen, the generated manifest, the adopt-skill change, and the roster and binding entries added since the receipt:

RISK-VALUE: DERIVED — exitClean = 0, exitDrift = 1, exitCouldNotCheck = 2 @ tools/harnessgen/main.go:22-24 — the three-state instrument contract (clean, drift and could-not-check as distinct exits, so CI can branch on each); a wrong value would let an unaccounted or binding-skewed skill ship. All three were observed live: rows 4 and 8 (0), row 3a (1), rows 5 and 6 (2).
RISK-VALUE: DERIVED — Skills = "./skills/" @ tools/harnessgen/codex.go:217 — the brief's Context requires the manifest to point at the same skills tree the Claude plugin uses. Row 4 proves the committed manifest equals the generated one, and row 5 proves the tree is closed under the coverage rule.
RISK-VALUE: DERIVED — version = "1.0.32" @ plugins/assay/.codex-plugin/plugin.json:3 — generated from, and equal to, the Claude manifest version, not set independently (rows 3 and 3a).
RISK-VALUE: NAMED, NOT DERIVED — composition cap 32 KiB @ plugins/assay/skills/adopt/SKILL.md:101 — this pass did not re-derive it. It matches the stream's documentation-measured capability record, as the 2026-10-02 pass noted, but confirming it against Codex itself needs upstream sources outside the offline envelope. It is reversible doc text and ranks last.

Findings (none block): (F1) The witness judges exit status only, because every row ends in `echo $?`; the hash decode and the hand run carry the proof. (F2) Row 6 names three skills, not one. (F3) The new cut-release skill (#2146) entered the coverage roster and the binding file in the same change, so the coverage discipline held for a live addition, not only for the planted probe.

VERIFY: PASS

## Review

Gate: **model** (from frontmatter). Review focus: the exclusion list — every skill
excluded from Codex packaging must cite 03's ruling or 01's matrix, never convenience;
and the adopt scenario's install steps must match the measured `install-mechanism` row,
not superpowers' 2026-03 shape.
