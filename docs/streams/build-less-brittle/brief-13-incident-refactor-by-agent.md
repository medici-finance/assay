---
brief: assay:assay:build-less-brittle:13
title: "Incident-time refactor by an agent, with one human residue: the strong-tier session runs the investigation, assembles the oracle, drafts the refactor PR, and asks the driver exactly one typed decision, only for what the record does not settle"
why: >-
  Briefs 09 and 12 say what an investigation and an oracle contain. Left there, each is a
  hand-off: the investigation waits for an authoring session, the design brief waits for a
  worker, the oracle's unknowns wait for someone to ask. Every hand-off is a place the mark
  goes cold, which is how Google's predictor died (Lewis et al. 2013). This brief closes the
  loop: when a class reaches its threshold (04), a module is marked brittle (08) or a worker
  hits strike two (05), one strong-tier session does the whole thing itself, from the owning
  brief and decision record through the class history, the module's git history, the Verify
  tables, the mutation specs and regression tags, the findings and the research notes, to a
  ruling (09), an oracle with generated and triaged characterization tests (12), and the
  refactor as its own draft PR after the fix. Human contact is one typed decision, and only
  for the residue of `intent-changed?`: the behaviours nobody specified that the record cannot
  settle, presented with the agent's recommendation and a default. `drifted` and
  `implementation-wrong` need no human beyond the merge. This is why briefs 01, 04 and 11
  come first: an agent can only research a record that exists. Without an owner and a DR (01),
  incidents filed by class with their kind and origin (04), and each incident mapped to the
  test that pins it (11), every gap in the record becomes a human ask, and the residue grows
  back into the whole question.
wave: 6
depends: ["build-less-brittle/05", "build-less-brittle/09", "build-less-brittle/12"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; driver-ruled amendment)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 row 15, §4.13, §4.7 (no new gate), §8"
  - "docs/brittle-investigation-template.md (09) and docs/refactor-oracle-template.md (12): the two artifacts this run produces; docs/investigations/README.md (where they land)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues (the strong-tier lane 04 and 09 route design-owed classes through; this brief's run is that dispatch)"
  - "plugins/assay/skills/ask-decision/SKILL.md (one decision at a time, options immutable at filing, a recommended default first, the exact reply shape): the residue decision uses this shape unchanged"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md §2 (security-gate refusal: never quietly weaken a control) and §5 (lineage self-check); docs/contracts.md seam contracts (a consumer-facing interface is a seam row or a `consumers:` entry); author-brief rule 10 and the `single-point-of-failure:` line (a trust boundary)"
  - "docs/streams/build-less-brittle/spec.md §11 (postmortem action items need an owner and a verifiable end state; a correct prediction that is not actionable changes nothing, so the investigation is the load-bearing step)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no procedure runs investigation, oracle and refactor in one dispatch; a design-owed class today yields a design brief and stops; the escalation vocabulary is exactly the labels `question`, `help wanted`, `needs-decision`"
exec-tier: strong
exec-tier-why: "(a) the run is a chain of judgements the runbook must bound precisely (what counts as settled by the record, what is residue, what is a hard gate); (b) it spans two templates, a skill, the ledger and the tracker."
domain: complex
consumers:
  - "docs/incident-refactor-run.md (planned): follow-up build-less-brittle/13 (this brief)"
  - "docs/refactor-oracle-template.md (planned, by 12) §6 Residue: follow-up build-less-brittle/13 (this brief)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Un-briefed issues: follow-up build-less-brittle/13 (this brief; ≤ 6 lines, offset)"
  - "tools/desk/internal/testledger (Residue and DecisionBody, pure functions over an oracle file): follow-up build-less-brittle/13 (this brief)"
  - "the rehearsal record (two synthetic class issues, two closed draft PRs, one answered decision issue): follow-up build-less-brittle/13 (this brief; filed at the 3-per-24h budget)"
  - "installed deskdispatch binaries: out-of-scope (no kit text changes; the runbook is read from the tree at dispatch time)"
---

# Brief 13 — Incident-time refactor by an agent, with one human residue

## Context

files:
- `docs/incident-refactor-run.md` (planned): NEW. The runbook: trigger, the ordered reads, the outputs in order, the one-ask rule, the hard-gate exceptions, and the rehearsal record.
- `docs/refactor-oracle-template.md` (planned): from build-less-brittle/12;: `## 6. Residue` (≤ 15 lines): the decision-issue body shape and the reply grammar.
- `plugins/assay/skills/worker-desk/SKILL.md`: §"Un-briefed issues" (≤ 6 lines, offset).
- `tools/desk/internal/testledger/ledger.go` (planned): from build-less-brittle/11;, `ledger_test.go` (planned): `Residue(oracle) []Row`, `DecisionBody(rows) string`, `TestResidueFixtures`, `TestDecisionBodyShape`.
- `tools/desk/internal/testledger/testdata/rehearsal/{complete,gap}/` (planned): NEW. A standalone Go module (its own `go.mod`, invisible to `./...`) with a brief, a DR and a class-issue snapshot; `complete/` has every behaviour sourced, `gap/` has one behaviour no record explains. The oracles the rehearsal produced are copied in as `oracle.md`.
- `changelog/build-less-brittle-13.md` (planned)

