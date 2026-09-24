---
brief: assay:assay:harness-portability:12
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
schema: brief-v2
authored: 2026-08-26 by harness-portability authoring session (Cursor third-column dispatch)
sources: ["authoring dispatch (Ian, 2026-08-26): target BOTH Cursor surfaces, headless-first — the headless cursor-agent CLI primary, the IDE agent secondary", "the measured Cursor capability matrix (HP/12 ground-truth: documentary + public-docs sweep, no live environment 2026-08-26; §5 live-confirm rows)", "the Codex chain this mirrors: HP/01 ground-truth, HP/03 ruling, HP/04 binding+lint, HP/06 packaging — Cursor is the same shape, lighter", "the harness-target ruling (the non-negotiable floor C: isolation/evidence/review-gates never degrade)", "cursor.com/docs {cli/headless, context/rules, skills, hooks} + 2026 third-party write-ups (all dated in the research doc's Appendix A)"]
consumers: ["plugins/assay/references/cursor.md: fixed-here (new binding, capability→mechanism + per-skill degradation)", "tools/harnessgen: fixed-here (new `cursor` verb + tests; readPackaging gains a marker param, codex callsite updated)", "plugins/assay/cursor/{packaging.md,assay.mdc}: fixed-here (coverage roster + generated .mdc rule)", "plugins/assay/skills/adopt/SKILL.md: fixed-here (Cursor install scenario, section 2c)", "plugins/assay/codex/AGENTS-assay.md: out-of-scope (reused unchanged — Cursor reads AGENTS.md natively, the shared fragment)", "docs/how-assay-works.md: fixed-here (Cursor column added to the capability table)", "docs/adopting-assay.md: out-of-scope (Cursor quickstart, sibling change)", "freshness.yaml: fixed-here (binding + research doc rot on a 45-day clock)"]
exec-tier: strong
exec-tier-why: >-
  (b): correctness is cross-artifact — the generated .mdc vs the resident source, the
  coverage roster vs the skills tree, the binding cells vs the packaged set, and the
  neutrality lint over the bodies must all agree; the failure mode is a skew no
  single-file test sees.
