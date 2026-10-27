# Fixed-delivery verification — proposed redesign

Status: **draft; design, existing-brief amendments and retrospective experiments only**.
This PR changes planning scope and dependencies, with no runtime, lifecycle-state,
original Verify/Evidence, permission or activation change.
The driver requested fixed-delivery acceptance with main health separate. The policy
changes and historical migration below still need their precise governing decisions
before implementation. No existing decision token is reinterpreted.

This follows, rather than rewrites, the verified
[delivery-binding design](../../streams/build-less-brittle/design-delivery-binding.md).
That record and build-less-brittle/14's Evidence remain unchanged. The binding design
solves attribution for consumer checks; it does **not** by itself stop acceptance from
moving to each new main tip. This proposal supplies that missing lifecycle decision.

Sources and experiment cutoff: public Assay at
`ae2f92ee89ef16f6c215616abcbe50575cbd4987`, observed 2026-10-26;
installed statusgen and desk-tools v1.0.63, source
`125611dfbdfb950cf09c003e941c48902803fb1f`. Open PR status is a dated observation,
not an implementation dependency assumed shipped. See [replay](replay.md) for limitations.

Freshness recheck: main advanced to `e25479a69d69ad678c15253309fcac6b7b0c86b0`
while this was prepared. PR 4571 now shares the trailer predicate between prospective
attribution and historical backfill, walking past mention-only candidates. Reuse that
fix; do not implement a second trailer parser. It does not freeze D or change coverage,
and does not supply a past reviewed snapshot for historical attribution. The quantitative
replay intentionally retains the earlier cutoff; it is not a claim about the later queue.

## Purpose and fixed subject

Verify answers: **did this independently identified delivery satisfy its approved
obligations, under the recorded execution conditions?** It produces independent
acceptance of a finite delivered change. It does not continually renew that delivery's
acceptance against all subsequent changes. Validation of intended use remains the
requirement owner's activity described in [validation](../../validation.md).

Keep these coordinates distinct, using the existing binding, protected-table and witness
records rather than a second manifest or acceptance database:

| Coordinate | Meaning | Existing owner / representation |
|---|---|---|
| Delivery D | Immutable mainline commit completing the reviewed delivery, and its exact first parent | Delivering-PR resolver; binding design's protected annotated tag |
| Criteria A | Original approved Verify table, plus any explicitly approved amendment and row selection | `Verification-Table`, `Acceptance-Transition`, `protectedAcceptance` |
| Witness W | The source actually executed, runner, conditions and observation | Existing execution witness; v2 work only where qualified reuse needs it |
| Recording E | Revision carrying the evidence/status update | Existing Evidence and outcome records, committed independently |
| Main H | Current integration revision whose health CI evaluates | Existing push-to-main CI and findings/issue flow |

D is chosen by delivery attribution, never because W passed. E and H may advance
without changing D or A. An exact D run is a straightforward conservative option:
unknown dependency scope need not be narrowed to make it finish. W from another revision
can count only when the existing coverage owner establishes applicability to D, with
criteria, source, build/policy and relevant environment preserved. Keep W's real identity.
Neither package-only non-overlap nor a `files:` declaration proves this.

Acceptance is not deployment or release approval. A subsequently discovered defect can
make a release unsafe even though its earlier acceptance was honestly recorded. If new
facts show that D never met A, record that finding and correct the acceptance claim
through the existing process; immutability does not protect a false claim from challenge.
A repair delivered later is a new delivery/version with its own acceptance, not an
opportunity to silently substitute later code for D.

## The two parts

**Context (the requested “meta” part).** Keep the existing Context/provenance surfaces.
Identify source and criteria, the binding source, supported consumers and dispositions,
implementation scope versus test inputs, required execution conditions, and unresolved
authority. Distinguish test validity from eligibility to record closure. A parser failure
or missing binding holds closure; it does not turn a witnessed functional pass into a
functional failure. A missing route or a wrong source can be substantive, even when all
unit tests pass.

**Tests.** Keep the existing Verify table, row identities, commands, Expect, class, shell
and original Evidence. They are the actual acceptance obligations, including flow,
mutation, consumer and required environment checks. Check them against D and A. Do not
turn a consumer obligation into bookkeeping just to clear it. Do not auto-credit an empty
diff as consumer coverage. Supplemental observations remain supplemental.

