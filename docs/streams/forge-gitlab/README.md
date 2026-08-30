---
stream: forge-gitlab
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# forge-gitlab Stream

Run the Assay fleet on **GitLab Enterprise** with guarantees at least as strong as
the existing GitHub controls — per control, verified on a live deployment, even
where the mechanism is completely different. The methodology core (briefs,
registers, lifecycle, board, statusgen) is already forge-agnostic; this stream
delivers the identity/permission/CI layer (service accounts, protected-branch
gates, a locked ci-config project) and the tooling seam (a `Forge` interface in
deskkit with `github` extracted as-is and `gitlab` implemented against REST v4,
plus rotate-on-mint token custody). See [spec.md](spec.md) for the accepted
design, the per-control security-parity table, and the tier floor it recorded in
August 2026 (Premium; Ultimate for public/risk-classed work; Free/CE
non-conforming) — a floor the edition matrix below now puts back in front of a
human.

Coordination note: [desktools-go-git](../desktools-go-git/README.md) refactors
the same tools' **git-binary** seam while this stream refactors their **forge-API**
seam. The seams are disjoint, but both touch `tools/desk/**` broadly — land
forge-gitlab/01's extraction either before desktools-go-git's migration waves or
rebased across them; never concurrently with an in-flight migration brief of the
same tool.

## Edition — CE-first

**Recorded preference: Community Edition first.** Premium and Ultimate features are opt-in
refinements, never prerequisites for the core lane (01-05, 07, 08).
[edition-matrix.md](edition-matrix.md) states, per operation and with a GitLab docs citation
per row, which tier each thing needs. Its finding: every operation the `Forge` interface
performs is Free-tier, so the tooling and the pilot run on CE; what is tier-gated is a
handful of *guarantees* — identity-granular protected-branch allowlists and merge-request
approval rules (Premium), external status checks, custom roles and the instance token-lifetime
policy (Ultimate) — each with a named CE fallback the desk tools own.

Minimum tier per brief (the `tier:` line in each brief's front-matter, with the detail in its
`## Edition` section):

| Brief | 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08 |
|---|---|---|---|---|---|---|---|---|
| Minimum tier | free | free | free | free | free | ultimate | free | free |

Open point: spec.md section 1 declares Free/CE non-conforming, which the matrix's per-feature
citations do not support as written. Reconciling the two is a human ruling
(medici-finance/assay#219), not a model's; until it lands, spec.md keeps its wording and the
matrix records the evidence.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [`Forge` interface extraction in deskkit — `github` impl pinned by goldens](brief-01-forge-interface-extraction.md) | 1 | L | verified | 2026-08-26 opus-4.8[1m]-verifier | — |
| 02 | [`gitlab` forge implementation (MRs, notes, approvals, statuses)](brief-02-gitlab-forge-impl.md) | 2 | M | implemented | — | — |
| 03 | [GitLab token custody — rotate-on-mint + expiry backstop in desktoken](brief-03-gitlab-token-custody.md) | 2 | M | todo | — | — |
| 04 | [Fleet provisioning + adopter doc + ci-config-project runbook](brief-04-provisioning-and-adopter-doc.md) | 3 | M | todo | — | — |
| 05 | [Live pilot — one brief round-tripped on a real GitLab group; parity table walked](brief-05-live-pilot-parity-walk.md) | 4 | M | todo | — | — |
| 06 | [Ultimate refinements — custom reviewer role + external-status-check verdict lane](brief-06-ultimate-refinements.md) | 5 | M | todo | — | — |
| 07 | [GitHub forge backend on `go-gh` — retire the exec-`gh` shell path](brief-07-github-forge-go-gh.md) | 2 | M | implemented | — | — |
| 08 | [Close the forge surface — enumerated operations, no passthrough, shell-exec ban](brief-08-close-the-forge-surface.md) | 3 | M | todo | — | — |

## Critical path

`forge-gitlab/01` (interface extraction — zero-behavior-change, golden-pinned) ->
`forge-gitlab/02` (gitlab implementation) -> `forge-gitlab/04` (provisioning +
docs) -> `forge-gitlab/05` (live pilot — **human gate**: a real Premium/Ultimate
group, real credentials, and the security-parity walk recorded as Evidence).

The head is genuinely 01 and it is unblocked: the desk tools and deskkit are in
this repository and no upstream release gates the extraction. The pacing item is
**05** — nothing may claim GitLab support before the pilot round-trips one brief
todo→done and walks the parity table on live settings. The tempting-but-wrong
first step is writing the GitLab REST code first (02 before 01): without the
extracted interface and goldens, the GitHub behavior has no pinned contract to
stay equal to, and every later refactor re-litigates it.

Parallel to the pilot chain runs a **surface-hardening sub-track** off the same
seam: `forge-gitlab/07` (re-seat the GitHub backend on the official `go-gh`
library, retiring the exec-`gh` forge path) then `forge-gitlab/08` (close the
surface — enumerated operations only, no arbitrary-endpoint passthrough on
either backend, and a checked ban on `gh`/`glab` shelling across `tools/desk`).
It depends only on the interface (01) and the two backends (02 for the symmetric
`glab` side, 07 for the GitHub side); it does not gate — and is not gated by —
the live pilot (05). It is where the spec's "constrained typed surface is
*stronger* than an ambient full-CLI surface" (§3) becomes shipped, enforced
configuration rather than a design intention.

## Dependency waves

- **Wave 1** — `forge-gitlab/01` (no dependencies; the seam + goldens everything
  else builds on).
- **Wave 2** — `forge-gitlab/02`, `forge-gitlab/03` (depend on 01;
  parallelizable — the API implementation and the credential machinery are
  separable deliverables).
- **Wave 3** — `forge-gitlab/04` (depends on 02 + 03; provisioning script,
  adopter doc, ci-config-project runbook).
- **Wave 4** — `forge-gitlab/05` (depends on 04; the human-gated conformance
  pilot).
- **Wave 5** — `forge-gitlab/06` (depends on 05; Ultimate-tier refinements,
  scoped by what the pilot surfaces).

Surface-hardening sub-track (parallel, off the interface — not on the pilot
critical path):

- **Wave 2** — `forge-gitlab/07` (depends on 01; GitHub backend re-seated on
  `go-gh`, exec-`gh` forge path retired — parallelizable with 02/03).
- **Wave 3** — `forge-gitlab/08` (depends on 07 + 02; enumerated surface, no
  passthrough on either backend, checked `gh`/`glab` shell-exec ban).
