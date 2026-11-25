# Verification during PR review

**Status:** draft — proposed workflow for independently verifying a merge candidate and recording the result when it lands.

Assay currently asks an independent verifier to start after merge. By then the
review context can be cold and unrelated changes on main can make the witness
harder to use. Move eligible verification into the PR workflow. Preserve
independent review and execution verdicts, with one handoff on the merge event.

## Outcome and scope

For the first pilot, a trusted same-repository PR implementing one low-risk
`gate: model` brief can arrive at `verified` when its merged tree and approved
Verify table match the candidate that an independent verifier already passed.
Processing the merge should require no second execution. The normal verify-desk
queue handles work when no qualifying witness exists.

A pilot brief must have a complete table of deterministic, offline,
content-based checks. Rows that require moving Git refs, the identity or history
of the actual merge commit, a live service, or operator-only credentials follow
the existing post-merge route. Exclude human-gated and cross-repository briefs,
forks, multiple deliveries per brief, and briefs with additional mandatory
workflow-pattern claims from the first pilot. The pilot keeps human merge
authority, current branch requirements, and the existing authority for `done`.

## Starting point

Inspected public Assay main at `9c284587ff373eda0e606739b863474e6ccd3bff` on
2026-11-25. The behavior below is source-observed.

- [Pre-merge verifier dispatch](../desk-tools/deskdispatch.md) and [issue #7623](https://github.com/medici-finance/assay/issues/7623) already distinguish an exact unmerged head from merged main. The existing `--target-head` kit checks out the PR head alone. This path is disabled pending credential-free containment. Its approved-head checks include a same-repository trusted author, exact-head reviewer-App approval and a separate security pass. It currently forbids credit and Evidence landing from a pre-merge run.
- [Lifecycle §2.4](../../spec/lifecycle-v1.md#24-verified) requires independent execution on merged main. Credit at merge needs an explicit amendment to that contract and the protected landing path. This draft records the proposal.
- Assay already defines a [candidate and acceptance binding](../../spec/verification-domain-v1.md) and [independent evidence provenance](../../spec/evidence-provenance-v1.md). The [candidate-workspace brief](../streams/graph-execution/brief-23-candidate-workspaces.md) owns the reusable workspace design. Reuse its identity and validation contract at the Git boundary. Full workflow-runtime adoption is outside this pilot.
- The [fixed-delivery verification proposal](fixed-delivery-verification/README.md) deals with the subject of acceptance after delivery. This draft deals with when eligible evidence can be collected and how it can be credited at merge. A result accepted at merge may still hit the separate current-main coverage requirement on the existing `done` path.
- The [merge-time recheck](../../plugins/assay/skills/pr-review-desk/references/merge-time-recheck.md) records failures where the PR and base merged cleanly but the combined result was wrong. Testing the combined tree is the substantive integration check.

## Workflow and ownership

| Step | Owner | Result |
|---|---|---|
| Review the exact PR head | Review desk and its review agents | The existing correctness verdict and separate security verdict refer to the current head. The review desk confirms the Verify table describes the actual change. |
| Request independent verification | Review desk requests through existing desk dispatch authority | An eligible candidate gets one verifier assignment per candidate and acceptance generation. The review desk tracks the run and requests a fresh attempt for a superseding candidate. The verifier owns the execution judgment. |
| Freeze and run the candidate | Dispatch control plane and independent verifier | Freeze the reviewed head combined with the current target base. Provide an immutable execution tree and an approved Verify definition to a contained runner. Collect the verifier's separately admitted witness. |
| Merge | Human driver through the existing merge path | A trusted forge observation identifies the merged PR, reviewed head, target ref and actual landed commit. A closed but unmerged PR provides no landing fact. |
| Credit and publish | Existing verification and Evidence publication owners | Validate the witness against what landed. Add the witness and the `verified` transition in one scoped publication. A failed, incomplete or unbound result stays in the normal verification or repair flow. |

The pilot adds the combined head-and-base freeze described above. For the first
pilot, request verification after both exact-head verdicts. This
keeps the existing admission order while moving the work ahead of merge.
Preparation can overlap review. Scheduling execution before both verdicts would
change the admission rule and is outside the pilot.

The coordinator's dispatch credentials remain in the control plane. The
reviewer, implementer and verifier keep separate accountable duties. Apply
independence to the responsible and delegating principals recorded by the
existing provenance contract. The review desk's request can sponsor the run.
It cannot author the verdict or stand in for the verifier.

## Execution and credit contract

Use the existing verifier run, outcome and protected acceptance representations
and their trusted issuers. Add candidate linkage to those records only where an
approved contract can carry it. Keep one evidence ledger and one lifecycle writer
for this transition.

The verifier's admitted result binds the repository, target ref, PR number, exact
reviewed head, base commit and tree, candidate commit and tree, qualified brief,
and the approved acceptance version and digest. The acceptance digest covers
the required row set, commands, expected results, and execution requirements
through the existing owner. The witness records original execution revision,
runner and toolchain profile, run and attempt identity, each row's verdict and
exit status, and its output digest. The control plane checks the protected Verify
definition before execution and again at credit.

Rows run in a credential-free, contained environment. Supply the candidate source,
approved tools and staged read-only dependencies. Keep forge credentials,
signers, host home, agent and Docker sockets, and other host state outside the
runner. Disable external network access. An isolated subagent session or an
environment-variable scrub alone falls short of the execution boundary.
The trusted runner/control plane collects the witness outside the candidate's
authority. PR-produced artifacts and chat summaries can inform diagnosis.
[Current row reach and isolation limits](../verify-row-strength.md#what-an-admitted-row-can-reach)

At merge, the evidence owner verifies the following facts.

1. The merge observation is authoritative, its repository, PR and target ref
   match the admission record, and the actual landed commit is in the target's
   history. The admitted reviewed head and required verdicts match the
   authoritative merge subject. The latest applicable result governs.
2. The original witness is complete, independently attributable, and bound to
   the admitted candidate and approved definition. Any changed acceptance
   definition, revoked approval, or incompatible evaluator or execution policy
   holds credit.
3. For the first pilot, the admitted base matches the landed integration
   parent and the entire candidate Git tree equals the landed tree. A
   forge-generated commit with a different commit ID can qualify for the
   pilot's content-based rows when those checks pass. Treat a changed base or
   tree as a mismatch even where narrower future applicability may be possible.
4. Every mandatory row passes in an admitted environment. The publication
   appends the original witness and separately records the landed commit and
   why it qualifies. The run keeps the revision it actually executed, and the
   lifecycle transition records the time of accepted publication.

Credit against the actual landed commit before assessing publication. A later
unrelated commit arriving while the evidence writer runs should only trigger a
row-scoped publication retry. Re-read the merge facts, protected acceptance and
current brief row on retry. The writer can retry a merge-qualified result across
unrelated main updates. A changed or superseded acceptance or an already
completed transition governs the retry. Concurrent or duplicate events can
publish at most one transition for the same landed PR, approved definition and
witness. A missing receipt remains visible as pending or routed to the ordinary
verify queue. Use the existing
[row-scoped Evidence publication boundary](../../tools/desk/cmd/deskevidence/rowscope.go).

If a newer head or applicable decision arrives before merge, supersede the old
request and retain its witness for audit. Restart and event replay use the
existing dispatch claim and accepted publication identity to recover pending
work. Reviewers and operators can see which of the two verdicts is missing
and why a candidate is held.

An initial candidate can come from the repository's existing integration
mechanism if the dispatcher can bind its exact head and base. Use an
authoritatively generated candidate where the forge supplies one. The pilot
can operate in shadow using a deterministic local merge candidate and actual
post-merge comparison. A required merge queue or a new merge permission is a
separate rollout decision.

A candidate failure appears in the PR's verification record with the failed
assertion and applicable source. The worker can address an in-scope failure in
the PR. The existing issue and Verify routes carry environment holds and wider
failures. An issue can track an explicitly deferred gap outside the accepted
claim. An issue alone earns no credit for a failed mandatory row.

## Delivery milestones and acceptance

| Proposed milestone | Independent acceptance |
|---|---|
| **Contained run under review** | On an opted-in fixture PR with both exact-head approvals, one independent verifier runs the complete offline table and returns a bound record. An actor mismatch, moved head, missing security pass, fork, forged receipt, inaccessible result or credential/egress probe gets no admitted PASS. Test an otherwise clean textual merge that introduces a semantic failure. |
| **Report-only merge comparison** | For one eligible model-gated brief, compare candidate evidence with a fresh independent run on the actual merge. Record candidate and landed identity and the outcome for each required row. Replays cover matching trees, same-tree/different-commit, changed base or tree, changed command or Expect, a wrong brief/PR, and a planted mandatory failure. Include an earlier PASS followed by an admissible FAIL or withdrawn approval, replayed with out-of-order notifications. No false credits in the exercise. |
| **Opt-in credit at merge** | The matching case records one `verified` transition through existing publication without repeating the Verify run. Mismatching, failed, withdrawn, partial and out-of-scope cases stay on their existing paths. Duplicate events, restart and a competing main update preserve scope and exactly one credited transition. Turning off opt-in restores the ordinary verification route while preserving already recorded evidence. |

Measure PR ready-to-merge time, merge-to-verified time, total Verify executions
per brief, and hold reasons in the pilot. Compare against the ordinary
post-merge path. Faster closeout must account for any shift in waiting time to
the PR. One good pilot establishes the handoff, with broader throughput and
false-credit exposure still to measure. Use existing
[brief flow instrumentation](../../spec/brief-flow-event-v1.md#15-wait-receipts)
where available.

## Decisions and handoff

The driver must approve the lifecycle and protected-landing amendment and the
first opted-in repository and brief before write-enabled adoption. The pilot
assumes report-only comparison before credit, the current dual review
precondition, and full candidate-tree equality at the landed integration
parent. Run and publication owners must qualify containment and trusted
receipt selection before widening eligibility.

Route dispatch, containment, candidate freeze, evidence admission and closure
through their existing owners. Review changes against [Go engineering
guidance](../go-engineering.md), [Assay contracts](../contracts.md) and
the protected acceptance and provenance rules cited above. At implementation
pickup, refresh the public main revision and the delivery state of #7623 and
the candidate-workspace brief `graph-execution/23`. Record which existing seams are available and which
still need an adapter. The approved spec can then supply self-contained briefs
and acceptance tests through the normal authoring process.
