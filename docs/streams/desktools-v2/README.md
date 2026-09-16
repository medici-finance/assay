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

Rebuild the desk tools' forge access **properly**, now that the flows have solidified. The
tools were written ad-hoc, one verb at a time, while the flows still changed weekly; the
ad-hoc origin now shows as one recurring bug — **GitHub-specific facts hardcoded past the
`Forge` seam, each generating its own defect** (#1145, #1146, #628, #1019, #1201, #884,
#1223; the shell-exec ban ratchet #305/#834/#838). A `Forge` interface already exists
(`tools/desk/internal/deskkit/forge.go`, two complete backends); the defect is that callers
reach *around* it. v2 makes reaching around it **structurally impossible** — a ban-lint, a
native installation-token client so no read shells `gh`, and tool-by-tool migration with the
old path removed as each lands. Full thesis, boundaries with the sibling streams
(`forge-neutral`, `desktools-go-git`, `desk-tools`), and open questions for the approver:
[spec.md](spec.md).

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
<!-- statusgen:briefs:end -->

## Critical path

`desktools-v2/01` (audit & inventory — the frozen file:line table every migration ticks
against) -> `desktools-v2/03` (native read-path installation-token client — **human gate**,
the credential-custody review) -> `desktools-v2/06` (installation-token scoping — **human
gate**, the per-repo token-isolation review).

The real blocker at the head is **01**. It is verified head, not an assumed one: the native
read client (#1223) is *not* blocked by an unproven upstream — `forge_github.go` already runs
on `go-gh` with an explicitly-minted token and refuses an empty one, so the client foundation
exists on `origin/main` today. What is missing is the enumeration of *which* read sites still
reach around it, which is exactly what brief 01 produces. Nothing downstream can be scoped
without that inventory, so 01 is the true head; the two human gates on 03 and 06
(credential-custody reviews) are the pacing items behind it.

## Dependency waves

- **Wave 1** — `desktools-v2/01` (no dependencies; the file:line audit of every `gh`
  shell-out and hardcoded-forge-assumption site across Go, shell, and skills — the frozen
  checklist the migration briefs reference by row).
- **Wave 2** — `desktools-v2/02` (depends on 01; the ban-lint + seam contract, advisory
  first) and `desktools-v2/03` (depends on 01; the native read-path client — human-gated
  credential-custody review — that the read migrations build on). Parallelizable.
- **Wave 3** — `desktools-v2/04` (depends on 02 + 03; deskclose query shape),
  `desktools-v2/05` (depends on 02; deskpushguard remote name + `insteadOf`), and
  `desktools-v2/06` (depends on 03; installation-token scoping — human-gated). Each migrates
  one leak site and removes its reach-around in the same change.

Critical path: `01 → 03 → 06`.

## Relationship to the sibling streams

- **`forge-neutral`** owns the forge *write* path + the resolver/custody (`forge-neutral/01`);
  v2 consumes that resolver and never re-implements it. v2's own contribution is the *ban*
  and the *native read client*. See [spec.md](spec.md) §3.
- **`desktools-go-git`** owns *git*-transport migration; v2 owns the forge-assumption half of
  the push-guard fix (#1201/#884) and coordinates the transport half.
- **`desk-tools`** is the general planning board for the current suite; v2 is the
  architectural successor for the forge-abstraction slice only.
