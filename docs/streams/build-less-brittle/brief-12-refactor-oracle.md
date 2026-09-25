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
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format; third-pass amendment)"
sources:
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
  - "docs/refactor-oracle-template.md (planned): follow-up build-less-brittle/12 (this brief)"
  - "docs/brittle-investigation-template.md §Next act (`redesign` names the oracle as the act before the design brief): follow-up build-less-brittle/12 (this brief; ≤ 3 lines)"
  - "docs/investigations/README.md (the `-oracle.md` filename rule): follow-up build-less-brittle/12 (this brief; ≤ 2 lines)"
  - "plugins/assay/skills/author-brief/SKILL.md (a design brief from `redesign` carries `oracle:` and the five acceptance rows): follow-up build-less-brittle/12 (this brief; ≤ 3 lines, offset)"
  - "tools/desk/cmd/deskdispatch/references/review-prompt.md §Design fit first (the oracle's triage table is a material claim): follow-up build-less-brittle/12 (this brief; ≤ 2 lines, offset)"
  - "tools/desk/internal/testledger (a `characterization-untriaged` report line): follow-up build-less-brittle/12 (this brief)"
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
     `divergence:` and its reconciled reading of the intent against what the code does now.
     Source: 09's file (the oracle never re-derives it).
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

Rows run from the root of `medici-finance/assay`. The deliverable is prose plus one report
line. Rows 1–4 gate the template's shape and vocabulary, rows 5–7 dereference the commands
the template tells a session to run, row 8 is the ledger's mutation row, rows 9–12 are the
wiring and net ≤ 0 rows.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c -e '^## 1\. Intent$' -e '^## 2\. Failure modes$' -e '^## 3\. Current behaviour$' -e '^## 4\. Invariants$' -e '^## 5\. Acceptance$' docs/refactor-oracle-template.md` | `5` |
| 2 | `sed -n '/^## 3\. Current behaviour/,/^## 4\./p' docs/refactor-oracle-template.md \| grep -E '^[\|] *Test[A-Za-z0-9_]+ *[\|]' \| awk -F'\|' '{gsub(/ /,"",$3); print $3}' \| sort -u \| tr '\n' ' '` | `drop keep unknown ` (the worked example's triage table uses all three verdicts and nothing else) |
| 3 | `b=$(printf '\140'); sed -n '/^## 5\. Acceptance/,$p' docs/refactor-oracle-template.md \| grep -cE "^[\|] *[1-5] *[\|] $b"` | `5` (the acceptance rule is five runnable rows; the backtick comes from `printf '\140'` because ERE has no `\x60` escape and GNU and BSD grep disagree on it) |
| 4 | `grep -c 'Retires-test: .*accident per oracle' docs/refactor-oracle-template.md && grep -c 'unknown' docs/refactor-oracle-template.md` | ≥ `1`, then ≥ `3` (a drop leaves with 11's trailer; unknown is named as the refusal) |
| 5 | `cmd=$(sed -n '/^## 2\. Failure modes/,/^## 3\./p' docs/refactor-oracle-template.md \| grep -oE "git grep -n 'regression: [^']*' -- '\*_test.go'" \| head -1); n=$(git grep -h '^// regression: #' -- '*_test.go' \| grep -oE '#[0-9]+' \| head -1); test -n "$cmd" && eval "${cmd/<N>/${n#\#}}" \| grep -c 'regression:'` | ≥ `1` (the failure-mode read the template gives finds a real tagged test from 11's seeds) |
| 6 | `sed -n '/^## 4\. Invariants/,/^## 5\./p' docs/refactor-oracle-template.md \| grep -c 'go test ./internal/arch/' && cd tools/desk && go test ./internal/arch/ -count=1` | ≥ `1`, then `ok` (the invariants command the template names exists and is green) |
| 7 | `sed -n '/^## 1\. Intent/,/^## 2\./p' docs/refactor-oracle-template.md \| grep -c -e 'brittle-investigation-template' -e 'divergence:' && test -f docs/brittle-investigation-template.md && echo INVESTIGATION-EXISTS` | ≥ `1`, then `INVESTIGATION-EXISTS` (the intent part reads 09's file, which exists) |
| 8 | `cd tools/desk && go test ./internal/testledger/ -run TestLedgerFixture -count=1 -v \| grep -c 'characterization-untriaged: .*TestFixtureUntriaged'` | `1` (a tagged characterization test the fixture oracle does not list is reported; the listed one is not) |
| 9 | `grep -c 'oracle' docs/brittle-investigation-template.md && grep -c 'oracle.md' docs/investigations/README.md` | two counts, each ≥ `1` |
| 10 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && grep -c 'oracle:' plugins/assay/skills/author-brief/SKILL.md && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | ≥ `1`, then `NET-OK` |
| 11 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/12$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && s=$(sed -n '/^## [0-9]*\. Design fit first/,/^## [0-9]*\. /p' tools/desk/cmd/deskdispatch/references/review-prompt.md); echo "$s" \| grep -c 'oracle' && test "$(git show "$tip:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" -le "$(git show "$base:tools/desk/cmd/deskdispatch/references/review-prompt.md" \| wc -l)" && echo NET-OK` | ≥ `1`, then `NET-OK` |
| 12 | `statusgen --consumers --root . --brief build-less-brittle/12; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer assembles §2 and §3 of the template by hand
for one real module from 08's sample (`tools/desk/internal/deskkit/forge_gitlab.go` at
f7bde6bfa): do the read commands find the class instances and the exported surface, and does
the triage grammar let a real behaviour be recorded as `unknown` without inventing a source?
A template that only works on the worked example is a finding. The reviewer also confirms
the acceptance rows refuse on an `unknown` and on a failure mode with an empty test cell:
an oracle that can land with a hole in it is the drift this brief exists to stop.
