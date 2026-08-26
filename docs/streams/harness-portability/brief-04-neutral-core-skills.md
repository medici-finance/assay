---
brief: harness-portability/04
title: Neutral-core skill bodies + per-harness binding files + neutrality lint
why: >-
  The skill bodies are the method, and today they speak Claude: tool names (Agent,
  SendMessage), background-subagent behaviour, worktree mechanics. On any other harness
  those instructions dangle. Rewriting the touchpoints into a closed capability
  vocabulary, bound per harness by one small reference file each, makes the SAME text
  the method on every harness — and the lint makes the neutrality a property CI holds,
  not a convention that erodes with the next edit.
wave: 2
depends: ["harness-portability/02", "harness-portability/03"]
unblocks: ["harness-portability/06"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "measured touchpoints 2026-08-07: backticked Agent/SendMessage in 2 of 7 SKILL.md files; subagent|dispatch|worktree occurrences — batch-fanout 32, verify-desk 18, pr-review-desk 14, the-desk 11, author-brief 3, market-intelligence 1, adopt 0", "superpowers 6.2.0 references/ convention (per-harness binding notes — same skill, one reference file per harness)", "harness-portability/01's capability matrix (the Codex bindings' factual source)", "the harness-target ruling (HP/03 — the degradation cells the codex binding must carry)", "freshness-checked 2026-08-07 (no references/ dir exists under plugins/assay)"]
consumers: ["every Claude Code session loading assay:* skills (this repo, the upstream skills repo, adopters): fixed-here (the neutral text + claude binding must preserve current behaviour; regression rows below)", "the upstream thin-pointer wrappers (post harness-portability/02): unaffected (pointers carry no method text)", "plugins/assay/hooks/inject-resident-rules.sh: out-of-scope (resident rules are harness-portability/05's surface)", "the plugindrift SOURCES coverage: fixed-here (new references/ files declared so coverage stays closed)"]
exec-tier: strong
exec-tier-why: >-
  (b)+(c): a sweeping rewrite across seven prose artifacts where a subtle error — a
  guarantee softened while rephrasing, a degradation left implicit — survives every
  structural test; correctness is cross-artifact (vocabulary closure across bodies and
  both binding files).
---

# Brief 04 — Neutral-core skill bodies, binding files, neutrality lint

## Context

files:
- **amend** `plugins/assay/skills/*/SKILL.md` (7 files) — harness touchpoints rewritten
  to capability vocabulary
