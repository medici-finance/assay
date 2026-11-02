---
brief: assay:assay:build-less-brittle:12
title: "The refactor oracle: what a redesign is coded against — intent, failure modes, triaged characterization tests, invariants, and an acceptance rule"
why: >-
  Coding is cheap now. The question is what we code AGAINST. The tree has the original briefs
  and the incident record, and they are not enough to refactor from: a brief records intent at
  authoring time and then drifts (a 2026-09-24 audit counted about 28 briefs marked
  done or implemented whose deliverable is not in the tree, so the brief is not the code); an
  incident records what broke, not what must hold; and a design-code roundtrip measured across
  five tool setups lost fidelity in both directions, with a pure-LLM edit loop degrading over
  12 iterations from a ~30% baseline and holding near 90% only under deterministic guardrails
  (Gordon, AIEWF 2026). The literature is settled on the remedy: refactoring needs a test
  harness (Fowler), the harness for code you did not write is a set of characterization tests
  that capture current behaviour before you change it (Feathers), and what the tests must
  preserve is the module's interface and invariants (Ousterhout). Generating those tests is
  now cheap; triaging them, intent versus accident, is the judgement. So before any brief-09
  `redesign` starts, the worker assembles an oracle with four parts and an acceptance rule,
  and records the triage in the PR.
wave: 5
depends: ["build-less-brittle/09", "build-less-brittle/10", "build-less-brittle/11"]
unblocks: ["build-less-brittle/13"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 4
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; third-pass amendment)"
sources:
  - "medici-finance/assay#2789; scratch-isolation amendment, freshness-checked 2026-10-13 @ 53c536312"
  - "docs/streams/build-less-brittle/spec.md — 2026-09-30 pending scope amendment"
  - "freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3: amend unfinished scope; no implementation claim"
  - "docs/streams/build-less-brittle/spec.md §3 row 14, §4.12, §6 (the redesign soft link), §11"
  - "docs/streams/build-less-brittle/spec.md §11 (Fowler: refactor first, strangler seam, 'when it's easier to rewrite'; Ousterhout: complexity, deep modules; Metz: inline the wrong abstraction and re-extract; spec-first for agent code is practice guidance, not a result; Wang et al. ICSE 2026: 7.8% of 'solved' patches fail the developer suite, so passing generated tests overstate correctness)"
  - "Fowler, Refactoring 2nd ed. (2018) ch. 2 'Self-testing code' and ch. 4; Feathers, Working Effectively with Legacy Code (2004) ch. 13 'Characterization tests' and the legacy code change algorithm; Ousterhout, A Philosophy of Software Design (2018) ch. 4–6 on interfaces, deep modules and invariants"
  - "docs/brittle-investigation-template.md (brief 09: the divergence and recommendation the oracle starts from; `redesign` is the only outcome that needs it)"
  - "docs/contracts.md §Semantic owners and §Rule register (briefs 01, 07) and tools/desk/internal/arch (brief 10): the invariants part; `// regression:` tags (brief 11): the failure-modes part"
  - "a desk-tool redesign's parity harness (spec §6: a fixed corpus chosen before the new behaviour is built, plus sanitized replay capsules; cutover briefs verify against Verify tables): the same idea at system scale, cited as a soft link only"
  - "the AIEWF 2026 talk 'The Design-Code Roundtrip That Isn't' (Gordon, ReWeaver): the drift measurement quoted in why:, from a note-taker's record of the talk, secondary and not revalidated against the recording"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no oracle, characterization-test or triage convention in docs/, spec/ or the skills; docs/investigations/ does not yet exist (brief 09 creates it)"
exec-tier: strong
exec-tier-why: "(a) the template must tell a session precisely what settles a keep-versus-drop triage, which is judgement; (b) it spans the investigation template, the semantic index, the arch tests, the regression tags and the author-brief skill."
domain: complicated
consumers:
  - "docs/refactor-oracle-template.md: fixed-here"
  - "docs/brittle-investigation-template.md §Next act (`redesign` names the oracle as the act before the design brief): fixed-here"
  - "docs/investigations/README.md (the `-oracle.md` filename rule): fixed-here"
  - "plugins/assay/skills/author-brief/SKILL.md (a design brief from `redesign` carries `oracle:` and the five acceptance rows): fixed-here"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §Design fit first (the oracle's triage table is a material claim): fixed-here"
  - "tools/desk/internal/testledger (a `characterization-untriaged` report line): fixed-here"
  - "build-less-brittle/13 (the agent-run incident refactor assembles this oracle itself): follow-up build-less-brittle/13"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 12 — The refactor oracle

## Context

files:
- `docs/refactor-oracle-template.md` (planned): NEW. The oracle's frontmatter, five sections, the triage-table grammar, the acceptance rule as five runnable rows, and a worked example over the fictional `cmd/example` module brief 09's template also uses.
- `docs/brittle-investigation-template.md` (planned): from build-less-brittle/09: `## Next act`, the `redesign` line (≤ 3 lines).
- `docs/investigations/README.md` (planned): from build-less-brittle/09: the oracle filename rule (≤ 2 lines).
- `plugins/assay/skills/author-brief/SKILL.md`: one rule under the design-fit rule 9 (≤ 3 lines, offset).
- `tools/desk/cmd/deskdispatch/references/review-prompt.md`: §"Design fit first" (≤ 2 lines, offset).
- `tools/desk/internal/testledger/ledger.go` (planned): from build-less-brittle/11, `ledger_test.go` (planned): the `// characterization:` tag and one report line (`characterization-untriaged`).
- `changelog/build-less-brittle-12.md` (planned)

facts:
- **Are briefs plus incidents sufficient to refactor against? No.** Three reasons, each with
  its evidence. (1) *A brief records intent at authoring time and drifts.* A 2026-09-24 audit
  counted about 28 briefs marked done or implemented whose deliverable is not in the tree,
  and 18 delivered but never flipped (an unpublished count; a later status-discrepancy
  audit is to re-derive it with evidence classes). A brief the tree does not match is not
  the code's specification. (2) *An incident records what broke, not what must hold.* A class
  issue (04) lists symptoms and fixes; it does not say which of the module's other behaviours
  are load-bearing. Refactoring against incidents alone preserves the fixes and nothing else.
  (3) *Partially delivered briefs mean the brief is not the code.* Where the brief and the
  code disagree, the investigation (09) decides which is right; the oracle then records the
  decision so the refactor codes against a reconciled reading, not against either document.
- **The four parts, and where each comes from.** The oracle is one file,
  `docs/investigations/<yyyy-mm-dd>-<module-slug>-oracle.md`, beside the investigation:
  1. **Intent.** The owning brief(s) and `DR-` record, quoted; the investigation's
     `divergence:` and its reconciled reading of intent at the cited revisions.
     Source: 09's file; reuse, supersede or revalidate it under the work-input amendment below.
  2. **Failure modes.** Every incident in the class issue and every findings entry whose
     `affects:` names the module, each mapped to its regression test by
     `git grep -n 'regression: .*#<N>' -- '*_test.go'` (11). A failure mode with no test is a
     row with an empty test cell, and the acceptance rule refuses it.
  3. **Current behaviour.** Characterization tests generated over the module's public
     surface: every exported function and method of the owner package the S- row names,
     one test per observable behaviour (Feathers: assert what it does, not what it should).
     Each carries `// characterization: <module> <oracle-file>` above its declaration. Then
     the **triage table**: `| test | verdict | reason | maps to |`, verdict one of `keep`
     (intent; maps to an S- row, DR, brief fact or incident), `drop` (an accident or a bug;
     maps to the failure mode or the divergence that says so), `unknown` (the record does not
     settle it). Generating the tests is cheap; the table is the judgement, and it is the
     record the PR carries.
  4. **Invariants.** The module's S- row and the contracts entry (01), the register rows
     that serve it (07), the arch rules (10) and the weight line (03). The refactor must leave
     every one green, with the owner still declared by its `// semantic:` marker.
  5. **The acceptance rule.** The refactor lands only when 1–4 hold: the intent is the
     investigation's reconciled reading; every failure mode has a tagged test that passes at
     head; every `keep` test passes at head, every `drop` test is gone with a
     `Retires-test: <name> — accident per oracle §3` trailer (11's report shows no untrailed
     line), and no `unknown` remains; the invariants are green; and the PR body carries
     `## Oracle` with the triage table verbatim. The template ships the five rows as commands
     the design brief copies into its own Verify table.
- **Triage is the judgement, so it is recorded, not inferred.** A `drop` needs a reason the
  record supports (an incident, a divergence line, a DR sentence). A `drop` with no source is
  an `unknown`. An `unknown` at landing is `NEEDS_CONTEXT` to the design brief's review:
  brief 13 says how the agent presents the residue and how the driver answers it in one
  reply; this brief only refuses to land over one.
- **What the ledger adds.** `TestReportTestLedger` (planned) (11) gains one line,
  `characterization-untriaged: <pkg>.<Test> (<oracle-file> has no row for it)`, for a tagged
  characterization test the named oracle does not list. Same package, same never-fails rule.
- **The redesign soft link.** A desk-tool redesign's parity harness (a fixed corpus chosen
  before the new behaviour is built, plus replay capsules; cutover briefs verified against
  Verify tables) is this oracle at the system scale. Where a replay corpus exists for a
  module, the oracle MAY cite it as a §3 source. Nothing here waits on it, calls it, or
  changes shape if it never lands (spec §6).
