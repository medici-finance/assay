---
brief: assay:assay:desk-tools:60
title: "Operate is human, and the gate ladder — what an agent fleet owns, what a human owns, and the written condition under which a human gate may ever widen (every threshold recorded as not set)"
why: >-
  Nothing in the methodology states which phases of a change an agent fleet owns and which a
  human keeps, so "agents run the delivery lifecycle" invites the pager objection every time it
  is said. And nothing states the condition under which a human gate may ever relax for a class
  of change, so any future "let docs-only pull requests merge without review" is a ruling with
  nothing to cite and nothing to measure against. Writing the ladder now — classes, the gates
  each class must keep, and the metric that would one day justify a widening, with every
  threshold recorded as not set — means nothing relaxes, a claimed relaxation is a reviewable
  falsehood, and the ladder only ever tightens until a human rules a number for one rung.
wave: 1
depends: []
unblocks: []
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: >-
  A policy document adopters copy: it states what a model-gated identity never holds (the
  pager, incident response, irreversible mutation, deploy authority) and names, per class of
  change, the metric that would later justify relaxing a gate. `customer: yes` because it is a
  published commitment about where the human stays in the loop. The human at sign-off confirms
  that every rung's threshold reads `not set`, that no rung names a ruler, and that nothing in
  the change alters any gate's behaviour.