- **create** `plugins/assay/references/claude-code.md` — Claude Code bindings
- **create** `plugins/assay/references/codex.md` — Codex bindings + degradation cells
- **create** `tools/harnesslint` — Go module: neutrality + vocabulary-closure lint
- **amend** the `Makefile`, the CI workflows (the lint's CI hook), `go.work`
- **amend** the `plugins/assay` SOURCES coverage roster — declare the new references/ files

facts:
- **The capability vocabulary is CLOSED (stream README)**: `dispatch-worker`,
  `message-agent`, `isolate-workspace`, `invoke-skill`, `session-notifications`.
  Amending the set means amending the stream README in the same PR — the lint reads the
  set from one place.
- **The seam (README, decided)**: bodies name capabilities, never harness tools; each
  `references/<harness>.md` maps capability → mechanism and carries that harness's
  per-skill `runs/degrades/refuses` cells from 03's ruling. Harness tool names are
  LEGAL inside references/ (that is their purpose) and ILLEGAL in skill bodies.
- Banned-token seed list for bodies (extend during implementation, in the lint's
  config, with reasons): `SendMessage`, backticked `Agent`, `Task tool`,
  `CLAUDE_PLUGIN_ROOT`, `claude.ai`, `SessionStart` — plus Codex-only names
  (`spawn_agent`, `wait_agent`, `close_agent`, `AGENTS.md`-as-mechanism) so neutrality
  cuts both ways: the core may name NO harness's tools, not just not-Claude's.
  Plain-prose uses of common words ("agent", "task") are not banned — the lint matches
  the specific token forms, and the fixture suite proves both directions.
- The rewrite is a TOUCHPOINT pass, not a rewording pass: measured surface is ~80
  occurrence sites across 7 files (see sources). Method content, war stories, and
  guarantees are preserved verbatim wherever a harness name is not load-bearing.
- 02 must have landed: the bodies being rewritten are the re-synced canonical text —
  rewriting the stale text would hand the re-sync a wall of conflicts.
- Go module conventions: register in `go.work`; the `make build` sweep covers only six
  modules today — decide and record whether harnesslint joins the sweep, and add the CI
  cross-module registry row if the lint reads outside its module (the cross-module
  registry rule).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **No guarantee softens in rephrasing.** Where a body says "own worktree, NEVER the
  shared checkout", the neutral form keeps the NEVER — `isolate-workspace` names the
  mechanism, not the rule. Diff review hunts exactly this class.
- The lint follows the three-state rule: parse error / unreadable file / empty
  vocabulary → non-zero with `could-not-check`, never a silent pass.

## Task

1. **Build `tools/harnesslint`** first (it then gates your own rewrite):
   - Mode `bodies`: scan `plugins/assay/skills/*/SKILL.md` for banned tokens (config
     file with per-token reason) and for capability names outside the closed set read
     from the stream README. Any hit → non-zero, file:line named.
   - Mode `bindings`: every capability in the closed set resolves in EVERY
     `references/<harness>.md`, and every skill has a degradation cell in each binding
     file. Missing → non-zero.
   - Table-driven tests over fixtures: a dirty body (each banned token class), a body
     with an unknown capability, a binding file missing a capability, a binding file
     missing a skill cell — each must go red individually.
2. **Write the claude-code binding file**: current Claude Code bindings (dispatch-worker
   → the Agent tool + background completion notifications; message-agent → SendMessage;
   isolate-workspace → git worktree recipe; invoke-skill → the Skill mechanism +
   description-driven triggering; session-notifications → task notifications), plus the
   trivial all-`runs` degradation column.
3. **Write the codex binding file** from 01's matrix + 03's ruling: mechanism per
   capability (e.g. dispatch-worker → `spawn_agent`/`wait_agent`/`close_agent` behind
   `multi_agent = true`, if 01 confirms), sandbox constraints, and the ruled
   degradation cell per skill — including the serial-fanout degradation text and the
   isolation refusal text verbatim, so a Codex session states them rather than
   improvising.
4. **Rewrite the seven bodies' touchpoints** to the vocabulary; add each body's single
   pointer line to `references/` ("bindings for your harness: see
   `../../references/<harness>.md`").
5. **Wire CI** (lint on PR paths touching `plugins/assay/**`), update the SOURCES
   coverage roster, and record the `make build` sweep decision.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp04r1.out 2>&1; echo $?` | `0` — includes the per-fixture red tests (task 1) |
| 2 | `go run ./tools/harnesslint bodies plugins/assay/skills > /tmp/hp04r2.out 2>&1; echo $?` | `0` — the shipped bodies are neutral |
| 2a | **Mutation — the lint can fail**: `cp -r plugins/assay/skills /tmp/hp04-dirty && printf '\nUse the \x60Agent\x60 tool with SendMessage.\n' >> /tmp/hp04-dirty/adopt/SKILL.md && go run ./tools/harnesslint bodies /tmp/hp04-dirty > /tmp/hp04r2a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty` | non-zero, output names the adopt skill file and both tokens — the live lint, not just its test suite, goes red on a planted violation |
| 3 | `go run ./tools/harnesslint bindings plugins/assay/references > /tmp/hp04r3.out 2>&1; echo $?` | `0` — vocabulary closure holds in both binding files, every skill has a cell in each |
| 3a | **Mutation**: `cp -r plugins/assay/references /tmp/hp04-dirty-bind && grep -vF 'dispatch-worker' plugins/assay/references/codex.md > /tmp/hp04-dirty-bind/codex.md && go run ./tools/harnesslint bindings /tmp/hp04-dirty-bind > /tmp/hp04r3a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty-bind` | non-zero naming `dispatch-worker` — closure is checked, not assumed |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills > /tmp/hp04r4.out; test ! -s /tmp/hp04r4.out; echo $?` | `0` — spot confirmation independent of the lint's own matcher |
| 4a | **Positive control for row 4** — `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | `>= 1` — same pattern, same engine, finds the token where it legally lives; row 4's empty result therefore means clean, not blind |
| 5 | `for c in dispatch-worker message-agent isolate-workspace invoke-skill session-notifications; do grep -qF "$c" plugins/assay/references/claude-code.md && grep -qF "$c" plugins/assay/references/codex.md \|\| echo "MISSING $c"; done > /tmp/hp04r5.out; test ! -s /tmp/hp04r5.out; echo $?` | `0` (control: the row-2a fixture method — append `no-such-cap` to the loop list and confirm MISSING prints) |
| 6 | **Neighbour row** — `go run ./tools/plugindrift; echo $?` | `0` — the pre-existing `skills/*/SKILL.md` coverage stays closed once the new `references/` files sit alongside it in the tree. This does NOT verify references/ coverage itself: plugindrift's coverage glob is `skills/*/SKILL.md` only, so it never scans `plugins/assay/references/`, and its plain (non-`--fail-on-drift`) exit code is already `0` today regardless of drift — confirmed by running it pre-implementation: exit `0` with 5 BEHIND + 1 UNREACHABLE still present. Row 5 (the capability-loop grep) is what actually proves the references/ files exist and are complete |
| 7 | CI wiring: `grep -rlE 'harnesslint' .github/workflows > /tmp/hp04r7.out; test -s /tmp/hp04r7.out; echo $?` | `0` — the lint has a CI caller; a lint no workflow runs is documentation |
| 8 | **BLOCKED (needs live Claude session, non-CI)** — regression: one full loop cycle (fanout a trivial brief → review → verify) driven from the rewritten skills in a real Claude Code session | Behaviour matches pre-rewrite: dispatch occurs, isolation held, evidence recorded. This is the flow row for the shared value "the method text"; a non-implementer runs it and pastes the session summary |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Rows 2a/3a mutation outputs pasted, not summarised. Row 8 stays BLOCKED until a
     non-implementer runs the live cycle. "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | harnesslint go test (GOFLAGS=-buildvcs=false go test ./... in the module) | 0 | ok — 13 tests incl. per-fixture red tests (each banned-token class, unknown capability, missing capability, missing skill cell, all three states) | 2026-08-16 | assay-worker-app[bot] (implementer) |
| 2 | harnesslint bodies over the shipped skills | 0 | checked-clean: bodies — no violations | 2026-08-16 | assay-worker-app[bot] |
| 2a | mutation — append backticked-Agent + SendMessage to a copy of the adopt skill body, run bodies lint | 1 | two lines at the adopt copy line 64 naming banned token "SendMessage" and banned token backticked-Agent; checked-failed: bodies — 2 violation(s). Names the adopt file and BOTH tokens | 2026-08-16 | assay-worker-app[bot] |
| 3 | harnesslint bindings over the shipped references | 0 | checked-clean: bindings — no violations | 2026-08-16 | assay-worker-app[bot] |
| 3a | mutation — strip dispatch-worker from a copy of the Codex binding, run bindings lint | 1 | codex copy: capability "dispatch-worker" does not resolve — no capability:dispatch-worker binding present; checked-failed: bindings — 1 violation(s). Names dispatch-worker | 2026-08-16 | assay-worker-app[bot] |
| 4 | git grep SendMessage in the skills tree, assert empty | 0 | empty result — no SendMessage in any body | 2026-08-16 | assay-worker-app[bot] |
| 4a | git grep -c SendMessage in the Claude binding (positive control) | 0 | count 2 (>= 1) — the token lives where it legally maps message-agent, so row 4's empty result is clean not blind | 2026-08-16 | assay-worker-app[bot] |
| 5 | capability-loop grep across both binding files, with control | 0 | empty (all five resolve in both); control appending no-such-cap prints MISSING no-such-cap | 2026-08-16 | assay-worker-app[bot] |
| 6 | plugindrift neighbour (go run ./tools/plugindrift) | 0 | coverage 9 bundled skills — 2 pinned, 5 canonical, 2 unported, 0 unaccounted; exit 0 (SKILL-only glob unchanged; references closure held by harnesslint) | 2026-08-16 | assay-worker-app[bot] |
| 7 | grep -rl harnesslint in the workflows dir, assert non-empty | 0 | the tools workflow — live-lint step + matrix leg named test (tools/harnesslint) | 2026-08-16 | assay-worker-app[bot] |
| 8 | one full loop cycle driven from the rewritten skills in a real Claude Code session | BLOCKED | needs a live Claude Code session; a non-implementer runs the flow row and pastes the session summary (dispatch occurs, isolation held, evidence recorded) | — | (awaiting non-implementer) |

### Non-implementer verifier run — VERIFY: PASS (rows 1–7 + mutations; row 8 UNRUN, blocked-live-session) — opus-4.8[1m]-verifier (verify-desk dispatch), independent re-run against merged main, 2026-08-22

Runner ≠ implementer (implementer was assay-worker-app[bot]). Isolated worktree at merged main. All mechanical + mutation rows re-executed fresh; implementer Evidence not trusted. Risk line `{regulatory: no, customer: no, irreversible: no, sensitive-data: no}`, `gate: model`.

| # | Command | Exit | Observed | Date | Runner |
|---|---------|------|----------|------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./...` | 0 | `ok …/tools/harnesslint 0.351s` — 35 RUN entries incl. per-fixture red tests (each banned-token class, unknown/missing capability, missing skill cell, all three states) | 2026-08-22 | opus-4.8[1m]-verifier |
| 2 | `go run ./tools/harnesslint bodies plugins/assay/skills` | 0 | `checked-clean: bodies — no violations` | 2026-08-22 | opus-4.8[1m]-verifier |
| 2a | mutation — append backticked-Agent + SendMessage to a copy of adopt/SKILL.md, run bodies lint | 1 | two lines at `adopt/SKILL.md:64` naming `"SendMessage"` and backticked-Agent; `checked-failed: bodies — 2 violation(s)` — live lint goes red on a planted violation | 2026-08-22 | opus-4.8[1m]-verifier |
| 3 | `go run ./tools/harnesslint bindings plugins/assay/references` | 0 | `checked-clean: bindings — no violations` | 2026-08-22 | opus-4.8[1m]-verifier |
| 3a | mutation — strip `dispatch-worker` from a copy of codex.md, run bindings lint | 1 | `codex.md: capability "dispatch-worker" does not resolve …`; `checked-failed: bindings — 1 violation(s)` | 2026-08-22 | opus-4.8[1m]-verifier |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills; test ! -s` | 0 | empty result — no SendMessage in any body | 2026-08-22 | opus-4.8[1m]-verifier |
| 4a | `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | 0 | count 2 (≥1); row-4 empty is clean, not blind | 2026-08-22 | opus-4.8[1m]-verifier |
| 5 | capability-loop grep across both binding files; `test ! -s` | 0 | empty (all five resolve in both); control appending `no-such-cap` prints `MISSING no-such-cap` | 2026-08-22 | opus-4.8[1m]-verifier |
| 6 | `go run ./tools/plugindrift` | 0 | `coverage: 9 bundled skills/*/SKILL.md … 0 unaccounted`; exit 0 (prints BEHIND 3 but plain exit is 0, as the brief documents) | 2026-08-22 | opus-4.8[1m]-verifier |
| 7 | `grep -rlE 'harnesslint' .github/workflows; test -s` | 0 | the tools workflow — matrix leg + live "Neutrality + vocabulary-closure lint" step running `harnesslint bodies`/`bindings` | 2026-08-22 | opus-4.8[1m]-verifier |
| 8 | one full loop cycle (fanout→review→verify) from the rewritten skills in a live session | UNRUN | blocked-live-session: not runnable by a dispatched (non-interactive) verifier; routed to a named follow-up (the live-verify pattern) for a live-session run | — | routed → follow-up |

**RISK-VALUE: DERIVED — exit codes = 0/1/2 in the harnesslint entrypoint** — matches the three-state instrument invariant (checked-clean=0, checked-failed=1, could-not-check=2 distinct); the report path and the empty-vocab/empty-banned guards route to could-not-check (2) not a silent 0.
**RISK-VALUE: DERIVED — closed capability vocabulary = {dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications} in the stream README's machine-readable block** — matches the brief's decided closed set verbatim; the lint reads it from this single place, so amending requires amending the README in-PR; rows 3/5 confirm all five resolve in both binding files. All enumerated literals are reversible CI/prose knobs (risk all `no`).

**VERIFY: PASS** — every runnable row (1, 2, 2a, 3, 3a, 4, 4a, 5, 6, 7) passed independently against merged main; row 8 is a live-session integration flow, unrunnable by a dispatched verifier, recorded UNRUN and routed to a follow-up (not assumed-pass).

Regression note (Ground rule "no guarantee softens"): the four rewritten bodies keep every
guarantee verbatim — pr-review-desk READ-ONLY + "never the shared checkout"; verify-desk
"ALWAYS dispatch, NEVER verify inline" + the VERBATIM home-worktree line + "NEVER the shared
checkout"; worker-desk "NEVER the shared checkout" + the full --detach / refs/remotes/origin/main
recipe; the-desk "never by an empty output file". Only harness tool-names (the backticked Agent
tool, SendMessage, the background/completion-notification mechanism words) became capability names;
the rules they carried are unchanged. Model gate: a non-implementer still owns the row-8 flow row
and any verified flip.

## Review

Gate: **model** (from frontmatter). Review priority: the diff of the seven bodies,
hunting the softened-guarantee class (a NEVER weakened, a refusal turned best-effort, a
degradation left implicit) — the structural rows cannot catch it; a reviewer reading
the 3-dot diff can.
