---
stream: contributor-trust
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: []
board: generated
spec: docs/streams/contributor-trust/spec.md
---

# contributor-trust Stream

Be ready for the people and the agents who send changes to a public repository. Today the
inbound bar is binary — an identity is in the operator's roster or it is a stranger, and one
item is blessed or it is quarantined — so a contributor whose third change is landing is
assessed exactly like an account nobody has seen, an unknown author's body claims are read
with a maintainer's credence, and the one rule protecting the build runners from an untrusted
head is a sentence somebody has to remember. This stream adds a tier vocabulary, a place to
record it, a structured act that writes it, review depth and continuous-integration posture
keyed on it, and a published statement of the whole bar.

## Why now

The first unsolicited pull requests from unknown accounts arrived on 2026-09-12. The shape,
recorded in [the scoping document](spec.md) §1 and worth stating precisely because it is what
the controls are designed against: a long-dormant account resumed activity and opened roughly
eleven pull requests across roughly eleven unrelated repositories inside twelve hours, each
fork → branch → pull request in three to five minutes, bodies near-identical in shape and all
asserting some variant of "fixed, all tests pass", at least one of those assertions provably
false against its own diff, several closed by the receiving maintainers within hours — and the
changes themselves small and benign.

Every item in that list has an innocent explanation on its own. Together they describe an
automated sweep arriving faster than a review queue absorbs, carrying claims nobody checked
before writing them. The failure being designed against is not malice; it is **an unverified
claim entering a review queue at machine speed**, with a maintainer's attention as the only
thing between it and a merge. Nothing here is a judgement about any account, and nothing in
this stream names one.

## What is measured, not assumed

Measured on `origin/main` at authoring (2026-09-12 @ `e96b7f6d`):

- The author bar is operator-configured trusted logins, a blessing authority, a human-login
  map and trusted bot slugs. Unset, it trusts nobody; a human login with no pinned numeric id
  is refused on the strict path. There is nothing between trusted and untrusted.
- The blessing is recognised from **authorship alone**: any comment by the configured
  authority admits the item. It is bless-then-edit aware — content added after the latest
  blessing comment voids it — and it carries no marker, no scope, no reason and no audit row.
- `.github/workflows/inbound-triage.yml` already comments and labels on issue-first and
  per-author concurrency. Both enforcement switches default off, it uses
  `pull_request_target`, and its safety rests on never checking out the pull request's code —
  a property recorded in a comment, asserted by nothing.
- Sixteen workflow files; no audit of which of them a fork head could reach a secret through.
- "Never build or test an unblessed fork head" is a resident rule with **no enforcement of any
  kind**: broken, it reports nothing anywhere.
- Review dispatch reads no author property at all. Every pull request gets the same lane set.
- `CONTRIBUTING.md` is accurate about issue-first, the three-open-pull-request guideline,
  maintainer-merge and automation-ignoring-strangers, and silent on everything above. There is
  no pull-request template.

## The seam — decided here

**Tiers gate automation, never judgement, and never merge.** A tier decides how much machinery
runs unattended: review depth, fork approve-and-run posture, whether desk automation may act
on an identity's items, changelog-proxy eligibility. The highest tier merges nothing; merge
stays a human act behind branch protection.

**The ledger is operator-side, never a file in this tree.** Its rows are trust judgements
about named external people, and this repository is public and its registers are append-only.
A demotion recorded here would be a permanent public mark on an individual, out of all
proportion to the workflow problem being solved. The **model** is published in full — tier
names, what each unlocks, what moves an identity, which signals are measured and which are
deliberately excluded. The **rows** never are.

**Signals are facts; verdicts are people's.** Every mechanical output here states what was
measured and abstains from concluding, because each signal is a weak correlate of bulk
submission with an ordinary innocent explanation, and a design that lets the instrument
conclude converts weak correlations into a strong-looking statement about a person.
Could-not-check is a third state everywhere and is never rounded to clean.

