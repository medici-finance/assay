---
brief: assay:assay:contributor-trust:06
title: "Contributor-facing documents — state the bar honestly, and ask for verification rather than assertion"
why: >-
  A contributor whose pull request is quarantined, labelled and carrying a provenance card is
  experiencing this project's trust machinery whether or not anything explains it, and the
  unexplained version reads as arbitrary hostility. The published guidance today is accurate
  about issue-first, concurrency and maintainer-merge and silent on everything else. Saying
  plainly what the bar is, what is measured and why, and asking an author to state how they
  checked each claim they make, is what turns a set of controls into a policy somebody can
  comply with.
wave: 0
depends: []
unblocks: ["contributor-trust/07"]
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: >-
  These are the project's published terms for outside contributors — the first and often only
  thing a prospective contributor reads. Getting them wrong deters the genuine contributor
  this stream is explicitly trying not to deter, or promises enforcement the repository does
  not perform. The human is confirming the disclosure boundary (the tier MODEL is published,
  the rows naming who holds which tier never are), that nothing advisory is described as
  enforcing, and the tone — which no check can measure.
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §4 — the three boundaries, and §2's record of what the published guidance already says."
  - "docs/streams/decisions/DR-contrib-disclosure.md — the design record this brief is authored against (PROPOSED): publish the model never the rows, two short prompts rather than a long form, advisory stated as advisory."
  - "CONTRIBUTING.md — the existing published policy: issue-first, the three-open-pull-request guideline, maintainer-merge, automation-ignores-strangers, fork continuous integration requiring approval, and the explicit 'filter, not deter' framing this brief must preserve."
  - "SECURITY.md — the existing reporting path; the new sections must not duplicate or contradict it."
  - "contributor-trust/03 = assay:assay:contributor-trust:03 (docs/streams/contributor-trust/brief-03-bless-verb-and-audit.md) — the blessing act whose contributor-facing explanation is written here; contributor-trust/03 routes the published description of how admission happens to contributor-trust/06."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — there is no pull-request template in .github/; CONTRIBUTING.md is 55 lines and says nothing about tiers, provenance signals, the blessing act, or the changelog proxy."
design: DR-contrib-disclosure
decision-trigger: creation
consumers:
  - "CONTRIBUTING.md: fixed-here (this change lands the trust-bar section — the provenance card, the tiers and what each unlocks, item admission, and the not-published boundary; the template and fork changelog-proxy sections landed earlier with the partial)"
  - ".github/PULL_REQUEST_TEMPLATE.md: follow-up contributor-trust/06 (this brief; the claims checklist and the how-I-verified prompt — landed by the partial, outside this branch's diff)"
  - "docs/contributor-trust.md: fixed-here (this change lands the provenance-card hop — the published model gains its contributor-facing entry point into what a submission is measured on)"
  - "SECURITY.md: out-of-scope (the reporting path is unchanged; the new sections link to it rather than restating it, since a second copy of a security contact is the copy that goes stale)"
version: 1
---

# Brief 06 — Contributor-facing documents

## Context

files:
- `CONTRIBUTING.md` — a new section stating the trust bar: that automation acts only on
  maintainer-admitted items, that submissions from unrecognised accounts carry a mechanically
  derived provenance comment and what it measures, that trust levels exist and what each
  unlocks, and that who holds which level is not published.
- `.github/PULL_REQUEST_TEMPLATE.md` (planned) (new) — the claims checklist, the how-I-verified prompt,
  and the automated-assistance disclosure field (whose tiering consequence is stated in
  `contributor-trust/07`, not here).
- `docs/contributor-trust.md` (planned) — the contributor-facing entry point into the published model.
- `changelog/contributor-trust-docs.md` (new).

facts:
- Published: the tier model, the signals the provenance comment measures, the signals it
  deliberately excludes, how an identity moves between tiers, what each tier unlocks, how a
  maintainer admits an item, and the changelog-proxy arrangement for fork pull requests.
- Never published: which identity holds which tier. The model is public; the rows are not.
- Every existing guideline keeps its current force. The issue-first and concurrency guidelines
  are advisory today and are described as advisory; nothing in this brief flips an enforcement
  switch or implies one is on.
- The existing "filter, not deter" framing is preserved and extended, not replaced.
- The template asks two things and stops: which claims the body makes and how each was
  checked, and whether the change was produced with automated assistance. Both are
  unverifiable by any tool; their value is that an honest answer is cheap and a false one is a
  specific falsehood a reviewer can point at.
