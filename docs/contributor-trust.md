# Contributor trust tiers

This page states the whole bar a pull request or issue from outside the
operator's own roster is measured against. It is the published half of the
contributor-trust model: the tier names, what each unlocks, and how an identity
moves between them. It never publishes the rows themselves — see
[Where the rows live](#where-the-rows-live) below.

## The four tiers

Four tiers, ordered: `unknown` < `blessed-once` < `contributor` < `maintainer`.

- **`unknown`** — the default. Every identity nobody has recorded anything about,
  including everybody outside the operator's own roster today. This is the
  entire bar as it exists before this model, unchanged for anyone the operator
  has not deliberately promoted.
- **`blessed-once`** — a maintainer admitted one specific item, and nothing more.
  It expires with that item: landing it, or closing it, does not carry the grant
  forward to the identity's next submission.
- **`contributor`** — a standing grant a maintainer recorded after seeing the
  identity's work land and judging it sound. Unlike `blessed-once`, it persists
  across submissions until a maintainer records a demotion.
- **`maintainer`** — the operator's existing roster (unchanged by this model).
  It is resolved from the roster, never from the ledger, and no ledger row can
  lower it.

A tier is a grant of **automation**, never of merge authority. The highest tier
merges nothing; merging a change stays a human act behind branch protection,
whatever tier its author holds.

## What each tier unlocks

The grant is small and fixed on purpose. Nothing outside this table is a tier
capability:

| Tier | review-depth | fork-ci-auto-run | desk-automation | changelog-proxy |
|------|--------------|-------------------|------------------|------------------|
| unknown | no | no | no | no |
| blessed-once | yes | no | no | no |
| contributor | yes | yes | yes | yes |
| maintainer | yes | yes | yes | yes |

- **review-depth** — whether the review lane runs the deeper, claims-versus-diff
  check rather than the default pass.
- **fork-ci-auto-run** — whether continuous-integration workflows on the
  identity's pull request run without a maintainer clicking approve first.
- **desk-automation** — whether desk automation may act on the identity's items
  at all (dispatch, comment, label) without a maintainer present.
- **changelog-proxy** — whether the release aggregator accepts a
  maintainer-supplied changelog fragment proxying for the identity's own, on the
  fork-PR paths that cannot touch the contributor's branch directly.

`blessed-once` unlocks only the one-time admission's own review depth: it is
scoped to the single item a maintainer named, never a standing grant, which is
exactly the conflation the present per-item bless invites and this tier exists
to close.

## How an identity moves

Promotion and demotion are **recorded human acts with reasons** — no counter
promotes an identity automatically. A burst of trivial accepted changes must
never buy workflow auto-approval on its own, and account compromise is
precisely the case where an identity's prior good behaviour predicts nothing.
The structured act that writes a `blessed-once` or `contributor` row, with a
scope, a reason and an audit trail, is `contributor-trust/03`.

## Where the rows live

**The rows are never published.** The ledger — which identity holds which tier,
against which repository, recorded by whom and why — is operator-side
configuration, held next to the roster the operator already keeps outside this
repository. It is not a file in this tree, it is not reviewable by pull request,
and no tool writes it except the structured blessing act.

This is a deliberate asymmetry: a tier is a trust judgement about a named
external person, and a demotion is an adverse one. Publishing those judgements
in a public, append-only, never-deleted register would put a permanent public
mark on an individual, out of all proportion to the workflow problem this model
solves. What is published, in full, is the **model** on this page — the tier
names, what each unlocks, and how one moves. The rows never are.

An absent, unreadable, or malformed ledger resolves every identity to `unknown`
— today's bar — never to something wider. A ledger failure can only narrow what
automation does, never widen it.

## This model is inert until a row is written

Adopting these tiers changes nothing on its own. With no ledger configured,
every identity outside the operator's roster resolves `unknown`, exactly as it
does today — the model is opt-in by construction, and nothing reads it until a
maintainer records the first promotion.
