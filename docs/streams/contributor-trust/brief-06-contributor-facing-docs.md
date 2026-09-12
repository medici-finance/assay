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
  - "CONTRIBUTING.md: follow-up contributor-trust/06 (this brief; the trust-bar section)"
  - ".github/PULL_REQUEST_TEMPLATE.md: follow-up contributor-trust/06 (this brief; the claims checklist and the how-I-verified prompt)"
  - "docs/contributor-trust.md: follow-up contributor-trust/06 (this brief; the published model gains its contributor-facing entry point)"
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
| 11 | `statusgen --root . --consumers --brief contributor-trust/06` | exit 0; output does not contain `DISPROVED` | check |

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

## Review
Gate: human (from frontmatter). Reviewer records verdict + date in the stream README table.
Human gate is MANDATORY when any risk answer is yes.