version: 1
id: 4c63c55e-ceac-42ba-a7f8-72e27fde4083
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
| 2 | `(cd tools/harnessgen && GOWORK=off go run . cursor --check --root ../..); echo $?` | `0` — committed `.mdc` matches the single resident source (`plugins/assay/resident-rules.md` (planned)) + coverage + binding all clean |
| 3 | **Mutation — drift detected**: `printf '\nX\n' >> "$PWD/plugins/assay/cursor/assay.mdc" && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check >/tmp/hp12r3.out 2>&1; echo $?; (cd tools/harnessgen && GOWORK=off go run . cursor --root ../..)` | `1` naming `assay.mdc`; the `.mdc` is untracked so the revert is a regenerate (`harnessgen cursor`), after which `--check` passes again |
| 4 | **Mutation — coverage closed**: `mkdir -p /tmp/hp12t && cp -r plugins/assay /tmp/hp12t/ && mkdir /tmp/hp12t/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp12t/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check --bundle /tmp/hp12t/assay >/tmp/hp12r4.out 2>&1; echo $?; rm -rf /tmp/hp12t` | exit `2`, output names `probe-skill` — an unaccounted skill is a hard error |
| 5 | **Mutation — binding consistency**: `mkdir -p /tmp/hp12b && cp -r plugins/assay /tmp/hp12b/ && sed 's/`the-desk`/the-desk/g' plugins/assay/references/cursor.md > /tmp/hp12b/assay/references/cursor.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check --bundle /tmp/hp12b/assay >/tmp/hp12r5.out 2>&1; echo $?; rm -rf /tmp/hp12b` | exit `2` naming `the-desk` — packaging↔binding skew is a build error |
| 6 | Neutrality holds: `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills && /tmp/hl870 bindings plugins/assay/references; echo $?` | `0` — adopt's Cursor section stays neutral; `cursor.md` resolves every capability + has a cell per skill |
| 7 | Neighbours unbroken: `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..) && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | `0` — the `resident` and `codex` verbs still pass beside the new one |
| 8 | Adopt path present: `grep -qi 'Running Assay on Cursor' docs/adopting-assay.md && grep -qF 'plugins/assay/cursor/' docs/adopting-assay.md; echo $?` | `0` — the Cursor install scenario + the generated-rule step both present. (Retarget: the adopter-facing install scenario landed in `docs/adopting-assay.md` — §"Running Assay on Cursor — a second first-class harness" — after `plugins/assay/skills/adopt/SKILL.md` was converted to a thin router; the generated-rule step is the referenced `plugins/assay/cursor/` output, home of the generated `assay.mdc`.) |
| 8a | **Positive control for row 8**: `grep -qF 'plugins/assay/cursor-no-such-token' docs/adopting-assay.md; echo $?` | `1` — the probe reports absence for an absent token |
| 9 | `.mdc` frontmatter: `grep -qF 'alwaysApply: true' plugins/assay/cursor/assay.mdc; echo $?` | `0` — the generated rule carries the `.cursor/rules` always-apply contract |
| 10 | Live-confirm rows flagged, not asserted: `grep -c 'needs: live-install confirmation' docs/research/cursor-harness-capabilities.md` | `≥ 5` — the unrunnable rows are flagged, never greened |
| 11 | New entries fresh: `(cd tools/freshness && GOWORK=off go run . --root ../..) 2>&1 \| grep -E -e 'references/cursor.md' -e 'cursor-harness'` | both new entries report `FRESH` (the tool's whole-repo exit is 1 only from pre-existing unrelated stale artifacts, never from these entries) |

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
### Verify run — 2026-09-11, non-implementer dispatched verifier (opus-4.8[1m]-verifier) — VERDICT: FAIL (held at `implemented`; stale probe, adopter substance present)

Ran the Verify table against public medici-finance/assay merged main `553dc2ae530f00e861a14536af7cc884ab77ccf5` (two-protocol head confirmed), offline in an isolated worktree; the row-3 mutate-then-regenerate cycle restored assay.mdc byte-identical and the worktree was left clean. Non-implementer. tools/harnessgen, tools/harnesslint and tools/freshness are each their own Go module (no repo-root go.mod), so the literal `go run ./tools/…` rows fail go.mod-not-found; the real properties were run module-aware via built binaries (non-blocking command-string note, folded under #870).

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | go test in tools/harnessgen | 0 | ok tools/harnessgen; TestCursor* present (drift, coverage, binding-skew, parse-error, frontmatter) | PASS |
| 2 | harnessgen cursor --check (module-aware) | 0 | clean — plugins/assay/cursor/assay.mdc matches the resident source | PASS |
| 3 | append a line to assay.mdc → --check → regenerate → recheck | 1 → 0 | DRIFT naming assay.mdc; regenerate wrote assay.mdc; recheck clean; tree clean | PASS |
| 4 | built binary + planted undeclared skill | 2 | could-not-check: coverage rule failed — skill "probe-skill" in neither the packaged roster nor the excluded list | PASS |
| 5 | built binary + stripped a degradation cell | 2 | could-not-check: packaging↔binding skew — packaged skill "the-desk" has no degradation cell in references/cursor.md | PASS |
| 6 | harnesslint bodies + bindings (module-aware) | 0 | checked-clean: bodies no violations; bindings no violations | PASS |
| 7 | harnessgen resident --check AND codex --check (module-aware) | 0 | both clean — the neighbour verbs still pass beside cursor | PASS |
| 8 | grep cursor AND cursor/assay.mdc in plugins/assay/skills/adopt/SKILL.md | 1 | zero `cursor` occurrences, no `cursor/assay.mdc` — the file is a 58-line thin router deferring to docs/adopting-assay.md | FAIL |
| 8a | positive control — grep an absent token | 1 | absent token reports absence (the probe works; the file simply lacks cursor content) | PASS |
| 9 | grep alwaysApply: true in plugins/assay/cursor/assay.mdc | 0 | present at assay.mdc line 3; generator writes it in tools/harnessgen | PASS |
| 10 | count live-install-confirmation flags in docs/research/cursor-harness-capabilities.md | 0 | count 9 (>= 5) — the unrunnable rows are flagged, not asserted | PASS |
| 11 | freshness for the two new cursor entries (module-aware) | — | FRESH docs/research/cursor-harness-capabilities.md; FRESH plugins/assay/references/cursor.md (whole-repo exit 1 only from an unrelated pre-existing STALE artifact) | PASS |

**Why FAIL — a stale probe, not a Change Failure.** Row 8's specified probe targets `plugins/assay/skills/adopt/SKILL.md` for the Cursor install scenario (this brief's `consumers` frontmatter marks it `fixed-here`, "section 2c"). That file is now a thin router that defers to `docs/adopting-assay.md`, and the adopter-facing Cursor substance DID land — a full "Running Assay on Cursor — a second first-class harness" section in `docs/adopting-assay.md` (referencing `plugins/assay/references/cursor.md`) plus the Cursor column in `docs/how-assay-works.md`. So the deliverable INTENT (an adopter can install Assay on Cursor) is satisfied; only the brief's own probe against the SKILL.md is stale after the adopt-skill was converted to a router. This is distinct from harness-portability/06's row 7, where the AGENTS-assay Codex resident-rules step is genuinely absent adopter-facing (a real content gap, #872). Folded under #870 (re-home Verify-row staleness family) with the retarget fix: point row 8 at `docs/adopting-assay.md` / confirm the router design. Brief stays at `implemented`; re-run row 8 after the retarget (or after a section-2c amendment to adopt/SKILL.md, if that placement is still intended — a spec call).

**Risk-bearing value:** `RISK-VALUE: DERIVED — the fail-closed three-state exit gate (0 clean / 1 drift / 2 could-not-check) in tools/harnessgen is the top risk-bearing literal; a coverage or binding skew must hard-error, never silently pass. Re-derived live: exit 2 on the coverage mutation (row 4, naming probe-skill) and the binding-skew mutation (row 5, naming the-desk) against the built binary (exit 2 is observable only on a built binary; go run collapses 2→1); clean=0 (rows 2/7), drift=1 (row 3). Secondary DERIVED: alwaysApply: true at plugins/assay/cursor/assay.mdc line 3; live-confirm count 9 >= 5.`
### Non-implementer verifier re-run — VERIFY: FAIL (row 8 stale spec pointer, already tracked) — sonnet-5-verifier (verify-desk dispatch), @ merged main `5fbf75834e1d2e5a80b44524649b4030f50e80f1`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted. Second independent verify pass (prior: 2026-09-11).

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/harnessgen && go test ./...` | exit 0 | exit 0, all named TestCursor* subtests pass | 2026-09-18 | sonnet-5-verifier |
| 2 | `harnessgen cursor --check` | exit 0 | exit 0, clean — matches resident source | 2026-09-18 | sonnet-5-verifier |
| 3 | mutation: append line, check, regenerate, recheck | 1 then 0 | checked-failed then checked-clean, working tree clean after | 2026-09-18 | sonnet-5-verifier |
| 4 | mutation: plant undeclared skill | exit 2 naming it | checked-failed (could-not-check) as expected | 2026-09-18 | sonnet-5-verifier |
| 5 | mutation: strip a degradation cell | exit 2 naming it | checked-failed as expected | 2026-09-18 | sonnet-5-verifier |
| 6 | `harnesslint bodies` + `bindings` | exit 0 | both checked-clean, 3 non-matrix skips correctly excluded | 2026-09-18 | sonnet-5-verifier |
| 7 | `harnessgen resident --check` + `codex --check` | exit 0 | both checked-clean | 2026-09-18 | sonnet-5-verifier |
| 8 | grep Cursor mentions + cursor/assay.mdc reference in adopt/SKILL.md | exit 0 | **FAIL** — zero cursor occurrences in the 58-line file | 2026-09-18 | sonnet-5-verifier |
| 8a | control: grep an absent token | exit 1 | correctly absent | 2026-09-18 | sonnet-5-verifier |
| 9 | `grep -qF 'alwaysApply: true' plugins/assay/cursor/assay.mdc` | exit 0 | present at line 3 | 2026-09-18 | sonnet-5-verifier |
| 10 | live-install-confirmation flag count in cursor-harness-capabilities.md | ≥5 | 9 | 2026-09-18 | sonnet-5-verifier |
| 11 | freshness tool run, cursor entries | fresh | both entries FRESH, reviewed 2026-08-26, within 45d window | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all rows map 1:1 to Verify rows; no invented scope.

