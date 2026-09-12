---
brief: assay:assay:contributor-trust:07
title: "Agent contributors — disclosure of automated authorship, and tiering the operating human rather than the tool"
why: >-
  The submissions this stream was commissioned after were almost certainly produced by an
  agent operated by a person, and the project has no position on that. It needs one, because
  the two obvious positions are both wrong: banning agent-written contributions is
  unenforceable and would reject good work, while ignoring the question leaves the tier model
  trying to sort tools instead of the people accountable for them. Asking for disclosure and
  tiering the operator settles it, and gives the automated-sweep signals somewhere to land
  other than an unstated hunch.
wave: 1
depends: ["contributor-trust/01", "contributor-trust/02", "contributor-trust/06"]
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: yes}
gate-why: >-
  This brief states a public position on agent-authored contributions and defines the
  conditions under which an identity's trust level is LOWERED — an adverse recorded judgement
  about a named person, driven by signals that each have an innocent explanation. The human is
  confirming three things: that disclosure is requested and never required as a merge
  condition (an unenforceable requirement invites dishonest answers and punishes honest ones),
  that the signals named as downgrade grounds are about submission behaviour and never about
  the use of automation as such, and that a downgrade remains a recorded human act with a
  reason rather than anything a counter performs.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §1 — the observed pattern is an automated sweep, and §4's boundary that promotion and demotion are recorded human acts with reasons."
  - "docs/streams/decisions/DR-trust-tiers-ledger.md — the design record this brief extends (PROPOSED): tiers grant automation never merge authority, and nothing moves an identity between tiers by itself."
  - "docs/streams/contributor-trust/brief-01-provenance-probe.md — the signal set this brief refers downgrade grounds to; the signals are defined there and are not redefined here."
  - "docs/streams/contributor-trust/brief-06-contributor-facing-docs.md — the pull-request template carrying the automated-assistance disclosure field; this brief states what the answer is used for."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — no document in the tree states any position on agent-authored external contributions."
design: DR-trust-tiers-ledger
decision-trigger: creation
exec-tier: strong
exec-tier-why: >-
  (c) the deliverable defines when a trust level is lowered for a named person; a rule that is
  slightly too broad produces adverse judgements the evidence does not support, and no test
  of the surrounding machinery would fail.
consumers:
  - "docs/contributor-trust.md: follow-up contributor-trust/07 (this brief; the agent-contributor position and the downgrade grounds)"
  - "CONTRIBUTING.md: follow-up contributor-trust/07 (this brief; the contributor-facing sentence on what the disclosure answer is used for)"
  - ".github/PULL_REQUEST_TEMPLATE.md: follow-up contributor-trust/07 (this brief; the disclosure field's help text gains its purpose)"
  - "tools/desk/internal/deskkit/trusttier.go: out-of-scope (this brief adds no code path and no automatic tier change; the downgrade it defines is performed by a human through the existing recorded act)"
version: 1
---

# Brief 07 — Agent contributors

## Context

files:
- `docs/contributor-trust.md` (planned) — the agent-contributor position: disclosure is requested, the
  tier attaches to the operating human, and the enumerated grounds on which a maintainer may
  record a downgrade.
- `CONTRIBUTING.md` — one contributor-facing paragraph: what the disclosure field is for, and
  that answering it honestly costs nothing.
- `.github/PULL_REQUEST_TEMPLATE.md` (planned) — the disclosure field's help text gains its purpose.
- `changelog/contributor-trust-agents.md` (new).

single-point-of-failure: the requirement that a downgrade is a recorded human act with a
written reason. Behind it, two independent layers — the grounds are an enumerated closed list
(a maintainer recording a downgrade outside the list is recording something the published
policy does not support, which a reader can see) and no code path performs a tier change at
all, so no accumulation of signals can move an identity without somebody writing the row.

facts:
- Position: a contribution produced with automated assistance is welcome on the same terms as
  any other. What is judged is the submission, never the tool.
- The tier attaches to the operating human — the account that opened the item and is
  accountable for it. There is no separate tier for a tool, and an agent operating under a
  person's account inherits that person's tier.
- Disclosure is REQUESTED, never required as a condition of merge. An undisclosed
  agent-authored change is not a violation; an author who discloses is not penalised for it.
  Requiring it would be unenforceable and would punish exactly the honest answers it wants.
- Downgrade grounds, enumerated and closed: submissions whose body claims are repeatedly
  contradicted by their own diffs; submissions opened at a rate that makes individual
  authorship implausible AND whose bodies are near-identical in shape; and failure to respond
  substantively to review findings across multiple items. Every ground is about submission
  behaviour. The use of automation is NOT a ground, on its own or in combination.
- A downgrade is recorded the same way a promotion is: a human writes the row, with a date and
  a reason. No signal, and no accumulation of signals, performs one.
- The grounds are published, so an identity can see what would lower their level and can
  answer it.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
Some of the changes this project receives from outside are written with automated assistance,
and the project currently says nothing about that anywhere. Two things need settling.

