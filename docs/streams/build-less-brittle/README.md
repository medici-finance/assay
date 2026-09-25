---
stream: build-less-brittle
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: [1660]
board: generated
spec: docs/streams/build-less-brittle/spec.md
---

# build-less-brittle: change the processes that grow brittle code

The desk machinery grows symptom by symptom. Intake files "X refused Y", a worker fixes exactly
that, and review checks the fix is correct but not whether it is in the right place or should
exist. Nothing in the loop removes anything, so each fix can become the next bug: #1145 → #1274
→ #1631 → #1650 is one credential chain. This stream changes **intake, authoring, the worker and
review**, not the tools. It moves the unit of work from the symptom to the mechanism, and it
makes weight visible and approved.

The spec ([`spec.md`](spec.md)) is approved by the driver's merge of the pull request that lands
this stream, which also lands briefs 01–13 citing it, so its header reads `**Status:** routed`
(`spec/lifecycle-v1.md` §8.2, §8.4).

## End state

- Symptoms attach to `error-class` issues. A class that recurs gets a design brief, not a
  point fix.
- Every new brief carries `design-fit:` (owner, contract, retires, weight, why-add).
- A worker's second fix in a class becomes a design note. Every PR body reports measured
  `## Weight`.
- Any PR that adds weight or a rule passes a strong-tier design-fit stage first. `design-fit`
  is a scope basis and a finding class: advisory at landing, blocking by a later recorded
  decision (D-A), then calibrated by the existing reversal-rate rule.
- `go test` reports (advisory at landing) and, once promoted, fails when a ratcheted
  dimension grows past its ceiling without the driver's `grow` reply.
- `docs/contracts.md` names one owner per meaning and registers every rule. A monthly diet
  asks keep-or-retire in one reply and never deletes by itself.
- Every skill and kit this stream edits ends at or below its starting line count.
- A hotspot report from git history (churn × complexity, temporal coupling) nominates modules;
  class defect history confirms; a `brittle` mark lands in `docs/contracts.md` and on the
  board as a findings entry. Every mark binds to one strong-tier investigation that reads the
  original intent against what happened and recommends reconcile, redesign or accept.
- Three architectural fitness functions run as `go test`: dependency direction, the hub's
  import allow-list, and one declared implementation per registered meaning.
- Regression tests are a ratchet: every fix's test carries `// regression: #<N>`, a tagged
  test leaves only with a `Retires-test: <name> — <why>` trailer, and a `go test` report
  lists every departure without one for the reviewer to judge. It never blocks by itself.
- Before any `redesign`, an oracle: intent (the investigation's reconciled reading), failure
  modes mapped to their tagged tests, characterization tests generated over the public
  surface and triaged keep / drop / unknown, the invariants, and an acceptance rule with the
  triage recorded in the PR body.
- On a threshold class, a brittle mark or a two-strikes stop, one strong-tier session runs
  the investigation, assembles the oracle and drafts the refactor PR itself, asking the
  driver exactly one typed decision, only for what the record does not settle.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Semantic-owner index in docs/contracts.md — one meaning, one home](brief-01-semantic-owner-index.md) | 0 | M | todo | — | — |
