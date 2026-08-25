---
brief: iso-9001/03
title: A finding closes on a fired control — the corrective-action effectiveness record
why: >-
  A findings entry records that a corrective action was taken. `resolved: yes` means the work
  landed; it does not mean the failure mode can no longer occur, and nothing in the schema
  carries the difference. The register already holds the nature of the nonconformity and the
  action; it does not hold the result. The corpus already applies the right rule to its own
  audits — a finding closes only when its check goes green, never on a fix commit — and
  already demands, of any brief that adds a check, a mutation row proving the check goes RED.
  This brief generalises that shape to the register, so that closing a finding requires naming
  the command that re-establishes the failure mode is gone, the date it was run, and who ran
  it.
wave: 1
depends: ["iso-9001/01"]
unblocks: ["iso-9001/06"]
effort: M
exec-tier: strong
exec-tier-why: >-
  A schema addition plus a severity policy over an inherited corpus. The transition scoping is
  the judgement — a rule that fires retroactively on every landed finding manufactures
  false positives and is the fastest route to an exemption file.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`statusgen/registerentries.go` — the findings entry schema today: `id`, `date`, `title`, `affects`, `ack`, `resolved`, the `parked-until` / `parked-by` / `parked-reason` triple, and the optional `class` / `control` pair. No effectiveness field, no date, no runner."
  - "`statusgen/findingcontrol.go` — the nearest existing thing: a recurring-class finding must name a landed control before the class counts closed. It is a NOTICE, it is scoped to recurring-class findings only, and it checks that a control LANDED rather than that it FIRES. Its header states its own escalation posture: advisory this phase, a hard error is a later, separate decision."
  - "`docs/brief-rules.md` rule 16 — a brief that adds a CHECK must include a mutation-test Verify row: revert the fix or break the guarded thing, run the check, confirm it goes RED. Where a corrective action is 'add a control', effectiveness is already demonstrated; this brief generalises the shape rather than inventing one."
  - "`docs/mistake-proofing.md` §3 D1 (a control must be shown to fire) and §3 D4 (warnings do not compose — land advisory, census, then flip fatal on a named condition; do not add a permanent NOTICE)."
  - "The parked triple as the in-tree precedent for a group of keys that are all required together, with a named human as the authority: `parked-until` + `parked-by` + `parked-reason`, 'a park is a snooze, not a mute'."
  - "depends iso-9001/01: the per-control evidence row shape — the control, the injected error, the verdict, the date, the tool version — is defined there. This brief reuses it for a finding's effectiveness record rather than inventing a second shape for the same idea."
  - "The standard-side reading: the corrective-action clause asks the organisation to evaluate the need to eliminate the cause, determine whether similar nonconformities exist or could occur elsewhere, review the EFFECTIVENESS of the action taken, and retain records of the nature of the nonconformity, the actions taken and THE RESULTS. The most-written finding against it is a correction recorded as a corrective action, followed by a record closed on the day the action was implemented with no later effectiveness check."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — `git grep -n effectiveness -- statusgen/registerentries.go statusgen/findingcontrol.go` returns nothing; the field does not exist."
---

# Brief 03 — the effectiveness record

## Context

single-point-of-failure: after this brief, the lint's presence obligation is the control
standing between a finding closed on a fix commit and a clean register. It is deliberately a
thin one — it proves an effectiveness record EXISTS and was attributed and dated, not that
the named command actually re-establishes anything. The independent second layer is
unchanged and must stay: the reviewer question "does this command genuinely fail if the
failure mode returned?" is asked at a different time, by a different actor, on different
evidence. This brief adds a floor; the failure message must say so.

files:
- `statusgen/registerentries.go` (implementation home) — the findings entry schema.
- `statusgen/findingcontrol.go` — the severity path for a recurring-class finding, which this
  brief extends from "a control landed" to "a control was shown to fire".
- `docs/registers.md` — the schema documentation for the new keys.

facts:
- **Three keys, required together, or none.** Mirror the parked triple exactly:
  `effectiveness:` (the command, verbatim, that re-establishes the failure mode is gone),
  `effectiveness-date:` and `effectiveness-by:` (a `human:<name>` or a runner identity, in the
  same shape the Verified cell already uses). One or two of the three present is a hard
  error, the same way a half-filled park is: a partial record is worse than no record because
  it reads as one.
- **The severity must be transition-scoped by the finding's own `date:`.** A finding dated on
  or after the field's introduction and carrying `resolved: yes` owes the triple as a
  PROBLEM; an earlier finding owes it as a NOTICE. The inherited corpus is not made fatal by
  this change. Encode the boundary date as a named constant with a comment, not an inline
  literal, so a reviewer can find and argue with it.
- **`findingcontrol` escalates in the same change, and only for the class it already covers.**
  Once the triple exists, a `class: recurring` finding with `resolved: yes` and a landed
  `control:` but no effectiveness record is the exact gap that check was written to surface —
  promote it there from NOTICE to PROBLEM, under the same date scoping. Leave one-off
  findings alone: `findingcontrol`'s own header says an absent `class:` reads as one-off and
  must never be flagged, and widening that here would be a second decision hiding inside this
  one.
