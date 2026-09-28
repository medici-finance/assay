# SpecMem portable-memory spike — one stream's registers across two harnesses

**Spike (b) of the two-spike SpecMem/jcode evaluation.**
**Date**: 2026-08-24 | **Environment**: no live SpecMem MCP server and no second harness available — documentary + architectural assessment; the live cross-harness run is BLOCKED (see §7).
**Authored for**: harness-portability/10
**Provenance**: authored 2026-08-24 in the stream's pre-re-home source tree and ported here 2026-09-27, when the stream's re-home to this repository was found to have carried the brief but not this file. The findings are unchanged; only references that do not resolve in this repository were removed or reworded.
**Subject**: SpecMem — `SuperagenticAI/specmem`, Apache-2.0 (github.com/SuperagenticAI/specmem)

---

## 0. What this spike can and cannot conclude

The brief asks whether SpecMem can hold **one stream's briefs/registers portably across Claude Code AND a second harness** — the faithfulness test being: *does querying the SAME SpecMem store from two harnesses return the same specs/impact/context, or does one harness silently get a degraded view?*

Executing that test end-to-end requires two things this environment does not have: (a) a running SpecMem MCP server built over a copy of a stream's registers, and (b) a live second harness (Codex or jcode) with an MCP client to issue the same query Claude Code issues. Neither is available offline, and **an external harness's returned output must not be fabricated**. So this document delivers what *can* be established statically and to a high standard:

- the **architecture of SpecMem's portability claim** and where the portable boundary actually sits (§2–§3);
- the **register-mapping** between Assay's `brief-v1` + in-git register shapes and SpecMem's expected spec model, with the fit/gap verdicts (§4);
- the **go/no-go read** on whether SpecMem meaningfully de-couples memory from the harness, and at what adoption cost (§5);
- an **executable, reproducible protocol** for the live cross-harness faithfulness run, so the BLOCKED row can be discharged the moment an environment exists (§6);
- the **BLOCKED escalation** with the exact missing capability (§7).

This mirrors the pattern already used for `docs/research/codex-harness-capabilities.md` (brief 01): documentary + architectural verdicts now, live rows explicitly BLOCKED with a positive-control protocol, never a fabricated run.

Retrieval date for all SpecMem documentary evidence below: **2026-08-24**, from the upstream repository README (`github.com/SuperagenticAI/specmem`, Apache-2.0).

---

## 1. What SpecMem is (measured from upstream, not inherited)

SpecMem describes itself as an "Agent Experience (AgentEx) platform" — a **unified, embeddable, MCP-exposed memory layer for AI coding agents**. The pipeline it documents:

```
diverse spec formats  →  per-framework adapters  →  SpecIR (canonical internal representation)
                       →  vector DB / impact graph / diff timeline  →  output bundle in .specmem/
```

The scanned specs are normalised into a canonical **SpecIR**, persisted into a `.specmem/` output store:

| File | Content |
|------|---------|
| `agent_memory.json` | specs with metadata |
| `knowledge_index.json` | keyword → spec mapping |
| `impact_graph.json` | relationships between specs/tests |
| `vectordb/` | vector storage for semantic query |

The portability claim, quoted from upstream: *"Switch from Kiro → SpecKit → Tessl → Claude Code → Cursor without rewriting spec files or losing project knowledge."* This locates portability at the **store/SpecIR layer**, not at a per-harness runtime translation layer — a distinction that turns out to be the crux of the go/no-go (§3, §5).

---

## 2. The MCP surface — identical for every harness

SpecMem ships an MCP server, `specmem-mcp`. The documented connection config (verbatim from upstream README, retrieved 2026-08-24):

```json
{
  "mcpServers": {
    "specmem": {
      "command": "specmem-mcp",
      "args": [],
      "env": { "SPECMEM_WORKSPACE": "${workspaceFolder}" },
      "autoApprove": ["specmem_query", "specmem_tldr", "specmem_coverage"]
    }
  }
}
```

Exposed MCP tools (verbatim tool names + upstream descriptions):

| Tool | Purpose |
|------|---------|
| `specmem_query` | Search specifications by natural language |
| `specmem_impact` | Analyze change impact on specs and tests |
| `specmem_context` | Get optimized context bundle for files |
| `specmem_tldr` | Get TL;DR summary of key specs |
| `specmem_coverage` | Analyze spec coverage and test gaps |
| `specmem_validate` | Validate specifications for quality issues |

