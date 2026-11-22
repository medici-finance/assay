---
brief: assay:assay:desk-tools:66
title: "Let the coordinator desk waive the changelog check on briefs-only PRs, and make the gate read who applied the label"
why: >-
  A PR that changes only briefs (Verify-row amendments, brief authoring, stream README edits)
  has no notable change to record, yet the required changelog check stays red until a maintainer
  applies the changelog:skip label by hand. The check honors the label by its presence alone and
  never reads who applied it, so the workflow header's claim that the label is maintainer-only
  is not enforced: any identity that can write labels can waive the check. Two skill bodies say
  the desk may apply the label, which contradicts that header. This brief puts the choice of rule
  to the human on the decision issue as three options, makes the gate enforce the ruled one, and
  carries the same setting into the adopter install so a fresh install gets it.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  It changes a security and authorization control: who may waive a required check (sensitive-data
  is answered yes for that reason, as the trust-boundary briefs answer it). Option 2 would grant an
  automation identity label-write and let its label count, which the workflow header says this
  repository refuses. Option 1 would give briefs-only PRs from named authors a path waiver, the
  same shape as the Evidence lane. Either way the adopter install would carry the ruled rule to
  every adopter. Only the human can weigh the saved maintainer clicks against it.
decision-trigger: creation
issues: [7010]
outcome: none
schema: brief-v2
version: 1
authored: 2026-11-18 by a worker-desk authoring session (design proposal; no ruling recorded yet); revised 2026-11-22 after round-1 review
sources:
  - "medici-finance/assay#7010 (the request, and the decision issue: the coordinator desk cannot waive the changelog check on briefs-only PRs, and the gate never checks who applied the label; its triage comment numbers the three options this brief uses)"
  - ".github/workflows/changelog-check.yml header: the label path stays the maintainer's; the Evidence lane is decided in-workflow because handing label-write to an automation App is the widening this repository refuses"
  - "plugins/assay/skills/worker-desk/SKILL.md (a non-notable PR carries the label, 'applied by the desk or a human') and plugins/assay/skills/author-brief/SKILL.md (a non-notable brief 'leaves the changelog:skip waiver to the desk or a human')"
  - "changelog/README.md, section Automatic board reconciliation (the existing narrow classification exception, with an exact-identity and whole-diff check)"
  - "docs/adopting-assay.md, the install step that sets Actions variables and the scaffolded statusgen workflow's roster variables"
  - "freshness-checked 2026-11-22 @ 236854c9d (origin/main)"
exec-tier: strong
exec-tier-why: "It changes who can satisfy a required check, must fail closed when the actor cannot be read, and must keep the live workflow, its staged copy, the skill bodies and the adopter install in agreement."
domain: complicated
consumers:
  - ".github/workflows/changelog-check.yml: follow-up desk-tools/66 (this brief; the label path reads the applier, and under options 1 and 2 the changed-file classification, before it counts)"
  - "tools/changelog/changelog-check.yml: follow-up desk-tools/66 (this brief; the staged copy carries the same steps, so the two do not drift)"
  - "tools/changelog/README.md: follow-up desk-tools/66 (this brief; states who may waive and for which PRs)"
  - "changelog/README.md: follow-up desk-tools/66 (this brief; states who may waive and for which PRs)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desk-tools/66 (this brief; the label sentence states the ruled rule)"
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up desk-tools/66 (this brief; the label sentence states the ruled rule)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: follow-up desk-tools/66 (this brief; the label sentence states the ruled rule)"
  - "plugins/assay/skills/pr-review-desk/references/verdict-format.md: follow-up desk-tools/66 (this brief; the clause that says a human applies the label names the applier rule)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: follow-up desk-tools/66 (this brief; the changelog clause states the ruled rule)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md: follow-up desk-tools/66 (this brief; the alternative kit quotes the same changelog clause verbatim, so it changes in step)"
  - ".cursor/skills/: follow-up desk-tools/66 (this brief; generated copies of the skill bodies, regenerated from the sources above)"
  - "docs/adopting-assay.md: follow-up desk-tools/66 (this brief; the install step states the rule and, under options 1 and 2, lists the new Actions variable beside the roster variables)"
id: 78505216-2312-490f-a7b2-a8412bbc55cd
---

