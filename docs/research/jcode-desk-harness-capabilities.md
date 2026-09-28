# jcode desk-harness capabilities — parity matrix + fleet-density read

**Spike (a) of the two-spike jcode/SpecMem evaluation requested 2026-08-16: can jcode drive the desks?**
**Authored for**: harness-portability/09
**Provenance**: authored 2026-08-24 in the stream's pre-re-home source tree and ported here 2026-09-27, when the stream's re-home to this repository was found to have carried the brief but not this file. The findings are unchanged; only references that do not resolve in this repository were removed or reworded.
**Date**: 2026-08-24 | **Environment**: NO live jcode (offline spike — see "Measurement status" below)
**Subject**: `1jehuang/jcode` (MIT, Rust), a terminal coding-agent harness. Docs retrieved 2026-08-24.

---

## Measurement status — read this first

The brief asks for **measured** parity and **measured** fleet-density: install jcode, run one desk
skill under it, and record real per-session RSS + cold-boot **on our workload**. This author ran
under an **offline envelope** — no external-harness install, no cluster contact — so the live half is
**not** performed here. This document therefore delivers, honestly split:

- **AUTHORED (offline-complete, decision-grade):** the required-primitive set (derived from *our*
  desks, §2), the per-primitive **documentary** verdict against jcode's public docs with the exact
  live probe that must confirm it (§3), the exec-tier probe *design + documentary lean* (§4), the
  full **prose-vs-discrete design sketch** for intake-desk (§5, entirely about our desks), the
  density-measurement **methodology** with the exact commands to run (§6), and a **conditional**
  go/no-go read (§7).
- **BLOCKED (needs a live jcode install to close):** every row's live confirmation, the
  our-workload RSS/boot numbers, and the live exec-tier verdict. These are marked
  `BLOCKED (needs live jcode)` per the stream's "Blocked is a state, not a failure" convention — never
  greened from vendor copy, never fabricated.

**Vendor numbers appear in this doc only in a clearly-labelled column and are NOT inherited as
findings** — the brief is explicit that density must be measured on our workload, not read off the
vendor's synthetic figure. Where a vendor claim is quoted, its source URL and the vendor's own
methodology caveat are quoted with it.

---

## 1. What jcode is (documentary)

A Rust-native terminal coding-agent harness (no Electron/Node runtime). Headline pitch: an
order-of-magnitude smaller resident footprint than Claude Code, multi-provider model routing, a
persistent semantic-memory layer, and an autonomous "swarm" multi-agent primitive.

| | jcode |
|---|---|
| **Language / runtime** | Rust, single native binary (no Node) |
| **Install** | `curl -fsSL https://jcode.sh/install \| bash`; `brew tap 1jehuang/jcode && brew install jcode`; `cargo build --release` (MIT) |
| **Model backends** | Native Claude / OpenAI / Gemini / Copilot / Azure / Alibaba; aggregators OpenRouter, custom OpenAI-compatible; **Anthropic-compatible gateways via `type = "anthropic-compatible"`** with bearer/custom-header auth — matches the endpoints the cells use |
| **Model selection** | `/model` picks the model **per session / per provider profile** (`--provider-profile`). No per-child/per-task model field documented (see §4) |
| **MCP** | Reads `~/.jcode/mcp.json` + `.jcode/mcp.json`, and **for compatibility reads Claude Code's `~/.claude.json` / `.mcp.json`**. `"mcpServers"` with **stdio** servers; **HTTP/SSE entries are recognized but skipped** |
| **Memory** | Persistent cross-session semantic memory: `jcode-embedding` (local ONNX vectors), `jcode-memory-types`, memory graph queried by cosine similarity, consolidated by a memory side-agent |
| **Swarm** | `jcode-swarm-core` crate; agents spawn teammates via a swarm tool; DM / broadcast / repo-only inter-agent messaging; vendor claims 10+ parallel streams |
| **Isolation** | **None today.** Roadmap explicitly: *"git worktrees is not a good solution"* — a replacement primitive is *planned, not implemented* |

Sources: §Appendix A. jcode is early and fast-moving (vendor benchmarks pin `v0.9.1888-dev`; a
current release `v0.54.4` is noted as differing) — hence freshness registration (§8).