facts:
- **Trigger and lane.** The three triggers converge on one object, a class issue labelled
  `design-owed` (04), with `brittle` when 08 marked the module, and with the worker's
  `strike two` note when 05 stopped it. It dispatches on the un-briefed lane at **strong**
  tier, as 04 and 09 already say. This brief changes what that dispatch delivers. No new
  verb, label, flag or reply.
- **The run, in order** (the runbook fixes it; the session does not reorder):
  1. *Read the record.* The owning brief(s) and `DR-` record (09's intent reads); the class
     issue and every prior incident attached to it, with kind, incident group and
     `introduced-by`; the module's git history over the window, each change mapped to the
     issue its subject or `Issue:`/`Brief:` trailer names; the Verify tables and Evidence of
     every brief that touched the module; the mutation maps and the `// regression:` tags
     (11); findings entries whose `affects:` names the module; research notes that cite it.
     A read that cannot be made is recorded as `could-not-read (<what>)`, never skipped.
  2. *Rule.* The investigation file (09): one divergence, one recommendation, an end state.
     On `reconcile`, `accept` or `clear` the run stops after the fix brief or record
     amendment 09 already prescribes; nothing below applies.
  3. *On `redesign`: assemble the oracle* (12). Generate characterization tests over the
     owner's public surface; triage every one the record settles (`keep` with its source,
     `drop` with its source); leave `unknown` only where no brief, DR, incident, finding or
     test says which way. Write the DR amendment and the design brief (with `oracle:` and the
     five acceptance rows) in the same authoring PR as the investigation and the oracle.
  4. *Draft the refactor as its own PR*, after the incident's fix has landed (through the
     ordinary path, or the `bleed` reply when 05 stopped it): `Brief:` the design brief,
     `## Oracle` with the triage table, `## Weight`, `## Tests retired` where the report is
     non-empty, `Retires-test:` trailers on every `drop`. Draft until the residue, if any,
     is answered.
  5. *The one ask.* If §3 holds any `unknown`, file one `needs-decision` issue in the
     ask-decision shape: every unknown row as one question block with the agent's
     recommendation and a default first, options immutable at filing, one reply line.
     Reply grammar: `residue: default`, or `residue: keep <test…>; drop rest` (or the
     inverse). On the reply the run (or the shepherd that resumes it) writes the verdicts
     into §3 citing the reply URL, retires the `drop` tests with trailers, and pushes. Zero
     unknowns means zero issues filed. Never a second ask: an answer that raises a new
     unknown is a `could-not-resolve` line in §3 and the PR stays draft with that line, for
     the driver to see at the merge.
- **Where the human is, and is not.** `drifted` and `intent-right, implementation-wrong`
  need no human beyond the merge: the record settles every behaviour, so §3 has no
  `unknown`, no issue is filed, review is the model loop, and the ready-flip is
  pr-review-desk's. `intent-changed` is where residue lives, and the residue is asked once.
  The **only other human contact** is the three hard gates that already exist, restated here
  so the run names them rather than discovers them: (a) *weakening or removing a control*
  (worker kit §2: never quietly weaken; a retired control needs the layer that still refuses
  the threat, 02's rule 2); (b) *a consumer-facing interface change* (a seam row in
  `docs/contracts.md`, or a `consumers:` entry outside the module); (c) *crossing a trust
  boundary* (author-brief rule 10; a `single-point-of-failure:` line that changes). Any of
  the three makes the design brief `gate: human` under the existing template rule, with a
  `needs-decision` issue that says which gate and why. None of this is new machinery.
- **Residue is computable from the oracle.** `Residue(oracle)` returns §3's `unknown` rows;
  `DecisionBody(rows)` renders the ask. Both are pure functions in the ledger package (11),
  so the rule "zero unknowns, zero asks; any unknowns, one ask" is a property of the file,
  not of the session's mood, and the fixtures below pin it.
