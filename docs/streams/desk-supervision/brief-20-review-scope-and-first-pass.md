---
brief: "assay:assay:desk-supervision:20"
title: "Review scope and first-pass completeness"
why: "An incremental reviewer search keeps discovering old instances after each worker fix. Require one declared first-pass search and a stable blocking boundary so a small change does not acquire unbounded cleanup scope."
wave: 0
depends: []
unblocks: []
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
gate-why: "Changes which review findings can hold a PR or how a standing rejection can clear; approve the precise rule before changing that control."
decision-trigger: "start"
design: "DR-review-scope-20"
issues: [1387]
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["https://github.com/medici-finance/assay/issues/1387", "docs/streams/desk-supervision/recovery-increments.md", "../decisions/DR-review-scope-20.md — the design record this brief's human gate dereferences; it transcribes the approved ruling", "source freshness 2026-09-20 @ 3db05fb44"]
consumers: ["tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here", "tools/desk/cmd/deskdispatch/references/worker-prompt.md: fixed-here", "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Must distinguish genuine changed evidence from a bypass of a standing review rejection."
domain: "complicated"
version: 1
id: "ca278906-2fd7-45ce-a4b8-aa2921d2ba9c"
---

# Brief 20 — Review scope and first-pass completeness

## Context

Home: medici-finance/assay. Planning only; issue 1387 explicitly records a proposed fix, not implementation authority. This brief contributes part of that issue; do not close the issue until both 20 and 21 and the class-counting behavior in 19 are accepted.

files:
- tools/desk/cmd/deskdispatch/references/review-prompt.md
- tools/desk/cmd/deskdispatch/references/worker-prompt.md
- plugins/assay/skills/pr-review-desk/SKILL.md
- tools/desk/cmd/deskdispatch/reviewscope_test.go (planned)
- docs/streams/desk-supervision/review-scope-cases.md (planned)

facts:
- Review-prompt clause 8 currently requires a whole-diff search on re-review and treats repository search as optional where cheap; it does not mandate a complete first-pass inventory.
- The canonical skill already caps full rounds at three per finding class. Preserve that threshold and human arbitration authority.
- Clause 11 forbids same-head clearance except a narrow required-check case. The code has an explicit check-only marker parser; this is an existing extension point, not license for prose-inferred exemptions.
- A prompt-only exception cannot repair a contradictory ready gate. Coordinate every producer and reader named in this brief.

## Read first

- [Recovery increments](recovery-increments.md).
- [Issue 1387](https://github.com/medici-finance/assay/issues/1387).
- The existing source paths above; new test and schema paths are explicitly planned.

## Human decision

Approve the precise review-control change described here before implementation begins.
Blocking findings would require a demonstrated change impact or an explicit acceptance obligation. Reviewers would inventory related claims on the first pass and file unrelated stale prose separately. Missing a sibling occurrence would retain the same claim class and existing round count. Genuine safety defects would remain blocking, including outside changed lines.

Options:
1. **Approve this scope rule** — apply the first-pass inventory and impact-based blocking boundary while preserving the existing three-round limit.
2. **Retain the existing rule** — leave the broader prose gate unchanged and record its continuing review cost.

Default if no answer: none — blocks implementation.

### Recorded ruling

Approved — **option 1** — by the driver (`human:<name>`) on his own GitHub login, 2026-09-21,
[issue 1402 comment](https://github.com/medici-finance/assay/issues/1402#issuecomment-5762179072):

> Approve desk-supervision/20's first-pass inventory and impact-based blocking
> boundary. A blocker must demonstrate changed behavior, an explicit acceptance
> obligation, a material PR-body/Verify claim, or a concrete safety consequence.
> Unrelated pre-existing prose goes to a follow-up. Untouched files are not
> automatically exempt. Sibling occurrences retain their class and round count.
> Preserve the existing three-round policy and independent security review.

This ruling authorises the implementation below. It narrows what counts as a NEW
blocker; it does not touch the three-round cap mechanics or the independent security
review, both of which remain unchanged.

## Interface contract

The first review of a false-claim class must inventory its relevant occurrences before issuing the verdict. Search the changed surface, the brief's required deliverables and references to the affected entity across the repository; read matches in context. Record command, scope, exclusions and input revision. An incomplete search is reported incomplete, never certified clean. Repository search is discovery, not authority to make every hit a merge blocker.

Every blocking finding must name a concrete failure and its scope basis: changed behavior, an explicit acceptance deliverable, a material PR-body/Verify claim, or a demonstrated safety consequence of this change. Unrelated pre-existing prose belongs in a linked follow-up. Untouched does not automatically mean irrelevant: a required operator-state table can be a deliverable even when the worker omitted it from the diff. Conversely, sharing a directory or substring is insufficient scope.

Group all occurrences of the same proposition under one claim-class ID. Follow-up fixes and missed sibling occurrences do not reset the existing class-round counter. A previously non-blocking instance cannot become blocking merely because another file was edited: require changed impact or new evidence and explicitly record the reason. A missed first-pass occurrence is review coverage failure as well as work to classify; do not silently charge the worker another fresh class. Preserve the existing three-round cap; a proposed two-round policy in issue 1387 is not adopted by this brief.

## Ground rules

- No runtime activation, production probes, automatic merge or reinterpretation of an existing human ruling.
- Implement in one isolated branch and draft PR after the decision; stop at implemented.
- Use synthetic fixtures. Do not copy private incident identifiers or review transcripts into this public repository.

## Task

1. Implement the exact approved contract in the named canonical producers and readers; do not change the numeric round threshold.
2. Keep the new review behavior consistent with worker instructions and the existing filing, verdict and ready boundaries.
3. Add synthetic regression cases for the named positive and negative paths. Record fail-first evidence for guard changes.
4. Document the contract and compatibility, add a changelog fragment and any required warmup marker. Release and pin through the existing procedure; source merge is not adoption.

## Verify

Named tests below are planned. The verifier must see each named PASS; exit zero with no matching test is not acceptance. Use offline injected forge state.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeFirstPassInventory$ -v -count=1` | named PASS for TestReviewScopeFirstPassInventory; first-pass packet names the search, exclusions and all known in-scope occurrences in a synthetic multi-file example; incomplete searches never assert clean |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeRequiredAndUnrelated$ -v -count=1` | named PASS for TestReviewScopeRequiredAndUnrelated; an omitted required operator table remains blocking; unrelated stale prose is follow-up; a concrete safety consequence remains blocking even outside the edited lines |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeNoSilentPromotion$ -v -count=1` | named PASS for TestReviewScopeNoSilentPromotion; a previously non-blocking occurrence requires changed impact/evidence before promotion; a late sibling instance retains its original claim class |

Prompt assertions alone do not prove reviewer behavior. The planned case corpus must include a first-pass review packet and a follow-up packet for each test scenario; an independent reviewer scores all three scope outcomes against the contract and records model/version, packet hashes and disagreements in Evidence. A literal-text test alone cannot satisfy this row.

| 4 | check:manual +flow | `cat docs/streams/desk-supervision/review-scope-cases.md` | independent reviewer records that the synthetic packets satisfy all three scope outcomes above; any disagreement prevents acceptance |

## Evidence

Pending implementation and independent verification. Authoring claims no acceptance result.

## Review

Gate: human. Verify the recorded decision and every bypass-negative before acceptance. This is a review-control change, not automatic approval of the incident PR.