---

## 2. What the desks require of ANY driving harness (derived from our repo — authored)

The load-bearing set, taken from the brief's `facts` and confirmed against
`plugins/assay/skills/*/SKILL.md` and the stream README. These are the primitives a harness must
supply to *drive a desk*, independent of jcode:

| # | Primitive | Why the desks need it | Non-degradable? |
|---|---|---|---|
| P1 | **Tool surface — shell CLIs** | The desk tools (`statusgen`, `deskroster`, `deskboard`, `deskfile`, `deskpr`, …) are plain argv CLIs invoked from a **bash/shell tool**, not MCP servers. Any harness with a shell tool runs them unchanged (measured harness-agnostic, stream README) | availability required |
| P2 | **Tool surface — MCP (browser)** | `claude-in-chrome` is the one true MCP surface a desk uses (UI estate / live-verify). Needs an MCP client **or** a native browser-automation equivalent | degradable (UI-verify only) |
| P3 | **Resident-rules injection at session start** | Assay ships exactly ONE hook — SessionStart — and it is *the load-bearing delivery mechanism*: the resident rules (AGENTS.md content, the register/evidence discipline) must arrive in **every** session **without manual paste**, deterministically | **non-degradable** (method must arrive) |
| P4 | **Persistent memory equivalent** | The desks carry state across sessions (registers, MEMORY, claim locks). A durable memory the model can query | degradable (git registers already carry the durable state) |
| P5 | **Background / parallel sub-agents** | `worker-desk` fans out **8** workers; intake arms a persistent monitor; pr-review-desk runs a PR monitor. Needs spawn + message + completion-signal | degradable → **serial with stated degradation** (stream ruling) |
| P6 | **Workspace isolation the writeguard assumes** | Every implementer works in its **own git worktree**; the writeguard + deskkit refuse a shared-checkout write. The stream's own ruling: a harness that **cannot isolate an implementer refuses the fanout** rather than working in the shared checkout | **non-degradable** (isolation is a hard guarantee) |
| P7 | **Per-task model binding (exec-tier)** | Not a desk *requirement* but the brief's first-class **adoption reason**: can `exec-tier: strong/weak` become a runtime guarantee (a strong parent spawning a weak child, model chosen from a per-task field) rather than the honour-system annotation it is today? | adoption lever, not a gate |

Non-degradable = P3, P6 (and the review gates, out of scope here). A harness that misses one of
those **refuses**, it does not silently degrade.

---

## 3. Parity matrix — per primitive (documentary verdict + BLOCKED live probe)

Verdicts: `present` / `workaround` / `absent` / `unmeasured`. The **documentary** verdict is from
public docs (cited); the **live** confirmation is BLOCKED offline and names the exact probe.

### P1 — shell-CLI tool surface
- **Documentary verdict: `present`.** jcode ships 30+ built-in tools incl. shell execution; the
  desk CLIs are argv binaries invoked via that shell, exactly as under Claude Code. Nothing
  harness-specific in `statusgen`/`desk*` (stream README, measured 2026-08-07).
- **Live probe (BLOCKED — needs live jcode):** `jcode` session → `statusgen --root . --scan-issues
  --dry-run` via the shell tool; expect the same ACTION lines a Claude session prints.
- *Positive control:* a harness lacking a shell/exec tool would surface no way to invoke an argv CLI
  — jcode's documented `shell execution` tool is that control.

### P2 — MCP (browser) tool surface
- **Documentary verdict: `workaround`.** jcode has an MCP client and **reads Claude's own
  `~/.claude.json` / `.mcp.json`**, so a **stdio** MCP server is picked up directly. BUT **HTTP/SSE
  MCP entries are recognized and skipped** — if `claude-in-chrome` is wired over HTTP/SSE in a cell
  it will not load, and the fallback is jcode's **native browser-automation** tool (different tool
  surface, so the skill body's `mcp__claude-in-chrome__*` names would not resolve 1:1).
- **Live probe (BLOCKED):** point jcode at the cell's `.mcp.json`; `jcode` → list tools; check
  whether `claude-in-chrome` tools appear (stdio) or must be replaced by jcode's native browser tool.