- **The rehearsal, recorded.** The implementer runs the procedure twice, as real strong-tier
  dispatches, on the two synthetic incidents under `testdata/rehearsal/`: two class issues
  (labelled `error-class`, `design-owed`, titled `class: rehearsal-complete …` and `class:
  rehearsal-gap …`), each pointing at its standalone module. Expected: both runs end in a
  draft refactor PR carrying `## Oracle`; the complete run files no issue and its PR carries
  none of the three escalation labels; the gap run files exactly one `needs-decision` issue
  with exactly one question, which the driver answers `residue: default`. Both PRs are then
  closed unmerged (synthetic), and the issues closed citing the rehearsal. The PR and issue
  numbers are recorded in this brief's Evidence, and the Verify rows read them. Budget: the
  three filings fit one `deskfile` day; the decision reply is the driver's, so the rehearsal
  may span two days.
- **Tier and bound.** Strong tier, the strongest the pool offers (a project value). The run
  is bounded by 09's caps on the history read and by 12's surface definition; a module whose
  public surface exceeds 40 exported symbols is split by S- row before generation, and the
  runbook says so.
- Line count at f7bde6bfa: worker-desk 868 (04, 09 and this brief share the cap).

design-fit:
  owner: docs/incident-refactor-run.md (planned; a runbook beside the two templates it sequences)
  contract: none — a procedure; it produces 09's and 12's artifacts and files 04's issues
  retires: ["the hand-off between a redesign recommendation (09) and its design brief (an authoring dispatch that no longer exists as a separate step for design-owed classes)"]
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (worker-desk); ~120 lines of pure functions in testledger, reported under golines
  why-add: n/a (no ratcheted growth). The alternative, keeping investigation, oracle and refactor as three dispatches, was rejected: each hand-off is a queue position where the mark goes cold (C4), and the strong-tier context assembled for the investigation is exactly what the oracle and the refactor need.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The rehearsal PRs are NEVER merged and the rehearsal module never leaves `testdata/`.
- The one-ask rule is a ceiling, not a target: a run that files an issue for something the record settles has failed the runbook, and the reviewer checks the gap run's question against the fixture.
- Never widen a hard gate to avoid an ask, and never narrow one to avoid a human.

## Task

1. `docs/incident-refactor-run.md` (planned): the trigger, the ordered reads (each with its command,
   reusing 09's and 12's verbatim), the outputs in order, the one-ask rule with the reply
   grammar, the three hard gates with the existing rule each cites, the 40-symbol split rule,
   and the rehearsal record section (filled in step 6).
2. `docs/refactor-oracle-template.md` (planned) `## 6. Residue` (≤ 15 lines): the decision body shape
   (one question block per `unknown` row: the test, what it observes, the recommendation,
   the default, the two options), the immutable-options line, the reply grammar.
