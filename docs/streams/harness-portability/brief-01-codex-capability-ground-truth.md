---
brief: harness-portability/01
title: Codex capability ground-truth — measured matrix, not inherited prior art
why: >-
  Every downstream decision in this stream — the binding file's content, the per-skill
  degradation ruling, the packaging shape — binds to what Codex actually supports. The
  only capability facts in hand today are superpowers' measurements dated 2026-03-23,
  ~4.5 months stale for a fast-moving harness. Designing against stale third-party
  facts rebuilds, at design time, the same drift failure this stream exists to prevent.
wave: 0
depends: []
unblocks: ["harness-portability/03", "harness-portability/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07): make Assay run natively on Codex/GPT as a first-class second harness", "superpowers 6.2.0 prior art (upstream github.com/obra/superpowers): its skills/using-superpowers/references/codex-tools.md + docs/superpowers/specs/2026-03-23-codex-app-compatibility-design.md", "freshness-checked 2026-08-07 (no codex-specific research file existed; the tools-landscape survey mentions Codex only as a landscape entry)"]
exec-tier: strong
exec-tier-why: >-
  (a) distinguishing "capability absent" from "capability not found" requires judgement
  and deliberate probing (scoped-view discipline), and the matrix's verdicts are the
  facts every later brief builds on.
---

# Brief 01 — Codex capability ground-truth

## Context

files:
- **create** `docs/research/codex-harness-capabilities.md` (planned) — the measured matrix
- **amend** `freshness.yaml` — register the new file (empirical harness facts rot)

facts:
- Prior art to extract, then RE-MEASURE, never inherit: superpowers 6.2.0 —
  its references/codex-tools.md (multi-agent dispatch: `spawn_agent`/`wait_agent`/`close_agent`
  behind `[features] multi_agent = true` in `~/.codex/config.toml`; environment detection
  via `git rev-parse --git-dir` vs `--git-common-dir`; App finishing flow) and
  its 2026-03-23 Codex-App-compatibility design doc (Codex App
  sandbox: `git add`/`commit` work, `git checkout -b`/`git push`/`gh pr create` blocked
  under workspace-write; subagents share the parent filesystem; `network_access = true`
  silently broken on macOS). Every one of these is a claim dated 2026-03-23.
- Superpowers' Codex packaging observed at the same version: `.codex-plugin/plugin.json`
  with `"skills": "./skills/"` and `"hooks": {}` — evidence Codex App plugins carry a
  skills dir and that superpowers ships no hooks to it. Whether Codex supports ANY
  session-start hook equivalent is exactly the kind of open question this brief settles.
- Codex reads `AGENTS.md` natively; which paths it reads (repo root, nested,
  `~/.codex/AGENTS.md` global), how they compose, and size limits are unmeasured here.
- Two distinct targets exist and must be reported separately where they differ:
  **Codex CLI** (open-source terminal tool) and **Codex App** (managed worktrees,
  Seatbelt sandbox). Brief 03 rules which are in scope for v1; this brief measures both
  as far as the environment allows.
- **The live environment is the external head.** No in-house Codex install is known to
  exist (2026-08-07). The documentary half (prior-art extraction + public-docs sweep)
  proceeds without it; every live-harness row below is BLOCKED until Ian provides or
  sanctions an environment. Record `codex --version` (and App version) with every
  measured verdict.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Scoped-view discipline**: before recording any capability `absent`, show the probe
  that would have found it present (the positive-control habit, applied to research).
  A verdict with no probe is `unmeasured`, not `absent`.
- Do not install software or create accounts on your own authority — the environment is
  provided, not procured.

## Task

1. **Extract prior art** into a "claims to re-verify" appendix: every empirical claim in
   superpowers' codex-tools.md + the 2026-03-23 design doc, each tagged with its date.
2. **Sweep current public documentation** (OpenAI's Codex docs, the `openai-codex-plugins`
   repo, Codex CLI release notes) for: AGENTS.md loading rules, plugin/skill support and
   manifest schema, any hook/session-start mechanism, multi-agent status, sandbox modes.
   Cite each fact to a URL + retrieval date.
3. **Measure on a live environment** (when provided) every row of the capability matrix.
4. **Write the matrix** — one row per capability, columns:
   `Capability | Verdict | How measured | Codex version | Date`. Verdict vocabulary is
   closed: `supported` / `absent` / `partial` / `unmeasured` (with reason). Required
   capability rows (at minimum, one per CLI and App where they differ):
   `resident-rules-channel` (AGENTS.md paths, composition, size limits),
   `skills-discovery` (does the harness read SKILL.md-shaped skills; frontmatter contract),
   `auto-trigger` (description-driven invocation vs invoke-by-name only),
   `session-start-hook` (any injection mechanism beyond AGENTS.md),
   `subagent-dispatch` (spawn/wait/close; the config flag; parallelism limits),
   `agent-messaging` (can a running subagent be messaged/resumed),
   `background-notifications` (task-completion signals),
   `sandbox-git` (which git verbs work per sandbox mode),
   `workspace-isolation` (worktree creation/detection),
   `install-mechanism` (how an adopter installs a bundle).
