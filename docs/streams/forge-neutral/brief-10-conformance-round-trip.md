---
brief: forge-neutral/10
title: Conformance — one round trip driven entirely by desk verbs, and the writes they refuse
why: >-
  Every other brief in this stream is checked by tests against recorded fixtures. Only a live
  round trip can show that the verbs actually carry a brief from todo to done on a real
  deployment with nothing hand-built in the loop — which is exactly what the 2026-09-02 pilot
  could NOT do, and why it recorded that every write was a curl. And only a negative-path walk
  can show the other half of the claim: that a session holding the verbs cannot perform a write
  the verbs enumerate as refused. A stream about a boundary is not finished until someone has
  tried to stand on both sides of it.
wave: 5
depends: ["forge-neutral/03", "forge-neutral/04", "forge-neutral/05", "forge-neutral/06", "forge-neutral/07", "forge-neutral/08", "forge-neutral/09"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-gitlab/pilot-report.md §2 — the verbs had no GitLab backend, so every write was hand-built; this brief is the run where that sentence stops being true"
  - "docs/streams/forge-gitlab/pilot-report.md D-8 — the Evidence lane and the board lane each gain a human hop on a forge with no direct-default-branch push, and the stacking that contains the cost"
  - "docs/streams/forge-gitlab/brief-05-live-pilot-parity-walk.md — the pilot this conformance run is the successor to, and whose group it reuses"
  - "docs/streams/forge-neutral/brief-04-write-verbs-issues-and-evidence.md — the Evidence-landing lane whose loop shape this run proves before any skill text changes"
  - "docs/streams/forge-neutral/brief-05-claim-layer-forge-shape.md — the claim namespace whose release this run round-trips live"
  - "freshness-checked 2026-09-02 @ deae247 — no desk verb reaches a forge through the Forge seam; the pilot's own §2 is the baseline this run is measured against"
exec-tier: strong
exec-tier-why: "a live conformance walk against a real deployment with real credentials, where the finding is the whole deliverable and a row credited from the wrong evidence (a branch read standing in for a default-branch read) survives every local check (questions a and b)."
gate-why: >-
  This run uses real credentials against a live deployment and performs real writes as the
  fleet's role identities. It also produces the record on which "the fleet runs on GitLab"
  would be claimed, so a row credited generously here retires a control everywhere. The human
  is confirming the deployment and credentials before the run, and afterwards that every row
  is PASS / FAILED / COULD-NOT-CHECK on evidence actually read — never pre-credited from
  readiness.
domain: complex
consumers:
  - "docs/streams/forge-neutral/conformance-report.md: fixed-here (the run record)"
  - "plugins/assay/skills/verify-desk/SKILL.md: fixed-here (the Evidence-landing lane's extra hop, now that the round trip has proved the loop shape — the follow-up forge-neutral/04 deferred to this brief)"
  - "plugins/assay/skills/worker-desk/SKILL.md, plugins/assay/skills/pr-shepherd/SKILL.md: fixed-here (the claim-namespace wording forge-neutral/05 deferred to this brief)"
  - "docs/adopting-assay-gitlab.md: fixed-here (what an adopter can now do with verbs rather than by hand)"
  - "docs/streams/forge-gitlab/pilot-report.md: out-of-scope (a dated record of a different run; superseded by reference, never edited)"
---

# Brief 10 — Conformance round trip

## Context
files:
- `docs/streams/forge-neutral/conformance-report.md` (planned) — the run record.
- `plugins/assay/skills/verify-desk/SKILL.md` — the Evidence-landing lane.
- `plugins/assay/skills/worker-desk/SKILL.md`,
  `plugins/assay/skills/pr-shepherd/SKILL.md` — the claim-namespace wording.
- `docs/adopting-assay-gitlab.md` — the adopter runbook.

single-point-of-failure: the run record is the one artifact anything downstream would rely on,
so the control that matters is that no row may be credited from anything but a live read. Two
independent layers: the three-state verdict discipline inherited from the pilot (a row the
instrument did not look at says COULD-NOT-CHECK, never PASS), and the requirement that every
row cite the verb invocation AND its exit code — so a row asserted from a plan rather than a
run has no citation to show, which a reader catches on a different signal than a wrong verdict.

facts:
- The pilot's §2 is the baseline: *"Every write in §1 was a hand-built `curl` call against
  REST v4 or a raw `git` push. The desk verbs the GitHub lane uses — `deskpr`, `deskpost`,
  `deskflip`, `deskfile`, `deskevidence` — have no GitLab backend, so none of them was
  reachable for any step."* (`docs/streams/forge-gitlab/pilot-report.md` §2).
- The pilot group is real and its ids are recorded in that report §0; it is private, its
  default branch is push = No one for every identity, and merge is the human owner's.
- On a forge with no direct-default-branch push the Evidence row and the board regeneration
  each become a change; the pilot showed the cost is contained by STACKING the board change
  onto the Evidence change rather than serialising two human gates
  (`pilot-report.md` D-8).
- Verify row 4 of the pilot was recorded COULD-NOT-CHECK rather than pre-credited, because a
  branch read is evidence of readiness and not of the row (`pilot-report.md` §4 row 4). That
  discipline is inherited here verbatim.
- `forge-neutral/09`'s leak gate must have a verdict surface on a merge request before this
  run can honestly walk a gated change.
- Token hygiene on the pilot: no token value printed, in an argv, or written inside a
  checkout; per-role credentials 0600 outside every checkout; the owner credential used for
  reads only (`pilot-report.md` §6). The same discipline applies.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl outside the run this brief
  defines. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Zero hand-built API calls in the round trip.** If a step cannot be performed by a verb,
  that is the finding — record it as a deviation and stop; do not reach for `curl` to complete
  the walk. A round trip completed by hand is the thing this brief exists to stop claiming.
- Merge is the human's on every change, as on the pilot. Nothing here flips a change ready or
  merges one.
- A row the instrument did not look at is COULD-NOT-CHECK. Never PASS.

## Task
1. **Round trip, verbs only.** On the pilot deployment, carry one brief `todo → done`:
   dispatch and claim it, open the draft change, post the reviewer verdict, flip it ready,
   have the human merge, run the Verify row against the merged default branch, land the
   Evidence row, regenerate the board, and release the claim. Every forge write is a desk verb
   invocation. Record, per step, the verb, its arguments, its exit code, and the resulting
   artifact id.
2. **The negative-path walk.** With only the verbs available and a role credential present,
   attempt each write the verbs enumerate as refused, and record what happened: a write to the
   default branch; a merge; a ready flip with no at-head verdict; a comment posted as an
   identity the roster does not bind; an operation the resolved forge cannot serve. Each must
   refuse with its exit-code class and a message naming the reason. **A refusal that the
   session can route around by another verb is a finding, and so is a refusal whose message
   does not say what would make it succeed.**
3. **The boundary statement, measured not asserted.** Record what a session holding a role
   token file could do if it reached the forge directly rather than through a verb, and what
   the verbs would have refused — as an evidence table, not prose. This is the stream's thesis
   and it should be readable as a measurement.
4. **Write the record.** `conformance-report.md`, in the pilot report's shape: a substrate
   section with live ids, the round-trip table, the negative-path table, the boundary table, a
   deviations section, and a token-hygiene section. Every claim cites a verb invocation and an
   exit code, or an endpoint and a status code.
5. **Discharge the deferred skill edits.** With the loop shape proved, update the verify-desk
   skill's Evidence-landing lane and the worker-desk / pr-shepherd claim-namespace wording —
   the follow-ups `forge-neutral/04` and `forge-neutral/05` deferred here on purpose, so the
   prose changes after the shape is known rather than before.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c '^[\|] ' docs/streams/forge-neutral/conformance-report.md` | ≥ 20 — the four tables (substrate, round trip, negative path, boundary) are present as tables |
| 2 | `grep -c 'curl' docs/streams/forge-neutral/conformance-report.md` | the only occurrences are in the baseline citation and the boundary table; **any occurrence describing a step of the round trip itself is a FAIL** — read as a list, not a count |
| 3 | `grep -c 'COULD-NOT-CHECK' docs/streams/forge-neutral/conformance-report.md` | ≥ 0 — a run with no could-not-check rows is possible but must be argued; the row exists so the count is stated rather than assumed |
| 4 | the round-trip table's `Mechanism` column | every row names a desk verb and an exit code; a row naming an endpoint instead is a deviation and must appear in the deviations section |
| 5 | the negative-path table | every enumerated refusal has a row with its exit-code class and the refusal message; a refusal that succeeded is recorded FAILED, never omitted |
| 6 | `statusgen --root . --lint` — run inside a clone of the pilot tracking root, on the merged default branch (manual: the verifier names the deployment) | `LINT: PASS`, exit 0 — including the Evidence-actor check, which after `forge-neutral/07` must report the row BACKED rather than could-not-check |
| 7 | the brief's Evidence row on the pilot deployment, read on the merged default branch | committed by the verifier role identity, and the witness `Runner` names that same identity with source `forge identity` — the D-9 disagreement is gone |
| 8 | `git ls-remote origin` filtered to the claim namespace, run against the pilot deployment after the round trip (manual: the verifier names the remote and the namespace forge-neutral/05 chose) | the claim for the round-tripped brief is absent — released through the verb, not left behind |
| 9 | the leak gate's verdict on the round trip's change | present and readable; if the deployment's tier cannot express the blocking form, the pipeline-side job ran and its result is cited |
| 10 | `statusgen --root . --consumers --brief forge-neutral/10` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| A step is quietly completed with a hand-built call to finish the walk, and the report reads clean | rows 2 + 4 — every round-trip row must name a verb and an exit code, so a hand-built step has no legal cell to sit in |
| A row is pre-credited from a branch read rather than from the merged default branch — the exact trap the pilot named and avoided | rows 6, 7 and 8 all specify the merged default branch or the remote, not a branch |
| The negative-path walk is skipped or reported as "not attempted", so the boundary claim rests on the design rather than a trial | row 5 requires a row per enumerated refusal, with its message |
| A refusal is real but routable — one verb refuses and another performs the same write | row 5's requirement that a refusal which succeeded is recorded FAILED; a routed-around refusal IS a succeeded write |
| The Evidence-actor check still reports could-not-check on the pilot, so `forge-neutral/07` did not actually land | row 6 |
| The witness and the Evidence table still disagree | row 7 |
| The claim is taken and never released, and nobody looks | row 8 |
| The leak gate is absent and its absence is not recorded | row 9 |
| The report is honest but the skill text is updated to a loop shape the run did not actually walk | row 4 is the source the skill edits must match; agreement between them is **review-only** |
| A token value reaches the report | **no row** — the pilot's §6 discipline is inherited and the Review gate reads the report for any credential material before it lands |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a false conformance claim and its being believed? (The
   run record.) Is it acceptable alone? (No — the citation requirement is the second layer: a
   row asserted from a plan rather than a run has no verb invocation and no exit code to
   show.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 5:
   with the design assumed correct, the negative-path walk still has to observe each refusal
   actually firing, and records a routed-around refusal as FAILED.)