# Brief 66 — Let the coordinator desk waive the changelog check on briefs-only PRs

## Dependencies
- No typed edge. The workflow, `check.sh`, the skill bodies and the install doc all exist on main.
- I checked the head of the work by reading `.github/workflows/changelog-check.yml` on main.
  `SKIP` is `contains(github.event.pull_request.labels.*.name, 'changelog:skip') || steps.evidence.outputs.evidence == 'true'`.
  The first half is label presence from the event payload, with no applier. That is the gap, so
  the workflow change comes first and the wording changes follow it.

## Context

files:
- `.github/workflows/changelog-check.yml` — the `SKIP` expression and the Evidence classification step it already has
- `tools/changelog/changelog-check.yml` — the staged copy of the same workflow
- `tools/changelog/check.sh`, `tools/changelog/check_test.sh` — the decision and its offline tests; `check.sh` is read, not changed
- `changelog/README.md`, `tools/changelog/README.md` — the convention text
- `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/author-brief/SKILL.md`, `plugins/assay/skills/pr-shepherd/SKILL.md`, `plugins/assay/skills/pr-review-desk/references/verdict-format.md` — the label wording
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md`, `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md` — the worker kit's changelog clause, in the default and the alternative kit
- `.cursor/skills/` — generated copies of the skill bodies above
- `docs/adopting-assay.md` — the install step for Actions variables
- **add** — `tools/changelog/labeler_test.sh` (planned), `tools/changelog/labeler-mutations.json` (planned)
- **add** — no changelog fragment: the task section for this authoring PR waives it; the implementation PR adds one

facts:
- The workflow header calls the label a maintainer act. Nothing enforces it. The label is read
  from the event payload, which names the labels on the PR and not who applied them.
- Who applied a label is on the forge: the issue events read returns each `labeled` event with the
  actor and the label name. The workflow holds `contents: read` and `pull-requests: read` (the
  second was added for the Evidence classify step). Whether that scope reads those events, and
  whether it reads a user's collaborator permission, are things step 1 checks, not assumes.
- The workflow already has a second SKIP condition computed in the workflow, for PRs authored by
  the verify desk's App that change only `docs/streams/`. Its design note says a label cannot be
  handed to an automation App without widening the waiver, so it decides from PR metadata
  instead. That lane covers Evidence PRs and nothing else, and a mixed PR is excluded.
- The board-reconciliation exception does the same for one writer. It binds an exact identity and
  branch, then compares the whole diff, and falls back to the ordinary requirement when it cannot
  read the event.
- The staged copy already differs from the live workflow well past its banner: `runs-on`, the
  concurrency block, `permissions`, the SKIP expression (no Evidence half) and the whole Evidence
  classify step. Whole-body equality is therefore not a usable check today. Verify rows 10 and 11
  compare only the steps this brief adds, and the wider drift is recorded here as out of scope.
- The label sentence is worded differently in each place that carries it. As they stand (read with
  whitespace collapsed, since two of them wrap across a line):
  - `plugins/assay/skills/worker-desk/SKILL.md`: "applied by the desk or a human"
  - `plugins/assay/skills/author-brief/SKILL.md`: "leaves the `changelog:skip` waiver to the desk or a human"
  - `plugins/assay/skills/pr-shepherd/SKILL.md`: "that label is the desk's or a human's"
  - `plugins/assay/skills/pr-review-desk/references/verdict-format.md`: "a human applies `changelog:skip`"
  - `tools/desk/cmd/deskdispatch/references/worker-prompt.md` and `worker-prompt-objective.md`: "label the desk or a human applies"
  - the `.cursor/skills/` copies of the first four
  The sources disagree with each other, not only with the header. The verdict-format clause is
  true today if a human stays the only applier, but it is reworded anyway so it names the rule the
  gate enforces.
- #6993 and #6994, the two PRs that waited red on `changelog` when the request was filed, have
  since merged after a maintainer applied the label; they show the cost, not a standing queue.
- The scaffolded statusgen workflow already receives roster values as Actions variables. A fresh
  install sets variables in its install step, so a new variable reaches adopters there and not
  through a copied workflow file.

design-fit:
  owner: .github/workflows/changelog-check.yml decides; tools/changelog/check.sh stays the one script that reads SKIP
  contract: none
  retires: [the unenforced "maintainer-only" sentence in the workflow header]
  weight: "verbs 0, flags 0, refusals 2 (applier not allowed, files not briefs-only; the second only under options 1 and 2), Actions variables 1 under options 1 and 2 and 0 under option 3"
  why-add: "The gate replaces an unenforced claim with a check that reads the applier. Without it the label stays open to any writer or stays a manual maintainer click on every briefs-only PR."

## Human decision

Decision issue: #7010 (this repository). The human records the ruling there as their own comment,
`Ruling: 1`, `Ruling: 2` or `Ruling: 3`. This brief uses the same three numbers as the triage
comment on #7010, so a ruling there means the same thing here. The request as filed asks for the
coordinator desk to apply the label, which is option 2 below.

Every PR that changes only briefs (a Verify-row amendment, a new brief, a stream README edit)
fails the required changelog check, because such a PR has no release note to write. Today a
maintainer applies the `changelog:skip` label by hand to turn it green. Separately, the check
counts the label whoever applied it, so the rule that only a maintainer waives is a sentence in a
file header, not something the gate enforces. In every option the gate starts reading who applied
the latest label, and the skill and kit sentences listed under facts are corrected to match.

Options, numbered as on #7010:
1. **Automatic path waiver, no label.** A PR authored by an identity named in the Actions variable
   `ASSAY_BRIEFS_ONLY_ACTORS` (the coordinator desk's App, and whichever other authors the human
   names) passes the check, with a notice in the log, when every changed file is a brief or a
   stream README. No App gets label-write, and the label stays maintainer-only, now enforced. This
   is the same shape as the Evidence lane. The threat it opens: a wrong file-list test could waive
   a PR that changes code, and a brief can carry Verify-row commands that verifier tooling later
   executes. The waiver covers only the changelog check and leaves review in place.
2. **Desk App on the label allow-list.** The gate counts the label only when its applier holds the
   maintain or admin role, or is an identity named in `ASSAY_BRIEFS_ONLY_ACTORS`, and for that
   named identity only when the PR is briefs-only. This is the request as filed. The threat it
   opens: the desk's App gets label-write, and a compromised or mistaken desk could waive a
   notable change if the briefs-only test were wrong. The test is a file-list read, so it is cheap
   to get right and to pin.
3. **Status quo plus enforcement.** The label stays maintainer-only and the gate enforces it:
   only an applier holding the maintain or admin role counts. Briefs-only PRs keep waiting for the
   maintainer. No variable is added. It closes the gap in the header and leaves the cost in the
   request.

Recommendation: option 1, which is the recommended default in the triage comment on #7010.
Option 3 is the safer fallback if the human will not give briefs-only PRs a path waiver.
Default if no answer: nothing is built and the status quo stays, after 2026-12-02. Silence changes
nothing.

## Ground rules
- This brief records no ruling. Implement only the option the human records on the decision issue
  (#7010), under that issue's numbering, and nothing from the others.
- The Verify table below names, in each row's Expect cell, the options the row applies to. The
  ruling's number alone decides which rows apply. The verifier records a row for another option as
  not applicable and cites the ruling; the implementer never edits, adds or drops a row to fit.
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Fail closed. When the applier, the applier's role or the file list cannot be read, the label (or
  the path waiver) does not count and the check stays in force, as the Evidence lane already
  does. Never treat could-not-check as allowed.
- The absolute `## Unreleased` guard in `check.sh` is not touched. The waiver never excuses it.
- The workflow file needs a workflows-capable identity, so the implementation PR may stage the
  change and hand the promotion to the human, as the staged copy header describes.
