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
decision-issue: 1402
ruling: "https://github.com/medici-finance/assay/issues/1402#issuecomment-5762179072"
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

Default if no answer: none — blocks until answered.

### Recorded ruling

Approved — **option 1** — by the driver (`human:<name>`) on his own GitHub login, 2026-09-21,
in the ruling comment on decision issue #1402 (the brief's `ruling:` key links it):

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

| # | Class | Command | Expect |
|---|---|---|---|
| 4 | gate:model +flow | `cat docs/streams/desk-supervision/review-scope-cases.md` | independent reviewer records that the synthetic packets satisfy all three scope outcomes above; any disagreement prevents acceptance |

## Evidence

Pending implementation and independent verification. Authoring claims no acceptance result.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeFirstPassInventory$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:6dd3f4c5a27e | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeRequiredAndUnrelated$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:05e58f98c02e | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeNoSilentPromotion$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:70e9d90229e2 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cat docs/streams/desk-supervision/review-scope-cases.md` | pass exit=0 sandbox=container-netns | sha256:d70f69cad728 | 2026-10-20 | assay-verifier-app[bot] @ b1cf3d790708 (on-behalf-of human:ian) (forge-identity) |

Verification pass, 2026-10-20, by a dispatched verifier (claude-opus-5-5, strong tier) that is not this item's implementer, at merged source b1cf3d7907080ca6ff8dc3b023a0e7558af1123e. The witness table above was produced by `statusgen verifyrun --sandbox container-netns` in a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47), started with `--network none` under the engine's default seccomp profile, per the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun reported 4 pass, 0 fail, 0 could-not-run of 4 rows, and its row-4 pass is exit-status only: `cat` exiting 0 says nothing about row 4's reviewer obligation, which is scored by hand below. The implementing change is PR #1405 (merge 85bd384c226a); the decision record is the driver's ruling on #1402 (comment 5762179072, posted from the driver's own user account, option 1), which this pass checked and does not reinterpret.

Prior Evidence: none. Before this pass the section held only the authoring placeholder, so this is a first pass and there is no earlier verdict to compare against.

By-hand re-run on the darwin host (go1.27.2), same source:

| # | Command | Observed | Date | Runner |
|---|---------|----------|------|--------|
| 1 | row 1 command as written | exit 0; `--- PASS: TestReviewScopeFirstPassInventory` | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |
| 2 | row 2 command as written | exit 0; `--- PASS: TestReviewScopeRequiredAndUnrelated` with all four subtests PASS (omitted required operator table, unrelated pre-existing prose, safety consequence outside edited lines, co-location without a basis) | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |
| 3 | row 3 command as written | exit 0; `--- PASS: TestReviewScopeNoSilentPromotion` | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |

Discrimination (supplemental, in a scratch clone at the same source; it discharges no row and only shows that rows 1 to 3 can fail): each of four one-line mutations of the executable specification turned its row red, and the restored file went green again. M1, an incomplete search allowed to certify clean, made row 1 fail. M2, a basis-backed finding outside the edited lines downgraded to follow-up, made row 2 fail. M3, the no-silent-promotion guard removed, made row 3 fail. M4, a late sibling keeping its re-discovered class id, made row 3 fail.

Row 4, the independent reviewer score. The scorer is claude-opus-5-5 (Opus 5.5, 1M context), a dispatched verifier that took no part in implementing or reviewing PR #1405. Corpus file sha256 d70f69cad7282a3c47fc9a3c346b07f6290ac1a5c2b3eaa2dd5545bd8220c504 (git blob a7602b80987c), which matches the witness row-4 output hash. Each packet hash below is the sha256 prefix of that fenced packet's body. Each case was scored against review-prompt clause 13 and the executable specification in the deskkit reviewscope source:

| case | packet sha256 | expected | scored | agree? |
|---|---|---|---|---|
| A.1 | f99b41cf5c73 | complete packet names all in-scope occurrences together | records search, scope, exclusions and revision; both in-scope occurrences are in one verdict; the vendored substring match is read and excluded | yes |
| A.2 | e49e0398e1b6 | incomplete search rejected, never certified clean | marked incomplete, with no exclusions or revision recorded, so the would-be approval cannot certify the class clean | yes |
| B.1 | 59b54241a5f0 | omitted required operator table is a BLOCKER | basis acceptance-obligation; untouched file, still blocking | yes |
| B.2 | 0e14e20cdede | unrelated pre-existing prose is a FOLLOW-UP | no basis, unrelated and pre-existing, so it routes to a linked follow-up | yes |
| B.3 | fc3d4854d2e6 | safety consequence outside edited lines is a BLOCKER | basis safety-consequence; outside edited lines does not downgrade it | yes |
| B.4 | f721d02a7886 | co-location only is a FOLLOW-UP | no basis; a shared directory is not scope | yes |
| C.1 | 35076f204d25 | no promotion without changed impact or new evidence | previously non-blocking, no changed impact and no new evidence, so it stays a follow-up | yes |
| C.2 | 137f2a5903bb | late sibling keeps class and round count | folded into the existing class settles-in-one-block, with the round count staying 2 | yes |

Every scored outcome agrees with the expected one: 8 of 8, with no disagreements.

Row 4 corpus-composition obligation, failed. The Verify section sets row 4's admission bar in the paragraph above it: "The planned case corpus must include a first-pass review packet and a follow-up packet for each test scenario". The corpus at this source does not have that shape:
- Scenario A (row 1) has one first-pass packet (A.1) and one incomplete-search negative (A.2). A.2 is a different first pass, not a follow-up or re-review packet. Nothing in scenario A is routed to follow-up either.
- Scenario B (row 2) has four single findings (B.1 to B.4) with no first-pass packet and no follow-up packet.
- Scenario C (row 3) comes closest: C.1 narrates round 1 and round 2, and C.2 starts from a round-2 class state. No section, however, is a labelled first-pass packet paired with a follow-up packet.

This is an observed absence measured against a presence obligation. It is a deliverable defect in the case corpus, not an environment limit, and the agreeing scores above do not cure it. The fix is to add the missing paired first-pass and follow-up packets for scenarios A and B, and to label C's pair. If the brief meant something narrower by "follow-up packet", that is a check-definition question for the human gate to settle.

Risk-bearing value enumeration. The diff of PR #1405 introduces five literal scope-basis identifiers: changed-behaviour, acceptance-obligation, material-claim, safety-consequence and design-fit, at lines 61, 64, 67, 70 and 74 of the deskkit reviewscope source. The Deliverables name one numeric threshold, the round cap "Default cap N = 3" at line 563 of the pr-review-desk SKILL. That cap predates PR #1405 and is unchanged by it: it is present at the PR's parent 413383fe619c and at this source. All six are reversible edit-and-redeploy review-policy knobs, and none moves funds, data or authority irreversibly.
- RISK-VALUE: DERIVED — round cap N = 3 @ plugins/assay/skills/pr-review-desk/SKILL.md:563 — the ruling on #1402 says "Preserve the existing three-round policy", and the brief says the two-round proposal is not adopted. The value is the pre-existing literal, byte-unchanged across the implementing change.
- RISK-VALUE: DERIVED — BasisAcceptanceObligation = "acceptance-obligation" (with changed-behaviour, material-claim and safety-consequence) @ tools/desk/internal/deskkit/reviewscope.go:61-70 — these are the four bases the ruling enumerates verbatim: changed behavior, explicit acceptance obligation, material PR-body/Verify claim, concrete safety consequence.
- RISK-VALUE: NAMED, NOT DERIVED — BasisDesignFit = "design-fit" @ tools/desk/internal/deskkit/reviewscope.go:74 — this fifth basis is not in the ruling on #1402. A later change added it (commit d1258928d, build-less-brittle/06), and the source marks it advisory at landing. Whether it falls inside this brief's ruled boundary is for the human gate. This pass cannot derive it from the brief's own ruling.

Verification-Attestation: medici-finance/assay#3710 run=e48064de047177b737e8e960b2efd2568933b249516df2192e3f21451b062964 source=b1cf3d7907080ca6ff8dc3b023a0e7558af1123e brief=docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md model=claude-opus-5-5 tier=strong

**VERIFY: FAIL** — rows 1 to 3 pass (exit 0 in the witness, named PASS lines by hand), and all eight row-4 cases score in agreement. Row 4 still fails its stated corpus obligation, because the corpus lacks a paired first-pass and follow-up packet for each test scenario. This is a first stamp only: the gate:human sign-off stays with the human and is not given here.

Re-verification pass, 2026-10-20, by a fresh dispatched verifier (claude-opus-5-5, Opus 5.5 with 1M context, strong tier). It is not this item's implementer, not the author or reviewer of PR #1405 or PR #3751, and not the earlier verifier. It ran at merged source 14eeef75b19e368598f406928d9df8781c3674f0, which contains PR #3751 (merge 2624ebeb37ce), the corpus rework that answers the earlier pass's row-4 finding (defect #3748). The earlier Evidence block above stays as it was landed. This block re-runs every row, 1 to 4. Re-derivation of the original Verify table (deskpathguard rederive at this source) returns 4 rows, 0 author-added, and each row's class, command and expect match the table above.

Execution witness: `statusgen verifyrun --sandbox container-netns` (statusgen built from this tree), run in a throwaway clone of this home in a local docker container. The image is assay-verify:go1.27 (id sha256:955fd6c46c93). The container ran with `--network none`, GOPROXY=off, GOTOOLCHAIN=local and the engine's default seccomp profile, under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). verifyrun reported 4 pass, 0 fail and 0 could-not-run of 4 Verify rows, and `verifyrun --check` agreed. Row 4's witness pass is exit status only. Row 4's reviewer obligation is scored by hand below.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeFirstPassInventory$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:bd62fb499caf | 2026-10-20 | assay-verifier-app[bot] @ 14eeef75b19e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeRequiredAndUnrelated$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:05e58f98c02e | 2026-10-20 | assay-verifier-app[bot] @ 14eeef75b19e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskdispatch/ -run ^TestReviewScopeNoSilentPromotion$ -v -count=1` | pass exit=0 sandbox=container-netns | sha256:70e9d90229e2 | 2026-10-20 | assay-verifier-app[bot] @ 14eeef75b19e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cat docs/streams/desk-supervision/review-scope-cases.md` | pass exit=0 sandbox=container-netns | sha256:7edf880e5abb | 2026-10-20 | assay-verifier-app[bot] @ 14eeef75b19e (on-behalf-of human:ian) (forge-identity) |

By-hand re-run on the darwin host (go1.27.2 darwin/arm64), same source:

| # | Command | Observed | Date | Runner |
|---|---------|----------|------|--------|
| 1 | row 1 command as written | exit 0; `--- PASS: TestReviewScopeFirstPassInventory` | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |
| 2 | row 2 command as written | exit 0; `--- PASS: TestReviewScopeRequiredAndUnrelated`, with all four subtests passing (omitted required operator table, unrelated pre-existing prose, safety consequence outside edited lines, co-location without a basis) | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |
| 3 | row 3 command as written | exit 0; `--- PASS: TestReviewScopeNoSilentPromotion` | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |
| 4 | row 4 command as written | exit 0; 19894 bytes, sha256 7edf880e5abbd322567bae8084142211404059d629964732b9d190acd22566bf (git blob ef983af7f90e), which matches the witness row-4 hash | 2026-10-20 | dispatched verifier (claude-opus-5-5), darwin host |

The executable specification (the deskkit reviewscope source), the three tests, review-prompt clause 13 and the pr-review-desk SKILL are all byte-unchanged between the earlier pass's source b1cf3d790708 and this one. PR #3751 changed only the corpus and a changelog fragment.

Discrimination (supplemental, in the scratch witness clone at this source; it discharges no row and only shows that rows 1 to 3 can fail): four one-line mutations of the executable specification were each applied on their own, and each turned its row red (exit 1). M1, an incomplete search allowed to certify clean, failed row 1. M2, a basis-backed finding outside the edited lines downgraded to follow-up, failed row 2. M3, the no-silent-promotion guard removed, failed row 3. M4, a late sibling keeping its re-discovered class id, failed row 3. The restored file was byte-identical to the source and passed all three rows again.

Row 4: what the bar requires, read from the brief before reading the corpus. The paragraph above row 4 and row 4's Expect cell set four obligations:
- (a) Composition: for each test scenario (rows 1, 2 and 3), the corpus has a first-pass review packet and a follow-up packet.
- (b) An independent reviewer scores all three scope outcomes, meaning the row 1 to 3 Expect cells, against the contract.
- (c) The reviewer records model and version, packet hashes and any disagreements.
- (d) Any disagreement prevents acceptance.

Obligation (a), composition, is met at this source. Each scenario carries an explicitly labelled first-pass review packet and one or more explicitly labelled follow-up packets:
- Scenario A (row 1): first pass A.1, contrasted with the non-conforming first pass A.2; follow-up A.3, which re-reviews A.1's fix.
- Scenario B (row 2): first pass B.6, whose findings are B.1 to B.4; follow-up B.5, which re-reviews B.6's fix.
- Scenario C (row 3): first pass C.3; follow-up C.4 (review pass 2, finding C.1), then follow-up C.5 (review pass 3, finding C.2).

The corpus's scoring record has one row for each of the 14 packets.

Obligations (b) and (c), the scores. The scorer is claude-opus-5-5 (Opus 5.5, 1M context). The corpus file hash is given in row 4 above. Each packet hash is the 12-hex prefix of the sha256 of that fenced packet's body. The packets left unchanged by PR #3751 (A.2, B.1 to B.4, C.1 and C.2) hash identically to the earlier pass. Each case was scored against review-prompt clause 13 and the executable specification (Recorded, InScopeOccurrences, CertifyClean, ClassifyFinding, AbsorbSibling):

| case | packet sha256 | expected | scored | agree? |
|---|---|---|---|---|
| A.1 | d63f2dfbeddf | complete first pass names all in-scope occurrences together | records search, scope, exclusions and revision; names both in-scope occurrences in one verdict, each with basis acceptance-obligation; the vendored match is read and excluded; round count 0 at the opening verdict | yes |
| A.2 | e49e0398e1b6 | incomplete search rejected, never certified clean | records no exclusions and no revision, so Recorded fails, and it marks itself incomplete; the would-be approval cannot certify the class clean | yes |
| A.3 | 7ca408660e6a | complete recorded re-run with no in-scope occurrence certifies the class clean | the re-run of A.1's recorded search is complete, with search, scope, exclusions and revision recorded; its only hit is the excluded vendor match, so CertifyClean admits it; the excluded match is not promoted to a blocker | yes |
| B.6 | be67de15b9f2 | recorded first pass holds only on the two basis-backed findings | Recorded holds, with "none" stated as the exclusions; the verdict holds B.1 and B.3 and routes B.2 and B.4 to linked follow-ups | yes |
| B.1 | 59b54241a5f0 | omitted required operator table is a BLOCKER | basis acceptance-obligation; the file is untouched by the diff and the finding is still a blocker | yes |
| B.2 | 0e14e20cdede | unrelated pre-existing prose is a FOLLOW-UP | no basis, unrelated and pre-existing, so it goes to a linked follow-up | yes |
| B.3 | fc3d4854d2e6 | safety consequence outside edited lines is a BLOCKER | basis safety-consequence, demonstrated through the change; being outside the edited lines does not downgrade it | yes |
| B.4 | f721d02a7886 | co-location only is a FOLLOW-UP | no basis; sharing a directory is not scope | yes |
| B.5 | 2f67034f4f7f | blockers cleared only by evidence at the new head; follow-ups not promoted | B.1 and B.3 are each cleared by what was read at the fix head; B.2 and B.4 were previously non-blocking, with no changed impact and no new evidence, so they stay follow-ups; APPROVED | yes |
| C.3 | bb88d815b051 | recorded first pass holds only on basis-backed occurrences; advisory note non-blocking | holds on the two acceptance-obligation occurrences; the basis-less note goes to a follow-up; the literal-search gap at the reworded required-deliverable line is stated as review's own coverage gap | yes |
| C.4 | 33ce35d35319 | holds only on the unfixed basis-backed occurrence; round count 1 | holds only on the unfixed overview occurrence; round count 1, by the ledger's rule (opening 0, plus 1 per re-review after a worker response) | yes |
| C.1 | 35076f204d25 | no promotion without changed impact or new evidence | previously non-blocking; an unrelated edit is neither changed impact nor new evidence, so it stays a follow-up | yes |
| C.5 | 528a47216fc9 | late sibling held in the existing class; round count 2, below the cap | the reworded sibling (acceptance-obligation) is folded into the existing class as review's coverage failure, not a fresh class; round count 2, below the unchanged cap of 3 | yes |
| C.2 | 137f2a5903bb | late sibling keeps class and round count | the re-discovered label is absorbed into the existing class id; the round count stays 2, neither reset nor charged as a fresh class | yes |

All 14 scored outcomes agree with the expected ones. Every scenario's first-pass and follow-up packets together cover its scope outcome: scenario A covers row 1, B covers row 2 and C covers row 3. There are no outcome disagreements.

Scorer note (an observation, not an outcome disagreement; for the human gate). A.1, A.3 and C.3 record the grep command over src/ and docs/ only, while the scope they declare includes references to the claim across the repository. Their excluded vendor/ hit is recorded as read rather than as a grep result. I scored these packets under one reading: vendor/ is the stated exclusion, the reviewer read it by hand, and no further synthetic tree exists. Under that reading the declared scope is covered. The packets do not state that reading, though, and a stricter reading would want the vendor read recorded as part of the search. This looseness is already in A.1 as the earlier pass scored it, and PR #3751 did not change it. It changes no scored outcome above. It is a candidate for a linked follow-up that tightens the recorded search line.

Risk-bearing value enumeration. The risk metadata is present: all four fields are "no", and the item is not irreversible. PR #3751 is the only change to the deliverables since the earlier pass. It touches the corpus and a changelog fragment and adds no code literal. Its corpus prose restates the round cap ("the unchanged cap of 3", corpus line 47) and illustrative round counts of 0, 1 and 2. The literals in the implementing change and the Deliverables are unchanged since the earlier pass. They are five scope-basis identifiers in the deskkit reviewscope source at lines 61, 64, 67, 70 and 74, and the round cap "Default cap N = 3" at line 563 of the pr-review-desk SKILL. The cap line is byte-identical to line 416 of the SKILL at PR #1405's parent 413383fe619c. All of these are reversible edit-and-redeploy review-policy knobs.
- RISK-VALUE: DERIVED — round cap N = 3 @ plugins/assay/skills/pr-review-desk/SKILL.md:563 — the brief's recorded ruling on #1402 says "Preserve the existing three-round policy", and the brief does not adopt the two-round proposal. This is the pre-existing literal, byte-unchanged across both changes. The corpus restatement at line 47 matches it.
- RISK-VALUE: DERIVED — BasisAcceptanceObligation = "acceptance-obligation" (with changed-behaviour, material-claim and safety-consequence) @ tools/desk/internal/deskkit/reviewscope.go:61-70 — these are the four bases the ruling names: changed behavior, explicit acceptance obligation, material PR-body/Verify claim and concrete safety consequence.
- RISK-VALUE: NAMED, NOT DERIVED — BasisDesignFit = "design-fit" @ tools/desk/internal/deskkit/reviewscope.go:74 — this fifth basis is not in the ruling on #1402. A later change (build-less-brittle/06) added it, and the source marks it advisory at landing. This pass cannot derive it from the brief's own ruling, so whether it falls inside the ruled boundary is for the human gate.

The decision record was read from the brief's Recorded ruling section at this source. It was not fetched again from the forge, and this pass does not reinterpret it.

Verification-Attestation: medici-finance/assay#3761 run=dd63c6f42fb0c781032fed84d6fbff5ba93d583fcf70f3476c6e0884a51415b9 source=14eeef75b19e368598f406928d9df8781c3674f0 brief=docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 4 of 4 rows. Rows 1 to 3 exit 0 in the container-netns witness and show named PASS lines by hand. Row 4's corpus now pairs a first-pass packet with follow-up packets for every scenario, and all 14 packets score in agreement. This is a model stamp only: the gate:human sign-off stays with the human and is not given here.

## Review

Gate: human. Verify the recorded decision and every bypass-negative before acceptance. This is a review-control change, not automatic approval of the incident PR.
