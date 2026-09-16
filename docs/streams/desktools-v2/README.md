---
stream: desktools-v2
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
spec: docs/streams/desktools-v2/spec.md
issues: []
board: generated
---

# desktools-v2 Stream

**Proposed, not yet approved.** This stream is `status: parked` and cites a `**Status:**
draft` scoping doc ([spec.md](spec.md)) — the sanctioned representation of a not-yet-ruled
stream (`spec/lifecycle-v1.md` §8; `statusgen` `streamSourceProblem`: "a parked stream may
cite a `draft`"). Its briefs are authored and kept but shelved out of Next-up and never
dispatched until a human rules the spec `approved` and flips this stream `active`.
`approved` is the human's call.

Rebuild the desk tools' forge access **properly**, now that the flows have solidified. A
`Forge` interface already exists (`tools/desk/internal/deskkit/forge.go`, two complete
backends) and the desk verbs are ~mostly on it; the defect is that things still reach *past*
it — with a GitHub-specific fact (a `gh` subprocess, remote name, query shape) or with an
**ambient credential**. v2 makes reaching past it **structurally impossible**, on three
first-class principles (see [spec.md](spec.md) §2):

1. **CUSTODY is the foundation** — explicit minted-token only, never an ambient CLI credential;
   the re-minted credential is the App PEM + installation id, so **key presence is the custody
   boundary**; one role's key per environment, making even the desktop behave like a locked
   container.
2. **The read path covers statusgen** — statusgen shells `gh` directly with its own ambient
   custody and no native client (the `scanloop`-in-container break, #628); "retire `gh`" is
   not done until statusgen consumes the shared Forge library — which needs `deskkit` promoted
   to an importable shared library first (part of "built properly").
3. **Purpose-built queries** — typed access-pattern operations (review-queue snapshot, head-sha
   batch, board sweep), each backend one tuned query: N+1 → one round-trip, rate-limit headroom,
   and one consistent snapshot (freshness), with the GraphQL document never crossing the seam.

Boundaries with the sibling streams (`forge-neutral`, `desktools-go-git`, `desk-tools`) and the
open questions for the approver are in [spec.md](spec.md) §4/§7.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line)](brief-01-audit-and-inventory.md) | 1 | M | todo | — | — |
| 02 | [the v2 seam contract + the ban-lint (advisory/counting first)](brief-02-seam-contract-and-ban-lint.md) | 2 | M | todo | — | — |
| 03 | [native read-path installation-token client — retire gh shell-out in the read path (#1223 pilot)](brief-03-native-read-client.md) | 2 | M | todo | — | — |
| 04 | [migrate deskclose off the hardcoded pullRequest GraphQL query (#1019)](brief-04-migrate-deskclose-query.md) | 3 | S | todo | — | — |
| 05 | [migrate deskpushguard off the hardcoded remote name + expand insteadOf (#1201 / #884)](brief-05-migrate-deskpushguard-remote.md) | 3 | M | todo | — | — |
| 06 | [installation-token scoping — one token per repo, no ambient-credential hiding (#628 / #1145 / #1146)](brief-06-installation-token-scoping.md) | 3 | M | todo | — | — |
| 07 | [promote deskkit's Forge to an importable shared library (the built-properly enabler)](brief-07-deskkit-importable-shared-library.md) | 2 | L | todo | — | — |
| 08 | [migrate statusgen's forge reads onto the shared Forge under minted-token custody](brief-08-statusgen-onto-shared-forge.md) | 3 | L | todo | — | — |
| 09 | [purpose-built access-pattern query operations (one tuned snapshot, not N per-item calls)](brief-09-access-pattern-queries.md) | 3 | L | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`desktools-v2/01` (audit & inventory — the frozen file:line table, incl. statusgen) ->
`desktools-v2/07` (promote `deskkit` to an importable shared library — the "built properly"
enabler) -> `desktools-v2/08` (migrate statusgen's reads onto it under minted custody —
**human gate**, and the `scanloop`-in-container fix).

The real blocker at the head is **01**, and it is a verified head, not an assumed one: the
desk-side native client foundation already exists (`forge_github.go` runs on `go-gh` with a
minted token and refuses an empty one, checked on `origin/main` @ `e9fa19d3`), and the desk
verbs are mostly migrated. What the 2026-09-16 audit showed is that the *weight* is elsewhere
— statusgen, a separate binary shelling `gh` with no native client — so the pacing chain is
`01 → 07 → 08`: nothing can migrate statusgen until `deskkit` is importable (07), and 07 needs
the inventory (01). The tempting-but-wrong first step is to treat #1223 as a desk-verb swap;
it is mostly done there, and the load-bearing work is the statusgen port behind the
importable-library enabler. A parallel human-gated custody chain, `01 → 03 → 06`, runs beside
it (the desk read-client custody contract and installation-token scoping).

## Dependency waves

- **Wave 1** — `desktools-v2/01` (no dependencies; the file:line audit across `statusgen/**`,
  `tools/desk/**`, `tools/cellctl/**`, skills and workflows — the frozen checklist the
  migration briefs reference by row).
- **Wave 2** — `desktools-v2/02` (ban-lint + seam contract, advisory first, scope includes
  `statusgen/**`), `desktools-v2/03` (native read-client custody contract + the 5 desk `gh`
  exceptions — human-gated), and `desktools-v2/07` (promote `deskkit` to an importable shared
  library — the enabler). All depend on 01; parallelizable.
- **Wave 3** — `desktools-v2/04` (deskclose query; depends 02+03), `desktools-v2/05`
  (deskpushguard remote name + `insteadOf`; depends 02), `desktools-v2/06`
  (installation-token scoping; depends 03 — human-gated), `desktools-v2/08` (statusgen reads
  onto the shared Forge under minted custody; depends 07+03 — human-gated), and
  `desktools-v2/09` (purpose-built access-pattern query layer; depends 02+07). Each migrates a
  leak site or adds a tuned operation and removes its reach-past in the same change.

Critical path: `01 → 07 → 08` (with the parallel custody chain `01 → 03 → 06`).

## Relationship to the sibling streams

- **`forge-neutral`** owns the forge *write* path + the resolver/custody (`forge-neutral/01`);
  v2 consumes that resolver and never re-implements it. v2's own contribution is the *ban*
  (extended to statusgen), the *importable shared library*, the *custody-first native read
  clients*, and the *access-pattern query layer*. See [spec.md](spec.md) §4.
- **`desktools-go-git`** owns *git*-transport migration; v2 owns the forge-assumption half of
  the push-guard fix (#1201/#884) and coordinates the transport half.
- **`desk-tools`** is the general planning board for the current suite; v2 is the
  architectural successor for the forge-abstraction slice only.
