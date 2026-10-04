---
brief: assay:assay:harness-portability:04
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
schema: brief-v2
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07)", "measured touchpoints 2026-08-07: backticked Agent/SendMessage in 2 of 7 SKILL.md files; subagent|dispatch|worktree occurrences — batch-fanout 32, verify-desk 18, pr-review-desk 14, the-desk 11, author-brief 3, market-intelligence 1, adopt 0", "superpowers 6.2.0 references/ convention (per-harness binding notes — same skill, one reference file per harness)", "harness-portability/01's capability matrix (the Codex bindings' factual source)", "the harness-target ruling (HP/03 — the degradation cells the codex binding must carry)", "freshness-checked 2026-08-07 (no references/ dir exists under plugins/assay)"]
consumers: ["every Claude Code session loading assay:* skills (this repo, the upstream skills repo, adopters): fixed-here (the neutral text + claude binding must preserve current behaviour; regression rows below)", "the upstream thin-pointer wrappers (post harness-portability/02): unaffected (pointers carry no method text)", "plugins/assay/hooks/inject-resident-rules.sh: out-of-scope (resident rules are harness-portability/05's surface)", "the plugindrift SOURCES coverage: fixed-here (new references/ files declared so coverage stays closed)"]
exec-tier: strong
exec-tier-why: >-
  (b)+(c): a sweeping rewrite across seven prose artifacts where a subtle error — a
  guarantee softened while rephrasing, a degradation left implicit — survives every
  structural test; correctness is cross-artifact (vocabulary closure across bodies and
  both binding files).
version: 1
id: 657026c6-eb54-49ef-bd9e-0380ed3ac161
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
| 2 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills > /tmp/hp04r2.out 2>&1; echo $?` | `0` — the shipped bodies are neutral |
| 2a | **Mutation — the lint can fail**: `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/skills /tmp/hp04-dirty && printf '\nUse the \x60Agent\x60 tool with SendMessage.\n' >> /tmp/hp04-dirty/adopt/SKILL.md && /tmp/hl870 bodies /tmp/hp04-dirty > /tmp/hp04r2a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty` | non-zero, output names the adopt skill file and both tokens — the live lint, not just its test suite, goes red on a planted violation |
| 3 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bindings plugins/assay/references > /tmp/hp04r3.out 2>&1; echo $?` | `0` — vocabulary closure holds in both binding files, every skill has a cell in each |
| 3a | **Mutation**: `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/references /tmp/hp04-dirty-bind && grep -vF 'dispatch-worker' plugins/assay/references/codex.md > /tmp/hp04-dirty-bind/codex.md && /tmp/hl870 bindings /tmp/hp04-dirty-bind > /tmp/hp04r3a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty-bind` | non-zero naming `dispatch-worker` — closure is checked, not assumed |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills > /tmp/hp04r4.out; test ! -s /tmp/hp04r4.out; echo $?` | `0` — spot confirmation independent of the lint's own matcher |
| 4a | **Positive control for row 4** — `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | `>= 1` — same pattern, same engine, finds the token where it legally lives; row 4's empty result therefore means clean, not blind |
| 5 | `for c in dispatch-worker message-agent isolate-workspace invoke-skill session-notifications; do grep -qF "$c" plugins/assay/references/claude-code.md && grep -qF "$c" plugins/assay/references/codex.md \|\| echo "MISSING $c"; done > /tmp/hp04r5.out; test ! -s /tmp/hp04r5.out; echo $?` | `0` (control: the row-2a fixture method — append `no-such-cap` to the loop list and confirm MISSING prints) |
| 6 | **Neighbour row** — `(cd tools/plugindrift && GOWORK=off go run . --root ../..); echo $?` | `0` — the pre-existing `skills/*/SKILL.md` coverage stays closed once the new `references/` files sit alongside it in the tree. This does NOT verify references/ coverage itself: plugindrift's coverage glob is `skills/*/SKILL.md` only, so it never scans `plugins/assay/references/`, and its plain (non-`--fail-on-drift`) exit code is already `0` today regardless of drift — confirmed by running it pre-implementation: exit `0` with 5 BEHIND + 1 UNREACHABLE still present. Row 5 (the capability-loop grep) is what actually proves the references/ files exist and are complete |
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
### Non-implementer verifier run — 2026-09-16 sonnet-5-verifier (verify-desk dispatch), SECOND verify pass — **VERIFY: FAIL**

Runner ≠ implementer ≠ the 2026-08-22 verifier above. Own isolated temp worktree off fetched
`origin/main` (`0bf1166a`), `KUBECONFIG=/dev/null`. Re-executed every row fresh rather than
trusting the 2026-08-22 block — 3+ weeks of unrelated commits have landed on this tree since,
and two of the literal Verify commands now fail for reasons outside this brief's own diff.

