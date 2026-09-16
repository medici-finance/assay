---
brief: assay:assay:measured-status:05
title: "attribution.go: a same-identity author/verifier pair in a multi-identity repo becomes a hard PROBLEM, not a NOTICE — the independence gate the check exists to establish"
why: >-
  The committer-identity cross-check reads a real second signal — the git identity of a brief's
  authoring commit versus the commit that most recently added its Evidence — but it only ever
  emits a NOTICE, so a same-identity author/verifier pair still passes the verified/done gate.
  The defense-in-depth property the check was built for (security-hardening/27) therefore does
  not actually gate anything. This flips the discriminating case to a hard PROBLEM so
  independence is enforced by the code, not left as a line a reader may skim past.
wave: 2
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1116]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
design: DR-independence-gate
gate-why: >-
  This narrows the verified/done trust gate: a state that is a NOTICE today becomes a hard
  PROBLEM (exit 1), so briefs that pass now will red. A human confirms the hard-reject scope
  (multi-identity repo, same-identity author/verifier pair) and accepts that a legitimate
  same-identity re-touch will red until an independent commit or a recorded override cures it.
decision-trigger: creation
sources:
  - "#1116 — attribution.go same-identity author/verifier pair only NOTICEs, never hard-rejects (security-hardening/27 gap)"
  - "statusgen/attribution.go — attributionProblems: the committer-identity cross-check block calls only addNotice(...); add(...) is reachable only from the token-level checks"
  - "statusgen/attribution_test.go — TestAttributionIdentityCrossCheckMultiIdentity / ...SingleIdentityInconclusive assert the same-identity pair is NOT a problem today"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the len(sameIdentity)>0 branch is addNotice; no add(...) call exists in the committer-identity block"
exec-tier: strong
exec-tier-why: changes a trust-gate verdict from advisory to blocking on a live board; a mis-scoped reject reds every brief in a single-identity checkout
domain: complicated
value: high
version: 1
id: 2cb59a99-aca1-47f2-be84-60be1a432231
---

# Brief 05 — Same-identity author/verifier becomes a hard reject

## Context
files:
- `statusgen/attribution.go` — `attributionProblems`, the committer-identity cross-check block
  and its `sameIdentity` resolution (currently `addNotice`).
- `statusgen/attribution_test.go` — the two cross-check tests asserting today's NOTICE-only behaviour.
- `statusgen/testdata/attribution/` — the brief fixtures the tests read.
facts:
- three-state read MUST be preserved: one git identity behind every checked brief is
  INCONCLUSIVE (commit metadata cannot tell author from verifier) and stays a NOTICE; MULTIPLE
  identities with a same-identity author/verifier pair is the DISCRIMINATING case that becomes a
  hard PROBLEM; git-unreadable stays the loud could-not-check NOTICE it is today.
- the token-level checks (`selfVerificationReason`, `evidenceHasIndependentRow`) already
  `add(...)` hard problems; this brief brings the committer-identity signal up to the same force
  for its discriminating case only — it does not touch the token checks.
- the two existing tests assert the NOTICE-only behaviour and MUST be updated to assert the hard
  PROBLEM for the multi-identity same-pair case (and the still-NOTICE for the inconclusive case).
- statusgen is one Go module; tests run from the repo root.

## Human decision
A verification is supposed to be independent: the person or agent who checks a piece of work
should not be the same one who did it. The tool can already read a second, mechanical signal
for this — it compares who committed the original work against who most recently committed the
record that says the work was verified. Right now, when those are the same identity, the tool
only prints a soft notice, and the work can still be marked verified and done. This asks
whether that soft notice should become a hard failure that blocks the verified and done
status, in the one case where the signal is trustworthy: a repository that has several distinct
committer identities, where the original work and its verification record were nonetheless
committed under one and the same identity. A repository with only a single identity cannot tell
the two roles apart at all and would stay a soft notice.

Options:
1. **Make it a hard failure (recommended)** — in a multi-identity repository, a same-identity
   author-and-verifier pair blocks verified/done until an independent commit, or a recorded
   human override, cures it. Consequence: some items that pass today will fail until a genuinely
   independent act is recorded. This is the property the check was built to enforce.
2. **Leave it a soft notice** — nothing that passes today starts failing, but independence
   remains unenforced and a same-identity pair can still reach verified/done. Consequence: the
   known weakness this addresses stays open.
3. **Hard failure, but also require an explicit override path** — same as option 1 plus a
   recorded, human-authorized override for the legitimate same-identity re-touch case, so a
   genuine post-verification fix has a sanctioned way through.

Default if no answer: none — blocks until answered (this narrows a trust gate and must not be defaulted).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In `attributionProblems`, change the `len(sameIdentity) > 0` resolution for a MULTI-identity
   repo from `addNotice(...)` to `add(...)` (a hard PROBLEM), keeping the message content
   (which briefs, which identity). Leave the single-identity inconclusive branch and the
   git-unreadable branch as NOTICEs.
2. Update `TestAttributionIdentityCrossCheckMultiIdentity` to assert the same-identity pair now
   produces a PROBLEM, and keep `...SingleIdentityInconclusive` asserting a NOTICE (no problem).
3. Add/confirm a fixture proving the multi-identity same-pair case reds and an independent-pair
   case stays green.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run TestAttributionIdentityCrossCheck -count=1` | exit 0; output contains "ok" |
| 2 | `go test ./statusgen/ -run 'TestAttributionIdentity' -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (both the hard-reject and the inconclusive-NOTICE cases ran and passed) |
| 3 | `go vet ./statusgen/` | exit 0 (the flipped branch compiles and vets clean) |
| 4 | `go test ./statusgen/ -count=1` | exit 0 (the whole statusgen suite is green with the flip) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

## Review
Gate: human (from frontmatter). Human gate is MANDATORY — this narrows a trust gate. Reviewer
records verdict + date in the stream README table.
