---
brief: assay:assay:statusgen:16
title: '`--scan-issues`: a `risk:high` label derives `gate: human` on the issue''s placeholder (`risk:med` does not)'
why: >-
  Triage records how risky an item is as a label, `risk:high`, `risk:med` or `risk:low`. The
  scanner that turns an open issue into a dispatchable placeholder does not read that label. It
  gates the placeholder to a human only when the title or a label happens to contain one of
  three words (auth, funds, security). So an item a triage session scored as high risk is
  written `gate: model` and can be implemented and reviewed by models alone. The driver ruled
  on 2026-10-08 that `risk:high` forces the human gate and `risk:med` does not. This brief
  makes the scanner apply that.
wave: 1
depends: []
unblocks: []
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
  - "freshness-checked 2026-10-08 @ 403b8ec8c (origin/main) — `riskGateLabels` is `funds`, `security`; no Go file reads a `risk:` issue label; no open PR touches `statusgen/placeholder.go`"
consumers:
  - "statusgen/placeholder.go: follow-up statusgen/16 (this brief; one entry in the base label set and its comment)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up statusgen/16 (this brief; the scored-triage section gains the one sentence stating what `risk:high` now does)"
---

# Brief 16 — `risk:high` label derives the human gate on a placeholder

## Context

files:
- `statusgen/placeholder.go` — one entry in `riskGateLabels`, and the comment above it.
- `statusgen/placeholder_test.go` — the two new unit tests named in Verify.
- `statusgen/scanissues_test.go` — the new scan-level test named in Verify.
- `plugins/assay/skills/intake-desk/SKILL.md` — one sentence in the scored-triage section.
- `changelog/<branch-slug>.md` — the per-PR fragment this repo enforces.

single-point-of-failure: the `risk:high` label being on the issue at the moment the scanner
first writes its placeholder. Behind it sit two layers that fail on different signals: the
PR-diff risk gate (path triggers, read from the change itself, not from any label), and a human
writing `gate: human` into the placeholder by hand, which the parser honours over anything
derived.

facts (all read on main @ 403b8ec8c, 2026-10-08):
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
- **One read-time effect.** A placeholder file with no `gate:` line and a stored
  `risk:high` label will read as `gate: human` after this change, because the parser's
  fallback uses the same function. The scanner always writes the line, so this reaches only
  hand-written or older files.

Out of scope — open, NOT decided by this brief or by the ruling behind it:
1. Whether the keyword list (`auth`, `funds`, `security`) should take more vocabulary.
2. Whether reactivation (a reopened issue, or an excluded label removed) should re-derive a
   placeholder's gate and labels, and more generally whether a label change after the
   placeholder exists should ever update the stored gate.
3. Back-filling `gate: human` onto placeholders that already exist for `risk:high` issues.

Also unchanged: the PR-diff risk gate and its path triggers, the excluded-label set, the trust
gate, and `derivePlaceholderEffort`.

design-fit:
  owner: `statusgen/placeholder.go` — `derivePlaceholderGate` and the base vocabulary it reads
  contract: none — placeholder gate derivation has no row in the semantic-owner index; one function owns it and its three readers call that function
  retires: []
  weight: verbs 0, flags 0, refusals 0, base risk labels +1, rule-text lines +1 (one sentence in the intake-desk skill)
  why-add: >-
    The label entry goes INTO the owner; no second mechanism is added. Putting it in the
    deployment extension instead was considered and rejected (see facts). The skill sentence
    is added because a triage label that used to be ordering data now changes who must sign
    off, and the place the label is defined is where a triage session will look. Nothing
    existing could be removed to pay for it: the three keywords stay, by the ruling's scope.

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
- Do not edit the existing cases of `TestDerivePlaceholderGate`; they are the proof that
  funds, security and auth behave as before.
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
     pins that the brief left the derive-once rule alone; it does not settle open question 2;
   - a placeholder file with `labels: [risk:high]` and NO `gate:` line reads back `human`.
6. In the intake-desk skill, add ONE sentence to the scored-triage section, beside "Judgment
   recorded, not computed": a `risk:high` label present when the issue scanner first writes
   the item's placeholder derives `gate: human` on it; `risk:med` and `risk:low` do not; a
   label added after the placeholder exists does not change its gate. Edit only that section;
   leave every generated block alone.
7. Add the changelog fragment.

