---
stream: metrics-harvest
status: active
priority: P1
track: product
tiering: implement=cheap verify=strong
---

# Metrics Harvest Stream

Builds the **generic, repo-agnostic, AI-free metrics layer** for the three Medici products —
**assay, plumb, lending** — each spanning MULTIPLE repos across TWO GitHub orgs
(`medici-finance` + `the-org`). A **harvester** snapshots deterministic per-repo metrics
(open PRs, open issues, commits-to-main + author split, DORA-style throughput where
derivable) into a dated, per-domain/per-repo artifact tree on a schedule; an **aggregator**
rolls those per-repo artifacts into per-domain totals and an all-products cross-domain
roll-up. Scoping source: I-07 (human:<name> direction, 2026-07-20).

This **generalizes the oit `tools/daily-harvest`** (oit
`methodology-metrics/22`, a single-repo/single-org collector) across products and orgs. The
code lands in **this repo** (`assay-toolkit`, Apache-2.0 free tier) as a standalone Go tool
under `tools/` (matching the existing `tools/freshness/` per-tool-module convention). The
**Desk Console's Measures pane** (desk-console stream) is the
eventual consumer; this stream builds nothing product-specific.

## Why here and not in `desk-console`

The Desk Console owns the premium **consumer** — the Measures pane (spec §7.3) and the
metrics CronJob (desk-console/05) — whose code
targets the **private, non-open** `console-repo` repo, and which by its own charter
"consumes, does not rebuild" the deterministic generators. The multi-domain
harvester+aggregator are the opposite: **open-tier, repo-agnostic `tools/` code** that must
live in the Apache-2.0 toolkit and be valuable to any repo. Folding build-briefs for open
toolkit code into the private-Console stream would mismatch its code target and disrupt its
tightly-sequenced 13-brief critical path. So it is a distinct stream that desk-console/05 and
desk-console/12 consume **upstream** (feathering table below).

## Briefs

| # | Brief | Wave | Effort | Gate | Status | Verified | Reviewed |
|---|-------|------|--------|------|--------|----------|----------|
| 01 | [Multi-domain harvester — per-domain repo config, cross-org gh/git snapshots, dated artifact tree](./brief-01-multi-domain-harvester.md) | 0 | M | model | implemented | — | — |
| 02 | [Cross-domain aggregator — per-domain totals + all-products roll-up over the harvest tree](./brief-02-cross-domain-aggregator.md) | 1 | M | model | todo | — | — |

## Domain → repo grouping (data, not code)

The harvester reads a declared per-domain repo list from
tools/metrics-harvest/domains.yaml (new — data — add repos by editing config, never by
editing code). Enumerated authoritatively against the live org listings 2026-07-20
(`gh repo list medici-finance` + `gh repo list the-org`):

There are **three product domains** plus a distinct **org-wide** grouping (organization-level
repos that belong to no single product; the aggregator surfaces it at the top level, separate
from the product roll-ups). All groupings ruled by human:<name> 2026-07-20 (see I-07):

| Grouping | Repos (org/repo) |
|----------|------------------|
| **assay** (product) | `medici-finance/assay-toolkit`, `medici-finance/assay-site`, `medici-finance/console-repo`, `medici-finance/assay-decks` |
| **plumb** (product) | `medici-finance/reconciler`, `medici-finance/site-repo`, `medici-finance/reconciler-decks` |
| **lending** (product) | `oit`, `example-org/agent-runtime`, `example-org/medici-examples`, `medici-finance/medici-decks` |
| **org** (org-wide, not a product) | `medici-finance/decks`, `medici-finance/platform-repo` |

human:<name>'s grouping rulings (2026-07-20):
- `medici-decks` → **lending** (belongs to the lending platform).
- `decks` + `platform-repo` → **org-wide** (`org` key), ACTIVE — organization-wide, not a
  single product; the aggregator surfaces `org` at the top level distinct from the three
  products.
- `proposals` → **excluded / deferred** ("quiet, ignore for now"); ships as a one-line
  commented note in `domains.yaml`, not an entry.

**Out of scope — legacy/unrelated `the-org` repos** (NOT part of any Medici product):
`routerengine-blog`, `ui`, `n-ui`, `jojig-cosmes`, `n-contracts`, `tefi-oracle-contracts`,
`contracts`, `app`. These predate the Medici products; excluded from `domains.yaml`.

## Critical path — the REAL head is EXTERNAL (per-product + all-covering GitHub Apps)

```
[EXTERNAL GATE (human:<name>): FOUR GitHub Apps, created + installed, tokens as assay-toolkit CI secrets —
 metrics-harvest-assay / -plumb / -lending (each scoped ONLY to its product's repo set) +
 metrics-harvest-all (reads across all products + the org-wide repos)]
                                   │  (not in this stream; nothing scheduled-dispatchable without them)
                                   ▼
   01 harvester ─────────────────────────────────► 02 aggregator
   (per-product harvest uses its product App;         (uses the all-covering App to roll up
    org-wide harvest uses the all-covering App)         01's tree across every grouping)
```