decision-trigger: creation
issues: []
schema: brief-v2
version: 1
authored: 2026-10-27 by a desk-dispatched authoring session (public half of a downstream house's plan item whose remaining deliverables live in this repo)
sources:
  - "a downstream house's private plan item (stating the fleet/human boundary and a gate ladder): its remaining public deliverables are this brief. Not cited by path or number: a public brief cannot anchor on a private one."
  - "spec/lifecycle-v1.md section 9 — deploy authority is already human-only (9.3 item 3); nothing states the rest of the operate boundary or any widening condition"
  - "plugins/assay/skills/pr-review-desk/SKILL.md § Finding-class calibration — the finding-class register this brief adds one row to"
  - "docs/streams/desk-tools/README.md — briefs 17–19, where the human belongs in a public-repo loop"
  - "freshness-checked 2026-10-27 @ 53e8a16da (origin/main) — no docs/gate-ladder.md; 0 matches for pager or 'claims a relaxation' in the spec or the review skill"
  - "docs/streams/issue-flow/rulings.md R-8 and tools/desk/internal/deskkit/autolane.go — the one widening condition already written down: the auto-approve lane's enactment gate (ships unsigned, so the lane is enacted for no area); re-checked 2026-10-28 @ 5ced727d5"
exec-tier: any
domain: complicated
outcome: none
id: 49524710-280e-46af-b473-5a3526411fc2
---

# Brief 60 — Operate is human, and the gate ladder

## Context

files:
- `spec/lifecycle-v1.md` — one new subsection, `### 9.4 Operate is human`, after 9.3; no existing section is renumbered
- `docs/gate-ladder.md` (planned) — NEW
- `plugins/assay/skills/pr-review-desk/SKILL.md` — one finding-class register row plus its definition
- **add** — `changelog/<branch-slug>.md` — one fragment

facts:
- Section 9 of `spec/lifecycle-v1.md` already requires a named human deploy authority per
  environment and forbids a model-gated identity from holding it (9.3 item 3). The new 9.4
  extends that one boundary; it does not restate 9.1–9.3.
- The four change classes, each a single lowercase hyphenated token: `docs-only`,
  `board-only`, `tool-code`, `core-system`. `core-system` is a change touching a surface the
  project layer declares core (for example a funds path, an identity chain, a trust
  boundary); the ladder names the class, the adopter names the surfaces.
- The three metrics, each stated as a fraction (numerator / denominator over a stated window,
  never a bare percentage, matching the finding-class reversal rate's convention):
  zero-human-touch merge rate, reversal rate (merged changes of the class later reverted or
  fixed forward), and control-failure count (escaped defects a gate of the class should have
  caught). A threshold, once set, names in its cell the metric it bounds and the window it is
  measured over; a bare number is not a set threshold.
- The rung row is a fixed contract that downstream tools may parse: exactly one pipe-table
  row per class, of the shape
  `| <class> | <gates required> | <metrics> | threshold: not set | ruled-by: — | date: — |`.
  The first cell is the bare class token; exactly one cell starts `threshold: `. The literal
  `threshold: ` appears nowhere in the document except in rung rows, so counting it counts
  rungs. Setting a threshold is a one-line edit that is itself a human ruling, recorded in the
  same row (`ruled-by:` names a human, `date:` the ruling's date). A model-gated identity never
  fills `ruled-by:`.
- The finding-class register lives in the review role's skill. desk-tools/53 (open) renames
  that role from `pr-review-desk` to `review-desk`, keeping a thin deprecated
  `pr-review-desk` redirect for at least one release; if it lands first, Task 3 edits the
  renamed file. Verify rows 4 and 5 match the register under either name.
- One widening condition is already written down: the auto-approve lane (`deskautolane`),
  whose enactment gate is `docs/streams/issue-flow/rulings.md` R-8 — a human acceptance
  recorded on a configured thread, scoped by area. R-8 ships unsigned, so the lane is enacted
  for no area today. As worded, 9.4's "MUST NOT relax … unless a rung carries a threshold"
  would make that lane non-conforming once enacted; the ruling below settles how the two
  relate before 9.4 is written.
- Nothing in this brief changes any gate's behaviour. No tool reads the ladder to relax
  anything; wiring a set threshold into a gate is a separate, human-gated change.

single-point-of-failure: the ladder document is a policy, not a control. The control that
depends on it is the review lane's "claims a relaxation" finding class, which blocks any change
that claims or enables a widening the ladder does not back. The layer behind it: no gate reads
the ladder, so a wrongly edited rung widens nothing on its own, and the human merge still stands
in front of every change.

layering: no new component. One spec subsection, one reference document incorporated by
reference from it (as `docs/deploy-model.md` is from 9.1), and one register row in an existing
skill. Task 1–3; Verify rows 1–5.

design-fit:
  owner: "spec/lifecycle-v1.md — the lifecycle's normative owner; docs/gate-ladder.md is incorporated by reference from section 9.4"
  contract: none — a published policy and one review finding class; no semantic-owner row is touched.
  retires: []
  weight: "verbs 0, flags 0, refusals 0; rule-text lines: about +15 spec lines, one new reference document (about 60 lines), +1 register row and about 6 lines of finding-class definition in the review skill"
  why-add: "No existing owner states the fleet/human boundary beyond deploy authority, or any widening condition; section 9 is the narrowest owner that can carry it. Removal considered: leaving every widening a case-by-case ruling (option 3 below), kept as an option for the human rather than chosen by the author."

## Human decision

There is no written statement of what an agent fleet owns versus what a human owns, and no
written condition under which a human review gate could ever be relaxed for a class of change.
The proposal writes both: a short normative statement that humans own the pager, incident
response, irreversible changes and deploy authority, while agents may own authoring, merge
preparation, verification and release preparation; and a ladder listing, per class of change
(documentation only, board only, tool code, core system), the gates that must exist and the
measurements that would later justify relaxing one, with every threshold left explicitly unset.

Options:
1. **Write the ladder with every threshold unset** (recommended) — nothing relaxes; each
   number comes later as its own human ruling, recorded in that rung's row. A claimed
   relaxation before then is a review finding.
2. **Write the ladder with thresholds now** — needs the numbers decided first; the first rung
   could relax as soon as its metric is met.
3. **No ladder** — widening stays a case-by-case ruling with no stated condition.

Whichever option is ruled, the ruling also settles how 9.4 treats the auto-approve lane (see
facts): **(a)** recommended — 9.4 also counts a widening enacted under a recorded human
acceptance of R-8's form as ruled, for the area that acceptance names, and the ladder points to
R-8 rather than restating it; **(b)** 9.4 admits only ladder rungs, so enacting the auto-approve
lane for any area first needs a ruled rung for its class.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not implement before the decision above is ruled; option 1 is what the Task below builds.

## Task

1. **Spec subsection.** Add `### 9.4 Operate is human` to `spec/lifecycle-v1.md` after 9.3.
   Short and normative:
   - 9.4 applies whether or not an implementation models deployment (section 9's preamble
     scopes 9.1–9.3 to deployment; 9.4 says it is not so scoped).
   - An agent fleet MAY own authoring, merge preparation, verification and release
     preparation under the gates of sections 2 and 4. Merge preparation stops short of the
     merge, and release preparation short of the tag or release cut.
   - A conforming implementation MUST NOT assign the pager, incident response, irreversible
     mutation or deploy authority to a model-gated identity. Deploy authority is 9.3 item 3,
     cited, not restated.
   - A conforming implementation MUST NOT relax a human gate for a class of change unless
     that class's rung in the ladder document carries a human-ruled threshold and the
     measured metric meets it. 9.4 links the ladder as `../docs/gate-ladder.md`, relative to
     `spec/`. How this clause treats the auto-approve lane is as ruled under Human decision.
   - A claim that a gate has been relaxed, where no such rung backs it, is a defect in the
     claim, not a relaxation.
2. **`docs/gate-ladder.md` (planned).** State the purpose in one paragraph, then the four classes from
   facts with a one-line definition each. Then the rung table: a header row, a separator row,
   and exactly one rung row per class in the facts format.
   - **Gates required** records the gates section 4 already imposes on that class today. This
     brief adds no gate. No rung relaxes the security review lane, and no threshold reaches
     weakening a security control or its CI assertion, exposing secrets, or an identity or
     auth change.
   - **Metrics** names the three metrics.
   - Every rung reads `threshold: not set | ruled-by: — | date: —`.
   - A closing paragraph states the edit rule: a threshold is set only by a human ruling
     recorded in the row. It also states the parse contract: one row per class, bare class
     token first, one `threshold:` cell, and a set value names its metric and window. Write
     that paragraph without the literal string `threshold: `.
3. **Finding class.** In the review skill's finding-class register, add the row
   `| claims a relaxation | blocking |`. Under the register, add a short definition:
   - The class fires when a change's diff, body or published text claims, enables or
     documents a widening of a human gate that no rung in the ladder document backs with a
     set threshold.
   - Extend the demotion rule's existing exception, which today covers `security` and
     `public-surface / leak`, to this class as well. It never demotes on reversal rate
     without a recorded human ruling.
4. **Derived bundles.** Regenerate any harness bundle the repo's drift check derives from
   `plugins/assay/`, so that Verify row 7 exits 0.
5. **Changelog.** Add one fragment under `changelog/`, named after the branch.

## Verify (executable — no prose-only DoD items)

Run every row from a `medici-finance/assay` checkout. Rows 1–5 are FAIL-FIRST on the
authoring base (no ladder, no 9.4, no register row).

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check +flow | `test "$(grep -cF 'threshold: not set' docs/gate-ladder.md)" -eq 4 && test "$(grep -cF 'threshold: ' docs/gate-ladder.md)" -eq 4 && test "$(grep -cF 'ruled-by: — ' docs/gate-ladder.md)" -eq 4` | exit 0 — four rungs, every one unset and unruled, and no other text carries the threshold cell's literal |
| 2 | check +dereference | `n=0; for c in docs-only board-only tool-code core-system; do test "$(grep -c "^\| $c \| .*\| threshold: " docs/gate-ladder.md)" -eq 1 && n=$((n+1)); done; test "$n" -eq 4` | exit 0 — for each class, exactly one row has the bare class token as its first cell and also carries the threshold cell |
| 3 | check +dereference | `sed -n '/^### 9.4 Operate is human/,/^## 10\./p' spec/lifecycle-v1.md > "${TMPDIR:-/tmp}/op94.md" && for p in pager 'incident response' 'irreversible mutation' 'deploy authority' '../docs/gate-ladder.md'; do grep -qF "$p" "${TMPDIR:-/tmp}/op94.md" \|\| exit 1; done; test -f docs/gate-ladder.md` | exit 0 — 9.4 names all four human-owned powers and links a ladder that exists |
| 4 | check | `cat plugins/assay/skills/*review-desk/SKILL.md 2>/dev/null \| grep -cF 'claims a relaxation'` | ≥ 2 (the register row and its definition, under either role name, see facts) |
| 5 | check +dereference | `grep -rlF '\| claims a relaxation \| blocking \|' plugins/assay/skills/ \| grep -cF 'review-desk/SKILL.md'` | ≥ 1 — the class is registered as `blocking` in the review role's skill (either role name, see facts) |
| 6 | check:ci | `cd tools/skillslint && go run . --root ../.. > "${TMPDIR:-/tmp}/sl60.out" 2>&1 && ! grep -qF ': FAIL' "${TMPDIR:-/tmp}/sl60.out" && grep -cF 'HOUSE-VALUES: PASS' "${TMPDIR:-/tmp}/sl60.out"` | `1` — skill lint exits 0, prints no `: FAIL` line, and the house-value check passes over the edited skill |
| 7 | check:ci | `cd tools/desk && go run ./cmd/deskinstall --harness cursor --forge github --repo ../.. --bundle ../../plugins/assay --check 2>&1 \| grep -cF 'deskinstall --harness cursor --check: clean'` | `1` — the derived bundle matches the edited skill |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: human (from frontmatter — a risk answer is yes). Human gate is MANDATORY. Reviewer records verdict + date in the stream README table.
