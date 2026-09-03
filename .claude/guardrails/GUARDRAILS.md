---
name: guardrails
description: The declared source for the guardrail rule blocks that more than one desk-role skill must state verbatim. Not a skill — a data file read by tools/skillslint. Edit a rule HERE, then run `make guardrail-sync`; never hand-edit a copy in a SKILL.md.
---

# Shared guardrails — the declared source

Four cross-cutting rules — insight-routing, escalation labels, git-push policy,
no-attribution — were restated four to five times across the desk-role skills
under `plugins/assay/skills/`, and had already drifted (the example lists, the
"file a bug where" clause, and the push-policy wording differed copy to copy).
This file is the "once" for the blocks listed below, and `tools/skillslint` is
the diff that keeps every other copy honest.

**Why the copies still exist at all.** These rules are load-bearing *while a loop
is running*: a stop-flag or push-policy check that a session has to go and fetch
from another file is a check that does not happen. So the answer is not a pointer
— it is **derivation**. The text is authored once, here; every copy in a SKILL.md
is regenerated from this file (`make guardrail-sync`) and byte-diffed against it
(`tools/skillslint`). A copy is never hand-maintained, so it cannot drift
silently — which is what the duplication was actually about. This is the same
shape as a compiled-config-plus-a-test-that-is-the-diff: one declared source, a
generator, and a check that is the diff.

**Three-state, never fail-open.** The checker reports `checked-clean`,
`checked-failed`, or `could-not-check`. A site it cannot read, a block whose
anchor it cannot find, and a declared site file that does not exist are all
`could-not-check` — reported as FAILURES, never as clean. "Nothing to compare"
is not a pass.

## What is NOT here (named, not silently capped)

- **worktree isolation** — stated as a block in one skill only (worker-desk),
  so it is not a duplicated rule.
- **hygiene-tick / stop-flag** — these are also stated by more than one skill and
  are candidates for the same treatment; they are out of scope for this pass,
  which covers only the four cross-cutting guardrails above. Adding them here is
  a follow-up, recorded rather than silently skipped.
- **intake-desk** — deliberately not a site for any of the four blocks. Its
  copies are shorter paraphrases folded into intake-specific rules; normalizing
  them is a follow-up wording pass.
- **pr-shepherd / insight-routing + escalation-labels** — the shepherd is a
  worker-side role, not a desk: it neither files insights as a loop output nor
  owns the escalation-label vocabulary (it uses `needs-decision` once, inside the
  security-gate carve-out, quoting the rule it is bound by rather than restating
  it). It IS a site for git-push-policy and no-attribution, which bind every role
  that pushes.
- **pr-review-desk / insight-routing** — pr-review-desk carries a one-line
  *pointer* to the insight-routing rule inside its discovery-routing table, not a
  restatement. A pointer needs no derivation, so it is not a declared site.

## Publication note

These skills are the public, adopter-facing Assay plugin bundle: the rules are
stated in generic terms ("the driver" / "the human driver" for the human at the
controls, "the project's own toolkit/methodology repo" for wherever process
insights are filed, "mutating cluster commands" for infra verbs). The canonical
text below is authored in that same generic voice — there is no separate scrubbed
twin here, so every site is compared byte-for-byte with no substitution.

## guardrail: insight-routing

File systemic/process insights as issues in the project's own toolkit/methodology
repo, not as commentary. The per-loop example lists that made the old copies wrap
differently are folded into one shared list — the examples were always
illustrative, not per-loop policy.

pr-review-desk is deliberately not a site (it carries a one-line pointer, not a
restatement). intake-desk: see "What is NOT here".

- site: plugins/assay/skills/the-desk/SKILL.md
- site: plugins/assay/skills/worker-desk/SKILL.md
- site: plugins/assay/skills/verify-desk/SKILL.md

```text
- **Insight-routing:** a systemic/process insight produced in passing (a wrap-up, a dispatch or drain
  note, an Evidence aside, a "this keeps recurring" observation) MUST also be filed as an issue in the
  project's own toolkit/methodology repo — commentary is not a register. Include the triggering
  evidence and affected loops. Repo-specific defects still go to that repo's own tracker (label `bug`).
```

## guardrail: escalation-labels

`question` / `help wanted` discipline. A bare label is unanswerable; the labeler
comments what it needs and from whom. intake-desk: see "What is NOT here".

- site: plugins/assay/skills/the-desk/SKILL.md
- site: plugins/assay/skills/worker-desk/SKILL.md
- site: plugins/assay/skills/verify-desk/SKILL.md

```text
- **Escalation labels:** any desk/loop may label a PR or issue `question` (needs an answer from the
  driver or a stronger-tier model — the item PARKS only when the fork is one-way; a reversible item proceeds on its
  stated default with the label riding on it) or `help wanted` (the desk hit its capability/authority edge). Both are
  GitHub default labels — they exist in every repo, no setup. Discipline: a bare label is unanswerable — the labeler
  MUST comment what it needs and from whom when labeling; whoever answers removes the label with their response. A
  `question` that matures into a formal decision fork promotes to `needs-decision` with the pros/cons template.
  Labeled items are WAITING-ON-INPUT: they join the human/escalation queue and are NOT orphans for the worker sweep.
```

