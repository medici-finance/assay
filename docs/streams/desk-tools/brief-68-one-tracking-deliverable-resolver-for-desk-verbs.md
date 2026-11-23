---
brief: assay:assay:desk-tools:68
title: One tracking/deliverable resolver for desk verbs
why: >-
  A brief can be tracked in one repository and deliver into another. Each desk verb has been
  choosing between the two on its own, from `--repo`, `--root` or the checkout's origin, and
  each fix so far aligned one verb while the next still read the other repository. The record
  holds four counted incident groups plus later instances: the decision gate keyed on the
  deliverable (#5525, #6494), a verify witness that cannot be faithful (#4812), an outcome
  record that cannot read its own Evidence commit, a claim key that does not match the board
  row, a deliverable home that cannot be found because it is not a stream root, and a claim
  read in the checkout's origin (#7490). This brief adds one resolver that names both
  repositories for an item and finds a repository's checkout without requiring it to be a
  stream root, then moves the recorded verbs onto it.
wave: 1
depends: []
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [6274, 6494]
outcome: none
schema: brief-v2
version: 1
authored: 2026-11-23 by a worker-desk session (error-class design brief for issue 6274)
sources:
  - "medici-finance/assay#6274 — the error-class record: mechanism, instance table, the 2026-11-12 and 2026-11-17 outcome-binding instances, the claim-key instance, the stream-less deliverable instance, the #7490 strike-two note, and the trigger comment that sets gate model (it changes which repository a verb reads and weakens no control)"
  - "medici-finance/assay#6494 — dispatch reads a bare `decision-issue: #N` in the deliverable repository"
  - "medici-finance/assay#5525 — the decision gate's `ensure` dedupe searched the deliverable repository; fixed in 1764da53cc (desk-tools v1.0.88), covered by `TestDecisionGateUsesTrackingRepo`"
  - "medici-finance/assay#5008 — the same-repository form of the outcome-binding miss (Evidence written through the forge, absent from the local store)"
  - "medici-finance/assay#4812 — no faithful verifyrun witness for a cross-repository verify"
  - "medici-finance/assay#7490 — deskwt remove and the dead-session reap read the claim in the checkout's origin; a point fix is in flight on its own branch"
  - "tools/desk/internal/deskkit/roots.go (ConfiguredRoots, RootForRepo, ResolveRoot), verifierdeliverable.go (VerifierDeliverables), verifywake.go (RootRevisionReader.Revision), read at e9bb42182b"
  - "freshness-checked 2026-11-23 @ e9bb42182b (origin/main)"
exec-tier: strong
exec-tier-why: "It changes which repository several verbs read for a decision gate, a verifier association and an Evidence binding. Each must stay fail-closed, and a wrong answer admits work against the wrong repository's gate."
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/itemrepos.go: fixed-here (the resolver: ItemRepos, ResolveItemRepos, CheckoutForRepo, DESK_CHECKOUTS)"
  - "tools/desk/cmd/verifyloop/association.go: fixed-here (the deliverable home comes from CheckoutForRepo, so a stream-less homed-in repository associates)"
  - "tools/desk/internal/deskkit/verifywake.go: fixed-here (the repo-source wake key reads the deliverable through CheckoutForRepo)"
  - "tools/desk/cmd/deskdispatch/briefadmission.go: fixed-here (the pre-claim decision hold reads ItemRepos.DecisionRepo, the tracking repository)"
  - "tools/desk/cmd/deskevidence/outcomebinding.go: fixed-here (a missing Evidence commit is fetched from the tracking repository and read again)"
  - "tools/desk/README.md: fixed-here (documents ItemRepos, CheckoutForRepo and DESK_CHECKOUTS beside the roots inventory)"
  - "tools/desk/cmd/deskdispatch/deliverable.go: out-of-scope (follow-up F1 below: its own declaration parser moves onto ResolveItemRepos)"
  - "tools/desk/cmd/deskrebaseline/brief.go: out-of-scope (follow-up F2 below)"
  - "tools/desk/cmd/deskinbox/show_gate.go: out-of-scope (follow-up F2 below)"
  - "tools/desk/cmd/deskmerge/currency.go: out-of-scope (follow-up F2 below)"
  - "tools/desk/internal/deskkit/briefrisk.go: out-of-scope (follow-up F2 below)"
  - "tools/desk/cmd/deskwt/claimfree.go: out-of-scope (the #7490 point fix owns it; follow-up F5 below reconciles it onto the resolver)"
---

# Brief 68 — One tracking/deliverable resolver for desk verbs

## Dependencies

None typed. It builds on what is on main: the verifier association's declaration policy
(`VerifierDeliverables`), the decision gate's tracking target (`decisionTarget`, #5525), and
deskevidence's private fetch refs for the landed closure (#5008). The #7490 point fix for
`deskwt` is in flight on its own branch and touches no file this brief changes. F5 below
moves that fix's claim-repository read onto the resolver after it lands.

## Context

files:
- `tools/desk/internal/deskkit/itemrepos.go` — NEW: `ItemRepos`, `NewItemRepos`, `ResolveItemRepos`, `Checkout`, `CheckoutForRepo`, `DESK_CHECKOUTS`
- `tools/desk/internal/deskkit/itemrepos_test.go` — NEW: role, resolution, checkout and wake-reader tests
- `tools/desk/internal/deskkit/verifywake.go` — the `repo-source:` key in the root revision reader
- `tools/desk/cmd/verifyloop/association.go` — `associateVerifierItem`
- `tools/desk/cmd/verifyloop/association_test.go`
- `tools/desk/cmd/deskdispatch/briefadmission.go` — `briefDecisionHold`
- `tools/desk/cmd/deskdispatch/decisiontracking_test.go`
- `tools/desk/cmd/deskevidence/outcomebinding.go` — `validateOutcomeEvidenceBinding`
- `tools/desk/cmd/deskevidence/outcomebinding_test.go`
- `tools/desk/cmd/deskevidence/deskevidence_test.go` — the shared fixture stubs the new fetch seam
- `tools/desk/README.md` — the roots inventory paragraph
- **add** — `changelog/crossrepo-item-repos.md`

facts:
- **Two repositories, many roles.** The tracking repository holds the brief, its board row,
  its decision issue, its Evidence, its outcome record and the verifier's claim (#5999). The
  deliverable repository is where code lands, where homes are cut, and where a worker or
  reviewer claims. The two are the same for ordinary work. A brief declares the deliverable
  with `deliverable_repo: <alias>`, `homed-in: <owner/name>` or `[alias]` file tags, resolved
  through the tracking commit's `docs/streams/graph-repos.yaml`.
- **Two parsers today.** `deskkit.VerifierDeliverables` (the verifier association's policy:
  registry-only aliases, `homed-in:` must be registered, conflicts refuse) and deskdispatch's
  own `resolveDeliverable` / `aliasRegistry` both read those declarations.
- **Checkouts are found only through stream roots.** `RootForRepo` answers only from
  `DESK_ROOTS` (or the compiled roots), and the board refuses a root without `docs/streams/`.
  A launcher that builds `DESK_ROOTS` from stream-bearing checkouts therefore drops a
  deliverable repository with no streams, and every verb that looks the deliverable up
  through `RootForRepo` reports it unconfigured. The 2026-11-18 instance is that case: two
  homed-in briefs stayed could-not-check at verifier association. `RootForRepo` is also
  case-sensitive while forge slugs are not.
- **Instances fixed here.**
  - Decision hold (#6494): `briefDecisionHold` read the brief's bare `decision-issue: #N`
    in the deliverable repository it was handed, while the gate's `ensure` step already used
    the tracking repository (#5525, 1764da53cc). The hold missed an open, unruled decision
    and the dispatch went ahead.
  - Verifier association (2026-11-18): `associateVerifierItem` refused with
    "verifier deliverable root is not configured" for a stream-less deliverable.
  - Wake reader: `RootRevisionReader.Revision` read a `repo-source:` key through
    `RootForRepo`, so the same stream-less deliverable was could-not-check forever and the
    planner kept re-offering the item.
  - Outcome binding (2026-11-12, 2026-11-17; same mechanism as #5008):
    `validateOutcomeEvidenceBinding` ran `git show <evidence sha>:<brief>` in `--root`.
    deskevidence writes Evidence through the forge API in the tracking repository, so that
    commit is in no local store until something fetches it, and the record was refused with
    exit 6.
- **Instances not fixed here** (see Follow-ups): the dispatch dry-run plans its PR in the
  tracking repository for a `deliverable_repo:` brief; the emitted verifier prompt names the
  deliverable as the outcome target; the claim key built on the deliverable does not match
  the board row's key; #4812's verify witness; #7490's claim read in `deskwt`.

single-point-of-failure: the resolver is the one place a verb learns which repository a
role lives in. If it is wrong, a verb reads the wrong repository's gate or store. Every
answer therefore fails closed:
- a declaration the tracking commit cannot resolve is could-not-check (exit 6);
- two deliverables are ambiguous (exit 6);
- a checkout outside the allowed repository set refuses (exit 5);
- a checkout whose origin names another repository refuses (exit 5), and is never skipped
  for the next candidate.

No answer falls back to the other repository. Every consumer keeps the identity proofs it
already had: the association re-proves the deliverable origin at fetched main, and the wake
reader keeps its own-work-tree and closure checks.

layering: one file in `deskkit`, beside `roots.go` and `verifierdeliverable.go`, and no new
package. `ItemRepos` is a value with one method per role, so a verb names the role it needs
(`DecisionRepo`, `EvidenceRepo`, `OutcomeRepo`, `VerifierClaimRepo`, `HomeRepo`) instead of
picking a repository. `ResolveItemRepos` reuses `VerifierDeliverables`, so this adds no third
declaration parser. `CheckoutForRepo` checks `DESK_ROOTS`, then `DESK_CHECKOUTS` (same
grammar and refusals), then a directory named after the repository beside the tracking
checkout. That is the convention the compiled roots already use.

design-fit:
  owner: tools/desk/internal/deskkit — itemrepos.go (new); no registered contract row owned "which repository an item's role lives in" before this
  contract: none — the class record's hypothesised invariant; a contract row may follow once F1 and F2 leave one implementation
  retires: [briefDecisionHold reading the decision issue in the dispatch's deliverable repo, the verifier association's stream-root-only lookup of the deliverable home, the wake reader's stream-root-only repo-source lookup, the outcome binding's local-store-only read of the Evidence commit]
  weight: "verbs 0, flags 0, +1 environment variable (DESK_CHECKOUTS), refusals +2 (a mis-mapped DESK_CHECKOUTS entry, an ambiguous deliverable); +1 file, +6 tests"
  why-add: "Without a shared resolver each verb keeps its own repository choice, which is the mechanism the class records. Two alternatives were rejected. Requiring every deliverable repository to be a stream root would force board machinery onto repositories that have no streams. Reading the Evidence commit through the forge would break the binding's comparison of local bytes against forge bytes, which is two reads from two sources. Fetching the commit keeps one byte source."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands outside the task's
  instructions. Stop at `implemented`. A different, non-implementing identity sets verified
  and done.
- Every change keeps its verb fail-closed. A test that needs a weaker refusal is a STOP:
  report it and do not edit the expectation.
- The resolver never widens the allowed repository set. `DESK_CHECKOUTS` configures where a
  checkout is, never whether a repository is trusted.
- Do not touch `tools/desk/cmd/deskwt/`. The #7490 fix owns it until F5.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not guess.

## Task
1. **Add the resolver** (`itemrepos.go`):
   - `ItemRepos{Tracking, Deliverable}`: `NewItemRepos` validates both slugs, and an empty
     deliverable means the tracking repository. Role methods: `DecisionRepo`, `EvidenceRepo`,
     `OutcomeRepo` and `VerifierClaimRepo` return Tracking; `HomeRepo` returns Deliverable;
     `CrossRepo` compares without case.
   - `ResolveItemRepos(trackingRoot, tracking, revision, brief)` reads the declaration at the
     tracking commit through `VerifierDeliverables`: zero deliverables means the same
     repository, one is paired, more than one is could-not-check.
   - `CheckoutForRepo(repo, near)` resolves and proves a local checkout as described under
     layering. It matches slugs without case and refuses a repository outside the allowed
     set. A missing checkout is could-not-check and the message names both variables.
2. **Verifier association** (`tools/desk/cmd/verifyloop/association.go`): take the deliverable home from
   `CheckoutForRepo(repos[0], root)` instead of `RootForRepo`, and keep the later snapshot and
   origin re-proof.
3. **Wake reader** (`tools/desk/internal/deskkit/verifywake.go`): resolve a `repo-source:<repo>` key through
   `CheckoutForRepo(repo, rr.Root)`, and keep every check after it.
4. **Decision hold** (`tools/desk/cmd/deskdispatch/briefadmission.go`): read the decision issue in
   `ItemRepos.DecisionRepo()`, built from the plan's tracking target (`plan.decision`, already
   resolved before the claim for every human-gated item) and the dispatch repository.
5. **Outcome binding** (`tools/desk/cmd/deskevidence/outcomebinding.go`): when `git show` cannot read the
   Evidence commit, validate the landing branch name and fetch that branch and main from the
   tracking repository into the root's private `refs/deskevidence-fetch/` refs, using the
   landed closure's fetch path. Then read again. The commit is still named by full SHA, and
   the landed-content comparison after it is unchanged. A failed fetch stays could-not-check.
6. **Tests that fail before and pass after**, one per instance (Verify rows 1 to 4).
7. **Docs.** Add the README paragraph and the changelog fragment.

### Follow-ups (not in this brief; each becomes its own brief or a fix on its issue)
- **F1** — deskdispatch's `resolveDeliverable` / `aliasRegistry` resolve through
  `ResolveItemRepos`, leaving one declaration parser. The dispatch dry-run then plans its PR in
  `HomeRepo()` (the 2026-11-12 dispatch-plan instance).
- **F2** — the remaining `RootForRepo` callers that look up a repository which may be a
  deliverable move to `CheckoutForRepo`: `tools/desk/cmd/deskrebaseline/brief.go`,
  `tools/desk/cmd/deskinbox/show_gate.go`, `tools/desk/cmd/deskmerge/currency.go`,
  `tools/desk/internal/deskkit/briefrisk.go`. Each gets its own test, and `RootForRepo`
  is left for stream-root readers only.
- **F3** — the emitted verifier prompt names `OutcomeRepo()` as the outcome target, and the
  claim key for a cross-repository dispatch is derived from the board row's key. The claim
  key shape needs the ruling the claim-key instance asked for.
- **F4** — #4812: a faithful verifyrun witness for a cross-repository verify.
- **F5** — after #7490 lands, its claim-repository read in `deskwt` moves behind `deskkit`.
  It should read the home's recorded claim repository, or `ItemRepos.VerifierClaimRepo()` /
  `HomeRepo()` for the dispatch kind. The point fix's tests stay as they are.
- **F6** — the class record's close condition 2: one end-to-end fixture tracked in A and
  delivering into B, running dispatch → Evidence → outcome record. That fixture also closes
  the class once F1 to F5 are in.
- An installation's desk launcher can populate `DESK_CHECKOUTS` for stream-less deliverable
  checkouts. That is installation configuration, not part of this repository.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestItemReposRoles TestResolveItemReposFromBrief TestCheckoutForRepoStreamless TestWakeRepoSourceStreamlessCheckout; do go test ./internal/deskkit/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestItemReposRoles ' "$L" && grep -F -e '--- PASS: TestResolveItemReposFromBrief ' "$L" && grep -F -e '--- PASS: TestCheckoutForRepoStreamless ' "$L" && grep -F -e '--- PASS: TestWakeRepoSourceStreamlessCheckout ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; four named PASS lines: the role mapping, resolution from the tracking commit, a stream-less checkout found and proved by origin (a mis-mapped one refuses, an unconfigured one is could-not-check), and the wake reader using it. Mutation: drop the origin proof in `provedCheckout`, and the `wrong origin refuses` subtest must fail |
| 2 | check:ci +flow | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestCrossRepoAssociationStreamlessDeliverable TestCrossRepoAssociationRefusals; do go test ./cmd/verifyloop/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestCrossRepoAssociationStreamlessDeliverable ' "$L" && grep -F -e '--- PASS: TestCrossRepoAssociationRefusals ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; a homed-in repository with no `docs/streams` associates when `DESK_CHECKOUTS` names it, stays could-not-check when nothing does, and every existing association refusal still holds |
| 3 | check:ci +flow +mutation | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestDecisionHoldReadsTrackingRepo TestDecisionGateUsesTrackingRepo TestUnruledDecisionHolds; do go test ./cmd/deskdispatch/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestDecisionHoldReadsTrackingRepo ' "$L" && grep -F -e '--- PASS: TestDecisionGateUsesTrackingRepo ' "$L" && grep -F -e '--- PASS: TestUnruledDecisionHolds ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; for both `homed-in:` and `deliverable_repo:`, the hold reads the decision issue only in the tracking repository and holds before the claim; the gate's ensure step and the same-repository hold are unchanged. Mutation: pass the dispatch repository to `decisionWaitForgeFn` in place of `ItemRepos.DecisionRepo()` (the base behaviour), and `TestDecisionHoldReadsTrackingRepo` must fail |
| 4 | check:ci +flow +mutation | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestOutcomeBindingFetchesEvidenceCommitOnMiss TestOutcomeBindingRequiresCurrentLandedEvidence TestOutcomeCannotLandWithoutCurrentEvidence; do go test ./cmd/deskevidence/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestOutcomeBindingFetchesEvidenceCommitOnMiss ' "$L" && grep -F -e '--- PASS: TestOutcomeBindingRequiresCurrentLandedEvidence ' "$L" && grep -F -e '--- PASS: TestOutcomeCannotLandWithoutCurrentEvidence ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; an Evidence commit present only on the tracking repository binds once fetched, an unfetchable one stays could-not-check, and every existing binding refusal (previous run, wrong branch, unreadable SHA) still holds. Mutation: return the first `git show` miss without calling `evidenceCommitFetchFn`, and the `fetch=true` subtest must fail |
| 5 | check:ci +neighbour | `cd tools/desk && go vet ./internal/deskkit/ ./cmd/verifyloop/ ./cmd/deskdispatch/ ./cmd/deskevidence/ && L=$(mktemp) && trap 'rm -f "$L"' 0 && go test ./cmd/verifyloop/ ./cmd/deskdispatch/ ./cmd/deskevidence/ -count=1 -timeout 600s > "$L" 2>&1 && ! grep -E -e '^(--- )?FAIL' "$L"` | exit 0; vet is clean and the three command packages pass in full beside the new tests |
| 6 | check +dereference | `S=$(mktemp) && trap 'rm -f "$S"' 0 && statusgen --root . --consumers --brief assay:assay:desk-tools:68 > "$S" && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved' "$S"` | exit 0 and a `summary:` line with a nonzero corroborated count and `0 disproved`: each `fixed-here` consumer is a path the delivery changed |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The decision hold reads the deliverable repository again and dispatches past an open decision | row 3 (the fake forge serves the open decision only for the tracking repository and records every repository asked) |
| `DESK_CHECKOUTS` maps a repository to another repository's checkout and the resolver accepts it | row 1 (`wrong origin refuses`) |
| A mis-mapped entry is skipped and a later candidate is used instead | row 1 (the refusal is checked by exit code, with no fall-through) |
| `DESK_CHECKOUTS` widens the allowed repository set | row 1 (`outside the repo set`) |
| An unconfigured deliverable is treated as the tracking repository | rows 1 and 2 (could-not-check, not success) |
| The outcome binding fetches from the deliverable repository, or accepts an unfetched commit | row 4 (the fetch must name the tracking repository; an unfetchable commit stays exit 6) |
| A fetched commit lets different bytes bind | row 4 (`TestOutcomeBindingRequiresCurrentLandedEvidence`: branch mismatch, old SHA and previous-run modes still refuse) |
| A test that needs the network runs in CI | rows 4 and 5 (`setupFake` stubs the fetch seam to fail, so only the new test serves refs, from a local repository) |
| The sibling-directory convention picks up an unrelated directory | No row beyond the origin proof in row 1; review-only: the reviewer confirms that a sibling whose origin names another repository refuses |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This PR claims only the implementer's local runs recorded in its description. -->

## Review
Gate: model. The class record's trigger comment sets that gate: the change moves which
repository a verb reads and weakens no control. The reviewer checks four things. First, every
new path is fail-closed (exit 5 or 6, never a fall-back to the other repository). Second,
`DESK_CHECKOUTS` refuses exactly what `DESK_ROOTS` refuses. Third, the outcome-binding fetch
only supplies objects and decides nothing. Fourth, no file under `tools/desk/cmd/deskwt/`
changed. Rows 1 to 4 were each written to fail on the delivery base, and the implementing PR
records those base runs. A verifier may confirm them with `verifyrun --fail-first` against
the parent of the delivering merge.
