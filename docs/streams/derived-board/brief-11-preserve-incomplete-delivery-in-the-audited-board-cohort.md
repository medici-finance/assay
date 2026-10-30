---
brief: assay:assay:derived-board:11
title: Preserve incomplete delivery in the audited board cohort
why: >-
  The board promotes an unfinished performance brief because its merged slice lacks
  a partial-delivery declaration. Binding that source repair to the independently
  reviewed board cohort prevents another plausible completion claim from hiding owed work.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-20 by assay-worker-app
sources:
  - https://github.com/medici-finance/assay/issues/2703#issuecomment-6302222772
  - https://github.com/medici-finance/assay/pull/3200#pullrequestreview-5592762972
  - https://github.com/medici-finance/assay/pull/3200#pullrequestreview-5592834194
  - https://github.com/medici-finance/assay/pull/3759
  - https://github.com/medici-finance/assay/pull/3745
  - https://github.com/medici-finance/assay/commit/44829891e8f74b1520b64f4180910d96e17be96a
  - docs/contracts.md#semantic-owners
  - freshness-checked 2026-10-20 @ cfe5c927e (main; original audit c8c4020d1 retained)
regression-of: medici-finance/assay#3787
exec-tier: strong
exec-tier-why: Cross-consumer delivery meaning and the recurring error-class design require strong execution (questions b and d).
domain: complicated
outcome: none
consumers:
  - "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (register the actual source-reading test; existing ci.yml already runs unfiltered on both events)"
  - "docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md: fixed-here (supported partial declaration and version increment; owed tasks and verification preserved)"
  - "statusgen/witnessadmission_test.go: out-of-scope (existing hermetic owner controls reused unchanged)"
  - "statusgen/boardcohort_test.go: fixed-here (bounded test-only membership/disposition join and existing owner flow)"
  - "statusgen/testdata/witness-admission/: fixed-here (bounded public primary evidence, recorded matcher fields and mutation script)"
  - "docs/streams/derived-board/README.md: out-of-scope (existing narrative and producer-owned cells remain unchanged)"
  - "statusgen/briefdelivery.go: out-of-scope (reuse existing reader and malformed/unresolved metadata floors unchanged)"
  - "statusgen/briefauthoring.go: out-of-scope (reuse shared admission and Evidence classification unchanged)"
  - "statusgen/lifecycle.go: out-of-scope (existing partial fold and legitimate full-witness meaning preserved)"
  - "statusgen/reconcile.go: out-of-scope (existing shared admission caller exercised unchanged)"
  - "statusgen/reconcilebackfill.go: out-of-scope (existing historical matcher exercised unchanged)"
  - "statusgen/reconcileapply.go: out-of-scope (existing refusal and protected-cell boundary preserved)"
  - "statusgen/regen.go: out-of-scope (existing drift consumer exercised unchanged)"
  - "docs/streams/statusgen/README.md: out-of-scope (legitimate board producer alone regenerates lifecycle cells after source merge)"
  - "tools/desk/cmd/fanoutloop/represented.go: out-of-scope (separate delivery classifier; canonical S-delivery owner remains unknown)"
  - "tools/desk/cmd/deskdispatch/phantom.go: out-of-scope (separate delivery classifier remains outside this bounded repair)"
  - "tools/desk/cmd/desksupervise/reconcile.go: out-of-scope (separate delivery classifier remains outside this bounded repair)"
---

# Brief 11 — Preserve incomplete delivery in the audited board cohort

## Context

files:
- `docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md`
- `statusgen/witnessadmission_test.go` (read-only existing controls)
- `statusgen/boardcohort_test.go` (delivered)
- `tools/desk/internal/deskkit/citrigger_test.go` (source-reader registration)
- `statusgen/testdata/witness-admission/board-cohort.json` (delivered)
- `statusgen/testdata/witness-admission/board-cohort-provenance.md` (delivered)
- `changelog/audited-board-partial-delivery.md` (delivered)

facts:
- source: At c8c4020d1, statusgen/26 has no delivery declaration for #3759. Its Tasks 3/4 and Verify 2 remain owed: snapshot reuse inventory/parity and synthetic workloads. The workload script/directory and the named parity test are absent. PR #3759 merged as 44829891e8f74b1520b64f4180910d96e17be96a on 2026-10-20T20:13:29Z; its actual body explicitly delivers sub-split (a), retains in-progress, and disclaims the remaining work. The forge and local merge object agree on its eleven changed files.
- bounded-membership: Independent correctness review 5592762972 and security review 5592834194 each inspected all ten changed rows at board head a1d5037539688ef46cf2dbc8be380a40ecfa1d35, against main c8c4020d1 and merge base 0d6a1bb5bc835c2fe80d0b23da53faba23964abf. The actual board diff/body plus both reviews fix membership independently of declarations and regression expectations. This is not every candidate in repository history. Correctness F-impl-witness-2 retains its implemented-without-delivery class and round history; security S-impl-witness-1 is the same occurrence, not a reset or separate class.
- class: A merged slice is treated as full delivery despite owed acceptance tasks; a regression built only from known declarations cannot detect a missing declaration at another brief.
- owner: S-delivery has no declared canonical semantic owner. Existing statusgen/briefdelivery.go reads supported declarations, and shared admission/fold already enforce partial meaning. No reader or classifier change is needed for this source repair.
- reproduction: Correctness independently attempted read-only reconciliation but timed out reading the pull-list body: lookedAt=false, wouldRefuse present, could-not-check. Security separately built from main c8c4020d1 and reproduced all ten actual pairs: lookedAt=true, no unread input or wouldRefuse, within 74 wouldApply / 57 held / 525 cells / ten unknown cells. That successful scoped run does not rewrite the correctness timeout or certify the other potential promotions.
- neighbor: PR #3745 (merge 4cd89433272a47d0ba99f9f01f4c559885871fb8) describes an interim perf-v2 step, carries Issue #3109 rather than a Brief trailer, and leaves statusgen/26 owed. The body matches the existing backfillCandidates substring rule, but prRecordsFromPulls excludes its issue-only trailer. backfillBase normally does not revisit a partial-trailer in-progress cell; preserve and exercise this caller distinction. Refresh actual membership and eligibility at pickup, rather than assuming body citations are delivery.
- limits: The 34 corrected-witness candidates, 87 machine-classified cases, suspected cases and untraced history in issue #2703 remain individually unproven by this ten-row audit. Existing derived-board/07 and desk-tools/34 partial repairs and their metadata/guard floors remain intact.
- scheduling: At c8c4020d1 the source reader, admission and reconcile seams below are landed; no dependency on the unimplemented candidate-coverage helper planned by Authors derived-board/10 in PR #3794. That authoring merged as da85bf699 on 2026-10-20T23:16:21Z; refreshed main 8b7eec32f includes brief 10 but still lacks the helper and the #3759 declaration. The latest premerge reviews of 3794 were delta reviews; no new full review pair is claimed. Brief 11 is independently Wave 0, one bounded M/strong source-and-regression unit.

