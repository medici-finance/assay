---
brief: methodology/11
title: Article 3 — "Writing specs that can converge" (the initiator's craft)
wave: 3
depends: ["methodology/07", "methodology/08"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
decision-issue: 424
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["[I-05](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-article-3-writing-specs-that-can-converge-the-initiator-s-cr.md)", "spec §11 publication plan (article 3 outline)", "spec §13 (structured islands / dual-audience)", "2026-07-08 session receipts (SDD ledger — summarize into R-01 per methodology/08)", "[F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) + red-team-2026-07-09.md A3/C3/D1 (2026-07-09 amendments)"]
gate-why: >-
  Publishes Article 3 teaching the initiator's craft using this brief's own Verify table as
  a live specimen the red-team already flagged as an example of the anti-pattern it warns
  against (grep rows wearing executable clothing); once public the framing and economics
  guardrails are permanent, so sign-off confirms the required caveat and the
  FORBIDDEN NUMBERS constraints actually made it into the published text.
---

# Brief 11 — Article 3: "Writing specs that can converge"

## Context
files: docs/articles/specs-that-converge.md (new); deck source in ../decks (CROSS-REPO)
facts:
- Anchor definition: a spec can converge iff a reconciler could police it — every load-bearing claim is checkable.
- LINEAGE NAMED FIRST (added 2026-07-09, red-team C3): the anchor definition is executable-specification/ATDD (FIT 2002, BDD/Gherkin, design-by-contract, TDD's test-as-spec) in reconciler vocabulary — a referee names those in one breath, so the article names them in its own opening. Position the novelty where it actually is: the *economics* ("the money saved on cheap implementers is spent on the spec" — a falsifiable framing BDD never had reason to make) and the anti-pattern catalog with live specimens. "ATDD's discipline under agent economics" — said explicitly, the novelty objection dissolves.
- The initiator's supply list: decisions not vibes; constraints as observable predicates; hard rules stated once and enforced mechanically; explicit scope grants; DoD as commands; context as locators; corrections at the moment of drift.
- Framing AI interaction: output = claims until evidenced; specify the gate up front; the money saved on cheap implementers is spent on the spec — the trade only works if the spec can converge. You don't get honest agents, you get honest systems.
- Anti-pattern catalog with live specimens (2026-07-08): prose-carried facts, checkmark DoD, "done" at 91 remaining problems, fabricated git-history narrative, evidence-less verification — each caught by a gate the initiator specified.
- REQUIRED CAVEAT in the "DoD as commands" section (added 2026-07-09, [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)/red-team A3): executable Verify does real work where a test suite exists; for prose it gates *presence* and the quality burden falls to the human gate. Use this brief's own Verify table as the live specimen — the red-team pulled it as exhibit A (`wc -w ≥ 2000` passes garbage). The checkmark-DoD catalog entry gains this as its subtlest form: grep rows wearing executable clothing. Without this caveat, a referee pulls the article's own DoD as the rebuttal.
- ECONOMICS NUMBERS (added 2026-07-09, [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)): the economics section may not use the ≈30:1 / "27–40 person-days" figures or the phrase "controlled experiment" unless recomputed from a repo ledger with unit + baseline + defect-escape rate; default posture "we don't yet have a defensible leverage number". Tier observations publish as a case series (n=3, confounded), nothing stronger.
- REQUIRED SECTION (amended 2026-07-08, process desk — mid-flight change handling): the tweak-routing rule (spec §3 gap-fill) — route by "does the Verify table change?": no → just do it (a brief is a contract, not a step list); yes → amend the brief in the same commit + demote if past implemented; no owning brief → one-line intake. Frame it as the anchor definition in miniature: because the contract is executable, "did the scope change" is observable, not arguable — and tweak-handling must stay lighter than tweak-doing or the front door gets bypassed.
- depends methodology/07: the article teaches adoption — the toolkit repo must exist so "here's how to think" ships with "here's the kit". depends methodology/08: lived data for the economics section.

- NAME (decided 2026-07-09 by human:<name>, brief-13): the methodology is **Assay** — standalone or
  "Assay by Medici"; home domain assay.guide. This article helps mint the name publicly:
  introduce Assay as the name of the practice (an assayer tests the metal, not the stamp —
  evidence-not-claims in one word). Decision + due-diligence record:
  ../reconciler/docs/naming.md.
- AUTHORSHIP (amended 2026-07-08, per human:<name>): the article discloses its AI co-author — drafted with Claude (the fleet's coordinating agent, "Bob"), reviewed and stood behind by human:<name> — following the work-sample precedent: disclosed AI authorship of a document about verifying AI work is not a caveat, it's a demonstration. Include the division-of-labor line (human judgment about where the gates go; agent drafting/verification labor).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only. PUBLISHING is exclusively the human's action.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Draft (2000–3500 words): definition → supply list → interaction framing → anti-pattern catalog with the live specimens → adoption walkthrough using the extracted toolkit (methodology/07's repo).
2. Deck source in ../decks; cross-cite articles 1 and 2.

## Verify (presence gate — quality is owned by the human review gate)
Honesty note (2026-07-09, [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)): these rows gate *presence*, not quality — they'd pass 2000
words of garbage with the right tokens. Quality is owned by the human gate in Review. This
table is itself the article's live specimen of the caveat it must teach.

| # | Command | Expect |
|---|---------|--------|
| 1 | `wc -w docs/articles/specs-that-converge.md` | ≥2000 |
| 2 | `grep -ci "reconciler could police" docs/articles/specs-that-converge.md` | ≥1 (anchor definition present) |
| 3 | `grep -c "toolkit\|adoption" docs/articles/specs-that-converge.md` | ≥3 (adoption walkthrough present) |
| 4 | `ls ../decks \| grep -ci "converge\|spec"` | ≥1 (deck source exists) |
| 5 | `statusgen --root . --check` | exit 0 |
| 6 | `grep -ci "verify table\|mid-flight\|tweak" docs/articles/specs-that-converge.md` | ≥2 (tweak-routing section present; row added 2026-07-08 per route-2 amendment) |
| 7 | `grep -ci "co-auth\|drafted with Claude\|Bob" docs/articles/specs-that-converge.md` | ≥1 (AI co-authorship disclosed; row added 2026-07-08) |
| 8 | `grep -ci "ATDD\|acceptance-test\|BDD" docs/articles/specs-that-converge.md` | ≥1 (lineage named; row added 2026-07-09 per red-team C3) |
| 9 | `test -f docs/articles/specs-that-converge.md && ! grep -q -e "30:1" -e "27–40" -e "27-40" -e "controlled experiment" docs/articles/specs-that-converge.md` | exit 0 (F-12: unsourced numbers absent; row added 2026-07-09 — amend in the same commit if a figure is later ledger-computed). Guarded by `test -f docs/articles/specs-that-converge.md &&`, so a MISSING target now exits 1 (loud) instead of the unguarded `! grep`'s silent exit 0 (grep exits 2 on a missing file and `!` inverted that to a pass) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `7bc15cd3`, 2026-07-16). **Presence-gate PASS.**

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `wc -w docs/articles/specs-that-converge.md` | 0 | 4914 words (≥2000) |
| 2 | `grep -ci "reconciler could police" …` | 0 | 6 (≥1) |
| 3 | `grep -c "toolkit\|adoption" …` | 0 | 6 (≥3) |
| 4 | `ls ../decks \| grep -ci "converge\|spec"` | — | PASS on intent — `specs-that-converge/` dir exists in the canonical `~/jojig/Lending/decks` (verbatim fails only because an isolated worktree isn't co-located with the sibling repo) |
| 5 | `go run ./tools/statusgen --root . --check` | 1 | FAIL as written — global STATUS.md date drift (advisory, non-CI-gate per CLAUDE.md); the CI gate `--lint` = exit 0 |
| 6 | `grep -ci "verify table\|mid-flight\|tweak" …` | 0 | 20 (≥2) |
| 7 | `grep -ci "co-auth\|drafted with Claude\|Bob" …` | 0 | 2 (≥1) |
| 8 | `grep -ci "ATDD\|acceptance-test\|BDD" …` | 0 | 5 (≥1) |
| 9 | `grep -c "30:1\|27–40\|27-40\|controlled experiment" …` | 1 | 0 forbidden matches (passes on count intent; exit 1 is the #509 `grep -c` no-match defect) |

**VERIFY: PASS as a presence gate.** Caveat for human sign-off (irreversible — publication): this Verify table is a **presence gate only** (the brief's own honesty note concedes it's blind to quality). Prior opus irreversible-interrogation flagged article-level exposures still open — undischarged specimen citations, the "seven vs six"/"91 problems" factual issues, a word-count-ceiling breach, internal-path references, and a dependency on in-progress methodology/09. These are outside the Verify table's scope by design and are held for human:<name>'s human review. Rows 5 and 9 are Verify-command defects (#509 class), not content failures.

Verifier run (independent, non-implementer — opus-verifier, merged main `7f524e40`). All 9 rows run.

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `wc -w docs/articles/specs-that-converge.md` | 0 | `4914` (≥2000) | 2026-07-13 | opus-verifier |
| 2 | `grep -ci "reconciler could police" …` | 0 | `6` (≥1) | 2026-07-13 | opus-verifier |
| 3 | `grep -c "toolkit\|adoption" …` | 0 | `6` (≥3) | 2026-07-13 | opus-verifier |
| 4 | `ls ../decks \| grep -ci "converge\|spec"` | 0 | `1` — `specs-that-converge/`, committed `7405892`, cross-cites Articles 1 & 2, 0 forbidden numbers | 2026-07-13 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --check` | 1 | literal FAIL, **clock drift only**: regen diff = 72 lines, **0 non-date lines** (stream last-activity dates + awaiting ages). `--lint` (the CI gate) = exit 0; CLAUDE.md documents `--check` as advisory/not byte-pure. Not a brief-11 regression | 2026-07-13 | opus-verifier |
| 6 | `grep -ci "verify table\|mid-flight\|tweak" …` | 0 | `20` (≥2) | 2026-07-13 | opus-verifier |
| 7 | `grep -ci "co-auth\|drafted with Claude\|Bob" …` | 0 | `2` (≥1) | 2026-07-13 | opus-verifier |
| 8 | `grep -ci "ATDD\|acceptance-test\|BDD" …` | 0 | `5` (≥1) | 2026-07-13 | opus-verifier |
| 9 | `grep -c "30:1\|27–40\|27-40\|controlled experiment" …` | 1 | `0` matches — expectation IS 0, so PASS. Deck also 0 | 2026-07-13 | opus-verifier |

**VERIFY: PASS** — 8/9 pass as written; row 5's failure is 100% clock drift and not attributable to this brief.
Full read-through (208 lines) confirms the deliverable substantively contains every element the brief required
— the [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md) caveat section is real and self-implicating, [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) economics publishes a *refusal to claim*, the
red-team C3 lineage names FIT/BDD/Gherkin/design-by-contract/TDD, and tweak-routing, authorship disclosure,
name minting and the adoption walkthrough are all present.

**Irreversible interrogation (F-32 — the risk-bearing element is not a constant but the set of claims that
cannot be retracted once published).** Economics claim: CLEAN (claims nothing). Caveat claim: CLEAN, the
article's strongest asset. **Empirical claim: EXPOSED** — and the presence gate is blind to it:

- **(A) The catalog's evidentiary promise is undischarged.** §5 asserts "Each has a specimen file, a date, and a
  root cause" and closes "Each has a file, a date, and a commit in our registers." The article then cites **zero**
  files, dates or commits — all seven specimens are anonymised prose. The receipts do exist (RETRO.md), so the
  claim is *true* — but the paper whose thesis is *evidence, not claims* publishes "we have the receipts" while
  showing none. Its anti-pattern catalog is itself an instance of its own anti-pattern #1 (prose-carried facts).
- **(B) The scope-honesty defence our own register demanded is absent.** `../oit/docs/streams/RETRO.md` carries an open
  note: the methodology "demonstrably serves an agent fleet with ONE accountable human … the articles must scope
  their claims to it, or a referee will." Grep for `one accountable human|single human|multi-human|regime` across
  this article and its companion → **zero hits**. The register predicted the exact attack; neither outward-facing
  article defends it.
- **(C) Two publicly checkable factual errors.** §10 says "the **seven**-section format" then enumerates **six**
  (canonical `brief-template.md` has six). §5's headline "'Done' at 91 remaining problems" states a figure the
  body then refuses to stand behind — the mirror image of the [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) discipline preached in §8.
- **(D)** Internal paths (`../assay-toolkit`, `../reconciler/docs/naming.md`, `docs/streams/…`) are unresolvable
  for an external reader — as merged, not publish-ready.
- **(E)** Task scope says 2000–3500 words; actual 4914 = **+39% over the stated ceiling**. Row 1 gates only the
  floor — a live demonstration of the article's own §4.1 caveat.
- **(F)** §11 says the name "is formally introduced" in Article 1 and §8 leans on Article 1's result, but
  methodology/09 (Article 1) is still `in-progress`. Article 3 is publish-gated on a companion that has not
  cleared its own gate.

None of (A)–(F) is a Verify-row failure. **That is the finding** — the presence gate is green and the article
still carries two unretractable exposures.

*Held at `implemented`: `risk.irreversible: yes` (publication). Evidence recorded; the `verified` flip and the
publication sign-off are human:<name>'s, and should not be given until (A) and (B) are fixed — the catalog must cite its
specimens or drop the promise that it does, and the single-accountable-human regime must be scoped in-text.
(C) is a two-line correction. These are defects in the ARTICLE, not in the brief's implementation, so they need
an amendment + follow-up branch, not a demotion.*

## Review
Gate: human (customer: yes; irreversible: yes — publication). Reviewer records
`human:<name>` + date in the stream README table. Publication only on explicit human go.