CLI lifecycle: `specmem init` (`--hooks`) → `specmem scan` (index specs) → `specmem build` (emit the Agent Experience Pack) → `specmem validate`.

**Architectural consequence for the portability question.** The three surfaces the brief names — specs lookup, impact analysis, optimized-context retrieval — map exactly onto `specmem_query`, `specmem_impact`, `specmem_context`. Critically, the MCP surface is **defined once by the server and consumed identically by any MCP client**. Claude Code and Codex are both MCP clients: each would register the *same* `specmem-mcp` server pointed at the *same* `SPECMEM_WORKSPACE`, and each would call the *same* tool names over the *same* JSON-RPC contract against the *same* `.specmem/` store. There is no per-harness code path inside the query engine — the harness is a thin transport in front of one shared index.

This is strong static evidence that **the faithfulness test's most-feared failure mode — one harness silently getting a degraded view — is unlikely by construction for the query/impact/context tools**: divergence would require the two MCP clients to receive different responses to identical requests against one server, which is not how MCP request/response works. The residual risk is not per-harness divergence; it is **whether the store faithfully represents Assay's registers in the first place** (§4). That reframing is itself a spike result.

---

## 3. Where the portable boundary actually sits

The portable object is **SpecMem's own SpecIR store**, not Assay's registers. SpecMem achieves cross-agent portability by *ingesting* many source formats through adapters and *normalising* them into SpecIR; downstream agents then read SpecIR, not the original files. So "portable across harnesses" is true **of content that has been successfully adapted into SpecIR** — and the adoption cost is entirely front-loaded into that adaptation step.

Documented adapter source patterns (retrieved 2026-08-24):

| Framework | Auto-detected pattern |
|-----------|-----------------------|
| Kiro | `.kiro/specs/**/*.md` |
| SpecKit | `.speckit/**/*.yaml` |
| Tessl | `.tessl/**/*.md` |
| Claude Code | `Claude.md`, `CLAUDE.md` |
| Cursor | `cursor.json`, `.cursorrules` |
| Codex | `.codex/**/*.md` |
| Factory | `.factory/**/*.yaml` |
| Warp | `.warp/**/*.md` |
| Gemini CLI | `GEMINI.md`, `.gemini/**/*.md` |

Kiro is the first-class citizen: its adapter reads the SDD triad (`requirements.md` / `design.md` / `tasks.md`) under `.kiro/specs/`, and SpecMem's `specmem_validate` "Structure" rule literally checks that "Required sections (requirements, design, tasks) exist," with acceptance criteria in EARS form. The other harnesses are served by adapters, but the **Claude Code adapter pattern is `CLAUDE.md` only** and the **Codex adapter is `.codex/**/*.md`** — neither auto-detects `docs/streams/<stream>/brief-*.md`, `Verify` tables, `freshness.yaml`, or `STATUS.md`. Whether the scanned source directory is **configurable to an arbitrary path** is **not documented** upstream (the CLI operates from project root against the framework patterns above) — a material unknown flagged in §4 and §7.

---

## 4. Register-mapping: Assay `brief-v1` + registers ↔ SpecMem SpecIR

This is the load-bearing analysis. It maps each Assay register shape onto SpecMem's expected model and classes the fit as `portable` (survives faithfully), `native-only` (needs each harness's own format / does not fit SpecIR), or `adapter-gap` (would fit only after non-trivial restructuring or an adapter SpecMem does not ship).