The fixed membership is:

| Brief | Reviewed witness | Review disposition |
|---|---|---|
| apps-installer/10 | #3171 | full delivery examined |
| brief-quality/01 | #1719 | full design delivery examined; no implementation of its follow-ons claimed |
| contributor-trust/09 | #980 | full delivery examined |
| desk-containers/06 | #3729 | source and offline checker delivered; no live deployment claimed |
| desk-tools/27 | #1196 | full delivery examined; VERIFY FAIL does not erase implementation |
| desk-tools/37 | #3331 | full bounded test/oracle delivery examined |
| graph-execution/28 | #3728 | full implementation examined; independent Verify still owed |
| graph-execution/32 | #3307 | bounded corpus/report delivered; no production mechanism claimed |
| measured-status/01 | #1726 | full delivery examined |
| statusgen/26 | #3759 | partial; Tasks 3/4 and Verify 2 still owed |

layering: Keep the existing source declaration and shared admission/fold. A bounded test-only membership/disposition join, adapting existing source/admission/reconcile test seams, checks coverage; production admission enforces the declaration. Tests call those actual owners with hermetic forge seams (Tasks 1–3, Verify 1–5), not a new runtime parser or prose classifier.

design-fit:
  owner: unknown (S-delivery); bounded source owner is statusgen/26 delivery metadata
  contract: S-delivery
  retires: []
  weight: verbs +0, flags +0, runtime refusals +0, rule-text lines +0; one source declaration and bounded tests/fixtures
  why-add: n/a; the existing reader already expresses the repair without a production mechanism

single-point-of-failure: Metadata alone cannot establish completeness. The independent fixed-cohort review supplies membership and evidence; the existing reader supplies enforcement. Missing source membership and malformed metadata fail differently. Semantic adequacy remains independent review; neither layer clears candidates outside this cohort.

## Read first

- The source issue design note and both full independent board reviews linked in sources; this is design-first follow-up to the class stop, not a waiver.
- PR #3759's full body, merged files and statusgen/26 Task/DoD/Verify; compare the exact source obligations with actual deliverables.
- `statusgen/briefdelivery.go`, `briefauthoring.go`, `lifecycle.go`, `reconcile.go`, `reconcilebackfill.go`, `reconcileapply.go`, `regen.go`.
- `statusgen/witnessadmission_test.go`, `reconcilepartial_test.go`, `reconcileapply_test.go`; `docs/contracts.md`, `spec/brief-v1.md`, `docs/go-engineering.md`.

## Ground rules

- Implement only after independent review and human merge of this authoring PR. No production change is delivered by the authoring PR.
- Keep all work in Assay. No infrastructure contact, workflow trigger, raw push, ready flip, generated-cell edit or verification stamp. Publish source through the normal draft PR path; human merge remains separate.
- Preserve statusgen/26 Tasks 3/4 and Verify 2 verbatim; no moving, waiving or inventing child delivery. This brief does not implement them.
- Missing/contradictory provenance or new scope is NEEDS_CONTEXT/could-not-check; do not manufacture a full/partial decision to make the board pass.

## Interface and consumer flow

| Consumer | Existing seam and future proof |
|---|---|
| Brief declaration and alias registry | readDeclaredPartials and resolveDeliveryRef read the real statusgen/26 source; no alternate parser (rows 1, 4) |
| Trailer and historical candidates | prRecordsFromPulls and backfillCandidates feed newAuthoringClassifier, nonDelivery and markDeclaredPartials under backfillBase eligibility; actual in-progress #3759 case does not enter fallback (rows 1–3, 6) |
| Lifecycle and verification | deriveOne and classifyPRs retain in-progress for partial-only and a genuine full winner under later partial plus VERIFY FAIL (rows 2, 5) |
| Drift and writer | regenDriftNotices and runReconcileCaptured exercise the real flow and existing apply/no-write boundary in isolated fixture roots (rows 2, 4, 6) |
| Generated board | Normal producer after source merge; fresh trust/reviews/human merge remain separate receipts, never an agent-triggered test |

## Task

