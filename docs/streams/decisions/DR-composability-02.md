---
id: DR-composability-02
date: "2026-10-26"
title: "Install ledger, paired inverses and the `disable` verb, as delivered: `deskdisable` reverses a component's apply steps LIFO, runs only inside reverses from a closed compiled allowlist of three components, never executes an outside compensation, and refuses before any write on an unknown component, a stranded dependent or an unregistered inside step; the `.assay/ledger.jsonl` writer/reader ships, but nothing outside tests writes the ledger yet"
consequence: critical
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/4285#issuecomment-6425251325"
alternatives:
  - "Treat decision issue #1023's \"not planned, already merged\" closure as the design approval (the first reading put on #1139) — not chosen: the driver ruled on #1139, \"A-Require explicit design approval\" (https://github.com/medici-finance/assay/issues/1139#issuecomment-6353072853). No further rationale was recorded, and none is supplied here."
  - "Execute a reverse by interpreting the manifest's free-text `inverse:` line — not delivered: the manifest text is documentation for a human, and `deskdisable` reverses an inside step only through a compiled executor registered for that exact component and step id (`insideExecutors` in tools/desk/cmd/deskdisable/executors.go)."
  - "A best-effort reverse for an inside step whose target the manifest does not pin down (the plugin skills, the hook wiring, the CI workflow, the statusgen and desk-tools binaries) — ruled out by the brief's ground rule (\"never a best-effort delete\"). The delivered tool refuses such a component by name, exit 5, before touching anything."
  - "Unattended forge-side compensations, such as the brief-00 labels text \"delete a label if no issue carries it\" — ruled out by the brief's facts (\"this brief marks NONE of the forge-side ones so\"). The delivered labels manifest now reads `list-for-human`, and the delivered tool has no path that executes an outside step at all: it does not read an `unattended` field."
  - "Remove the whole `docs/streams/` directory, as the streams-scaffold manifest's `inverse:` text says — narrowed in the executor: it removes only the scaffold README and then any directory left empty, so adopter content elsewhere under `docs/streams/` survives."
  - "Judge stranded dependents by version range, as `deskmanifest lint` does for activation — not delivered: `dependentsOf` ignores ranges and counts every manifest present under `--root` as a dependent, so the check errs toward refusing."
accepted:
  - "Real reverses exist for three components only: assay/streams-scaffold, assay/main-guard and assay/registers-scaffold. `deskdisable` refuses every other component that has an inside step (exit 5, nothing changed), so for most of the tree there is no \"turn it off\" yet."
  - "The three executors remove fixed paths with no check of content or origin: the scaffold README under docs/streams, the FINDINGS, INTAKE and RETRO register files, the .githooks/pre-commit hook, and the core.hooksPath setting. Everything at those paths is deleted, whoever wrote it and whenever. That includes all register history: the registers are append-only logs that adopters keep writing after install (spec/registers-v1.md §3.1), and no manifest requires `assay.registers`, so `deskdisable assay/registers-scaffold --yes` erases that history as its normal, unrefused result. Only content committed to git can be recovered. The setting is unset with `git -C <root> config`, which acts on whichever repository encloses `--root`, while the hook removed is the one under `--root`. Verify row 2's fixture places adopter content only at other paths."
  - "`deskdisable` never checks that a component was installed. It acts whenever a component.yaml declaring the target id exists anywhere under `--root`; when two declare the same id, the one with the last sorted path wins. In a tree that vendors or carries the manifests as source, everything at the executors' paths is non-Assay content, and disable deletes it all the same."
  - "Nothing outside tests writes the ledger. `AppendLedger` has no production caller; `deskinstall` only prints the ledger path, because its own effects are inside the boundary. An outside step's checklist line therefore reads \"(no ledger entries found for this step)\" on a real install until a later brief (composability/03) writes ledger lines at creation."
  - "main-guard's reverse treats every git exit 5 from `git config --unset core.hooksPath` as \"already absent\". When the setting holds several values, git also exits 5 and changes nothing. The executor then reports success, deletes .githooks/pre-commit, and leaves every value set. Git uses the last of several values, so hooks then point at the deleted hook whenever the .githooks value is the last one."
  - "Every outside step is printed as a human checklist line, whatever its manifest text or ledger state. Removing forge labels, Actions variables, the reviewer App installation and the forge binding stays human work."
  - "`deskdisable` reads the ledger but never edits it, and its dependent check counts every manifest under `--root` as present, because no notion of an ACTIVE component exists yet. It refuses more often than the brief's \"ACTIVE component\" wording asked."
  - "A further limit on the fixed-path claim is recorded in internal review notes; maintainer follow-up required. It is listed here so that an approval of this record covers it explicitly, as it covers the entries above."