**Promotion and demotion are recorded human acts with reasons.** No counter promotes; a burst
of trivial accepted changes must not buy workflow auto-approval, and account compromise is
precisely the case where prior good behaviour predicts nothing.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Contributor provenance probe — mechanical signals about an unknown author, rendered as a neutral card](brief-01-provenance-probe.md) | 0 | M | todo | — | — |
| 02 | [Trust tiers + the contributor ledger — one vocabulary for how much automation an external identity gets](brief-02-trust-tiers-and-ledger.md) | 0 | M | todo | — | — |
| 03 | [`deskbless` — a structured blessing act with a machine marker, a scope, a reason and an audit row](brief-03-bless-verb-and-audit.md) | 1 | M | todo | — | — |
| 04 | [Review depth by tier — an unknown author's pull request gets a claims-versus-diff fact check and a fail-first reproduction](brief-04-review-depth-by-tier.md) | 1 | M | todo | — | — |
| 05 | [Fork-safe continuous-integration posture — audited workflows, tier-keyed approve-and-run, and never-build-unblessed enforced by a check](brief-05-fork-safe-ci-posture.md) | 2 | M | todo | — | — |
| 06 | [Contributor-facing documents — state the bar honestly, and ask for verification rather than assertion](brief-06-contributor-facing-docs.md) | 0 | S | todo | — | — |
| 07 | [Agent contributors — disclosure of automated authorship, and tiering the operating human rather than the tool](brief-07-agent-contributors.md) | 1 | S | todo | — | — |
| 08 | [External-contribution metrics — inbound pull requests by tier and outcome, on the board](brief-08-external-contribution-metrics.md) | 1 | S | todo | — | — |
| 09 | [External-contributor credit in release notes — the aggregator names the author a fork change came from](brief-09-external-contributor-credit.md) | 1 | S | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

`contributor-trust/02` (tiers + ledger) → `contributor-trust/03` (the blessing verb) →
`contributor-trust/05` (fork continuous-integration posture).

**The head was verified before authoring, and it is not the obvious one.** The tempting first
move is the provenance probe — it is the most visible response to what arrived, and it can be
built against public metadata today. It is not the head, for two reasons that were checked
rather than assumed. First, the probe has nowhere to put its answer: with no tier vocabulary
and no ledger, a gathered signal set is a comment and nothing more, and every downstream
control still has to re-derive "is this author known?" from the same binary predicate.
Second, and decisively, the tempting home for the ledger is a register in this tree like every
other register here — and that home is wrong, because these rows name external people and this
tree is public and append-only. That is a design fork that has to be settled before anything
writes a row, not after.

So `02` is the head: it fixes the vocabulary, fixes where the record lives, and is **inert on
landing** — with no ledger configured every identity resolves exactly as it does today and no
existing caller's answer changes, which is what makes a trust-boundary change safe to land
before it is used. `03` follows because the blessing is the only act that writes the ledger,
and because the fork guard in `05` needs a blessing signal it can trust to key on: with
admission still recognised from free text, a mechanical never-build-unblessed check would gate
on a predicate that any casual comment satisfies. `05` is last on the path because it is the
only brief that changes who can cause code to execute on the project's runners.

Smallest unblocking move: rule on `02`'s human decision — where the ledger lives — and land it.
Briefs `01` and `06` are independent of the path and can run in the same wave; `04`, `07`, `08`
and `09` open as soon as `02` lands.

**Brief `09` sits in this stream, and off the critical path, on purpose.** It is the only brief
here that gives something back rather than asking for something: the release aggregator lifts
fragment bullets and nothing else, so a fix merged from a fork is credited nowhere in the
changelog or the release notes. It depends on `02` for one thing only — the single answer to
"is this author external?", which is the same question the rest of the stream asks and which
belongs in one place — and it blocks nothing, so it can be worked the moment `02` lands. The
credit is fail-closed toward silence: a resolution that cannot be made confidently produces no
name, because a wrong name in a published release is worse than a missing one, and no cut is
ever refused for want of a credit.

## Dependency waves

```
Wave 0: [01 provenance probe]  [02 tiers + ledger]  [06 contributor docs]
Wave 1: [03 bless verb] ← 02   [04 review depth] ← 02   [08 metrics] ← 02
        [09 release-note credit] ← 02   [07 agent contributors] ← 01, 02, 06
Wave 2: [05 fork CI posture] ← 02, 03
```

Critical path: `02 → 03 → 05`.

## Shared conventions

- **No external contributor is named anywhere** — not in a brief, not in a fixture, not in a
  test, not in generated output. Patterns are described; people are not. Every illustration
  uses an invented identity. The one place a real login is ever emitted is the release-note
  credit of `contributor-trust/09`, which names the author of a change the project merged, at
  that author's option and only forward.
- **Three states, everywhere.** Every instrument here reports checked-clean, checked-failed or
  could-not-check, and the third is never rounded to the first. A signal that could not be
  gathered is not an absence of concern.
- **Fail closed toward `unknown`.** An absent, unreadable or malformed ledger resolves every
  identity to `unknown`, which is today's bar — so a ledger failure can only ever narrow what
  automation does, never widen it.
- **Nothing here weakens a guard.** If greening a check would require removing or weakening a
  security control or its assertion, that is a stop and a human ruling, never a resolution.
- **No brief in this stream builds, tests or checks out an untrusted head**, including in a
  fixture. Metadata reads only.
- Every risk-gated brief cites a design-decision record under `docs/streams/decisions/`. All
  five are **PROPOSED**: they capture the design as authored so the design-approval gate has
  something to dereference, and each carries a placeholder approver until a human rules on the
  brief's decision issue.
- Public-tree self-containment: briefs here name no private repository, machine path or
  session.
