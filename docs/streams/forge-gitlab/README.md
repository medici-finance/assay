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
design, the per-control security-parity table, and the tier floor (Premium;
Ultimate for public/risk-classed work; Free/CE non-conforming).

Coordination note: [desktools-go-git](../desktools-go-git/README.md) refactors
the same tools' **git-binary** seam while this stream refactors their **forge-API**
seam. The seams are disjoint, but both touch `tools/desk/**` broadly — land
forge-gitlab/01's extraction either before desktools-go-git's migration waves or
rebased across them; never concurrently with an in-flight migration brief of the
same tool.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [`Forge` interface extraction in deskkit — `github` impl pinned by goldens](brief-01-forge-interface-extraction.md) | 1 | L | implemented | — | — |
| 02 | [`gitlab` forge implementation (MRs, notes, approvals, statuses)](brief-02-gitlab-forge-impl.md) | 2 | M | todo | — | — |
| 03 | [GitLab token custody — rotate-on-mint + expiry backstop in desktoken](brief-03-gitlab-token-custody.md) | 2 | M | todo | — | — |
| 04 | [Fleet provisioning + adopter doc + ci-config-project runbook](brief-04-provisioning-and-adopter-doc.md) | 3 | M | todo | — | — |
| 05 | [Live pilot — one brief round-tripped on a real GitLab group; parity table walked](brief-05-live-pilot-parity-walk.md) | 4 | M | todo | — | — |
| 06 | [Ultimate refinements — custom reviewer role + external-status-check verdict lane](brief-06-ultimate-refinements.md) | 5 | M | todo | — | — |

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