These are logical parts using the existing brief format, not a mandatory bulk heading
migration. Report a context hold and the observed test results separately within the
existing outcome/Evidence view. Overall acceptance stays held until both are adequate.
A reporting split grants no authority and clears no hold on its own.

## Review owns the context check at the ready flip

Brief design writes the intended tests and consumers. The PR-review desk rechecks the
actual implemented change immediately before its existing ready flip. Reuse that desk's
context/pickup assessment and the open prospective-delivery work in
[PR 4139](https://github.com/medici-finance/assay/pull/4139); it is not shipped at the cutoff.
There is no additional pre-verification desk or stage.

At the flip, review establishes that the approved criteria still express the intended
contract, the diff supplies the claimed consumers/scope, commands will be runnable after
merge, required tools/conditions are available or an explicit hold is known, and a valid
plan exists to bind the eventual merge. D does not exist yet: review approves the
prospective attribution and checks the symbolic binding contract, not a fabricated SHA.
Changes to the PR head or material controlling context require that existing assessment
to be revisited. Do not wait until Verify to discover a known missing delivery subject.

After merge, the existing independent CI writer proposed in the binding design resolves
and freezes actual D. It must refuse a mismatch with the reviewed candidate, partial
implementation, ambiguity or unavailable provenance. The verifier cannot choose a more
convenient delivery. This is a continuation of the reviewed delivery, not another review
stage. Consumer checks use D and its first parent through the existing immutable snapshot
checker; functional tests execute D in an isolated execution root with the recorded tools.
Current trusted tooling may control execution, but must not overlay newer product/test
source onto D. If an approved later test harness is needed, its provenance and authority
must be explicit; a missing test at D is not silently repaired with main's copy.

## Execution, reuse and closure

The existing Verify desk remains the independent acceptance owner. It runs the applicable
original obligations at fixed D, reviews primary output and records the verdict. Existing
execution/admission, identity, human-row and risk gates remain. This proposal does not
introduce automatic credit for an implementer's self-report or equate provenance with
functional correctness.

The pure semantic owner is `statusgen/evaluation.EvaluateCoverage`:
`CoverageInput.Revision` already accepts an explicit subject. The adapter
`statusgen/coverage.go` has `coverageOptions.Revision`, but production defaults it to HEAD.
Adapt its existing callers in coverage, auto-flip and reconciliation to the same admitted
per-brief D. The CLI, Verify runner, closure evaluator, evidence writer and outcome checks
must agree on D/A; changing a display or only one flip caller would reproduce the conflict.
The current contract in `spec/lifecycle-v1.md` explicitly evaluates later main. Replace
that rule for delivery acceptance; retain content applicability checks against D.

One further seam needs finishing: the evaluator currently chooses the last witness for a
row, and protected acceptance can reject the latest BLOCKED run before exposing any of
its witnesses. Select observations for the admitted **same D/A/conditions** through the
existing observation adapter before handing them to the pure evaluator. Do not let a
later H health failure replace a D acceptance witness. Within the same accepted context,
retain last-result precedence: do not search backward for a green result past a later
applicable failure. Missing subject information in a v1 historical record is a migration
hold, not permission to guess its context. No second verdict engine or ledger is needed.

For bookkeeping-only repairs (scope formatting, record placement, an otherwise correct
binding being recorded), preserve all original observations. Credit each unaffected row
only when its identity, criteria, effective source and conditions still apply to D. Rerun
only the obligations whose proof is missing or materially changed; use a conservative
fresh D run where safe reuse cannot be established. Never rewrite an old witness, its
output hash or its execution revision.

This last paragraph is **proposed policy**, not a description of today's complete path.
`docs/protected-paths.md` requires fresh witnesses for every original row after protected
amendment; `statusgen/acceptancetransition.go` also requires execution revisions descended
from the amendment. A fixed D can predate that amendment. Those requirements must be
explicitly revised: assess execution of approved amended criteria against D, retain
unchanged applicable rows, and demand fresh results for affected rows. Preserve exact
human approval of changed commands/criteria and provenance of execution after that
approval; a source commit alone is no longer a proxy for execution time. This proposal
does not broaden the edits allowed by the command-only transition: Expect, class, shell
and row order stay protected. A changed requirement outside that route needs separately
approved contract/delivery work, not a compatibility rewrite. Do not activate
reuse by bypassing today's audit, forging a descendant source SHA, or changing an Expect.

## Main health stays with CI and the existing repair flow

Existing `.github/workflows/ci.yml` already runs on pushes to main. It evaluates H,
including integration, build, regression and platform checks covered by its triggers.
Health failures route through existing CI ownership, findings/issues and repair briefs.
They can block a release under the existing release policy. They do not keep every
historical brief in `implemented` or invalidate a correct D result merely because H moved.

This is separation of claim and ownership, not a promise that current CI covers every
historical Verify row. Assess its actual suite/trigger coverage when adapting the owners;
reuse those suites for recurring checks that truly belong to H. Keep delivery-specific
judgment, one-time mutations and consumer attribution with D. Time-varying production or
external checks are dated observations under existing access policy, never permanent
claims that a historical service remains healthy. No new health daemon, queue or desk.

## Historical briefs and completed records

Do not mass-edit original Verify tables or reopen completed briefs. Migrate open work
as it reaches the existing review/Verify desk; finish shared compatibility in the named
owners so the same mismatch is not repaired by a human for every row. A one-time
read-only census may group candidates, but is not authority to close them.

For an already merged brief, the review desk owns the historical context assessment
before its existing review-to-Verify handoff. There is no retroactive ready flip. Recover
D from the original delivery boundary and reviewed diff/PR provenance; use existing
markers as coordinates to corroborate, not independent identity authority. The binder's
historical path must replay the resolver at each eligible first-parent boundary, in
chronological order, rather than select today's newest matching follow-up. It must account
for multi-PR/partial deliveries explicitly; “oldest match” alone is also wrong.

Today’s mutable PR body or approval state is not proof of what was reviewed at that past
boundary. Missing historical provenance, multiple plausible deliveries, a contradiction,
or unavailable complete history holds the migration for independently reviewed attribution
through the existing decision/review process. Do not manufacture a trusted binding from a
plausible SHA. Prospective freeze-before-follow-up protection is absent for backfills;
that difference is a real residual risk.

Classify historical work before applying compatibility:

| Historical shape | Proposed treatment | Still required |
|---|---|---|
| Correct D/A and applicable execution evidence; recording or scope syntax alone broken | Repair context/outcome recording; retain applicable row results | Existing review, admission and closure authority |
| Named consumer row lacking pins, but correct historical D can be independently bound | Bounded compatibility supplies D through the existing consumer evaluator | Fresh meaningful consumer observation; original criteria and unchecked judgments |
| Bare unscoped consumer row | Bind brief and D only under explicit historical policy | Recognize this changes effective subject despite identical command text; rerun the consumer obligation |
| Wrong explicit range, altered command/Expect/class, or disputed consumer claims | Existing exact protected amendment or delivery repair route | No automatic reinterpretation or “meta” waiver |
| W is later than D and full input equivalence cannot be established | Preserve W as an observation; execute missing proof on D | No retargeting W or narrowing to implementation `files:` |
| Older narrative-only or unreachable witnesses | Keep historical record and grandfathering under current cutoff rules | No invented hashes, execution or retroactive execution credit; fresh proof if closing open work |
| Already verified/done | Preserve the original acceptance statement and its historical limits | New defects use existing findings/repair flow; do not bulk reopen for new formatting |

For verified-but-not-done backlog entries, finish the existing continuation against
the admitted original D/A and applicable evidence. Their earlier verification remains
a historical statement; a current-H coverage refusal is not by itself evidence to
demote it. Missing D/proof still holds completion. Already-done records keep the existing
grandfathering cutoff and are not given invented retrospective attestation.

Statusgen/38's planned v1-to-v2 backfill can recover available content records; it cannot
establish delivery identity, invent lost output or approve a changed obligation. Reuse it
for its actual job after /36 and /37 are qualified. Fixed-D acceptance itself can proceed
conservatively with existing v1 witnesses from a fresh D execution; do not make the whole
migration wait for narrower content caching or assume that v2 is already shipped.

## Deterministic completion through existing work

The finish line is deterministic **selection, evidence eligibility, next action and
resumable effects** for the same admitted D/A, observations and recorded conditions.
Independent judgment remains explicit; a changing external condition is an input,
not permission to silently change the subject. Freezing D alone does not complete this
loop. Neither does merging the current open PRs without the assignments below.

The work map was checked on public main
`5d6ddab1247792e1dc4220cb779f08c24ffe9459` on 2026-10-27, against open/merged PRs,
brief Tasks, real implementation files and installed v1.0.63 source ancestry. A final
fetch at `0c292571a3e0794598cb11733d0b6c966c89aae0` adds Evidence and report-skill
changes, not these implementation slices. Board `todo` is not evidence that code is
absent. The earlier [152-brief replay](replay.md) retains its own older cutoff; no
current backlog count or new closure claim is inferred from this work map.

### Ownership assignments made in this PR

The two gaps identified by the finish-map audit are now assigned in existing briefs.
Their Task/scope versions and typed dependencies are amended here; implementation,
acceptance, custody choices and activation remain separate. The verified /14 design and
all original Verify/Evidence sections remain unchanged. No new brief ID is introduced.

| Existing owner | Assigned slice | Explicit prerequisites |
|---|---|---|
| statusgen/36 | I-2 binding admission/checker/lint; common D/A coverage, protected closure and witness-selection policy | Existing authority decisions; one semantic owner and existing v1/v2 guards |
| statusgen/37 | I-1 binding producer and historical attribution; I-3 staged writer/custody configuration; isolated D execution and truthful E recording; scope/outcome compatibility | /36; existing writer/custody/approval rulings before affected effects |
| statusgen/29 | I-4 authoring consumers; finish PR 4139 in the existing prospective preflight/deskflip/review owners | /27 and /36 |
| statusgen/31 | Corroborated historical metadata/disposition, not witness or binding fabrication | /28 and /36 |
| desk-tools/46 → /47 | Accepted D/A/run scheduling identity, then remaining authorized landing effects | /46 needs /45 and statusgen/37; /47 needs /46 |
| statusgen/38 | Bounded historical subject compatibility and recoverable witness backfill | /37; per-case metadata/provenance remains a real hold |
| desk-tools/51 | Released review/Verify context-test/action contract and consumer version qualification | Existing /45–50 and /52, plus statusgen/29 and /38 |
| verification-handoff/02 → /04 → /05 | Retain blocked artifacts/effects, claim repair, return exact original or successor subject to independent verification | /02 additionally needs desk-tools/47; /05 additionally needs desk-tools/46; other existing edges retained |
| verification-handoff/06 | Complete fixed-delivery/repair/health production-adapter qualification and installed-version proof | /05, desk-tools/51, statusgen/38 and desktools-v2/73 |
| statusgen/35 | Execute/report the actual active historical cohort using those existing operations and owned residuals | Existing /27–32 plus /38, desk-tools/47 and handoff/05 |

All new edges have reciprocal `unblocks:` entries. Affected wave increases propagate to
handoff/04 and statusgen/33. The generator refreshes only authored wave/effort cells;
lifecycle cells and acceptance stamps stay unchanged. Historical corpus
execution (/35) and synthetic end-to-end qualification (handoff/06) are parallel consumers,
so product acceptance does not wait for every historical failure or notice to disappear.
The authoring amendment is not a claim that these capabilities have shipped. The open
PRs for affected briefs must be assessed against the amended scope at pickup/review;
this does not silently revoke unrelated work already approved under its original contract.

### Remaining implementation and integration

| Existing brief or PR | Observed position | Exact finish owed |
|---|---|---|
| [PR 4579](https://github.com/medici-finance/assay/pull/4579), this proposal | Open, reviewed; design only | Resolve fixed-subject, historical-subject and row-reuse policy on the existing decision surfaces; preserve old decision-token meanings. Publication is not activation |
| build-less-brittle/14 design I-1–I-4; [4380](https://github.com/medici-finance/assay/issues/4380)/[4381](https://github.com/medici-finance/assay/issues/4381) | Design delivered; I-1/I-3 assigned to /37, I-2 to /36 and I-4 to /29 here | Trusted delivery attribution and ref admission, offline graph/marker checks, existing consumer checker/lint integration, writer protection and guidance. Include historical attribution and D/A correction/version semantics. A locally forged tag is not authenticated by a remote ruleset |
| [PR 4139](https://github.com/medici-finance/assay/pull/4139) | Open, changes requested | Owned by amended statusgen/29; finish prospective ready-flip context check; resolve producer binding for the preflight signal, trusted-base ruling resolution, duplicate classification and outstanding qualification/adopter findings. It assesses the prospective implemented transition, not full Verify closure |
| statusgen/36, [PR 4409](https://github.com/medici-finance/assay/pull/4409) | Open, changes requested | Finish the shared reader/digest owner and existing old-reader qualification/review. The unresolved qualification choice is tracked on issue 4459. Amended /36 owns binding admission and common D/A acceptance; preserve comparison behavior for the same explicit v1 target |
| statusgen/37 → /38 | Authored; no implementation PRs found | Finish the writers/desk readers, then bounded historical witness backfill. Make admission, criteria, actual execution W, recording E and accepted D agree through existing seams; never manufacture missing historical output or identity |
| desk-tools/46 → /47 | Authored; no implementation PRs found; /46 depends on /45 | Recognize an independently accepted unchanged PASS as continuation, then resume only remaining sanctioned Evidence/outcome/status effects. Preserve applicable rows, same-context failure precedence, partial writes and authority exclusions |
| desk-tools/48 and /52 → /51 | Authored; no implementation PRs found | Finish existing capacity reporting and distinct decision/verification-card authority, then drive verifier guidance/prompts from the implemented lane-to-action contract. /51 also depends on /45, /46, /47, /49 and /50. Do not silently drop these declared dependencies or invent a new limiter |
| verification-handoff/02 → /04 → /05 → /06 | Authored; no implementation PRs found | Retain blocker/run/artifacts; discover and claim the existing successor repair; return repaired work to independent acceptance; qualify retries, partial writes and real module boundaries. /02 also depends on /01 and desk-tools/49; /04 also depends on /03. Reconcile /06 guidance with /51 rather than write a second verifier contract |
| Contracts, lifecycle, template and shipped references | Current lifecycle still defines later-main acceptance | Align S-evidence-coverage and the protected acceptance owner with D/A; keep H health separate. Release compatible tools, pin consumers and qualify the installed loop under /51 and handoff/06. A source merge is not a deployed behavior |

### Delivered pieces to accept and integrate, not rebuild

All five source implementations below are ancestors of installed v1.0.63 source
`125611dfbdfb950cf09c003e941c48902803fb1f`, despite their observed board rows
remaining `todo`. Source presence does not establish independent acceptance or live
adapter activation. Complete the existing verification/sign-off/reconciliation owed;
do not dispatch duplicate implementation from the stale board label.

| Existing brief | Merged delivery / evidence | Remaining boundary |
|---|---|---|
| desk-tools/45, canonical queue | [3376](https://github.com/medici-finance/assay/pull/3376) | Complete independent acceptance and consistent consumer integration; /46 and /51 consume this owner |
| desk-tools/49, canonical filing identity | [4229](https://github.com/medici-finance/assay/pull/4229) | Complete acceptance; handoff/02 must consume its lookup rather than implement another dedupe rule |
| desk-tools/50, environment admission | [4392](https://github.com/medici-finance/assay/pull/4392) | Complete acceptance and actual prompt/runner integration; retain issue 4393's authority boundary |
| verification-handoff/01, shared contract | [4225](https://github.com/medici-finance/assay/pull/4225); independent PASS [4255](https://github.com/medici-finance/assay/pull/4255); design record [4412](https://github.com/medici-finance/assay/pull/4412) | Reconcile existing credited Evidence and remaining human/lifecycle acceptance; production dependents must use the shared assessment |
| verification-handoff/03, accountable disposition | [4413](https://github.com/medici-finance/assay/pull/4413) | Independently verify and complete its existing acceptance; connect worker discovery through /04 |

Likewise statusgen/28's notice separation is delivered in
[4140](https://github.com/medici-finance/assay/pull/4140), and /32's reconciliation
routing in [4137](https://github.com/medici-finance/assay/pull/4137). Neither is a new
implementation prerequisite to author again. The binding resolver's shared trailer fix
in PR 4571 is also reused, not duplicated.

### Historical backlog and existing defect work

| Existing owner / work | Finish and limit |
|---|---|
| statusgen/38 | Recover only available, attributable content witnesses after /36–37. Delivery attribution and exact amendment authority are separate. Unreachable or narrative-only evidence cannot acquire invented execution credit |
| statusgen/31 and /35 | Own derivable active metadata repair and active backlog cleanup; preserve closed records and all original Evidence. /35 currently depends on /27–32, including unfinished /27, /29, /30 and /31. Keep that broader debt plan distinct from the minimal D acceptance cutover; it is not permission to add another repair mechanism or to make every delivery wait for all notice debt |
| Shared declared-scope reader: [3697](https://github.com/medici-finance/assay/issues/3697), [3708](https://github.com/medici-finance/assay/issues/3708) | Finish wrapped/prose and basename handling in the existing owner and outcome-writing path. PR 3703 was a partial corpus cleanup; it did not clear the class |
| [PR 4371](https://github.com/medici-finance/assay/pull/4371) | Finish relative-brief resolution, containment review finding and merge conflict. Do not claim that this alone fixes every CI/root spelling |
| [PR 4150](https://github.com/medici-finance/assay/pull/4150), [PR 4149](https://github.com/medici-finance/assay/pull/4149) | Finish evidence-bound historical diagnostics and the specific statusgen/02 table repair. At the work-map read, PR 4150 is approved; PR 4149 still has a changelog-format failure |
| desktools-v2/73, [PR 4102](https://github.com/medici-finance/assay/pull/4102) | Finish agreement between rendered Evidence and admission/lint interpretation; review still identifies a divergence. Keep one acceptance meaning across these readers |
| [4596](https://github.com/medici-finance/assay/issues/4596) | Finish authoritative lint's verifier binding and relative-root resolution. Earlier acceptance-forge activation already merged in PR 3515; issue 3493's old publication-blocked comment is not evidence that all wiring is still absent |
| [4340](https://github.com/medici-finance/assay/issues/4340), [4451](https://github.com/medici-finance/assay/issues/4451), [4598](https://github.com/medici-finance/assay/issues/4598) | Resolve the reported same-evidence disagreements over latest witnesses, Runner eligibility and BLOCKED verdicts against shared fixtures and existing owners. An open issue alone does not prove every historical report still reproduces; verify the residual before changing code |
| [4554](https://github.com/medici-finance/assay/issues/4554) | Make the existing outcome-record dry run validate/plan without committing. This is an effect-boundary defect within the existing landing tool |
| [4486](https://github.com/medici-finance/assay/issues/4486) | Fix Evidence merge authentication/merge retries so existing main CI actually triggers. A declared push workflow is insufficient when the merge token suppresses its run; no new main-health checker is proposed |

Original incident PRs 4559, 4561 and 4562 are already merged. Historical recovery stays
with the admitted original delivery and existing obligations on
[4560](https://github.com/medici-finance/assay/issues/4560) and
[4563](https://github.com/medici-finance/assay/issues/4563), not another push to those
closed branches. Preserve core observations; repair context and obtain missing proof
on D. desk-supervision/13 remains a substantive hold with two disproved consumer routes.
The 35 verified entries in the old replay still need valid continuation; they are not
35 new implementation jobs and are not automatically done.

The existing build-less-brittle/15 recorded-read-set plan, ruled on issue 4512 and
authored in PR 4510, is not silently cancelled or added to this critical path. Fixed-D
acceptance can use conservative fresh execution at D without a tracer. Its proposed
single-main historical re-witness batch must be assessed against the newly selected
fixed-delivery policy before being treated as this migration. Likewise, the broader
shared-launcher adoption in graph-execution/26–27 is not a prerequisite for consistent
verdicts and resumable effects in the existing Verify loop.

### Finish order and qualification ownership

The dependency order is: settle the precise policy under the ownership assigned above;
finish binding and common D/A admission; finish accepted-PASS continuation and resumable
landing; migrate recoverable historical proof and route genuine residual defects;
then qualify, release, pin and exercise the actual running loop. Independent strands
above can progress without pretending that an authoring merge is implementation.

Use **verification-handoff/06's existing production-adapter replay** together with
**desk-tools/51's existing lane-to-action/version qualification** for the complete
path. Their planning scope is amended here to cover D/A; do not create another
qualification stage or register. They must show the same admitted inputs yield the same
verdict/next act; unrelated H/bookkeeping movement preserves valid D proof; affected
criteria/inputs/conditions hold the affected rows; newer applicable failures are not
hidden by old passes; crashes and duplicate events resume one remaining authorized
effect; and H regressions remain visible in existing CI. The current backlog replay
qualifies diagnosis and evaluator seams, not this unimplemented end-to-end behavior.

No new verification mechanism, store, service, witness schema beyond existing /36–38,
or review/Verify stage is proposed. This planning amendment updates existing brief scope and dependencies; it does not
dispatch work, implement runtime, approve acceptance, change a human gate, or activate a
dry-run sink. Existing criteria and historical Evidence remain intact.

## Authority and decisions before implementation

The requested direction is captured here. Precise policy is not smuggled into existing
answers. The following are **proposal-local labels**, not live forge reply tokens:

- **FDV-SUBJECT:** replace the moving-HEAD delivery-acceptance rule with admitted D/A;
  keep H health with CI. Recommended: this proposal. Alternative: retain later-main
  acceptance and accept repeated conservative invalidation. This changes
  `spec/lifecycle-v1.md` and S-evidence-coverage, not just documentation headings.
- **FDV-HISTORY:** authorize bounded historical subject binding/compatibility and assign
  its context check to review. Recommended: the class treatment above, with explicit
  original/changed-subject distinctions. Alternative: keep all historical missing-subject
  rows on per-brief exact amendment. This requires a newly stated question on existing
  issue 4380, with a unique token, before any ruling is consumed.
- **FDV-REUSE:** replace full-table post-amendment fresh-witness/descendant-source rules
  in protected-paths with row applicability to D and approved A; preserve unchanged
  valid results and rerun affected obligations. Recommended: this bounded change.
  Alternative: keep full-table fresh execution after every protected amendment; that
  deliberately retains extra historical work. Identity, exact amendment approval,
  conditions and all non-affected closure gates remain required under either option.

Existing `D1:` and `D3:` replies on issue 4380 retain **only their original questions**.
In particular `D3: a` on comment 6398480713 means existing rows are unchanged and authorizes
no historical compatibility or witness-reuse relaxation. Do not silently supersede that
question or D1(a)'s old merged-row wording. Issue 4381 still owns the writer/custody choice;
this proposal does not grant a permission or choose a different authority behind it.

Before affected implementation starts, put the changed questions together on the existing decision
surfaces with unique reply tokens and exact policy deltas. No answer is inferred from an
old token, a bot relay, a draft proposal, or elapsed time. The design publication itself
is not activation. Do not add a second decision register or a new blanket human gate.

## Falsification, residual risk and implementation handoff

The [backlog replay](replay.md) separates diagnostics from acceptance. Its strongest
negative control is desk-supervision/13: fixing context still exposes two disproved
routes. Its main warning is that choosing W gives superficially green results for empty
consumer checks. No historical brief was closed by the experiment.

Before rollout, the existing implementation work must demonstrate the complete path:
independently bind D, execute approved A on D, record at later E, advance H through unrelated
changes, and still complete acceptance of D. A relevant D/A/condition change must hold only
the affected proof; a later applicable failure within the same context must supersede a
pass. Missing/forged/ambiguous binding, wrong parent, changed criteria, missing input or
unknown environment must hold. A real H regression must stay visible and block the
appropriate integration/release decision without replacing D's evidence. These are
acceptance examples for the existing owners, not a new standing verification gate.

The experiment qualifies the evaluator seam and concrete backlog diagnosis; it does not
prove this end-to-end path is already implemented. Historical attribution remains a
single semantic failure point: review of the reconstructed original delivery stands
behind the replay, while graph checks and writer protection only reject structurally
invalid or unauthorized records. They cannot detect a plausible but wrong reviewed D.
The verified binding design's residual must therefore remain explicit, especially for
backfills; it cannot be described as independently caught by the same resolver twice.

Pickup disposition: **adapt existing unfinished work; hold runtime implementation**.
Pin this proposal, the controlling lifecycle/closure contracts and the then-current
source/tool revisions at pickup. Recheck the dated work map, including the open PRs,
/36–38, desk-tools/45–52, handoff/01–06, attribution/custody decisions and actual
deployed callers before changing them. Existing execution briefs are amended for review
in this PR; none is implemented or dispatched by this planning change. Preserve the prior verified design and original Evidence.