- *Positive control:* a stdio echo MCP server placed in `.mcp.json` must appear in jcode's tool list;
  if even that is absent, MCP support is broken, not just HTTP/SSE.

### P3 — resident-rules injection at session start  ← load-bearing
- **Documentary verdict: `workaround` (leaning absent for the *guarantee*).** jcode has **no
  documented SessionStart-equivalent hook** that deterministically injects a file every session. An
  `AGENTS.md` exists in the jcode repo but its load semantics are not documented, and skills are
  **"not all loaded on startup" — injected on demand by semantic-embedding similarity.** On-demand
  semantic injection is **not** the always-on, every-session guarantee SessionStart provides — the
  resident rules could simply not be retrieved for a given turn. Candidate workarounds to confirm
  live: (a) a jcode `AGENTS.md` that loads unconditionally like Codex's; (b) a "prompt overlay"
  (docs reference a "Skills System and Prompt Overlays" section) pinned to always-load; (c) a memory
  seed. None is confirmed to be *unconditional*.
- **Live probe (BLOCKED):** put the resident-rules text in each candidate channel; start a fresh
  jcode session; ask a question whose only correct answer depends on a resident rule; confirm the
  rule is present **without** having referenced it. Repeat across ≥5 cold starts — the guarantee is
  *every* session, not *most*.
- *Positive control:* under Claude Code the SessionStart hook injects on 100% of cold starts
  (`plugins/assay/hooks/hooks.json` — one event, SessionStart); jcode must match that hit rate to
  count as `present`, not `workaround`.