| Assay register | Shape / role | Nearest SpecMem concept | Fit verdict | Note |
|---|---|---|---|---|
| `brief-vN` frontmatter (`brief:`, `wave`, `depends`, `unblocks`, `gate`, `risk{}`, `issues`, `schema`) | machine-read dependency + governance metadata | SpecIR spec + metadata (`agent_memory.json`) | **adapter-gap** | The dependency graph (`depends`/`unblocks`) is closest to `impact_graph.json`, but SpecMem's impact graph is derived from spec/test relationships, not from an authored wave/DAG. No adapter reads brief frontmatter; the fields would be lost or flattened unless first re-expressed as SpecIR metadata. |
| Brief body `## Task` / `## Context` | narrative implementation contract | `requirements.md` (user stories / acceptance criteria) | **adapter-gap** | Semantically close to a requirements doc, but SpecMem expects the EARS-structured `requirements.md` section; an Assay brief is one markdown file, not the requirements/design/tasks triad. `specmem_validate`'s Structure rule would flag it as missing required sections. |
| Brief `## Verify` table (executable cmd → expect) | the evidence contract — the register's spine | `tasks.md` checklist + `specmem_coverage` (spec→test gaps) | **portable-with-mapping** | This is the best-fit surface: Verify rows are exactly the "spec coverage / test gaps" SpecMem models. If mapped into SpecIR, `specmem_coverage`/`specmem_impact` could genuinely serve it cross-harness. But the mapping is manual — no adapter emits SpecIR from a Verify table today. |
| `## Evidence` (filled by non-implementer) | audit trail, dated runner attributions | none | **native-only** | SpecMem models specs and their coverage, not a dated human/runner attestation ledger. Evidence would remain in-git. |
| `freshness.yaml` (staleness manifest + per-claim anchors) | temporal validity of artifacts vs upstream commits | `diff timeline` (loosely) | **native-only** | SpecMem's diff engine tracks spec change over time, but not "this claim is invalidated by an upstream repo's commit glob." The upstream-glob invalidation model is Assay-specific. |
| `STATUS.md` / board README status table | generated rollup (statusgen) | `specmem_tldr` (summary) | **native-only** | `specmem_tldr` summarises specs; it does not reproduce statusgen's board semantics (wave/critical-path/verified/reviewed columns). Derived, stays generated in-git. |
| `CLAUDE.md` / resident rules | session-resident method text | Claude Code adapter (`CLAUDE.md` pattern) | **portable** | This is the ONE Assay artifact SpecMem's Claude Code adapter natively ingests. Ironically it is method text, not a stream register — so it validates the adapter path without addressing the brief's actual object (a stream's briefs/registers). |

**Reading of the table.** The register whose portability would most help — the **Verify/coverage spine** — is a genuine fit for SpecMem's model and would plausibly serve identically across two MCP clients. But *getting Assay's registers into SpecIR at all* is the cost: SpecMem's spec model is the **SDD requirements/design/tasks triad**, and Assay's `brief-v1` is a single governed markdown file with frontmatter, a Verify table, and a separate Evidence ledger. There is **no shipped adapter for the Assay brief shape**, and the source-path configurability that a custom scan would need is **undocumented**. So the honest boundary is: SpecMem can very likely serve *a stream restructured into its triad* portably across harnesses; it cannot today ingest *Assay's registers as they exist* without either (a) an Assay→SpecIR adapter someone writes, or (b) restructuring the stream into `.kiro/specs/`-style triads — which reintroduces exactly the format lock-in (now to Kiro's shape) the stream exists to remove.

---

## 5. Go / no-go read

**Verdict: NO-GO for adopting SpecMem as the desks' portable register store on current evidence — but the harness-independence *mechanism* is architecturally sound and worth a bounded re-look if the register model ever converges on SDD triads.**

Reasoning, separating the two halves of the portability claim the brief conflates:

1. **Harness-independence of the query surface — CREDIBLE (static).** Both Claude Code and Codex are MCP clients hitting one `specmem-mcp` server over one `.specmem/` store with identical tool names and contract (§2). By construction the same query returns the same bytes to both; a silently-degraded per-harness view is not a natural failure mode of MCP request/response. This half of the claim is well-supported without a live run — though it remains formally BLOCKED pending the §6 confirmation, because "credible by architecture" is not "measured."

2. **Faithful capture of Assay's registers — NOT MET today.** The portable object is SpecIR, and SpecMem ships no adapter for `brief-v1`/Verify/freshness/STATUS shapes; only `CLAUDE.md` is natively ingested (§3–§4). Portability of Assay's *registers* therefore costs either a bespoke adapter or a migration of every stream into the requirements/design/tasks triad. That migration:
   - re-locks the registers to a *different* vendor's format (Kiro's SDD shape), trading one lock-in for another — the opposite of the stream's goal;
   - abandons Assay-specific machinery with no SpecMem equivalent (the Evidence attestation ledger, `freshness.yaml`'s upstream-glob invalidation, statusgen's board semantics — all classed `native-only` in §4);
   - front-loads real authoring cost for a benefit (cross-harness query) that Assay's registers already get for free, because they are **plain in-git markdown any MCP-capable harness can already read directly** — the very property that makes them portable without a memory layer.