3. testledger: `Residue`, `DecisionBody`; fixtures `testdata/rehearsal/{complete,gap}/oracle.md`
   (copied from the rehearsal's outputs in step 6; until then, hand-authored and replaced);
   `TestResidueFixtures` (0 and 1), `TestDecisionBodyShape` (exactly one `## Question` per
   unknown, `Recommended:` and `Default:` present, `Options are immutable` present, the reply
   grammar line present).
4. worker-desk §"Un-briefed issues" (≤ 6 lines, offset): a `design-owed` class dispatches
   the incident-refactor run per the runbook; the deliverables in order; one ask at most.
5. The two synthetic modules and their records under `testdata/rehearsal/`.
6. Run the rehearsal (two dispatches), record PR and issue numbers in the runbook and in
   this brief's Evidence, copy the produced oracles into the fixtures, close the PRs unmerged
   and the issues citing the rehearsal.
7. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 1–3 pin the residue rule and the ask
shape mechanically. Rows 4–7 are the rehearsal's dereference rows: (a) the complete record
ends in a draft refactor PR with zero human asks, (b) the gap record ends in exactly one
typed decision. They read the PR and issue numbers from the runbook's rehearsal record
(`rehearsal-complete-pr: <N>`, `rehearsal-gap-pr: <N>`, `rehearsal-gap-issue: <N>`, one per
line), which step 6 writes and the Evidence repeats. Rows 8–10 are wiring and net ≤ 0 rows.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && { go test ./internal/testledger/ -run TestResidueFixtures -count=1 -v; go test ./internal/testledger/ -run TestDecisionBodyShape -count=1 -v; } \| grep -c -- '--- PASS'` | `2` |
| 2 | `cd tools/desk && go test ./internal/testledger/ -run TestResidueFixtures -count=1 -v \| grep -cE 'residue: complete=0 gap=1'` | `1` (the complete record has no unknown; the gap record has exactly one) |
| 3 | `cd tools/desk && f=internal/testledger/testdata/rehearsal/complete/oracle.md && cp "$f" /tmp/bl13-o.bak && awk 'BEGIN{d=0} !d && / keep /{sub(/ keep /," unknown "); d=1} {print}' /tmp/bl13-o.bak > "$f" && go test ./internal/testledger/ -run TestResidueFixtures -count=1 > /tmp/bl13-mut.out 2>&1; rc=$?; cp /tmp/bl13-o.bak "$f"; test $rc -ne 0 && grep -c 'complete=1' /tmp/bl13-mut.out` | `1` (mutation: one verdict flipped to unknown in the complete fixture is one residue, and the fixture test says so) |
| 4 | `n=$(grep -oE '^rehearsal-complete-pr: [0-9]+' docs/incident-refactor-run.md \| grep -oE '[0-9]+'); test -n "$n" && gh pr view "$n" -R medici-finance/assay --json state,body -q '"\(.state) \(.body \| test("## Oracle")) \(.body \| test(" keep "))"'` | `CLOSED true true` (the complete run produced a refactor PR with the triage table; it was closed, never merged) |
| 5 | `n=$(grep -oE '^rehearsal-complete-pr: [0-9]+' docs/incident-refactor-run.md \| grep -oE '[0-9]+'); test -n "$n" && gh pr view "$n" -R medici-finance/assay --json labels -q '[.labels[].name] \| map(select(. == "question" or . == "help wanted" or . == "needs-decision")) \| length'; gh issue list -R medici-finance/assay --state all --label needs-decision --search 'rehearsal-complete in:body' --json number -q 'length'` | `0`, then `0` (zero human asks on the complete record, in the tracked vocabulary) |
| 6 | `n=$(grep -oE '^rehearsal-gap-issue: [0-9]+' docs/incident-refactor-run.md \| grep -oE '[0-9]+'); gh issue list -R medici-finance/assay --state all --label needs-decision --search 'rehearsal-gap in:body' --json number -q 'length'; gh issue view "$n" -R medici-finance/assay --json body -q '.body' \| grep -c '^## Question'` | `1`, then `1` (exactly one decision issue with exactly one question for the gap record) |
| 7 | `n=$(grep -oE '^rehearsal-gap-issue: [0-9]+' docs/incident-refactor-run.md \| grep -oE '[0-9]+'); p=$(grep -oE '^rehearsal-gap-pr: [0-9]+' docs/incident-refactor-run.md \| grep -oE '[0-9]+'); gh issue view "$n" -R medici-finance/assay --json comments -q '[.comments[].body \| select(startswith("residue: "))] \| length'; gh pr diff "$p" -R medici-finance/assay --name-only \| grep -c -- '-oracle.md'` | ≥ `1`, then `1` (the driver answered in the reply grammar; the gap PR carries its oracle) |
| 8 | `grep -c -e '^## 6\. Residue' -e 'Options are immutable' -e 'residue: default' docs/refactor-oracle-template.md` | ≥ `3` |
| 9 | `grep -c 'incident-refactor-run' plugins/assay/skills/worker-desk/SKILL.md && test "$(wc -l < plugins/assay/skills/worker-desk/SKILL.md)" -le 868 && echo NET-OK` | ≥ `1`, then `NET-OK` |
| 10 | `for g in 'weaken' 'consumer-facing' 'trust boundary'; do grep -ci "$g" docs/incident-refactor-run.md; done \| grep -c '^[1-9]'` | `3` (the three hard gates are named; no fourth is introduced: `grep -c 'gate: human' docs/incident-refactor-run.md` is the reviewer's cross-check) |
| 11 | `test "$(git diff --name-only HEAD~1 -- tools/desk/cmd \| grep -v _test.go \| wc -l \| tr -d ' ')" = 0 && ! git diff --name-only HEAD~1 \| grep -q '^tools/desk/internal/testledger/testdata/rehearsal/.*go.mod$' && echo NO-SHIPPED-CHANGE \|\| git diff --name-only HEAD~1 \| grep 'testdata/rehearsal' \| head -3` | `NO-SHIPPED-CHANGE`, or the rehearsal module's own files only (nothing shipped changed; the module is a fixture) |
| 12 | `statusgen --consumers --root . --brief build-less-brittle/13; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer.
     Rows 4–7 also record <PR-C>, <PR-G>, <ISSUE-G> and the reply's URL. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer reads the gap run's one question against the
fixture: it must ask about the one behaviour the record does not explain and nothing else;
a question the record could have answered is a finding against the runbook, not the fixture.
The reviewer also reads the complete run's PR for any prose ask ("should we…") that dodged
the labels: the vocabulary rows prove the labels, the reviewer proves the prose. Finally,
the reviewer confirms the three hard gates in the runbook each cite an existing rule and
that no fourth gate appears.