1. **Bind the bounded cohort to primary evidence.** Record the ten pairs above from the actual pinned board diff/body and both independent full reviews, separately from expected dispositions and declarations. Store only the bounded public primary excerpts and matcher fields needed, with source URLs, revisions and hashes. Compare the independently extracted identities before accepting any disposition. Re-read each cited delivery's actual body, merge/file evidence and brief obligations at pickup; keep exact matching fields and only the public scope evidence needed, with hashes/URLs and reviewed revisions. For #3759 independently confirm the real eleven-file merge set and the still-missing named workload/parity deliverables. Source evidence for #1196 must establish the real full delivery against the scoped Task/DoD, not copy the generic deliveryFiles test stub; #2295 remains the Evidence-only counterexample. Scrub fixtures before public commit; unpublishable required evidence holds the proof. Derive the exact ten-row membership from the pinned board tree diff and compare with the independent review manifest; missing, extra, duplicated or mismatched pairs fail by identity. If the board changes, preserve this historical bound and report a separate amendment; do not silently expand it or infer every other candidate is full. Separately enumerate actual merged trailer and historical backfill candidates for statusgen/26 at pickup through existing prRecordsFromPulls and backfillCandidates, retaining collection completeness/unread information and the cutoff. Distinguish raw potential body matches from candidates eligible under the real current cell/base state. Route #3745 as a potential body match but not a current witness after the #3759 partial trailer establishes in-progress; its actual issue-only trailer and interim scope are independently recorded above. A newly discovered actually eligible candidate without supported scope is a named unresolved candidate and could-not-check/hold on #2703. A hypothetical different base state authorizes neither another declaration nor a semantic change.
2. **Declare the partial truth and exercise the real owner.** Add the supported delivery entry `{in: "assay#3759", covers: partial, note: "sub-split (a); snapshot reuse, workloads and Verify 2 remain owed"}` to statusgen/26, using its existing registry identity. Preserve all Task/Verify/Evidence content and existing declarations; add only this delivery entry and increment the brief version if required by its metadata contract. Do not weaken any other acceptance or provenance field. Production Go code stays byte-identical. Add a regression that reads current source declarations rather than building them in the test: before the entry, #3759 can become the false implemented witness; after it, statusgen/26 remains in-progress with no implementation witness. Use actual recorded matcher fields/merge identity through trailer admission and the existing backfill eligibility boundary: the real #3759 partial trailer yields in-progress and does not enter fallback. Exercise existing legitimate historical backfill controls separately, preserving their distinct fixture/base conditions; never strip the real trailer to pretend that historical mutation is the current production case. Record pre-fix red and restored green; isolated omission mutations remain re-runnable after the source fix. No PR-number special case, arbitrary prose heuristic, duplicate classifier, new runtime verb or changed meaning of full delivery.
3. **Guard the bounded class across consumers.** Check every member of the independently fixed cohort against its evidence disposition and source through shared admission, lifecycle, drift and isolated apply, retaining each caller's actual eligibility conditions. Require #3759 excluded as a completion witness without erasing any reviewed genuine delivery. Plant a second, older partial candidate in an isolated unaffected brief's recorded matcher input, absent from BOTH the expected disposition record and source declarations; the coverage join must fail naming the exact unresolved brief/PR, even when newer known partials are excluded. Do not pre-label the plant in the expected list, derive membership from declarations, or assume an unlisted candidate is full. Retain that mutation as a positive control; disabling the coverage join must make its test fail. It is explicitly synthetic, never evidence that a real full delivery is partial. Also bypass the coverage guard and show that the existing reader alone excludes a declared partial; malformed/unresolved metadata or missing forge/file evidence must retain unknown/refusal and zero writes. Preserve Evidence-only and authoring-only exclusions, full-after-partial behavior, a genuine full delivery despite later partial and VERIFY FAIL, cached reads, provenance identity, and all protected lifecycle cells. Partial alongside an earlier genuine full delivery must not erase that full witness.
4. **Hand off source, not a board stamp.** Record actual commands, revisions, red/green and limitations in the implementation PR. Stop at implemented after source review/human merge. Only the legitimate producer regenerates the board; this brief neither triggers it nor commits generated cells. Fresh regenerated board content requires current human trust admission, independent full reviews and human merge before independent graph-execution/28 Verify and then graph-execution/29. Retain both existing independent reconciliation outcomes: correctness timed out; security reproduced the ten pairs at the recorded head. Neither an offline fixture nor that old successful snapshot certifies the later repaired board; obtain fresh receipts there. Keep #2703 open. This ten-row acceptance neither depends on nor certifies derived-board/10's independent derived-board/07 historical-cohort proof. It does not clear the 34/87 broader audit or qualify Linux containers/Kubernetes runtime deployment.

If proof needs a production semantic change, new classifier, owner, flag, rule, refusal, runtime gate or broad new collection/parser mechanism, stop with NEEDS_CONTEXT. Test-only evidence joins may compare identities and reviewed dispositions; they must not interpret prose as a semantic classifier. Reuse landed seams first; no dependence on a sibling future helper.

## Definition of Done

The supported #3759 declaration and production-owner regression stop the false promotion without weakening legitimate full witnesses. Every fixed-cohort member has primary evidence and an explicit tested disposition, including a discriminating second-site plant and independent lower-reader failure tests. Production Go, other declarations, and all unfinished statusgen/26 obligations remain unchanged. Source implementation receives its own review and human merge; generated board and runtime acceptance remain separately owned.

## Verify

Run from the repository root. Named tests use hermetic fixtures. Worker receipts are recorded below; independent post-merge verification remains owed. Each command asserts a real PASS. The existing exact-class runner skips +qualified rows: a nonimplementer must explicitly execute every applicable command and record its output; parser enumeration is not execution. Unavailable Git objects/evidence are could-not-check. Semantic evidence adequacy requires independent review.

