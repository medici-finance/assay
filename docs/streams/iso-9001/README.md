---
stream: iso-9001
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# iso-9001 Stream — make the artifacts an ISO 9001 adopter needs actually shippable

[`docs/iso9001-mapping.md`](../../iso9001-mapping.md) reads this repo's shipped artifacts
against the ISO 9001 clause skeleton and says, per clause, what exists, whether it is
enforced or advisory, and what the adopter must still supply. This stream is the *work* that
page's gaps became — the subset that is Assay's to close rather than the adopter's.

**The shape of the gap, in one line:** on the clauses that matter most to a certified
adopter — 7.1.5 monitoring resources, 8.6 release, 8.7 nonconforming output, 10.2 corrective
action — the expensive half is already built and the cheap half is missing. The mutation
gate that proves a control fires runs on **every release** and blocks it on a survivor, and
then emits nothing an adopter can hold. The release records who authorized it in a place
nobody reads. The findings register records that a corrective action was *taken* and never
that it *worked*. Three of the six briefs here are "emit, don't build".

**And one uncomfortable finding, which is 02.** On three shipped surfaces the documentation
now understates the code: the evidence bundle says no command is run and no output hash is
recorded when `verifyrun` records both plus the tree SHA; it calls segregation "a string
check" when `corroborate` disproves a false human stamp against the forge; and both
[`README.md`](../../../README.md) and [`FINDINGS.md`](../FINDINGS.md) claim contiguous
numbering is enforced, which [`docs/registers.md`](../../registers.md) explicitly forbids
claiming. The honesty discipline is strong enough that the self-descriptions have gone stale
in the *pessimistic* direction. That is harmless for the methodology and expensive for
anyone reading the corpus to decide whether to adopt it —
[`docs/mistake-proofing.md`](../../mistake-proofing.md) **B9** is precisely the standing rule
that would catch all three, and it is unimplemented.

## What this stream is not

It is not a bid for certification, and no brief here produces a compliance claim. Every
deliverable is an artifact an adopter can *cite in their own* management system, plus the
honest statement of what it does not establish. If a brief here ever starts to read like
marketing, it has gone wrong: the corpus's habit of stating its limits in the same breath as
its claims is worth more at an audit than any of the claims, and an ISO page written in
compliance-marketing voice would destroy it.