- **Scope.** The oracle precedes a `redesign`. `reconcile` and `accept` do not need one: a
  reconcile brief's `retires:` is its deletion bundle and its fix tests are 11's; an accept
  amends the record. A `redesign` without an oracle is a design brief the author-brief skill
  refuses to author.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): author-brief 768 (02 also edits it), review-prompt.md 324.

design-fit:
  owner: docs/refactor-oracle-template.md (planned; a template beside 09's docs/brittle-investigation-template.md, also planned)
  contract: none — a template; the tests it governs are tagged for 11's ledger and the invariants it cites are 01's and 10's
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (author-brief, review kit)
  why-add: n/a (no ratcheted growth). The alternative, folding the oracle into 09's investigation file, was rejected: the investigation is a read-only strong-tier reading of history, two of its four outcomes code nothing, and keeping it that way is what stops it becoming a patch generator. The oracle is assembled by whoever will code, and it contains generated tests.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- This brief writes the template and the wiring. It assembles no oracle and generates no characterization test over a real module.
- Public tree: mechanisms and public issue numbers only.
- Never make `unknown` a landable verdict to go green. An oracle with an `unknown` row is a refusal to land, by construction.

## Work-input amendment — 2026-09-30

Bind the oracle to the investigation revision, acceptance version, source/dependency
fingerprints and environment where relevant. Reusing reconciled intent means reuse for
those inputs, not an immutable claim about all future code. Before coding and acceptance,
compare relevant changes; preserve historical triage and supersede/revalidate affected
assumptions when another brief changes them. Unknown applicability cannot satisfy a keep
row. Final test evidence still names the candidate revision.

Extend the declared fictional example with an intervening policy/API change and an
unrelated edit. The acceptance/read commands must expose the pinned versus candidate
sources; the reviewer checks that the example cannot carry a stale PASS forward. Explicitly
identify semantic relevance as review-only where no deterministic proof exists. The oracle
remains required only for redesign, with no dependency on graph tooling or a new document
for ordinary tasks. In the template, label the bindings `pinned-source:` and
`candidate-source:` and place `### Intervening-change example` under `## 1. Intent`,
before `## 2. Failure modes`. Put both worked changes there without adding triage or
acceptance rows; sections 3 and 5 retain their existing counted tables.
Fit the skill/kit wording into the existing net-zero budget.

## Task

1. `docs/refactor-oracle-template.md` (planned): frontmatter (`module`, `s-row`, `investigation`,
   `divergence`, `date`, `tier: strong`), sections `## 1. Intent`, `## 2. Failure modes`,
   `## 3. Current behaviour`, `## 4. Invariants`, `## 5. Acceptance`; under each, the
   source it must quote and the read command; the triage-table grammar with the closed
   vocabulary; the five acceptance rows as runnable commands with `<module>` and
   `<oracle-file>` placeholders; a worked example over `cmd/example` whose triage table has
   ≥ 3 rows and all three verdicts, and whose acceptance section shows one row refusing on
   an `unknown`.
2. `docs/investigations/README.md` (planned) (≤ 2 lines): `<date>-<module>-oracle.md` sits beside
   the investigation; the design brief's `oracle:` line points at it.
3. `docs/brittle-investigation-template.md` (planned) `## Next act` (≤ 3 lines): on `redesign`, the
   oracle is the act before the design brief; the DR amendment cites it.
4. author-brief (≤ 3 lines, offset): a design brief raised by `redesign` carries `oracle:
   docs/investigations/<file>` in Context beside `design-fit:` and copies the template's five
   acceptance rows into its Verify table; a `redesign` brief without one is not authored.
5. Review kit §"Design fit first" (≤ 2 lines, offset): on a PR whose brief carries `oracle:`,
   read the oracle; the `## Oracle` triage table is a material claim (the existing basis).
6. testledger: the `// characterization:` tag in `Tests` (planned); the `characterization-untriaged`
   line; a fixture case (one tagged test listed in the fixture oracle, one not).
7. Changelog fragment.

## Verify (executable — no prose-only DoD items)

Run consumer checks with `TMPDIR` pointing at owned scratch outside the admitted source checkout.
This scratch-location amendment (#2789) preserves the revision selection and assertions;
historical Evidence does not verify the revised row, which needs fresh verifier admission.

Rows run from the root of `medici-finance/assay`. The deliverable is prose plus one report
line. Rows 1–4 gate the template's shape and vocabulary, rows 5–7 dereference the commands
the template tells a session to run, row 8 is the ledger's mutation row, rows 9–12 are the
wiring and net ≤ 0 rows.

Each row asserts its own counts with `test` and prints one marker line; Expect names that
marker, so no verdict is read off the last number of the output. Rows 10–12 find the delivering
change from HEAD's own history: the oldest `Brief: build-less-brittle/12` commit outside
`docs/streams` and `changelog`, on any parent, then the oldest first-parent commit of HEAD that
contains it (the merge, or the commit itself when it landed directly), diffed against that
commit's first parent. A tree that does not hold the delivery prints `NO-DELIVERY` and fails.
Rows re-authored 2026-11-01 (#4169); historical Evidence does not verify them.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `f=docs/refactor-oracle-template.md; n=$(grep -xF -e '## 1. Intent' -e '## 2. Failure modes' -e '## 3. Current behaviour' -e '## 4. Invariants' -e '## 5. Acceptance' "$f" \| sort -u \| wc -l \| tr -d ' '); echo "sections=$n"; test "$n" -eq 5 && echo SECTIONS-OK` | exit 0, output is `SECTIONS-OK` (the five section headings, each present, counted once each so a repeated heading cannot stand in for a missing one) | check |
| 2 | `f=docs/refactor-oracle-template.md; v=$(sed -n '/^## 3\. Current behaviour/,/^## 4\./p' "$f" \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '); echo "verdicts=$v"; test "$v" = 'drop keep unknown ' && echo VERDICTS-OK` | exit 0, output is `VERDICTS-OK` (the worked example's triage table uses all three verdicts and nothing else) | check |
| 3 | `f=docs/refactor-oracle-template.md; b=$(printf '\140'); n=$(sed -n '/^## 5\. Acceptance/,$p' "$f" \| grep -cE "^[\|] *[1-5] *[\|] $b"); echo "acceptance-rows=$n"; test "$n" -eq 5 && echo ACCEPTANCE-OK` | exit 0, output is `ACCEPTANCE-OK` (the acceptance rule is five runnable rows; the backtick comes from `printf '\140'` because ERE has no `\x60` escape and GNU and BSD grep disagree on it) | check |
| 4 | `f=docs/refactor-oracle-template.md; r=$(grep -c 'Retires-test: .*accident per oracle' "$f"); u=$(grep -c 'unknown' "$f"); echo "retires=$r unknown=$u"; test "$r" -ge 1 && test "$u" -ge 3 && echo VOCABULARY-OK` | exit 0, output is `VOCABULARY-OK` (a drop leaves with 11's trailer, one line or more; unknown is named as the refusal, three lines or more) | check |
| 5 | `f=docs/refactor-oracle-template.md; cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' "$f" \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && test -n "$n" && c=$(eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:') && echo "tagged=$c" && test "$c" -ge 1 && echo FAILURE-READ-OK` | exit 0, output is `FAILURE-READ-OK` (the failure-mode read the template gives finds a real tagged test from 11's seeds) | check +dereference |
| 6 | `f=docs/refactor-oracle-template.md; n=$(sed -n '/^## 4\. Invariants/,/^## 5\./p' "$f" \| grep -c 'go -C tools/desk test ./internal/arch/'); echo "named=$n"; test "$n" -ge 1 && GOPROXY=off GOTOOLCHAIN=local go -C tools/desk test ./internal/arch/ -count=1 && echo INVARIANTS-GREEN` | exit 0, output is `INVARIANTS-GREEN` (the invariants command the template names exists and is green; `GOPROXY=off GOTOOLCHAIN=local` keeps the run offline, so a cold module cache fails the row rather than fetching) | check +dereference |
| 7 | `f=docs/refactor-oracle-template.md; n=$(sed -n '/^## 1\. Intent/,/^## 2\./p' "$f" \| grep -c -e 'brittle-investigation-template' -e 'divergence:'); echo "intent-refs=$n"; test "$n" -ge 1 && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | exit 0, output is `INVESTIGATION-EXISTS` (the intent part reads 09's file, which exists; the count is asserted inside the command and printed on its own line, so the marker is the only thing Expect reads) | check +dereference |
| 8 | `n=$(cd tools/desk && GOPROXY=off GOTOOLCHAIN=local go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '); echo "lines=$n"; test "$n" -eq 2 && echo LEDGER-FIXTURE-OK` | exit 0, output is `LEDGER-FIXTURE-OK` (exactly two lines: the fixture test passes, and its own logged report line names the tagged characterization test the fixture oracle does not list; the listed one is not reported. The line is anchored on the `ledger_test.go:N:` log prefix, so the expected line echoed inside a failure message does not count: with the report line suppressed the row prints `lines=0` and fails). Row anchored 2026-10-21 (#3916): the earlier unanchored form counted `1` on that mutant | check +mutation |
| 9 | `a=$(grep -c 'refactor-oracle-template\.md' docs/brittle-investigation-template.md); r=$(grep -c -e '-oracle\.md' docs/investigations/README.md); echo "template=$a readme=$r"; test "$a" -ge 1 && test "$r" -ge 1 && echo ORACLE-WIRED` | exit 0, output is `ORACLE-WIRED` (09's template names the oracle template as the act after `redesign`, and the investigations README names the `-oracle.md` file that sits beside an investigation) | check |
| 10 | `t=$(git log --full-history --format=%H --grep='^Brief: build-less-brittle/12$' HEAD -- . ':!docs/streams' ':!changelog' \| tail -1); test -n "$t" \|\| { echo NO-DELIVERY; exit 1; }; m=$(git rev-list --first-parent HEAD \| grep -Fx -f <(echo "$t"; git rev-list --ancestry-path "$t"..HEAD) \| tail -1); test -n "$m" \|\| { echo NO-DELIVERY; exit 1; }; echo "delivery=$(git rev-parse --short=12 "$m") parent=$(git rev-parse --short=12 "$m^1")"; f=plugins/assay/skills/author-brief/SKILL.md; n=$(grep -c 'oracle:' "$f"); o=$(git show "$m:$f" \| wc -l); p=$(git show "$m^1:$f" \| wc -l); echo "oracle-lines=$n before=$p after=$o"; test "$n" -ge 1 && test "$o" -le "$p" && echo NET-OK` | exit 0, output is `NET-OK` (author-brief names `oracle:`, and the delivering change did not grow the file) | check |
| 11 | `t=$(git log --full-history --format=%H --grep='^Brief: build-less-brittle/12$' HEAD -- . ':!docs/streams' ':!changelog' \| tail -1); test -n "$t" \|\| { echo NO-DELIVERY; exit 1; }; m=$(git rev-list --first-parent HEAD \| grep -Fx -f <(echo "$t"; git rev-list --ancestry-path "$t"..HEAD) \| tail -1); test -n "$m" \|\| { echo NO-DELIVERY; exit 1; }; echo "delivery=$(git rev-parse --short=12 "$m") parent=$(git rev-parse --short=12 "$m^1")"; f=tools/desk/cmd/deskdispatch/references/review-prompt.md; n=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' "$f" \| grep -c 'oracle'); o=$(git show "$m:$f" \| wc -l); p=$(git show "$m^1:$f" \| wc -l); echo "oracle-lines=$n before=$p after=$o"; test "$n" -ge 1 && test "$o" -le "$p" && echo NET-OK` | exit 0, output is `NET-OK` (the review kit's Design fit first section names the oracle, and the delivering change did not grow the file) | check |
| 12 | `d=; t=$(git log --full-history --format=%H --grep='^Brief: build-less-brittle/12$' HEAD -- . ':!docs/streams' ':!changelog' \| tail -1); test -n "$t" \|\| { echo NO-DELIVERY; exit 1; }; m=$(git rev-list --first-parent HEAD \| grep -Fx -f <(echo "$t"; git rev-list --ancestry-path "$t"..HEAD) \| tail -1); test -n "$m" \|\| { echo NO-DELIVERY; exit 1; }; echo "delivery=$(git rev-parse --short=12 "$m") parent=$(git rev-parse --short=12 "$m^1")"; b=$(git rev-parse "$m^1") && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$m" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; test "$s" -eq 0 && echo CONSUMERS-OK` | exit 0, output is `CONSUMERS-OK` (no `consumers:` routing claim is disproved: the `consumers:` claims are judged against the delivering change itself, in a throwaway clone, diffed against its parent; the implementer replaces each self-routed entry with `fixed-here` in the same change). A disproved claim fails the row and names the claim; an unresolvable delivering change fails the row rather than passing on an empty diff. Row re-authored 2026-10-06 (#1915): the unpinned form ran against merged main, where the brief is no longer in the diff, and reported COULD-NOT-CHECK; this form runs the same way on any tree that holds the delivery (the #1908 pattern). Expect first re-written 2026-10-03 (#1862) | check |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | exit 0, output is `SOURCE-BINDINGS` (presence only; the review below checks the read commands and stale-result behavior) | check |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | pass exit=0 | sha256:789964276840 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | pass exit=0 | sha256:70df52893449 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go -C tools/desk test ./internal/arch/' && go -C tools/desk test ./internal/arch/ -count=1` | pass exit=0 | sha256:03093fa521ca | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:aff1bea21ba5 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '` | pass exit=0 | sha256:53c234e5e847 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | pass exit=0 | sha256:3c0c0f61d914 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:9a271f2a916b | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | pass exit=0 | sha256:e84db75bb66f | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4007 run=0eae6fbeeaf2f02565dc279f4f2d02de697c943315d8718d9fc3ff1a2f083a55 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/build-less-brittle/brief-12-refactor-oracle.md model=claude-opus-5-5 tier=strong

### Verifier verdict, 2026-10-22 (non-implementer, strong tier)

**VERIFY: BLOCKED** — 9/13 rows pass; row 7 could-not-run (witness parse); rows 10, 11 and 12 exit 1 on an unresolved delivering change (check-definition). No acceptance obligation was observed failing (blocker: #4169).

- Source: merged main 05dc9fbc88f2, admitted by the Verification-Attestation line above (medici-finance/assay#4007). Protected-table re-derivation: 13 rows, 0 author-added, every row from-base with its delivered Command and Expect. statusgen and the desk tools were built from source at 05dc9fbc88f2.
- Observed outputs (a re-run reproduced 12 of the 13 witness hashes; row 6's hash varies with the elapsed time `go test` prints, and the row still prints 1 then ok): row 1 prints 5; row 2 prints "drop keep unknown "; row 3 prints 5; row 4 prints 1 then 8; row 5 prints 6; row 6 prints 1 then ok for the arch package; row 7 prints 2 then INVESTIGATION-EXISTS; row 8 prints 2; row 9 prints 6 then 1; row 13 prints SOURCE-BINDINGS. Rows 1, 2, 3, 4, 5, 6, 8, 9 and 13 meet their Expect.
- Row 7: the printed output matches the Expect by reading, but the witness reads its numeric floor from the last output token, which is INVESTIGATION-EXISTS, so it records could-not-run. No acceptance credit is taken for row 7.
- Rows 10, 11, 12: the first-parent log search for the Brief: build-less-brittle/12 trailer on main returns nothing. The work landed by merge commit 52d47e5282cf (medici-finance/assay#3916), whose message carries no Brief trailer; the trailer sits on its second-parent commits (9a2362994cb4, 32912a003230, b31b22d3d8b0, 446eddf9a292). The fallback base is the merge-base of main and HEAD, which equals HEAD, so each row's base-differs-from-tip guard exits 1, as the row 12 Expect prescribes for an unresolvable delivering change. Row 11 prints 0 only because its section read sits after that failed guard.
- Supplemental probes (diagnostic only; they discharge no row) with base 52d47e5282cf~1 and tip 52d47e5282cf: the author-brief skill is 840 lines at both (6 added, 6 removed) and carries oracle: once; the review kit is 699 lines at both (5 added, 5 removed) and its Design fit first section names oracle once; statusgen --consumers in a throwaway clone exits 0 with 6 corroborated, 0 disproved, and 2 entries the gate does not judge (the follow-up and out-of-scope routings).
- Next actor: brief-author, for a definition repair of rows 7 and 10 to 12 (resolve the delivering first-parent merge when the trailer is on its second parent, and split row 7's count from its marker), then a fresh dispatch and admission (blocker: #4169).

Risk-bearing values. Frontmatter risk is all no, including irreversible. The diff touches docs, two prompt texts and the test ledger; every literal below can be undone by an edit and a release.

- RISK-VALUE: DERIVED — CharPrefix = "// characterization: " @ tools/desk/internal/testledger/ledger.go:74 — the brief fixes the tag text as `// characterization: <module> <oracle-file>` (facts, part 3; Task 6), and it parallels TagPrefix at ledger.go:70.
- RISK-VALUE: DERIVED — tag field count = 2 (len(f) == 2) @ tools/desk/internal/testledger/ledger.go:242 — the tag carries exactly two fields, module and oracle file; the ".md" suffix test on the same line follows from the oracle filename rule.
- RISK-VALUE: DERIVED — triage section marker = "## 3." @ tools/desk/internal/testledger/ledger.go:626 — the triage table lives under section 3, Current behaviour (Task 1), and the template heading at docs/refactor-oracle-template.md:133 matches that prefix.
- RISK-VALUE: DERIVED — verdict vocabulary = keep, drop, unknown @ docs/refactor-oracle-template.md:154 — the brief's closed vocabulary (facts, part 3), with unknown a refusal at landing at lines 157-161, as the ground rule requires.
- RISK-VALUE: DERIVED — acceptance rows = 5 @ docs/refactor-oracle-template.md:191 — the brief's acceptance rule has five conditions (facts, part 5); rows 1 to 5 sit at lines 198-202.
- Ranked last, not derived: the rule-text budgets (net 0 or fewer lines for the skill and kit) are reversible weight knobs; the supplemental probe above observed net 0 for both.

Witness note: verifyrun derives the Runner cell from the executing identity, including its on-behalf-of tag. The verifier did not write or edit that cell.
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | pass exit=0 | sha256:789964276840 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | pass exit=0 | sha256:70df52893449 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go -C tools/desk test ./internal/arch/' && go -C tools/desk test ./internal/arch/ -count=1` | pass exit=0 | sha256:190a2fb10912 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:aff1bea21ba5 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '` | pass exit=0 | sha256:53c234e5e847 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | pass exit=0 | sha256:3c0c0f61d914 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:9a271f2a916b | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | pass exit=0 | sha256:e84db75bb66f | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4590 run=94eba874e3606a328fa180c94fee3fbb15ac1db2729434c101d28df3525ebcf9 source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/build-less-brittle/brief-12-refactor-oracle.md model=claude-opus-5-5 tier=strong

### Verifier verdict, 2026-10-27 (non-implementer, strong tier)

**VERIFY: BLOCKED** — 9/13 rows pass; row 7 could-not-run (witness parse); rows 10, 11 and 12 exit 1 on an unresolved delivering change (check-definition). No acceptance obligation was observed failing. The Verify table is unchanged since the 2026-10-22 pass and its repair is still open (blocker: #4169).

- Source: merged main e25479a69d69, admitted by the Verification-Attestation line above. Protected-table re-derivation (deskpathguard rederive): 13 rows, 0 author-added, every row from-base with its delivered Command and Expect. Installed statusgen v1.0.63, host darwin-arm64; every row ran on the host with TMPDIR pointing at owned scratch outside the admitted checkout. No row is classed check:ci, so no container bracket applies.
- Delivery: merge commit 52d47e5282cf (#3916), first parent 543042b3d2dc. Its second parent carries the four commits with the Brief: build-less-brittle/12 trailer (9a2362994cb4, 446eddf9a292, 32912a003230, b31b22d3d8b0); none of them is on the first-parent line of main.
- Observed outputs (a manual re-run of each command printed the same values): row 1 prints 5; row 2 prints "drop keep unknown "; row 3 prints 5; row 4 prints 1 then 8; row 5 prints 6 (the template's failure-mode read, with the first seeded tag number substituted); row 6 prints 1 then ok for the arch package; row 7 prints 2 then INVESTIGATION-EXISTS; row 8 prints 2; row 9 prints 6 then 1; row 13 prints SOURCE-BINDINGS. Rows 1, 2, 3, 4, 5, 6, 8, 9 and 13 meet their Expect.
- Row 7: the printed output matches the Expect by reading, but the witness reads its numeric floor from the last output token, which is INVESTIGATION-EXISTS, so it records could-not-run. No acceptance credit is taken for row 7.
- Rows 10, 11, 12: the first-parent log search for the Brief: build-less-brittle/12 trailer on main returns nothing, so the fallback base is the merge-base of main and HEAD, which equals HEAD (e25479a69d69), and each row's base-differs-from-tip guard exits 1. Row 11 prints 0 only because its section read sits after that failed guard. This is the check-definition defect #4169 records, not an observed failure of the delivered work.
- Supplemental probes (diagnostic only; they discharge no row) with base 52d47e5282cf~1 and tip 52d47e5282cf: the author-brief skill is 840 lines at both and carries oracle: once at head; the review kit is 699 lines at both and its Design fit first section names oracle once at head; statusgen --consumers in a throwaway clone exits 0 with 6 corroborated, 0 disproved and 2 unchecked (the follow-up and out-of-scope routings, which the gate does not judge).
- Fail-first: brief-wide risk is all no and no row carries a risk-bearing, live, mutating or end-to-end tag; statusgen verifyrun --fail-first with base 52d47e5282cf~1 selected no row ("no risk-bearing row in this table").
- Next actor: brief-author, for the definition repair #4169 names (resolve the delivering first-parent merge when the trailer sits on its second parent, and split row 7's count from its marker), then a fresh dispatch and admission.

Risk-bearing values. Frontmatter risk is all no, including irreversible. The delivered diff touches docs, two prompt texts and the test ledger; every literal below can be undone by an edit and a release.

- RISK-VALUE: DERIVED — CharPrefix = "// characterization: " @ tools/desk/internal/testledger/ledger.go:74 — the brief fixes the tag text as `// characterization: <module> <oracle-file>` (facts, part 3; Task 6), and it parallels TagPrefix on the same file's regression tag.
- RISK-VALUE: DERIVED — tag field count = 2 (len(f) == 2) @ tools/desk/internal/testledger/ledger.go:242 — the tag carries exactly two fields, module and oracle file; the ".md" suffix test on the same line follows from the oracle filename rule.
- RISK-VALUE: DERIVED — triage section marker = "## 3." @ tools/desk/internal/testledger/ledger.go:626 — the triage table lives under section 3, Current behaviour (Task 1), and the template's section 3 heading at docs/refactor-oracle-template.md:133 matches that prefix.
- RISK-VALUE: DERIVED — verdict vocabulary = keep, drop, unknown @ docs/refactor-oracle-template.md:154 — the brief's closed vocabulary (facts, part 3), with unknown a refusal at landing (line 157 onward), as the ground rule requires.
- RISK-VALUE: DERIVED — acceptance rows = 5 @ docs/refactor-oracle-template.md:196 — the brief's acceptance rule has five conditions (facts, part 5); rows 1 to 5 sit at lines 198-202.
- Ranked last, not derived: the rule-text budgets (net 0 or fewer lines for the skill and kit) are reversible weight knobs; the supplemental probe above observed net 0 for both at the delivering merge.

Witness note: verifyrun derives the Runner cell from the executing identity, including its on-behalf-of tag. The verifier did not write or edit that cell.
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | pass exit=0 | sha256:789964276840 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | pass exit=0 | sha256:70df52893449 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go -C tools/desk test ./internal/arch/' && go -C tools/desk test ./internal/arch/ -count=1` | pass exit=0 | sha256:85b96a03632c | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:aff1bea21ba5 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '` | pass exit=0 | sha256:53c234e5e847 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | pass exit=0 | sha256:3c0c0f61d914 | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:9a271f2a916b | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | pass exit=0 | sha256:e84db75bb66f | 2026-10-28 | assay-verifier-app[bot] @ c70ef7d2cf63 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4743 run=a11383caa67b08d501fe20e2ad1ef4b73da76acff4e690d74c24b7a8d947240e source=c70ef7d2cf636bcadb0605e8fb20145cf0cd99b2 brief=docs/streams/build-less-brittle/brief-12-refactor-oracle.md model=claude-opus-5-5 tier=strong

### Verifier verdict, 2026-10-28 (non-implementer, strong tier)

**VERIFY: BLOCKED** — 9/13 rows pass; row 7 could-not-run (witness parse); rows 10, 11 and 12 exit 1 on an unresolved delivering change (check-definition). No acceptance obligation was observed failing. The Verify table is unchanged since the 2026-10-27 pass and its repair is still open (blocker: #4169).

- Source: merged main c70ef7d2cf63, admitted by the Verification-Attestation line above. Protected-table re-derivation (deskpathguard rederive): 13 rows, 0 author-added, every row from-base with its delivered Command and Expect. Installed statusgen v1.0.64, host darwin-arm64; every row ran on the host with TMPDIR pointing at owned scratch outside the admitted checkout. No row is classed check:ci, so no container bracket applies.
- Delivery: merge commit 52d47e5282cf (#3916), first parent 543042b3d2dc, second parent 15b2821d066f. The commits carrying the Brief: build-less-brittle/12 trailer sit on the second-parent side; the merge message itself carries no trailer, so none is on the first-parent line of main.
- Observed outputs (a manual re-run of each command printed the same values): row 1 prints 5; row 2 prints "drop keep unknown "; row 3 prints 5; row 4 prints 1 then 8; row 5 prints 6; row 6 prints 1 then ok for the arch package; row 7 prints 2 then INVESTIGATION-EXISTS; row 8 prints 2; row 9 prints 6 then 1; row 13 prints SOURCE-BINDINGS. Rows 1, 2, 3, 4, 5, 6, 8, 9 and 13 meet their Expect. Row 6's hash varies with the elapsed time go test prints.
- Row 7: the printed output matches the Expect by reading, but the witness reads its numeric floor from the last output token, which is INVESTIGATION-EXISTS, so it records could-not-run. No acceptance credit is taken for row 7.
- Rows 10, 11, 12: the first-parent log search for the trailer on main returns no commit, so the fallback base is the merge-base of main and HEAD, which equals HEAD (c70ef7d2cf63), and each row's base-differs-from-tip guard exits 1. Row 11 prints 0 only because its section read sits after that failed guard. This is the check-definition defect #4169 records, not an observed failure of the delivered work.
- Supplemental probes (diagnostic only; they discharge no row) with base 52d47e5282cf~1 and tip 52d47e5282cf: the author-brief skill is 840 lines at both and carries oracle: once at head; the review kit is 699 lines at both and its Design fit first section names oracle once at head; statusgen --consumers in a throwaway clone exits 0 with 6 corroborated, 0 disproved and 2 unchecked (the follow-up and out-of-scope routings, which the gate does not judge).
- Fail-first: brief-wide risk is all no and no row carries a risk-bearing, live, mutating or end-to-end tag; statusgen verifyrun --fail-first with base 52d47e5282cf~1 selected no row (every row base=- not-selected).
- Next actor: brief-author, for the definition repair #4169 names (resolve the delivering first-parent merge when the trailer sits on its second parent, and split row 7's count from its marker), then a fresh dispatch and admission.

Risk-bearing values. Frontmatter risk is all no, including irreversible. The delivered diff (52d47e5282cf~1 to c70ef7d2cf63) touches docs, two prompt texts and the test ledger; every literal below can be undone by an edit and a release.

- RISK-VALUE: DERIVED — CharPrefix = "// characterization: " @ tools/desk/internal/testledger/ledger.go:74 — the brief fixes the tag text as `// characterization: <module> <oracle-file>` (facts, part 3; Task 6), and it parallels TagPrefix, the regression tag, in the same file.
- RISK-VALUE: DERIVED — tag field count = 2 (len(f) == 2) @ tools/desk/internal/testledger/ledger.go:242 — the tag carries exactly two fields, module and oracle file; the ".md" suffix test on the same line follows from the oracle filename rule.
- RISK-VALUE: DERIVED — triage section marker = "## 3." @ tools/desk/internal/testledger/ledger.go:626 — the triage table lives under section 3, Current behaviour (Task 1), and the template's section 3 heading at docs/refactor-oracle-template.md:133 matches that prefix.
- RISK-VALUE: DERIVED — report line = "characterization-untriaged: %s.%s (%s has no row for it)" @ tools/desk/internal/testledger/ledger.go:611 — the brief's facts fix the line as characterization-untriaged, the package and test, then the oracle file that has no row for it.
- RISK-VALUE: DERIVED — verdict vocabulary = keep, drop, unknown @ docs/refactor-oracle-template.md:152-157 — the brief's closed vocabulary (facts, part 3), with unknown a refusal at landing (lines 156-161), as the ground rule requires.
- RISK-VALUE: DERIVED — acceptance rows = 5 @ docs/refactor-oracle-template.md:198-202 — the brief's acceptance rule has five conditions (facts, part 5), one row each under the section 5 heading at line 189.
- Ranked last, not derived: the rule-text budgets (net 0 or fewer lines for the skill and kit) are reversible weight knobs; the supplemental probe above observed net 0 for both at the delivering merge.

Witness note: verifyrun derives the Runner cell from the executing identity, including its on-behalf-of tag. The verifier did not write or edit that cell.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | pass exit=0 | sha256:789964276840 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | pass exit=0 | sha256:70df52893449 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go -C tools/desk test ./internal/arch/' && go -C tools/desk test ./internal/arch/ -count=1` | pass exit=0 | sha256:c9b3b3c59a99 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:aff1bea21ba5 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '` | pass exit=0 | sha256:53c234e5e847 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | pass exit=0 | sha256:3c0c0f61d914 | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:9a271f2a916b | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | pass exit=0 | sha256:e84db75bb66f | 2026-10-29 | assay-verifier-app[bot] @ 499bc4184905 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4857 run=9a79606ca3783b525eb5047ef73fb4c565b011cd7012269297f6d99b3492a0bb source=499bc41849052646b5682f96c6f29ac30a103c5c brief=docs/streams/build-less-brittle/brief-12-refactor-oracle.md model=claude-opus-5-5 tier=strong

### Verifier verdict, 2026-10-29 (non-implementer, strong tier)

**VERIFY: BLOCKED** — 9/13 rows pass; row 7 could-not-run (witness parse); rows 10, 11 and 12 exit 1 on an unresolved delivering change (check-definition). No acceptance obligation was observed failing. The Verify table is unchanged since the 2026-10-28 pass and its repair is still open (blocker: #4169).

- Source: merged main 499bc4184905, admitted by the Verification-Attestation line above. Protected-table re-derivation (deskpathguard rederive): 13 rows, 0 author-added, every row from-base with its delivered Command and Expect. Installed statusgen v1.0.65, host darwin-arm64; every row ran on the host with TMPDIR pointing at owned scratch outside the admitted checkout. No row is classed check:ci, so no container bracket applies.
- Delivery: merge commit 52d47e5282cf (#3916), first parent 543042b3d2dc, second parent 15b2821d066f. The commits carrying the Brief: build-less-brittle/12 trailer (9a2362994cb4, 32912a003230, b31b22d3d8b0) sit on the second-parent side; the merge message carries no trailer, so none is on the first-parent line of main.
- Observed outputs (a manual re-run of each command printed the same values): row 1 prints 5; row 2 prints "drop keep unknown "; row 3 prints 5; row 4 prints 1 then 8; row 5 prints 6 (the template's failure-mode read with #3752, the first seeded tag, substituted); row 6 prints 1 then ok for the arch package; row 7 prints 2 then INVESTIGATION-EXISTS; row 8 prints 2; row 9 prints 6 then 1; row 13 prints SOURCE-BINDINGS. Rows 1, 2, 3, 4, 5, 6, 8, 9 and 13 meet their Expect. Row 6's hash varies with the elapsed time go test prints.
- Row 7: the printed output matches the Expect by reading, but the witness reads its numeric floor from the last output token, which is INVESTIGATION-EXISTS, so it records could-not-run. No acceptance credit is taken for row 7.
- Rows 10, 11, 12: the first-parent log search for the trailer on main returns no commit, so the fallback base is the merge-base of main and HEAD, which equals HEAD (499bc4184905), and each row's base-differs-from-tip guard exits 1. Row 11 prints 0 only because its section read sits after that failed guard. This is the check-definition defect #4169 records, not an observed failure of the delivered work.
- Supplemental probes (diagnostic only; they discharge no row) with base 52d47e5282cf~1 (543042b3d2dc) and tip 52d47e5282cf: the author-brief skill is 840 lines at both (6 added, 6 removed) and carries oracle: once; the review kit is 699 lines at both (5 added, 5 removed) and its Design fit first section names oracle once; statusgen --consumers in a throwaway clone exits 0 with 6 corroborated, 0 disproved and 2 unchecked (the follow-up and out-of-scope routings, which the gate does not judge).
- Fail-first: brief-wide risk is all no and no row carries a risk-bearing, live, mutating or end-to-end tag; statusgen verifyrun --fail-first --dry-run with base 52d47e5282cf~1 selected no row ("no risk-bearing row in this table; every row is recorded not-selected at base").
- Next actor: brief-author, for the definition repair #4169 names (resolve the delivering first-parent merge when the trailer sits on its second parent, and split row 7's count from its marker), then a fresh dispatch and admission.

Risk-bearing values. Frontmatter risk is all no, including irreversible. The delivered diff (52d47e5282cf~1 to 52d47e5282cf, 13 files outside the stream docs) touches docs, two prompt texts and the test ledger; every literal below can be undone by an edit and a release.

- RISK-VALUE: DERIVED — CharPrefix = "// characterization: " @ tools/desk/internal/testledger/ledger.go:74 — the brief fixes the tag text as `// characterization: <module> <oracle-file>` (facts, part 3; Task 6), and it parallels TagPrefix, the regression tag, at line 70 of the same file.
- RISK-VALUE: DERIVED — tag field count = 2 (len(f) == 2) @ tools/desk/internal/testledger/ledger.go:242 — the tag carries exactly two fields, module and oracle file; the ".md" suffix test on the same line follows from the oracle filename rule.
- RISK-VALUE: DERIVED — triage section marker = "## 3." @ tools/desk/internal/testledger/ledger.go:626 — the triage table lives under section 3, Current behaviour (Task 1), and the template's section 3 heading at docs/refactor-oracle-template.md:133 matches that prefix.
- RISK-VALUE: DERIVED — report line = "characterization-untriaged: %s.%s (%s has no row for it)" @ tools/desk/internal/testledger/ledger.go:611 — the brief's facts fix the line as characterization-untriaged, the package and test, then the oracle file that has no row for it.
- RISK-VALUE: DERIVED — verdict vocabulary = keep, drop, unknown @ docs/refactor-oracle-template.md:153-157 — the brief's closed vocabulary (facts, part 3), with unknown a refusal at landing (lines 156-160), as the ground rule requires.
- RISK-VALUE: DERIVED — acceptance rows = 5 @ docs/refactor-oracle-template.md:198-202 — the brief's acceptance rule has five conditions (facts, part 5), one row each under the section 5 heading at line 189.
- Ranked last, not derived: the rule-text budgets (net 0 or fewer lines for the skill and kit) are reversible weight knobs; the supplemental probe above observed net 0 for both at the delivering merge.

Witness note: verifyrun derives the Runner cell from the executing identity, including its on-behalf-of tag. The verifier did not write or edit that cell.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | pass exit=0 | sha256:789964276840 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | pass exit=0 | sha256:f0b5c2c2211c | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | pass exit=0 | sha256:70df52893449 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go -C tools/desk test ./internal/arch/' && go -C tools/desk test ./internal/arch/ -count=1` | pass exit=0 | sha256:29b780019af0 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | could-not-run exit=0 — Expect requires a count ≥ 1 but the output carries no number to read | sha256:aff1bea21ba5 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -cE -e '^--- PASS: TestLedgerFixture ' -e '^ +ledger_test\.go:[0-9]+: characterization-untriaged: pkg/charz\.TestFixtureUntriaged '` | pass exit=0 | sha256:53c234e5e847 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | pass exit=0 | sha256:3c0c0f61d914 | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | fail exit=1 | sha256:9a271f2a916b | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; b=$(git rev-parse "$base") && t=$(git rev-parse "$tip") && test "$b" != "$t" && d=$(mktemp -d "${TMPDIR:-/tmp}/bl12-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach "$t" && statusgen --consumers --root "$d" --brief build-less-brittle/12 --base "$b"; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
| 13 | `grep -q "pinned-source:" docs/refactor-oracle-template.md && grep -q "candidate-source:" docs/refactor-oracle-template.md && awk '/^## / {p=($0 == "## 1. Intent")} p && /^### Intervening-change example$/ {found=1} END {exit !found}' docs/refactor-oracle-template.md && echo SOURCE-BINDINGS` | pass exit=0 | sha256:e84db75bb66f | 2026-10-30 | assay-verifier-app[bot] @ 0006683ef1c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5044 run=907ea8c29a580cac9f45a3e71a636eed67fc38a25f5c9d96916269461f37c097 source=0006683ef1c8dc7341d0c2e67a4cb6b566e7814e brief=docs/streams/build-less-brittle/brief-12-refactor-oracle.md model=claude-opus-5-5 tier=strong

### Verifier verdict, 2026-10-30 (non-implementer, strong tier)

**VERIFY: BLOCKED** — 9/13 rows pass; row 7 could-not-run (witness parse); rows 10, 11 and 12 exit 1 on an unresolved delivering change (check-definition). No acceptance obligation was observed failing. The Verify table is unchanged since the 2026-10-29 pass and its repair is still open (blocker: #4169, read open on 2026-10-30).

- Source: merged main 0006683ef1c8, admitted by the Verification-Attestation line above. Installed statusgen v1.0.67, host darwin-arm64; every row ran on the host with TMPDIR pointing at owned scratch outside the admitted checkout. No row is classed check:ci, so no container bracket applies.
- Delivery: merge commit 52d47e5282cf (#3916), first parent 543042b3d2dc, second parent 15b2821d066f. The four commits carrying the Brief: build-less-brittle/12 trailer (9a2362994, 446eddf9a, 32912a003, b31b22d3d) sit on the second-parent side; the first-parent search on main returns zero commits.
- Observed outputs (a manual re-run of each command printed the same values): row 1 prints 5; row 2 prints "drop keep unknown "; row 3 prints 5; row 4 prints 1 then 8; row 5 prints 6; row 6 prints 1 then ok for the arch package; row 7 prints 2 then INVESTIGATION-EXISTS; row 8 prints 2; row 9 prints 6 then 1; row 13 prints SOURCE-BINDINGS. Rows 1, 2, 3, 4, 5, 6, 8, 9 and 13 meet their Expect. Row 6's hash varies with the elapsed time go test prints.
- Row 7: the printed output matches the Expect by reading, but the witness reads its numeric floor from the last output token, which is INVESTIGATION-EXISTS, so it records could-not-run. No acceptance credit is taken for row 7.
- Rows 10, 11, 12: with no first-parent commit found, the fallback base is the merge-base of main and HEAD, which equals HEAD (0006683ef1c8), and each row's base-differs-from-tip guard exits 1. Row 11 prints 0 only because its section read sits after that failed guard. This is the check-definition defect #4169 records, not an observed failure of the delivered work.
- Supplemental probes (diagnostic only; they discharge no row) with base 52d47e5282cf~1 (543042b3d2dc) and tip 52d47e5282cf: the author-brief skill is 840 lines at both (6 added, 6 removed) and carries oracle: once at head; the review kit is 699 lines at both (5 added, 5 removed) and its Design fit first section names oracle once at head; statusgen --consumers in a throwaway clone exits 0 with 6 corroborated, 0 disproved and 2 unchecked (the follow-up and out-of-scope routings, which the gate does not judge).
- Fail-first: brief-wide risk is all no and no row carries a risk-bearing, live, mutating or end-to-end tag; statusgen verifyrun --fail-first --dry-run with base 52d47e5282cf~1 selected no row ("no risk-bearing row in this table; every row is recorded not-selected at base").
- Re-derivation limit, new in this pass: deskpathguard rederive reports 13 rows, 0 author-added, but with base=0006683ef1c8, which is HEAD in this detached home, so its from-base result compares the table with itself and establishes nothing about the original table. Compared directly, the table at the delivery merge-base 63b43bb06452 differs from the delivered one: #3916 added the Class column and amended rows 6 and 8; the first-parent table before the merge equals the merge-base table, and the current table equals the delivered one. #3916 carries no wrote-to-the-test label, so this run takes the delivered table as governing and records no Verification-Table marker. Supplemental runs of the merge-base definitions at this source (no acceptance credit): row 6 prints 0 and exits 1, because the template names the go -C form its grep does not match; row 8 prints 1 and exits 0. The desk or the #4169 repair should decide whether the amendment needs closure when the table next changes.
- Next actor: brief-author, for the definition repair #4169 names (resolve the delivering first-parent merge when the trailer sits on its second parent, and split row 7's count from its marker), then a fresh dispatch and admission.

Risk-bearing values. Frontmatter risk is all no, including irreversible. The delivered diff (52d47e5282cf~1 to 52d47e5282cf, outside the stream docs) touches docs, two prompt texts and the test ledger; every literal below can be undone by an edit and a release.

- RISK-VALUE: DERIVED — CharPrefix = "// characterization: " @ tools/desk/internal/testledger/ledger.go:74 — the brief fixes the tag text as `// characterization: <module> <oracle-file>` (facts, part 3; Task 6), and it parallels TagPrefix, the regression tag, at line 70 of the same file.
- RISK-VALUE: DERIVED — tag field count = 2 (len(f) == 2) @ tools/desk/internal/testledger/ledger.go:242 — the tag carries exactly two fields, module and oracle file; the ".md" suffix test on the same line follows from the oracle filename rule.
- RISK-VALUE: DERIVED — triage section marker = "## 3." @ tools/desk/internal/testledger/ledger.go:626 — the triage table lives under section 3, Current behaviour (Task 1), and the template's section 3 heading at docs/refactor-oracle-template.md:133 matches that prefix.
- RISK-VALUE: DERIVED — report line = "characterization-untriaged: %s.%s (%s has no row for it)" @ tools/desk/internal/testledger/ledger.go:611 — the brief's facts fix the line as characterization-untriaged, the package and test, then the oracle file that has no row for it.
- RISK-VALUE: DERIVED — verdict vocabulary = keep, drop, unknown @ docs/refactor-oracle-template.md:154-157 — the brief's closed vocabulary (facts, part 3), with unknown a refusal at landing (lines 157-161), as the ground rule requires.
- RISK-VALUE: DERIVED — acceptance rows = 5 @ docs/refactor-oracle-template.md:198-202 — the brief's acceptance rule has five conditions (facts, part 5), one row each under the section 5 heading at line 189.
- Ranked last, not derived: the rule-text budgets (net 0 or fewer lines for the skill and kit) are reversible weight knobs; the supplemental probe above observed net 0 for both at the delivering merge.

Witness note: verifyrun derives the Runner cell from the executing identity, including its on-behalf-of tag. The verifier did not write or edit that cell.


## Review
Gate: model (from frontmatter). The reviewer assembles §2 and §3 of the template by hand
for one real markable module from 08's sample: `tools/desk/cmd/deskflip` (it holds the sample's
`flip.go`; a `cmd/<verb>` directory is a module, and `S-review-verdict` names it), at the PR's
merge-base. Not `forge_gitlab.go`, the sample's top file: a bare file is never marked and no
`S-` row names it (docs/contracts.md §Brittle marks). Do the read commands find the class
instances and the exported surface, and does the triage grammar let a real behaviour be
recorded as `unknown` without inventing a source?
A template that only works on the worked example is a finding. The reviewer also confirms
the acceptance rows refuse on an `unknown` and on a failure mode with an empty test cell:
an oracle that can land with a hole in it is the drift this brief exists to stop.

The reviewer also runs the template's read/acceptance commands over its pinned and candidate
example sources, records both revisions, and walks the policy/API change and unrelated edit.
The relevant change must require revalidation; unchanged analysis may be reused only with an
applicability basis, never by retargeting old PASS evidence. Record source links, outputs and
semantic relevance judgment in the review; row 13's presence checks alone do not satisfy this.
