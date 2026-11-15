# Historical consumer declaration correction for statusgen/36

**Status:** draft
**Issue:** #5684
**Routes-to:** docs/streams/statusgen/

This proposal is a design decision for the existing consumer-delivery owner. It
activates no checker behavior, grants no acceptance credit and changes no original
brief, Evidence, status or `fixed-here` meaning. It supplements [the correction
note](correction.md); it is not an approval of that protected Verify amendment.

## Evidence and inherited contract

Pickup: `399f67a7ea4e0ac0c694f5aecf95974968bb18cb`, 2026-11-15. Inspect
`docs/contracts.md` (S-evidence-coverage), `docs/consumer-delivery-checks.md`,
`docs/protected-paths.md`, `docs/go-engineering.md`, `statusgen/consumers.go`,
`statusgen/coverage.go`, `statusgen/autoflip.go`, `statusgen/reconcilefold.go`,
and the actual delivered versions below before implementing. Disposition: hold
consumer authority changes pending this design ruling; continue unrelated fixes.
The current owner has no historical-correction input or authentication path.
Existing forge readers, role/human mapping and immutable-object ports are owners
to reuse, not evidence that the proposed integration already exists.

The original producer is `3b2235af1af6dd42118cbf975eb4ac2b8e8ab409`; its exact
first parent is `c288fd881e7fe5532614ad25a4610be7c6acf8a3`. Brief blob
`051ddc7b265a8d29681c8552b9721019b8a2afcd`, line 36, declares:

```text
statusgen/autoflip.go and statusgen/reconcilefold.go (two of the three evaluateCoverage call sites, none of which passes Dependencies today; the third, coverage.go, is routed on the line above): fixed-here
```

Both named paths are unchanged across those pins:

| Caller | Identical base/delivery blob | Delivered call line |
|---|---|---|
| `statusgen/autoflip.go` | `5b7e7576d8f1ebfa0adf62465eb4ece8d7438fb3` | 1094 |
| `statusgen/reconcilefold.go` | `580cf396ff70323b26071d02aada41bf5bf01ca5` | 138 |

Both call `evaluateCoverage(root, streams, coverageOptions{})`. The adapter changes
from blob `4a1551ead0f2011644dd0a5e7d2422e8294b5530` to
`91e104e4af4456a922f22b6b029fe912854f06de`; shared library coverage changes from
`57e3ce0e145c0965b658f113eb9f681e9073eae0` to
`fbb360a08549aae6e623c75fc649610271d745bc`; witness reader blob
`c6a48ff3334d96587134a792122441dc406b8a4e` is newly delivered.
These are static facts, not behavioral proof.

At delivery and pickup, `statusgen/consumers.go:34` defines `fixed-here` as the
named path occurring in the diff. The delivered rejection is line 802; pickup
line 819. The immutable report is 7 corroborated, 1 disproved, 1 unchecked,
exit 1. It resolves the first site in the combined prose entry; manual blob
comparison establishes the second site is unchanged too. A correction must cover
both, rather than exploit that parsing limitation.

`fixed-here` cannot truthfully express unchanged callers. `follow-up` requires a
real deferred brief and back-reference and proves no historical adoption.
`out-of-scope` is an unchecked judgment and discards the promised adoption proof.
Editing current declarations, widening the base or using a later producer changes
the subject. No existing kind or old acceptance approval supplies this authority.
The recurring defect is a false historical declaration combined with an owner
that can only read immutable original declarations; it is not a path-lookup bug.

## Decision to be ruled

Recommended: permit a **separately authenticated historical correction** for this
exact declaration, only after both original callers have independent behavioral
proof against the original producer. Preserve and report the false original
claim. Mark it superseded by the named correction, never corroborate it as
`fixed-here`. With no admissible correction it remains disproved, as today.

Alternative: decline retrospective correction. Keep the immutable result red
and the original consumer acceptance obligation open; a later implementation
cannot make those original paths changed. This entails no checker changes.
Neither option accepts exit 1, relaxes the 1.10 performance limit or retires tests.

A design approval scopes continuation of the existing `statusgen/36` repair
under issue #5684, subject to ordinary native worker admission and review. It
authorizes no unnamed future brief. If the work is routed to a new brief instead,
that brief must meet its own admission and human-binding requirements; this
36-only ruling cannot be advertised as that authority. It grants no automatic
activation, acceptance-table approval, merge, consumer proof PASS or lifecycle
advancement. The driver decides; worker/reviewer comments cannot provide it.

## Proposed decision record and one ruling

Stable proposed ID: `DR-statusgen36-historical-consumer-correction`, applying to
this correction of `statusgen/36`. The pending record below remains inside this
draft. It is not installed in `docs/streams/decisions/` and is not an approved
design. Preserve original approved `DR-statusgen-36` and the original brief's
`design:`/ruling unchanged. A draft cannot veto that approved work.