| # | Command | Expect | Class |
|---|---|---|---|
| 1 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortSource$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortSource' "$o")` | exit 0; exactly ten pairs from the independently pinned board diff and review manifest, not declaration enumeration; source-bound #3759 test is red before metadata and green after; missing/extra pair fails with identity | check:ci +dereference +mutation |
| 2 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFlow$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFlow' "$o")` | exit 0; actual trailer → admission → lifecycle → drift → isolated apply keeps statusgen/26 in-progress without completion witness or fallback; separate legitimate historical controls exercise backfill; omission mutation catches false implemented; protected cells unchanged | check:ci +flow |
| 3 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortPlant$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortPlant' "$o")` | exit 0; committed older second-site candidate omitted from both expected/source records fails coverage naming its brief/PR; disabling the join makes the control fail; bypassed guard still leaves the real lower reader enforcing a declared partial | check:ci +mutation |
| 4 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFloors$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFloors' "$o")` | exit 0; malformed/unresolved declaration, missing file evidence or mismatched provenance cannot produce a clean witness or apply; unknown/refusal and zero writes; pinned delivery snapshots preserve Task/Verify/Evidence bytes outside the entry/version; current reader accepts later comment/version/Evidence/additional-partial maintenance while retaining #3759 partial | check:ci +mutation |
| 5 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFull$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFull' "$o")` | exit 0; actual evidenced #1196 full remains implemented despite later partial and VERIFY FAIL; Evidence-only #2295 cannot replace it; all nine bounded full dispositions retain their reviewed meaning and cache behavior | check:ci +neighbour |
| 6 | `(cd statusgen && for t in TestRolloutSourceAdmission TestCISliceSourceAdmission TestOwedTaskSourceAdmission TestEvidenceAdmission TestAdmissionCacheAndDelivery TestPartialBackfillSkips TestPartialRegenDriftAgrees TestApplyReconcileWrites; do o=$(mktemp); go test . -run "^$t" -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- "--- PASS: $t" "$o"; r=$?; if test "$r" -ne 0; then exit "$r"; fi; done)` | exit 0; existing neighbors pass, including all five apply-prefix tests; no existing partial declaration or protected-cell floor weakened | check:ci +neighbour |
| 7 | `o=$(mktemp); git diff --name-only 44829891e8f74b1520b64f4180910d96e17be96a^1 44829891e8f74b1520b64f4180910d96e17be96a > "$o" && python3 -c 'import pathlib,sys; actual=set(pathlib.Path(sys.argv[1]).read_text().splitlines()); expected={"changelog/statusgen-26-v3.md","docs/streams/statusgen/README.md","docs/telemetry.md","statusgen/brieffile.go","statusgen/dispatchqueue.go","statusgen/forgeread.go","statusgen/load.go","statusgen/main.go","statusgen/parse.go","statusgen/perfrecord.go","statusgen/perfrecord_test.go"}; assert actual==expected, actual; print("PR 3759: exact eleven delivery paths confirmed")' "$o"` | exit 0; exact confirmation; real Git merge independently corroborates the fixture's delivery paths; semantic completeness still reviewed against the actual brief | check +dereference |
| 8 | `statusgen --root . --consumers --brief derived-board/11 --delivery "$DELIVERY" --base 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7` | Set DELIVERY to the immutable implementation delivery commit recorded below; exit 0; 4 corroborated, 0 disproved, 13 unchecked. Unchecked scope remains review. For the pre-merge authoring check omit --delivery and compare the complete working branch against refreshed main cfe5c927ec7049728f4b97306beaedf43a444b9f. | check |
| 9 | `statusgen --root . --lint` | exit 0; LINT PASS; no new PROBLEM or generated-cell write | check |

## Pre-mortem

| Failure mode | Caught by |
|---|---|
| Regression copies declarations and omits another reviewed candidate | 1 derives the fixed cohort from independent board diff/review; 3 plants a second site |
| Only trailer admission is fixed while backfill/drift/apply promotes the slice | 2 and 6 |
| A full witness or verification failure is confused with incomplete delivery | 5 and 6 |
| Missing data or malformed metadata silently defaults to full | 4; lower reader tested with coverage bypassed in 3 |
| Metadata repair rewrites owed tasks or other protections | 4 pinned delivery-time byte checks and 6 neighbor controls; current declaration omission still fails 1/2 |
| A historical byte proof freezes the living brief | 4 permits later comment/version/Evidence/additional-partial maintenance through current readers |
| A fixture's semantic disposition is wrong despite valid formatting | review-only: dereference primary scope, obligations and actual files; row 7 independently binds #3759 paths |
| Ten reviewed rows are mistaken for all history, or source merge for approved board/runtime | review-only: enforce fixed membership and separate producer/trust/review/merge/verifier receipts |
| Empty selector or skipped qualified row is presented as execution | PASS assertions and explicit nonimplementer command receipts |

## Evidence

Not executed at authoring. Worker source checks on 2026-10-20 use delivery
`92315c7fa0f0b1e9a5e66a3c6dfdabc8a1d0cde7` against base
`5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7`. Set `DELIVERY` to that immutable
source commit for row 8's original delivery check. Review repair
`85fe816784cb1b75198615824d71de3101d8fa12` follows a normal merge of main
`cfe5c927ec7049728f4b97306beaedf43a444b9f`; it changes the test proof and fixture
provenance, not the source declaration or routing. The repaired standing proof
uses the pinned before/after blobs for delivery-time byte identity, while live
source readers tolerate subsequent maintenance.