- Never write a credential-shaped literal into a test. Use fixed fake logins and fake ids.

## Task
All options (1, 2 and 3):
1. Probe first, with the workflow's own scope, on a scratch PR in a test repository. Record
   whether `pull-requests: read` returns (a) the login and numeric id of the actor of a `labeled`
   event on the issue events read, and (b) a named user's role on the collaborator permission read.
   Under options 1 and 2 also record (c) the PR file-list read, which the existing classify step
   already uses. If any read is not available, name the narrowest added scope and put it on #7010
   before any code. Do not widen the workflow's permissions on a guess.
2. Add a workflow step, `Classify - who applied changelog:skip`, placed before the gate step. It
   finds the latest `labeled` event for `changelog:skip` that no later `unlabeled` event removed,
   and reads that actor's login and id. It reads that actor's role from the collaborator
   permission endpoint, by the id the event gave. A missing event, an unreadable response, an
   unreadable role or a login that is not a plain identity outputs not-proven.
3. Build the `SKIP` input from the label AND an applier the ruled rule allows, as the Evidence
   lane's output already joins it. The applier rule: the user holds the maintain or admin role.
   `check.sh` still receives a plain true or false, and its Unreleased guard still runs first.
   Mirror the step and the `SKIP` change in `tools/changelog/changelog-check.yml`.
