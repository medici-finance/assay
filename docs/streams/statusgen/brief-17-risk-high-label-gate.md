---
brief: assay:assay:statusgen:17
title: '`--scan-issues`: a `risk:high` label derives `gate: human` on the issue''s placeholder (`risk:med` does not)'
why: >-
  Triage records how risky an item is as a label, `risk:high`, `risk:med` or `risk:low`. The
  scanner that turns an open issue into a dispatchable placeholder does not read that label. It
  gates the placeholder to a human only when the title or a label happens to contain one of
  three words (auth, funds, security). So an item a triage session scored as high risk is
  written `gate: model` and can be implemented and reviewed by models alone. The driver ruled
  on 2026-10-08 that `risk:high` forces the human gate and `risk:med` does not. This brief
  makes the scanner apply that when the label is already on the issue at the moment its
  placeholder is first written. Triage sets the label and the scanner does not wait for triage,
  so an item can get its placeholder first and keep `gate: model`. For that case the brief
  adds an instruction to the triage session, not a mechanism; whether the scanner should ever
  re-derive a stored gate is open and tracked in #2405.
wave: 1
depends: []
unblocks: ["statusgen/18"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
outcome: none
schema: brief-v2
version: 1
authored: 2026-10-08 by the-desk session (on behalf of the driver)
sources:
  - "ruled by the driver, 2026-10-08: a `risk:high` label on a scanned issue forces `gate: human` on its issue-loop placeholder; `risk:med` does not"
  - "statusgen/placeholder.go — `riskGateLabels`, `riskGateKeywords`, `registerRiskGateVocabulary`, `derivePlaceholderGate`: the rule this brief changes"
  - "plugins/assay/skills/intake-desk/SKILL.md §'Scored triage — the impact/risk/effort triple at exit' — the only definition of the `risk:{high,med,low}` labels"
  - "plugins/assay/skills/intake-desk/SKILL.md §'The judgment half', item 1 — the scan lane writes the placeholder and leaves triage to the session, so the placeholder can exist before the `risk:` label does"
  - "freshness-checked 2026-10-08 @ 82caf63b2 (origin/main) — `riskGateLabels` is `funds`, `security`; no Go file reads a `risk:` issue label; no open PR touches `statusgen/placeholder.go`"
consumers:
  - "statusgen/placeholder.go: fixed-here statusgen/17 (this brief; one entry in the base label set and its comment)"
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here statusgen/17 (this brief; the scored-triage section gains one bullet: what `risk:high` does at first write, and what a triage session does when the placeholder came first)"
---

# Brief 17 — `risk:high` label derives the human gate on a placeholder

## Context

files:
- `statusgen/placeholder.go` — one entry in `riskGateLabels`, and the comment above it.
- `statusgen/placeholder_test.go` — the two new unit tests named in Verify.
- `statusgen/scanissues_test.go` — the new scan-level test named in Verify.
- `plugins/assay/skills/intake-desk/SKILL.md` — one bullet in the scored-triage section.
- `docs/streams/statusgen/brief-17-risk-high-label-gate.md` — this brief's two `consumers:`
  routings (Task 7).
- `changelog/<branch-slug>.md` — the per-PR fragment this repo enforces.

single-point-of-failure: the `risk:high` label being on the issue at the moment the scanner
first writes its placeholder. Nothing orders that. Triage sets the label at its exit and the
scan lane writes the placeholder without waiting for triage, so an item scored after its
placeholder exists keeps `gate: model`. One layer sits behind the control and it is an
instruction, not a mechanism: the triage session that scores an item `risk:high` and finds its
placeholder reading `gate: model` labels the issue `help wanted` and names the file (Task 6).
No tool reports a session that skips it. Two other controls exist and neither catches this
fault, because neither reads the label or the triage score: the PR-diff risk gate keys on the
changed paths, and a hand-written `gate: human` needs a person who has been told. A second
mechanical layer is not designed here because it would mean re-deriving or reporting on a
stored gate, which is open question 2 below (#2405).

facts (all read on main @ 82caf63b2, 2026-10-08):
- **The rule today.** `derivePlaceholderGate(labels, title)` in `statusgen/placeholder.go`
  returns `human` in two cases and `model` otherwise. Stage 1 is an EXACT label match: each
  label is trimmed and lower-cased and looked up in `placeholderRiskLabels`, which is built
  from the base list `riskGateLabels` (`funds`, `security`) plus any deployment additions.
  Stage 2 is a whole-word, case-insensitive match of `riskGateKeywords` (`auth`, `funds`,
  `security`, plus additions) against the title and the label text joined together.
- **So a `risk:high` issue gates to `model` today** unless its title or another label trips
  one of the three words. No Go file in the tree reads a `risk:` issue label.
- **One function, three readers.** `planScan` in `statusgen/scanissues.go` and the same-repo
  transcriber in `statusgen/transcribescan.go` call it with the issue's labels and title and
  write the result into the new file as an explicit `gate:` line. `parsePlaceholderFile` in
  `statusgen/placeholder.go` calls it with the stored labels and an empty title, but only for
  a file that has NO `gate:` line. Change the vocabulary once; do not add a per-reader rule.
- **A base entry survives a config reload.** `registerRiskGateVocabulary` rebuilds both
  matchers from the base lists plus the deployment's words every time configuration loads, and
  never mutates the base lists. An entry in `riskGateLabels` is therefore present with the
  deployment extension set, unset, or reloaded.
- **Why the base list and not the deployment extension.** The extension is fed from
  `ASSAY_RISK_PATH_TRIGGERS_EXTRA` through `riskKeywordsFromPathTriggers`, which turns each
  configured path into its leading directory name and registers that one word as BOTH a label
  and a keyword. Routing `risk:high` through it would make every adopter re-declare a label
  the methodology itself defines, and would also match the text `risk:high` in a title.
- **`risk:{high,med,low}` is the methodology's own vocabulary.** The intake-desk skill's
  scored-triage section defines the three labels and says the score is a judgement the triage
  session records. `risk:high` is the only one this brief reads.
- **The gate is derived once.** The scanner computes the gate when it first writes a
  placeholder, and skips any issue whose placeholder file already exists ("never overwrite").
  The parser treats a stored `gate:` as explicit and does not re-derive it. Two consequences
  this brief does NOT change: a `risk:high` label added AFTER the placeholder was written
  leaves the stored gate as it is, and placeholders already on the board are not rewritten.
- **The label and the first write are unordered.** The intake-desk skill is the only place the
  label is defined, and it is set at the exit of triage. The same skill says the scan lane
  writes the placeholder and leaves triage to the session. No Go file sets a `risk:` label at
  filing. So the label is on the issue at first write only when someone applied it before the
  scan ran. Derive-once is safe in the other direction: removing the label later does not lower
  a stored `gate: human`.
- **A `help wanted` issue is in the human queue.** The skill's shared desk rules define the
  label as an authority edge that needs a comment saying what is needed and from whom, and
  count a labelled item as waiting on input. It is not one of the scanner's excluded labels, so
  the placeholder stays where it is.
- **Fail direction.** An issue read with no labels derives `model`. A repo whose issue list
  cannot be read is skipped with a notice and writes no placeholder.
- **One read-time effect.** A placeholder file with no `gate:` line and a stored
  `risk:high` label will read as `gate: human` after this change, because the parser's
  fallback uses the same function. The scanner always writes the line, so this reaches only
  hand-written or older files.

Out of scope — open, NOT decided by this brief or by the ruling behind it. Questions 2 and 3
are tracked in #2405, which lists the options and proposes none:
1. Whether the keyword list (`auth`, `funds`, `security`) should take more vocabulary.
2. Whether reactivation (a reopened issue, or an excluded label removed) should re-derive a
   placeholder's gate and labels, and more generally whether a label change after the
   placeholder exists should ever update the stored gate.
3. Back-filling `gate: human` onto placeholders that already exist for `risk:high` issues.

Also not settled here: who may set or remove `risk:high`. The derivation trusts whatever labels
the issue carries at first write, and the scanner's trust gate checks the issue's author, not
whoever applied a label.

Also unchanged: the PR-diff risk gate and its path triggers, the excluded-label set, the trust
gate, and `derivePlaceholderEffort`.

design-fit:
  owner: `statusgen/placeholder.go` — `derivePlaceholderGate` and the base vocabulary it reads
  contract: none — placeholder gate derivation has no row in the semantic-owner index; one function owns it and its three readers call that function
  retires: []
  weight: verbs 0, flags 0, refusals 0, base risk labels +1, rule-text lines +6 (one bullet in the intake-desk skill, as wrapped in Task 6)
  why-add: >-
    The label entry goes INTO the owner; no second mechanism is added. Putting it in the
    deployment extension instead was considered and rejected (see facts). The skill bullet
    is added because a triage label that used to be ordering data now changes who must sign
    off, and the place the label is defined is where a triage session will look. Its last
    sentence is the only thing that puts a late-scored item in front of a person, so it cannot
    be cut to one line. Nothing existing could be removed to pay for it: the three keywords
    stay, by the ruling's scope.

## Ground rules
- NEVER push to main or trigger workflows by hand. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- `risk:high` only. Do not add `risk:med` or `risk:low`, and do not add a prefix or pattern
  match on `risk:` — stage 1 stays an exact-label lookup.
- Add the entry to `riskGateLabels` only, never to `riskGateKeywords`: a title that merely
  mentions `risk:high` must not gate.
- Do not change the scanner's never-overwrite rule, do not rewrite any existing placeholder
  file, and do not make the parser re-derive a stored `gate:`. Those are the open questions
  listed under Context.
- Add tests as new functions. Do not edit, reorder or delete anything inside the seven existing
  test functions Verify row 10 names: they carry the existing human-gate cases, and row 10
  compares their text to what main carried when this brief was written. If row 10 is red before
  you have touched them, main changed one of them: report NEEDS_CONTEXT, do not re-pin.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Fail-first: write the tests in steps 3–5 before step 2, run Verify row 2, and paste the red
   output (the `risk:high` case reporting `model`, want `human`) into the PR body under
   `## Fail-first`.
2. In `statusgen/placeholder.go`, add `"risk:high"` to `riskGateLabels`. Extend the comment
   above it to say that the base label set carries the methodology's top triage risk label,
   that `risk:med` and `risk:low` are deliberately absent, and that the match is exact.
3. `TestPlaceholderGateRiskHighLabel` (planned) in `statusgen/placeholder_test.go`, a table over
   `derivePlaceholderGate`:
   - `[bug, risk:high]`, any neutral title → `human`
   - `[Risk:High]` → `human` (labels are lower-cased) and `[" risk:high "]` → `human` (trimmed)
   - `[bug, risk:med]` → `model`; `[risk:low]` → `model`; `[risk:med, risk:low]` → `model`
   - `[risk:higher]` → `model` and `[risk]` → `model` (exact match, no prefix)
   - `[bug]` with the title `raise to risk:high after triage` → `model` (label only, not text)
   - `[risk:med, security]` → `human` (the existing label still gates beside a `risk:` label)
   - `[auth]` → `human` (a bare `auth` label is not in the label set and gates through the
     keyword stage; no case pins that today)
4. `TestPlaceholderGateRiskHighSurvivesExtension` (planned) in the same file, saving and
   restoring the vocabulary the way `TestRiskGateVocabularyExtension` does: after
   `registerRiskGateVocabulary` with one neutral extra word, and again after calling it with
   nil lists, `[risk:high]` → `human` and `[risk:med]` → `model`.
5. `TestScanRiskHighPlaceholderGate` (planned) in `statusgen/scanissues_test.go`, through
   `planScan` with `fixtureLister` and `blessAll`, on a temp root with one stream:
   - three open issues labelled `[bug, risk:high]`, `[bug, risk:med]` and `[security]`, with
     neutral titles: the three plans' `Content` carry `gate: human`, `gate: model` and
     `gate: human` in that order, and each `Content`, written to a file and read back with
     `parsePlaceholderFile`, yields the same gate;
   - a fourth issue labelled `[risk:high]` whose placeholder file ALREADY exists with
     `gate: model`: `planScan` produces no plan for it and the file reads back `model`. This
     pins that the brief left the derive-once rule alone. It is the state the step 6
     instruction exists for, and it does not settle open question 2;
   - a placeholder file with `labels: [risk:high]` and NO `gate:` line reads back `human`.
6. In the intake-desk skill, add this ONE bullet to the scored-triage section, directly after
   the "Judgment recorded, not computed" bullet. Keep the wording: Verify row 7 pins five of
   its clauses. Re-wrap it if the file needs that. Edit only that section and leave every
   generated block alone.

   ```
   - **`risk:high` gates the placeholder, at first write only.** A `risk:high` label that is on the
     issue when the issue scanner first writes its placeholder derives `gate: human` on it; `risk:med`
     and `risk:low` do not. The scanner does not wait for triage and derives the gate once, so a label
     set after the placeholder exists does not change its gate. When you score an item `risk:high` and
     its placeholder already reads `gate: model`, label the issue `help wanted` and comment, naming the
     placeholder file, that human:<name> must set `gate: human` in it by hand.
   ```

   The last sentence instructs a session. Nothing checks that a session follows it; row 7
   checks only that the skill says it.
7. In this brief's frontmatter, change both `consumers:` entries from the `follow-up` routing
   to `fixed-here`, keeping each site and its parenthesised note. As authored the two entries
   point at this brief; once the change lands, both paths are fixed in the same diff. Verify
   row 9 refuses a leftover `follow-up` and asserts that both entries are corroborated.
8. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector, writes the output to a file and asserts
that test's `--- PASS:` line, so a missing or renamed test fails the row. Rows 1–8 and 10 read
the tree and run the same before and after merge. Row 9 reads the implementing diff and is run
on the implementing branch before merge; its Expect says what it does on merged main.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | exit 0 | check:ci |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | exit 0; a `risk:high` label derives `human`, including the upper-case and padded forms; `risk:med`, `risk:low`, `risk:higher`, `risk` and a title that only mentions `risk:high` all derive `model`; a bare `auth` label derives `human`. Mutation: with `"risk:high"` removed from `riskGateLabels` the row exits 1 on the `risk:high` case (`model`, want `human`); with it added to `riskGateKeywords` instead the row exits 1 on the title-only case | check:ci +mutation |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | exit 0; with a deployment extension registered, and again with it cleared, `risk:high` still derives `human` and `risk:med` still derives `model` | check:ci |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | exit 0; the three named tests run and pass as the tree carries them: `funds` and `security` labels and the `auth` / `funds` / `security` keywords gate, `authored` does not, an unregistered product word derives `model`, and the config-driven extension gates. The row runs whatever cases the file holds. It cannot see a case that was edited or deleted; row 10 does | check:ci |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | exit 0; through `planScan`, the placeholder written for a `risk:high` issue carries `gate: human`, the one for a `risk:med`-only issue carries `gate: model`, the `security` one carries `gate: human`, each reads back with the same gate, an existing `gate: model` placeholder is neither re-planned nor re-read as `human`, and a gate-less file with a stored `risk:high` label reads `human` | check:ci +flow |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | exit 0; the whole package passes, so no other test depended on `risk:high` deriving `model` | check:ci |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | exit 0; the base vocabulary names the label, and the skill's scored-triage section, read with its lines joined so that re-wrapping does not matter, carries all five clauses: the label at first write derives `gate: human`; `risk:med` and `risk:low` do not; a later label does not change the gate; a session that finds `gate: model` labels the issue `help wanted`; and what it asks for is the hand edit. A `.` in a pattern stands for a backtick. Red when the bullet is absent, sits outside that section, or lacks any one clause. The row reads the skill's text; it does not observe whether a session follows the instruction | check:ci +dereference |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | exit 0; the intake-desk skill still passes every skill check after the one-bullet edit, and no generated block drifted | check:ci |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | exit 0, run on the implementing branch before merge: no `consumers:` entry still routes to this brief as a follow-up, and with both rewritten to `fixed-here` both paths are in the branch's diff. Red when the routings are left as authored (the first grep), and red when either path is missing from the diff (the instrument reports `DISPROVED` and exits 1). On merged main the merge-base is HEAD and the brief is not in the diff, so the instrument exits 2, could-not-check, and the row does not pass there; a post-merge run checks out the delivering commit and passes its parent as `--base` | check |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | exit 0; the seven existing test functions that assert a human gate (five in `statusgen/placeholder_test.go`, two in `statusgen/scanissues_test.go`) are byte-identical to main at 82caf63b2, where their 308 lines hash to the value in the command. Red when a case in any of them is edited, deleted or reordered, when a line is inserted into one (an early return, a skip), and when one is renamed or removed. New test functions elsewhere in the two files do not change it. It pins those functions' text only, not the helpers they call, and it does not run them: rows 4 and 6 do | check:ci |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | fail exit=2 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 18d752a4c931 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2664 run=9ee14495fcc5ac71483d7446dead01a9565f0b347fb65fcdbe101fade4601b47 source=18d752a4c9318e27bcfd49fa44f9cfb62923b691 brief=docs/streams/statusgen/brief-17-risk-high-label-gate.md model=gpt-6.1-sol tier=any



**VERIFY: BLOCKED** — no implementation failure is established. The required execution witness returned exit 2. Rows 1–8 and 10 were explicitly unrun because the required Linux network-off sandbox is unavailable on darwin; no unsandboxed substitute or mutation was run. Row 9 executed and returned exit 2: its consumer instrument reported COULD-NOT-CHECK because the merged brief is absent from the diff against HEAD, exactly the limitation described in its Expect cell. Its redirected diagnostic is: “no entry was corroborated and none was disproved.” The witness's fail exit=2 classification is preserved above; it is not rounded up to a pass.

Environment blocker: [assay#2665](https://github.com/medici-finance/assay/issues/2665). The operator must provide a supported Linux fenced verifier; a fresh independent admission must preserve check:ci network-off enforcement. The post-merge row 9 delivering-diff requirement must also be discharged without moving an admitted home's detached source. Status stays implemented; parent owns the Evidence lane and claim release.

Grounding preceded the delivering diff and tests: only an exact trimmed, case-insensitive highest-risk label present at first write adds human authority; lower risk labels and title-only text stay model, existing explicit gates stay explicit, and vocabulary reloads must preserve the base label. This pass asserts no dynamic result for those requirements.

RISK-VALUE: DERIVED — riskGateLabels added entry = "risk:high" @ statusgen/placeholder.go:89 — the brief records the driver ruling that only the highest triage label forces human sign-off; exact-label ownership preserves title-only and lower-score behavior.

RISK-VALUE: DERIVED — first-write gate = "human" @ plugins/assay/skills/intake-desk/SKILL.md:191 — Task 6 explicitly pins this authority for a risk:high label present at first write, while stored explicit gates remain authoritative.

RISK-VALUE: DERIVED — late-scored escalation label = "help wanted" @ plugins/assay/skills/intake-desk/SKILL.md:194 — Task 6 requires a human hand-edit request when a risk:high issue already has gate:model; no model or scanner may rewrite the stored gate under this brief.

Independent brief-grounded static sub-verifications covered four authority-bearing clusters: rows 2–3 (label/reload), rows 4 and 10 (regression/pin), row 5 (scan flow), and row 7 (late triage). Each returned execution could-not-check without running the unavailable check:ci executor or a substitute; static source coverage matched its assigned requirements. Row 2 mutations and row 10 byte-hash verification remain explicitly unrun. These static readings add no execution pass.

Risk ranking: the new production human-authority binding and late-triage human handoff rank first, because a wrong value could let risky work pass a model gate before any correction. The label inclusion and handoff derive from the recorded driver ruling and Task 6, independently of passing tests. Fixture IDs 401–405, expected plan count 3, the neutral extension word settlement, test titles, expectation strings, assertion diagnostics and file names rank last: editing/re-running tests fully reverses them; none moves funds or production identity. The seven protected regression functions and pinned hash are inherited requirements, not a new authority or newly selected number; their dynamic tests and byte-hash check remain unrun.

<details>
<summary>Conservative literal enumeration over the complete delivering diff</summary>

Literal enumeration covers every added Go literal on changed lines and every introduced backticked policy token across all seven paths of delivering diff 69ca08c699a1fec7d76dace08859578d262b4ec0^..69ca08c699a1fec7d76dace08859578d262b4ec0. Existing funds/security on the replaced production list are conservatively included. Fixture and diagnostic literals rank last; they do not authorize any irreversible act.

- feat-statusgen-17.md.L2.literal1 = `statusgen --scan-issues` @ changelog/feat-statusgen-17.md:2
- feat-statusgen-17.md.L2.literal2 = `gate: human` @ changelog/feat-statusgen-17.md:2
- feat-statusgen-17.md.L2.literal3 = `risk:high` @ changelog/feat-statusgen-17.md:2
- feat-statusgen-17.md.L2.literal4 = `risk:med` @ changelog/feat-statusgen-17.md:2
- feat-statusgen-17.md.L2.literal5 = `risk:low` @ changelog/feat-statusgen-17.md:2
- README.md.L44.literal1 = `--scan-issues` @ docs/streams/statusgen/README.md:44
- README.md.L44.literal2 = `risk:high` @ docs/streams/statusgen/README.md:44
- README.md.L44.literal3 = `gate: human` @ docs/streams/statusgen/README.md:44
- README.md.L44.literal4 = `risk:med` @ docs/streams/statusgen/README.md:44
- brief-17-risk-high-label-gate.md.L35.literal1 = `risk:high` @ docs/streams/statusgen/brief-17-risk-high-label-gate.md:35
- SKILL.md.L188.literal1 = `risk:high` @ plugins/assay/skills/intake-desk/SKILL.md:188
- SKILL.md.L188.literal2 = `risk:high` @ plugins/assay/skills/intake-desk/SKILL.md:188
- SKILL.md.L189.literal1 = `gate: human` @ plugins/assay/skills/intake-desk/SKILL.md:189
- SKILL.md.L189.literal2 = `risk:med` @ plugins/assay/skills/intake-desk/SKILL.md:189
- SKILL.md.L190.literal1 = `risk:low` @ plugins/assay/skills/intake-desk/SKILL.md:190
- SKILL.md.L191.literal1 = `risk:high` @ plugins/assay/skills/intake-desk/SKILL.md:191
- SKILL.md.L192.literal1 = `gate: model` @ plugins/assay/skills/intake-desk/SKILL.md:192
- SKILL.md.L192.literal2 = `help wanted` @ plugins/assay/skills/intake-desk/SKILL.md:192
- SKILL.md.L193.literal1 = `gate: human` @ plugins/assay/skills/intake-desk/SKILL.md:193
- placeholder.go.L89.literal1 = "funds" @ statusgen/placeholder.go:89
- placeholder.go.L89.literal2 = "security" @ statusgen/placeholder.go:89
- placeholder.go.L89.literal3 = "risk:high" @ statusgen/placeholder.go:89
- placeholder_test.go.L355.literal1 = "risk:high label" @ statusgen/placeholder_test.go:355
- placeholder_test.go.L355.literal2 = "bug" @ statusgen/placeholder_test.go:355
- placeholder_test.go.L355.literal3 = "risk:high" @ statusgen/placeholder_test.go:355
- placeholder_test.go.L355.literal4 = "frontend keydown bubbles to card" @ statusgen/placeholder_test.go:355
- placeholder_test.go.L355.literal5 = "human" @ statusgen/placeholder_test.go:355
- placeholder_test.go.L356.literal1 = "upper-case label is lower-cased" @ statusgen/placeholder_test.go:356
- placeholder_test.go.L356.literal2 = "Risk:High" @ statusgen/placeholder_test.go:356
- placeholder_test.go.L356.literal3 = "" @ statusgen/placeholder_test.go:356
- placeholder_test.go.L356.literal4 = "human" @ statusgen/placeholder_test.go:356
- placeholder_test.go.L357.literal1 = "padded label is trimmed" @ statusgen/placeholder_test.go:357
- placeholder_test.go.L357.literal2 = " risk:high " @ statusgen/placeholder_test.go:357
- placeholder_test.go.L357.literal3 = "" @ statusgen/placeholder_test.go:357
- placeholder_test.go.L357.literal4 = "human" @ statusgen/placeholder_test.go:357
- placeholder_test.go.L358.literal1 = "risk:med does not gate" @ statusgen/placeholder_test.go:358
- placeholder_test.go.L358.literal2 = "bug" @ statusgen/placeholder_test.go:358
- placeholder_test.go.L358.literal3 = "risk:med" @ statusgen/placeholder_test.go:358
- placeholder_test.go.L358.literal4 = "frontend keydown bubbles to card" @ statusgen/placeholder_test.go:358
- placeholder_test.go.L358.literal5 = "model" @ statusgen/placeholder_test.go:358
- placeholder_test.go.L359.literal1 = "risk:low does not gate" @ statusgen/placeholder_test.go:359
- placeholder_test.go.L359.literal2 = "risk:low" @ statusgen/placeholder_test.go:359
- placeholder_test.go.L359.literal3 = "" @ statusgen/placeholder_test.go:359
- placeholder_test.go.L359.literal4 = "model" @ statusgen/placeholder_test.go:359
- placeholder_test.go.L360.literal1 = "med and low together do not gate" @ statusgen/placeholder_test.go:360
- placeholder_test.go.L360.literal2 = "risk:med" @ statusgen/placeholder_test.go:360
- placeholder_test.go.L360.literal3 = "risk:low" @ statusgen/placeholder_test.go:360
- placeholder_test.go.L360.literal4 = "" @ statusgen/placeholder_test.go:360
- placeholder_test.go.L360.literal5 = "model" @ statusgen/placeholder_test.go:360
- placeholder_test.go.L361.literal1 = "risk:higher is not a prefix match" @ statusgen/placeholder_test.go:361
- placeholder_test.go.L361.literal2 = "risk:higher" @ statusgen/placeholder_test.go:361
- placeholder_test.go.L361.literal3 = "" @ statusgen/placeholder_test.go:361
- placeholder_test.go.L361.literal4 = "model" @ statusgen/placeholder_test.go:361
- placeholder_test.go.L362.literal1 = "bare risk label does not gate" @ statusgen/placeholder_test.go:362
- placeholder_test.go.L362.literal2 = "risk" @ statusgen/placeholder_test.go:362
- placeholder_test.go.L362.literal3 = "" @ statusgen/placeholder_test.go:362
- placeholder_test.go.L362.literal4 = "model" @ statusgen/placeholder_test.go:362
- placeholder_test.go.L363.literal1 = "title text alone does not gate" @ statusgen/placeholder_test.go:363
- placeholder_test.go.L363.literal2 = "bug" @ statusgen/placeholder_test.go:363
- placeholder_test.go.L363.literal3 = "raise to risk:high after triage" @ statusgen/placeholder_test.go:363
- placeholder_test.go.L363.literal4 = "model" @ statusgen/placeholder_test.go:363
- placeholder_test.go.L364.literal1 = "existing label still gates beside risk:med" @ statusgen/placeholder_test.go:364
- placeholder_test.go.L364.literal2 = "risk:med" @ statusgen/placeholder_test.go:364
- placeholder_test.go.L364.literal3 = "security" @ statusgen/placeholder_test.go:364
- placeholder_test.go.L364.literal4 = "" @ statusgen/placeholder_test.go:364
- placeholder_test.go.L364.literal5 = "human" @ statusgen/placeholder_test.go:364
- placeholder_test.go.L365.literal1 = "bare auth label gates through the keyword stage" @ statusgen/placeholder_test.go:365
- placeholder_test.go.L365.literal2 = "auth" @ statusgen/placeholder_test.go:365
- placeholder_test.go.L365.literal3 = "" @ statusgen/placeholder_test.go:365
- placeholder_test.go.L365.literal4 = "human" @ statusgen/placeholder_test.go:365
- placeholder_test.go.L370.literal1 = "derivePlaceholderGate(%v, %q) = %q, want %q" @ statusgen/placeholder_test.go:370
- placeholder_test.go.L386.literal1 = "risk:high" @ statusgen/placeholder_test.go:386
- placeholder_test.go.L386.literal2 = "" @ statusgen/placeholder_test.go:386
- placeholder_test.go.L386.literal3 = "human" @ statusgen/placeholder_test.go:386
- placeholder_test.go.L387.literal1 = "%s: risk:high must gate human, got %q" @ statusgen/placeholder_test.go:387
- placeholder_test.go.L389.literal1 = "risk:med" @ statusgen/placeholder_test.go:389
- placeholder_test.go.L389.literal2 = "" @ statusgen/placeholder_test.go:389
- placeholder_test.go.L389.literal3 = "model" @ statusgen/placeholder_test.go:389
- placeholder_test.go.L390.literal1 = "%s: risk:med must stay model, got %q" @ statusgen/placeholder_test.go:390
- placeholder_test.go.L393.literal1 = "settlement" @ statusgen/placeholder_test.go:393
- placeholder_test.go.L393.literal2 = "settlement" @ statusgen/placeholder_test.go:393
- placeholder_test.go.L394.literal1 = "with a deployment extension" @ statusgen/placeholder_test.go:394
- placeholder_test.go.L396.literal1 = "with the extension cleared" @ statusgen/placeholder_test.go:396
- scanissues_test.go.L1931.literal1 = "issue-404.md" @ statusgen/scanissues_test.go:1931
- scanissues_test.go.L1931.literal2 = "alpha" @ statusgen/scanissues_test.go:1931
- scanissues_test.go.L1931.literal3 = 404 @ statusgen/scanissues_test.go:1931
- scanissues_test.go.L1931.literal4 = "risk:high" @ statusgen/scanissues_test.go:1931
- scanissues_test.go.L1931.literal5 = "gate: model" @ statusgen/scanissues_test.go:1931
- scanissues_test.go.L1935.literal1 = 401 @ statusgen/scanissues_test.go:1935
- scanissues_test.go.L1935.literal2 = "plain bug one" @ statusgen/scanissues_test.go:1935
- scanissues_test.go.L1935.literal3 = "bug" @ statusgen/scanissues_test.go:1935
- scanissues_test.go.L1935.literal4 = "risk:high" @ statusgen/scanissues_test.go:1935
- scanissues_test.go.L1936.literal1 = 402 @ statusgen/scanissues_test.go:1936
- scanissues_test.go.L1936.literal2 = "plain bug two" @ statusgen/scanissues_test.go:1936
- scanissues_test.go.L1936.literal3 = "bug" @ statusgen/scanissues_test.go:1936
- scanissues_test.go.L1936.literal4 = "risk:med" @ statusgen/scanissues_test.go:1936
- scanissues_test.go.L1937.literal1 = 403 @ statusgen/scanissues_test.go:1937
- scanissues_test.go.L1937.literal2 = "plain bug three" @ statusgen/scanissues_test.go:1937
- scanissues_test.go.L1937.literal3 = "security" @ statusgen/scanissues_test.go:1937
- scanissues_test.go.L1938.literal1 = 404 @ statusgen/scanissues_test.go:1938
- scanissues_test.go.L1938.literal2 = "plain bug four" @ statusgen/scanissues_test.go:1938
- scanissues_test.go.L1938.literal3 = "risk:high" @ statusgen/scanissues_test.go:1938
- scanissues_test.go.L1941.literal1 = "" @ statusgen/scanissues_test.go:1941
- scanissues_test.go.L1942.literal1 = 3 @ statusgen/scanissues_test.go:1942
- scanissues_test.go.L1943.literal1 = "expected 3 plans (404 already has a placeholder), got %d: %+v" @ statusgen/scanissues_test.go:1943
- scanissues_test.go.L1949.literal1 = 401 @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1949.literal2 = "human" @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1949.literal3 = 402 @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1949.literal4 = "model" @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1949.literal5 = 403 @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1949.literal6 = "human" @ statusgen/scanissues_test.go:1949
- scanissues_test.go.L1952.literal1 = "issue %d not planned" @ statusgen/scanissues_test.go:1952
- scanissues_test.go.L1954.literal1 = "gate: " @ statusgen/scanissues_test.go:1954
- scanissues_test.go.L1954.literal2 = "\n" @ statusgen/scanissues_test.go:1954
- scanissues_test.go.L1955.literal1 = "issue %d content should carry gate: %s:\n%s" @ statusgen/scanissues_test.go:1955
- scanissues_test.go.L1959.literal1 = "issue %d read back gate %q (ok=%v err=%v), want %q" @ statusgen/scanissues_test.go:1959
- scanissues_test.go.L1962.literal1 = 404 @ statusgen/scanissues_test.go:1962
- scanissues_test.go.L1963.literal1 = "issue 404 has an existing placeholder — the scanner must not re-plan it" @ statusgen/scanissues_test.go:1963
- scanissues_test.go.L1965.literal1 = "docs/streams/alpha" @ statusgen/scanissues_test.go:1965
- scanissues_test.go.L1965.literal2 = "issue-404.md" @ statusgen/scanissues_test.go:1965
- scanissues_test.go.L1966.literal1 = "model" @ statusgen/scanissues_test.go:1966
- scanissues_test.go.L1967.literal1 = "existing gate: model placeholder must stay model, got ok=%v err=%v gate=%q" @ statusgen/scanissues_test.go:1967
- scanissues_test.go.L1971.literal1 = "issue-405.md" @ statusgen/scanissues_test.go:1971
- scanissues_test.go.L1971.literal2 = "alpha" @ statusgen/scanissues_test.go:1971
- scanissues_test.go.L1971.literal3 = 405 @ statusgen/scanissues_test.go:1971
- scanissues_test.go.L1971.literal4 = "risk:high" @ statusgen/scanissues_test.go:1971
- scanissues_test.go.L1973.literal1 = "human" @ statusgen/scanissues_test.go:1973
- scanissues_test.go.L1974.literal1 = "gate-less risk:high file should read human, got ok=%v err=%v gate=%q" @ statusgen/scanissues_test.go:1974

</details>

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | fail exit=2 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ bfef0dab238a (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2546 run=8adbb999e55178c9c76e3dbd29868210e53cf079ee09d3e22ae66cc847494d24 source=bfef0dab238ad5aad468f736d78daa8c4c236dd7 brief=docs/streams/statusgen/brief-17-risk-high-label-gate.md model=claude-opus-5-5 tier=any

Independent verifier, hand run on merged main at bfef0dab238a (delivering change 69ca08c699a1,
#2530; this verifier is not its implementer). The witness table above is the execution witness
from this same run: on this darwin host every check:ci row comes back could-not-run because the
witness's network-off sandbox needs Linux `unshare --net`, so each check:ci row was executed by
hand below, from the home at the attested source, with TMPDIR pointed at a scratch directory
outside the home and KUBECONFIG=/dev/null. Toolchain: go1.27.1 darwin/arm64, git 2.56.0,
statusgen v1.0.37. Row 9 is the brief's pre-merge row; on merged main it exits 2 by its own
Expect (the witness records that as fail exit=2), so it was also executed by the brief's
post-merge recipe in a fresh clone at the delivering commit, with its parent as the base.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1 command, verbatim | exit 0 | pass exit=0; go build and go vet print nothing | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 command, verbatim | exit 0; risk:high derives human, med/low/higher/bare/title-only derive model, bare auth derives human | pass exit=0; `--- PASS: TestPlaceholderGateRiskHighLabel (0.00s)` with all 11 subtests PASS. Mutation A (risk:high removed from riskGateLabels, in a scratch clone, restored with git checkout): exit 1, `derivePlaceholderGate([bug risk:high], ...) = "model", want "human"`, also the upper-case and padded cases. Mutation B (moved to riskGateKeywords instead): exit 1, `derivePlaceholderGate([bug], "raise to risk:high after triage") = "human", want "model"` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 command, verbatim | exit 0 | pass exit=0; `--- PASS: TestPlaceholderGateRiskHighSurvivesExtension (0.00s)` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command, verbatim | exit 0; the three named tests pass | pass exit=0; `--- PASS: TestDerivePlaceholderGate (0.00s)`, `--- PASS: TestPlaceholderGateFromConfigRiskExtra (0.25s)`, `--- PASS: TestRiskGateVocabularyExtension (0.00s)` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 command, verbatim | exit 0 | pass exit=0; `--- PASS: TestScanRiskHighPlaceholderGate (0.56s)` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 command, verbatim | exit 0 | pass exit=0; `ok github.com/medici-finance/assay/statusgen 481.628s` (inside the row's 600s timeout) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 command, verbatim | exit 0; label in base vocabulary, all five skill clauses present | pass exit=0; the joined Scored-triage section is 2880 bytes and all five greps match | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 command, verbatim | exit 0 | pass exit=0; skillslint reports PASS on every check, last lines `ENFORCEMENT-BLOCK: PASS` and `POSIX-TOKEN: PASS — 16 skill file(s)` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 command, verbatim; then the post-merge recipe | exit 0 pre-merge; exit 2 on merged main | on merged main: exit 2, `--consumers: COULD-NOT-CHECK: assay:assay:statusgen:17 is not in the diff against bfef0dab238a...`, as the row's Expect says. Post-merge recipe, fresh clone checked out at 69ca08c699a1 (single parent a65c313f8c71), same command with `--base a65c313f8c71e0fefae32808e4c498b437f2543d`: pass exit=0; both entries CORROBORATED, `summary: 2 corroborated, 0 disproved, 0 unchecked`; the follow-up grep finds no leftover routing | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10 command, verbatim | exit 0; hash 60bbdc98a3e2 | pass exit=0; 308 lines, git hash-object `60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing value (gate: model, all four risk answers no, irreversible: no; enumerated anyway
over the delivering diff's non-test Go change, statusgen/placeholder.go, which introduces one
literal):

RISK-VALUE: DERIVED — riskGateLabels entry = "risk:high" @ statusgen/placeholder.go:89 — the
driver's 2026-10-08 ruling makes risk:high, and only risk:high, force the human gate; the
intake-desk skill (plugins/assay/skills/intake-desk/SKILL.md:183) defines the label set as
risk:{high,med,low}, so "risk:high" is the canonical spelling; the stage-1 lookup lower-cases
and trims each label (placeholder.go:165), so the entry has to be stored lower-case to match,
and it is. Mutations A and B above show both the presence and the placement (label set, not
keyword set) are load-bearing. A wrong value is reversible by an edit and a release.

**VERIFY: PASS** — 10 of 10 rows observed green by hand at bfef0dab238a (row 9 through its
post-merge recipe at the delivering commit); the witness's check:ci rows are could-not-run on
this darwin host only because the network-off sandbox needs Linux.
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 946fd67330e5 (10 of 10 green; row 9 through its post-merge recipe; both row 2 mutations red)

Verification-Attestation: medici-finance/assay#2935 run=457e98dbcd3764d858efe66d601a4a15e57c7f604f6b621dd6c46aa4306e6b09 source=946fd67330e5bbc6acc0fe3e0bbe9521d7786caa brief=docs/streams/statusgen/brief-17-risk-high-label-gate.md model=claude-opus-5-5 tier=strong

Fresh verifier, not the implementer, admitted by the dispatcher check before the first row and again before handback. Every row ran by hand from the detached home at 946fd67330e5 (delivering change 69ca08c699a1, single parent a65c313f8c71), with TMPDIR on a scratch directory outside the home and KUBECONFIG=/dev/null. Toolchain: go1.27.2 darwin/arm64, git 2.56.0. Row 9 is the brief's pre-merge row: on merged main the merge-base is HEAD, so it exits 2 as its Expect says. Its post-merge recipe ran in a scratch clone at the delivering commit, with that commit's parent as the base. The row 2 mutations ran in the same scratch clone and were reverted there. The home was never edited.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | pass exit=0 | build and vet print nothing | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | pass exit=0 | `--- PASS: Test Placeholder Gate Risk High Label (0.00s)`, all 11 subtests PASS. Mutation, risk:high removed from riskGateLabels: exit 1, `derivePlaceholderGate([bug risk:high], ...) = "model", want "human"`, and the upper-case and padded cases fail the same way. Mutation, moved to riskGateKeywords instead: exit 1, `derivePlaceholderGate([bug], "raise to risk:high after triage") = "human", want "model"` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | pass exit=0 | `--- PASS: Test Placeholder Gate Risk High Survives Extension (0.00s)` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | pass exit=0 | `--- PASS: Test Derive Placeholder Gate (0.00s)`, `--- PASS: Test Placeholder Gate From Config Risk Extra (0.00s)`, `--- PASS: Test Risk Gate Vocabulary Extension (0.00s)` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | pass exit=0 | `--- PASS: Test Scan Risk High Placeholder Gate (0.02s)` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | pass exit=0 | `ok github.com/medici-finance/assay/statusgen 382.245s` (inside the 600s timeout) | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | pass exit=0 | the joined Scored-triage section is 2880 bytes; the label grep and all five clause greps match | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | pass exit=0 | no FAIL line; last lines `ENFORCEMENT-BLOCK: PASS` and `POSIX-TOKEN: PASS — 17 skill file(s)` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | pass exit=0 (post-merge recipe) | on merged main: exit 2, `--consumers: COULD-NOT-CHECK: assay:assay:statusgen:17 is not in the diff against 946fd67330e5...`, which is what the row's Expect predicts. Post-merge recipe in a scratch clone at 69ca08c699a1, same command with `--base a65c313f8c71e0fefae32808e4c498b437f2543d`: exit 0, both entries CORROBORATED, `summary: 2 corroborated, 0 disproved, 0 unchecked`; the follow-up grep finds no leftover routing | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | pass exit=0 | 308 lines, git hash-object `60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |

Risk-bearing value. The brief is gate: model with all four risk answers no and irreversible: no. The enumeration covered the delivering diff's non-test changes anyway. In placeholder.go there is one new literal. The skill bullet adds no new value: it uses the existing `help wanted` and `gate: human` vocabulary.

RISK-VALUE: DERIVED — riskGateLabels entry = "risk:high" @ statusgen/placeholder.go:89 — the driver's 2026-10-08 ruling recorded in the brief says risk:high, and only risk:high, forces the human gate. The intake-desk skill (plugins/assay/skills/intake-desk/SKILL.md:183) defines the label set as risk:{high,med,low}, so this is the canonical spelling. The stage-1 lookup lower-cases and trims each label (placeholder.go:165), so the stored entry must be lower-case, and it is. Both mutations above show that the entry's presence matters and that its placement (label set, not keyword set) matters. If the value were wrong, an edit and a release would undo it.

**VERIFY: PASS** — 10 of 10 rows observed green at 946fd67330e5. Row 9 passed through its post-merge recipe at the delivering commit; on merged main it exits 2 by design.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | fail exit=2 | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-15 | assay-verifier-app[bot] @ a39ad90497c1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3072 run=231bf67d21496be4b67b50f39a483ecd4883901c50ae1f80a229fc9f0c593ddf source=a39ad90497c10cdc16413695c3ce9b74c2b098d6 brief=docs/streams/statusgen/brief-17-risk-high-label-gate.md model=claude-opus-5-5 tier=any

### Non-implementer verifier run — 2026-10-15 claude-opus-5-5 verifier at a39ad90497c1 (10 of 10 green by hand under a network-off sandbox; row 9 through its post-merge recipe; both row 2 mutations red)

Verification-Attestation: medici-finance/assay#3072 run=231bf67d21496be4b67b50f39a483ecd4883901c50ae1f80a229fc9f0c593ddf source=a39ad90497c10cdc16413695c3ce9b74c2b098d6 brief=docs/streams/statusgen/brief-17-risk-high-label-gate.md model=claude-opus-5-5 tier=any

Fresh verifier, not the implementer, admitted by the dispatcher check before the first row. The witness table directly above is the execution witness from this same run, taken on the clean tree first: on this darwin host its check:ci rows come back could-not-run because its network-off sandbox needs Linux `unshare --net`, and its row 9 records exit 2, which the row's own Expect predicts on merged main. Every row was then executed by hand from the detached home at a39ad90497c1 (delivering change 69ca08c699a1, single parent a65c313f8c71, an ancestor of a39ad90497c1), each inside macOS sandbox-exec with the profile `(version 1)(allow default)(deny network-outbound)(allow network-outbound (remote ip "localhost:*"))`. The deny was proved inside that sandbox before the runs: curl to a public host by name exit 6, curl to 1.1.1.1:443 exit 7 (Couldn't connect), loopback TCP connect succeeded; the same IP curl unsandboxed exit 0. Environment: throwaway HOME, the real GOCACHE and GOMODCACHE, GOPROXY=off, GOTOOLCHAIN=local, TMPDIR on a scratch directory outside the home, KUBECONFIG=/dev/null. Toolchain: go1.27.2 darwin/arm64, git 2.56.0, statusgen v1.0.44. Row 9's merged-main run and its post-merge recipe, and both row 2 mutations, ran in a scratch clone outside the home; each mutation was reverted there and an empty git diff over statusgen/ was confirmed. The home was edited only in this Evidence section.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | pass exit=0 | build and vet print nothing | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg17-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg17-r2.out"` | pass exit=0 | PASS line for TestPlaceholderGateRiskHighLabel (0.00s); all 11 subtests PASS. Mutation A, risk:high removed from riskGateLabels: exit 1, derivePlaceholderGate([bug risk:high], "frontend keydown bubbles to card") = "model", want "human", and the upper-case and padded subtests fail the same way. Mutation B, moved to riskGateKeywords instead: exit 1, derivePlaceholderGate([bug], "raise to risk:high after triage") = "human", want "model". Restored, diff empty, re-run exit 0 | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg17-r3.out"` | pass exit=0 | PASS line for TestPlaceholderGateRiskHighSurvivesExtension (0.00s) | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg17-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg17-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg17-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg17-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg17-r4c.out"` | pass exit=0 | PASS lines for TestDerivePlaceholderGate, TestPlaceholderGateFromConfigRiskExtra and TestRiskGateVocabularyExtension, each (0.00s) | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg17-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg17-r5.out"` | pass exit=0 | PASS line for TestScanRiskHighPlaceholderGate (0.01s) | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | pass exit=0 | ok github.com/medici-finance/assay/statusgen 281.463s (inside the 600s timeout) | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && awk '/^## /{f=/^## Scored triage/} f{$1=$1; printf "%s ", $0}' plugins/assay/skills/intake-desk/SKILL.md > "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'first writes its placeholder derives .gate: human.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e '.risk:med. and .risk:low. do not' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'does not change its gate' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'already reads .gate: model., label the issue .help wanted.' "${TMPDIR:-/tmp}/sg17-r7.txt" && grep -q -e 'must set .gate: human. in it by hand' "${TMPDIR:-/tmp}/sg17-r7.txt"` | pass exit=0 | the joined Scored-triage section is 2880 bytes; the label grep and all five clause greps match | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg17-skl" . && "${TMPDIR:-/tmp}/sg17-skl" --root ../..` | pass exit=0 | no FAIL line; last lines GUARDRAILS: PASS (33 copies), ENFORCEMENT-BLOCK: PASS, POSIX-TOKEN: PASS for 17 skill files | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 9 | `! grep -q -e '[:] follow-up statusgen/17' docs/streams/statusgen/brief-17-risk-high-label-gate.md && cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg17c" . && cd .. && "${TMPDIR:-/tmp}/sg17c" --root . --consumers --brief statusgen/17 --base "$(git merge-base refs/remotes/origin/main HEAD)" > "${TMPDIR:-/tmp}/sg17-r9.out" 2>&1 && grep -q -F -e 'summary: 2 corroborated, 0 disproved, 0 unchecked' "${TMPDIR:-/tmp}/sg17-r9.out"` | pass exit=0 (post-merge recipe) | on clean merged main (scratch clone at a39ad90497c1): exit 2, the instrument says the brief is not in the diff against a39ad90497c1, which the row's Expect predicts. In the home after the witness had appended its table, the dirty brief puts it in the working-tree diff and the instrument reports 2 unchecked, so that run exits 1 on the final grep; this is the order of this run, not the code. Post-merge recipe in a scratch clone at 69ca08c699a1, same command with --base a65c313f8c71e0fefae32808e4c498b437f2543d: exit 0, both entries CORROBORATED, summary: 2 corroborated, 0 disproved, 0 unchecked; the follow-up grep finds no leftover routing | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |
| 10 | `awk '/^func TestDerivePlaceholderGate\(/,/^}/ {print} /^func TestPlaceholderGateFromConfigRiskExtra\(/,/^}/ {print} /^func TestRiskGateVocabularyExtension\(/,/^}/ {print} /^func TestParsePlaceholderFileValid\(/,/^}/ {print} /^func TestPlaceholderNextUpFirstClass\(/,/^}/ {print}' statusgen/placeholder_test.go > "${TMPDIR:-/tmp}/sg17-r10.txt" && awk '/^func TestScanIssuesPlan\(/,/^}/ {print} /^func TestScanIssuesEmittedFilesRoundTrip\(/,/^}/ {print}' statusgen/scanissues_test.go >> "${TMPDIR:-/tmp}/sg17-r10.txt" && test "$(git hash-object "${TMPDIR:-/tmp}/sg17-r10.txt")" = 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a` | pass exit=0 | 308 lines, git hash-object 60bbdc98a3e2fd04a4acc263c251c3c61f95ee6a | 2026-10-15 | claude-opus-5-5 verifier @ a39ad90497c1 |

Risk-bearing value. The brief is gate: model with all four risk answers no and irreversible: no. The enumeration covered the delivering diff's non-test changes anyway (changelog fragment, stream README row, brief consumers routing, the intake-desk skill bullet, statusgen/placeholder.go). The only new code literal is one entry in placeholder.go; the skill bullet reuses the existing `help wanted`, `gate: human` and `gate: model` vocabulary and adds no number. No later commit after 69ca08c699a1 touched statusgen/placeholder.go or the two test files.

RISK-VALUE: DERIVED — riskGateLabels entry = "risk:high" @ statusgen/placeholder.go:89 — the driver's 2026-10-08 ruling recorded in the brief says risk:high, and only risk:high, forces the human gate. The intake-desk skill (plugins/assay/skills/intake-desk/SKILL.md:183) defines the label set as risk:{high,med,low}, so this is the canonical spelling. The stage-1 lookup lower-cases and trims each label (statusgen/placeholder.go:165), so the stored entry must be lower-case, and it is. Mutation A shows the entry's presence is load-bearing and mutation B shows its placement (label set, not keyword set at statusgen/placeholder.go:80) is load-bearing. If the value were wrong, an edit and a release would undo it.

**VERIFY: PASS** — 10 of 10 rows observed green by hand at a39ad90497c1 under a network-off sandbox. Row 9 passed through its post-merge recipe at the delivering commit; on clean merged main it exits 2 by design.


## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