First, the position. The proposal is that a change produced with automated help is welcome on
exactly the same terms as any other — what gets judged is the change, not how it was written
— and that the pull-request template asks the author to say whether they used automated help.
Asking is not requiring: somebody who does not answer has not broken a rule, and somebody who
answers honestly is not penalised for it. The trust level attaches to the person whose account
opened the change and who is accountable for it; there is no separate level for a tool.

Second, when a trust level may be lowered. The proposal is a short closed list: a pattern of
descriptions that their own changes contradict; a submission rate that makes individual
authorship implausible combined with descriptions that are near-identical to each other; and
repeatedly not engaging with review feedback across several changes. Each is about how
somebody submits, not about whether they used a tool. Using automation would explicitly not
be grounds on its own or in any combination. Lowering a level would remain something a
maintainer writes down, with a reason, exactly as raising one is — nothing automatic.

Options:
1. **Request disclosure, tier the operating person, and publish the closed list of grounds
   (recommended)** — the project has a stated position, honest answers cost nothing, and an
   identity can see what would count against them and answer it. Consequence accepted: the
   published grounds are also a description of what to avoid, so the pattern they describe can
   be adjusted; this is acceptable because no ground is automatic and each still needs a
   maintainer's written judgement.
2. **Request disclosure but keep the grounds unpublished** — less to work around.
   Consequence: a level can be lowered on reasoning the affected person cannot see or answer,
   which is the shape of an arbitrary judgement even when the judgement is right.
3. **Require disclosure as a condition of merge** — a stronger signal. Consequence:
   unenforceable, so it mostly produces dishonest answers, and it turns an honest disclosure
   into a liability.
4. **Say nothing** — no position to defend. Consequence: the trust model has no answer to the
   most common question about the submissions it exists to handle, and maintainers improvise
   one case at a time.

Default if no answer: none — blocks until answered; this defines when an adverse judgement may
be recorded about a person.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Name no external contributor and describe no real submission. Every illustration is
  invented.
- Add no code path that changes a tier. This brief is a stated position and a published list;
  the recording act already exists and is a human's.

## Task

1. Write the position and the closed downgrade list into the published model, stating
   explicitly that the use of automation is not a ground.
2. The contributor-facing paragraph and the template help text: what the disclosure answer is
   used for, and that it is requested rather than required.
3. State in the same place that the tier attaches to the operating human, so an agent
   operating under an account inherits that account's tier and gains no standing of its own.
4. The changelog fragment.

## Verify (executable — no prose-only DoD items)

This table gates PRESENCE and the boundary statements. Whether the position is the right one
is the review gate's and the human gate's judgement, not this table's.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -n -i 'operating human' docs/contributor-trust.md` | exit 0; at least one matching line | check |
| 2 | `grep -n -i 'is not a ground' docs/contributor-trust.md` | exit 0; at least one matching line (the use of automation is excluded explicitly, not by omission) | check +dereference |
| 3 | `grep -n -i 'requested' CONTRIBUTING.md` | exit 0; at least one matching line (disclosure is requested, not required) | check |
| 4 | `git -C . grep -n -i -e 'must disclose' -e 'required to disclose' -- CONTRIBUTING.md .github/PULL_REQUEST_TEMPLATE.md docs/contributor-trust.md` | exit 1; no matching line | check +mutation |
| 5 | `grep -n -c -i 'downgrade' docs/contributor-trust.md` | exit 0; output is `3` or more (each enumerated ground is stated, not summarised in one line) | check |
| 6 | `git -C . grep -n -i 'recorded human act' -- docs/contributor-trust.md` | exit 0; at least one matching line (a downgrade is written by a person, with a reason) | check +dereference |
| 7 | `git -C . grep -n -i -e downgrade -e demote -- tools statusgen` | exit 1; no matching line (no code path performs a tier change; the downgrade this brief defines is a human's recorded act) | check +mutation |
| 8 | `grep -n -i 'automated' .github/PULL_REQUEST_TEMPLATE.md && grep -n -i 'operating human' CONTRIBUTING.md && grep -n -i 'is not a ground' docs/contributor-trust.md` | exit 0; at least one matching line from each (the disclosure field, the contributor-facing explanation of what it is for, and the position it feeds all exist and connect) | check +flow |
| 9 | `statusgen --root . --consumers --brief contributor-trust/07` | exit 0; output does not contain `DISPROVED` | check |

Pre-mortem to detection map. "The published text excludes automation as a downgrade ground by
saying nothing about it, so a maintainer reasonably reads the bulk-submission ground as
covering it" is caught by row 2, which requires the exclusion to be explicit. "Disclosure
drifts into a requirement in the template's wording" is caught by rows 3 and 4. "The grounds
are collapsed into one vague sentence that could justify anything" is caught by row 5. "The
downgrade quietly becomes something a counter performs" is caught by rows 6 and 7. "The
grounds are the right shape but too broad in substance" — no row; the closed list's content is
exactly what the human gate is ruling on.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