4. Add `tools/changelog/labeler_test.sh` (planned) with offline cases, one name per case, and run
   it from the existing changelog suite. Cases for every option: `maintainer-label` (green),
   `non-maintainer-label` (red), `unlisted-app-label` (red: an App that is not a maintainer and is
   not named by the ruled rule), `unreadable-event` (red) and `unreadable-role` (red).
5. Add `tools/changelog/labeler-mutations.json` (planned). Mutants for every option: drop the
   applier check, and treat an unreadable event or role as allowed. Each is paired with the case
   that catches it.
6. State the ruled rule once in each file listed under consumers, regenerate `.cursor/skills/`, and
   add to `docs/adopting-assay.md`, beside the roster variables, what the gate now requires of the
   applier and what an adopter must set. Rewrite each sentence listed under facts so none of the
   old wordings survives, including the verdict-format clause.
7. Record the ruling as the human's own comment on the decision issue, in the README row.

Option 1 only:
8. Add the Actions variable `ASSAY_BRIEFS_ONLY_ACTORS`: comma-separated entries, each
   `<account id>:<login>` with the exact login including any `[bot]` suffix. The gate matches
   the PR author on the id and requires the login to match as well, so a renamed or reclaimed login
   does not match. An unset variable names nobody.
9. Add a step, `Classify - is this a briefs-only PR`, before the gate step. The PR passes, with a
   notice naming the lane in the log, only when its author is an entry of the variable and every
   changed path matches `docs/streams/<stream>/brief-*.md` or `docs/streams/<stream>/README.md`,
   and none is a generated single-writer file or under `.github/`. A path outside that set, an
   unset variable or a file list that cannot be read leaves the gate in force. Add the cases
   `path-waiver-desk-briefs` (green), `path-waiver-not-briefs-only`, `path-waiver-workflow-file`,
   `path-waiver-other-author`, `path-waiver-unset` and `path-waiver-unreadable-files` (all red).
   Add mutants that widen the path set to all of `docs/` and that skip the author binding.

Option 2 only:
8. Add the Actions variable `ASSAY_BRIEFS_ONLY_ACTORS` in the same entry form as under option 1.
   For an applier whose id and login match an entry, the label counts only on a briefs-only PR, as
   defined in the option 1 step 9 path test; any other applier that is not a maintainer does not
   count. An unset variable names nobody.
9. Add a step, `Classify - is this a briefs-only PR`, with the path test of option 1 step 9. Add
   the cases `desk-label-briefs` (green), `desk-label-not-briefs-only`,
   `desk-label-workflow-file`, `desk-label-unset` and `desk-label-unreadable-files` (all red).
   Add mutants that widen the path set to all of `docs/` and that skip the briefs-only test for
   the named applier.

The step and variable names above are planned. Option 3 adds no variable and no second step.

