---
stream: assay-dogfood
serves: assay
status: active
priority: P1
track: platform
issues: []
---

# Assay-Dogfood Stream

Get this project to **consume the methodology the way an external adopter would**: the loop
skills + resident rules arrive as a versioned Claude Code **plugin from a marketplace hosted
in `medici-finance/assay-toolkit`** (private; worked locally as `../assay-toolkit`), statusgen
arrives as a **pinned release binary**, and working agents
in consumer repos hold **read + issues only** on the methodology's source. Dogfooding is the
point (human:<name>, 2026-07-10: "a new initiative — get us to dogfood this"): every gap we hit is
the gap an adopter would hit, filed where they would file it.

Origin: INTAKE **I-30**, executing I-24's phasing (② release-consumed statusgen, ③ the
skills/rules bundle — marketplace named candidate-primary 2026-07-10) with the adopter drill
as the exit proof. This is also the I-02 productization surface: the install story becomes
`/plugin marketplace add` + install.

**Supersedes / absorbs** (noted in each brief): methodology/07 (toolkit extraction — the
plugin IS the extraction), methodology/22 (single-home the operating rules — the plugin +
SessionStart hook is the single home), and the #221 problem class for skills (a versioned
plugin cache replaces loose `~/.claude` files as the distribution surface; the local-cache
residual keeps the pinned-release hash-check in scope). Binaries stay behind desk-tools
C-1's `sudo make desk-install` gate — plugins do not ship Go binaries.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Bootstrap ../assay-toolkit — repo, plugin scaffold, marketplace, governance permissions](./brief-01-bootstrap-assay-tools.md) | 0 | M | implemented | — | — |
| 02 | [Skills bundle v0.1 — the five loop skills + resident-rules SessionStart hook, as `assay:*`](./brief-02-skills-bundle.md) | 1 | M | implemented | — | — |
| 03 | [statusgen as a pinned release — build/tag in assay-toolkit, consumers verify by hash](./brief-03-statusgen-release.md) | 1 | M | done | 2026-07-23 glm-5.2-verifier | 2026-07-20 assay-reviewer-app[bot] |
| 04 | [Dogfood cutover — this repo installs the plugin, retires loose copies, CLAUDE.md shrinks](./brief-04-dogfood-cutover.md) | 2 | M | todo | — | — |
| 05 | [Adopter drill — a clean consumer onboards from the marketplace alone; gaps become issues](./brief-05-adopter-drill.md) | 3 | S | todo | — | — |

## Dependency waves

```
Wave 0: [01]
Wave 1: [02, 03] ← 01
Wave 2: [04 ← 02, 03]
Wave 3: [05 ← 04]
```

Critical path: **01 → 02 → 04 → 05**. The head is real: nothing distributes until the
assay-toolkit repo exists with its permission model — and repo creation + permissions are
human:<name>'s acts (brief 01 is `gate: human` for exactly that reason).

## Shared conventions (inherited by every brief)

- The governance boundary is the point: in consumer repos, agent identities hold **read +
  issues only** on assay-toolkit. Proposing a methodology change = filing an issue THERE.
  Wanting to bypass that friction is the feature working, not a bug. **Current state (human:<name>,
  2026-07-10, #262):** the repo lives in the `medici-finance` org where the `the-org`
  machine account holds write/admin via org membership — an accepted, time-boxed exception
  **until Sep 2026**; the read-only boundary is the target model, not yet enforced.
- Every brief that changes what a running session loads (plugin content, hooks, rules) is a
  SHARED-VALUE change: enumerate consumers (all three repos' sessions + any adopter) and
  verify the flow, not just the site.
- Honest-residual discipline (F-08): the local plugin cache is user-writable; nothing here
  claims tamper-proof distribution — the claim is versioned, namespaced, hash-checkable
  distribution with drift visible to lint.
- Cross-repo work in `../assay-toolkit` cannot ride this repo's PRs: briefs record assay-toolkit
  commits/PR links in Evidence, and this repo's PR carries the consumer-side changes.