- **Do not add a `cause:` field in this brief.** Root-cause capture is a real gap and it is a
  different obligation with a different failure mode (a cause stated as a restatement of the
  symptom passes any presence check). Presence of a `cause:` string proves nothing, so a lint
  obligation on it would be proofing judgement, which D7 forbids. Name it as out of scope in
  the coverage note.
- **This check must survive the firing audit.** `--lint-audit` flags a rule that has never
  fired as a retirement candidate. This rule needs its own positive control — which is the
  rule applied to itself, and is the point.
- **Rule-tag every emitted message.** New PROBLEM/NOTICE lines carry a stable `[rule-tag]`
  bracket token, the convention `statusgen/lintaudit.go` already extracts; an untagged line
  falls into the unattributed bucket and is invisible to the firing audit.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Add the three optional keys to the findings entry struct** in
   `statusgen/registerentries.go`, beside the parked triple, with a comment stating what each
   is for and that the three are required together.
2. **Enforce the all-or-nothing rule**: one or two of the three present is a hard error naming
   which are missing. Reuse the parked triple's existing message shape rather than inventing a
   second dialect.
3. **Add the closure obligation**, transition-scoped by the finding's `date:` against a named
   constant: `resolved: yes` on or after the boundary without the triple is a PROBLEM; before
   it, a NOTICE. Carry a rule-tag.
4. **Promote the `findingcontrol` path for recurring-class findings** from NOTICE to PROBLEM,
   under the same date scoping, when the effectiveness record is absent. Do not change the
   behaviour for `class:` absent or `class: one-off`.
5. **Write the failure message to be actionable and honest.** It names the finding ID, states
   which of the three keys are missing, states that the check verifies the record's PRESENCE
   and attribution and not whether the named command re-establishes anything, and points at
   rule 16's mutation row as the worked example of what a good one looks like.
6. **Document the keys in `docs/registers.md`** in the shared-conventions section, in the same
   register as the parked triple, with one sentence of reason each. Record the coverage
   boundary beside them (D6): root cause is deliberately not a field, and a present
   effectiveness command is not a demonstration that it fires.
7. **Positive control on this check — the rule applied to itself.** Tests that inject a
   `resolved: yes` finding after the boundary with no triple and assert a PROBLEM; the same
   finding with the triple asserting silence; a finding dated before the boundary asserting a
   NOTICE and not a PROBLEM; and a partial triple asserting the all-or-nothing error.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -n 'effectiveness' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE, inverts**: no such field at authoring (2026-08-25 @ `6871a3b`) |
| 2 | `git grep -nF 'yaml:"resolved"' -- statusgen/registerentries.go` | exit 0 — **DEREFERENCE**: the field the new obligation is keyed on still exists and was not renamed under the change |
| 3 | `git grep -n 'recurringClass' -- statusgen/findingcontrol.go` | exit 0 — **DEREFERENCE**: the recurring-class concept the severity promotion is scoped to is real, so the promotion is not a dangling reference |
| 4 | `git grep -n 'effectiveness' -- docs/registers.md` | exit 0 — the schema page documents the new keys; a lint that fails a pull request citing a rule needs the rule written down |
| 5 | `cd statusgen && go test ./... -count=1 -run EffectivenessTripleRequiredTogether` | exit 0 — one or two of the three keys present is a hard error naming the missing ones; all three, or none, is silent |
| 6 | `cd statusgen && go test ./... -count=1 -run EffectivenessMissingOnResolvedIsAProblem` | exit 0 — **mutation row (rule 16), positive control**: a `resolved: yes` finding dated after the boundary with no triple is a PROBLEM; the same finding with the triple is silent. Disarm the obligation and this test goes RED |
| 7 | `cd statusgen && go test ./... -count=1 -run EffectivenessInheritedCorpusStaysAdvisory` | exit 0 — the promotion is transition-scoped: a finding dated before the boundary produces a NOTICE and never a PROBLEM |
| 8 | `cd statusgen && go test ./... -count=1 -run FindingControlOneOffUnchanged` | exit 0 — **neighbour row (rule 17)**: `findingcontrol` shares the findings frontmatter reader; a finding with no `class:` or `class: one-off` behaves exactly as before this change |
| 9 | `cd statusgen && go test ./... -count=1` | exit 0 — the full lint suite passes |
| 10 | `git grep -cE -e 'presence' -e 'adequacy' -- statusgen/findingcontrol.go statusgen/registerentries.go` | exit 0; a non-zero count — the presence-not-adequacy boundary is stated in the source the failure message is built from |
| 11 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree still lints clean under the new obligation, which is also the census: this repo's register carries no entry the new rule fires on |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in
the stream README table. Reviewer questions specific to this brief: (1) is the boundary date a
named constant with a comment rather than an inline literal, and is the inherited corpus
genuinely left advisory? (2) does the check carry its OWN positive control — the rule applied
to itself? (3) is the `findingcontrol` promotion confined to `class: recurring`, with a test
proving one-off behaviour is unchanged? (4) does the failure message say which half it covers
— presence and attribution, not whether the named command re-establishes anything? (5) was
`cause:` genuinely left out rather than smuggled in as an unenforced key?