| Rows | Worker observation |
|---|---|
| 1–5 | All five named tests print PASS. Initial source test without the declaration fails naming statusgen/26#3759; rerunnable omission also exposes the actual false implemented witness `PR #3759 (merged 4482989)`. Restored declaration gives in-progress and no implementation witness. |
| 1–5 mutations | Baseline, full-suite live-source comment/version/Evidence/additional-partial maintenance, and restored runs PASS. Source omission, flow omission, disabled coverage join, disabled write refusal and disabled Evidence exclusion are all CAUGHT. Exact source restoration and production Go byte identity PASS. |
| 6 | All eight selectors PASS, including all five TestApplyReconcileWrites prefix cases. |
| 7 | Actual merge independently confirms the exact eleven delivery paths. Named workload files and TestInvocationSnapshotReuseParity remain absent. |
| 8 | Original pinned delivery/base and complete current branch against refreshed main: 4 corroborated, 0 disproved, 13 unchecked, 0 claimless briefs. Repair commit 85fe81678 alone against first parent 83e882001 gives 2 corroborated, 2 disproved, 13 unchecked: that delta does not redeliver the existing declaration or registry row. It is not the whole delivery check. A non-first-parent range was could-not-check and is not a pass. Unchecked entries remain review judgments. |
| 9 | Source-built statusgen reports LINT: PASS. |
| Supplement | TestCLIInventory PASS; test-departure ledger clean. The two legacy cross-module registry tests SKIP because tools.yml is absent in this published tree: could-not-check, not passes. Direct inspection confirms ci.yml has unfiltered push/pull_request events and runs the full statusgen suite. |

Review C2's harmless-comment reproduction failed before the repair. The full
standing suite now passes after temporary live-source maintenance, and changed
pinned delivery evidence is rejected. The omission helper removes the exact entry
regardless of later-entry order. A1 corrects #3745's original line mapping; A2
names the other members' omitted trailers; A3 marks the reused test file read-only.
The mutation launcher now forwards only named build/runtime environment variables;
Go environment-file loading and downloads are disabled. This does not claim that
fake-server tests open no sockets.

The mutation entry is `statusgen/testdata/witness-admission/mutate-board-cohort.test.py`.
A nonimplementer must rerun every applicable row on merged main; these are worker
checks, not independent verification. Production Go is byte-identical to refreshed main;
statusgen/26 permits only its declaration and version increment. No generated
board, Kubernetes deployment, Linux runtime or Windows qualification is claimed.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `o=$(mktemp); git diff --name-only 44829891e8f74b1520b64f4180910d96e17be96a^1 44829891e8f74b1520b64f4180910d96e17be96a > "$o" && python3 -c 'import pathlib,sys; actual=set(pathlib.Path(sys.argv[1]).read_text().splitlines()); expected={"changelog/statusgen-26-v3.md","docs/streams/statusgen/README.md","docs/telemetry.md","statusgen/brieffile.go","statusgen/dispatchqueue.go","statusgen/forgeread.go","statusgen/load.go","statusgen/main.go","statusgen/parse.go","statusgen/perfrecord.go","statusgen/perfrecord_test.go"}; assert actual==expected, actual; print("PR 3759: exact eleven delivery paths confirmed")' "$o"` | pass exit=0 | sha256:ea0361a76ccf | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief derived-board/11 --delivery "$DELIVERY" --base 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7` | pass exit=0 | sha256:cba4c48dfef6 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:a621a2dd6336 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4297 run=21613db84b99d88311625b29f0df29fbbaf2a4115174aeeb2ebcfb5297598d5e source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortSource$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortSource' "$o")` | pass exit=0 sandbox=container-netns | sha256:49fcd25d2085 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFlow$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFlow' "$o")` | pass exit=0 sandbox=container-netns | sha256:0f8345f9b70f | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortPlant$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortPlant' "$o")` | pass exit=0 sandbox=container-netns | sha256:f3afc71942ac | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFloors$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFloors' "$o")` | pass exit=0 sandbox=container-netns | sha256:bdced2cb7b29 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFull$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFull' "$o")` | pass exit=0 sandbox=container-netns | sha256:80712ec3294d | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && for t in TestRolloutSourceAdmission TestCISliceSourceAdmission TestOwedTaskSourceAdmission TestEvidenceAdmission TestAdmissionCacheAndDelivery TestPartialBackfillSkips TestPartialRegenDriftAgrees TestApplyReconcileWrites; do o=$(mktemp); go test . -run "^$t" -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- "--- PASS: $t" "$o"; r=$?; if test "$r" -ne 0; then exit "$r"; fi; done)` | pass exit=0 sandbox=container-netns | sha256:fef749e63ad8 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4297 run=21613db84b99d88311625b29f0df29fbbaf2a4115174aeeb2ebcfb5297598d5e source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=21613db84b99d88311625b29f0df29fbbaf2a4115174aeeb2ebcfb5297598d5e source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:f6f684d36c12e670925f806ec49b660b75c6984f6697b716f1de8d2e3c87ba81 opened=2026-10-24T17:09:57Z closed=2026-10-24T17:13:12Z before=receipt after=check-verifier