3. **The decisive framing.** Assay's lock-in is not that its registers are *unreadable by other harnesses* — they are ordinary markdown. The lock-in this stream removes lives in the **delivery mechanism** (SessionStart hook, plugin manifest, skill discovery contract — see the stream README), which SpecMem does not touch. SpecMem solves a problem Assay's registers do not have (cross-format spec normalisation) while not solving the one it does (harness-native delivery of method text). Adoption cost is high; marginal portability benefit over "commit markdown to git" is low.

**Adoption cost, if pursued anyway (bounded estimate):** write and maintain an Assay-`brief-v1`→SpecIR adapter (SpecMem ships none; source-path configurability undocumented); stand up and version the `specmem-mcp` server per repo; keep the `.specmem/` store in sync with in-git registers (a second source of truth to reconcile — the exact drift-surface the stream's README calls out as this repo's live failure mode). Net: SpecMem is a **watch-list** dependency, re-evaluated only if (a) it ships an arbitrary-source-dir scan + a non-triad adapter path, or (b) Assay's own register model moves toward SDD triads for independent reasons.

This spike **informs, does not gate**, the HP/03 harness-target ruling, exactly as the brief frames it.

---

## 6. Reproducible protocol for the live cross-harness faithfulness run (discharges the BLOCKED row)

When a SpecMem build and a second MCP-capable harness are both available, run this verbatim; it is designed so the SAME query is issued from BOTH harnesses and the two returned outputs are quoted side-by-side (Verify item 3):

1. **Copy a low-stakes stream** (never a load-bearing register): `cp -r docs/streams/harness-portability /tmp/specmem-trial/` — brief bodies + README only; the authoritative registers stay in-git and untouched.
2. **Build the store**: from `/tmp/specmem-trial/`, `specmem init && specmem scan && specmem build`. Record whether `specmem scan` picked up `brief-*.md` at all, or only a `CLAUDE.md`-shaped file — this measures §3's undocumented source-path question. If it ignores the briefs, restructure one brief into `requirements.md`/`design.md`/`tasks.md` under `.kiro/specs/trial/` and re-scan; note that restructuring cost.
3. **Register the same `specmem-mcp` server** (config from §2, `SPECMEM_WORKSPACE=/tmp/specmem-trial`) in **both** Claude Code and the second harness (Codex/jcode).
4. **Issue one identical query from each harness** for all three surfaces, e.g. `specmem_query "what unblocks brief 07"`, `specmem_impact` on a brief file, `specmem_context` for a brief path.
5. **Quote both harnesses' raw returned outputs verbatim** into the Evidence table below and mark each triplet `identical` / `degraded` / `native-only`. Do **not** paraphrase — the faithfulness verdict is the byte-comparison.

Positive control: a query with a known answer present in the store must return that answer in BOTH harnesses; if either returns empty, the store/transport is misconfigured, not a portability finding.

---

## 7. BLOCKED escalation

**BLOCKED — live cross-harness run not executable in this environment.** Verify item 3 (the SAME query run from BOTH harnesses with actual returned output quoted and compared) requires a running `specmem-mcp` server and a second live MCP-capable harness; neither is available offline, and external-harness output must not be fabricated. The row is delivered as **static architecture + register-mapping + go/no-go + a verbatim reproduction protocol** (§2–§6); the byte-level faithfulness comparison is the single outstanding item, discharged by running §6 once the stream's driver provisions or sanctions the environment. The go/no-go read (§5) does not hinge on that run — it turns on the register-mapping fit (§4), which is fully assessable statically.

---

## 8. Evidence

<!-- Verify items 1/2/4 satisfied by this file + freshness registration. Item 3 (two-harness live comparison) is BLOCKED per §7 — filled by whoever runs the §6 protocol. -->

| Verify # | Item | State | Evidence |
|---|---|---|---|
| 1 | findings doc exists | met | this file, `docs/research/specmem-portability-spike.md` |
| 2 | faithfulness verdicts present (`identical`/`degraded`/`portable`/`native-only`) | met | §4 mapping table + §5 verdicts use all four terms |
| 3 | same query from BOTH harnesses, outputs quoted + compared | **BLOCKED** | no live SpecMem server + second harness offline (§7); protocol to discharge in §6 — no fabricated output |
| 4 | file registered in `freshness.yaml` | met | entry added under `artifacts:` |