### P4 — persistent-memory equivalent
- **Documentary verdict: `present` (arguably richer than Claude's).** `jcode-embedding` (local ONNX)
  + a memory graph queried by cosine similarity + automatic consolidation via a memory side-agent +
  session-search RAG. This exceeds the flat MEMORY.md file the desks use today. **Cost caveat:** the memory
  layer is what moves resident RSS from the 27.8 MB *embeddings-off* headline to **167 MB
  embeddings-on** (§6) — and the desks *need* the memory, so the honest density figure is the
  embeddings-on one.
- **Live probe (BLOCKED):** store a fact in session 1, restart, query it in session 2; confirm recall
  and record RSS with embeddings on.
- *Positive control:* a harness with no cross-session store returns nothing on the session-2 query.

### P5 — background / parallel sub-agents
- **Documentary verdict: `present`.** `jcode-swarm-core`: spawn teammates via a swarm tool, DM /
  broadcast / repo-only messaging, vendor claims 10+ parallel streams — covers worker-desk's fan-out
  of 8 and the messaging pr-review-desk/verify-desk use. (Depth of the completion-signal / durable
  resume across process restarts is **unmeasured** — the Codex sibling doc found exactly that class
  of resume bug, so probe it.)
- **Live probe (BLOCKED):** spawn one child agent, message it, confirm a completion signal reaches
  the parent; kill and resume the parent, confirm the child is still addressable.
- *Positive control:* a single-threaded harness exposes no spawn tool; jcode's swarm tool is that
  control.

### P6 — workspace isolation the writeguard assumes  ← non-degradable, and the blocker
- **Documentary verdict: `absent` (today).** jcode's **roadmap explicitly rejects git worktrees**
  (*"git was clearly not built for multi-agent workflows, and git worktrees is not a good
  solution"*) and its planned isolation primitive is **not yet implemented**. The desks' hard rule
  (writeguard, stream README) is that an implementer works in its **own worktree** or the
  fanout **refuses**. With no worktree isolation, jcode's swarm agents share one working tree — the
  exact shared-checkout write the guards are built to stop.
- **Live probe (BLOCKED):** spawn two swarm agents, have each write a different file, check whether
  they collide in one tree or are isolated; attempt the writeguard's own refusal path.
- *Positive control:* under Claude Code, `git rev-parse --show-toplevel` differs per worker worktree;
  jcode must show the same per-agent separation to count as `present`. **This is the primary
  parity gap that blocks desk-driving (§7).**

### P7 — per-task model binding (exec-tier) — see §4.

**Documentary tally (NOT a substitute for the live matrix):**

| Verdict | count | primitives |
|---|---|---|
| `present` | 3 | P1, P4, P5 |
| `workaround` | 2 | P2, P3 |
| `absent` | 1 | **P6 (isolation)** |
| per §4 | 1 | P7 |

---

## 4. Exec-tier probe — design + documentary lean (authored; live verdict BLOCKED)

**The question (brief fact):** today `exec-tier: strong/weak` is an unenforceable honour-system
annotation — statusgen never checks which model actually ran. Can jcode make it a **runtime
guarantee** by binding a model **per task / per child-agent** (a strong parent spawning a weak
child, model chosen from a per-task field)? A yes is a first-class adoption reason.

**Documentary lean: NO / per-session-only (pending live confirmation).** Across the docs swept,
model selection is `/model` **per session** and `--provider-profile` **per session/provider**. The
swarm docs describe spawning teammates and inter-agent messaging but **do not document a per-child
model override field** — the same shape as the Codex sibling finding (openai/codex issue 32674:
per-child model overrides blocked). No `model:` field on the swarm-spawn call is documented.

**Probe design (BLOCKED — needs live jcode):**
1. Configure two provider profiles — a strong and a weak/cheap model.
2. Start a strong-model parent session; via the swarm tool spawn a child and attempt to pin the
   child to the weak profile **from the spawn call** (look for a `model` / `profile` argument on the
   swarm-spawn tool; inspect `jcode-swarm-core`'s spawn schema).
3. Verify from provider-side telemetry / the child's self-report which model actually served the
   child's turns.
4. **Verdict rule:** `runtime-guarantee` only if the child demonstrably ran the weak model chosen at
   spawn time; `per-session-only` if the child inherits the parent/global `/model`; `absent` if no
   per-agent model control exists at all.

**Consequence for adoption:** if the live probe confirms per-session-only, jcode does **not** convert
`exec-tier` into a runtime guarantee — it removes one of the two headline adoption reasons, leaving
density + the discrete-meld (§5, §7) to carry the case.

---

## 5. Prose-vs-discrete design sketch — intake-desk (authored, offline-complete)

The brief's "meld into jcode" question: the desks are mostly **deterministic mechanics narrated in
prose SKILL.md and executed by a general LLM** (expensive, non-deterministic). Sketch — for the
lightest real desk, **intake-desk** (chosen per the brief) — which loop parts become **discrete
jcode-native code/tools/config** vs which **stay LLM-judgment**. This is a redesign sketch, **not an
implementation** (nothing built here). Derived from `plugins/assay/skills/intake-desk/SKILL.md`.

| intake-desk loop step | Today (prose + general LLM) | Redesign | Where it lands under jcode |
|---|---|---|---|
| Boot: prune stale worktrees, lock session worktree | LLM runs shell steps narrated in §Boot | **DISCRETE** | jcode-native startup task / config; deterministic |
| Boot: `deskroster preflight` (5-check envelope) | LLM invokes CLI, reads result | **DISCRETE** | already a CLI (P1) — a jcode "tool" wrapper, no judgment |
| Board read: `statusgen --scan-issues --dry-run` + intake-debt lint | LLM invokes, reads the ACTION list | **DISCRETE** | CLI behind a jcode tool; the ACTION list is deterministic |
| Arm inbound monitor (`inbound-monitor.sh`, ~60s) | LLM narrates arming a Monitor | **DISCRETE** | jcode-native durable background task / timer; the monitor is already a script |
| CREATE-PLACEHOLDER (scan → branch → draft PR) | LLM runs the git+scanner+PR sequence | **DISCRETE** | scripted; the only variable is the PR body text |
| CLOSE-ON-FIX-LANDED (check timeline, close on merge) | LLM checks PR state, closes issue | **DISCRETE** (state machine) | deterministic rules ("PR merged → close; approved-not-merged → comment; draft → leave") — pure config/code |
| RETIRE closed-issue placeholders | scanner step | **DISCRETE** | scripted |
| System-label exclusion (`verify-gate`/`needs-decision`) | prose rule | **DISCRETE** | a filter list in config |
| **Routing test** ("hand to a worker as-is = issue; needs judgment = intake") | LLM classifies | **STAYS LLM-JUDGMENT** | the load-bearing triage call — non-deterministic by nature |
| **Four triage exits** (`scoped→stream` / `scoped→issue` / `decision-needed` / `rejected/watching`) | LLM decides the exit + reason | **STAYS LLM-JUDGMENT** | classification + reason authoring |
| **`needs-decision` issue authoring** (Situation / 2–4 Options / consequences) | LLM writes the decision doc | **STAYS LLM-JUDGMENT** | design-tier authoring |
| **Scope an idea toward a brief** | LLM (author-brief tier) | **STAYS LLM-JUDGMENT** | design-tier |
| Trust-gate (blessed identity check) | prose rule + LLM | **DISCRETE** check, **LLM** only on ambiguity | roster lookup is code; the edge case is judgment |

**The split, named:** the **mechanical spine** — preflight, board reads, the scan→PR carrier, the
close-on-merge state machine, the monitor, label filters, the trust-roster lookup — is **discrete**
and becomes jcode-native code/tools/config, executed deterministically and cheaply (and, notably,
**this is the part that does not need a strong model at all**). What **stays LLM-judgment** is the
irreducible core the desk exists for: the **routing test**, the **five-exit classification**,
**decision-issue authoring**, and **idea→brief scoping** — the calls the intake-desk's own "run this
on a SMART model" rule already fences off. The redesign does not shrink the judgment; it stops paying
a general LLM to *narrate* the mechanics around it. Same shape would apply to the other desks
(deskboard reads, guard checks, token-mint, the drain loop = discrete; triage/verdict/decision
authoring = LLM). **Sketch only — not implemented in this spike.**

---

## 6. Fleet-density — methodology + vendor claim (our-workload numbers BLOCKED)

**Vendor self-benchmark (documentary — NOT our workload, NOT inherited as a finding):**

| Metric (PSS) | jcode (embed off) | jcode (embed on) | Claude Code | Codex CLI |
|---|---|---|---|---|
| 1 session | 27.8 MB | **167.1 MB** | 386.6 MB | 140.0 MB |
| 10 sessions | 117.0 MB | ~261 MB | 2300.6 MB | 334.8 MB |
| marginal / added session | ~9.9 MB | — | ~212.7 MB | ~21.6 MB |
| boot (median time-to-first-frame) | 14.0 ms | — | 3436.9 ms | 882.8 ms |

Source: jcode README, quoted via explainx.ai (2026-07), **Linux, PSS, pinned `v0.9.1888-dev`**.
**Vendor's own caveat, quoted:** *"Treat as author self-benchmarks, not independent lab results"*;
the source **does not specify** the hardware, whether tools were loaded, the workload (idle vs task),
or the exact PSS command. **Two things this table makes load-bearing for us:** (a) the headline
27.8 MB is **embeddings-off**; the desks need the memory layer (P4), so the honest jcode figure to
beat Claude with is **167 MB**, still ~2.3× under Claude's 386 MB but not the ~14× headline; (b) it
is an **idle client**, not a desk with the real tool surface loaded.

**Our-workload measurement — BLOCKED (needs live jcode). Exact protocol to run when unblocked:**
```bash
# one desk session, real tools loaded (intake-desk board read), embeddings ON (P4 needed):
jcode &                      # start a session running the intake-desk skill to steady state
JPID=$(pgrep -n jcode)
# resident set (prefer PSS to match the vendor method; RSS as a floor):
grep -i '^Pss:' /proc/$JPID/smaps_rollup | awk '{s+=$2} END{print s" kB PSS"}'   # Linux
# ps RSS cross-check (portable):
ps -o rss= -p $JPID                                                                # kB RSS
# cold-boot (time to usable session), median of >=5 cold starts:
for i in $(seq 5); do /usr/bin/time -v jcode --version 2>&1 | grep -i 'Elapsed'; done
# repeat the same protocol for a Claude Code desk session on the SAME host for a paired delta.
```
Record the number **beside** the vendor claim, name the host, and state embeddings on/off. Per the
brief's Verify item 3, **no density number is asserted in this doc** — the vendor figures are
sourced+caveated above, and the our-workload figures are BLOCKED, not estimated.

**Why density is the point (design read, not a measurement):** if even the embeddings-on ~167 MB
holds on our workload, that is ~2.3× more desk+worker sessions per node than Claude's ~386 MB — a
direct multiplier on the desk-console substrate's binding constraint. The win is *plausibly real* but
must be measured with the memory layer on and the real tool surface loaded, not read off an idle
embeddings-off client.

---

## 7. Go / no-go read (conditional — the gating number/probe is BLOCKED)

**Read: NO-GO to *drive desks* on jcode today; a CONDITIONAL YES worth a migration brief only if the
live probes clear the two blockers below.** Reasons, in priority order:

1. **Isolation (P6) is the hard blocker — `absent` today.** The desks' non-degradable guarantee is
   per-implementer worktree isolation; jcode has removed worktrees on purpose and has not shipped its
   replacement. Until jcode has an isolation primitive the writeguard can lean on, `worker-desk`
   fan-out and any behind-the-guard write **must refuse** under jcode — the stream's own rule. This
   alone makes it a no-go for the fan-out desks now. *(Watch jcode's planned isolation primitive;
   re-evaluate when it lands.)*
2. **Resident-rules delivery (P3) is `workaround`, not `present`.** Assay's one load-bearing hook has
   no confirmed always-on jcode equivalent; on-demand semantic injection risks the method simply not
   arriving in a session. A confirmed unconditional channel (AGENTS.md-always-load or a pinned prompt
   overlay) would clear this — BLOCKED on the live probe.
3. **Exec-tier (P7) leans `per-session-only`** — documentary read finds no per-task model field, so
   the headline "runtime exec-tier guarantee" adoption reason is **probably not available**. Removes
   one of the two big reasons to migrate; BLOCKED on the live probe to be certain.
4. **Density win is plausibly real but unproven on our workload** — and the honest figure is the
   embeddings-on ~167 MB (P4 is required), ~2.3× under Claude, not the ~14× headline. Worth
   measuring; not worth migrating on vendor copy.
5. **The discrete-meld (§5) is the genuinely attractive finding and is harness-independent** — most
   of a desk's loop is deterministic mechanics that should become discrete code/tools regardless of
   whether jcode is adopted. That redesign banks value on Claude Code today and would *also* de-risk
   any future jcode move.

**Net:** the strong primitives (memory, swarm, density-direction, Anthropic-compatible backends) are
real, but the **two non-degradable desk guarantees — isolation and always-on resident rules — are
the exact things jcode does not yet clearly provide.** Ruling jcode in now would rebuild the
single-vendor-drift-at-decision-time this stream exists to remove. **Recommended next move:** do NOT
open a migration brief yet; (i) file the live-measurement follow-up (install jcode, run the §3/§4/§6
probes) as the unblocking act; (ii) pursue the discrete-meld (§5) as a Claude-Code-today redesign
that is valuable independent of jcode; (iii) re-open this read when jcode ships its isolation
primitive. This spike **informs** — it does not gate — the HP/03 harness-target ruling.

---

## Appendix A: Sources (retrieved 2026-08-24)

| Source | URL | Note |
|---|---|---|
| jcode GitHub repo | https://github.com/1jehuang/jcode | MCP config, install, backends, swarm, no-worktree roadmap |
| jcode homepage | https://jcode.sh/ | install, positioning |
| jcode DeepWiki | https://deepwiki.com/1jehuang/jcode | crate map (`jcode-swarm-core`, `jcode-embedding`, `jcode-memory-types`), skills/prompt-overlays, ONNX memory |
| explainx.ai — jcode swarm/memory/perf | https://explainx.ai/blog/jcode-agent-harness-swarm-memory-performance-july-2026 | benchmark numbers + the "author self-benchmarks, not independent lab results" caveat |
| DEV.to — jcode Rust-native agent harness | https://dev.to/terminalchai/jcode-the-rust-native-agent-harness-for-multi-session-development-l4g | multi-session positioning |
| Codex sibling matrix (cross-check) | docs/research/codex-harness-capabilities.md | per-child model-override precedent (openai/codex issue 32674); resume-durability class |

**All jcode capability verdicts here are DOCUMENTARY (public docs) + the named live probe that must
confirm them. No verdict is inherited as measured; every live row and every our-workload number is
`BLOCKED (needs live jcode)`.**