**Root-cause confirmation, not re-derived from scratch.** Rows 1,2,3,6,7,11's module-aware command forms already reflect the fix from PR #878 (merged 2026-09-11, closing #870's go.mod-scoping half) — confirmed via `gh pr diff 878`. Row 8 is unchanged since the 2026-09-11 pass: `plugins/assay/skills/adopt/SKILL.md` is a thin 58-line router deferring to `docs/adopting-assay.md`, which DOES carry the real Cursor install-scenario content (confirmed at line 1232: "Running Assay on Cursor — a second first-class harness"). This exact row-8 staleness for hp/12 is named in #870's comment thread (distinct from #872, the genuine hp/06 content gap) — the row-8/consumers-frontmatter retarget was flagged but not yet executed as a brief edit. Already tracked, no new issue filed.

RISK-VALUE: DERIVED — exitClean=0, exitDrift=1, exitCouldNotCheck=2 @ tools/harnessgen/main.go:22-24 — all four values re-derived live this pass (rows 3,4,5).
RISK-VALUE: DERIVED — `alwaysApply: true` @ plugins/assay/cursor/assay.mdc:3 — confirmed present.
RISK-VALUE: DERIVED — live-confirm flag count = 9 (≥5 threshold) @ docs/research/cursor-harness-capabilities.md — confirmed via grep.