| # | Command | Exit | Observed | Date | Runner |
|---|---------|------|----------|------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./...` | 0 | `ok github.com/medici-finance/assay/tools/harnesslint 0.386s` — `go test -v` shows 21 `--- PASS` / 0 `--- FAIL`, incl. the per-fixture red tests | 2026-09-16 | sonnet-5-verifier |
| 2 | `go build -C tools/harnesslint -o hl870 . && hl870 bodies plugins/assay/skills` | 0 | `checked-clean: bodies — no violations` (the #418 ask-decision/install token findings are no longer present in the shipped bodies) | 2026-09-16 | sonnet-5-verifier |
| 2a | mutation — append backticked-Agent + SendMessage to a copy of adopt/SKILL.md, run bodies lint | 1 | two lines at the adopt copy line 60 naming banned token `"SendMessage"` and banned token backticked-`` `Agent` ``; `checked-failed: bodies — 2 violation(s)` | 2026-09-16 | sonnet-5-verifier |
| 3 | `hl870 bindings plugins/assay/references` | **1** | **FAIL — 20 violation(s).** `desk-shell.md` and `tick-contract.md` are correctly skipped (declared `non-matrix-reference`), but `plugins/assay/references/standing-note.md` — added 2026-09-14, commit `4605ed6b` (#1026), **after** this row last measured clean (issue #393's 2026-09-04 drain) — carries no such declaration and no capability bindings at all: 7/7 capabilities unresolved + 13 skills missing a degradation cell against it. `claude-code.md`/`codex.md`/`cursor.md` themselves stay clean (this is not the `human-runsheet` drift already tracked in #970). Root-caused: **not this brief's own diff** — filed as new issue medici-finance/assay#1182 | 2026-09-16 | sonnet-5-verifier |
| 3a | mutation — strip `dispatch-worker` from a copy of codex.md, run bindings lint | 1 | `/tmp/.../codex.md: capability "dispatch-worker" does not resolve — no ``capability:dispatch-worker`` binding present`; `checked-failed: bindings — 8 violation(s)` (extra 7 are the pre-existing standing-note.md violations carried in the same copied tree) — closure IS checked, not assumed | 2026-09-16 | sonnet-5-verifier |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills; test ! -s` | 0 | empty result — no `SendMessage` in any shipped body | 2026-09-16 | sonnet-5-verifier |
| 4a | `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | 0 | count 1 (≥1) — row 4's empty result is clean, not blind | 2026-09-16 | sonnet-5-verifier |
| 5 | capability-loop grep (5-capability set) across `claude-code.md`+`codex.md`; `test ! -s` | 0 | empty — all 5 resolve in both; control appending `no-such-cap` to the loop list prints `MISSING no-such-cap` | 2026-09-16 | sonnet-5-verifier |
| 6 | `cd tools/plugindrift && go run . --root ../..` | 0 | `plugindrift: plugins/assay v0.3.0 — 13 file(s) …`; `coverage: 13 bundled skills/*/SKILL.md — 0 pinned, 6 canonical, 7 unported, 0 unaccounted`; `PLUGINDRIFT: CLEAN` — the `skills/*/SKILL.md`-only coverage glob stays closed with `references/` sitting alongside it, as the brief documents | 2026-09-16 | sonnet-5-verifier |
| 7 | `grep -rlE 'harnesslint' .github/workflows; test -s` | **1** | **FAIL — no match.** `git log --all -p -- .github/workflows/*.yml \| grep harnesslint` returns nothing at any point in this repo's history; the CI wiring exists only as an unapplied `tools/harnesslint/ci.yml.patch` (verifier Apps/worker Apps lack the `workflows` scope to push it — same class as closed issues #740/#209). This exact gap is already the tracked subject of the declared follow-up **harness-portability/15** ("Public CI wiring + harnesslint clean-up"), itself still `implemented` (not verified/done); its own 2026-09-12 non-implementer verify pass documents the identical unapplied-patch finding for its own rows 1/3. Not filing a duplicate issue — routing to hp/15 | 2026-09-16 | sonnet-5-verifier |
| 8 | one full loop cycle (fanout→review→verify) from the rewritten skills in a live session | UNRUN | blocked-live-session: not runnable by a dispatched (non-interactive) verifier — same as the 2026-08-22 pass; still routed to a named live-session follow-up | — | routed → follow-up |

**RISK-VALUE: ENUMERATE, then rank, then derive.** Literal constants/bounds the brief's own diff introduces (re-derived by inspection of the shipped `tools/harnesslint` + the stream README's machine-readable block, not carried over from a prior pass):

| Rank | Identifier = literal | Location | Irreversibility |
|---|---|---|---|
| 1 | `exitClean = 0`, `exitFailed = 1`, `exitCannot = 2`, `exitUsage = 2` | `tools/harnesslint/main.go:36-39` | Highest — this is the three-state contract (`checked-clean`/`checked-failed`/`could-not-check`) any CI caller (present or, per row 7, future) gates on; `exitUsage` deliberately collapsing onto `exitCannot`'s value rather than a fourth code is a design choice that keeps the *reported states* three, matching the house's C4 three-state doctrine exactly — a silent 0 on a parse error or empty vocab would be the failure mode this collapsing exists to prevent (confirmed at `lint.go`'s empty-vocab/empty-banned guard paths). Changing any of these three values is a breaking change to every future CI caller's `$?` check. |
| 2 | closed capability vocabulary — 7 entries (`dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications, durable-monitor, stop-worker`) | `docs/streams/harness-portability/README.md:378-384` (`<!-- assay:capability-vocabulary -->` block) | Lower — the brief's own facts explicitly design this as an in-PR-amendable set ("amending the set means amending the stream README in the same PR"); it has already grown from the 5 entries brief-04 itself introduced to 7 (durable-monitor, stop-worker added by a later brief), which is the intended mechanism working, not drift. Confirmed derivable: rows 3/3a prove closure is actually enforced against whatever the README currently declares, not a hardcoded copy in the tool. |

Both literals are reversible CI/prose knobs (frontmatter `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}` — confirmed accurate; nothing above binds money, auth, or an external contract).

**VERIFY: FAIL** — rows 3 and 7 fail the literal command as written against merged main `0bf1166a`. Both are root-caused to work OUTSIDE this brief's own diff (row 3: a 2026-09-14 reference-file addition from a different stream, medici-finance/assay#1182 filed; row 7: the already-tracked hp/15 CI-wiring follow-up, still itself `implemented`). Brief-04's own deliverable — the seven rewritten skill bodies, the `claude-code.md`/`codex.md` binding files, and the harnesslint tool's own logic — verifies clean on every row that exercises it directly (1, 2, 2a, 3a's closure-detection, 4, 4a, 5, 6). Per this brief's Verify table being literal and unscoped for rows 3/7 (unlike row 6, which carries an explicit scoping caveat), the honest verdict on the table as written is FAIL, not a silent PASS that papers over live drift. Status stays `implemented` — no README flip.
### Non-implementer verifier re-run — VERIFY: FAIL (row 7 pre-existing gap, already tracked; row 3 newly confirmed fixed) — sonnet-5-verifier (verify-desk dispatch), @ merged main `5fbf75834e1d2e5a80b44524649b4030f50e80f1`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted. Third independent verify pass (prior: 2026-08-22, 2026-09-16).

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `cd tools/harnesslint && go test ./... -v` | exit 0 | exit 0, 21 PASS incl. red-fixture subtests | 2026-09-18 | sonnet-5-verifier |
| 2 | `hl bodies plugins/assay/skills` | exit 0 | checked-clean, no violations | 2026-09-18 | sonnet-5-verifier |
| 2a | mutation: inject banned harness tokens into a copy | exit 1 | checked-failed as expected, both tokens named | 2026-09-18 | sonnet-5-verifier |
| 3 | `hl bindings plugins/assay/references` | exit 0 | **checked-clean this pass** — 3 non-matrix files correctly skipped (desk-shell.md, standing-note.md, tick-contract.md). Previously failed on standing-note.md (20 violations, filed #1182); fixed by merged PR #1293 (2026-09-17) which added the non-matrix declaration. Confirmed the fix landed — recommending #1182 be closed | 2026-09-18 | sonnet-5-verifier |
| 3a | mutation: strip a capability binding from a copy | exit 1 | checked-failed as expected, missing capability named, clean isolation | 2026-09-18 | sonnet-5-verifier |
| 4 | grep for SendMessage in skill bodies | exit 0, empty | exit 0, empty | 2026-09-18 | sonnet-5-verifier |
| 4a | control: same grep on claude-code.md | count ≥1 | count 2, confirms row 4 isn't blind | 2026-09-18 | sonnet-5-verifier |
| 5 | 5-capability loop grep + control | exit 0, empty; control fires | exit 0, empty; control MISSING fires correctly | 2026-09-18 | sonnet-5-verifier |
| 6 | plugindrift run | exit 0 | PLUGINDRIFT: CLEAN, 0 pinned/6 canonical/7 unported/0 unaccounted | 2026-09-18 | sonnet-5-verifier |
| 7 | grep for harnesslint wiring in .github/workflows | exit 0 (present) | **FAIL — absent.** Never wired, confirmed via full workflow history (0 hits ever). Root-caused to unrelated follow-up harness-portability/15 (still implemented, not done) — App tokens lack workflows scope | 2026-09-18 | sonnet-5-verifier |
| 8 | live-session loop-cycle regression | n/a | UNRUN — not runnable by a dispatched non-interactive verifier, same as both prior passes | 2026-09-18 | sonnet-5-verifier |

Scope traceability: all rows map 1:1 to Verify rows; no invented scope.

RISK-VALUE: DERIVED — exitClean=0, exitFailed=1, exitCannot=2, exitUsage=2 @ tools/harnesslint/lint.go:36-39 — matches the three-state instrument invariant exactly; all three states exercised live this pass (rows 1, 2a, 3a).
RISK-VALUE: DERIVED — 7-entry closed capability vocabulary @ docs/streams/harness-portability/README.md:377-385, scoped to this pass's cited base `5fbf75834e1d2e5a80b44524649b4030f50e80f1` — confirmed the lint reads and enforces this exact live set (rows 3/3a/5), not a stale copy. (Reviewer note 2026-09-19: the merge-target main advanced past this base one commit later, PR #1318, adding an 8th entry `cadence-tick` at lines 380-389; that growth is outside this row's own diff and does not change the pass/fail verdict on rows 3/3a/5, which are independently reproduced clean against current main in the reviewer's own re-run.)

VERIFY: FAIL — held at implemented. Row 7 fails, root-caused to the still-unlanded harness-portability/15 (CI wiring, App-token scope constraint) — not a regression in this brief's own diff, already tracked. Every row exercising this brief's own deliverable directly (1,2,2a,3,3a,4,4a,5,6) passes clean, including row 3 which newly confirms a prior regression (#1182) is fixed on merged main. Housekeeping: closed #1182 (fix landed via #1293, never auto-closed since the PR used "Issue:" not "Closes:").

### Non-implementer verifier re-run — VERIFY: FAIL (rows 3 and 7) — claude-opus-5-5 verifier (verify-desk dispatch), @ merged main `cf56ddeebc187e7923c4c6349bbdf95291bdfe05`, 2026-09-27

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp04r1.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 2 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills > /tmp/hp04r2.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 2a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/skills /tmp/hp04-dirty && printf '\nUse the \x60Agent\x60 tool with SendMessage.\n' >> /tmp/hp04-dirty/adopt/SKILL.md && /tmp/hl870 bodies /tmp/hp04-dirty > /tmp/hp04r2a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 3 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bindings plugins/assay/references > /tmp/hp04r3.out 2>&1; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 3a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/references /tmp/hp04-dirty-bind && grep -vF 'dispatch-worker' plugins/assay/references/codex.md > /tmp/hp04-dirty-bind/codex.md && /tmp/hl870 bindings /tmp/hp04-dirty-bind > /tmp/hp04r3a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty-bind` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills > /tmp/hp04r4.out; test ! -s /tmp/hp04r4.out; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 4a | `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | pass exit=0 | sha256:979226bec361 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 5 | `for c in dispatch-worker message-agent isolate-workspace invoke-skill session-notifications; do grep -qF "$c" plugins/assay/references/claude-code.md && grep -qF "$c" plugins/assay/references/codex.md \|\| echo "MISSING $c"; done > /tmp/hp04r5.out; test ! -s /tmp/hp04r5.out; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd tools/plugindrift && GOWORK=off go run . --root ../..); echo $?` | pass exit=0 | sha256:3e2492290367 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -rlE 'harnesslint' .github/workflows > /tmp/hp04r7.out; test -s /tmp/hp04r7.out; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |
| 8 | `**BLOCKED (needs live Claude session, non-CI)** — regression: one full loop cycle (fanout a trivial brief → review → verify) driven from the rewritten skills in a real Claude Code session` | fail exit=2 | sha256:a5466c565aba | 2026-09-27 | assay-verifier-app[bot] @ cf56ddeebc18 (on-behalf-of human:ian) (forge-identity) |

**Read the witness Result column with care.** Every Verify command except 4a and 8 ends in
`; echo $?`, so the subshell exit is always 0 and the witness records `pass exit=0` whatever
the lint returned. The verdict lives in the printed value, and the output hash identifies it:
sha256:9a271f2a916b is the hash of the one line `0`, sha256:4355a46b19d3 is the hash of the one
line `1` (both recomputed locally). Against each row's Expect cell:

- Row 1: printed `0`; suite `ok`, 48 `--- PASS` / 0 `--- FAIL` under `go test -v`, per-fixture red tests included. PASS.
- Row 2: printed `0`; `checked-clean: bodies — no violations`. PASS.
- Row 2a: printed `1` (expected non-zero); two violations at the adopt copy line 122 naming banned token "SendMessage" and banned token backticked-Agent, then `checked-failed: bodies — 2 violation(s)`. PASS.
- Row 3: printed `1`, expected `0`. **FAIL.** Four files skipped by declaration (desk-common, desk-shell, standing-note, tick-contract), then `plugins/assay/references/claude-code.md: no degradation cell for skill "system-demo" (its system-demo row is missing)` and `checked-failed: bindings — 1 violation(s)`. The system-demo skill landed on 2026-09-22 (#1465); the follow-up #1488 added its cell to the Codex and Cursor binding files but not to the Claude Code one. That is drift from a later stream, not a defect in this brief's own diff, and it is already tracked as medici-finance/assay#1703.
- Row 3a: printed `1` (expected non-zero); `codex.md: capability "dispatch-worker" does not resolve — no capability:dispatch-worker binding present`, `checked-failed: bindings — 1 violation(s)`. PASS. The row-3 system-demo gap does not show here because the copied references directory has no sibling skills roster, so only closure is checked. The lint's own doc comment describes that behaviour.
- Row 4: printed `0`; no SendMessage in any body. PASS.
- Row 4a: exit 0, `plugins/assay/references/claude-code.md:2` (count 2, at least 1). PASS, so row 4's empty result means the tree is clean, not that the grep missed.
- Row 5: printed `0`; all five capabilities resolve in both binding files. Control run with `no-such-cap` appended to the loop printed `MISSING no-such-cap`. PASS.
- Row 6: printed `0`; `coverage: 14 bundled skills/*/SKILL.md — 0 pinned, 6 canonical, 8 unported, 0 unaccounted`, `PLUGINDRIFT: CLEAN`. PASS.
- Row 7: printed `1`, expected `0`. **FAIL.** No workflow under .github/workflows mentions harnesslint. The wiring still exists only as the unapplied tools/harnesslint/ci.yml.patch. Tracked as medici-finance/assay#1332 (harness-portability/15's patch was never pushed and needs a human push, because the worker App lacks workflows scope) and #1703.
- Row 8: this row describes a live Claude Code loop cycle and has no executable command, so verifyrun's `fail exit=2` is a shell parse failure on the prose. It was not run: a dispatched, non-interactive verifier cannot drive a live harness session, and doing so falls outside the offline envelope. No non-implementer live-session record exists in this brief's Evidence. It remains BLOCKED on a human or live-session run.

Public-tree check: every deliverable this brief names is present at merged main in the public
tree. That covers tools/harnesslint (go.mod, lint.go, main.go, lint_test.go, testdata, banned-tokens.md),
plugins/assay/references/claude-code.md and codex.md, and the rewritten plugins/assay/skills bodies.
The CI hook is the one piece still missing (row 7). The repo has no go.work. The Makefile carries
no harnesslint target, and this pass did not re-examine the recorded sweep decision.

**Risk-bearing value enumeration.** The item has risk metadata, all `no`, and `gate: model`. It is not irreversible and the diff touches no risk-classed path. Literals enumerated over tools/harnesslint and the stream README's vocabulary block:

| Rank | Identifier = literal | Location | Irreversibility |
|---|---|---|---|
| 1 | exitClean = 0, exitFailed = 1, exitCannot = 2, exitUsage = 2 | tools/harnesslint/lint.go:36-39 | The three-state exit contract any CI caller gates on. A wrong value would silently green a could-not-check. Reversible with an edit and a redeploy. |
| 2 | closed capability vocabulary = {dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications, durable-monitor, stop-worker, cadence-tick} (8 entries) | docs/streams/harness-portability/README.md:410-419 | Designed to be amended in-PR. It has grown from the brief's 5 entries to 8 through later briefs. Reversible. |
| 3 | banned-token list = 19 entries (backticked Agent, SendMessage, Task tool, CLAUDE_PLUGIN_ROOT, claude.ai, SessionStart, spawn_agent, wait_agent, close_agent, resume_agent, send_input, send_message, followup_task, interrupt_agent, multi_agent, backticked Monitor, backticked TaskList, backticked EnterWorktree, persistent: true) | tools/harnesslint/banned-tokens.md:28-48 | A lint config knob, reversible. It is a superset of the brief's seed list. |
| 4 | defaultReadmePath = "docs/streams/harness-portability/README.md"; vocabMarker, bannedMarker, nonMatrixMarker marker strings | tools/harnesslint/lint.go:48,53,56,75 | Lookup-path knobs. An empty read routes to could-not-check (exit 2), not to a silent pass. Reversible. |

RISK-VALUE: DERIVED — exitClean = 0 / exitFailed = 1 / exitCannot = 2 / exitUsage = 2 @ tools/harnesslint/lint.go:36-39 — these codes keep the three reported states distinct (clean, failed, could-not-check), as the brief's ground rule requires ("parse error / unreadable file / empty vocabulary → non-zero with could-not-check, never a silent pass"). Folding usage errors onto 2 keeps the state count at three. This pass exercised both 0 and 1 live (rows 2, 2a, 3, 3a).
RISK-VALUE: DERIVED — capability vocabulary = the 8-entry set @ docs/streams/harness-portability/README.md:410-419 — the brief's decided seam puts the closed set in one place in the stream README, and the lint reads that block (vocabMarker) rather than keeping a copy. Rows 3a and 5 show closure is enforced against the live set. The growth from 5 to 8 entries is the intended amend-in-README mechanism.
The remaining entries, the banned-token list and the lookup paths and markers, are reversible lint-config knobs. They are ranked last and not derived.

VERIFY: FAIL — rows 3 and 7 fail their Expect cells at merged main cf56ddeebc18. Row 3: the Claude Code binding lacks a system-demo degradation cell, drift from a later stream tracked in medici-finance/assay#1703. Row 7: harnesslint is still not wired into any workflow. The patch is unapplied and tracked in #1332 and #1703. Row 8 remains BLOCKED on a live-session run. Every row that exercises this brief's own deliverables directly passes (1, 2, 2a, 3a, 4, 4a, 5, 6). The item stays at implemented.
### Non-implementer verifier re-run — VERIFY: BLOCKED (rows 1–7 pass; row 8 needs a live session) — claude-opus-5-5[1m] verifier (verify-desk dispatch), @ merged main `024c87b01aba8f6c7dd7ccd939e647a9b936be09`, 2026-10-01

**What moved since the last run (2026-09-27 @ cf56ddeebc18, VERIFY: FAIL on rows 3 and 7):** #1817 (4fbd20514) added the missing system-demo degradation cell to the Claude Code binding, which clears row 3. #1813 (f92fbcd9d, harness-portability/15) wired harnesslint into the ci workflow, both as a build-test matrix leg and as a dedicated gating `harnesslint` job over the real tree, which clears row 7. The brief's own Verify table has not changed.

Grounded expectation, written before reading the earlier blocks: rows 1–7 exit/print as expected at current main; row 8 is could-not-check for a dispatched, offline verifier.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp04r1.out 2>&1; echo $?` | `0`, per-fixture red tests included | printed `0`; output `ok github.com/medici-finance/assay/tools/harnesslint 0.621s`. A supplementary `go test -v -count=1 ./...` in the same module showed 21 of 21 top-level tests PASS and none FAIL. The count includes the per-banned-token red test, the unknown-capability red test, the missing-capability red test, the missing-skill-cell red test and the three-state could-not-check tests. Discharges Verify row 1. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills > /tmp/hp04r2.out 2>&1; echo $?` | `0` | printed `0`; output `checked-clean: bodies — no violations` (14 skill bodies). Discharges Verify row 2. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/skills /tmp/hp04-dirty && printf '\nUse the \x60Agent\x60 tool with SendMessage.\n' >> /tmp/hp04-dirty/adopt/SKILL.md && /tmp/hl870 bodies /tmp/hp04-dirty > /tmp/hp04r2a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty` | non-zero, names the adopt file and both tokens | printed `1`. Output has two lines at the adopt copy SKILL.md line 122. One names banned harness token "SendMessage", the other names banned harness token backticked-Agent. The run ends `checked-failed: bodies — 2 violation(s)`. Discharges Verify row 2a. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bindings plugins/assay/references > /tmp/hp04r3.out 2>&1; echo $?` | `0` | printed `0`; four reference files are excluded by their own reasoned non-matrix-reference declaration: desk-common, desk-shell, standing-note and tick-contract. The run ends `checked-clean: bindings — no violations`. All 14 skills, system-demo included, are named in each of claude-code.md, codex.md and cursor.md (claude-code.md line 58 is now `system-demo \| runs`). Discharges Verify row 3. The previous FAIL is cleared. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/references /tmp/hp04-dirty-bind && grep -vF 'dispatch-worker' plugins/assay/references/codex.md > /tmp/hp04-dirty-bind/codex.md && /tmp/hl870 bindings /tmp/hp04-dirty-bind > /tmp/hp04r3a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty-bind` | non-zero naming `dispatch-worker` | printed `1`; output `codex.md: capability "dispatch-worker" does not resolve — no capability:dispatch-worker binding present`, `checked-failed: bindings — 1 violation(s)`. Discharges Verify row 3a. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills > /tmp/hp04r4.out; test ! -s /tmp/hp04r4.out; echo $?` | `0` | printed `0`; the output file is 0 bytes. Discharges Verify row 4. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4a | `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | `>= 1` | exit 0; output `plugins/assay/references/claude-code.md:2` (count 2). The positive control holds, so row 4's empty result means the tree is clean, not that the grep missed. Discharges Verify row 4a. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `for c in dispatch-worker message-agent isolate-workspace invoke-skill session-notifications; do grep -qF "$c" plugins/assay/references/claude-code.md && grep -qF "$c" plugins/assay/references/codex.md \|\| echo "MISSING $c"; done > /tmp/hp04r5.out; test ! -s /tmp/hp04r5.out; echo $?` | `0`, and the control prints MISSING | printed `0`. The control run was the same loop with `no-such-cap` appended to the list. It printed `1` and the line `MISSING no-such-cap`. Discharges Verify row 5. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `(cd tools/plugindrift && GOWORK=off go run . --root ../..); echo $?` | `0` | printed `0`; output `coverage: 14 bundled skills/*/SKILL.md — 0 pinned, 6 canonical, 8 unported, 0 unaccounted`, `PLUGINDRIFT: CLEAN (0 origins …)`. Discharges Verify row 6. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `grep -rlE 'harnesslint' .github/workflows > /tmp/hp04r7.out; test -s /tmp/hp04r7.out; echo $?` | `0` | printed `0`; the match is .github/workflows/ci.yml. That file carries a build-test matrix case that runs `go test ./...` for tools/harnesslint, plus a dedicated `harnesslint` job that runs `bodies` over plugins/assay/skills and `bindings` over plugins/assay/references under `set -euo pipefail`. The workflow triggers on every `push` and `pull_request` with no path filter, which covers every PR touching plugins/assay. Discharges Verify row 7. The previous FAIL is cleared. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | (no executable command: a live Claude Code loop cycle, fanout a trivial brief → review → verify) | dispatch occurs, isolation held, evidence recorded | could-not-check, BLOCKED. This row needs an interactive live-harness loop that opens a worker PR, posts a review and lands Evidence. Those are mutating forge writes, and they fall outside this dispatch's offline, read-only envelope. No non-implementer live-session record exists in this brief's Evidence. Discharges Verify row 8 as BLOCKED. | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Toolchain: local go1.27.1 darwin/arm64. The module and the CI job pin go 1.25.0.

