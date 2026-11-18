---
brief: assay:assay:desk-tools:66
title: "Let the coordinator desk waive the changelog check on briefs-only PRs, and make the gate read who applied the label"
why: >-
  A PR that changes only briefs (Verify-row amendments, brief authoring, stream README edits)
  has no notable change to record, yet the required changelog check stays red until a maintainer
  applies the changelog:skip label by hand. The check honors the label by its presence alone and
  never reads who applied it, so the workflow header's claim that the label is maintainer-only
  is not enforced: any identity that can write labels can waive the check. Two skill bodies say
  the desk may apply the label, which contradicts that header. This brief picks one rule for
  who may waive, makes the gate enforce it, and carries the same setting into the adopter
  install so a fresh install gets it.
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  It decides who may waive a required check. Granting an automation identity label-write and
  letting its label count is the widening the workflow header says this repository refuses, and
  the adopter install would carry that widening to every adopter. Only the human can weigh the
  saved maintainer clicks against it.
decision-trigger: creation
issues: [7010]
outcome: none
schema: brief-v2
version: 1
authored: 2026-11-18 by a worker-desk authoring session (design proposal; no ruling recorded yet)
sources:
  - "medici-finance/assay#7010 (the request: the coordinator desk cannot waive the changelog check on briefs-only PRs, and the gate never checks who applied the label)"
  - ".github/workflows/changelog-check.yml header: the label path stays the maintainer's; the Evidence lane is decided in-workflow because handing label-write to an automation App is the widening this repository refuses"
  - "plugins/assay/skills/worker-desk/SKILL.md line 23 and plugins/assay/skills/author-brief/SKILL.md line 962 (the label is applied by the desk or a human)"
  - "changelog/README.md, section Automatic board reconciliation (the existing narrow classification exception, with an exact-identity and whole-diff check)"
  - "docs/adopting-assay.md, the install step that sets Actions variables and the scaffolded statusgen workflow's roster variables"
  - "freshness-checked 2026-11-18 @ 1f5392225 (origin/main)"