## guardrail: git-push-policy

ONE policy, role-keyed. The old copies were not different policies — they were
different ROLES quoting the same policy from their own seat ("pushing to main is
gated" and "the verify desk lands its own work" are the same grants table). The
canonical text states the fleet-wide core: merge / workflows / cluster gates, the
branch-push + draft-PR standing authorization, and the verify-desk `main` grant.
Each skill keeps its role-specific grants and denials (what it may flip, file,
close, or land) as its own bullets directly below the block — those are per-desk
irreducibles, not drift. intake-desk: see "What is NOT here".

- site: plugins/assay/skills/the-desk/SKILL.md
- site: plugins/assay/skills/worker-desk/SKILL.md
- site: plugins/assay/skills/pr-review-desk/SKILL.md
- site: plugins/assay/skills/verify-desk/SKILL.md
- site: plugins/assay/skills/pr-shepherd/SKILL.md

```text
- **Git push policy (ONE policy, role-keyed):** MERGE IS ALWAYS the driver's, and nobody triggers
  workflows or runs mutating cluster commands without their go. **Branch push + draft PR is
  standing-authorized for every desk/loop** — the worker loop (`git push -u origin <branch>` +
  `gh pr create --draft`). **The verify desk lands its own work**: its Evidence + status flips commit
  straight to `main` as the project directs — no push-go is needed there and none should be waited
  for. Any `main` push not covered by a standing authorization is gated on the driver's explicit go;
  committing local work is always fine. A guard/hook-BLOCKED push is a STOP signal — never route the
  same write through another tool. Each desk's own grants and denials (what it may flip, file, close,
  or land) stay in its skill, directly below this block.
```

## guardrail: no-attribution

No AI-attribution stamps in any artifact. The short paraphrases ("No attribution
lines anywhere.") are replaced by the one full sentence. intake-desk: see "What
is NOT here".

- site: plugins/assay/skills/the-desk/SKILL.md
- site: plugins/assay/skills/worker-desk/SKILL.md
- site: plugins/assay/skills/pr-review-desk/SKILL.md
- site: plugins/assay/skills/verify-desk/SKILL.md
- site: plugins/assay/skills/pr-shepherd/SKILL.md

```text
- No attribution lines anywhere: no `Co-Authored-By`, no "Generated with …" in commits, PRs, issues,
  or comments.
```

## guardrail: default-forward-reversibility

The 2026-09-03 driver ruling: a desk must NOT stop and wait on an urgent but
REVERSIBLE decision. The human holds the merge gate, so a wrong guess is caught
before it lands; asking for a go-ahead the gate makes redundant just makes the
driver decide twice. Every desk-role loop default-forwards on the reversible set
— author, dispatch, open the DRAFT PR, make the best-guess call, and NOTIFY — and
stops only for the genuinely one-way / outside-the-gate set the block fixes.

intake-desk IS a site for this block: it is a new, neutral rule authored once here
in the same generic voice as every copy, so there is no paraphrase carve-out for
it (unlike the four older blocks). pr-shepherd is NOT a site — it is a worker-side
role that quotes the security-gate rule it is bound by rather than owning the
escalation vocabulary; the desks own the default-forward call.

- site: plugins/assay/skills/the-desk/SKILL.md
- site: plugins/assay/skills/worker-desk/SKILL.md
- site: plugins/assay/skills/pr-review-desk/SKILL.md
- site: plugins/assay/skills/verify-desk/SKILL.md
- site: plugins/assay/skills/intake-desk/SKILL.md

```text
- **Reversibility test — default-forward on anything a human-held gate still catches:** before
  parking an item on the driver, ask ONE question: *is a wrong guess here caught by a gate the
  driver still controls — a draft PR awaiting merge, a filed issue awaiting close, a flip CI or a
  human must still make?* **Yes → default-forward.** Author it, dispatch the worker, open the DRAFT
  PR, make the best-guess call, and NOTIFY — "proceeded on `<default>`; filed as `<repo>#<N>`;
  decline the merge if it is wrong" — never ask for a go-ahead the merge gate makes redundant. The
  `needs-decision` / `question` issue is still filed, naming the default taken, but the ITEM does
  not park on it. Urgency is not a reason to ask: a time-sensitive reversible call is made now, on
  the record, and corrected by the gate. **No → STOP and wait for the human.** A wrong guess that
  lands irreversibly or reaches outside the gate is caught by nobody declining a merge. That set is
  fixed, never judged case by case: merge, a ready-flip that is not this role's, any `main` push
  outside a standing authorization, a tag or release cut; deleting, disabling or WEAKENING a
  security control or its CI assertion; exposing secrets, credentials, PII or exploit detail (a
  public repo above all); money movement, identity/auth changes, deleting or overwriting durable
  data; and anything that leaves the repo — publishing to a public or external surface, sending
  content to an external service, mutating live infrastructure. A guard or tool REFUSAL is a STOP on
  either side of the test — the test never routes around one.
```