5. **Register the file in `freshness.yaml`** — `max-age-days: 45`, upstreams empty
   (the upstream is a vendor, not a repo); the short leash is the point.

## Verify (executable — no prose-only DoD items)

Presence-gate rows for a research deliverable (quality is owned by review); rows 5–6
are live-harness and BLOCKED until the environment exists.

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/research/codex-harness-capabilities.md; echo $?` | `0` |
| 2 | `for c in resident-rules-channel skills-discovery auto-trigger session-start-hook subagent-dispatch agent-messaging background-notifications sandbox-git workspace-isolation install-mechanism; do grep -qF "$c" docs/research/codex-harness-capabilities.md \|\| echo "MISSING $c"; done > /tmp/hp01r2.out; test ! -s /tmp/hp01r2.out; echo $?` | `0` — all ten required capability rows present |
| 2a | **Positive control for row 2** — `grep -qF "no-such-capability-xyz" docs/research/codex-harness-capabilities.md; echo $?` | `1` — the probe reports absence for an absent token (`1` distinguishes "file exists, token absent" from a missing-file `2`, which row 1 already covers separately) |
| 3 | `grep -cE -e '[\|] *.?supported' -e '[\|] *.?absent' -e '[\|] *.?partial' -e '[\|] *.?unmeasured' docs/research/codex-harness-capabilities.md` | `>= 10` — every capability row carries a closed-vocabulary verdict (separate `-e` patterns; a `\|` "alternation" inside `-E` is a literal pipe, so the patterns are split; the `.?` tolerates the matrix's backticked verdict tokens) |
| 4 | `rm -f /tmp/hp01r4.out; grep -qF 'codex-harness-capabilities' freshness.yaml \|\| { echo "NOT REGISTERED"; exit 1; }; go run ./tools/freshness > /tmp/hp01r4.out 2>&1; grep -qE '^FRESH +docs/research/codex-harness-capabilities\.md' /tmp/hp01r4.out; echo $?` | `0` — registered and THIS file's own line reports `FRESH` (the tool's overall exit code is not load-bearing: an unrelated stale artifact elsewhere in the repo reddens the whole run regardless of this file, so this row checks the file's own line). The `rm -f` + fail-fast `NOT REGISTERED` guards against a stale-file false pass |
| 4a | **Mutation — the freshness leash can fail**: `go run ./tools/freshness --as-of 2027-01-01 > /tmp/hp01r4a.out 2>&1; grep -qE '^STALE +docs/research/codex-harness-capabilities\.md' /tmp/hp01r4a.out; echo $?` | `0` — force-aged past the 45-day leash, THIS file's line specifically flips to `STALE` (proving row 4's `FRESH` match is a real check, not a no-op) |
| 5 | **BLOCKED (needs live Codex)** — in the provided environment: `codex --version`, then the matrix's `subagent-dispatch` and `sandbox-git` verdicts re-measured live | Evidence records version + per-verb results. Until the environment exists this row is BLOCKED, never green |
| 6 | **BLOCKED (needs live Codex)** — `grep -cF 'unmeasured' docs/research/codex-harness-capabilities.md \|\| true` after the live pass | output `0` for CLI rows once measured (App rows may stay `unmeasured` if 03 rules App out; the `\|\| true` neutralises grep's exit-1-on-zero-matches, which is the success path here). A nonzero count with the environment available is red |

## Evidence

<!-- The delivery + independent verification recorded below ran in the stream's source
     tree before this public re-home; the measured matrix and freshness registration are
     part of the sequenced research-doc + tool de-house follow-on (see the stream README
     re-home note). The Verify table above is the executable spec; the rows re-run there. -->

**Delivery & verification (in the source tree, before this re-home).** The matrix
(`docs/research/codex-harness-capabilities.md` (planned) here) landed with all ten
required capability rows, each carrying a closed-vocabulary verdict, plus the
`freshness.yaml` registration (`max-age-days: 45`, `upstreams: []`). An independent
non-implementer verifier re-ran every runnable row against merged content — **VERIFY:
PASS** (`gate: model`, all four risk answers `no`): rows 1, 2, 2a, 3, 4 and 4a green
(row 4a force-ages the file past its 45-day leash and its own line flips to `STALE`,
proving the leash is a live check). Rows 5–6 stay **BLOCKED (needs live Codex)** — no
live Codex environment exists; that environment is the stream's external head, Ian to
provide or sanction, and a BLOCKED row is never counted a pass.

**Risk-bearing value.** The only irreversible-in-spirit literal is the freshness leash:
`max-age-days: 45` — a deliberately short leash (~1/3 of the ~135-day staleness window
that motivated the brief), forcing re-measurement well before the third-party facts rot
again. All reversible operational knobs; a wrong leash only shifts the re-measurement
reminder.

## Review

Gate: **model** (from frontmatter). The reviewer checks the matrix's verdicts cite a
probe or a URL+date each, that no `absent` verdict lacks its probe, and that prior-art
claims were re-verified rather than copied — the stale-inheritance failure is the one
this brief exists to prevent.
