---
brief: harness-portability/11
title: Durable-monitor capability + residual harness-token prose-audit
why: >-
  Brief 04's neutrality lint is TOKEN-only: it catches the fifteen banned strings
  but not the durable-watch mechanism the desks lean on — the `Monitor` tool with
  `persistent: true` (a Claude-Code-specific cross-turn watchdog) dangles on any
  harness without one. That mechanism has no name in the closed capability vocabulary,
  so a Codex session reads the desk bodies as instructions to arm a tool it does not
  have, silently, with no lint to flag it. The method's whole claim is "the method,
  not the model": a load-bearing wake-signal mechanism named as a raw Claude tool is
  exactly the single-vendor leak this stream exists to close. One focused follow-up
  names the capability, binds it per harness, and audits the ~15 residual sites the
  token-lint cannot see — replacing the disproportionate 04c/d/e split (now closed).
wave: 3
depends: ["harness-portability/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-17 by harness-portability authoring session
sources: ["brief 04 (HP/04): shipped tools/harnesslint (TOKEN-only bodies lint + bindings closure), the two plugins/assay/references/{claude-code,codex}.md binding files, and token-strips on 4 skill bodies — this brief builds directly on that landing", "residual analysis at brief 04's landing: the token-lint's banned set (tools/harnesslint banned-tokens config) contains NO Monitor/persistent/TaskList/EnterWorktree entry, and the closed vocab (README capability block) has NO durable-watch capability — so ~15 durable-monitor + EnterWorktree sites survive brief 04 uncaught", "replaces the closed 04c/04d/04e split: this single M-brief is the ~10-20-site core that remained after brief 04, scoped as one focused follow-up rather than three disproportionate sub-briefs", "the measured Codex capability matrix (HP/01) §3.6/§3.7: Codex has no durable cross-restart monitor — V2 child agents are process-local (resume broken across restarts, codex issues #19140/#33002), an in-subagent run_in_background task is silently killed — the measured basis for the Codex degradation cell", "stream README non-negotiable floor (§ 'What natively means' item 3; ruling C via codex.md): isolation, evidence, and the review gates never degrade; a durable wake-signal is a CONVENIENCE, so durable-monitor DEGRADES on a harness that lacks it, it does not refuse"]
consumers: ["docs/streams/harness-portability/README.md capability-vocabulary block: fixed-here (the vocab amendment — adding durable-monitor — IS part of this deliverable; harnesslint reads the closed set from this one block)", "plugins/assay/references/claude-code.md + codex.md: follow-up harness-portability/11 (both gain a capability:durable-monitor row — else harnesslint bindings fails closure — and codex.md gains the degradation cell; these binding files are edited in brief-11's implementation phase, not this authoring PR)", "plugins/assay/skills/{pr-review-desk,intake-desk,the-desk,verify-desk,worker-desk}/SKILL.md: follow-up harness-portability/11 (the ~15 residual sites rewritten to capability vocabulary — brief-11's implementation edit, not this authoring PR)", "tools/harnesslint banned-tokens config: follow-up harness-portability/11 (OPTIONAL hardening — add the backticked Monitor/TaskList/EnterWorktree + persistent:true forms so recurrence is lint-caught, not re-audited by hand; deferred to brief-11's implementation phase)"]
exec-tier: strong
exec-tier-why: >-
  (c): safety-adjacent method text. The durable monitor is the desks' liveness
  backstop against the silent-blind failure; a rewrite that loses the "best-effort,
  cadence-sweep is the real backstop" nuance, or a Codex degradation cell that reads
  refuse instead of degrade, weakens a guardrail while every token-lint still passes.
  The convenience-vs-guarantee classification and the verbatim-degradation discipline
  need care.
---

# Brief 11 — Durable-monitor capability + residual harness-token prose-audit

## Context

files: *(the three `references/*.md` files below plus the banned-tokens config are CREATED
by dep 04 (HP/04) and only AMENDED here.)*
- **amend** `docs/streams/harness-portability/README.md` — add ONE capability name to
  the machine-readable `<!-- assay:capability-vocabulary -->` block (and keep the prose
  list in "The seam" §1 in step, per the block's own note). This is the vocab amendment;
  `tools/harnesslint` reads the closed set from this block, so it MUST land in this PR.
  Also add the brief-11 row + status-table entry (this file is both the vocab source
  and the stream README).
- **amend** `plugins/assay/references/claude-code.md` — add a `capability:durable-monitor`
  row to the Capability → mechanism table (mechanism = the `Monitor` tool, `persistent: true`,
  `TaskList` to check for an existing monitor before arming a second).
- **amend** `plugins/assay/references/codex.md` — add the same capability row with its
  **degradation cell** authored from the measured facts (HP/01 §3.6/§3.7), and note in
  the affected per-skill Codex cells that the liveness monitor degrades to
  event-driven + fixed-cadence sweep.
- **amend** the five skill bodies below — rewrite the residual `Monitor` / `persistent: true`
  / `TaskList` / `EnterWorktree` sites to capability vocabulary.
- **amend** the `tools/harnesslint` banned-tokens config (OPTIONAL hardening, see Task 5) — add the
  backticked harness-token forms so a reintroduced `Monitor` is lint-caught.

facts:
- **What brief 04 shipped, and its blind spot.** Brief 04 shipped a TOKEN-only
  `harnesslint bodies` (fifteen banned strings in its banned-tokens config)
  and a `harnesslint bindings` closure check, plus the two per-harness binding files. The
  closed capability vocabulary today is exactly five names —
  `dispatch-worker`, `message-agent`, `isolate-workspace`, `invoke-skill`,
  `session-notifications` — read by the lint from the README's capability block. **None of
  those five is a durable-watch capability, and `Monitor` / `persistent: true` / `TaskList`
  / `EnterWorktree` are NOT in the banned-token set.** So the token-lint is green while ~15
  sites still name Claude-specific mechanisms in prose. This brief closes exactly that gap.
- **The durable-monitor mechanism.** Several desks arm a `persistent: true` `Monitor` — a
  re-arming poll that survives across turns and re-invokes the session on a new event or a
  fixed cadence. It is the desk-side liveness backstop against the silent-blind failure
  (a wave of actionable PRs piled up while the desk reported idle). The bodies are explicit
  that it is **best-effort — NOT the sole wake signal**: the fixed-cadence board sweep is the
  real backstop and the always-on observability service is the durable home.
  That nuance is load-bearing and must survive the rewrite.
- **The new capability name.** Proposed: **`durable-monitor`** (mirrors the bodies' own
  "durable monitor" / "durable watchdog" language). Alternative for review: `watch-condition`.
  The authoring session did NOT invent policy here — the name is flagged for the
  reviewer/desk to ratify; pick one, use it consistently in the vocab block, both binding
  files, and every rewritten site.
- **Codex has no durable monitor — so this capability DEGRADES, it does not refuse.** Per
  HP/01 §3.6/§3.7: Codex V2 child agents are process-local (resume broken across restarts,
  codex issues #19140/#33002) and an in-subagent `run_in_background` task is silently killed, so
  there is no reliable durable cross-turn wake signal. Per the stream's non-negotiable floor
  (README "What natively means" item 3; ruling C in `codex.md`), the three guarantees that
  never degrade are **isolation, evidence, and the review gates** — a wake-signal is a
  **convenience**, not a guarantee. Therefore the Codex cell is **`degrades`**: the skill
  falls back to the event-driven + fixed-cadence board sweep and states the gap in-session;
  the always-on observability service is the durable liveness home, not the harness. This
  classification is a *derivation* from the ruled floor, not a new ruling — but the exact
  cell wording (and whether any per-skill cell warrants a sharper note) is left for review.
- **`EnterWorktree` maps to the EXISTING `isolate-workspace` capability** — no new
  capability warranted. Both sites already sit beside the neutral `git worktree add` recipe;
  the rewrite drops the "or the harness EnterWorktree" clause (the Claude mechanism already
  lives in `claude-code.md`'s `isolate-workspace` row) or points to `capability:isolate-workspace`.

## The residual site inventory (measured at brief 04's landing)

**IN SCOPE — durable-monitor (harness-token sites; the strongest rewrite case):**

| # | File:line | Text |
|---|-----------|------|
| 1 | `pr-review-desk/SKILL.md:118` | "the harness `Monitor` tool, `persistent: true`" |
| 2 | `pr-review-desk/SKILL.md:119` | "check `TaskList` first" |
| 3 | `pr-review-desk/SKILL.md:125` | "A `persistent: true` Monitor survives across turns" |
| 4 | `pr-review-desk/SKILL.md:131` | "Use the harness `Monitor` tool, not a disowned shell loop." |
| 5 | `pr-review-desk/SKILL.md:132` | "a second, independent `Monitor` (`persistent: true`)" |
| 6 | `pr-review-desk/SKILL.md:144` | "`Monitor`-based sweeps are the desk-side fallback" |
| 7 | `intake-desk/SKILL.md:131` | "arm the `Monitor` on the durable inbound-monitor script" |
| 8 | `the-desk/SKILL.md:185` | "the fixed-cadence `Monitor` that would run the read-only board sweep" |
| 9 | `verify-desk/SKILL.md:70` | "the fixed-cadence `Monitor` … deferred to the observability service" |

**IN SCOPE — durable-monitor (plain-prose sites; token-lint CANNOT catch these — the audit half):**

| # | File:line | Text |
|---|-----------|------|
| 10 | `intake-desk/SKILL.md:123` | "**Arm the persistent monitor**" |
| 11 | `the-desk/SKILL.md:387` | "via a persistent monitor on `gh pr list`" |
| 12 | `pr-review-desk/SKILL.md:147` | "the Monitor is a best-effort wake signal" |
| 13 | `pr-review-desk/SKILL.md:483` | "the two Monitors keep it fresh" |
| 14 | `worker-desk/SKILL.md:521` | "arm a Monitor (or poll each turn)" |

**IN SCOPE — isolate-workspace (EnterWorktree → existing capability):**

| # | File:line | Text |
|---|-----------|------|
| 15 | `verify-desk/SKILL.md:112` | "# or the harness EnterWorktree" |
| 16 | `worker-desk/SKILL.md:24` | "(or EnterWorktree)" |

**EXPLICITLY OUT OF SCOPE (state, do not touch):**
- **Plain prose the 04-split briefs ruled left alone**: e.g. "dispatch the batch",
  "its own worktree" — English, not a harness mechanism; already neutral.
- **Packaging / path tokens** — `.claude/`, `~/.claude`, `CLAUDE.md`, `CLAUDE_SESSION_ID`:
  deferred to **harness-portability/05** (resident-rules / packaging surface), not a
  skill-body capability touchpoint.
- **Proper nouns** — `Claude Cowork` is a competitor product name and stays verbatim.
- **Marginal execution-model wording** — `subagent` / `background` / `foreground`
  (author-brief 1, pr-review-desk 7, the-desk 1, verify-desk 1, worker-desk 2 hits):
  **OPTIONAL, lowest priority.** Codex ALSO has subagents and background execution, so
  most of this is genuinely neutral already; neutralize ONLY a site that names a
  Claude-specific execution guarantee, and leave the generic ones. Do not chase the count.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions. WORKER app: never approve / merge / flip-ready.
- Stop at `implemented` — you do not set verified/done.
- **The vocab amendment lands in THIS PR.** `harnesslint` reads the closed set from the
  README block; adding `capability:durable-monitor` to a body without adding the name to
  the block is a lint failure, and vice-versa (an orphan capability in the vocab with no
  binding row fails `harnesslint bindings`). All three — README block, both binding
  files, the bodies — move together.
- **Do NOT invent policy.** The capability NAME and the exact Codex degradation wording
  are flagged for review; propose, cite the floor, do not unilaterally rule.
- Preserve the "best-effort / cadence-sweep is the real backstop / observability service
  is the durable home" nuance at every rewritten monitor site — it is a guardrail, not
  decoration.
- If anything contradicts repo state (e.g. brief 04 landed differently): report
  NEEDS_CONTEXT, don't guess.

## Task

1. **Amend the closed vocabulary.** Add the ratified capability name (proposed
   `durable-monitor`) to the README `<!-- assay:capability-vocabulary -->` block AND the
   prose list in "The seam" §1. Nothing else in `harnesslint` needs editing — the lint's
   known set is entirely README-derived (`loadVocabulary` reads this block; there is no
   hard-coded vocabulary in `lint.go`).
2. **Bind it per harness.** Add a `capability:durable-monitor` row to BOTH
   `plugins/assay/references/claude-code.md` (mechanism: the `Monitor` tool with
   `persistent: true`; `TaskList` to avoid arming a second) and
   `plugins/assay/references/codex.md` (degradation cell from HP/01 §3.6/§3.7, classed
   `degrades` per the convenience floor). Update the affected per-skill Codex cells
   (`pr-review-desk`, `intake-desk`, `the-desk`) to note the liveness monitor degrades to
   event-driven + fixed-cadence sweep — the review VERDICT / evidence / isolation
   guarantees are unchanged.
3. **Rewrite the durable-monitor sites (#1–#14).** Replace each harness-token and
   plain-prose reference with `capability:durable-monitor` phrasing, preserving the
   best-effort / cadence-backstop / observability-service nuance.
4. **Rewrite the EnterWorktree sites (#15–#16)** to `capability:isolate-workspace`
   (drop the "or the harness EnterWorktree" clause — that mechanism lives in
   `claude-code.md`'s existing `isolate-workspace` row).
5. **OPTIONAL hardening — extend the token-lint so this residual cannot silently recur.**
   Add the backticked harness-token forms to the `tools/harnesslint` banned-tokens config with
   reasons: `` `Monitor` ``, `` `TaskList` ``, `` `EnterWorktree` ``, and `persistent: true`
   (use backticked/specific forms like the existing `` `Agent` `` precedent to avoid
   false-positives on the plain word "monitor" in observability prose). Note the limit
   honestly: the token-lint reduces recurrence of the *backticked* forms but the
   *plain-prose* monitor wording (#10–#14) still needs the manual audit — that is the
   nature of a prose-audit brief.
6. Add the brief-11 row to the README Briefs table and the dependency-wave / gate
   sections as house convention requires.

## Verify (executable — no prose-only DoD items)

Run from repo root, on a tree with brief 04 merged (harnesslint + binding files
present). Every absence-assertion below pairs a positive control (stream README discipline).

| # | Command | Expect |
|---|---------|--------|
| 1 | `sed -n '/assay:capability-vocabulary/,/-->/p' docs/streams/harness-portability/README.md \| grep -qx 'durable-monitor'; echo $?` | `0` — the ratified name is in the machine-readable closed set (adjust the literal if review renames it) |
| 2 | `go run ./tools/harnesslint bindings plugins/assay/references; echo $?` | `0` — every capability in the closed set (now incl. `durable-monitor`) resolves in BOTH binding files and every skill has a degradation cell |
| 2a | `grep -lc 'capability:durable-monitor' plugins/assay/references/claude-code.md plugins/assay/references/codex.md \| wc -l \| tr -d ' '` | `2` — positive control: the row is present in both files, not merely "closure didn't complain" |
| 3 | `go run ./tools/harnesslint bodies plugins/assay/skills; echo $?` | `0` — bodies still checked-clean (capability refs all in vocab, no banned tokens) |
| 4 | `grep -rn -e 'Monitor' -e 'persistent: true' -e 'TaskList' plugins/assay/skills/*/SKILL.md; echo "exit=$?"` | `exit=1` — grep (case-sensitive, separate `-e` patterns so no cell-shredding pipe) finds NO remaining capital-`Monitor` / `persistent: true` / `TaskList` harness token in any body. `durable-monitor` (lowercase, hyphenated) and lowercase prose "monitor" do NOT match, so the capability name and neutral prose are untouched |
| 4a | `grep -rc 'capability:durable-monitor' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/intake-desk/SKILL.md plugins/assay/skills/the-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md \| awk -F: '{s+=$2} END{print s}'` | `>= 8` — positive control for row 4: the durable-monitor sites were REWRITTEN to the capability, not merely deleted |
| 5 | `grep -rn 'EnterWorktree' plugins/assay/skills/*/SKILL.md; echo "exit=$?"` | `exit=1` — no EnterWorktree token remains |
| 5a | `grep -rc 'capability:isolate-workspace' plugins/assay/skills/verify-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md \| awk -F: '{s+=$2} END{print (s>=2)}'` | `1` — positive control for row 5: both former EnterWorktree sites now carry `capability:isolate-workspace` |
| 6 | **Recurrence guard (OPTIONAL hardening) — the lint now CATCHES a reintroduced token.** `go build -o /tmp/hl11 ./tools/harnesslint; f=plugins/assay/skills/the-desk/SKILL.md; cp "$f" /tmp/hp11.bak; printf '\nProbe line arming a persistent: true monitor.\n' >> "$f"; /tmp/hl11 bodies plugins/assay/skills; echo "exit=$?"; cp /tmp/hp11.bak "$f"` | `exit=1` (built binary, so harnesslint's checked-failed=1 is unambiguous — a compile failure would have failed `go build` first), output names `plugins/assay/skills/the-desk/SKILL.md` with the banned `persistent: true` token; after restore, row 3 returns `0` — the red was the plant (planted `persistent: true` avoids the nested-backtick a `Monitor` plant would need in this cell; SKIP this row if the Task 5 hardening was not taken, and say so in Evidence) |
| 7 | `grep -c 'never degrade' plugins/assay/references/codex.md` | `>= 1` — the non-negotiable floor (isolation / evidence / gates never degrade) is intact after the edit; durable-monitor sits BELOW it as a convenience |
| 8 | `go run ./statusgen --root . --lint; echo $?` | `0` — stream README + brief frontmatter + Evidence lint clean |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s), date, runner).
     Row 6's plant/restore output pasted, not summarised.
     "verified" requires a non-implementer per the stream README and this brief's gate. -->

**Tooling note.** `harnesslint` and `statusgen` are the stream's source-tree tools; per the
re-home note above, they are not vendored into this public repo, so rows 2/2a-adjacent, 3, and
6 were run with a `harnesslint` binary built from that source tree (its embedded banned-tokens
config already carries the harness-portability/11 additions — `` `Monitor` ``, `` `TaskList` ``,
`` `EnterWorktree` ``, `persistent: true`), and row 8 with `statusgen` from its module directory.
All commands ran offline (`KUBECONFIG=/dev/null`).

**Prior-work note.** The vocabulary amendment (Task 1), both binding-file rows (Task 2), the
EnterWorktree removal (Task 4), and the token-lint hardening (Task 5) already landed ahead of this
implementation in the neutral-form re-stage of the desk bodies; this PR completes Task 3 (the four
desk bodies still carried the durable-watch concept in plain prose, not the capability vocabulary)
and the residual capital-`Monitor` audit site, and flips the board row.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `sed -n '/assay:capability-vocabulary/,/-->/p' README.md \| grep -qx 'durable-monitor'` | `0` | name present in the machine-readable closed set | 2026-09-04 | HP/11 implementer |
| 2 | `harnesslint bindings plugins/assay/references` | `1` | **Pre-existing, OUT OF SCOPE:** 9 violations, all `no degradation cell for skill "ask-decision" / "install" / "upgrade-assay"` across all three reference files — bundle skills added after the binding files, unrelated to `durable-monitor`. Zero violations name a capability row; the `durable-monitor` closure is satisfied (see 2a). Not fixable within this brief's scope; surfaced for follow-up. | 2026-09-04 | HP/11 implementer |
| 2a | `grep -lc 'capability:durable-monitor' plugins/assay/references/{claude-code,codex}.md \| wc -l` | — | `2` — the row is present in BOTH binding files (positive control) | 2026-09-04 | HP/11 implementer |
| 3 | `harnesslint bodies plugins/assay/skills` | `1` | **Pre-existing, OUT OF SCOPE:** 3 violations, all `CLAUDE_PLUGIN_ROOT` in `plugins/assay/skills/ask-decision/SKILL.md` (lines 48, 52, 149) — a non-desk skill outside this brief's five-body scope. ZERO violations in any of the five desk bodies this brief touches; my edits add no banned token. | 2026-09-04 | HP/11 implementer |
| 4 | `grep -rn -e 'Monitor' -e 'persistent: true' -e 'TaskList' plugins/assay/skills/*/SKILL.md` | `1` | no capital-`Monitor` / `persistent: true` / `TaskList` harness token remains in any body | 2026-09-04 | HP/11 implementer |
| 4a | `grep -rc 'capability:durable-monitor' {pr-review-desk,intake-desk,the-desk,verify-desk}/SKILL.md \| awk` | — | `10` (>= 8) — sites REWRITTEN to the capability, not deleted (positive control) | 2026-09-04 | HP/11 implementer |
| 5 | `grep -rn 'EnterWorktree' plugins/assay/skills/*/SKILL.md` | `1` | no EnterWorktree token remains | 2026-09-04 | HP/11 implementer |
| 5a | `grep -rc 'capability:isolate-workspace' {verify-desk,worker-desk}/SKILL.md \| awk '{print (s>=2)}'` | — | `1` — both former EnterWorktree sites carry `capability:isolate-workspace` (positive control) | 2026-09-04 | HP/11 implementer |
| 6 | plant `persistent: true` into `plugins/assay/skills/the-desk/SKILL.md`; `harnesslint bodies`; restore | `1` | plant caught: `plugins/assay/skills/the-desk/SKILL.md:306: banned harness token "persistent: true" — … name the` `` `durable-monitor` `` `capability …`. After restore, `grep -c 'persistent: true' plugins/assay/skills/the-desk/SKILL.md` = `0` and row 3's desk bodies return to zero-violation. (Hardening from Task 5 was taken — in the source tree — so this row runs.) | 2026-09-04 | HP/11 implementer |
| 7 | `grep -c 'never degrade' plugins/assay/references/codex.md` | — | `3` (>= 1) — the isolation/evidence/gates never-degrade floor is intact; `durable-monitor` sits below it as a convenience | 2026-09-04 | HP/11 implementer |
| 8 | `statusgen --root . --lint` | `0` | `LINT: PASS`. Notices are pre-existing and on other streams (ordering-gate prose, closed-brief witnesses); harness-portability is not among them. | 2026-09-04 | HP/11 implementer |

**Rows 2 and 3 are checked-failed for reasons OUTSIDE this brief's scope** (three bundle skills —
`ask-decision`, `install`, `upgrade-assay` — lack binding rows / carry `CLAUDE_PLUGIN_ROOT`), not by
any change here; they are reported as-observed, never rounded to green. This brief's own contribution
(rows 1, 2a, 4, 4a, 5, 5a, 6, 7, 8, and the `durable-monitor` closure inside the bindings check) is
all green. The out-of-scope binding/coverage gap is flagged on the PR for a follow-up.

## Review

Gate: **model** (from frontmatter — this is git-revertible method text; no
funds/customers/regulators/irreversible surface). Review priority, in order:

1. **The capability name and the Codex degradation cell are the two judgement calls** —
   ratify `durable-monitor` (or pick `watch-condition`) and confirm the cell reads
   `degrades`, not `refuses`, because a wake-signal is a convenience below the
   never-degrade floor. If review wants `refuses` for a specific skill, that is a
   ruling to record, not an authoring default.
2. **The best-effort / cadence-backstop / observability-service nuance must survive** at
   every rewritten monitor site — a rewrite that flattens it to "arm the durable-monitor
   capability" and drops the "NOT the sole wake signal" guardrail re-opens the
   silent-blind failure at the method-text level.
3. Confirm the OUT-OF-SCOPE line held: no packaging/path tokens (HP/05 surface) and no
   proper nouns (`Claude Cowork`) were touched.