## Verify (executable — no prose-only DoD items)
Every row that runs a named test anchors its selector, writes the output to a file and asserts
that test's `--- PASS:` line, so a missing or renamed test fails the row.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go build ./... && GOWORK=off go vet ./...` | exit 0 | check:ci |
| 2 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighLabel$' -v . > "${TMPDIR:-/tmp}/sg16-r2.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighLabel' "${TMPDIR:-/tmp}/sg16-r2.out"` | exit 0; a `risk:high` label derives `human`, including the upper-case and padded forms; `risk:med`, `risk:low`, `risk:higher`, `risk` and a title that only mentions `risk:high` all derive `model`. Mutation: with `"risk:high"` removed from `riskGateLabels` the row exits 1 on the `risk:high` case (`model`, want `human`); with it added to `riskGateKeywords` instead the row exits 1 on the title-only case | check:ci +mutation |
| 3 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateRiskHighSurvivesExtension$' -v . > "${TMPDIR:-/tmp}/sg16-r3.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateRiskHighSurvivesExtension' "${TMPDIR:-/tmp}/sg16-r3.out"` | exit 0; with a deployment extension registered, and again with it cleared, `risk:high` still derives `human` and `risk:med` still derives `model` | check:ci |
| 4 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestDerivePlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg16-r4a.out" 2>&1 && grep -F -e '--- PASS: TestDerivePlaceholderGate' "${TMPDIR:-/tmp}/sg16-r4a.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestPlaceholderGateFromConfigRiskExtra$' -v . > "${TMPDIR:-/tmp}/sg16-r4b.out" 2>&1 && grep -F -e '--- PASS: TestPlaceholderGateFromConfigRiskExtra' "${TMPDIR:-/tmp}/sg16-r4b.out" && GOWORK=off go test -count=1 -timeout 300s -run '^TestRiskGateVocabularyExtension$' -v . > "${TMPDIR:-/tmp}/sg16-r4c.out" 2>&1 && grep -F -e '--- PASS: TestRiskGateVocabularyExtension' "${TMPDIR:-/tmp}/sg16-r4c.out"` | exit 0; the three tests that exist today pass with their cases unedited: `funds` and `security` labels and the `auth` / `funds` / `security` keywords gate as before, `authored` still does not, an unregistered product word still derives `model`, and the config-driven extension still gates | check:ci |
| 5 | `cd statusgen && GOWORK=off go test -count=1 -timeout 300s -run '^TestScanRiskHighPlaceholderGate$' -v . > "${TMPDIR:-/tmp}/sg16-r5.out" 2>&1 && grep -F -e '--- PASS: TestScanRiskHighPlaceholderGate' "${TMPDIR:-/tmp}/sg16-r5.out"` | exit 0; through `planScan`, the placeholder written for a `risk:high` issue carries `gate: human`, the one for a `risk:med`-only issue carries `gate: model`, the `security` one carries `gate: human`, each reads back with the same gate, an existing `gate: model` placeholder is neither re-planned nor re-read as `human`, and a gate-less file with a stored `risk:high` label reads `human` | check:ci +flow |
| 6 | `cd statusgen && GOWORK=off go test -count=1 -timeout 600s .` | exit 0; the whole package passes, so no other test depended on `risk:high` deriving `model` | check:ci |
| 7 | `grep -q -F -e '"risk:high"' statusgen/placeholder.go && grep -n -F -e 'risk:high' plugins/assay/skills/intake-desk/SKILL.md` | exit 0; the base vocabulary names the label, and the skill line printed is the scored-triage sentence stating that `risk:high` derives `gate: human`, that `risk:med` and `risk:low` do not, and that a later label does not change the gate | check:ci +dereference |
| 8 | `cd tools/skillslint && go build -o "${TMPDIR:-/tmp}/sg16-skl" . && "${TMPDIR:-/tmp}/sg16-skl" --root ../..` | exit 0; the intake-desk skill still passes every skill check after the one-sentence edit, and no generated block drifted | check:ci |
| 9 | `cd statusgen && GOWORK=off go build -o "${TMPDIR:-/tmp}/sg16c" . && cd .. && "${TMPDIR:-/tmp}/sg16c" --root . --consumers --base "$(git merge-base refs/remotes/origin/main HEAD)"` | exit 0; run on the implementing branch before merge, both `consumers:` routings above are corroborated by its diff | check |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