## Verify (executable — no prose-only DoD items)
The risk answer is yes, so every row is fail-first: each names the red it shows at the base commit
(the parent of the delivering merge) and the green it shows after delivery. The verifier records
each with `verifyrun --fail-first` against that parent. Rows 2, 3, 5 and 7 are negative: they pass
only when the check stays red. Each Expect cell opens with the options the row applies to.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh maintainer-label` | Applies under options 1, 2 and 3. Exit 0. Red at base: the script does not exist. Green after delivery: a label applied by a maintain or admin user greens the check |
| 2 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh non-maintainer-label unlisted-app-label` | Applies under options 1, 2 and 3. Exit 0 only when the check stays RED in both cases. Red at base: the script does not exist (and the old gate would green on the label alone). Green after delivery: a label from a user without the role, and from an App the ruled rule does not name, stays red |
| 3 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh unreadable-event unreadable-role` | Applies under options 1, 2 and 3. Exit 0 only when the check stays RED in both cases. Red at base: the script does not exist. Green after delivery: an applier or role that cannot be read is could-not-check and the label does not count |
| 4 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh path-waiver-desk-briefs` | Applies under option 1 only; not applicable under options 2 and 3. Exit 0. Red at base: the script does not exist. Green after delivery: a briefs-only PR from a named author greens with no label |
| 5 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh path-waiver-not-briefs-only path-waiver-workflow-file path-waiver-other-author path-waiver-unset path-waiver-unreadable-files` | Applies under option 1 only; not applicable under options 2 and 3. Exit 0 only when the check stays RED in all five cases. Red at base: the script does not exist. Green after delivery: a PR that also changes code or a workflow file, an author who is not named, an unset variable and an unreadable file list each leave the gate in force |
| 6 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh desk-label-briefs` | Applies under option 2 only; not applicable under options 1 and 3. Exit 0. Red at base: the script does not exist. Green after delivery: the named App's label on a briefs-only PR greens the check |
| 7 | check:ci +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh desk-label-not-briefs-only desk-label-workflow-file desk-label-unset desk-label-unreadable-files` | Applies under option 2 only; not applicable under options 1 and 3. Exit 0 only when the check stays RED in all four cases. Red at base: the script does not exist (and the old gate would green on the label alone). Green after delivery: the named App's label on a PR that also changes code or a workflow file, with the variable unset, or with an unreadable file list, stays red |
| 8 | check:ci +neighbour | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/check_test.sh && bash tools/changelog/reconcile_test.sh` | Applies under options 1, 2 and 3. Exit 0. Red at base: the first test cannot pass because the script does not exist. Green after delivery: the existing fragment, proxy, Unreleased and board-reconciliation cases still pass |
| 9 | check:ci +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && test -f tools/changelog/labeler-mutations.json && cd tools/desk && go run ./cmd/muhar -spec ../changelog/labeler-mutations.json > "$O" 2>&1 && grep -e 'Totals: [2-9] caught, 0 NOT CAUGHT, 0 could-not-mutate[.]' -e 'Totals: [1-9][0-9][0-9]* caught, 0 NOT CAUGHT, 0 could-not-mutate[.]' "$O"` | Applies under options 1, 2 and 3. Exit 0. Red at base: no mutation spec. Green after delivery: every mutant is caught and none is left uncaught or unmutated. The spec holds 2 mutants under option 3 and 4 under options 1 and 2, each paired with the case that catches it; the review counts them, since the row's pattern only floors the count |
| 10 | check +dereference | `s() { awk -v k="$2" 'index($0, "- name: " k) {p=1; print; next} p && /^      - name:/ {exit} p' "$1"; }; k='Classify - who applied changelog:skip'; a=$(s .github/workflows/changelog-check.yml "$k") && b=$(s tools/changelog/changelog-check.yml "$k") && test -n "$a" && test "$a" = "$b"` | Applies under options 1, 2 and 3. Exit 0. Red at base: neither copy has the step, so the extract is empty. Green after delivery: the applier step is byte-identical in the live workflow and the staged copy. Only this step is compared; the rest of the two bodies already differs (see facts) |
| 11 | check +dereference | `s() { awk -v k="$2" 'index($0, "- name: " k) {p=1; print; next} p && /^      - name:/ {exit} p' "$1"; }; k='Classify - is this a briefs-only PR'; a=$(s .github/workflows/changelog-check.yml "$k") && b=$(s tools/changelog/changelog-check.yml "$k") && test -n "$a" && test "$a" = "$b"` | Applies under options 1 and 2; not applicable under option 3. Exit 0. Red at base: neither copy has the step. Green after delivery: the briefs-only step is byte-identical in both copies |
| 12 | check +dereference | `t=$(mktemp) && trap 'rm -f "$t"' 0 && for f in plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/author-brief/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/pr-review-desk/references/verdict-format.md tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md .cursor/skills/worker-desk/SKILL.md .cursor/skills/author-brief/SKILL.md .cursor/skills/pr-shepherd/SKILL.md .cursor/skills/pr-review-desk/references/verdict-format.md; do if ! test -f "$f"; then exit 1; fi; tr -s '[:space:]' ' ' < "$f" > "$t"; if grep -q -e 'applied by the desk or a human' -e 'leaves the .changelog:skip. waiver to the desk or a human' -e 'that label is the desk.s or a human.s' -e 'a human applies .changelog:skip.' -e 'label the desk or a human applies' "$t"; then echo "old rule in $f"; exit 1; fi; done` | Applies under options 1, 2 and 3. Exit 0. Red at base: every one of the ten files still carries its old sentence, listed under facts. Green after delivery: none of the five old wordings survives in any file, read with whitespace collapsed so a line wrap cannot hide one. The row checks that the old text is gone; the review reads that each file now states the ruled rule |
| 13 | check +flow | `for f in docs/adopting-assay.md .github/workflows/changelog-check.yml tools/changelog/changelog-check.yml; do if ! grep -q -F -e 'ASSAY_BRIEFS_ONLY_ACTORS' "$f"; then exit 1; fi; done` | Applies under options 1 and 2; not applicable under option 3. Exit 0. Red at base: the variable appears nowhere. Green after delivery: it appears in the install doc and in both workflow copies, each of them checked on its own |
| 14 | check +flow | `grep -q -F -e 'changelog:skip' docs/adopting-assay.md` | Applies under options 1, 2 and 3. Exit 0. Red at base: the install doc does not mention the label. Green after delivery: it states what the gate requires of the applier, and under options 1 and 2 that an unset variable names nobody |
| 15 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-66-changelog-skip-labeler-allow-list.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/66 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e '0 disproved, 0 unchecked' "$o"` | Applies under options 1, 2 and 3. Exit 0. Red at base: the consumers are not in a delivering diff, and the frontmatter still routes each to a follow-up. Green after delivery: the delivery flips every entry to fixed-here and every consumer is in the delivering diff. |