```yaml
id: DR-statusgen36-historical-consumer-correction
date: "2026-11-15" # proposal date; a landed record transcribes the ruling date
title: "Prove both original consumers through an authenticated historical correction"
consequence: critical
decided-by: "human:<name>" # pending public placeholder, not a ruling
alternatives:
  - "Decline retrospective correction and retain the unresolved original consumer obligation; this remains the alternative offered to the driver."
  - "Reinterpret unchanged fixed-here paths as corroborated; rejected by the proposal because it drops the changed-path contract and both behavioral obligations."
accepted:
  - "Keep the false original claim visible; only an exact newly approved Command and independent original-source proofs may supersede its unresolved result."
  - "Reuse the existing exact protected acceptance approval and its opt-in semantics; no duplicate future human correction-artifact comment."
  - "Add bounded descriptor/proof validation in the existing consumer owner, with no activation before design ruling, implementation, review and ordinary acceptance approval."
```

`consequence: critical` is the proposal's classification: a wrong correction
could accept historical consumer adoption that never occurred. No approver or
ruling URL is invented. After the single source-bound human ruling, the normal
record-landing change may transcribe it under this stable ID, with the public
human placeholder and the actual ruling URL, following
`spec/registers-v1.md` sections 7.2–7.5 and `spec/lifecycle-v1.md` section 4.4.

For that one comment to corroborate later, a distinct genuine design-decision
issue must be created through native filing with the marker below in its body
from the outset. Link it from #5684 and bind this exact spec revision there. The
original bug body and acceptance marker stay intact; a comment alone cannot
substitute for the required decision-issue body marker:

```html
<!-- decision-gate: DR-statusgen36-historical-consumer-correction -->
```

The worker packet must read back that marked design issue before presenting the
one human act on it. The future DR record's ruling URL names that same issue.
The comment must name this DR ID, `statusgen/36` and the exact reviewed spec
commit/path/digest together. It approves or declines this bounded design only;
it supplies no implementation activation, consumer proof PASS or future Verify
approval. No second generic design comment should be requested later merely to
add a missing DR ID or brief binding. Record landing must not claim the pending
block above is already approved or replace the original approved design.

## Proposed bounded input and authority

The future checker must require an explicit immutable correction descriptor
selected by full commit and repository-relative path, alongside the original
`--delivery`, `--base` and `--brief`. It must not discover an override from a
mutable working tree or ambient configuration. Human approval corroboration
must use the existing protected acceptance owner
and its exact `--acceptance-forge` opt-in and permission semantics. Generic
`--forge` does not grant approval authority. Any separate verifier-attestation
read must also require the existing explicit `--forge` opt-in, through its native
forge seam and ordinary permission envelope. If either required opt-in/read is
unavailable, the correction is could-not-check and cannot clear a disproved
result. A descriptor alone grants neither authority nor permission to contact
the forge.

Ordinary consumers runs and all unrelated declarations retain current behavior.
No code or proof command is executed by the checker.

The descriptor must bind, without wildcards or version ranges:

- repository identity, brief ID, full producer and first-parent IDs, delivered
  brief path/blob, and exact raw declaration bytes plus their SHA-256;
- both caller paths/blobs and the changed adapter/library blobs above;
- exact design-spec revision, immutable external harness and fixture digests,
  and both independent proof results;
- the brief's existing marked acceptance issue, the only issue on which the
  independent verifier attestation may be read. The attestation must uniquely
  match the canonical descriptor digest and trusted verifier role; report its
  exact URL. It is not a future URL embedded inside the descriptor it attests.

The full correction commit/path/blob and digest bind the descriptor externally
through the approved Command. Human instance authority comes from the existing
protected transition below; the descriptor contains no self-hash, future
attestation URL or second human approval field.

The verifier payload must bind the complete canonical descriptor digest. The
human instance approval binds it through the exact approved row-23 Command: that
Command must be one direct invocation in a documented canonical argument grammar,
with the full correction commit, path and SHA-256 as literal arguments. No shell
substitutions, comments, alternate invocations or prose-only mention can supply
that binding. The checker must compare its requested tuple to that exact approved
argument vector, rather than grep arbitrary shell text. The final wire grammar
and canonical bytes must be specified and reviewed
in the implementation brief before code, following existing strict canonical
record handling. Reject duplicate keys, missing fields, extraneous scope and
ambiguous combined sites. Do not add a loose prose approval or a generic path
exception. The correction descriptor is new authority-bearing input and must
receive independent security review.