| 02 | [design-fit: in every new brief — owner, contract, retires, weight, why-add](brief-02-design-fit-in-briefs.md) | 1 | M | todo | — | — |
| 03 | [Weight counter and CI ratchet (a Go test, not a verb)](brief-03-weight-ratchet.md) | 0 | M | implemented | — | — |
| 04 | [Intake files by error class; a recurring class triggers a design brief, not another point fix](brief-04-intake-by-error-class.md) | 2 | M | todo | — | — |
| 05 | [Worker two-strikes: the second fix in a class becomes a design note; PR bodies report measured weight](brief-05-worker-two-strikes-and-weight.md) | 3 | M | todo | — | — |
| 06 | [Design-fit review stage: right layer, should it exist, what does it replace — before correctness; advisory at landing, blocking by a later recorded decision](brief-06-design-fit-review-stage.md) | 2 | M | todo | — | — |
| 07 | [Rule register (owner, invariant, justifying issue, catch source) and the monthly rule diet](brief-07-rule-register-and-diet.md) | 1 | M | todo | — | — |
| 08 | [Hotspot metric (churn × complexity, temporal coupling) from git history, and the brittle mark](brief-08-hotspot-metric-and-brittle-mark.md) | 1 | M | todo | — | — |
| 09 | [Brittle investigation: a strong-tier task template that reads the original intent and the issues found, and recommends reconcile, redesign or accept](brief-09-brittle-investigation.md) | 3 | M | todo | — | — |
| 10 | [Architectural fitness functions in CI: dependency direction, the hub's import allow-list, and one implementation per registered meaning](brief-10-fitness-functions.md) | 2 | M | todo | — | — |
| 11 | [Regression tests are a ratchet: every fix ships a tagged bug-reproducing test, a tagged test leaves only with a Retires-test: trailer, and a report lists what left untagged for review to judge](brief-11-regression-test-ratchet.md) | 4 | M | todo | — | — |
| 12 | [The refactor oracle: what a redesign is coded against — intent, failure modes, triaged characterization tests, invariants, and an acceptance rule](brief-12-refactor-oracle.md) | 5 | M | todo | — | — |
| 13 | [Incident-time refactor by an agent, with one human residue: the strong-tier session runs the investigation, assembles the oracle, drafts the refactor PR, and asks the driver exactly one typed decision, only for what the record does not settle](brief-13-incident-refactor-by-agent.md) | 6 | L | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

```
01 semantic index ──► 02 design-fit in briefs ──► 04 intake by class ──► 05 worker two-strikes ──► 11 test ledger ──► 12 oracle ──► 13 agent refactor
        │                       │                        │                     ▲                   ▲                 ▲              ▲
        ├──► 07 rule register ──┴──► 06 design-fit review stage ───────────────┼───────────────────┘                 │              │
        │            │                                   │                     │                                     │              │
        │            └──► 10 fitness functions ──────────┼─────────────────────┼─────────────────────────────────────┘              │
        │                                                └──► 09 brittle investigation ──────────────────────────────┴──────────────┘
        └──► 08 hotspot metric + brittle mark ───────────────┘
03 weight ratchet ──────────────────► 06, 07 ──────────────────────────────────┘
```

**Longest chain:** `01 → 02 → 04 → 05 → 11 → 12 → 13` (seven briefs). Before the
third-pass amendment it was `01 → 02 → 04 → 05` and, of equal length, `01 → 02 → 04 → 09`.
An adopting project's go-live needs 02 and 04–07 merged, then a release that embeds the
amended kits; the three later briefs reach the installed binary at the release after that.

**Head: brief 01.** The spec approval rides the merge that lands this stream (spec §10 D-C);
after it, 01 and 03 dispatch at once.

**Tempting-but-unverified step: 04's parking.** The procedure parks design-owed symptoms with
the existing `blocked` status. Next-up does skip non-`todo` rows (`statusgen/nextup.go:408, 699`)
and `blocked` is a valid token (`statusgen/checks.go:16`). What is **not** verified is whether
the issue scanner preserves a hand-set status when it regenerates a placeholder. 04's Task
step 1 checks this first. If the scanner overwrites it, 04 stops with NEEDS_CONTEXT, and a
scanner fix becomes the real head of the 04 → 05 chain.

**Verified at authoring (2026-09-24 @ `f7bde6bfa`):**

- `ci.yml` runs `go test ./...` in `tools/desk` on push and pull_request, so 03 needs no
  workflow change.
- `deskfile attach` exists, refuses closed issues, and is unbudgeted. `deskfile new` is
  budgeted at 3 per 24h.
- `ScopeBases()` is pinned to the review kit's `reviewscope` block by
  `TestReviewScopeKitMatchesModel`, so 06 must change both together.
- `TestDefectClassClauseIsOneWordingAcrossImplementerKits` requires the token `ALLOW-LIST` in
  worker clause 14. The add-a-guard obligation is itself pinned by a test, so 05 retires the
  token with the obligation.
- An edge into another repository's stream reads as "unknown stream" to a single-root lint.
  A project that adopts this stream from its own tracker therefore depends on it through
  `facts:` preconditions only.

## Dependency waves

| Wave | Briefs | Starts when |
|---|---|---|
| 0 | 01, 03 | this stream merged (the spec approval) |
| 1 | 02, 07, 08 | 01 merged (07 also after 03) |
| 2 | 04, 06, 10 | 02 merged (06 also after 03 and 07; 10 after 01 and 07) |
| 3 | 05, 09 | 03 and 04 merged (09 after 04 and 08) |
| 4 | 11 | 05, 06 and 07 merged (it edits the same kits and register) |
| 5 | 12 | 09, 10 and 11 merged |
| 6 | 13 | 05, 09 and 12 merged; its rehearsal needs the driver's one `residue: default` reply |

**Third-pass amendment (2026-09-24).** Briefs 11–13 were added, and two open decisions were
settled (spec §10): D-A, the ratchet and `design-fit` start advisory for a month and are
promoted by a later recorded decision keyed to the project's baseline measurements (03, 06
amended); D-B, two-strikes is a hard stop with `bleed` only for production-down or security
fixes (05 amended). The driver ratified both from their own account on #1660
([ratification comment](https://github.com/medici-finance/assay/issues/1660#issuecomment-5826448154), 2026-09-25). Approving the spec itself (D-C) is still the
driver's merge of the pull request that lands this stream. The three briefs answer "what do we refactor
against": briefs plus incidents are not sufficient; the answer is a tagged, ratcheted
regression suite (11), an oracle assembled before any redesign (12), and one strong-tier
session that runs the loop with one human residue (13).

**SOTA amendment (2026-09-24, second pass).** Briefs 08–10 were added after a survey of what
mature teams do about brittle code (spec §11, sources inline): a hotspot ranking
from git history rather than complexity alone, a mark that binds to an actionable
investigation, and executable architecture rules. The "no logic in main()" shape rule belongs
to a separate desk-tool redesign and is deliberately not here; the rule that is here is "one shared
meaning, one implementation".

## Shared conventions

- **Net ≤ 0 lines** for every skill or kit a brief edits. Each such brief carries a line-count
  row that compares the file at the brief's own change against the file just before it. The
  counts quoted in `facts:` are for scale only; main moves, so no row caps at a typed number.
- **Rows that compare against "before this brief" derive their base in the command, never
  `HEAD~1`.** On merged main, the change is the oldest first-parent commit carrying
  `Brief: build-less-brittle/NN` that touches anything outside `docs/streams/` and
  `changelog/` (the implementing squash-merge, not the authoring or Evidence commit), and the
  base is its parent. Before merge, the base is the merge-base with main and the tip is `HEAD`.
  An empty range fails the row instead of passing it.
- **Every brief carries its own `design-fit:` block.** The stream uses what it introduces.
- **No new verb, flag, status token or workflow.** Reuse `deskfile attach`, labels, the `blocked`
  status and `go test`.
- **Consolidate meaning; preserve independent enforcement.** A second owner is a finding. A
  second enforcement point at a different trust boundary is a layer.
- **Human gates are one typed reply:** `grow <PR#>`, `retire <ids>; keep rest`, `bleed`,
  `keep`/`revert`, `promote …`/`hold`, `residue: default` (or `residue: keep …; drop rest`).
  A reply counts only when its author, read from the forge, is the driver's own login (a
  project value). A reply from any other login is quarantined and never acted on (spec §4.7).
- **A test is tagged, and leaves with a reason.** `// regression: #<N>` above every fix's
  test; `Retires-test: <name> — <why>` on every deletion or rename; the ledger report is a
  rubric for the reviewer, never a gate.
- **Nothing is refactored against recollection.** A `redesign` has an oracle first, and the
  triage table lands in the PR body.
- **Independent of any desk-tool redesign.** Nothing here waits on or calls one. Couplings are optional and
  listed in spec §6.
- **Verify rows** run from the repository root. Go rows `cd tools/desk` or `cd statusgen`
  (no root `go.mod`).
- A changelog fragment ships with every PR.