---

**The driver approved this design record on 2026-10-26**, after the requested revision
landed in #4288. The unedited human comment on this record's decision issue names
`DR-composability-02`:
https://github.com/medici-finance/assay/issues/4285#issuecomment-6425251325.
This records design approval of the delivered behavior and accepted limits below. It does
not certify the implementation, discharge independent verification, approve a live removal,
or itself move the brief's lifecycle status.

## The ruling recorded

The brief is `docs/streams/composability/brief-02-ledger-and-inverses.md` (`gate: human`,
`risk.irreversible: yes`, `authored: 2026-09-08`, after the §4.4 cutover). Its first decision
issue was #1023, "Decision: assay:assay:composability:02 — Install ledger, paired inverses,
and the `disable` verb", with the marker `<!-- decision-gate: assay:assay:composability:02 -->`.
It offered "1. Approve as briefed", "2. Approve with changes" and "3. Hold / reject". The
delivering pull request, #953, had merged two days before. The driver's only comment there
(type User, 2026-09-14T03:03:46Z, not edited) reads, verbatim:

> "not planned" since its work already merged in #953

and the issue closed at the same second. That comment names no record and picks none of the
three options.

The standing question was then filed as #1139, "composability/02: does a decision-issue closed
'not planned, already merged' satisfy or moot the §4.4 design-approval gate going forward?". Its
body sets out two readings: the closure satisfies the gate, or it moots the gate without
satisfying it. The driver's own comment (type User, 2026-10-22T22:52:17Z, not edited),
verbatim: https://github.com/medici-finance/assay/issues/1139#issuecomment-6353072853

> A-Require explicit design approval

The issue closed at the same second. Its only other comment is a worker relay from
2026-10-08, which states the consequence of "no": "a decision record is needed before any row
change".

**Option labels.** The issue body does not label its readings A and B. This record
therefore relies on the ruling's own words, "Require explicit design approval", and not on the
letter. In those words, the #1023 closure does not satisfy §4.4 for composability/02, and the
brief needs an explicitly approved design record. This file is that record's draft.

**Approval provenance.** `decided-by` remains the public placeholder `human:<name>`.
Under spec/registers-v1.md §7.5 it is corroborated through the `ruling:` link, rather than a
pull-request approval or a worker relay. Decision issue #4285 carries both the record marker
`<!-- decision-gate: DR-composability-02 -->` and the brief marker
`<!-- decision-gate: composability/02 -->`. Its 2026-10-26 human comment reads
`Approve DR-composability-02 for composability/02`, with identical creation and update timestamps. The earlier
#1023 and #1139 comments remain historical context; neither is substituted for this approval.
The earlier B answer requested a revision, not approval. #4288 incorporated that revision
before the new approving comment was posted.

## The approved delivered design

