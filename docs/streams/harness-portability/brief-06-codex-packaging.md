---
brief: harness-portability/06
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
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "superpowers 6.2.0 precedent: .codex-plugin/plugin.json with skills: ./skills/ and hooks: {} — a Codex plugin manifest over the same skills tree", "harness-portability/01's install-mechanism + skills-discovery matrix rows (the authoritative schema facts; superpowers' shape is re-verified there, not inherited)", "the harness-target ruling (HP/03): ruled target set + channel — what to generate and where it installs from", "the plugin SOURCES coverage rule (every skill pinned or declared, unaccounted = hard error, exit 2) — the rule this brief ports into harnessgen", "freshness-checked 2026-08-07 (no .codex-plugin exists anywhere in this repo)"]
consumers: ["plugins/assay/.claude-plugin/plugin.json: fixed-here (becomes the version/metadata source harnessgen reads; not edited beyond what generation needs)", ".claude-plugin/marketplace.json: out-of-scope (Claude marketplace surface; unchanged by a Codex artifact)", "adopt skill (plugins/assay/skills/adopt/SKILL.md): fixed-here (gains the Codex install scenario)", "docs/adopting-assay.md + PARITY/RELEASE-NOTES: follow-up harness-portability/07", "the publication review: out-of-scope here (the generated artifacts are ordinary repo files; the publication manifest classifies them like everything else)"]
exec-tier: strong
exec-tier-why: >-
  (b): correctness is cross-artifact by definition — manifest vs bundle tree vs
  binding files vs the ruled degradation matrix must agree, and the failure mode is a
  skew no single-file test sees.
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
| 3a | **Mutation — skew detected**: `jq '.version="9.9.9"' plugins/assay/.codex-plugin/plugin.json > /tmp/hp06skew.json && cp /tmp/hp06skew.json plugins/assay/.codex-plugin/plugin.json && go run ./tools/harnessgen codex --check > /tmp/hp06r3a.out 2>&1; echo $?; git checkout -- plugins/assay/.codex-plugin/plugin.json` | non-zero naming the manifest; after checkout, `--check` passes again |
| 4 | `go run ./tools/harnessgen codex --check; echo $?` | `0` |
| 5 | **Mutation — coverage closed**: `mkdir -p /tmp/hp06-tree && cp -r plugins/assay /tmp/hp06-tree/ && mkdir /tmp/hp06-tree/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp06-tree/assay/skills/probe-skill/SKILL.md && go build -o /tmp/hp06gen ./tools/harnessgen && /tmp/hp06gen codex --check --bundle /tmp/hp06-tree/assay > /tmp/hp06r5.out 2>&1; echo $?; rm -rf /tmp/hp06-tree` | exit `2`, output names `probe-skill` — an unaccounted skill is a hard error, the coverage discipline held (built binary so the three-state exit `2` is observable; `go run` collapses non-zero to `1`) |
| 6 | **Mutation — binding consistency**: `mkdir -p /tmp/hp06-bind && cp -r plugins/assay /tmp/hp06-bind/ && grep -vF 'worker-desk' plugins/assay/references/codex.md > /tmp/hp06-bind/assay/references/codex.md && go build -o /tmp/hp06gen ./tools/harnessgen && /tmp/hp06gen codex --check --bundle /tmp/hp06-bind/assay > /tmp/hp06r6.out 2>&1; echo $?; rm -rf /tmp/hp06-bind` | exit `2` naming `worker-desk` — packaging-vs-binding skew is a build error |
| 7 | Adopt path present: `grep -qiF 'codex' plugins/assay/skills/adopt/SKILL.md && grep -qF 'AGENTS-assay' plugins/assay/skills/adopt/SKILL.md; echo $?` | `0` — two independent greps ANDed (install scenario + fragment step both present) |
| 7a | **Positive control for row 7** — `grep -qF 'AGENTS-assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | `1` — the probe reports absence for an absent token |
| 8 | **Neighbour row** — `go run ./tools/harnessgen resident --check; echo $?` | `0` — the 05 verb still passes beside the new one (shared generator plumbing) |

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

## Review

Gate: **model** (from frontmatter). Review focus: the exclusion list — every skill
excluded from Codex packaging must cite 03's ruling or 01's matrix, never convenience;
and the adopt scenario's install steps must match the measured `install-mechanism` row,
not superpowers' 2026-03 shape.