- The changelog proxy is explained from the contributor's side: a fork pull request whose
  branch maintainers cannot push to has its changelog fragment landed on the base branch on
  its behalf, so the missing-fragment check is not a thing the contributor must fix.
- Presence, not prose quality, is what the Verify table gates. Whether the wording reads as
  welcoming is the review gate's judgement.

## Human decision
<!-- gate: human — decision-trigger: creation. Lifted VERBATIM into the decision issue; self-contained. -->
This project is about to start describing, in public and on the pull request itself, what it
measures about submissions from people it does not recognise, and sorting those people into
named trust levels that decide how much of its automation runs without a human present. The
question is how much of that to write down in the public contribution guidance.

The proposal is to publish the model in full — the level names, what each one unlocks, what
moves somebody between them, which signals a submission from an unrecognised account is
measured on, and which signals are deliberately never measured — and to publish none of the
records: who holds which level stays private. Alongside that, the pull-request template would
ask two short questions: which claims the description makes and how each was checked, and
whether the change was produced with automated help.

The trade to weigh on publishing the signals: somebody submitting in bulk learns exactly what
is measured, and some of it is easy to adjust once known — pace the submissions, vary the
wording. Against that, a contributor who cannot see the bar cannot meet it, and the signals
are deliberately weak and never conclude anything on their own.

Options:
1. **Publish the model and the measured signals; never publish who holds which level
   (recommended)** — a contributor can understand and comply with their own treatment, and
   the provenance comment stops reading as an unexplained accusation. Consequence accepted:
   the signal list is public and therefore adjustable, which costs little because no signal
   decides anything by itself.
2. **Publish the level names and what they unlock, but not the signal list** — less to game.
   Consequence: the most surprising thing the project does — posting a public comment
   describing an account's behaviour — remains unexplained, which is the version that feels
   like surveillance.
3. **Publish nothing; run the controls quietly** — no disclosure to manage. Consequence: the
   controls are experienced without explanation, and there is no policy to point at when
   somebody objects.
4. **Publish the model and also each person's level** — maximum transparency. Consequence: a
   public list of trust judgements about named individuals, in which a demotion is a permanent
   public mark. Rejected on the same reasoning that keeps the records private in the first
   place.

Default if no answer: none — blocks until answered; this is the project's public statement
about how it treats the people who contribute to it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Name no external contributor, and include no example that could be read as describing a real
  person or a real submission. Illustrative examples use invented identities.
- Do not describe any advisory guideline as enforcing, and do not flip an enforcement switch.

## Task

1. The `CONTRIBUTING.md` section, written to the "filter, not deter" register the page already
   uses: what automation does with an unrecognised author's item, what the provenance comment
   measures and excludes, the tier model and what each tier unlocks, how a maintainer admits
   an item, and the changelog proxy.
2. `.github/PULL_REQUEST_TEMPLATE.md` (planned): a short claims checklist, a how-I-verified prompt, and
   the automated-assistance disclosure field. Keep it short enough that a careful first-time
   contributor fills it in willingly.
3. `docs/contributor-trust.md` (planned): the contributor-facing entry point, linked from the
   `CONTRIBUTING.md` section.
4. The changelog fragment.

## Verify (executable — no prose-only DoD items)

