---
brief: mistake-proofing/06
title: D1 promoted to a lint obligation — a change that adds a check must carry its mutation row
why: >-
  The methodology's sharpest requirement is that a control must be shown to fire: before trusting a
  clean report from any instrument, prove the instrument fails when the guarded thing is broken. It
  holds every instrument in the system to that standard and holds the requirement itself with a MUST
  in a markdown file. The human substitute works — a fail-first review discipline caught roughly
  fifteen real cannot-fail-control bugs in one night — which is simultaneously proof that the defect
  class is dense and proof that the only device is a reviewer being careful. This brief makes the
  positive control a control: a change that adds a check and carries no mutation row does not merge.
wave: 2
depends: ["mistake-proofing/03"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the mistake-proofing board)
sources:
  - "`docs/mistake-proofing.md` §3 D1: 'For every control-mode device there MUST exist a demonstration that an injected instance of the error it claims to stop actually reddens it — a mutation test, a positive control, a deliberate bad input in its test suite. A control that has never fired is either unnecessary or broken, and without injection you cannot tell which.' Also §5 ladder step 1, where D1 is the first thing an adopting repo does."
  - "Same spec §4 B2 and §3 D7 — the presence of the row is the control; whether the mutation is a good one stays with review. This brief promotes PRESENCE to fatal and claims nothing about adequacy."
  - "`docs/three-state-instrument-rule.md`, positive-control requirement — the same rule stated for instruments generally, and the source of the could-not-check state this check must keep distinct from silence."
  - "The device inventory behind this stream (2026-08-25): 'the positive-control requirement is prose in a system whose whole thesis is positive control — this is the system's sharpest irony'. Records the yield of the human substitute (~15 real cannot-fail-control bugs in one night) and names the phasing to copy."
  - "depends mistake-proofing/03: the mutation obligation value, the derivation mechanism and the advisory landing all come from 03. This brief is the promotion to fatal plus the check-shaped-path definition, and deliberately does not re-open 03's design."
  - "The in-tree precedent for a machine-checked positive control: the desk tree's supply-chain source guard, whose refusals are mutation-checked in CI by the mutation harness — proof that the wiring works, on a guard of comparable shape."
  - "freshness-checked 2026-08-25 @ 657cab1 (origin/main) — no mutation obligation exists in the lint; the mutation harness exists but is a local diagnostic, not a pull-request gate."
---

# Brief 06 — D1 as a lint obligation

## Context

single-point-of-failure: after this brief, the control standing between a cannot-fail check and the
main branch is the lint's presence obligation. That is one layer and it is deliberately a thin one —
it proves a mutation row EXISTS, not that the mutation is real. The independent second layer is
unchanged and must stay: the reviewer question "does any row prove this check reddens when the
guarded thing is broken?" is asked at a different time, by a different actor, on different evidence.
This brief adds a floor; it does not replace the reviewer, and the failure message must say so.

files:
- `statusgen/` (implementation home) — the obligation derivation added by mistake-proofing/03,
  promoted for the mutation obligation only, plus the check-shaped-path definition, plus tests.

facts:
- Everything this brief needs — the mutation obligation value, the derivation from the branch diff,
  the could-not-check state, the rule-tag convention — is delivered by mistake-proofing/03 and lands
  there as an **advisory notice**. This brief changes exactly one thing: the severity, for one
  obligation. It does not re-open 03's encoding decision.
- **The check-shaped path set is the whole judgement in this brief**, and it must be written down as
  an enumeration rather than inferred. The shapes that qualify: a lint or check source file in the
  tool tree, a guard in the desk tree, a CI workflow file, and a reviewed verify script. Err toward a
  narrow, enumerated set: an over-broad definition makes the obligation fire on unrelated changes,
  and an obligation that fires on unrelated changes is the fastest route to an exemption file.
- **Fail closed on the diff, fail open on the definition.** If the branch diff is unavailable, the
  derivation is could-not-check and refuses — the same posture as every other check in the tree. If
  the diff is available but no path matches the enumerated set, nothing is owed. Those are different
  failures and must not be collapsed: "I could not look" is not "I looked and found nothing".
- The methodology already treats a control with no demonstration as a finding, and already asks
  "when did this last stop something?" as an audit question. There is a firing-audit mode that flags
  a rule with zero firings and no referencing test as a retirement candidate. **This brief's own
  check must survive that audit**, which means it needs its own positive control — which is the
  rule applied to itself, and is the point.
