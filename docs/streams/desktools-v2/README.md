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
2. **The read path covers statusgen — across the `deskread` verb boundary.** statusgen shells
   `gh` directly (the `scanloop`-in-container break, #628). It reaches the seam by RUNNING the
   `deskread` verb, never by importing `deskkit` (`statusgen/forgeread.go`), and that migration
   is owned by the sibling brief `forge-neutral/18`. v2 does not redo it: it brings
   `statusgen/**` under the ban so the progress is measurable, then holds the zero.
3. **Purpose-built queries** — typed access-pattern operations (review-queue snapshot, head-sha
   batch, board sweep), each backend one tuned query: N+1 → one round-trip, rate-limit headroom,
   and one consistent snapshot (freshness), with the GraphQL document never crossing the seam.

A fourth commitment was added on 2026-09-17 ([spec.md](spec.md) §8): **one outbound-write
check at the write seam**. What a deployment may write to a forge is enforced by the tools,
keyed on the target's visibility, at the one place every write already passes — not remembered
per verb, and not carried as prose in a skill.

Boundaries with the sibling streams (`forge-neutral`, `desktools-go-git`, `desk-tools`) and the
open questions for the approver are in [spec.md](spec.md) §4/§7.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line)](brief-01-audit-and-inventory.md) | 1 | M | implemented | — | — |
| 02 | [the v2 seam contract + the ban-lint (advisory/counting first)](brief-02-seam-contract-and-ban-lint.md) | 2 | M | implemented | — | — |
| 03 | [native read-path installation-token client — retire gh shell-out in the read path (#1223 pilot)](brief-03-native-read-client.md) | 3 | M | todo | — | — |
| 04 | [deskclose reads an authorizing comment by its stated kind — retire the kind-less default (#1019)](brief-04-deskclose-authorization-read-kind.md) | 2 | S | implemented | — | — |
| 05 | [the push guards judge the remote actually being pushed to — deskpushguard base ref (#1201) and insteadOf in the push-transport gate (#884)](brief-05-push-guards-judge-the-real-remote.md) | 2 | M | todo | — | — |
| 06 | [installation-token scoping — one token per repo, no ambient-credential hiding (#628 / #1145 / #1146)](brief-06-installation-token-scoping.md) | 4 | M | todo | — | — |
| 08 | [hold statusgen at zero — the gh ban fails on statusgen and the scan is proven with no gh present](brief-08-hold-statusgen-at-zero.md) | 6 | S | todo | — | — |
| 09 | [purpose-built access-pattern query operations (one tuned snapshot, not N per-item calls)](brief-09-access-pattern-queries.md) | 3 | L | todo | — | — |
| 10 | [one outbound-write check at the forge write seam, keyed on the target's visibility](brief-10-one-outbound-write-check.md) | 2 | L | todo | — | — |
| 11 | [a house callout for the outbound-write check — deployment vocabulary stays out of the shipped tools](brief-11-outbound-house-callout.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`desktools-v2/01` (audit & inventory) -> `desktools-v2/02` (the ban-lint and its recorded
baseline) -> `desktools-v2/03` (the native read client's custody contract — **human gate**) ->
`desktools-v2/06` (installation-token scoping — **human gate**).

That is the longest chain, four deep with two human gates on it, so it paces the stream. A
second chain runs beside it and is the one the 2026-09-17 direction added:
`desktools-v2/01` -> `desktools-v2/10` (one outbound-write check — **human gate**) ->
`desktools-v2/11` (the house callout for it — **human gate**).

**The real blocker at the head, verified rather than assumed.** Two things stand in front of
brief 01, and neither is a brief. First, this stream is `parked` behind a `draft` spec: nothing
is dispatchable until a human rules the spec `approved`. Second, inside the stream the head is
**01**, and that was checked against the tree: `forge_github.go` already runs on `go-gh` with a
minted token and refuses an empty one, `TestForgeSingleConstructionSite` already pins the one
place a `Forge` is built, and every outward verb already obtains its `Forge` there — so neither
chain is waiting on a foundation that does not exist.

An earlier draft of this README put a different chain here — promote `deskkit` to an importable
library, then port statusgen onto it. That was the tempting-but-wrong first step: it was derived
without reading `statusgen/forgeread.go` or `forge-neutral/18`, which record the opposite
decision and already own that migration ([spec.md](spec.md) §2 Principle 2, §4). Brief 07 is
withdrawn and its number is not reused. `desktools-v2/08` is the one brief here that waits on
another stream: it cannot start until `forge-neutral/18` reaches zero `gh` sites in statusgen
(26 remained on 2026-09-17), which is why it sits in the last wave and on no critical path.

## Dependency waves

- **Wave 1** — `desktools-v2/01` (no dependencies; the file:line audit across `statusgen/**`,
  `tools/desk/**`, `tools/cellctl/**`, skills and workflows, plus the outward-writes table).
- **Wave 2** — `desktools-v2/02` (ban-lint + seam contract, advisory first, scope includes
  `statusgen/**`, baseline written to a file), `desktools-v2/04` (deskclose's authorization
  read states its kind, #1019), `desktools-v2/05` (the push guards judge the real remote,
  #1201 / #884) and `desktools-v2/10` (one outbound-write check — human-gated). All depend on
  01 only; parallelizable.
- **Wave 3** — `desktools-v2/03` (native read-client custody contract + the 5 desk `gh`
  exceptions; depends 01+02 — human-gated), `desktools-v2/09` (purpose-built access-pattern
  queries; depends 02) and `desktools-v2/11` (the house callout; depends 10 — human-gated).
- **Wave 4** — `desktools-v2/06` (installation-token scoping; depends 02+03 — human-gated).
- **Wave 6** — `desktools-v2/08` (hold statusgen at zero; depends 02 and the sibling
  `forge-neutral/18`, which is wave 5 of its own stream — the wave number follows that edge).

Critical path: `01 → 02 → 03 → 06`, with the outbound-write chain `01 → 10 → 11` beside it.

## Relationship to the sibling streams

- **`forge-neutral`** owns the forge *write* path, the resolver/custody (`forge-neutral/01`)
  **and statusgen's forge path** (`forge-neutral/07`, `/08`, and `/18` — statusgen off `gh`
  through the `deskread` verb, in progress). v2 consumes the resolver, waits on `/18`, and
  re-implements neither. v2's own contribution is the *ban* (extended to statusgen), the
  *custody contract* on the native read client, the *access-pattern query layer* and the
  *outbound-write check*. See [spec.md](spec.md) §4.
- **`desktools-go-git`** owns *git*-transport migration; v2 owns the forge-assumption half of
  the push-guard fixes (#1201/#884) and coordinates the transport half.
- **`desk-tools`** is the general planning board for the current suite; v2 is the
  architectural successor for the forge-abstraction slice only.
