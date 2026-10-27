# Fixed-delivery verification — proposed redesign

Status: **draft; design and retrospective experiments only**. No runtime, lifecycle,
acceptance, brief, Evidence, permission, or activation change is made by this proposal.
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

Statusgen/38's planned v1-to-v2 backfill can recover available content records; it cannot
establish delivery identity, invent lost output or approve a changed obligation. Reuse it
for its actual job after /36 and /37 are qualified. Fixed-D acceptance itself can proceed
conservatively with existing v1 witnesses from a fresh D execution; do not make the whole
migration wait for narrower content caching or assume that v2 is already shipped.

## Existing work to finish, and work to adapt

| Existing owner/work | Required delta / remaining gap |
|---|---|
| build-less-brittle/14 delivery-binding design; pending issues 4380/4381 | Finish independent writer, trusted ref admission, checker/lint and authoring consumers; add explicitly reviewed historical attribution and D/A version/correction semantics before activation. A locally forged annotated tag is not authenticated by a remote ruleset. Missing/untrusted/conflicting binding holds |
| PR 4139 prospective delivery at `deskflip` | Finish its outstanding review/CI and adopter applicability work; make the review-owned context check use it. It checks the prospective implemented transition, not full Verify closure |
| `statusgen/evaluation` and `coverage.go` | Keep one rule owner; admit D per brief, supply appropriate A and same-context observations; retire HEAD as delivery-acceptance default in lifecycle callers, not in health callers |
| `verifyrun`, protected closure/transition audit, `deskevidence`, Verify-loop admission/outcome writes | Execute D with independently sourced A and write at E; retain row observations on context holds; support approved row reuse without lying about W. Today these paths are not a finished fixed-delivery pipeline |
| statusgen/36 → /37 → /38 | Reuse planned reader/writer/backfill for qualified cross-revision reuse and missing-object recovery; adapt their “item revision” to D for acceptance. Do not replace them with another digest scheme or rush their open environment/scope ruling |
| Shared declared-scope grammar; issue 3697 and partial sweep PR 3703 | Finish historical wrapped/prose scope handling and outcome-writer compatibility. The sweep left known refusals; it was not a complete deployed repair |
| Existing main CI, findings and repair briefs | State H in health output and keep failures out of historical acceptance selection; inspect actual trigger/suite coverage, without claiming exhaustive health |
| Contracts, lifecycle spec, brief template and shipped desk references | Align the single source of method with D acceptance, review ownership and historical transition; release/pin to consumers and qualify what actually runs |

No new verification mechanism, store, service, witness schema beyond existing /36–38,
or review/Verify stage is proposed. The eliminated burden is recurrent acceptance against
moving H and repeated functional reruns caused only by context repairs. Remaining burden
is honest delivery attribution, one adequate D execution and independent acceptance.

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

Before work is decomposed, put the changed questions together on the existing decision
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
source/tool revisions at pickup. Recheck open PR 4139, /36–38, attribution/custody decisions
and actual deployed callers before changing them. No execution briefs are authored or
dispatched by this design change. Preserve the prior verified design and original Evidence.
