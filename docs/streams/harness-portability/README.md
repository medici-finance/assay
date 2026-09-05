---
stream: harness-portability
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# Harness Portability Stream

Make the Assay platform run **natively on Codex (OpenAI's harness) and Cursor as well as
Claude Code** — first-class additional harnesses, not lowest-common-denominator ports.
Today the method's delivery mechanism is single-vendor: the plugin manifest, the
SessionStart hook that injects the resident rules, the `SKILL.md` discovery contract, and
the tool names inside the skill bodies are all Claude Code shapes. That is a hard ceiling
on adoption for an open-source methodology whose whole claim is that the *method* matters,
not the model — and a coherence problem: Assay's own positioning describes it as sitting
"underneath whatever agent harness a team already runs" while the delivery mechanism
contradicts it.

> **Re-home note (2026-08-26).** This planning record is re-homed to public
> `medici-finance/assay` — harness portability is non-commercial adopter tooling, so its
> stream and per-harness binding files belong in the public repo (Ian's ruling). The
> briefs' **code** deliverables — the neutral-core skill bodies, the resident-rules single
> source, and the `tools/harnessgen`/`tools/harnesslint` generators plus the generated
> per-harness packaging under `plugins/assay/{codex,cursor,resident}/` — are a **sequenced
> follow-on de-house** (the same shape `statusgen` and `desk-tools` followed: source →
> public, then the source tree consumes the released binary). Until that lands, the Verify
> tables' generator/lint commands run in the tool's source tree. Statuses below reflect the
> completed planning/implementation work.

**What is measured, not assumed** (2026-08-07):

- `statusgen` and the `tools/**` binaries are plain Go CLIs invoked by argv — nothing
  harness-specific. **Out of scope; nothing to do.**
- The Claude-shaped surface is exactly: `plugins/assay/.claude-plugin/plugin.json` +
  root `.claude-plugin/marketplace.json` (manifest format); `plugins/assay/hooks/hooks.json`
  + `inject-resident-rules.sh` (**SessionStart is the only hook Assay ships** — it is how
  the resident rules arrive in every session, the load-bearing mechanism); the
  `skills/<name>/SKILL.md` frontmatter/discovery contract; and harness-specific language
  inside skill bodies (backticked `Agent`/`SendMessage` in 2 files; `subagent|dispatch|worktree`
  touchpoints: batch-fanout 32, verify-desk 18, pr-review-desk 14, the-desk 11,
  author-brief 3, market-intelligence 1, adopt 0).
- Root `AGENTS.md` is **repo-local by ruling** (a maintainer ruling, 2026-08-03): it carries
  this repo's mechanics, and adopters get the method through the bundle, never this
  clone. It **complements** a resident-rules file; it is not the delivery vehicle and
  this stream does not repurpose it. The Codex delivery artifact is a *bundle-shipped
  AGENTS.md fragment* installed into the adopter's repo (brief 05), not this file.
- **Prior art exists**: `superpowers` 6.2.0 (upstream `github.com/obra/superpowers`)
  ships the same skill payload to five harnesses — per-harness manifest dirs
  (`.codex-plugin/`, `.pi/`, `.kimi-plugin/`, `gemini-extension.json`), per-harness
  reference files (its "codex-tools" reference under `using-superpowers` et al.),
  read-only environment detection inside skill bodies, and a deterministic sync script with
  tests. Its empirical Codex findings are dated **2026-03-23** — ~4.5 months stale, which is
  why brief 01 re-measures rather than inherits.

**Update 2026-08-11 — ACP re-scopes the dispatch half, not this stream.** The Agent
Client Protocol (Zed's open JSON-RPC standard for driving coding agents) gives the
conductor loop a standard way to drive *any* ACP agent as a dispatched worker — so "the
governed drain loop runs on a non-Claude runner" becomes a tier→runner config row and no
longer waits on this stream. What this stream owns is unchanged and still required:
delivery of the method text itself (resident rules, skill discovery, the capability
vocabulary inside skill bodies) into an adopter's *interactive* sessions on a second
harness. Two consequences for the briefs: (1) brief 03's target/channel ruling should weigh
ACP-capable agents as an additional delivery row alongside native Codex packaging;
(2) the external head below still gates brief 01's behavioural rows and 07's smoke run
(native-skills parity), but it no longer gates dispatch-layer vendor neutrality — that
claim is carried by the ACP spec's phasing instead.

## The seam — decided here, not deferred

**Chosen: a three-layer split — neutral-core method text, thin per-harness binding
files, generated per-harness packaging.** This is superpowers' shape, adapted to Assay's
drift tooling:

1. **METHOD (single source):** the seven skill bodies and the resident rules are written
   in harness-neutral *capability vocabulary* — `dispatch-worker`, `message-agent`,
   `isolate-workspace`, `invoke-skill`, `session-notifications`, `durable-monitor`, `stop-worker` — never
   harness tool names. One text, every harness reads the same method.
2. **BINDINGS (one small file per harness):** `plugins/assay/references/<harness>.md`
   maps each capability to that harness's mechanism (Claude: the `Agent` tool,
   `SendMessage`, background task notifications; Codex: `spawn_agent`/`wait_agent`/
   `close_agent` behind the `multi_agent` config flag, sandbox constraints; Cursor:
   background agents in isolated git worktrees, `cursor-agent` headless) **and carries
   the degradation ruling per skill**. Hand-authored because the content is empirical
   harness behaviour — and therefore freshness-registered (see drift, below).
3. **PACKAGING (generated):** per-harness manifests (`.codex-plugin/`, the Codex
   AGENTS.md fragment, the Cursor `.cursor/rules` rule, the Claude SessionStart payload)
   are *generated* from single sources by a Go tool, byte-compared in CI. Derived
   artifacts are never hand-ported.

**Ruled out, with reasons:**

- **Per-harness skill forks / parallel bundles.** Doubles the exact drift surface that is
  already this repo's live failure: the bundled skills are a port of an upstream source and
  `plugindrift` measured them **tens of commits behind** at the time, detected and
  never re-ported. Two hand-maintained copies of the method text is the one shape with
  an existing body count.
- **A full capability-declaration runtime layer** (skills target an abstract machine-read
  API). Over-engineered at n=2 harnesses: skill bodies are prose read by a model, not
  programs — a shared vocabulary plus a binding file the model reads gets the same
  effect with no machinery nobody executes. Superpowers demonstrates reference files
  suffice at n=5.
- **Lowest-common-denominator single text** (no harness specifics anywhere). Loses the
  load-bearing detail — the exact config flag that enables dispatch on Codex, the exact
  sandbox limits — and pushes the mapping onto every session at run time, which is how a
  method fails to arrive.

## What "natively" means — and what it explicitly does not

On a supported harness, an adopter gets all four, or the gap is stated in-session:

1. **Resident rules arrive at session start without manual pasting**, via that harness's
   native channel — Claude: SessionStart hook; Codex: an AGENTS.md fragment the adopt
   flow installs into the adopter's repo (Codex reads AGENTS.md natively); Cursor:
   `AGENTS.md` natively or a generated `.cursor/rules` always-apply rule.
2. **All seven skills are discoverable and invokable** through the harness's native
   skill mechanism. If a harness has no description-driven auto-trigger, invoke-by-name
   is the floor and the binding file says so — parity of *availability*, not of
   trigger ergonomics.
3. **Ruled degradation, never silent.** Each skill on each harness is classed
   `runs / degrades / refuses` (ratified in brief 03). The non-degradable guarantees are
   isolation, evidence, and the review gates: a harness that cannot isolate an
   implementer **refuses** the fanout rather than working in the shared checkout; a
   harness without multi-agent dispatch runs `batch-fanout` **serially, one brief at a
   time, stating the degradation** — the guarantees survive, the parallelism doesn't.
4. **Install is a documented adopt path** (the `adopt` skill + `docs/adopting-assay.md`),
   not hand-copying.

**Out of scope for this stream:** other harnesses (Gemini, Pi — the seam permits them;
no artifacts are authored). **Cursor is now authored as the third harness column
(brief 12, 2026-08-26)** — Ian ruled it in (target both surfaces, headless-first); it
reuses this stream's seam exactly (capability vocabulary, binding file, `harnessgen`
verb, degradation floor) rather than a new stream, proving "only this table's row
changes." The desk-tools/statusgen Go binaries (already harness-agnostic, measured);
deskd and the Desk Console (separate product surface); hook classes beyond SessionStart
(Assay ships none — `hooks.json` contains exactly one event); **publishing into OpenAI's
plugin marketplace** (`openai-codex-plugins`) — that is a public act behind the
publication review and brief 03's channel ruling, not here.

## How drift is mechanically caught — four mechanisms, no documentation-only answers

The repo's live instance of this failure class, measured 2026-08-07 via `plugindrift`:
`the-desk`, `batch-fanout`, `verify-desk`, and `pr-review-desk` were each measured **tens
of commits behind** their upstream source at the pinned commit; `author-brief`'s primary
source (a local-git `~/.claude` checkout) permanently UNREACHABLE. Detected on every run;
never re-ported. A second harness must not double this surface:

1. **Root-cause the existing instance (brief 02):** re-sync once, then **flip the
   authority** — the bundle becomes the canonical home of the method text (enacting the
   ratified "bundled skill is authoritative on the METHOD" ruling and AGENTS.md's stated
   trajectory), and the upstream copies become thin pointers. One source ends the port
   relationship; there is nothing left to re-port.
2. **Derived artifacts are generated, never ported (briefs 05, 06):** the Codex
   packaging, the AGENTS.md fragment, and the Claude SessionStart payload are emitted by
   `tools/harnessgen` from single sources; a CI check regenerates and fails on any diff
   (the STATUS.md single-writer pattern). Drift between source and artifact becomes
   *impossible*, not detected.
3. **Hand-authored empirical files rot on a clock (briefs 01, 07):** the per-harness
   binding files and the capability matrix are registered in `freshness.yaml` with
   `max-age-days` + upstreams; `tools/freshness` (existing) flags them stale on cadence.
   Harness behaviour changing under us is caught by expiry, not by an incident.
4. **Coverage is closed (brief 06):** every `skills/*/SKILL.md` must be accounted for in
   the generator's map or an explicit exclusion with a reason — an unaccounted file is a
   hard error (the SOURCES.yaml coverage rule, ported to the new tool).

## Briefs

Status column carries the **planning/implementation** state (the work the brief describes
was completed in the source tree). The **Verified**/**Reviewed** columns are unset here
because this public board's own verify + review gates have not yet run against the re-homed
record — that is a follow-on, not a claim this re-home makes. Statuses therefore read
`implemented` (work done, public-board verification pending) rather than `done`.

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Codex capability ground-truth — measured matrix, not inherited prior art](./brief-01-codex-capability-ground-truth.md) | 0 | M | implemented | — | — |
| 02 | [Kill the drift debt — re-sync the bundle, flip the canonical home](./brief-02-drift-debt-authority-flip.md) | 0 | L | implemented | — | — |
| 03 | [Ruling: target harnesses, delivery channel, degradation matrix](./brief-03-target-channel-ruling.md) | 1 | S | implemented | — | — |
| 04 | [Neutral-core skill bodies + per-harness binding files + neutrality lint](./brief-04-neutral-core-skills.md) | 2 | L | implemented | — | — |
| 05 | [Resident rules: one source, per-harness delivery generated](./brief-05-resident-rules-single-source.md) | 2 | M | implemented | — | — |
| 06 | [Codex packaging — generated manifest, coverage rule, install path](./brief-06-codex-packaging.md) | 3 | M | implemented | — | — |
| 07 | [Adoption docs, freshness registration, live Codex smoke protocol + first run](./brief-07-adoption-live-smoke.md) | 4 | M | implemented | — | — |
| 09 | [jcode desk-harness spike — measured parity + fleet-density for driving desks](./brief-09-jcode-desk-harness-spike.md) | 0 | L | implemented | — | — |
| 10 | [SpecMem portable-memory spike — one stream's registers across two harnesses](./brief-10-specmem-portable-memory-spike.md) | 0 | M | implemented | — | — |
| 11 | [Durable-monitor capability + residual harness-token prose-audit](./brief-11-durable-monitor-capability.md) | 3 | M | done | 2026-09-05 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #475 @ ebbb6c6a828080a29ccef26b2707c49730871513) |
| 12 | [Cursor — the third harness column (ground-truth + binding + generator verb + public column)](./brief-12-cursor-third-column.md) | 5 | L | implemented | — | — |
| 13 | [Cursor live-desk-smoke protocol + first run](./brief-13-cursor-live-desk-smoke.md) | 6 | M | todo | — | — |

**Note on 07:** artifacts delivered (adoption docs, freshness registration, smoke
protocol). The live Codex smoke run itself is held — it needs a Codex environment (OpenAI
account + install), the external true head; Ian provides.

**Note on 13:** brief 12 built the structural Cursor column but explicitly scoped the live
proof out as a separate gate:human acceptance step mirroring 07's; no brief owned that step
until 13. 13 is not on the Codex critical path and does not gate 01–07 — it is the Cursor
column's own equivalent of 07, one wave after 12, and it is what turns "Cursor is
structurally supported" into "a full desk loop has run on Cursor." Blocked the same way 07
is: it needs a live Cursor install, Ian provides/sanctions.

## Critical path

```
[EXTERNAL HEAD: a live Codex environment — an OpenAI account plus a Codex CLI (and,
 if 03 rules it in, Codex App) install that Ian provides or sanctions. No such
 environment is known to exist in-house today. Without it, brief 01's behavioural
 rows are BLOCKED, 03 rules on stale prior art, and every "works on Codex" claim
 downstream is design-on-assumption. Structural work (02) proceeds in parallel.]
                      |
        01 ground-truth ──► 03 ruling (Ian) ──► 04 neutral core ──► 06 packaging ──► 07 adoption + live smoke
                                             ▲                  ▲
        02 re-sync + authority flip ─────────┘ 05 resident rules┴──────────────────► (07)
```

**In-stream head: 01.** Longest chain is `01 → 03 → 04 → 06 → 07`. 01 is at the head
because every downstream judgement — the binding file's content, the degradation
matrix, the packaging shape, even whether Codex-side skills auto-trigger at all — binds
to capability facts that currently exist only as superpowers' 2026-03-23 measurements.
Designing the seam against 4.5-month-old third-party facts rebuilds the drift failure
at design time.

**True head, and it is not a brief: the Codex environment.** Brief 01 can complete its
documentary half (prior-art extraction, public-docs sweep) without it, but its Verify
table marks every live-harness row **blocked** until the environment exists. Ian
providing/sanctioning that environment is the smallest unblocking move for the whole
stream.

### Tempting-but-wrong first steps

- **Starting at 04 (neutralize the skills).** Without 02, the rewrite lands on text that
  is tens of commits behind its source; the overdue re-sync then overwrites or conflicts
  with every neutralized line. Without 01/03, the capability vocabulary binds to assumed
  Codex behaviour. 04 is the biggest brief, which makes it feel like the work — it is
  wave 2 for a reason.
- **Starting at 06 (copy superpowers' `.codex-plugin/` now).** Packaging an
  un-neutralized bundle ships skills that instruct Codex to call tools it does not
  have. The manifest is the easy 10%; it is last-but-one deliberately.
- **Inheriting superpowers' findings instead of re-measuring.** Its Codex facts are
  dated 2026-03-23 (sandbox behaviour, `multi_agent` flag, App finishing flow). Anything
  may have moved; 01 exists to find out.

## Dependency waves

```
Wave 0: [01, 02, 09, 10]   (09 jcode + 10 SpecMem: independent evaluation spikes Ian asked for, 2026-08-16; each INFORMS — does not gate — the harness-target ruling (03), so neither carries an `unblocks: 03`; not on the Codex critical path)
Wave 1: [03]←01
Wave 2: [04]←{02,03}, [05]←{01,03}
Wave 3: [06]←{03,04,05}, [11]←04
Wave 4: [07]←{05,06}
Wave 5: [12]←{03,04,05,06}
Wave 6: [13]←12
```

Critical path: `ext(Codex env) → 01 → 03 → 04 → 06 → 07`. 02 runs parallel in wave 0 and
feeds 04; 05 runs parallel with 04 in wave 2. 11 (the residual harness-token prose-audit
+ durable-monitor capability the TOKEN-only 04 lint cannot catch) follows 04 in wave 3,
parallel to 06; it strengthens the bodies 06 packages and SHOULD land before 06 so no
un-honorable `Monitor` prose is shipped, but it is modeled parallel since 06 already gates
on 04's neutral core rather than carrying 11 in its `depends`. 12 (Cursor, the third
harness column) reuses the whole seam and lands after the Codex chain (wave 5). 13 (the
Cursor live-desk-smoke protocol + first run — this stream's Cursor-side equivalent of 07)
follows 12 in wave 6; like 07 it does not extend the Codex critical path.

## Gate distribution — derived, not spread

**03, 07, and 13 are `gate: human`**: 03 because only Ian can commit the target set and a
delivery channel whose marketplace option interacts with the one-way publication; 07 and 13
because their acceptance evidence is a live session on the second and third harness
respectively — an act outside CI that a human runs or sanctions. All other briefs answer the
four risk questions `no` and gate `model` — nothing here touches funds, customers, regulators,
or an irreversible surface; everything is git-revertible text and tooling. (Brief 02
declares one out-of-repo file under the rule-7 protocol; that is a serialization
constraint, not a risk gate.)

## Cross-repo and out-of-repo dependencies (facts, not `depends:`)

`depends:` arrays are in-repo only. The desk sequences these at dispatch:

| External | Relationship | Note |
|---|---|---|
| Live Codex environment (OpenAI account + install) | **True head** | Blocks 01's behavioural rows and 07's smoke run; Ian provides |
| Live Cursor environment (install) | **Head for 12, 13** | Blocks 12's live-confirm rows and 13's live-desk-smoke run (its full-loop acceptance step); Ian provides |
| The upstream `.claude/skills/{the-desk,batch-fanout,verify-desk,pr-review-desk}` copies | **Sibling PR (02)** | The authority flip converts them to thin pointers; both PRs cite each other's SHA |
| `~/.claude/skills/author-brief/SKILL.md` | **Out-of-repo file (02)** | Declared per rule 7: one in flight, applied last, committed in the `~/.claude` stopgap repo |
| The publication review (manifest + gate) | **Gates the marketplace channel only** | The in-bundle install path ships without it; nothing in this stream publishes |
| `github.com/obra/superpowers` | **Prior art only** | Read, not depended on; its findings are re-measured in 01 |

## Shared conventions

- **Capability vocabulary is closed**: the seven capability names above are the whole
  set until a brief amends this README. A skill body naming a capability not in the set
  is a lint error (04). The block below is the **machine-readable** copy of that set —
  `tools/harnesslint` reads it from here, so amending the vocabulary means editing this
  block in the same PR (the lint reads the set from one place, this one). Keep it in step
  with the prose list in "The seam" above. (`stop-worker` was added by desk-supervision/02
  — the desk window's cadence sweep stops a dispatched worker whose per-run stop is armed.)

<!-- assay:capability-vocabulary
dispatch-worker
message-agent
isolate-workspace
invoke-skill
session-notifications
durable-monitor
stop-worker
-->

- **Blocked is a state, not a failure**: Verify rows requiring the live harness are
  marked `BLOCKED (needs live Codex)` in Evidence with the reason — never run
  vacuously, never greened from prior art.
- **Every absence-assertion grep in this stream pairs a positive control**: a zero with
  no control is not evidence.
- **Nothing in this stream edits root `AGENTS.md`** — it is repo-local by ruling; the
  Codex delivery fragment is a separate bundle-shipped artifact.
