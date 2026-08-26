---
brief: harness-portability/12
title: Cursor — the third harness column
why: >-
  The seam this stream built (neutral capability vocabulary + a thin per-harness
  binding file + a generated per-harness packaging artifact + a ruled degradation
  floor) was designed so a third harness is "a new column, not a fork." Ian ruled
  Cursor in (2026-08-26): target BOTH surfaces, headless-first. Cursor's 2026
  convergence (native AGENTS.md, the same SKILL.md open standard, hooks, MCP,
  background agents in isolated git worktrees) makes it a LIGHTER lift than Codex —
  more skills run as-is, zero `absent` capability rows. This brief adds the Cursor
  column end-to-end, proving the extensibility claim with a second vendor family.
wave: 5
depends: ["harness-portability/03", "harness-portability/04", "harness-portability/05", "harness-portability/06"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 by harness-portability authoring session (Cursor third-column dispatch)
sources: ["authoring dispatch (Ian, 2026-08-26): target BOTH Cursor surfaces, headless-first — the headless cursor-agent CLI primary, the IDE agent secondary", "the measured Cursor capability matrix (HP/12 ground-truth: documentary + public-docs sweep, no live environment 2026-08-26; §5 live-confirm rows)", "the Codex chain this mirrors: HP/01 ground-truth, HP/03 ruling, HP/04 binding+lint, HP/06 packaging — Cursor is the same shape, lighter", "the harness-target ruling (the non-negotiable floor C: isolation/evidence/review-gates never degrade)", "cursor.com/docs {cli/headless, context/rules, skills, hooks} + 2026 third-party write-ups (all dated in the research doc's Appendix A)"]
consumers: ["plugins/assay/references/cursor.md: fixed-here (new binding, capability→mechanism + per-skill degradation)", "tools/harnessgen: fixed-here (new `cursor` verb + tests; readPackaging gains a marker param, codex callsite updated)", "plugins/assay/cursor/{packaging.md,assay.mdc}: fixed-here (coverage roster + generated .mdc rule)", "plugins/assay/skills/adopt/SKILL.md: fixed-here (Cursor install scenario, section 2c)", "plugins/assay/codex/AGENTS-assay.md: out-of-scope (reused unchanged — Cursor reads AGENTS.md natively, the shared fragment)", "docs/how-assay-works.md: fixed-here (Cursor column added to the capability table)", "docs/adopting-assay.md: out-of-scope (Cursor quickstart, sibling change)", "freshness.yaml: fixed-here (binding + research doc rot on a 45-day clock)"]
exec-tier: strong
exec-tier-why: >-
  (b): correctness is cross-artifact — the generated .mdc vs the resident source, the
  coverage roster vs the skills tree, the binding cells vs the packaged set, and the
  neutrality lint over the bodies must all agree; the failure mode is a skew no
  single-file test sees.
---

# Brief 12 — Cursor: the third harness column

> **Tool de-house note.** The generator work this brief describes — the `harnessgen`
> `cursor` verb + tests, and the generated `plugins/assay/cursor/{packaging.md,assay.mdc}`
> artifacts — lands with the `tools/harnessgen` + method-text de-house (the sequenced
> follow-on that brings the generator source and the `plugins/assay/{codex,cursor,resident}/`
> outputs to public, the same shape `statusgen`/`desk-tools` followed). Until then the Verify
> tables' `go run ./tools/harnessgen` / `harnesslint` commands run in the tool's source tree.

## Context

**The human ruling is captured (P0, gate:human input, recorded here):** Ian, 2026-08-26 —
**target BOTH Cursor surfaces, headless-first.** The headless `cursor-agent` CLI is the
primary surface (best fit to the desk/automation model and the isolation/evidence/
review-gate floor); the in-editor IDE agent is the secondary end-user surface. This is
reflected in the binding's capability→mechanism cells and the degradation column
(`plugins/assay/references/cursor.md`) and in the adopt flow (section 2c).

files:
- **create** `docs/research/cursor-harness-capabilities.md` (planned) — the §2.1–§2.10 capability
  ground-truth matrix, re-measured for Cursor (documentary; live-confirm rows flagged).
- **create** `plugins/assay/references/cursor.md` — binding file (capability→mechanism +
  per-skill degradation), mirroring `codex.md`, carrying **fewer** degradations.
- **create** `plugins/assay/cursor/packaging.md` (planned) — the coverage roster (SOURCES.yaml
  discipline), and **create** `plugins/assay/cursor/assay.mdc` — the GENERATED
  `.cursor/rules` resident-rules rule.
- **amend** `tools/harnessgen/` — new verb `cursor` (+ `--check`), `cursorRules()`
  generator, `readPackaging` marker param, cursor_test.go.
- **amend** `plugins/assay/skills/adopt/SKILL.md` — Cursor install scenario (section 2c).
- **amend** `docs/how-assay-works.md` — Cursor column in the capability table.
- **amend** `freshness.yaml`, `docs/streams/harness-portability/README.md`. **CI needs
  no workflow edit**: the tools workflow already triggers on `plugins/**` + `tools/**` +
  this stream README, and its `go test` matrix runs `TestCursorCommittedRuleMatchesSource`
  (`cursor --check`), exactly how `codex --check` is enforced (via the suite, no separate
  step). Not editing `.github/workflows/**` also keeps the PR pushable by the worker App
  (which has no `workflows` permission).

facts:
- **Cursor is lighter than Codex.** Codex CLI's `workspace-isolation` was `absent`
  (forcing `worker-desk` to refuse); Cursor has **zero `absent` rows** and native
  isolated-worktree background agents (§2.9). So `worker-desk` **runs** on the IDE surface,
  and headless it runs-or-refuses on a live-confirm of `git worktree add` permission.
- **No new plugin manifest.** Cursor consumes `SKILL.md`, `AGENTS.md`, and
  `.cursor/rules/*.mdc` directly from the repo tree (§2.10) — the packaging IS the
  instruction files. The one generated artifact is the `.mdc` rule; the coverage +
  binding discipline the `codex` verb runs guards the rest. Cursor reads the shared Codex
  `AGENTS.md` fragment natively, so the adopt flow offers either resident-rules channel.
- **The single-source guarantee holds.** `plugins/assay/cursor/assay.mdc` is generated
  from `plugins/assay/resident-rules.md` (planned) — the SAME source as the Claude payload and the
  Codex fragment — so the three cannot drift; `cursor --check` byte-compares in CI.
- **What a live install cannot yet confirm** is authored honestly, not fabricated: the
  rows in `docs/research/cursor-harness-capabilities.md` (planned) §5 are marked
  **`[needs: live-install confirmation]`**, NOT asserted `supported`.

## The one gate:model open item + the gate:human acceptance step

- **gate:model open item — the capability ground-truth matrix (P0).** Authored from the
  research's web-verified capabilities. The rows that turn on a behaviour only a live
  Cursor install settles are flagged `[needs: live-install confirmation]` (research §5):
  (1) headless `cursor-agent` `sessionStart`-hook coverage; (2) whether background-agent
  worktree isolation / `git worktree add` is reachable from the adopter's headless flow
  (the row that decides `worker-desk` runs vs refuses); plus the secondary confirmations
  (auto-trigger ergonomics headless, headless parallel-dispatch reachability). These are
  **not** greened here — no Cursor install is available to this session.
- **gate:human acceptance step — a live Cursor smoke run.** The parity acceptance is a
  live session on Cursor (external dependency: a Cursor install), exactly the posture the
  Codex stream held for its live smoke (HP/07). Ian provides/sanctions the environment;
  until then the live-confirm rows stay flagged, degraded-never, silent-never.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Draft PR only.
- Stop at `implemented` — you do not set verified/done.
- Do NOT fabricate ground-truth you cannot run: flag live-only rows, never assert them.
- Path-specific `git add`; never commit STATUS.md.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... >/tmp/hp12r1.out 2>&1; echo $?` | `0` — includes `TestCursor*` (drift, coverage, binding-skew, parse-error, frontmatter) |
| 2 | `go run ./tools/harnessgen cursor --check; echo $?` | `0` — committed `.mdc` matches the single resident source (`plugins/assay/resident-rules.md` (planned)) + coverage + binding all clean |
| 3 | **Mutation — drift detected**: `printf '\nX\n' >> "$PWD/plugins/assay/cursor/assay.mdc" && go build -o /tmp/hg12 ./tools/harnessgen && /tmp/hg12 cursor --check >/tmp/hp12r3.out 2>&1; echo $?; go run ./tools/harnessgen cursor` | `1` naming `assay.mdc`; the `.mdc` is untracked so the revert is a regenerate (`harnessgen cursor`), after which `--check` passes again |
| 4 | **Mutation — coverage closed**: `mkdir -p /tmp/hp12t && cp -r plugins/assay /tmp/hp12t/ && mkdir /tmp/hp12t/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp12t/assay/skills/probe-skill/SKILL.md && go build -o /tmp/hg12 ./tools/harnessgen && /tmp/hg12 cursor --check --bundle /tmp/hp12t/assay >/tmp/hp12r4.out 2>&1; echo $?; rm -rf /tmp/hp12t` | exit `2`, output names `probe-skill` — an unaccounted skill is a hard error |
| 5 | **Mutation — binding consistency**: `mkdir -p /tmp/hp12b && cp -r plugins/assay /tmp/hp12b/ && sed 's/`the-desk`/the-desk/g' plugins/assay/references/cursor.md > /tmp/hp12b/assay/references/cursor.md && go build -o /tmp/hg12 ./tools/harnessgen && /tmp/hg12 cursor --check --bundle /tmp/hp12b/assay >/tmp/hp12r5.out 2>&1; echo $?; rm -rf /tmp/hp12b` | exit `2` naming `the-desk` — packaging↔binding skew is a build error |
| 6 | Neutrality holds: `go run ./tools/harnesslint bodies plugins/assay/skills && go run ./tools/harnesslint bindings plugins/assay/references; echo $?` | `0` — adopt's Cursor section stays neutral; `cursor.md` resolves every capability + has a cell per skill |
| 7 | Neighbours unbroken: `go run ./tools/harnessgen resident --check && go run ./tools/harnessgen codex --check; echo $?` | `0` — the `resident` and `codex` verbs still pass beside the new one |
| 8 | Adopt path present: `grep -qi 'cursor' plugins/assay/skills/adopt/SKILL.md && grep -qF 'cursor/assay.mdc' plugins/assay/skills/adopt/SKILL.md; echo $?` | `0` — install scenario + the generated-rule step both present |
| 8a | **Positive control for row 8**: `grep -qF 'cursor/assay-no-such-token' plugins/assay/skills/adopt/SKILL.md; echo $?` | `1` — the probe reports absence for an absent token |
| 9 | `.mdc` frontmatter: `grep -qF 'alwaysApply: true' plugins/assay/cursor/assay.mdc; echo $?` | `0` — the generated rule carries the `.cursor/rules` always-apply contract |
| 10 | Live-confirm rows flagged, not asserted: `grep -c 'needs: live-install confirmation' docs/research/cursor-harness-capabilities.md` | `≥ 5` — the unrunnable rows are flagged, never greened |
| 11 | New entries fresh: `go run ./tools/freshness 2>&1 \| grep -E -e 'references/cursor.md' -e 'cursor-harness'` | both new entries report `FRESH` (the tool's whole-repo exit is 1 only from pre-existing unrelated stale artifacts, never from these entries) |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s), date, runner). Mutation rows pasted, not
     summarised. "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd tools/harnessgen && go test ./...` | 0 | `ok github.com/medici-finance/assay/tools/harnessgen 0.472s` (incl. `TestCursor*` drift/coverage/binding-skew/parse-error/frontmatter reds) | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 2 | `go run ./tools/harnessgen cursor --check` | 0 | `clean — plugins/assay/cursor/assay.mdc matches the resident source` | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 3 | `/tmp/hg12 cursor --check` after appending a line | 1 | `DRIFT — committed rule plugins/assay/cursor/assay.mdc differs from the resident source`; the `.mdc` is a new (untracked) file so the revert is `go run ./tools/harnessgen cursor` (regenerate), after which `--check` → 0 clean | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 4 | `/tmp/hg12 cursor --check --bundle …` (probe-skill added) | 2 | `could-not-check: coverage rule failed … skill "probe-skill" is on disk but appears in neither the packaged roster nor the excluded list` | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 5 | `/tmp/hg12 cursor --check --bundle …` (the-desk cell stripped) | 2 | `could-not-check: packaging↔binding skew … packaged skill "the-desk" has no degradation cell (\`the-desk\`) in …/cursor.md` | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 6 | `harnesslint bodies … && harnesslint bindings …` | 0 | `checked-clean: bodies` + `checked-clean: bindings` | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 7 | `resident --check && codex --check` | 0 | `resident: clean` + `codex: clean` (both verbs unbroken beside `cursor`) | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 8 | `grep -qi 'cursor' … && grep -qF 'cursor/assay.mdc' …` | 0 | both present (install scenario + generated-rule step) | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 8a | `grep -qF 'cursor/assay-no-such-token' …` | 1 | absent token reports absence (positive control) | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 9 | `grep -qF 'alwaysApply: true' …/assay.mdc` | 0 | frontmatter carries `alwaysApply: true` | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 10 | `grep -c 'needs: live-install confirmation' …/cursor-harness-capabilities.md` | 0 | `9` (≥ 5 — live-only rows flagged, not asserted) | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |
| 11 | `go run ./tools/freshness 2>&1 \| grep -E -e references/cursor.md -e cursor-harness` | (exit ignored) | `FRESH  docs/research/cursor-harness-capabilities.md …` + `FRESH  plugins/assay/references/cursor.md …`; whole-repo `freshness` exits 1 ONLY from pre-existing unrelated stale artifacts, not these entries | 2026-08-26 | assay-worker-app[bot] opus-4.8[1m] |

_Non-Verify sanity: `gofmt -l tools/harnessgen/` empty, `go vet ./tools/harnessgen` clean._

## Review

Gate: **model** (from frontmatter) for everything in this PR — all git-revertible text
and tooling; nothing touches funds, customers, regulators, or an irreversible surface.
The **live Cursor smoke run is a separate gate:human acceptance step** (external
dependency: a Cursor install, Ian's), the same posture HP/07 held for Codex. Review
focus: (1) the binding's degradation cells honour the non-negotiable floor
(isolation/evidence/review-gates never degrade; `worker-desk` runs on IDE, runs-or-refuses
headless — never silently degrades); (2) no row the research could not run is asserted
`supported` — the `[needs: live-install confirmation]` flags are load-bearing; (3) the
`.mdc` and coverage roster stay single-sourced (the `cursor --check` byte-compare + the
coverage/binding mutation tests are the proof).