- The mutation harness exists and is a correct three-state tool with a mandatory positive control
  and an explicit harness-broken state, but it is a local diagnostic rather than a pull-request
  gate. **Wiring it as a gate is out of scope here** — this brief promotes the presence obligation
  only. Name the harness in the failure message as the recommended way to produce the demonstration;
  do not make it a dependency.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Enumerate the check-shaped path set explicitly in source**, as a named list with a one-line
   rationale per entry, in the file the derivation lives in. Not a regex assembled inline; a list a
   reviewer can read and argue with. Keep it narrow.
2. **Promote the mutation obligation from advisory to fatal**, for changes this branch makes only —
   the inherited corpus stays advisory. This is the same transition-scoped posture other checks in
   the tree already use, and it is what makes the promotion landable in one pull request instead of
   a corpus migration.
3. **Write the failure message to be actionable and honest.** It names the path that triggered the
   obligation, states that a mutation row is required, states that the check verifies the row's
   PRESENCE and not its adequacy, and points at the mutation harness as the recommended way to
   produce the demonstration. Carry the stable rule-tag bracket token.
4. **Keep the two failures distinct.** Diff unavailable → could-not-check, refuse. Diff available,
   no check-shaped path → nothing owed, silent. A test asserts they are not collapsed.
5. **Positive control on this check — the rule applied to itself.** A test that injects a diff adding
   a check-shaped path with no mutation row and asserts a fatal problem; the same diff with the row
   present asserting silence; and a diff touching no check-shaped path asserting silence. This
   brief's own deliverable is a check, so it owes exactly what it demands, and the reviewer should
   verify that it does.
6. **Record the coverage boundary beside the check** (spec §3 D6): which check-shaped surfaces are
   deliberately outside the enumerated set, and the fact that a real mutation demonstration is
   stronger than a present row. Honest non-coverage is itself a device; a list of what this does not
   catch belongs next to what it does.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `git grep -ci 'mutation' -- statusgen/rowclass.go` | exit 1, no output — **DEREFERENCE, true at authoring (2026-08-25 @ `657cab1`)**: the mutation obligation does not exist in the row-class source. Inverts once mistake-proofing/03 and this brief land |
| 2 | check | `test -d tools/desk/cmd/muhar` | exit 0 — **DEREFERENCE**: the mutation harness the failure message points at really exists, so the recommendation is not a dangling reference |
| 3 | check | `git grep -n 'COLD' -- statusgen/lintaudit.go` | exit 0 — **DEREFERENCE**: the firing audit that flags a zero-firing rule as a retirement candidate is real, which is why this check owes its own positive control |
| 4 | check | `go test ./statusgen/ -run 'MutationObligation' -count=1` | exit 0 — the promoted obligation's tests pass |
| 5 | check +mutation | `go test ./statusgen/ -run 'MutationObligationFiresOnAddedCheck' -count=1` | exit 0 — **positive control** (the rule applied to itself): a diff adding a check-shaped path with no mutation row is fatal; the same diff with the row is silent; a diff touching no check-shaped path is silent. `+mutation` declares that this brief's own deliverable carries the demonstration it demands |
| 6 | check | `go test ./statusgen/ -run 'MutationObligationCouldNotCheckIsNotSilence' -count=1` | exit 0 — an unavailable diff refuses, and is distinguishable in the output from "nothing owed" |
| 7 | check | `go test ./statusgen/ -run 'MutationObligationInheritedCorpusStaysAdvisory' -count=1` | exit 0 — the promotion is transition-scoped: inherited briefs are not made fatal by this change |
| 8 | check | `git grep -c 'adequacy' -- statusgen/` | exit 0; a non-zero count — the presence-not-adequacy boundary is stated in the source the failure message is built from. Zero hits today (2026-08-25 @ `657cab1`) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the
stream README table. Reviewer questions specific to this brief: (1) is the check-shaped path set an
explicit, narrow, rationale-carrying enumeration in source? (2) does this check carry its OWN
mutation demonstration — the rule applied to itself? (3) are could-not-check and nothing-owed
distinguishable in the output, with a test? (4) is the promotion transition-scoped so the inherited
corpus is not made fatal?