Rows 10 and 11 compare the steps this brief adds, not the whole workflow bodies, because the live
and staged files already differ beyond their banners (see facts). If the implementation PR changes
the step bodies in only one copy, the row stays red and the gap is the finding, not a reason to
loosen the row. Row 12 reads the old sentences by their real text; if a sentence is reworded
upstream before delivery, the row is amended by the human, not loosened by the implementer.

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Any label writer still waives the check | row 2 (non-maintainer and unlisted App); row 9 (applier-check mutant) |
| An unreadable event or role is treated as allowed | row 3; row 9 (fail-open mutant) |
| Option 1: a PR that also changes code, a workflow or a generated file is waived by path, or an unnamed author is | row 5 (not-briefs-only, workflow-file, other-author, unset and unreadable cases); row 9 (path-set and author-binding mutants) |
| Option 2: the named App's label waives a PR that also changes code or a workflow | row 7; row 9 (path-set and briefs-only mutants) |
| The Unreleased guard or the fragment rule is weakened | row 8 |
| The staged copy keeps the old rule or drifts from the live step | rows 10 and 11 |
| A skill body, the worker kit or a generated copy keeps the old rule | row 12 (all ten files, by their real sentences), and review reading the new sentence |
| Adopters install without the variable and read the gate as open | row 13, row 14, and rows 5 and 7 (unset case): an unset variable names nobody |
| A login is renamed or reclaimed and still matches the variable | No row: the entry form `<account id>:<login>` makes the gate require both to match; review-only for the exact comparison, pinned by the planned cases using a matching id with a different login |
| The waiver reaches a briefs-only PR that changes Verify-row commands verifier tooling later executes | No row: option 1 and 2 accept it. The waiver covers only the changelog check; review of the brief and its Verify rows stays in force, and the PR names it |
| The named App holds label-write and is misused outside briefs (option 2) | No row beyond row 7; it is the threat option 2 accepts. Review-only: the PR names it |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human. The ruling picks one of the three options and is recorded as the human's own comment on
the decision issue (#7010). The implementation PR touches a required check and an adopter install default,
so it needs a `Security-Review: pass` at its head before any ready flip. Every Verify row is
risk-bearing and fail-first, recorded with `verifyrun --fail-first` against the parent of the delivering merge.