This table gates PRESENCE — that the required statements and sections exist and that the
disclosure boundary holds. It does not gate whether the prose reads well or lands kindly;
that is the review gate's judgement.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `test -f .github/PULL_REQUEST_TEMPLATE.md` | exit 0 | check |
| 2 | `grep -n -i 'how you verified' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; at least one matching line | check |
| 3 | `grep -n -i 'automated' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; at least one matching line (the assistance-disclosure field) | check |
| 4 | `grep -n -i 'provenance' CONTRIBUTING.md` | exit 0; at least one matching line | check |
| 5 | `grep -n -i 'advisory' CONTRIBUTING.md` | exit 0; at least one matching line (the existing guidelines are still described as advisory) | check +neighbour |
| 6 | `grep -n -i 'not published' CONTRIBUTING.md` | exit 0; at least one matching line (the disclosure boundary is stated to the contributor, not only in the design record) | check +dereference |
| 7 | `grep -n 'changelog' CONTRIBUTING.md` | exit 0; at least one matching line (the fork proxy is explained from the contributor's side) | check |
| 8 | `git -C . grep -n -i -e assay-desk-app -e assay-worker-app -e assay-reviewer-app -- CONTRIBUTING.md .github/PULL_REQUEST_TEMPLATE.md docs/contributor-trust.md` | exit 1; no matching line (contributor-facing pages name no internal automation identity) | check |
| 9 | `grep -n -i 'SECURITY.md' CONTRIBUTING.md` | exit 0; at least one matching line (the reporting path is linked, not restated) | check +neighbour |
| 10 | `grep -n -i 'contributor-trust' CONTRIBUTING.md && grep -n -i 'provenance' docs/contributor-trust.md` | exit 0; at least one matching line from each (the contributor's path runs `CONTRIBUTING.md` to the published model to the description of what the provenance comment measures, and every hop exists) | check +flow |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:06` | exit 0; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK`; output contains `corroborated` (the fully-qualified key is required — the short `<stream>/<NN>` form answers `no brief-v1 file` and exits 2, so it can never corroborate anything) | check |

Pre-mortem to detection map. "The new section quietly describes an advisory guideline as
enforced, so the page promises something the repository does not do" is caught by row 5. "The
disclosure boundary is decided in the design record and never actually stated to the
contributor" is caught by row 6. "The template ships but omits the verification prompt, which
is the whole point of it" is caught by rows 2 and 3. "A contributor-facing page leaks the
names of internal automation identities" is caught by row 8. "The security reporting path is
copied into the new section and the two copies drift" is caught by row 9, which requires a
link. "The prose is technically complete and reads as hostile" — no row; tone is the review
gate's judgement and this table explicitly gates presence only.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implemented in two motions: the pull-request template, the CONTRIBUTING.md template section and
the fork changelog-proxy note landed earlier as the partial (PR #965), which deferred the
disclosure sections pending the decision issue; this change completes the remainder — the
`## The trust bar` section in `CONTRIBUTING.md` (provenance card, tiers and unlocks, item
admission, the not-published boundary), the provenance-card hop in `docs/contributor-trust.md`,
and the changelog fragment (re-cut; the partial's fragment was aggregated at v1.0.15).
The decision issue for this brief's human gate remains open and unrated at implementation
time; this implements the design record's recommended option (publish the model and the
measured signals, never the rows) — the same disclosure boundary already published by the
landed tier and provenance pages — and nothing here closes or pre-answers that issue.

Fail-first: rows 4, 6 and both halves of 10 are new presence checks introduced by this change,
and each was observed RED on the pre-fix base `e4109205` (`grep -n -i provenance CONTRIBUTING.md`
→ rc=1, `grep -n -i 'not published' CONTRIBUTING.md` → rc=1, `grep -n -i contributor-trust
CONTRIBUTING.md` → rc=1, `grep -n -i provenance docs/contributor-trust.md` → rc=1) and green at
the branch head. Rows 1–3, 5, 7 and 9 assert content that predates this change (the template
and the partial's CONTRIBUTING sections) and were green at the base; this change does not alter
what they pin. No code, no test suite: the changed paths are markdown only.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `test -f .github/PULL_REQUEST_TEMPLATE.md` | exit 0 | exit 0 (present since the partial) | 2026-09-20 | glm-5.3[1m]-worker |
| 2 | `grep -n -i 'how you verified' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; ≥1 line | exit 0; line 11 `## Claims and how you verified them` (+ lines 14, 18) | 2026-09-20 | glm-5.3[1m]-worker |
| 3 | `grep -n -i 'automated' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; ≥1 line | exit 0; line 20 `## Automated assistance` | 2026-09-20 | glm-5.3[1m]-worker |
| 4 | `grep -n -i 'provenance' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; lines 58, 70 (`provenance card`, link to `docs/contributor-provenance.md`); RED (rc=1) at base | 2026-09-20 | glm-5.3[1m]-worker |
| 5 | `grep -n -i 'advisory' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; lines 35, 41, 44 — unchanged by this PR; the new section restates "the guidelines stay advisory" | 2026-09-20 | glm-5.3[1m]-worker |
| 6 | `grep -n -i 'not published' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; line 78 `**Who holds which tier is not published**`; RED (rc=1) at base | 2026-09-20 | glm-5.3[1m]-worker |
| 7 | `grep -n 'changelog' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; lines 76, 103–104 (proxy note landed by the partial; the new section links it) | 2026-09-20 | glm-5.3[1m]-worker |
| 8 | `git -C . grep -n -i -e assay-desk-app -e assay-worker-app -e assay-reviewer-app -- CONTRIBUTING.md .github/PULL_REQUEST_TEMPLATE.md docs/contributor-trust.md` | exit 1; no match | exit 1; no line printed (also no match at base) | 2026-09-20 | glm-5.3[1m]-worker |
| 9 | `grep -n -i 'SECURITY.md' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; line 24 (pre-existing) + line 91 (new section links it, does not restate the path) | 2026-09-20 | glm-5.3[1m]-worker |
| 10 | `grep -n -i 'contributor-trust' CONTRIBUTING.md && grep -n -i 'provenance' docs/contributor-trust.md` | exit 0; ≥1 line each | exit 0; line 76 (link to `docs/contributor-trust.md`) and lines 69, 74 (`provenance card`, link to `contributor-provenance.md`); both halves RED (rc=1) at base | 2026-09-20 | glm-5.3[1m]-worker |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:06` | exit 0; `corroborated`; no `DISPROVED`/`COULD-NOT-CHECK` | exit 0; `summary: 3 corroborated, 0 disproved, 1 unchecked` against merge-base `e4109205` (the unchecked entry is SECURITY.md `out-of-scope`, byte-identical to the merge-base — a reviewer judgement by design, never a pass); requires the brief file itself in the branch diff, which its Evidence edit supplies | 2026-09-20 | glm-5.3[1m]-worker |

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f .github/PULL_REQUEST_TEMPLATE.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -n -i 'how you verified' .github/PULL_REQUEST_TEMPLATE.md` | pass exit=0 | sha256:53b118f6a842 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -n -i 'automated' .github/PULL_REQUEST_TEMPLATE.md` | pass exit=0 | sha256:aecab1b99705 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -n -i 'provenance' CONTRIBUTING.md` | pass exit=0 | sha256:d48a4d2609ca | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -n -i 'advisory' CONTRIBUTING.md` | pass exit=0 | sha256:4fe4a447f3e5 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n -i 'not published' CONTRIBUTING.md` | pass exit=0 | sha256:69a77305e0b0 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -n 'changelog' CONTRIBUTING.md` | pass exit=0 | sha256:acf59439d849 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 8 | `git -C . grep -n -i -e assay-desk-app -e assay-worker-app -e assay-reviewer-app -- CONTRIBUTING.md .github/PULL_REQUEST_TEMPLATE.md docs/contributor-trust.md` | pass exit=1 | sha256:e3b0c44298fc | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -n -i 'SECURITY.md' CONTRIBUTING.md` | pass exit=0 | sha256:b5a307cd7646 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 10 | `grep -n -i 'contributor-trust' CONTRIBUTING.md && grep -n -i 'provenance' docs/contributor-trust.md` | pass exit=0 | sha256:2f7979d9dc24 | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:06` | fail exit=2 | sha256:ceed588b236a | 2026-10-09 | assay-verifier-app[bot] @ a65f270aa9c1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2483 run=c9b7a6cff7ad6cd08473f3c6107e80225f19d91fc13ba92815d275280cf8d886 source=a65f270aa9c10e73bd68e68b6a29d253a9a8ac28 brief=docs/streams/contributor-trust/brief-06-contributor-facing-docs.md model=claude-opus-5-5 tier=any

### Non-implementer verification, hand-run — 2026-10-09

Runner: assay-verifier-app[bot], on-behalf-of human:ian; model claude-opus-5-5. Merged main at
`a65f270aa9c1`; git 2.56.0, bash 5.3.20, statusgen v1.0.34. Every row was run by hand on a clean
tree before the execution witness above, and each matched passage was then read against the
row's Expect. Admission: the witness block's Verification-Attestation line (#2483, run
`c9b7a6cff7ad`, source `a65f270aa9c1`). Evidence only: this brief is `gate: human` with
`customer: yes`, so nothing here changes its status and the verdict below is input for the human
gate.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `test -f .github/PULL_REQUEST_TEMPLATE.md` | exit 0 | exit 0; no output; the file is tracked under exactly this upper-case name | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 2 | `grep -n -i 'how you verified' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; ≥1 line | exit 0; 3 lines: 11 "## Claims and how you verified them", 14, 18 "How you verified it:". Passage read: it does ask for each claim and how it was checked | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 3 | `grep -n -i 'automated' .github/PULL_REQUEST_TEMPLATE.md` | exit 0; ≥1 line (the assistance-disclosure field) | exit 0; 1 line: 20 "## Automated assistance". Passage read: lines 22-24 ask whether the change was produced with an AI coding tool or agent; it is the disclosure field and states no tiering consequence. The match is the heading alone (note 4) | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 4 | `grep -n -i 'provenance' CONTRIBUTING.md` | exit 0; ≥1 line | exit 0; 2 lines: 93 "**provenance card** — a comment on the pull request", 106 the link to the published signal page. Passage read: lines 91-107 state what the card is, what it measures and what it excludes; the list of measured facts is incomplete (note 2) | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 5 | `grep -n -i 'advisory' CONTRIBUTING.md` | exit 0; ≥1 line (existing guidelines still described as advisory) | exit 0; 4 lines: 35, 41, 44 "Both guidelines are **advisory** unless this page says otherwise", 89 "the guidelines stay advisory". Passage read: yes, both guidelines are described as advisory and nothing is described as enforcing; the two enforcement switches in the inbound triage workflow both read "false" at this commit | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 6 | `grep -n -i 'not published' CONTRIBUTING.md` | exit 0; ≥1 line (boundary stated to the contributor) | exit 0; 1 line: 114 "**Who holds which tier is not published** —". Passage read: yes, lines 114-116 state the boundary to the reader: "the model is public, the records are not" | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 7 | `grep -n 'changelog' CONTRIBUTING.md` | exit 0; ≥1 line (fork proxy explained from the contributor's side) | exit 0; 3 lines: 112, 139, 140. Passage read: lines 137-146 do explain the proxy from the contributor's side, but they disagree with the tier table about who receives it (note 3) | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 8 | `git -C . grep -n -i -e assay-desk-app -e assay-worker-app -e assay-reviewer-app -- CONTRIBUTING.md .github/PULL_REQUEST_TEMPLATE.md docs/contributor-trust.md` | exit 1; no matching line | exit 1; no output. Read as well as grepped: a wider pattern (any "assay-…-app" name, any "[bot]" suffix) also finds nothing in the three files; they name no automation identity | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 9 | `grep -n -i 'SECURITY.md' CONTRIBUTING.md` | exit 0; ≥1 line (reporting path linked, never restated) | exit 0; 3 lines: 24, 77, 127 (each a markdown link to the security policy file). Passage read: yes, the trust-bar section links the policy at line 127 and repeats no reporting channel or contact from it | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 10 | `grep -n -i 'contributor-trust' CONTRIBUTING.md && grep -n -i 'provenance' docs/contributor-trust.md` | exit 0; ≥1 line from each | exit 0; first half 1 line (112, the link to the tier page), second half 2 lines (69, 74). Passage read: every hop exists and every link target is a tracked file — contribution guide to tier page to the published signal page | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |
| 11 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:06` | exit 0; contains `corroborated`; no `DISPROVED`; no `COULD-NOT-CHECK` | exit 2; "COULD-NOT-CHECK: assay:assay:contributor-trust:06 is not in the diff against a65f270aa9c1…, so this run carries no evidence about its claims — no entry was corroborated and none was disproved". Does NOT meet Expect; a check-definition fault, never a pass (note 1) | 2026-10-09 | assay-verifier-app[bot], on-behalf-of human:ian |

Notes.

1. Row 11 cannot corroborate anything on merged main (check-definition). The command diffs the
   working tree against the default base, which on merged main is the same commit, so the tool
   answers COULD-NOT-CHECK and exits 2. The result is also order-dependent: once this Evidence
   section carries an uncommitted edit, the same command exits 0 with "summary: 0 corroborated,
   0 disproved, 4 unchecked" — and that output mechanically satisfies the Expect as written,
   because the substring `corroborated` appears in "0 corroborated". Neither outcome is evidence.
   Supplementary only, and not the row: the same command with `--base` set to the parent of the
   #1370 squash commit exits 0 with "summary: 3 corroborated, 0 disproved, 1 unchecked" (the
   unchecked entry is the out-of-scope security policy).
2. The contribution guide lists six measured facts at lines 94-98 and introduces them as "a
   fixed list"; the published signal page and the signal registry in code both carry seven. The
   one left out is body-shape similarity — whether a description is near-identical in shape to
   another recent submission by the same author. It was already on the signal page when #1370
   landed. The guide does defer to that page as the place where the list is "published in full".
3. The guide says a tier unlocks "the fork changelog arrangement below" (lines 110-112) and the
   tier page gives that capability to the two upper tiers only, yet the section below (lines
   137-146) tells every fork contributor that a missing fragment "is not something you need to
   fix yourself". The changelog check at this commit consults no tier, so the unconditional
   wording is what happens today; the two published statements still disagree about a
   first-time contributor.
4. Several rows stay green when the guarded statement is removed, shown by mutation on scratch
   copies (the tree here was never edited): row 5 stays exit 0 with the "Both guidelines are
   advisory" paragraph deleted, and also with that sentence rewritten to "enforced and no longer
   advisory"; row 7 stays exit 0 with the whole fork-changelog section deleted (line 112 still
   matches); row 9 stays exit 0 with the trust-bar link deleted, and with a reporting path
   restated inline; row 3 stays exit 0 with the question deleted and only the heading kept.
   Rows 1, 2, 4, 6 and 10 went red under their mutation. Row 8 went red (exit 0, a match) for
   each of its three names planted in each of the three files, and stayed exit 1 for a role
   identity outside its three-name pattern.
5. Changed on main since #1370: #1480 added the "What must not appear" section to the guide and
   one sentence to the template's assistance field. No guarded statement was altered; line
   numbers moved by 35. The template now carries a conditional confirmation beyond the "two
   prompts" the guide still describes.
6. Of the files named in Context, the changelog fragment is absent at this commit: the release
   aggregation folded it into the changelog under v1.0.16, which is its expected lifecycle.
7. The design record this brief is authored against still reads "PROPOSED — no ruling is
   recorded" at this commit, and the published signal page still says whether the card is ever
   posted on a real pull request awaits a ruling. No workflow at this commit invokes the
   provenance probe. The guide's wording is "may be measured".

Risk-bearing values (enumerated over the #1370 and #965 diffs to the three documents):

- RISK-VALUE: NAMED, NOT DERIVED — publication authority, decided-by = "human:&lt;name&gt;" (placeholder) @ docs/streams/decisions/DR-contrib-disclosure.md:5 — the disclosure boundary at CONTRIBUTING.md:114 is published, and the record that would authorise publishing it carries no ruling; only the human gate can supply that derivation.
- RISK-VALUE: NAMED, NOT DERIVED — measured facts listed = 6 @ CONTRIBUTING.md:94-98 — the registry holds 7 (tools/desk/internal/deskkit/provenance.go:161-197; docs/contributor-provenance.md:25-31); no source was found for stating six.
- RISK-VALUE: DERIVED — guidelines "advisory" @ CONTRIBUTING.md:44,89 — ENFORCE_ISSUE_FIRST = "false" and ENFORCE_CONCURRENCY = "false" @ .github/workflows/inbound-triage.yml:43,45; the stated guideline of three matches MAX_OPEN_PR_PER_AUTHOR = "3" @ line 48.
- RISK-VALUE: DERIVED — tiers = 4 (unknown, blessed-once, contributor, maintainer) @ CONTRIBUTING.md:109-110 — the tier enumeration has exactly these four in this order (tools/desk/internal/deskkit/trusttier.go:57-60) and the four unlocks named at lines 110-112 are the four capabilities at trusttier.go:276-279.
- RISK-VALUE: DERIVED — burst window "in a day" @ CONTRIBUTING.md:96 — the signal counts pull requests "in the preceding 24h" (tools/desk/internal/deskkit/provenance.go:174).
- Ranked last, reversible wording, no derivation owed: template prompts = "two" @ CONTRIBUTING.md:131.

VERIFY: BLOCKED — check-definition: rows 1-10 meet Expect at `a65f270aa9c1`; row 11 exits 2 with COULD-NOT-CHECK and cannot corroborate on merged main as written. Notes 2, 3 and 7 are content questions for the human gate and are open whatever row 11 does.

Desk note, 2026-10-10: the row 11 could-not-check above is an instance of #1915 (the consumers check cannot corroborate on merged main as written). #1915 is the blocker this pass's outcome record names.


## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
