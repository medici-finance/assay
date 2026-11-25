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
  - "tools/desk/cmd/deskevidence/outcomebinding.go: out-of-scope (medici-finance/assay#7492 fixes the Evidence-commit fetch on its own; follow-up F7 below moves it onto the resolver's tracking-repo answer)"
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
(`VerifierDeliverables`) and the decision gate's tracking target (`decisionTarget`, #5525).
Two point fixes are in flight on their own branches and touch no file this brief changes:
#7490 in `deskwt` and #7492 in deskevidence's outcome binding. F5 and F7 below move each
onto the resolver after it lands.

## Context

files:
- `tools/desk/internal/deskkit/itemrepos.go` — NEW: `ItemRepos`, `NewItemRepos`, `ResolveItemRepos`, `Checkout`, `CheckoutForRepo`, `DESK_CHECKOUTS`
- `tools/desk/internal/deskkit/itemrepos_test.go` — NEW: role, resolution, checkout and wake-reader tests
- `tools/desk/internal/deskkit/verifywake.go` — the `repo-source:` key in the root revision reader
- `tools/desk/cmd/verifyloop/association.go` — `associateVerifierItem`
- `tools/desk/cmd/verifyloop/association_test.go`
- `tools/desk/cmd/deskdispatch/briefadmission.go` — `briefDecisionHold`
- `tools/desk/cmd/deskdispatch/decisiontracking_test.go`
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
- **Instances not fixed here** (see Follow-ups): the outcome binding (2026-11-12,
  2026-11-17; same mechanism as #5008) reads the Evidence commit only from the local store,
  though deskevidence wrote it through the forge API in the tracking repository, and #7492
  fixes that on its own; the dispatch dry-run plans its PR in the tracking repository for a
  `deliverable_repo:` brief; the emitted verifier prompt names the
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
  retires: [briefDecisionHold reading the decision issue in the dispatch's deliverable repo, the verifier association's stream-root-only lookup of the deliverable home, the wake reader's stream-root-only repo-source lookup]
  weight: "verbs 0, flags 0, +1 environment variable (DESK_CHECKOUTS), refusals +2 (a mis-mapped DESK_CHECKOUTS entry, an ambiguous deliverable); +1 file, +5 tests"
  why-add: "Without a shared resolver each verb keeps its own repository choice, which is the mechanism the class records. The rejected alternative, requiring every deliverable repository to be a stream root, would force board machinery onto repositories that have no streams."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands outside the task's
  instructions. Stop at `implemented`. A different, non-implementing identity sets verified
  and done.
- Every change keeps its verb fail-closed. A test that needs a weaker refusal is a STOP:
  report it and do not edit the expectation.
- The resolver never widens the allowed repository set. `DESK_CHECKOUTS` configures where a
  checkout is, never whether a repository is trusted.
- Do not touch `tools/desk/cmd/deskwt/` or `tools/desk/cmd/deskevidence/`. The #7490 fix
  owns the first until F5, and the #7492 fix owns the second until F7.
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
5. **Tests that fail before and pass after**, one per instance (Verify rows 1 to 3).
6. **Docs.** Add the README paragraph and the changelog fragment.

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
  the class once F1 to F5 and F7 are in.
- **F7** — after #7492 lands, move its Evidence-commit fetch onto the resolver's tracking-repo
  answer (`ItemRepos.EvidenceRepo()`), keeping its tests.
- An installation's desk launcher can populate `DESK_CHECKOUTS` for stream-less deliverable
  checkouts. That is installation configuration, not part of this repository.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestItemReposRoles TestResolveItemReposFromBrief TestCheckoutForRepoStreamless TestWakeRepoSourceStreamlessCheckout; do go test ./internal/deskkit/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestItemReposRoles ' "$L" && grep -F -e '--- PASS: TestResolveItemReposFromBrief ' "$L" && grep -F -e '--- PASS: TestCheckoutForRepoStreamless ' "$L" && grep -F -e '--- PASS: TestWakeRepoSourceStreamlessCheckout ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; four named PASS lines: the role mapping, resolution from the tracking commit, a stream-less checkout found and proved by origin (a mis-mapped one refuses, an unconfigured one is could-not-check), and the wake reader using it. Mutation: drop the origin proof in `provedCheckout`, and the `wrong origin refuses` subtest must fail |
| 2 | check:ci +flow | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestCrossRepoAssociationStreamlessDeliverable TestCrossRepoAssociationRefusals; do go test ./cmd/verifyloop/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestCrossRepoAssociationStreamlessDeliverable ' "$L" && grep -F -e '--- PASS: TestCrossRepoAssociationRefusals ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; a homed-in repository with no `docs/streams` associates when `DESK_CHECKOUTS` names it, stays could-not-check when nothing does, and every existing association refusal still holds |
| 3 | check:ci +flow +mutation | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestDecisionHoldReadsTrackingRepo TestDecisionGateUsesTrackingRepo TestUnruledDecisionHolds; do go test ./cmd/deskdispatch/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestDecisionHoldReadsTrackingRepo ' "$L" && grep -F -e '--- PASS: TestDecisionGateUsesTrackingRepo ' "$L" && grep -F -e '--- PASS: TestUnruledDecisionHolds ' "$L" && ! grep -F -e '--- FAIL' "$L"` | exit 0; for both `homed-in:` and `deliverable_repo:`, the hold reads the decision issue only in the tracking repository and holds before the claim; the gate's ensure step and the same-repository hold are unchanged. Mutation: pass the dispatch repository to `decisionWaitForgeFn` in place of `ItemRepos.DecisionRepo()` (the base behaviour), and `TestDecisionHoldReadsTrackingRepo` must fail |
| 4 | check:ci +neighbour | `cd tools/desk && go vet ./internal/deskkit/ ./cmd/verifyloop/ ./cmd/deskdispatch/ && L=$(mktemp) && trap 'rm -f "$L"' 0 && go test ./cmd/verifyloop/ ./cmd/deskdispatch/ -count=1 -timeout 600s > "$L" 2>&1 && ! grep -E -e '^(--- )?FAIL' "$L"` | exit 0; vet is clean and the two command packages pass in full beside the new tests |
| 5 | check +dereference | `S=$(mktemp) && trap 'rm -f "$S"' 0 && statusgen --root . --consumers --brief assay:assay:desk-tools:68 > "$S" && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved' "$S"` | exit 0 and a `summary:` line with a nonzero corroborated count and `0 disproved`: each `fixed-here` consumer is a path the delivery changed |

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| The decision hold reads the deliverable repository again and dispatches past an open decision | row 3 (the fake forge serves the open decision only for the tracking repository and records every repository asked) |
| `DESK_CHECKOUTS` maps a repository to another repository's checkout and the resolver accepts it | row 1 (`wrong origin refuses`) |
| A mis-mapped entry is skipped and a later candidate is used instead | row 1 (the refusal is checked by exit code, with no fall-through) |
| `DESK_CHECKOUTS` widens the allowed repository set | row 1 (`outside the repo set`) |
| An unconfigured deliverable is treated as the tracking repository | rows 1 and 2 (could-not-check, not success) |
| The sibling-directory convention picks up an unrelated directory | No row beyond the origin proof in row 1; review-only: the reviewer confirms that a sibling whose origin names another repository refuses |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This PR claims only the implementer's local runs recorded in its description. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestItemReposRoles TestResolveItemReposFromBrief TestCheckoutForRepoStreamless TestWakeRepoSourceStreamlessCheckout; do go test ./internal/deskkit/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestItemReposRoles ' "$L" && grep -F -e '--- PASS: TestResolveItemReposFromBrief ' "$L" && grep -F -e '--- PASS: TestCheckoutForRepoStreamless ' "$L" && grep -F -e '--- PASS: TestWakeRepoSourceStreamlessCheckout ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:86b72f8e6d55 | 2026-11-24 | assay-verifier-app[bot] @ 769e2a8b3860 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestCrossRepoAssociationStreamlessDeliverable TestCrossRepoAssociationRefusals; do go test ./cmd/verifyloop/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestCrossRepoAssociationStreamlessDeliverable ' "$L" && grep -F -e '--- PASS: TestCrossRepoAssociationRefusals ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:c3c2b0c1824f | 2026-11-24 | assay-verifier-app[bot] @ 769e2a8b3860 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestDecisionHoldReadsTrackingRepo TestDecisionGateUsesTrackingRepo TestUnruledDecisionHolds; do go test ./cmd/deskdispatch/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestDecisionHoldReadsTrackingRepo ' "$L" && grep -F -e '--- PASS: TestDecisionGateUsesTrackingRepo ' "$L" && grep -F -e '--- PASS: TestUnruledDecisionHolds ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:f73cd6851090 | 2026-11-24 | assay-verifier-app[bot] @ 769e2a8b3860 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:d09946237cf6b46d57be52f8b478d9d13a66ead67281643ebf57f10d95462cc8
Container-Image: sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7540 run=177c5e336121c9a70c19fe7c9bcb9a05b4b735f691982610106add98e365c39e source=769e2a8b3860f66445772dc066a9cac7a1d9ef08 brief=docs/streams/desk-tools/brief-68-one-tracking-deliverable-resolver-for-desk-verbs.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=177c5e336121c9a70c19fe7c9bcb9a05b4b735f691982610106add98e365c39e source=769e2a8b3860f66445772dc066a9cac7a1d9ef08 tree=sha256:d09946237cf6b46d57be52f8b478d9d13a66ead67281643ebf57f10d95462cc8 witness=sha256:b7b9608148ea8fc2cb0855ad81b5fa5326a3d27d5a80a6f35498d51569d74232 opened=2026-11-24T15:28:15Z closed=2026-11-24T15:29:10Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && go vet ./internal/deskkit/ ./cmd/verifyloop/ ./cmd/deskdispatch/ && L=$(mktemp) && trap 'rm -f "$L"' 0 && go test ./cmd/verifyloop/ ./cmd/deskdispatch/ -count=1 -timeout 600s > "$L" 2>&1 && ! grep -E -e '^(--- )?FAIL' "$L"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 769e2a8b3860 (on-behalf-of human:ian) (forge-identity) |
| 5 | `S=$(mktemp) && trap 'rm -f "$S"' 0 && statusgen --root . --consumers --brief assay:assay:desk-tools:68 > "$S" && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved' "$S"` | fail exit=2 | sha256:9f050a4bddce | 2026-11-24 | assay-verifier-app[bot] @ 769e2a8b3860 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7540 run=177c5e336121c9a70c19fe7c9bcb9a05b4b735f691982610106add98e365c39e source=769e2a8b3860f66445772dc066a9cac7a1d9ef08 brief=docs/streams/desk-tools/brief-68-one-tracking-deliverable-resolver-for-desk-verbs.md model=claude-opus-5-5 tier=strong

Verify pass 2026-11-24 by assay-verifier-app[bot], model claude-opus-5-5, tier strong, a non-implementer, at merged main 769e2a8b3860 (attestation medici-finance/assay#7540). Host darwin arm64 with statusgen v1.0.92 and git 2.56.1, run offline in the declared verifier Git environment. The delivery is PR medici-finance/assay#7497, merge d922c9b1bd24 with first parent b86fbd3f790a. Its net diff touches 11 paths and nothing under tools/desk/cmd/deskwt/ or tools/desk/cmd/deskevidence/.

**VERIFY: BLOCKED** — rows 1 to 3 pass. Row 4 could not run in either admitted lane, and row 5's instrument could not check the delivery as written. No acceptance obligation was observed to fail.

- Rows 1 to 3 (check:ci) ran inside the closed admission bracket above. The rows ran with --rows 1,2,3 in a network-none container from verify-runner v1.0.92 (sha256:d3d958719dea), with a read-only module cache. All three passed with exit 0. Bracket-digest gave verdict=match. Supplemental readable reruns (no acceptance credit, 2026-11-24, same runner) print every named PASS line. TestCheckoutForRepoStreamless passes all 7 subtests: unconfigured, checkouts_env, stream_root_wins, wrong_origin_refuses, outside_the_repo_set, duplicate_entry and sibling_convention.
- Row 1 mutation (supplemental, no acceptance credit, host, scratch archive of 769e2a8b3860 outside the home): with the origin comparison in provedCheckout disabled, TestCheckoutForRepoStreamless/wrong_origin_refuses FAILs. Restoring the file passes it again.
- Row 3 mutation (supplemental, no acceptance credit, same archive): with the dispatch repository passed to decisionWaitForgeFn instead of ItemRepos.DecisionRepo(), TestDecisionHoldReadsTrackingRepo FAILs in both subtests. Both report that the decision hold read medici-finance/assay when exactly example-org/tracker was wanted. Restoring the file passes it again.
- Row 4 (check:ci), could-not-run. The host witness above records that this darwin host has no unshare network sandbox. The container lane cannot witness this row either. An earlier bracket in this dispatch, attempt 1 (opened 15:20:44Z, closed 15:22:04Z, witness retained outside the home), ran rows 1 to 4. Rows 1 to 3 passed there, and row 4 exited 1. A supplemental probe in the same image gave vet exit 0 and verifyloop ok. In deskdispatch, about 27 tests failed with "cannot resolve which forge serves medici-finance/assay ... the origin remote's host is absent". Those suite tests read the enclosing checkout's origin remote, and the linked worktree's gitdir is not mounted in the container (open tool defect medici-finance/assay#4327). On the host, the same tests fail the same way when run from a copy of 769e2a8b3860 outside any git checkout, and they pass inside the checkout. Supplemental unsandboxed host run of the row's go vet and go test (no acceptance credit, 2026-11-24, same runner): vet exit 0, verifyloop ok, deskdispatch ok, test exit 0. This row needs a Linux runner with unshare --net and a real checkout.
- Row 5 (check), the witness reads fail exit=2, but the instrument could not look. statusgen printed "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:68 is not in the diff against 769e2a8b3860..., and the brief's delivery could not be derived: ... no first-parent commit of HEAD names Brief: desk-tools/68" after searching 4938 first-parent commits. No claim was corroborated or disproved. Supplemental run with the delivery passed explicitly (no acceptance credit, 2026-11-24, same runner): statusgen --root . --consumers --brief assay:assay:desk-tools:68 --delivery d922c9b1bd24... --base b86fbd3f790a... exits 0 with "summary: 5 corroborated, 0 disproved, 7 unchecked". All five fixed-here consumers are corroborated, and the seven unchecked entries are out-of-scope routes. The row needs a reviewed check-definition repair that names the delivery and base, or a delivery that carries the Brief: trailer.

Risk-bearing values (risk block all no, not irreversible, gate model). Enumeration over the delivery diff found CheckoutsEnv = "DESK_CHECKOUTS" @ tools/desk/internal/deskkit/itemrepos.go:43, the deliverable count bound (case 0 same-repo, case 1 paired, otherwise could-not-check) @ itemrepos.go:103, and the refusal codes ExitRefused = 5 @ exitcodes.go:51 and ExitUnverifiable = 6 @ exitcodes.go:55 (referenced, not changed). Every one is reversible by an edit and a redeploy.
RISK-VALUE: DERIVED — deliverable count bound = 1 @ tools/desk/internal/deskkit/itemrepos.go:103 — a brief has exactly one home. Two declared deliverables leave HomeRepo undefined, so the brief's single-point-of-failure rule makes that could-not-check (exit 6) and forbids choosing one.
RISK-VALUE: DERIVED — ExitRefused = 5 / ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:51,55 — the brief requires a mis-mapped or out-of-set checkout to refuse (5) and an unresolvable or unconfigured one to be could-not-check (6). The row 1 subtests wrong_origin_refuses, outside_the_repo_set and unconfigured exercise that split.

Review note, outside this table's obligations: briefDecisionHold still dispatches with a stderr note when the new NewItemRepos call fails. That matches its existing behaviour on forge errors, where the human gate still binds approval.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestItemReposRoles TestResolveItemReposFromBrief TestCheckoutForRepoStreamless TestWakeRepoSourceStreamlessCheckout; do go test ./internal/deskkit/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestItemReposRoles ' "$L" && grep -F -e '--- PASS: TestResolveItemReposFromBrief ' "$L" && grep -F -e '--- PASS: TestCheckoutForRepoStreamless ' "$L" && grep -F -e '--- PASS: TestWakeRepoSourceStreamlessCheckout ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:86b72f8e6d55 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestCrossRepoAssociationStreamlessDeliverable TestCrossRepoAssociationRefusals; do go test ./cmd/verifyloop/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestCrossRepoAssociationStreamlessDeliverable ' "$L" && grep -F -e '--- PASS: TestCrossRepoAssociationRefusals ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:e84c0978def4 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && L=$(mktemp) && trap 'rm -f "$L"' 0 && for t in TestDecisionHoldReadsTrackingRepo TestDecisionGateUsesTrackingRepo TestUnruledDecisionHolds; do go test ./cmd/deskdispatch/ -run "^$t\$" -count=1 -v -timeout 300s >> "$L" 2>&1; done; grep -F -e '--- PASS: TestDecisionHoldReadsTrackingRepo ' "$L" && grep -F -e '--- PASS: TestDecisionGateUsesTrackingRepo ' "$L" && grep -F -e '--- PASS: TestUnruledDecisionHolds ' "$L" && ! grep -F -e '--- FAIL' "$L"` | pass exit=0 sandbox=container-netns | sha256:1aded74f829a | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go vet ./internal/deskkit/ ./cmd/verifyloop/ ./cmd/deskdispatch/ && L=$(mktemp) && trap 'rm -f "$L"' 0 && go test ./cmd/verifyloop/ ./cmd/deskdispatch/ -count=1 -timeout 600s > "$L" 2>&1 && ! grep -E -e '^(--- )?FAIL' "$L"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `S=$(mktemp) && trap 'rm -f "$S"' 0 && statusgen --root . --consumers --brief assay:assay:desk-tools:68 > "$S" && grep -E '^summary: [1-9][0-9]* corroborated, 0 disproved' "$S"` | fail exit=2 sandbox=container-netns | sha256:fd47a221e408 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7687 run=b5e72069fb0f7b737a5ab4714e04a2bf9d5015301a0d9b06241a7f0b49adca02 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desk-tools/brief-68-one-tracking-deliverable-resolver-for-desk-verbs.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=b5e72069fb0f7b737a5ab4714e04a2bf9d5015301a0d9b06241a7f0b49adca02 source=77ff0f2222a28df926c38ad30fea4a368173712e tree=sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6 witness=sha256:360eadfb9aee4a6a090ab730aff1774ad79267ea7182241e348f08e121e47fa2 opened=2026-11-25T04:32:45Z closed=2026-11-25T04:34:42Z before=receipt after=check-verifier

Verify pass 2026-11-25 by the verifier App, model claude-opus-5-5, tier strong, a non-implementer, at merged main 77ff0f2222a2 (attestation medici-finance/assay#7687). Host darwin arm64 with statusgen v1.0.94 and git 2.56.1, run offline in the declared verifier Git environment. The delivery is PR medici-finance/assay#7497, merge d922c9b1bd24 with first parent b86fbd3f790a. None of its delivered source or test files changed between that merge and 77ff0f2222a2.

**VERIFY: BLOCKED** — rows 1 to 3 pass. Rows 4 and 5 could not run to a verdict in any admitted lane on this host. No acceptance obligation was observed to fail.

- All five rows ran in the one closed admission bracket above: verify-runner v1.0.94 by digest (sha256:79e23ec185b1), network none, read-only root, home and module cache, GOPROXY=off. Rows 1, 2 and 3 pass with exit 0. Bracket-digest gave verdict=match. The --credit check gave credit=could-not-check because the witness file sits outside a git work tree.
- Row 1 mutation (supplemental, no acceptance credit, host, scratch archive of 77ff0f2222a2 outside the home): with the origin comparison in provedCheckout disabled, TestCheckoutForRepoStreamless/wrong_origin_refuses FAILs and the other six subtests pass. The restored file passes again.
- Row 3 mutation (supplemental, no acceptance credit, same archive): with the dispatch repository passed to decisionWaitForgeFn in place of ItemRepos.DecisionRepo(), TestDecisionHoldReadsTrackingRepo FAILs in both subtests. Each reports that the hold read medici-finance/assay when exactly example-org/tracker was wanted. The restored file passes again.
- Row 4 (check:ci), could-not-run. The witness cell reads fail exit=1, and the row sends its test output to a temporary file, so the cell holds no output. A diagnostic in the same image (not a Verify row) shows git cannot read the home there: "fatal: gitfile does not point to a valid repository", because the linked worktree's gitdir is outside the mount. The deskdispatch suite reads the enclosing checkout's origin remote. The pass at 769e2a8b3860 saw those tests fail in the container on that missing origin. Open tool defect medici-finance/assay#4327. Supplemental unsandboxed host run of the row's go vet and go test inside the home before the bracket (no acceptance credit, 2026-11-25): vet exit 0, verifyloop ok, deskdispatch ok, test exit 0. The row needs a Linux runner with unshare --net and a real checkout, or the #4327 fix.
- Row 5 (check), could-not-run. The witness cell reads fail exit=2 in the container, which has no readable git history. The host run (supplemental, no acceptance credit, 2026-11-25) also exits 2: "--consumers: COULD-NOT-CHECK: assay:assay:desk-tools:68 is not in the diff against 77ff0f2222a2..., and the brief's delivery could not be derived ... no first-parent commit of HEAD names Brief: desk-tools/68 ... (searched 4996 first-parent commits)". No claim was corroborated or disproved. With the delivery passed explicitly (--delivery d922c9b1bd24... --base b86fbd3f790a..., supplemental, no acceptance credit) it exits 0 with "summary: 5 corroborated, 0 disproved, 7 unchecked". The row needs the check-definition repair asked for in medici-finance/assay#7568.

Risk-bearing values (risk block all no, not irreversible, gate model). Enumeration over the delivery diff of tools/desk found CheckoutsEnv = "DESK_CHECKOUTS" @ tools/desk/internal/deskkit/itemrepos.go:43 and the deliverable count bound (case 0 same repository, case 1 paired, otherwise could-not-check) @ itemrepos.go:103-109. The refusal codes ExitRefused = 5 @ exitcodes.go:51 and ExitUnverifiable = 6 @ exitcodes.go:55 are referenced, not changed. Each is reversible by an edit and a redeploy.
RISK-VALUE: DERIVED — deliverable count bound = 1 @ tools/desk/internal/deskkit/itemrepos.go:103 — a brief has exactly one home. Two declared deliverables leave HomeRepo undefined, so the brief's single-point-of-failure rule makes that could-not-check (exit 6) and forbids choosing one.
RISK-VALUE: DERIVED — ExitRefused = 5 / ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:51,55 — the brief requires a mis-mapped or out-of-set checkout to refuse (5) and an unresolvable or unconfigured one to be could-not-check (6). Row 1's subtests wrong_origin_refuses, outside_the_repo_set and unconfigured exercise that split.

## Review
Gate: model. The class record's trigger comment sets that gate: the change moves which
repository a verb reads and weakens no control. The reviewer checks three things. First, every
new path is fail-closed (exit 5 or 6, never a fall-back to the other repository). Second,
`DESK_CHECKOUTS` refuses exactly what `DESK_ROOTS` refuses. Third, no file under
`tools/desk/cmd/deskwt/` or `tools/desk/cmd/deskevidence/` changed. Rows 1 to 3 were each written to fail on the delivery base, and the implementing PR
records those base runs. A verifier may confirm them with `verifyrun --fail-first` against
the parent of the delivering merge.