**The real head is not code in this stream — it is the App fleet that can read the repos.**
Every repo in all three products is **private** and they span two orgs (verified 2026-07-20:
`medici-finance/reconciler` and `example-org/agent-runtime` both `PRIVATE`). A workflow's default
`GITHUB_TOKEN` is scoped to its own repo only. Per human:<name>'s 2026-07-20 auth ruling this is solved
with **least-privilege GitHub Apps — one per product plus one all-covering App**, not a single
cross-org god-token:

| App | Reads (installation scope) | Used by |
|-----|----------------------------|---------|
| `metrics-harvest-assay` | only the **assay** repo set | per-product harvest of `assay` |
| `metrics-harvest-plumb` | only the **plumb** repo set | per-product harvest of `plumb` |
| `metrics-harvest-lending` | only the **lending** repo set (incl. `medici-decks`) | per-product harvest of `lending` |
| `metrics-harvest-all` | **all** product repos + the **org-wide** repos | the `org` harvest + the cross-domain aggregator (brief 02) |

Minimal permissions for every App: **`contents:read`, `issues:read`, `pull_requests:read`**
(metadata:read implied). **App creation + installation is an human:<name>-provisioned dependency** — the
same class of gate as before (oit#575 reddened the single-repo harvester's scheduled run on
exactly this), now per-product least-privilege instead of one token. Brief 01 is authorable and
locally runnable now (a developer's own `gh` token with the right repo access proves the
collector); its **scheduled** deliverable waits on the App fleet.

**Tempting-but-wrong first step: building the aggregator (02) first** because "the roll-up is
the point." It has nothing to roll up until the harvester writes a dated tree, and its trend
columns need ≥2 days of that tree — building it first means mocking the entire artifact
layout, then rewriting against the real one.

## Dependency waves

```
Wave 0: [01]
Wave 1: [02 ← 01]
```

Longest chain: **ext(App fleet) → 01 → 02**. `depends:` arrays are in-repo/in-stream only
(lint); the external App-provisioning gate is carried by this section and each brief's
`facts:`, never as a `depends:` edge.

## Feathering — external items this stream relates to (NOT in `depends`)

Not re-authored here; cross-repo/cross-stream ordering is a dispatch-time concern for the
desk. Statuses freshness-checked 2026-07-20.

| External item | Repo / stream | Status | Role here |
|---|---|---|---|
| `methodology-metrics/22` daily artifact harvest | cross-repo: oit | implemented (Change-Failure bug oit#575 open on the scheduled run) | The single-repo/single-org **precedent** this stream generalizes; its fail-loud shape + `[skip-status-regen]` marker + `prs.json`/`issues.json`/`git.txt` capture set are reused. oit#575 (token scoping) is the SAME class of gate as this stream's cross-org head — heed it |
| `desk-console/05` metrics loop as CronJob | in-repo: desk-console | todo | **Downstream consumer** — schedules metric generation; can run this harvester per the dogfood rule once the cross-org token exists |
| `desk-console/12` altitude panes (Measures) | in-repo: desk-console | todo | **Downstream consumer** — the Measures pane renders the aggregator's cross-domain roll-up |
| App fleet: `metrics-harvest-{assay,plumb,lending,all}` | EXTERNAL (human:<name>) | not provisioned | The **critical-path head** — per-product least-privilege Apps + one all-covering App (`contents:read`/`issues:read`/`pull_requests:read`), tokens as CI secrets |

Amendment/provisioning requests are dispatch items for the desk / human:<name>, not edits made from
this stream.

## Shared conventions

- **Deterministic, AI-free.** Every artifact is a `gh` API read or a `git` read — no model in
  the loop. The collector is a pure snapshotter; analysis is a later, separate concern (the
  same generation-vs-analysis split as oit methodology-metrics/22).
- **Config is data.** The per-grouping repo list lives in tools/metrics-harvest/domains.yaml
  (new); repos are added/removed by editing that file, never by editing Go. The `org` grouping
  is org-wide (not a product); the aggregator surfaces it at the top level. A deferred repo
  (`proposals`) stays a one-line comment until human:<name> rules it in.
- **Auth is per-product least-privilege** (human:<name> 2026-07-20): each product App reads only its
  own repo set; only `metrics-harvest-all` reads across products + org-wide. No single token
  reads everything except the all-covering App the aggregator uses.
- **Fail-LOUD.** A failed capture still writes its artifact (exit code + stderr as evidence on
  disk) and makes the process exit non-zero, so a scheduled run goes red and someone knows —
  matching the oit harvester.
- **Non-recursion marker.** Scheduled commits carry `[skip-status-regen]` so they never
  trigger `statusgen.yml`'s regen-and-commit loop (this repo's marker, per
  .github/workflows/statusgen.yml).
- **Discoverability = the README docs/ index.** A new tool/artifact tree is linked from
  `README.md` when it lands. (No `docs-site/`/`llms.txt` regen applies —
  assay-toolkit has none; that is the parent oit repo's rule.)
- **Scope fence.** This is the GENERIC/repo-agnostic layer. Anything product-specific (a
  DAML board, a reconciler-specific metric, a stream taxonomy) stays per-repo and is out of
  scope here.