The delivery is #953 (merged 2026-09-12T23:23:58Z at head `c9cf4d71`, merge commit `1cbd5bc4`),
plus the manifest follow-up #1024 (merged 2026-09-16T18:18:56Z at head `74e34027`, merge commit
`8faf7e6e`). Since then the only changes to `deskdisable` or the ledger on main are the
performance-instrumentation wrapper around `main` (#3051) and a test-only git-maintenance fix
(#2308). What shipped, read from that code:

1. **Every manifest's reverse is written.** Each `component.yaml` apply step's
   `TODO composability/02` placeholder was replaced. Inside steps carry an `inverse:` and
   outside steps a `ledger:` kind plus a `compensation:` (#953 for 23 manifests, #1024 for the
   three harness manifests). Verify row 1 printed `0` on main again on 2026-10-24. There are four
   outside steps: forge-github `forge-reachable` (compensation `none`, nothing Assay created),
   labels `label-set`, reviewer-app `reviewer-binding` and roster `trust-surface`. The last three
   are all `list-for-human`, and no manifest marks any compensation `unattended: true`.
2. **The ledger: an append-only writer and reader.** `tools/desk/internal/deskkit/ledger.go`
   defines `.assay/ledger.jsonl` lines `{component, version, step, kind, id, created, by}`.
   `AppendLedger` opens with `O_APPEND`, never rewrites a line, and refuses an entry missing
   component, step, kind or id. `ReadLedger` treats a missing file as zero entries and fails on
   a malformed line. No code outside tests calls `AppendLedger`. `deskinstall` prints the
   ledger path and records nothing, since its own effects are inside the boundary.
3. **The lint check.** `deskmanifest lint` gained `checkOutsideLedger`: an outside apply step
   with no `ledger:` value is a PROBLEM naming the step. It checks the manifest field, not the
   presence of ledger lines.
4. **The `disable` verb.** `deskdisable <component> [--root] [--dry-run] [--yes] [--cascade a,b,c]`
   in `tools/desk/cmd/deskdisable/` has exit codes 0 ok, 5 refused and 6 could-not-check:
   - A bare run with neither `--dry-run` nor `--yes` refuses.
   - It reads every `component.yaml` under `--root` and refuses an unknown target or cascade
     name. It also refuses an order that would leave any other manifest requiring a key a
     removed component provides, unless `--cascade` lists that dependent first.
   - It refuses any inside step without a registered executor, and any unknown boundary.
   - All of these checks run over the whole cascade before anything is written, so a refusal
     anywhere leaves the tree untouched.
   - It then replays each component's apply list in reverse. An inside step runs its
     registered executor, or is shown as `would-reverse` under `--dry-run`. An outside step is
     always a `human-checklist` line, prefixed `list-for-human:` (or `no action needed:` for a
     `none` compensation) and naming any ledger ids recorded for that step.
5. **The three executors.** `insideExecutors` is a closed map keyed by component and step id:
   - **streams-scaffold:** removes the scaffold README under docs/streams, then that directory
     and its parent only if empty.
   - **registers-scaffold:** removes the FINDINGS, INTAKE and RETRO register files, then the
     directory if empty.
   - **main-guard:** unsets `core.hooksPath` first, then removes the pre-commit hook and its
     directory if empty. It treats any git exit 5 from the unset as a no-op. Git returns 5 when
     the key is absent, which is the intended case, but also when `core.hooksPath` holds several
     values: a plain `git config --unset` refuses those with a "has multiple values" warning
     and changes nothing (checked with git on 2026-10-24). In that case the executor reports
     success, deletes the hook, and leaves every value set. Git uses the last of several values,
     so when the `.githooks` value is the last one, the result is the half-reversed state the
     unset-first ordering was meant to prevent: the setting points at a hook that is gone. The
     unset runs as `git -C <root> config`, so it acts on whichever repository encloses `--root`;
     the hook it removes is the one under `--root`.

   Each executor is idempotent on an already-reversed tree.

The brief and its delivering diffs are the authority. This summary does not amend either.

**Where delivery differs from the brief.** These differences are part of what an approval of
this record approves:

- The brief's SPOF line names a ledger-id refusal for forge objects. That layer does not exist,
  because no forge object is ever acted on.
- The brief's Task 2 asks for ledger lines written at creation by `deskinstall` and the
  scaffold paths. None are written.
- The brief's `unattended: true` path for compensations is not implemented.
- The brief's dependents rule names ACTIVE components. The tool counts every manifest present.
- The brief's facts say "The reviewer-App and main-guard components are human-installed
  today; their manifests' reverses are human checklists, not code, and `deskdisable` prints
  them". main-guard's reverse is compiled code instead (`mainGuardInverse`). It unsets
  `core.hooksPath` and deletes the pre-commit hook. The run prints a `reversed:` line for the
  step, but no human checklist item. The reviewer-App step is printed as the brief says.
- The brief's ground rule says a step whose reverse "cannot be written without deleting
  something Assay did not create" gets `compensation: list-for-human`, "never a best-effort
  delete". The three executors delete fixed paths without checking that Assay created them
  (see `accepted` and Review question 1), which is a best-effort delete under that rule. This
  record cites the same rule above to rule out reverses for the components whose targets the
  manifest does not pin down. The delivery applies the rule there, but not to the three
  components it did reverse.
- The brief's facts say inside reverses "MUST leave the tree byte-identical to the
  pre-install snapshot". The delivered tool takes no snapshot at install or at disable, and
  compares against nothing. The only snapshots are in the tests (`treeSnapshot` in the
  deskdisable test helpers). So byte-identity is proven for those fixtures only. On a real
  tree it is assumed: a disable run neither checks nor reports it.

## The Review-gate questions, answered from the code

The brief's `## Review` asks two questions.

**(1) The single control between a wrong reverse and adopter damage, and is it acceptable?**
For forge-side state, the control is structural: the build contains no code path that executes
an outside step, whatever the manifest's `boundary:` text or the ledger says. For tree-side
state, the brief's upper layer, the manifest's `boundary:` classification, is backed by the
closed `insideExecutors` allowlist. A step mis-labelled `inside` that is not one of the three
registered keys is refused before any write. **The single control is the allowlist's three
entries themselves, and their fixed target paths.** Nothing behind them checks that the file
being removed or the setting being unset was created by Assay: there is no pre-install snapshot,
no content hash and no ledger line for inside effects. Disable deletes all content at those
paths, not just files an adopter had before install. That includes the scaffold README, a
`.githooks/pre-commit`, a `core.hooksPath` value, and the whole history of the three register
files, which adopters keep appending to after install. No manifest requires `assay.registers`,
so disabling assay/registers-scaffold erases that history as its normal result. Nothing checks
that an install happened, either. The tool acts whenever a manifest declaring the id exists
under `--root`, so in a tree that carries the manifests as source, every file at those paths is
non-Assay content. Only committed content can be recovered from git. Uncommitted register
entries and a local `core.hooksPath` value cannot. A further limit on the fixed-path claim is
recorded in internal review notes; maintainer follow-up required. Whether this is acceptable is
the driver's decision on approving this record. Each limit, that one included, is listed under
`accepted` so that an approval covers it explicitly.

**(2) Which Verify row proves the lower layer catches a mis-classified step with the upper layer
wrong?** **No row.** The brief's Review offers "row 4 with a deliberately mis-labelled fixture
step, if the implementer adds it; otherwise the answer is 'no row'". Row 4 was not amended, and
no row's Expect states such a case. The implementer added a test,
`TestDisable_MisclassifiedOutsideStepRefusesRatherThanExecute`, and recorded it in the
brief's Evidence as an "extra row". It is not in the Verify table. Row 8's
`go test ./cmd/deskdisable/...` runs it, but row 8 expects only exit 0. The test also proves the
allowlist layer, not the ledger-id layer the brief named, which was not built. It passed on main
on 2026-10-24. That test does not cover a mis-labelled step that happens to share a registered
key, nor an adopter file at an executor's own path.

## Consequence level — transcriber's classification

`consequence: critical` is not part of the ruling, which states no level. The transcriber took
it from the brief: `risk.irreversible: yes`, and a `gate-why` that says a wrong compensation
"removes a control … from a live repo" and is "Irreversible in the forge case". This record
also fixes the ledger and compensation model that later briefs build unattended removal on.
The delivered build itself takes no forge-side action, and its tree-side damage is
recoverable from git only for committed content. Uncommitted register history and a local
`core.hooksPath` value are not recoverable. Because no forge-side action is taken, the driver
may judge `major` instead. The driver can
correct the level before the record's approval is linked.

## Decision history and remaining obligations

- **Design approval recorded.** The first answer on #4285 was B, "revise first"
  (https://github.com/medici-finance/assay/issues/4285#issuecomment-6384825452), requesting
  the neutral limit in the `accepted` list. #4288 merged it; the desk re-asked on the same
  issue. The new unedited 2026-10-26 comment linked by `ruling:` approves this record.
  A pull-request approval, desk relay or reviewer verdict was not used as its substitute.
- **Ownership proof for inside reverses.** Whether the three executors should refuse, or list
  for a human, when they cannot show Assay created their target is not decided here.
- **Ledger writes at creation.** These are composability/03's work (the brief's own consumer
  line), not this record's.
- **Independent verification and sign-off.** The brief's Verify table runs after merge by a
  non-implementer, and composability/02 is `gate: human`, so its `verified` and `done` flips
  need the human sign-off on that run. This record attests none of it.

## Lifecycle reconciliation

The delivered code already exists on main through #953 and #1024. Recording this approval
is not new implementation and grants no live removal authority. The brief cites this record
through `design: DR-composability-02`; any `todo` to `implemented` correction must be emitted
by the canonical reconciliation writer and pass the independent brief decision-gate checks.
The board's verification/review stamps and the brief's historical Evidence remain unchanged.
A design approval is not final verification acceptance. The original Verify obligations,
accepted residuals above, and human sign-off remain owed.