Four gaps the mapping names are **deliberately not** in this stream, because they are the
adopter's own and Assay must not appear to supply them: the quality policy (5.2), quality
objectives (6.2), the internal-audit programme (9.2), and the management review (9.3).

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Emit the tool-validation evidence pack as a release asset (7.1.5)](./brief-01-tool-validation-evidence-pack.md) | 0 | S | todo | — | — |
| 02 | [Align three shipped disclosures with the code they describe (B9)](./brief-02-disclosure-honesty-fixes.md) | 0 | S | done | 2026-09-01 opus-4.8[1m]-verifier | 2026-09-02 assay-reviewer-app[bot] (approved PR #318 @ 6ab8de53a40c1a4f71fa6c0a0ddccb4b27a000c8) |
| 03 | [A finding closes on a fired control — the effectiveness record (10.2)](./brief-03-corrective-action-effectiveness.md) | 1 | M | todo | — | — |
| 04 | [Record the authorizing human in the release itself (8.6)](./brief-04-release-authorizer-traceability.md) | 1 | S | todo | — | — |
| 05 | [Records control and retention, stated once (7.5.3)](./brief-05-records-control-and-retention.md) | 1 | S | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-05 assay-reviewer-app[bot] (approved PR #400 @ 90c19fd7a273835d01247292ad91f217a4ff9fe1) |
| 06 | [The auditor one-pager — what Assay is and is not](./brief-06-auditor-one-pager.md) | 2 | S | todo | — | — |

## Critical path

```
01 (emit the validation pack) ──┬──▶ 03 (effectiveness record) ──┐
                                └──▶ 04 (release authorizer) ────┼──▶ 06 (auditor one-pager)
02 (disclosure honesty) ───────────▶ 05 (records & retention) ───┘
```

**Critical path: `01 → 03 → 06`.** The head is **01**.

- **Why 01 is genuinely the head, verified not assumed.** 01 is the brief that first gives
  this repo a *shape* for a per-control evidence row — the control, the injected error that
  reddens it, the verdict, the date, the tool version. Nothing in the tree emits one today:
  `git grep -n 'tool-validation' -- .github/workflows/release.yml` returns nothing at
  `6871a3b`, and the six `muhar` reports the release already produces go to the job log and
  nowhere else. 03 reuses that row shape for a finding's effectiveness record rather than
  inventing a second one, and 06 cannot honestly describe what an adopter can cite until
  there is an artifact to cite. Nothing upstream blocks 01: the harness
  ([`tools/desk/cmd/muhar/`](../../../tools/desk/cmd/muhar/)), the six mutation specs and the
  six release-blocking assertions are all on `origin/main` today.
- **Why 04 depends on 01 rather than running beside it.** Both edit
  [`.github/workflows/release.yml`](../../../.github/workflows/release.yml). Two pull requests
  writing the same region of the same workflow are each green alone and red merged — the
  guard-hardening-versus-content interaction. 01 lands the asset-emitting steps; 04 adds the
  authorizer line over them.
- **Why 05 depends on 02.** 05 writes down what the register machinery actually guarantees.
  [`FINDINGS.md`](../FINDINGS.md) currently states that contiguity is enforced, which the
  registers page forbids claiming; writing 05 first copies that false claim into a
  records-control statement, which is the one document where a wrong enforcement claim is
  most expensive. 02 corrects the source first.
- **Smallest correct first move:** 01 and 02 in parallel. Both are contained changes against
  seams that exist on `origin/main` now, and neither depends on the other.
- **Tempting-but-wrong first step: starting with 06.** The one-pager is the most satisfying
  item and the wrong opening move. Written before 01, 04 and 05 it states a "what you can
  cite" list that three later briefs in this same stream change, so it is rewritten twice;
  worse, a claim-boundary page is the one document where going stale is the exact defect it
  exists to prevent.

## Dependency waves

```
Wave 0: [01, 02]
Wave 1: [03] ← 01,  [04] ← 01,  [05] ← 02
Wave 2: [06] ← {01, 03, 04, 05}
```

## Shared conventions (inherited by every brief)

- **Verify rows run from the repo root.** Every row is written repo-relative and is executed
  from the root of a checkout of this repo. A row that cannot resolve its target is
  **could-not-check**, not a pass — record it that way in Evidence rather than marking the
  row green ([`docs/three-state-instrument-rule.md`](../../three-state-instrument-rule.md)).
- **Verify rows were dereferenced at authoring time.** Every brief carries at least one row
  that resolves the CURRENT tree for the ABSENCE of the thing it proposes, and each was run
  against `origin/main` @ `6871a3b` on 2026-08-25. Those rows are expected to **invert** at
  implementation; a row that still reports absence after the work lands means the work did
  not land ([`docs/brief-rules.md`](../../brief-rules.md), "Verify row semantics:
  dereferencing vs. presence"). No row anchors on
  [`docs/iso9001-mapping.md`](../../iso9001-mapping.md), because that page lands in the same
  pull request as this stream and a row anchored on it would be green before any brief ran.
- **Three states, always.** Any new check or generator added by this stream reports
  checked-clean / checked-failed / could-not-check. The precedent to copy is the evidence
  exporter's: exit 3 means *exported but INCOMPLETE*, with the artifact carrying its own
  `omitted` list, because a silently incomplete compliance artifact is a worse outcome than a
  failed one.
- **No compliance claim, in any deliverable.** Nothing here asserts, implies, or is worded so
  as to be quotable as conformity, certification, or an audit opinion. Where a deliverable
  states what it establishes it states, in the same breath, what it does not.
- **Presence is the control; adequacy is review.** Where a brief adds a check it enforces
  that an artifact IS THERE; none of them judges whether it is any good, and each failure
  message must say which half it covers
  ([`docs/mistake-proofing.md`](../../mistake-proofing.md) **D7**).
- **A change that adds a check carries its mutation row.** Rule 16 binds here as everywhere:
  break the guarded thing, run the check, confirm it goes RED, and put that row in the table.
- **Workflow files are a permission surface.** Two briefs here (01, 04) change files under
  `.github/workflows/`. A bot installation token without the workflows permission cannot push
  such a diff at all — the push is refused outright rather than landing partially. If your
  write path refuses, that refusal is the answer: emit a patch and hand it to a push path
  that carries the permission. Do not route around it.

## Sources

- [`docs/iso9001-mapping.md`](../../iso9001-mapping.md) — the clause map these briefs close
  the Assay-side gaps of, and the statement of which gaps are the adopter's.
- [`docs/mistake-proofing.md`](../../mistake-proofing.md) — **D1** (a control must be shown to
  fire), **D6** (honesty about non-coverage is a device), **D7** (do not proof judgment),
  **B9** (authoring guidance is derived, never hand-copied).
- [`docs/three-state-instrument-rule.md`](../../three-state-instrument-rule.md) — the
  positive-control requirement, and the four-column instrument-register table shape
  (Instrument / what it prints when it cannot see / States / Disposition) that 01 populates.
- [`docs/brief-rules.md`](../../brief-rules.md) — rules 16 and 17, "dereferencing vs.
  presence", and the row-runner discipline every Verify row here is written against.
- [`docs/evidence-bundle.md`](../../evidence-bundle.md) — the export format, the
  Enforced/Advisory column, the exit-3-means-incomplete contract, and the disclaimer voice
  every deliverable here inherits.
- The standard-side reading behind the clause claims: ISO 9001's monitoring-resources clause
  bites where monitoring or measuring is used to verify the conformity of products and
  services, so a gate that can refuse a merge or a release is in scope; its measurement-
  traceability sub-clause is normally not applicable to software checks, and the known-answer
  corpus substitute is practitioner convention rather than standard text. The release clause
  asks in terms for traceability to the persons authorizing release. The corrective-action
  clause asks for the *results* of the action, not only the action.
- Freshness: `origin/main` read 2026-08-25 @ `6871a3b`. Every seam named in these briefs, and
  every DEREFERENCE row, was checked against that commit.