Authority corroboration reuses the existing protected acceptance planner and
its ordered transition chain. It must freshly authenticate the exact mapped-human
approval for the amendment whose row-23 Command first names this descriptor and
digest. The original producer/base/declaration and all previous links remain
bound. Keep pending version 3 unmerged until the proof/descriptor Command is
complete
where feasible, so it can share the ordinary link-2 act. The pending version-3
amendment can use link 2 if it includes the final immutable descriptor; if version 3 lands without that input, a later Command
amendment needs the ordinary next link. No old approval can authorize an input
it never named. The descriptor must not hash a future approval URL or its own
amendment commit: its instance authority is resolved from that existing chain,
avoiding a circular immutable-coordinate dependency.

The same native forge seam must fetch the exact marked issue and comments each
run, require unedited matching payloads and mapped-human authority under the
existing chain rules, and authenticate the separate trusted verifier App's
attestation for the descriptor digest. A worker cannot attest its own proof.
An edited/missing comment, foreign issue, unreadable mapping, unmerged or changed
proof subject, missing source object or digest mismatch cannot clear the original
failure. No cached positive authority may cross runs, identities, sources or
repositories. There is no separate correction-artifact human comment: the one
new exact protected Command approval supplies instance authority after design
approval, implementation, review and human merge. Today's design ruling grants
none of that future acceptance credit.

## Both behavioral obligations

An independent verifier must run one immutable external harness against the
original producer product source, with the original caller files byte-identical.
Adding harness tests outside that source must not change the source under proof.
Use existing real-Git v2 fixtures, offline native forge substitutes and the
existing coverage/read-set adapter ports. The proof names source, harness,
fixtures, commands, tool versions, outputs, exits and source dirtiness explicitly.
Existing witness-v2 records/identity mechanisms should be reused where their
actual schema can bind these subjects; missing bindings must be designed rather
than claimed present. Never execute arbitrary proof code during corroboration.

For autoflip, exercise the actual `autoFlipModel` path through its coverage gate:
valid content-bound v2 claims allow the eligible model-gated transition, and a
changed declared input makes coverage refuse that transition. Observe the actual
model result/effect through existing offline seams, not just a direct evaluator
call or static call graph.

For reconcile fold, exercise `treeFoldInputs` and the real `foldVerified` path:
valid v2 claims supply the required fold coverage, while the same changed-input
case leaves the fold refused. Confirm neither test can succeed using coverage
from the other caller. No real forge, lifecycle or repository write is authorized
by running these fixture tests.

The assertion is limited to these callers adopting this producer's changed
coverage semantics. It establishes no broader implementation completeness or
unchecked census judgment. The original exit-0 and every other consumer route
remain obligatory; preserve all unchecked dispositions in the report.

## Discriminating controls and outcomes

Each caller's positive case must release actual nonempty v2 claims and fail its
changed-input case. Against the same frozen harness, the original first-parent
binary/source must refuse the real v2 claim; a missing newly introduced test or
file cannot be its failure mechanism. Then independently rewire each caller to
the legacy evaluator: its positive proof must fail even while the other caller
remains correct. Also remove the second caller entirely. These controls must not
be satisfied by a first-token-only declaration parser.

Checker controls must reject wrong producer, wrong first parent, changed brief
blob/declaration, wrong or missing second caller blob, altered/edited approval,
untrusted or same-worker attestation, wrong proof digest, stale/foreign source,
missing immutable object and forge-read failure. Test a valid correction beside
an unrelated unchanged `fixed-here` claim: that unrelated claim must still be
disproved. Revoking/removing approval after a prior pass must fail on the next
run. Negative results never become unchecked success or successful samples.

A successful report must retain the original false claim and exact correction
provenance in details, display both proved caller paths, distinguish the approved
correction from ordinary `fixed-here` corroboration, retain all remaining
unchecked judgments, and exit 0 only with no unresolved disproved route. An
inadmissible correction leaves the original route disproved; unreadable required
inputs/authority report could-not-check and a nonzero result. No original false
claim may silently disappear from output.

## Delivery and remaining work

This draft is reviewable design intent only. After a mapped-human design ruling,
the continuing #5684 repair must define and review the canonical descriptor/payload
grammar in its implementation handoff before code, implement it in the existing
consumer owner, add the actual original-source
proof harness, run all positive and negative controls, and obtain independent
correctness/security review. The exact correction artifact and independent verifier proof must be present
before verify-desk derives the corrected Command and the driver approves its
exact landed subject through the existing protected acceptance chain. A dry-run
may explain whether all non-approval conditions are satisfied, but cannot clear
the original disproved result or manufacture acceptance before that approval. The current
row 23 remains red throughout preparation. No spec status is advanced here.

There are no measured complexity/churn metrics for this proposed authority path;
that is a quality-evidence gap, not a clean bill. The bounded new authority input
and two caller integrations, rather than broader cache or consumer refactoring,
are the justified scope. Unknown performance cost must be measured during
implementation; no new threshold is invented. Rollback is to stop supplying the
correction, which restores the original failure without rewriting history.