Independent post-merge verification, 2026-10-24, on merged main source
385cb29e5682dcb9a886b298076f722b4823ec1b (non-implementer verifier, claude-opus-5-5,
strong tier). Rows 1–6 (check:ci) ran inside the admission bracket in a network-off
container on the release-pinned verify-runner v1.0.59 image; the bracket digest reports
verdict=match with one Tree-Digest, Container-Image and attestation line. Rows 7–9 need
Git history and ran on the host through a separate disjoint row selection. Row 8 ran
with DELIVERY set to the recorded delivery commit 92315c7fa0f0b1e9a5e66a3c6dfdabc8a1d0cde7
and printed `summary: 4 corroborated, 0 disproved, 13 unchecked, 0 brief(s) claiming nothing`;
the 13 unchecked entries remain review judgments, not passes. Row 7 printed
`PR 3759: exact eleven delivery paths confirmed`. Row 9 printed `LINT: PASS`.

| Supplemental observation (no acceptance credit) | Result | Date | Runner |
|---|---|---|---|
| Mutation launcher for this brief, run in a disposable copy of the source commit outside the admitted home | baseline PASS; later-maintenance PASS; source-omission, flow-omission, coverage-disabled, write-refusal-disabled and evidence-exclusion-disabled all CAUGHT; restored PASS; source restoration and production Go byte identity PASS; exit 0 | 2026-10-24 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| statusgen/26 diff from base 5dcdff9fb to the source commit | only the version 2 to 3 bump and the single assay#3759 partial delivery entry; Tasks, Verify and Evidence unchanged | 2026-10-24 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| Owed statusgen/26 deliverables at the source commit | no Go file names TestInvocationSnapshotReuseParity; no synthetic workload files exist under statusgen; Tasks 3/4 and Verify 2 remain owed | 2026-10-24 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| Production owners named out-of-scope (briefdelivery, briefauthoring, lifecycle, reconcile, reconcilebackfill, reconcileapply, regen) | no commit touches them between base 5dcdff9fb and the source commit | 2026-10-24 | assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Discarded attempts: none of the witness runs were repeated. Before the bracket, one
offline compile probe ran in the same image without writing to the home. One manual re-run
of row 8 for its output mistakenly left DELIVERY empty, so it compared against the working
tree and reported one disproved claim. The correctly set re-run matched the witnessed run.
Neither carries acceptance credit.

Risk-bearing values. The fail-safe trigger does not fire: the risk metadata is present
with every field no, the brief is not irreversible, and no risk-classed path changed. The
enumeration over the delivery commits 92315c7fa and 85fe81678 still found these values:

- RISK-VALUE: DERIVED — covers = partial @ docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md:16 — statusgen/26 Tasks 3/4 and Verify 2 have no deliverable at the source commit (named parity test and workloads absent), and PR 3759's eleven files, confirmed by row 7, deliver only sub-split (a). So partial is the true scope and full would be a false completion.
- RISK-VALUE: DERIVED — version = 3 @ the same brief, line 14 — one increment from 2 for the added delivery metadata, which is what the brief-v2 metadata contract requires.
- Test-only bounds (members want 10 @ statusgen/boardcohort_test.go:136, reviews want 2 @ line 139, applied want 9 @ line 442, drift seen want 9 @ line 476, collection cutoff 3803 @ line 371) match the ten-row fixed cohort, its two independent reviews and its nine full members. They are reversible test constants and rank last.

**VERIFY: PASS** — 9 of 9 rows pass (rows 1–6 bracketed container witness, rows 7–9 host witness); gate model; no generated board, runtime deployment or broader #2703 cohort is certified.

Independent post-merge re-verification, 2026-10-25, on merged main source
0c3b558efec39c54781c270ef639fba38376ba8f (non-implementer verifier, claude-opus-5-5,
strong tier, attestation run a13ee6bc6891). This run replaces the 2026-10-24 run above for
the lifecycle flip: that earlier run passed every row, but its supplemental table carried
Runner cells in a shape the flip does not read. Every acceptance row below is a
statusgen verifyrun witness; no Runner cell is hand-written, and supplemental observations
are recorded in prose with no acceptance credit.