VERIFY: FAIL — held at implemented. Row 8 fails on a spec-pointer staleness (the brief's own consumers frontmatter and Verify row 8 still target adopt/SKILL.md directly rather than docs/adopting-assay.md's Cursor section, which is where the real content actually landed) — a decision call (retarget the row, or add a pointer into adopt/SKILL.md itself), already tracked at medici-finance/assay#870 (OPEN). Every other row passes clean, including the full mutation battery. No new issue filed.

### Row-8 stale-probe retarget — implementer (worker-desk dispatch), opus-4.8[1m] — 2026-09-22

The desk resolved the decision call the two prior passes flagged: **retarget Verify row 8 (and its
positive control 8a) to the file where the de-house actually landed the adopter-facing Cursor
install scenario** — `docs/adopting-assay.md` (§"Running Assay on Cursor — a second first-class
harness") — rather than the pre-de-house target `plugins/assay/skills/adopt/SKILL.md`, which the
de-house converted to a thin router. This corrects a STALE PROBE only; the underlying deliverable is
sound (an adopter can install Assay on Cursor). No assertion was weakened: the pair still proves
(a) the Cursor install scenario is documented and (b) the generated-rule step is present.

Fail-first / then-pass evidence, run offline (`KUBECONFIG=/dev/null`) against this worktree
(module-aware forms per PR #878):

| # | Command | Exit | Note |
|---|---------|------|------|
| 8 (stale, pre-fix) | `grep -qi 'cursor' plugins/assay/skills/adopt/SKILL.md && grep -qF 'cursor/assay.mdc' plugins/assay/skills/adopt/SKILL.md` | `1` | FAIL — router file carries neither the install scenario nor the `cursor/assay.mdc` token |
| 8 (retargeted) | `grep -qi 'Running Assay on Cursor' docs/adopting-assay.md && grep -qF 'plugins/assay/cursor/' docs/adopting-assay.md` | `0` | PASS — install scenario heading + generated `plugins/assay/cursor/` output both present |
| 8a (retargeted) | `grep -qF 'plugins/assay/cursor-no-such-token' docs/adopting-assay.md` | `1` | PASS — positive control reports absence for an absent token |

Whole-table re-run this pass (all rows for real; row 3 mutate→regenerate→recheck restored
`assay.mdc` byte-identical, tree left clean): rows 1,2,6,7 → exit 0; row 3 → 1 then 0; rows 4,5 →
exit 2 naming `probe-skill` / `the-desk`; row 8 → 0 (retargeted); row 8a → 1; row 9 → 0; row 10 →
9 (≥5); row 11 → both cursor entries FRESH. `assay.mdc` is now a TRACKED file (changed since
authoring), so row 3's revert is equally a regenerate or `git checkout --`; either restores clean.
Note (non-Verify residual, out of this dispatch's scope): the brief's `consumers:` frontmatter still
reads `plugins/assay/skills/adopt/SKILL.md: fixed-here (… section 2c)` and
`docs/adopting-assay.md: out-of-scope`; the actual landing is the reverse. Tracked under the
Verify-row re-home family (#870); not corrected here to avoid frontmatter/lint side-effects beyond
the probe fix. Implementer evidence — "verified" still requires a non-implementer re-run of the
retargeted table.

### Non-implementer verifier run — VERIFY: FAIL — 11/12 pass, 0 could-not-check, 1 fail — 2026-09-23 claude-opus-4-8-verifier

Fresh classification pass against merged main `438dd26a3e959707d2eb4347aa1310d9173777f9`,
offline (`KUBECONFIG=/dev/null`), in a detached worktree cut from `origin/main`. Non-implementer.
Each tool under `tools/` is its own Go module (no repo-root go.mod), so rows use the
module-aware command forms already carried in the table. This brief's Verify table has no
`check:ci`-classed rows. Row 3's mutate -> regenerate cycle restored `assay.mdc` byte-identical
(working tree clean for that file afterward). Runner is the dispatched verifier, not the implementer.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test -v ./...` | 0, TestCursor* present | PASS — cursor-suite-green; exit 0; --- PASS: TestCursorCheckDetectsDrift ⏎ --- PASS: TestCursorBindingSkewCaught ⏎ --- PASS: TestCursorWriteThenCheckClean ⏎ github.com/medici-finance/assay/tools/harnessgen | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `(cd tools/harnessgen && GOWORK=off go run . cursor --check --root ../..); echo $?` | 0 clean | PASS — clean; exit 0; harnessgen cursor --check: clean — ../../plugins/assay/cursor/assay.mdc matches the resident source ⏎ 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `printf '\nX\n' >> "$PWD/plugins/assay/cursor/assay.mdc" && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check; echo "drift=$?"; (cd tools/harnessgen && GOWORK=off go run . cursor --root ../..); /tmp/hg12 cursor --check; echo "recheck=$?"; git diff --exit-code -- plugins/assay/cursor/assay.mdc >/dev/null && echo "restored=byte-identical"` | 1 naming assay.mdc, then 0 | PASS — drift-then-restore; exit 0; harnessgen cursor --check: DRIFT — committed rule plugins/assay/cursor/assay.mdc differs from the resident source ⏎ drift=1 ⏎ wrote ../../plugins/assay/cursor/assay.mdc ⏎ recheck=0 ⏎ restored=byte-identical | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `mkdir -p /tmp/hp12t && cp -r plugins/assay /tmp/hp12t/ && mkdir /tmp/hp12t/assay/skills/probe-skill && printf -- '---\nname: probe-skill\ndescription: probe\n---\n' > /tmp/hp12t/assay/skills/probe-skill/SKILL.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check --bundle /tmp/hp12t/assay >/tmp/hp12r4.out 2>&1; echo "exit=$?"; cat /tmp/hp12r4.out; rm -rf /tmp/hp12t /tmp/hp12r4.out` | exit 2, output names probe-skill | PASS — coverage-cnc; exit 0; exit=2 ⏎ could-not-check: coverage rule failed ⏎ skill "probe-skill" is on disk but appears in neither the packaged roster nor the excluded list | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | mkdir -p /tmp/hp12b && cp -r plugins/assay /tmp/hp12b/ && sed 's/`the-desk`/the-desk/g' plugins/assay/references/cursor.md > /tmp/hp12b/assay/references/cursor.md && GOWORK=off go build -C tools/harnessgen -o /tmp/hg12 . && /tmp/hg12 cursor --check --bundle /tmp/hp12b/assay >/tmp/hp12r5.out 2>&1; echo "exit=$?"; cat /tmp/hp12r5.out; rm -rf /tmp/hp12b /tmp/hp12r5.out | exit 2 naming the-desk | PASS — skew-cnc; exit 0; exit=2 ⏎ could-not-check: packaging↔binding skew ⏎ packaged skill "the-desk" has no degradation cell | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills && /tmp/hl870 bindings plugins/assay/references; echo $?` | 0 | FAIL — bindings-drift; exit 0; checked-clean: bodies — no violations ⏎ plugins/assay/references/claude-code.md: no degradation cell for skill "system-demo" ⏎ checked-failed: bindings — 1 violation(s) | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `(cd tools/harnessgen && GOWORK=off go run . resident --check --root ../..) && (cd tools/harnessgen && GOWORK=off go run . codex --check --root ../..); echo $?` | 0 | PASS — verbs-unbroken; exit 0; harnessgen resident --check: clean — committed artifacts match the source ⏎ harnessgen codex --check: clean — ../../plugins/assay/.codex-plugin/plugin.json matches the metadata source ⏎ 0 | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `grep -qi 'Running Assay on Cursor' docs/adopting-assay.md && grep -qF 'plugins/assay/cursor/' docs/adopting-assay.md; echo "both=$?"; grep -ci 'Running Assay on Cursor' docs/adopting-assay.md; grep -cF 'plugins/assay/cursor/' docs/adopting-assay.md` | 0 | PASS — both-present; exit 0; both=0 ⏎ 2 ⏎ 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 8a | `grep -qF 'plugins/assay/cursor-no-such-token' docs/adopting-assay.md; echo $?` | 1 | PASS — absent-control; exit 0; 1 | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | `grep -qF 'alwaysApply: true' plugins/assay/cursor/assay.mdc; echo "exit=$?"; grep -n 'alwaysApply: true' plugins/assay/cursor/assay.mdc` | 0 | PASS — present-line3; exit 0; exit=0 ⏎ 3:alwaysApply: true | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | `grep -c 'needs: live-install confirmation' docs/research/cursor-harness-capabilities.md` | >= 5 | PASS — flagged-live-rows; exit 0; 9 | 2026-09-23 | claude-opus-4-8-verifier |
| 11 | `(cd tools/freshness && GOWORK=off go run . --root ../..) 2>&1 \| grep -E -e 'references/cursor.md' -e 'cursor-harness'` | both FRESH | PASS — fresh; exit 0; FRESH  docs/research/cursor-harness-capabilities.md  reviewed 2026-08-26, max-age 45d ⏎ FRESH  plugins/assay/references/cursor.md  reviewed 2026-08-26, max-age 45d | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — exitClean = 0, exitDrift = 1, exitCouldNotCheck = 2 @ tools/harnessgen/main.go:22-24 — the three-state fail-closed gate the `cursor` verb reuses. Re-derived live this pass: clean=0 (rows 2, 7), drift=1 (row 3), could-not-check=2 (rows 4, 5). The mapping is correct: could-not-check must be a distinct non-zero, non-drift value so a coverage or binding skew hard-errors rather than reading as clean; drift=1 is the recoverable regenerate-and-commit state. Exit 2 is observable only on a built binary (`go run` collapses 2 into 1), which is why rows 4/5 build the binary first.
RISK-VALUE: DERIVED — alwaysApply = true @ plugins/assay/cursor/assay.mdc:3 — the `.cursor/rules` contract injects the resident rule into every Cursor context only when always-apply is set; `true` is the sole value satisfying the resident-rules always-on posture (the same always-on stance as the Claude payload). Confirmed present (row 9).
RISK-VALUE: N/A for the remaining enumerated entries — the freshness max-age (45 days, `freshness.yaml`) and the row-10 flag threshold (>= 5) are reversible operational knobs, rank last, no derivation required.

Row 6 fails only because the Claude Code bindings reference lacks a system-demo degradation cell (real-tree drift from #1488; noted on #1332); this brief's own deliverable is complete.


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