**Findings**
- #1332 (harness-portability/15: CI-wiring patch never pushed) is still OPEN, but #1813 merged the wiring it asks for. It looks resolved and can be closed after a check.
- tools/harnesslint/ci.yml.patch is still in the tree. It is now a stale leftover, since the live wiring is in ci.yml. Cleanup candidate.
- Row 5 checks only the brief's original 5 capabilities across 2 binding files. The live vocabulary now has 8 entries, and there is a third binding file (cursor.md). Closure over the full set is proven by row 3 (the bindings lint reads the README vocabulary), so nothing is lost. The row itself is narrower than the current seam.
- Every Evidence row maps to a Verify row. This pass did no work outside the table.

**Risk-bearing value enumeration.** The item has risk metadata `{regulatory: no, customer: no, irreversible: no, sensitive-data: no}` and `gate: model`, and it touches no risk-classed path. Enumeration covered the brief's deliverables (tools/harnesslint, the binding files, the stream README vocabulary block) and the two fix diffs since the last run (#1817, #1813):

| Rank | Identifier = literal | Location | Irreversibility |
|---|---|---|---|
| 1 | exitClean = 0, exitFailed = 1, exitCannot = 2, exitUsage = 2 | tools/harnesslint/lint.go lines 36–39 | The three-state exit contract that the new CI job gates on (`set -euo pipefail` reddens on 1 and on 2). A wrong value would silently green a could-not-check. Reversible with an edit and redeploy. |
| 2 | capability vocabulary = {dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications, durable-monitor, stop-worker, cadence-tick} (8 entries) | docs/streams/harness-portability/README.md lines 410–419 | This is the closed set, amended in the README by design. Reversible. |
| 3 | system-demo degradation cell = runs | plugins/assay/references/claude-code.md line 58 (added by #1817) | A binding-cell value. It matches the `runs` cells in codex.md line 57 and cursor.md line 90. Reversible. |
| 4 | banned-token list = 19 entries | tools/harnesslint/banned-tokens.md lines 28–48 | Lint config knob. Reversible. |
| 5 | defaultReadmePath = "docs/streams/harness-portability/README.md", vocabMarker, bannedMarker | tools/harnesslint/lint.go lines 48, 53, 56 | Lookup knobs. An empty read routes to exit 2, never to a silent pass. Reversible. |

RISK-VALUE: DERIVED — exitClean = 0 / exitFailed = 1 / exitCannot = 2 / exitUsage = 2 @ tools/harnesslint/lint.go:36-39 — this is the brief's three-state ground rule ("parse error / unreadable file / empty vocabulary → non-zero with could-not-check, never a silent pass"). The mapping is 0 = clean, 1 = failed and 2 = could-not-check, and usage errors fold onto 2, so there are still exactly three states. Exit 0 and exit 1 were exercised live in rows 2, 2a, 3 and 3a. Both non-zero codes redden the CI job.
RISK-VALUE: DERIVED — capability vocabulary = the 8-entry set @ docs/streams/harness-portability/README.md:410-419 — the brief's decided seam keeps one closed set in the stream README, and the lint reads it from there instead of holding a copy. Growing from 5 to 8 entries is the intended amend-in-README path. Row 3 (closure checked over all 8 in all three binding files) and row 3a show closure is enforced against the live set.
RISK-VALUE: DERIVED — system-demo cell = runs @ plugins/assay/references/claude-code.md:58 — on Claude Code every capability binds natively, so the Claude binding's degradation column is all `runs` by design (brief task 2: "the trivial all-runs degradation column"). The Codex and Cursor bindings independently rule system-demo `runs`, because the skill needs no dispatch, isolation or gate surface.
Ranks 4 and 5 are reversible lint-config knobs. They are ranked last and not derived.

rows_passed=10 rows_total=11

VERIFY: BLOCKED — rows 1, 2, 2a, 3, 3a, 4, 4a, 5, 6 and 7 pass at merged main 024c87b01aba, and the two FAILs from 2026-09-27 are cleared by #1817 and #1813. Row 8 (live Claude Code loop cycle) is could-not-check for a dispatched offline verifier and stays BLOCKED on a human or live-session run. The item stays at implemented.

### 2026-10-02 non-implementer verifier re-run — VERIFY: BLOCKED (rows 1–7 pass; row 8 needs a live session) — 2026-10-02T22:56:45Z, merged main e1d99484ffd9

Header: run at 2026-10-02T22:56:45Z (UTC) against merged main e1d99484ffd9, in a detached worktree, with a throwaway HOME and an empty environment apart from PATH and the Go caches. Toolchain go1.27.1 darwin/arm64 (the module pins go 1.25.0). Runner is not the implementer. Every row was executed by hand as authored, from the repo root. Rows whose command ends in "; echo $?" always return shell exit 0, so the verdict for those rows is the printed value, shown beside the exit.

**What moved since the last recorded outcome (2026-09-27, verify-fail on rows 3, 7 and 8, blocker #1703):** #1703 is now CLOSED. #1817 added the system-demo cell to the Claude Code binding (row 3) and #1813 wired harnesslint into the ci workflow (row 7). Since the 2026-10-01 block above, the only changes under this brief's inputs are edits to three skill bodies (author-brief, pr-review-desk and two of its reference notes), a one-line stream README change, and this brief's own Evidence. The Verify table is unchanged, and the bodies lint is still clean over the edited bodies (row 2).

| # | Command | Exit | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | harnesslint module test suite, as authored (go test ./... with GOFLAGS=-buildvcs=false, output captured, exit echoed) | 0 (printed 0) | "ok" for the harnesslint package in 1.040s. A supplementary verbose run with -count=1 showed 21 of 21 top-level tests PASS, none FAIL, including the per-banned-token red test, the unknown-capability, missing-capability and missing-skill-cell red tests, and the could-not-check tests. | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | build harnesslint with GOWORK=off, run "bodies" over the shipped skills directory | 0 (printed 0) | "checked-clean: bodies — no violations" (14 skill bodies) | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2a | mutation, as authored: copy the skills tree, append a line with backticked Agent and SendMessage to the adopt body copy, run "bodies" over the copy, remove the copy | 0 (printed 1; non-zero expected) | two violations at line 122 of the adopt copy: banned harness token "SendMessage" and banned harness token backticked Agent; then "checked-failed: bodies — 2 violation(s)". Names the adopt file and both tokens. | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | build harnesslint, run "bindings" over the shipped references directory | 0 (printed 0) | four files skipped by their own declared non-matrix-reference reason (desk-common, desk-shell, standing-note, tick-contract), then "checked-clean: bindings — no violations". The Claude Code binding carries the system-demo row at line 58. | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3a | mutation, as authored: copy the references directory, replace the codex copy with one stripped of every dispatch-worker line, run "bindings" over the copy, remove the copy | 0 (printed 1; non-zero expected) | codex copy: capability "dispatch-worker" does not resolve — no capability:dispatch-worker binding present; then "checked-failed: bindings — 1 violation(s)" | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | git grep for SendMessage under the skills tree into a file, assert the file is empty | 0 (printed 0) | the output file is 0 bytes | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4a | positive control: git grep -c for SendMessage in the Claude Code binding file | 0 | count 2 (at least 1), so the empty result in row 4 means clean, not blind | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | loop over the five original capability names, grep each in the Claude Code and Codex binding files, assert nothing is reported missing | 0 (printed 0) | the output file is 0 bytes. Control: the same loop with no-such-cap appended printed 1 and the line "MISSING no-such-cap". | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | neighbour row: plugindrift with GOWORK=off and --root at the repo root | 0 (printed 0) | "coverage: 14 bundled skills/*/SKILL.md — 0 pinned, 6 canonical, 8 unported, 0 unaccounted"; "PLUGINDRIFT: CLEAN (0 origins …)" | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | grep -rl for harnesslint under the workflows directory, assert a match | 0 (printed 0) | one match, the ci workflow: a build-test case that runs the harnesslint test suite, plus a dedicated harnesslint job that runs "bodies" and "bindings" over the real tree | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | (prose row, no executable command) one full loop cycle, fanout a trivial brief → review → verify, driven from the rewritten skills in a real Claude Code session | unrun | could-not-check. The row needs an interactive live session that opens a worker PR, posts a review and lands Evidence; a dispatched read-only verifier cannot do that. No non-implementer live-session record exists in this Evidence section. Tracked in #1834 (open, help wanted). | 2026-10-02 | assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness (dry run, nothing written):** statusgen v1.0.31 verifyrun over this brief exited 1 with 10 of 11 rows recorded pass and row 8 recorded fail exit=2 (a shell parse failure on the prose row, not a regression). The witness is weak evidence for rows 1–3a, 4, 5, 6 and 7: each ends in "; echo $?", so it records pass at exit 0 whatever the tool returned. The by-hand printed values above are the real result. This table has no check:ci row.

**Vacuity check.**
- Rows 1, 2, 2a, 3, 3a discriminate. The harnesslint module and both binding files were added by this stream's work, so each row errors without them. Rows 2a and 3a are live mutations. Two further mutations in a scratch copy of the module: disabling the banned-token match turned the per-banned-token test red; disabling the capability-resolution check turned three bindings tests red (missing capability, undeclared reference, closure without roster).
- Row 3 failed on real drift on 2026-09-16 and 2026-09-27, and row 7 failed until #1813, so both have been observed red.
- Row 5 discriminates only on the file existing and containing the five strings anywhere. It covers 5 of the 8 live capabilities and 2 of the 3 binding files. Row 3 carries full closure.
- Row 6 is non-discriminating for this brief, as its own Expect cell says: the plain exit is 0 regardless of drift.
- Row 7 matches any mention of the word in a workflow, a comment included. Reading the ci workflow confirms a real job runs the lint.
- Rows 4 and 4a are a weak pair: row 4 checks one token only, which the bodies lint (row 2) already covers.

**Risk-bearing value enumeration.** Risk metadata is present and all "no"; gate is model; no risk-classed path is touched. Enumerated over the harnesslint module, the two binding files and the stream README vocabulary block:

| Rank | Identifier = literal | Location | Irreversibility |
|---|---|---|---|
| 1 | exitClean = 0, exitFailed = 1, exitCannot = 2, exitUsage = 2 | tools/harnesslint/lint.go lines 36–39 | The three-state exit contract the CI job gates on. Reversible by an edit and redeploy. |
| 2 | capability vocabulary = 8 entries (dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications, durable-monitor, stop-worker, cadence-tick) | stream README lines 410–419 | Closed set, amended in the README by design. Reversible. |
| 3 | banned-token list | tools/harnesslint/banned-tokens.md | Lint config knob. Reversible. |
| 4 | defaultReadmePath and the three marker strings | tools/harnesslint/lint.go lines 48, 53, 56, 75 | Lookup knobs; an empty read routes to exit 2. Reversible. |

RISK-VALUE: DERIVED — exitClean = 0 / exitFailed = 1 / exitCannot = 2 / exitUsage = 2 @ tools/harnesslint/lint.go:36-39 — the brief's ground rule requires parse error, unreadable file or empty vocabulary to be non-zero and distinct from a violation; 0, 1 and 2 keep the three states apart, and usage errors fold onto could-not-check so no fourth state exists. Exit 0 and 1 were exercised live in rows 2, 2a, 3 and 3a.
RISK-VALUE: DERIVED — capability vocabulary = the 8-entry set @ the stream README lines 410–419 — the brief's decided seam keeps one closed set in the stream README and the lint reads it from there; the five entries this brief introduced are all present, and the three later entries arrived by the intended amend-in-README path. Row 3 enforces closure over all 8.
Ranks 3 and 4 are reversible lint-config knobs, ranked last and not derived.

rows_passed=10 rows_total=11

VERIFY: BLOCKED — rows 1, 2, 2a, 3, 3a, 4, 4a, 5, 6 and 7 pass by hand at merged main e1d99484ffd9. Row 8 (a live Claude Code loop cycle) is could-not-check for a dispatched verifier and needs a human-run live session, tracked in #1834. The item stays at implemented.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Run on 2026-10-04 against merged main bed1a31ba875 (medici-finance/assay), in a detached worktree. Runner: assay-verifier-app[bot], a dispatched verifier that is not the implementer. Host toolchain: go1.27.1 darwin/arm64. The module pins go 1.25.0, and the Linux witness used go1.25.14. Frontmatter: `gate: model`, `risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}`.

Grounded expectation, written from the brief's Task and Verify table before reading the earlier Evidence blocks: rows 1 to 7 exit or print as the Expect column says, and row 8 cannot be checked by a dispatched offline verifier.

**Why this re-run:** the receipt of 2026-10-02 at e1d99484ffd9 (blocked, 10 of 11, blocker #1834) went stale. Since that sha, 106 commits landed. Eight declared inputs changed: claude-code.md, codex.md, SOURCES.yaml, the ci workflow, and the pr-review-desk, the-desk, verify-desk and worker-desk skill bodies. The harnesslint module, the stream README vocabulary block, the brief and the Makefile did not change. The changes do touch what this brief verifies. #2146 added a new skill body, cut-release, which is the 15th. That PR also added a cut-release degradation cell to each of the three binding files (all `runs`) and declared the skill unported in SOURCES.yaml. That is exactly the surface rows 2, 3 and 6 gate on. The four desk-body edits are the surface row 2 lints. The ci workflow change only adds a new desk-platform-compile job, and the harnesslint matrix leg and the dedicated harnesslint job are unchanged (row 7). Blocker #1834 is still OPEN (help wanted), checked read-only with gh issue view.

Path note: the rows were run by hand from the worktree root, as authored, with one change: the /tmp scratch prefix was moved under the worktree's own untracked .v/tmp directory to keep every write inside the verifier's home. The witness below ran the commands verbatim inside the container, where /tmp is the container's own.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | harnesslint module test suite, as authored: go test ./... with GOFLAGS=-buildvcs=false, output captured, exit echoed | `0`, per-fixture red tests included | printed `0`; output `ok github.com/medici-finance/assay/tools/harnesslint 0.465s`. A supplementary verbose run with -count=1 showed 21 of 21 top-level tests PASS and 0 FAIL. Those include the per-banned-token red test, the unknown-capability, missing-capability and missing-skill-cell red tests, and the three-state could-not-check tests. Discharges Verify row 1. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | build harnesslint with GOWORK=off, run `bodies` over the shipped skills directory | `0` | printed `0`; output `checked-clean: bodies — no violations`, over 15 skill bodies, the new cut-release body included. Discharges Verify row 2. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2a | mutation, as authored: copy the skills tree, append a line with backticked Agent and SendMessage to the adopt copy, run `bodies` over the copy, remove it | non-zero, names the adopt file and both tokens | printed `1`. The output has two lines at adopt copy SKILL.md line 122. One reads banned harness token "SendMessage" and the other reads banned harness token backticked Agent. It ends `checked-failed: bodies — 2 violation(s)`. Discharges Verify row 2a. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | build harnesslint, run `bindings` over the shipped references directory | `0` | printed `0`. Four files were skipped by their own declared non-matrix-reference reason (desk-common, desk-shell, standing-note, tick-contract). The output then ends `checked-clean: bindings — no violations`. The new cut-release cell is present in all three binding files: claude-code.md line 50, codex.md line 49 and cursor.md line 82. Discharges Verify row 3. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3a | mutation, as authored: copy the references directory, replace the codex copy with one stripped of every dispatch-worker line, run `bindings` over the copy, remove it | non-zero naming `dispatch-worker` | printed `1`; output `codex.md: capability "dispatch-worker" does not resolve — no capability:dispatch-worker binding present`, then `checked-failed: bindings — 1 violation(s)`. Discharges Verify row 3a. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | git grep for SendMessage under the skills tree into a file, assert the file is empty | `0` | printed `0`; the output file is 0 bytes. Discharges Verify row 4. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4a | positive control: git grep -c for SendMessage in the Claude Code binding file | `>= 1` | exit 0; output `plugins/assay/references/claude-code.md:2`, a count of 2. Row 4's empty result therefore means clean, not blind. Discharges Verify row 4a. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | loop over the five original capability names, grep each in the Claude Code and Codex binding files, assert nothing is reported missing | `0`, and the control prints MISSING | printed `0` and the output file is 0 bytes. In the control, the same loop with no-such-cap appended printed `1` and the line `MISSING no-such-cap`. Discharges Verify row 5. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | neighbour row: plugindrift with GOWORK=off and --root at the repo root | `0` | printed `0`; output `coverage: 15 bundled skills/*/SKILL.md — 0 pinned, 6 canonical, 9 unported, 0 unaccounted` and `PLUGINDRIFT: CLEAN (0 origins — every bundled file is canonical-here or authored-here)`. The new cut-release body is accounted for by its SOURCES.yaml unported entry. Discharges Verify row 6. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | grep -rl for harnesslint under the workflows directory, assert a match | `0` | printed `0`; one match, the ci workflow. Reading it confirms two callers. A build-test case runs `go test ./...` for the harnesslint module. A dedicated `harnesslint` job runs `bodies` over the skills and `bindings` over the references under `set -euo pipefail`. The workflow triggers on every push and pull request. Discharges Verify row 7. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | (prose row, no executable command) one full loop cycle, fanout a trivial brief → review → verify, driven from the rewritten skills in a real Claude Code session | dispatch occurs, isolation held, evidence recorded | could-not-check, BLOCKED. The row needs an interactive live session that opens a worker PR, posts a review and lands Evidence. Those are live forge writes outside this dispatch's offline, read-only envelope. No non-implementer live-session record exists in this Evidence section. Tracked in #1834, which is open. Discharges Verify row 8 as BLOCKED. | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (claude-opus-5-5) (on-behalf-of human:ian) |

**Vacuity check.** No row is a `--consumers` or `--diff-base` row, so none passes only because there is no diff. Rows 2a and 3a are live mutations that went red, so the lint can fail. Row 3 failed on real drift on 2026-09-16 and 2026-09-27, so it has been observed red. Row 6's exit code does not discriminate, as its own Expect cell says. Its discriminating content is the printed `0 unaccounted` / CLEAN line, and the new cut-release body moved that line from 14 to 15 skills, which shows the coverage really was re-read. Row 5 covers 5 of the 8 live capabilities in 2 of the 3 binding files. Row 3 carries full closure.

**Execution witness:** statusgen built from this sha's own source, version printed `dev`. It ran in a throwaway Linux container over a fresh clone under the worktree's .v directory, detached at bed1a31ba875 with the clone's origin/main pinned to the same sha. The container had `--network none`. Only the roster was mounted, read-only, with no credentials, tokens or PEMs. `--security-opt seccomp=unconfined` was set, and only inside this throwaway network-less container. Exact command:

`docker run --rm --pull never --network none --security-opt seccomp=unconfined -v <home>/.v/clone:/work -v $(go env GOMODCACHE):/go/pkg/mod:ro -v ~/.config/assay/roster.env:/root/.config/assay/roster.env:ro golang:1.25-trixie bash -c "<script>"`

The script sets `git config --global --add safe.directory '*'` and the in-container verifier identity. It builds statusgen from the clone's statusgen directory and then runs `statusgen verifyrun --brief docs/streams/harness-portability/brief-04-neutral-core-skills.md --timeout 10m` with GOFLAGS=-count=1, GOPROXY=off and GOTOOLCHAIN=local, followed by `statusgen verifyrun --check` on the same path. verifyrun exited 1, and so did the check. The first container pass ran the script from an untracked file inside the clone, so its witness was stamped `+dirty`. That pass was discarded and the clone reset, and the pass below ran the script inline on a clean tree.

verifyrun --check summary: `docs/streams/harness-portability/brief-04-neutral-core-skills.md: 10 pass, 1 fail, 0 could-not-run/missing (of 11 Verify rows)`

Rows 1, 2, 2a, 3, 3a, 4, 5 and 7 end in "; echo $?", so the witness records pass at exit 0 whatever the tool returned. The output hashes carry the printed value: sha256:9a271f2a916b is the hash of `0` plus a newline, and sha256:4355a46b19d3 is the hash of `1` plus a newline. Those hashes agree with the by-hand rows: 0 on rows 1, 2, 3, 4, 5 and 7, and 1 on mutation rows 2a and 3a. Row 8's `fail exit=2` is a shell parse failure on the prose row, not a regression. The table has no check:ci row.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/harnesslint && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp04r1.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 2 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bodies plugins/assay/skills > /tmp/hp04r2.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 2a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/skills /tmp/hp04-dirty && printf '\nUse the \x60Agent\x60 tool with SendMessage.\n' >> /tmp/hp04-dirty/adopt/SKILL.md && /tmp/hl870 bodies /tmp/hp04-dirty > /tmp/hp04r2a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 3 | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && /tmp/hl870 bindings plugins/assay/references > /tmp/hp04r3.out 2>&1; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 3a | `GOWORK=off go build -C tools/harnesslint -o /tmp/hl870 . && cp -r plugins/assay/references /tmp/hp04-dirty-bind && grep -vF 'dispatch-worker' plugins/assay/references/codex.md > /tmp/hp04-dirty-bind/codex.md && /tmp/hl870 bindings /tmp/hp04-dirty-bind > /tmp/hp04r3a.out 2>&1; echo $?; rm -rf /tmp/hp04-dirty-bind` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 4 | `git grep -nE 'SendMessage' -- plugins/assay/skills > /tmp/hp04r4.out; test ! -s /tmp/hp04r4.out; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 4a | `git grep -cE 'SendMessage' -- plugins/assay/references/claude-code.md` | pass exit=0 | sha256:979226bec361 | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 5 | `for c in dispatch-worker message-agent isolate-workspace invoke-skill session-notifications; do grep -qF "$c" plugins/assay/references/claude-code.md && grep -qF "$c" plugins/assay/references/codex.md \|\| echo "MISSING $c"; done > /tmp/hp04r5.out; test ! -s /tmp/hp04r5.out; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 6 | `(cd tools/plugindrift && GOWORK=off go run . --root ../..); echo $?` | pass exit=0 | sha256:e8a6985bd19d | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -rlE 'harnesslint' .github/workflows > /tmp/hp04r7.out; test -s /tmp/hp04r7.out; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |
| 8 | `**BLOCKED (needs live Claude session, non-CI)** — regression: one full loop cycle (fanout a trivial brief → review → verify) driven from the rewritten skills in a real Claude Code session` | fail exit=2 | sha256:a5466c565aba | 2026-10-04 | assay-verifier-app[bot] @ bed1a31ba875 (on-behalf-of human:ian) (git-config) |

**Risk-bearing value enumeration.** Risk metadata is present and every field is "no". The gate is model, and no risk-classed path is touched. The enumeration covered the brief's deliverables (the harnesslint module, the binding files and the stream README vocabulary block) and the input diff since e1d99484ffd9 (#2146's cut-release cells and SOURCES entry).

| Rank | Identifier = literal | Location | Irreversibility |
|---|---|---|---|
| 1 | exitClean = 0, exitFailed = 1, exitCannot = 2, exitUsage = 2 | tools/harnesslint/lint.go lines 36–39 | The three-state exit contract the CI job gates on. Reversible with an edit and a redeploy. |
| 2 | capability vocabulary = 8 entries (dispatch-worker, message-agent, isolate-workspace, invoke-skill, session-notifications, durable-monitor, stop-worker, cadence-tick) | docs/streams/harness-portability/README.md lines 410–419 | The closed set, amended in the README by design. Reversible. |
| 3 | cut-release degradation cell = runs | plugins/assay/references/claude-code.md line 50; codex.md line 49; cursor.md line 82 (added by #2146) | A binding-cell value. Reversible. |
| 4 | defaultReadmePath = "docs/streams/harness-portability/README.md", vocabMarker = "&lt;!-- assay:capability-vocabulary", bannedMarker = "&lt;!-- assay:banned-tokens" | tools/harnesslint/lint.go lines 48, 53, 56 | Lookup knobs. An empty read routes to exit 2, never to a silent pass. Reversible. |
| 5 | banned-token list | tools/harnesslint/banned-tokens.md | Lint config knob. Reversible. |

RISK-VALUE: DERIVED — exitClean = 0 / exitFailed = 1 / exitCannot = 2 / exitUsage = 2 @ tools/harnesslint/lint.go:36-39 — the brief's ground rule requires parse errors, unreadable files and an empty vocabulary to be non-zero with could-not-check, and kept apart from a violation. 0, 1 and 2 keep the three states distinct, and usage errors fold onto could-not-check, so no fourth state exists. Exits 0 and 1 were exercised live in rows 2, 2a, 3 and 3a, and the CI job's `set -euo pipefail` reddens on both 1 and 2.
RISK-VALUE: DERIVED — capability vocabulary = the 8-entry set @ docs/streams/harness-portability/README.md:410-419 — the brief's decided seam keeps one closed set in the stream README, and the lint reads it from there. The five entries this brief introduced are all present. The three later ones arrived by the intended amend-in-README path. Row 3 enforces closure over all 8.
RISK-VALUE: DERIVED — cut-release cell = runs @ plugins/assay/references/claude-code.md:50 (and codex.md:49, cursor.md:82) — the Claude binding's degradation column is all `runs` by design (brief task 2, "the trivial all-runs degradation column"). In the Codex and Cursor cells, the skill's only write is a gh write, stated as a sandbox or permission precondition. Its release-workflow dispatches and approvals are handed to the driver through human-runsheet. So the skill touches no dispatch, isolation or gate surface, and `runs` is the ruled posture. The new skill body carries no harness tokens (row 2 is clean over 15 bodies).
Ranks 4 and 5 are reversible lint-config knobs. They are ranked last and not derived.

**Findings (carried, not new):** tools/harnesslint/ci.yml.patch is still in the tree as a stale leftover, since the live wiring is in the ci workflow. Row 5 covers only the original 5 capabilities across 2 of the 3 binding files, and row 3 carries full closure.

rows_passed=10 rows_total=11

VERIFY: BLOCKED — rows 1, 2, 2a, 3, 3a, 4, 4a, 5, 6 and 7 pass by hand at merged main bed1a31ba875, the new cut-release skill and cells included. Row 8, a live Claude Code loop cycle, cannot be checked by a dispatched verifier and needs a human-run live session. Blocker: #1834 (open, help wanted). The item stays at implemented.

## Review

Gate: **model** (from frontmatter). Review priority: the diff of the seven bodies,
hunting the softened-guarantee class (a NEVER weakened, a refusal turned best-effort, a
degradation left implicit) — the structural rows cannot catch it; a reviewer reading
the 3-dot diff can.