exec-tier: strong
exec-tier-why: "It changes who can satisfy a required check, must fail closed when the actor cannot be read, and must keep the live workflow, its staged copy, the skill bodies and the adopter install in agreement."
domain: complicated
consumers:
  - ".github/workflows/changelog-check.yml: fixed-here (the label path reads the applier and the changed-file classification before it counts)"
  - "tools/changelog/changelog-check.yml: fixed-here (the staged copy carries the same step, so the two do not drift)"
  - "tools/changelog/check.sh: fixed-here (the SKIP input stays a plain true/false; the applier decision is computed before it, as the Evidence lane is)"
  - "tools/changelog/README.md and changelog/README.md: fixed-here (both state who may waive and for which PRs)"
  - "plugins/assay/skills/worker-desk/SKILL.md: fixed-here (the label sentence states the ruled rule)"
  - "plugins/assay/skills/author-brief/SKILL.md: fixed-here (the label sentence states the ruled rule)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: fixed-here (the two label sentences state the ruled rule)"
  - "plugins/assay/skills/pr-review-desk/references/verdict-format.md: fixed-here (the line that says a human applies the label states the ruled rule)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: fixed-here (the changelog clause states the ruled rule; the common clauses are quoted verbatim, so this is the one place the wording changes)"
  - ".cursor/skills/: fixed-here (generated copies of the skill bodies, regenerated from the sources above)"
  - "docs/adopting-assay.md: fixed-here (the install step lists the new Actions variable beside the roster variables)"
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
- `tools/changelog/check.sh`, `tools/changelog/check_test.sh` — the decision and its offline tests
- `changelog/README.md`, `tools/changelog/README.md` — the convention text
- `plugins/assay/skills/worker-desk/SKILL.md`, `plugins/assay/skills/author-brief/SKILL.md`, `plugins/assay/skills/pr-shepherd/SKILL.md`, `plugins/assay/skills/pr-review-desk/references/verdict-format.md` — the label wording
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md` — the worker kit's changelog clause
- `docs/adopting-assay.md` — the install step for Actions variables
- **add** — `tools/changelog/labeler_test.sh` (planned), `tools/changelog/labeler-mutations.json` (planned)
- **add** — no changelog fragment: the task section for this authoring PR waives it; the implementation PR adds one

facts:
- The workflow header calls the label a maintainer act. Nothing enforces it. The label is read
  from the event payload, which names the labels on the PR and not who applied them.
- Who applied a label is on the forge: the issue events read returns each `labeled` event with the
  actor and the label name. The workflow holds `pull-requests: read` only. Whether that scope
  reads those events is a thing step 1 checks, not assumes.
- The workflow already has a second SKIP condition computed in the workflow, for PRs authored by
  the verify desk's App that change only `docs/streams/`. Its design note says a label cannot be
  handed to an automation App without widening the waiver, so it decides from PR metadata
  instead. That lane covers Evidence PRs and nothing else, and a mixed PR is excluded.
- The board-reconciliation exception does the same for one writer. It binds an exact identity and
  branch, then compares the whole diff, and falls back to the ordinary requirement when it cannot
  read the event.
- `worker-desk` line 23 and `author-brief` line 962 say the desk or a human applies the label.
  The `pr-shepherd` and `pr-review-desk` bodies and the worker kit give the label to the desk or a
  human too, or to a human alone. The sources disagree with each other, not only with the header.
- The scaffolded statusgen workflow already receives roster values as Actions variables. A fresh
  install sets variables in its install step, so a new variable reaches adopters there and not
  through a copied workflow file.

design-fit:
  owner: .github/workflows/changelog-check.yml decides; tools/changelog/check.sh stays the one script that reads SKIP
  contract: none
  retires: [the unenforced "maintainer-only" sentence in the workflow header]
  weight: "verbs 0, flags 0, refusals 2 (applier not allowed, files not briefs-only), one Actions variable"
  why-add: "The variable replaces an unenforced claim with a list the gate reads. Without it the label either stays open to any writer or stays a manual maintainer click on every briefs-only PR."

## Human decision

Every PR that changes only briefs (a Verify-row amendment, a new brief, a stream README edit)
fails the required changelog check, because such a PR has no release note to write. Today a
maintainer applies the `changelog:skip` label by hand to turn it green. Two examples on main
right now, #6993 and #6994, have every other check green. Separately, the check counts the
label whoever applied it, so the rule that only a maintainer waives is a sentence in a file
header, not something the gate enforces.

Options:
1. **Allow-list the applier and let the coordinator desk's App apply the label to briefs-only PRs.**
   The gate reads who applied the latest `changelog:skip` label and counts it only when that login
   is on an allow-list kept in an Actions variable (maintainers plus the coordinator desk's App).
   For the desk's App it also requires that the PR changes only brief files and stream READMEs.
   This is what the filed request asks for. The threat it opens: the desk's App gets label-write,
   and a compromised or mistaken desk could waive a notable change if the briefs-only test were
   wrong. The test is a file-list read, so it is cheap to get right and to pin.
2. **Decide briefs-only PRs in the workflow, with no label, as the Evidence lane does.** A PR
   authored by the coordinator desk's App that changes only brief files passes the check with a
   notice in the log. No App gets label-write, and the label stays maintainer-only. The cost: the
   maintainer's label is still unenforced for everyone else, so the second half of the request
   needs option 3's allow-list as well.
3. **Allow-list the applier only.** The gate enforces who applied the label, the desk gets
   nothing new, and maintainers keep clicking on briefs-only PRs. It closes the gap in the
   header and leaves the cost in the request.
4. **Do nothing.** Briefs-only PRs keep waiting on a maintainer, and any label writer can waive.

Default if no answer: option 4 after 2026-12-02. Silence changes nothing. Option 1 is the
recommendation, because it is the request as filed. It also covers the second half of the
request, the applier check, and the adopter install carries it as one variable. Option 2 is the
safer fallback if the human will not hand label-write to the desk.

## Ground rules
- This brief records no ruling. Implement only the option the human records on the decision issue.
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Fail closed. When the applier or the file list cannot be read, the label does not count and the
  check stays in force, as the Evidence lane already does. Never treat could-not-check as allowed.
- The absolute `## Unreleased` guard in `check.sh` is not touched. The waiver never excuses it.
- The workflow file needs a workflows-capable identity, so the implementation PR may stage the
  change and hand the promotion to the human, as the staged copy header describes.
- Never write a credential-shaped literal into a test. Use fixed fake logins.

## Task
1. Read the issue events endpoint with the workflow's own scope on a scratch PR in a test
   repository and record whether `pull-requests: read` returns the actor of a `labeled` event.
   If it does not, name the narrowest added read scope and put it in the decision issue before
   any code. Do not widen the workflow's permissions on a guess.
2. Add a workflow step that finds the latest `labeled` event for `changelog:skip` and outputs the
   applier's login. A missing event, an unreadable response or a login that is not a plain
   identity outputs not-proven.
3. Add the allow-list as an Actions variable (planned name `ASSAY_CHANGELOG_SKIP_APPLIERS`,
   comma-separated logins). An unset variable means the list is empty, and then only
   a user holding the repository's maintain or admin role counts, read from the collaborator
   permission endpoint, not from a name.