Rows 1–6 (check:ci) ran inside the admission bracket in a network-off container on the
release-pinned verify-runner v1.0.60 image (linux/amd64, default seccomp profile). Rows 7–9
need Git history and ran on the host through a separate, disjoint row selection.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortSource$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortSource' "$o")` | pass exit=0 sandbox=container-netns | sha256:c9a47c8b4c33 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFlow$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFlow' "$o")` | pass exit=0 sandbox=container-netns | sha256:6e8f3e09ced2 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortPlant$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortPlant' "$o")` | pass exit=0 sandbox=container-netns | sha256:5c4db3a937b3 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFloors$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFloors' "$o")` | pass exit=0 sandbox=container-netns | sha256:b845a443ab79 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFull$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFull' "$o")` | pass exit=0 sandbox=container-netns | sha256:89d08394f5e4 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && for t in TestRolloutSourceAdmission TestCISliceSourceAdmission TestOwedTaskSourceAdmission TestEvidenceAdmission TestAdmissionCacheAndDelivery TestPartialBackfillSkips TestPartialRegenDriftAgrees TestApplyReconcileWrites; do o=$(mktemp); go test . -run "^$t" -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- "--- PASS: $t" "$o"; r=$?; if test "$r" -ne 0; then exit "$r"; fi; done)` | pass exit=0 sandbox=container-netns | sha256:cc39e5338fd4 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:b794d4196a454f598ff2f6b699cb30567ba710597411964c2ced43dac67d7a2a
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4443 run=a13ee6bc68911aae23ba022ab81a8b8772f74a113a9e5c768778c4fc8003edc9 source=0c3b558efec39c54781c270ef639fba38376ba8f brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=a13ee6bc68911aae23ba022ab81a8b8772f74a113a9e5c768778c4fc8003edc9 source=0c3b558efec39c54781c270ef639fba38376ba8f tree=sha256:b794d4196a454f598ff2f6b699cb30567ba710597411964c2ced43dac67d7a2a witness=sha256:24d0b9c9e19515099849cb441a8c6f32521dea0076cc391eb6ea8b24ece29579 opened=2026-10-25T19:37:00Z closed=2026-10-25T19:39:46Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `o=$(mktemp); git diff --name-only 44829891e8f74b1520b64f4180910d96e17be96a^1 44829891e8f74b1520b64f4180910d96e17be96a > "$o" && python3 -c 'import pathlib,sys; actual=set(pathlib.Path(sys.argv[1]).read_text().splitlines()); expected={"changelog/statusgen-26-v3.md","docs/streams/statusgen/README.md","docs/telemetry.md","statusgen/brieffile.go","statusgen/dispatchqueue.go","statusgen/forgeread.go","statusgen/load.go","statusgen/main.go","statusgen/parse.go","statusgen/perfrecord.go","statusgen/perfrecord_test.go"}; assert actual==expected, actual; print("PR 3759: exact eleven delivery paths confirmed")' "$o"` | pass exit=0 | sha256:ea0361a76ccf | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief derived-board/11 --delivery "$DELIVERY" --base 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7` | pass exit=0 | sha256:cba4c48dfef6 | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:2c2f6e9a64bc | 2026-10-25 | assay-verifier-app[bot] @ 0c3b558efec3 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4443 run=a13ee6bc68911aae23ba022ab81a8b8772f74a113a9e5c768778c4fc8003edc9 source=0c3b558efec39c54781c270ef639fba38376ba8f brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong

Key output lines. Row 7 printed `PR 3759: exact eleven delivery paths confirmed` (its output
digest equals the 2026-10-24 run's). Row 8 ran with DELIVERY set to the recorded delivery
commit 92315c7fa0f0b1e9a5e66a3c6dfdabc8a1d0cde7 and printed
`summary: 4 corroborated, 0 disproved, 13 unchecked, 0 brief(s) claiming nothing`; the 13
unchecked entries remain review judgments, not passes. Row 9 printed `LINT: PASS`. The
bracket digest for the container block reports verdict=match with one Tree-Digest,
Container-Image and attestation line.

Supplemental observations on the source commit (no acceptance credit): the statusgen/26
diff from base 5dcdff9fb holds only the version 2 to 3 bump and the single assay#3759 partial
delivery entry, with Tasks, Verify and Evidence unchanged; no Go file names
TestInvocationSnapshotReuseParity and no synthetic workload files exist under statusgen, so
Tasks 3/4 and Verify 2 remain owed; no commit between base 5dcdff9fb and the source commit
touches the production owners named out-of-scope (briefdelivery, briefauthoring, lifecycle,
reconcile, reconcilebackfill, reconcileapply, regen).

Discarded attempts: none. Each witness ran once. After the host witness, rows 7–9 were
re-run by hand in the home only to read their output lines; they matched and carry no credit.

Risk-bearing values. The fail-safe trigger does not fire: the risk metadata is present with
every field no, the brief is not irreversible, and no risk-classed path changed. The
enumeration over the delivery commits 92315c7fa and 85fe81678 found these values:

- RISK-VALUE: DERIVED — covers = partial @ docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md:16 — statusgen/26 Tasks 3/4 and Verify 2 have no deliverable at the source commit (named parity test and workloads absent), and PR 3759's eleven files, confirmed by row 7, deliver only sub-split (a); partial is the true scope and full would be a false completion.
- RISK-VALUE: DERIVED — version = 3 @ the same brief, line 14 — one increment from 2 for the added delivery metadata, as the brief-v2 metadata contract requires.
- Test-only bounds (members want 10 @ statusgen/boardcohort_test.go:137, collection cutoff 3803 @ line 371) match the ten-row fixed cohort and the recorded collection; they are reversible test constants and rank last.

**VERIFY: PASS** — 9 of 9 rows pass (rows 1–6 bracketed container witness, rows 7–9 host witness); gate model; no generated board, runtime deployment or broader #2703 cohort is certified.
Independent post-merge re-verification, 2026-10-27, on merged main source
47ef38f961aa1006ff296034d118630efdc177cb (non-implementer verifier, claude-opus-5-5,
strong tier, attestation run ac9080ce58e2). Every acceptance row below is a statusgen
verifyrun witness; no Runner cell is hand-written, and supplemental observations are
recorded in prose with no acceptance credit.

Rows 1–6 (check:ci) ran inside the admission bracket in a network-off container on the
release-pinned verify-runner v1.0.63 image (linux/amd64, default seccomp profile; digest
matches the release's verify-runner-image.txt, whose checksum matches checksums.txt).
Rows 7–9 need Git history and ran on the host through a separate, disjoint row selection.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortSource$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortSource' "$o")` | pass exit=0 sandbox=container-netns | sha256:86fca6908a07 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFlow$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFlow' "$o")` | pass exit=0 sandbox=container-netns | sha256:c261d3b300ba | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortPlant$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortPlant' "$o")` | pass exit=0 sandbox=container-netns | sha256:63c50dc753a1 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFloors$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFloors' "$o")` | pass exit=0 sandbox=container-netns | sha256:36aaebd6896a | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && o=$(mktemp); go test . -run '^TestBoardCohortFull$' -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- '--- PASS: TestBoardCohortFull' "$o")` | pass exit=0 sandbox=container-netns | sha256:5887f7967a67 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd statusgen && for t in TestRolloutSourceAdmission TestCISliceSourceAdmission TestOwedTaskSourceAdmission TestEvidenceAdmission TestAdmissionCacheAndDelivery TestPartialBackfillSkips TestPartialRegenDriftAgrees TestApplyReconcileWrites; do o=$(mktemp); go test . -run "^$t" -count=1 -timeout 60s -v > "$o" 2>&1 && grep -F -- "--- PASS: $t" "$o"; r=$?; if test "$r" -ne 0; then exit "$r"; fi; done)` | pass exit=0 sandbox=container-netns | sha256:9b5c2c1eecd5 | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4576 run=ac9080ce58e2796bd38ccbc6b70392fd6ce0e89bff92b691ebfb41fce80f1e01 source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ac9080ce58e2796bd38ccbc6b70392fd6ce0e89bff92b691ebfb41fce80f1e01 source=47ef38f961aa1006ff296034d118630efdc177cb tree=sha256:a284f4ec3df000d33368cb25837ba8237ac42a3d4307c251726ecf208aa795fd witness=sha256:46478b25f813730aad3ff01e64687e9acd0664fb28efe8e1e7af1518080845d3 opened=2026-10-27T01:28:15Z closed=2026-10-27T01:31:10Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `o=$(mktemp); git diff --name-only 44829891e8f74b1520b64f4180910d96e17be96a^1 44829891e8f74b1520b64f4180910d96e17be96a > "$o" && python3 -c 'import pathlib,sys; actual=set(pathlib.Path(sys.argv[1]).read_text().splitlines()); expected={"changelog/statusgen-26-v3.md","docs/streams/statusgen/README.md","docs/telemetry.md","statusgen/brieffile.go","statusgen/dispatchqueue.go","statusgen/forgeread.go","statusgen/load.go","statusgen/main.go","statusgen/parse.go","statusgen/perfrecord.go","statusgen/perfrecord_test.go"}; assert actual==expected, actual; print("PR 3759: exact eleven delivery paths confirmed")' "$o"` | pass exit=0 | sha256:ea0361a76ccf | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers --brief derived-board/11 --delivery "$DELIVERY" --base 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7` | pass exit=0 | sha256:249bf097816a | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --lint` | pass exit=0 | sha256:3409186eebac | 2026-10-27 | assay-verifier-app[bot] @ 47ef38f961aa (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4576 run=ac9080ce58e2796bd38ccbc6b70392fd6ce0e89bff92b691ebfb41fce80f1e01 source=47ef38f961aa1006ff296034d118630efdc177cb brief=docs/streams/derived-board/brief-11-preserve-incomplete-delivery-in-the-audited-board-cohort.md model=claude-opus-5-5 tier=strong

Key output lines. Row 7 printed `PR 3759: exact eleven delivery paths confirmed` (its output
digest equals the earlier runs'). Row 8 ran with DELIVERY set to the recorded delivery
commit 92315c7fa0f0b1e9a5e66a3c6dfdabc8a1d0cde7 and printed
`summary: 4 corroborated, 0 disproved, 13 unchecked, 0 brief(s) claiming nothing`; the 13
unchecked entries remain review judgments, not passes. Row 9 printed `LINT: PASS`. The
bracket digest for the container block reports verdict=match with one Tree-Digest,
Container-Image and attestation line. No row is risk-bearing under the fail-first
definition (the brief-wide risk fields are all no and no row's Command or Expect carries a
risk-bearing, live, mutating or end-to-end tag), so no fail-first witness is owed.

Supplemental observations on the source commit (no acceptance credit): the statusgen/26
diff from base 5dcdff9fb to the source commit holds only the version 2 to 3 bump and the
single assay#3759 partial delivery entry; no Go file names TestInvocationSnapshotReuseParity,
so Tasks 3/4 and Verify 2 remain owed; no commit between base 5dcdff9fb and the source
commit touches the production owners named out-of-scope (briefdelivery, briefauthoring,
lifecycle, reconcile, reconcilebackfill, reconcileapply, regen).

Discarded attempts: none. Each witness ran once. Before the bracket opened, one probe
listed the runner image's module cache and tool versions without mounting the home. After
the host witness, rows 8 and 9 were re-run by hand in the home only to read their output
lines; they matched and carry no credit.

Risk-bearing values. The fail-safe trigger does not fire: the risk metadata is present with
every field no, the brief is not irreversible, and no risk-classed path changed. The
enumeration over the delivery commits 92315c7fa and 85fe81678 found these values:

- RISK-VALUE: DERIVED — covers = partial @ docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md:16 — statusgen/26 Tasks 3/4 and Verify 2 have no deliverable at the source commit (the named parity test is absent), and PR 3759's eleven files, confirmed by row 7, deliver only sub-split (a); partial is the true scope and full would be a false completion.
- RISK-VALUE: DERIVED — version = 3 @ the same brief, line 14 — one increment from 2 for the added delivery metadata, as the brief-v2 metadata contract requires.
- Test-only bounds (members want 10 @ statusgen/boardcohort_test.go:136, collection cutoff 3803 @ line 371, applied want 9 @ line 442) match the ten-row fixed cohort, the recorded collection and its nine full members; they are reversible test constants and rank last.

**VERIFY: PASS** — 9 of 9 rows pass (rows 1–6 bracketed container witness, rows 7–9 host witness); gate model; no generated board, runtime deployment or broader #2703 cohort is certified.

## Review

Gate model: all four risks are no for a reversible public source declaration and hermetic controls; no live operation, sensitive data, regulatory or irreversible change is authorized. Human merge remains required. Strong execution follows the cross-artifact flow and recurring class. The fixed cohort and per-member semantic dispositions require full independent review. This is the required bounded design amendment, not an implementation retry or a class-stop waiver.