4. For an applier that is the coordinator desk's App, also require the briefs-only
   classification: every changed path matches `docs/streams/<stream>/brief-*.md` or
   `docs/streams/<stream>/README.md`, and none is a generated single-writer file. A path outside
   that set, or a file list that cannot be read, makes the label not count for that applier.
5. Build the `SKIP` input from the label AND an allowed applier. `check.sh` still receives a plain
   true or false, and its Unreleased guard still runs first. Mirror the change in
   `tools/changelog/changelog-check.yml`.
6. Add `tools/changelog/labeler_test.sh` (planned): offline cases for an allowed maintainer, a
   denied login, the desk's App on a briefs-only PR, the desk's App on a PR that also touches a
   workflow file, an unreadable event, and an unset variable. Run it from the existing changelog
   suite.
7. Add `tools/changelog/labeler-mutations.json` (planned) with mutants: drop the applier check,
   treat an unreadable event as allowed, widen the path set to all of `docs/`, and skip the
   briefs-only test for the desk's App. Each is paired with the case that catches it.
8. State the ruled rule once in each file listed under consumers, and regenerate `.cursor/skills/`.
   Add the variable and its meaning to `docs/adopting-assay.md` beside the roster variables, and
   say there that an unset value keeps the maintainer-only label.
9. Record the ruling as the human's own comment on the decision issue, in the README row.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `test -f tools/changelog/labeler_test.sh && bash tools/changelog/labeler_test.sh` | exit 0; an allowed applier greens, a denied login, an unreadable event and an unset list keep the check red, and the desk's App is waived only on a briefs-only PR |
| 2 | check +neighbour | `bash tools/changelog/check_test.sh && bash tools/changelog/reconcile_test.sh` | exit 0; the existing fragment, proxy, Unreleased and board-reconciliation cases still pass |
| 3 | check +mutation | `O=$(mktemp) && trap 'rm -f "$O"' 0 && test -f tools/changelog/labeler-mutations.json && cd tools/desk && go run ./cmd/muhar -spec ../changelog/labeler-mutations.json > "$O" 2>&1 && grep -F -e 'Totals: 4 caught, 0 NOT CAUGHT, 0 could-not-mutate.' "$O"` | exit 0; all four mutants caught |
| 4 | check +dereference | `a=$(sed -n '/^on:/,$p' .github/workflows/changelog-check.yml) && b=$(sed -n '/^on:/,$p' tools/changelog/changelog-check.yml) && test "$a" = "$b" && ! grep -rn -e 'applied by the desk or a human' plugins/assay/skills tools/desk/cmd/deskdispatch/references/worker-prompt.md` | exit 0; the staged copy matches the live workflow body and no skill text keeps the contradicting sentence |
| 5 | check +flow | `grep -F -e 'ASSAY_CHANGELOG_SKIP_APPLIERS' docs/adopting-assay.md .github/workflows/changelog-check.yml tools/changelog/changelog-check.yml` | exit 0; the variable name appears in the install doc and in both workflow copies |
| 6 | check +flow +dereference | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-66-changelog-skip-labeler-allow-list.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/66 --delivery "$DELIVERY_HEAD" --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e '0 disproved, 0 unchecked' "$o"` | exit 0; every consumer listed in the frontmatter is in the delivering diff |

Row 4 compares the live and staged workflow bodies, and expects the live and staged copies to match after their banners. If the two files
differ today in more than the banner, step 5 states the difference in the PR and the row is
amended by the human, not loosened by the implementer.

## Threat model and detection

| Failure mode | Caught by |
|---|---|
| Any label writer still waives the check | row 1 (denied login); row 3 (applier-check mutant) |
| An unreadable event is treated as allowed | row 1 (unreadable event); row 3 (fail-open mutant) |
| The desk's App waives a PR that also changes code or a workflow | row 1 (mixed-PR case); row 3 (path-set mutants) |
| The Unreleased guard or the fragment rule is weakened | row 2 |
| The staged copy or a skill body keeps the old rule | row 4 |
| Adopters install without the variable and read the gate as open | row 5; unset means maintainer-only |
| The desk's App holds label-write and is misused outside briefs | No row; it is the threat option 1 accepts. Review-only: the PR names it, and the briefs-only test bounds the label's effect to PRs with no code |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

## Review
Gate: human. The ruling picks one of the four options and is recorded as the human's own comment on
the decision issue. The implementation PR touches a required check and an adopter install default,
so it needs the usual reviewer verdict at its head before any ready flip.
