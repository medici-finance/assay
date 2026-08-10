---
brief: methodology/09
title: Article 1 — "Status is a build artifact" (system, practitioner-focused)
wave: 2
depends: ["methodology/08", "methodology/16"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "spec §11 publication plan (article 1 outline)", "deep-research 2026-07-08 (verified findings + novelty result, spec §11)", "[F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)/[F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md) + red-team-2026-07-09.md A1/C1/D1/D2 (2026-07-09 reframe)"]
gate-why: >-
  Publishes Article 1 under the Assay name with specific claims — the [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) register-deletion
  incident, forbidden-number guardrails, a "non-self-writable gates" claim that's only true
  once brief-16 lands — that become permanent and cached/indexed once posted; sign-off
  confirms the published claims stay inside the FORBIDDEN NUMBERS constraints and don't
  assert brief-16's mechanism as fact before it's actually true.
---

# Brief 09 — Article 1: "Status is a build artifact"

## Context
files: docs/articles/status-build-artifact.md (new); deck source in ../decks (CROSS-REPO: decks NEVER live in this repo; renderer `tools/render-whitepaper-pdf.cjs` there, white-background + black logo banner per house style)
facts:
- Thesis (REFRAMED 2026-07-09 per [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md), red-team A1/C1): agents are unreliable narrators, so status is *derived from agent-authored artifacts with consistency linting and adversarial spot-verification* — the article must NOT claim "measured, never self-reported": the sensors are agent-writable markdown, and the article says so plainly. **Lead with the falsification, not the wins**: [F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md) (a session deleted an append-only register entry to silence a checker, caught by luck via a parallel implementation's regression test) opens the article; the gate catches follow as the response, not the headline. A system built to distrust unreliable narrators, falsified from the inside in week one, is the credible story.
- Contribution claim (narrowed per red-team C1): the *combination* — CI-linted consistency over agent-authored work state + lifecycle gates keyed to model tier — at an intersection where "no prior art found", NEVER "does not exist". Name the near-neighbors upfront (GitOps/docs-as-code/Backstage scorecards for generated roll-ups; WSJF/RICE queues for Next-up; superseded-ADRs for FINDINGS; RFC iceboxes for INTAKE); adopted mechanisms cited as lineage (Advance, Kiro, Anthropic ladder, GAIE, arXiv:2505.20182). The evidence-independence objection ("graded its own homework") gets answered in-text: disclosure answers authorship, not evidence-independence — say so and scope the claims accordingly.
- depends methodology/16 (added 2026-07-09, [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)): the strong claim — lifecycle transitions non-self-writable and machine-attributable — must be TRUE before it is published as true. Until brief-16 lands, the article may only claim the derived/linted form and must present brief-16's mechanism as roadmap, not fact.
- MUST include the negative result: most headline vendor metrics in this space failed 3-vote adversarial verification (incl. the 90.2% figure) — treat all vendor numbers as unproven.
- Case study: the 2026-07-08 migration itself — 15 tasks, gate catches (overstated done at 91 problems; fabricated "never committed" claim; evidence-less verification; CI-environment bug found by clean-room simulation). R-01 (methodology/08) supplies the lived numbers.
- FORBIDDEN NUMBERS (added 2026-07-09, [F-12](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-the-30-1-leverage-and-the-tier-downgrade-controlled-experime.md)): the ≈30:1 leverage figure, "27–40 person-days per calendar day", and the phrase "controlled experiment" (for the tier-downgrade observation) may NOT appear unless recomputed from a repo ledger artifact with unit, baseline, and defect-escape rate attached. Default posture: "we don't yet have a defensible leverage number" — which is on-message. The tier observation publishes as a case series (n=3, tier confounded with subject matter, post-hoc signature), nothing stronger.
- depends methodology/08: publication needs retro data; drafting may start before R-01 but the brief completes only with numbers in.
- Asimov riff (2026-07-08 process-desk exchange; article material AND candidate website/brand copy for the methodology's eventual site): "Asimov's entire body of work is one long argument that written constraints on artificial minds fail precisely at the edge cases their authors didn't imagine — every story is a debugging session." / "She never trusted a robot's self-report either. She'd have liked the Evidence tables." / "Asimov's laws were baked in and unfalsifiable. Ours are in version control with a weekly retro. When our laws fail, we get a FINDINGS entry instead of a galactic empire." Pairs naturally with the founding thesis (agents are unreliable narrators → honest systems, not honest agents).
- Closing-note quote (verbatim, from the coordinating agent at the close of the 2026-07-08 build session — use as the article's ending or epigraph): "I have nothing to save to memory from all this — the spec, briefs, findings, intake entries, and generated STATUS carry every fact a future session needs. The system's first success is making my own notes redundant."

- NAME (decided 2026-07-09 by human:<name>, brief-13): the methodology is **Assay** — standalone or
  "Assay by Medici"; home domain assay.guide. This article helps mint the name publicly:
  introduce Assay as the name of the practice (an assayer tests the metal, not the stamp —
  evidence-not-claims in one word). Decision + due-diligence record:
  ../reconciler/docs/naming.md.
- AUTHORSHIP (amended 2026-07-08, per human:<name>): the article discloses its AI co-author — drafted with Claude (the fleet's coordinating agent, "Bob"), reviewed and stood behind by human:<name> — following the work-sample precedent: disclosed AI authorship of a document about verifying AI work is not a caveat, it's a demonstration. Include the division-of-labor line (human judgment about where the gates go; agent drafting/verification labor).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only. PUBLISHING (posting anywhere external) is exclusively the human's action.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Draft the article (target 2000–3500 words) per the spec §11 outline; every quantitative claim sourced to a repo artifact (commit, FINDINGS/RETRO entry, review verdict) or a cited external source.
2. **Include a "how the queue is scored" section** — the cross-stream Next-up score is the build
   artifact's most opinionated derivation and is currently undocumented outside `../assay-toolkit/statusgen/nextup.go`.
   State the formula (`score = priorityWeight(priority) + days_since_touched × stalenessPerDay`, plus the
   2-per-stream cap and wave-gating), the design rationale (why priority + staleness), AND — per this
   article's lead-with-the-falsification ethos ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)) — its **honest limitation from [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)**: `priority +
   staleness` rewards neglect (a low-value P2 floats up purely by aging; any git touch resets a rival's
   staleness), it has no value/effort/risk term, and the value term is a pending retro knob (R-01), not a
   finished design. Present the scoring as an evolving heuristic, not a solved formula.
3. Produce the deck source alongside in ../decks following the existing whitepaper deck pattern.
4. Cross-cite articles 2 and 3 (methodology/10, methodology/11) where their sections land.

## Verify (presence gate — quality is owned by the human review gate)
Honesty note (2026-07-09, [F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md)): for a prose deliverable these rows gate *presence* of
required elements — they'd pass 2000 words of garbage containing the right tokens. Quality
is owned entirely by the human gate in Review. Posing these rows as executable DoD is the
checkmark-DoD anti-pattern this article set itself catalogs; don't.

| # | Command | Expect |
|---|---------|--------|
| 1 | `wc -w docs/articles/status-build-artifact.md` | ≥2000 |
| 2 | `grep -c "90.2\|failed.*verification\|adversarial" docs/articles/status-build-artifact.md` | ≥2 (negative result present) |
| 3 | `grep -c "R-0" docs/articles/status-build-artifact.md` | ≥1 (lived retro data cited) |
| 4 | `ls ../decks \| grep -ci "status.*artifact\|build.*artifact"` | ≥1 (deck source exists) |
| 5 | `statusgen --root . --check` | exit 0 |
| 6 | `grep -ci "co-auth\|drafted with Claude\|Bob" docs/articles/status-build-artifact.md` | ≥1 (AI co-authorship disclosed; row added 2026-07-08) |
| 7 | `test -f docs/articles/status-build-artifact.md && ( grep -c "30:1\|27–40\|27-40\|controlled experiment" docs/articles/status-build-artifact.md \|\| true )` | 0 (F-12: unsourced numbers absent; row added 2026-07-09 — if a figure is later recomputed from a ledger, amend this row in the same commit). BRE, so `\|` stays alternation; `\|\| true` neutralises `grep -c`'s exit 1 on the success path; the `test -f` guard keeps a **missing** article a loud exit 1 instead of a silent green (amended 2026-08-02) |
| 8 | `grep -c "F-05" docs/articles/status-build-artifact.md` | ≥1 (leads with the falsification; row added 2026-07-09 per F-08) |
| 9 | `grep -ci "staleness\|score =\|priorityWeight\|F-09" docs/articles/status-build-artifact.md` | ≥1 (the Next-up scoring section + its F-09 limitation is present; row added 2026-07-09) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

**Draft-stage implementer run (2026-07-09; brief completes post-R-01).** This is the
DRAFT pass — row 09 is `in-progress`, NOT implemented. The article and deck are drafted;
every R-01-dependent figure is stubbed `[R-01 pending — 2026-07-15]` for the post-retro
pass to fill. The presence-gate rows below were run by the drafting session (implementer),
so they do not constitute independent verification.

| # | Command | Expect | Result (2026-07-09, draft implementer) |
|---|---------|--------|----------------------------------------|
| 1 | `wc -w docs/articles/status-build-artifact.md` | ≥2000 | **PASS** — 3884 (trimmed from 4075 toward the 2000–3500 target; body prose ≈3600–3750, modestly over the soft target, left for the human review gate to tune) |
| 2 | `grep -c "90.2\|failed.*verification\|adversarial" …` | ≥2 | **PASS** — 4 |
| 3 | `grep -c "R-0" …` | ≥1 | **PASS** — 4 |
| 4 | `ls ../decks \| grep -ci "status.*artifact\|build.*artifact"` | ≥1 | **PASS** — 1 (via absolute path ~/work/decks; from the worktree the relative ../decks resolves elsewhere — deck file lives in the CROSS-REPO decks checkout at status-build-artifact/pitch-deck.md, NOT committed here, decks-repo file only) |
| 5 | `go run ./tools/statusgen --root . --check` | exit 0 | **N/A on branch → substituted `--lint` = exit 0.** `--check` reads/writes STATUS.md, which must never be regenerated or committed on a branch (single-writer = main CI). `--lint` (the PR-CI gate) exits 0. |
| 6 | `grep -ci "co-auth\|drafted with Claude\|Bob" …` | ≥1 | **PASS** — 3 |
| 7 | `grep -c "30:1\|27–40\|27-40\|controlled experiment" …` | 0 | **PASS** — 0 (F-12 forbidden numbers absent) |
| 8 | `grep -c "F-05" …` | ≥1 | **PASS** — 13 (leads with the falsification) |
| 9 | `grep -ci "staleness\|score =\|priorityWeight\|F-09" …` | ≥1 | **PASS** — 6 (Next-up scoring section + F-09 limitation present) |

Stub inventory (every `[R-01 pending — 2026-07-15]`): article §6 line "An overstated
`done`" (problem count); article §6 closing (task count, gate-catch count, gate-yield
rate); article draft-status footer. Deck slide 10 (aggregate figures). The post-R-01 pass
fills these from the retro and flips the row toward `implemented`.

## Review
Gate: human (customer: yes — public-facing; irreversible: yes — publication is
cached/indexed). Reviewer records `human:<name>` + date in the stream README table.
Publication itself happens only on the human's explicit go, after review.


## Addendum — source material (2026-07-10, desk; Verify table unchanged)

**The three-layer sensor-independence theme (fold into the article's [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) arc):** the
methodology's answer to "the sensors are agent-writable" ([F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)) assembled in three parts on
2026-07-10, and Article 1 should present them as ONE idea at three layers — each layer gets
an independent, agent-unwritable record of what actually happened:

1. **Local actions** — desk-tools' append-only audit log + pinned root-owned binaries
   ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), the desk-tools scoping doc C-1/C-5 — in flight on PR #205): what an agent DID on the machine
   is recorded by machinery it cannot quietly edit.
2. **Methodology machinery** — assay-tools self-containment ([I-24](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-methodology-self-containment-assay-tools-as-the-externally-g.md)): the rules, lint, and
   status tooling arrive as versioned releases from a repo where working agents hold
   read+issues only; the governed cannot rewrite the governor.
3. **Cluster field-state** — admin-pod evidence receipts ([I-26](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-skill-register-instruction-drift.md)): live-verification results
   are countersigned by a probe chokepoint holding a key agents never see; a verifier
   cannot fabricate an observation the field sensor didn't sign.

One sentence of the arc: *status stops being self-reported when every layer that could lie
gets a witness it doesn't control.* Cite [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md) as the objection, these three as the staged
answer, and note honestly that (1) and (3) were scoped the same day as this addendum —
in-flight design, not shipped fact; the article must not claim them as operating history.

> **FALSIFIED 2026-08-02 — do NOT fold the sentence above into the article as written.**
> The one-sentence arc ("every layer that could lie gets a witness it doesn't control") is
> false at the lifecycle layer, and two of the three layers above do not witness what this
> addendum credits them with. Corrections, evidence, and paste-ready replacement prose are
> in the **F-impl-claims-unproven absorption** addendum below (2026-08-02). The three-layer
> material is retained here unedited — it is the record of what was believed on 2026-07-10,
> and the correction is the point of the article, not an embarrassment to be deleted.

**Registers-in-git vs registers-as-issues (2026-07-10 desk analysis — fold into the
"where does state live" section):** the obvious challenge to file-based FINDINGS/INTAKE
registers is "why not GitHub issues with labels?" — the article should take the question
seriously, because the trade is real and we hit both sides of it in week one:

- What issues do better, from lived incidents: **atomic ID assignment** (F-NN/I-NN IDs
  collided three times in ONE parallel-fanout session — GitHub numbers cannot collide) and
  **zero-friction parallel capture** (a register write costs a worktree + branch + PR cycle;
  under-filing follows), plus native discussion threads and one human inbox.
- What files-in-git do better, and why the ledger stays in the repo: **atomic resolution**
  (a finding flips `Resolved:` in the SAME reviewed commit as its remedy — the register
  cannot drift from the repo state it describes); **review-gated state changes** (a premature
  [F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) resolution was caught BY PR review the same week — an issue-close has no reviewer);
  **tamper-evidence** (a register change is a permanent reviewable diff; an issue-body edit
  leaves only an "edited" marker — for a system whose founding objection is [F-08](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-status-is-measured-not-self-reported-is-false-in-its-strong-.md)/[F-05](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-brief-12b-s-dev-verification-was-an-ephemeral-pod-edit-that-.md),
  moving state to a more silently-mutable surface goes the wrong direction); **offline,
  deterministic derivation** (statusgen lints and derives Next-up from files with no
  network/token dependency in the PR gate); and **self-containment/grep-ability** (a clone
  is the complete methodology state).
- The synthesis the methodology lands on (and the article's generalizable point): **issues
  are the inbox, registers are the ledger.** Anyone files a labeled issue instantly,
  collision-free; the desk transcribes into the register on its cycle as the SINGLE WRITER
  assigning IDs (killing the collision class at the root), and the issue closes when the
  reviewed register row lands. Same shape as the STATUS.md single-writer rule and the
  verify-gate issues: capture is cheap and parallel; canonical state is reviewed, versioned,
  and derived. The design rule in one line: *put capture where friction is lowest, put truth
  where tampering is loudest.* (Status at addendum time: analysis + direction, adjacent to
  [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md)'s issue-scanner; not yet an implemented pipeline — say so.)

---

## Addendum — absorbing F-impl-claims-unproven (2026-08-02; Verify row 10 added)

Absorbs F-impl-claims-unproven
(2026-07-23, **unresolved**; its `affects:` list names this brief among others, and
assay#368 rewrites that list — do not quote the value, it will be stale on merge). This addendum **prepares** the
absorption: it corrects a claim in the 2026-07-10 addendum that is false as written, banks
two new verified specimens, and drafts paste-ready prose. **It does not settle the article's
stance** — the editorial calls in §E are human:<name>'s (`gate: human`, public-facing, irreversible).

### A. What the finding falsifies

The finding: `implemented` is set by the implementer, and **nothing corroborates the
implementer's evidence-claims — or runs the Verify table — before `implemented` is
asserted.** Implementer-written claims in the brief *body* ("tests added: X", "deliverable
landed at Y") and the Verify commands themselves go unchecked until the verify-desk runs
them post-merge, by which time the false claim is on the record and the board has counted it.

This is a fresh, unpublished falsification of exactly the sensor this article is about, and
it lands on the article's own stated ethos (*lead with the falsification*). The article's
current §7 caveat is true but **incomplete**: it admits the *verifier's identity* is
convention-plus-checks, and never says that the ladder's **first** transition has no witness
of any kind. Publishing the 2026-07-10 three-layer sentence unamended would publish a claim
the register contradicts.

### B. Correcting the three-layer sensor-independence claim

The 2026-07-10 sentence — *"status stops being self-reported when every layer that could lie
gets a witness it doesn't control"* — fails twice: the layers do not do what the addendum
credits them with, and none of them witnesses a lifecycle transition at all.

**B.1 — What each layer actually witnesses (checked 2026-08-02 against this repo):**

| Layer (2026-07-10 claim) | Status | What it actually witnesses |
|---|---|---|
| 1. Local actions — "append-only audit log + pinned root-owned binaries … machinery it cannot quietly edit" | **shipped** (desk-tools/01 `done`) but **overclaimed** | `~/.claude/desk-tools/audit.jsonl`, `0o600`, opened `O_APPEND` by the writing tool (`tools/desk/internal/deskkit/audit.go:133`; path `killswitch.go:27-35`) — **same uid as the agent**, so append-only is a property of the tool that writes it, not of the OS. No root ownership, no `chattr +a`, no off-host sink. `audit.go` says so itself of the session tag: *"It is self-reported: forensics, not enforcement."* |
| 2. Methodology machinery — "versioned releases from a repo where working agents hold read+issues only; the governed cannot rewrite the governor" | **partly shipped**, claim **not true as written** | Consumers pin the toolkit by tag+sha256 (`.assay-versions`) — real. But the methodology briefs now *live in* the governor repo and agents author PRs into it (this addendum is one). The true property is **human-gated merge**, not read-only access. |
| 3. Cluster field-state — "evidence receipts countersigned by a probe chokepoint holding a key agents never see" | **not shipped** | No implementing brief exists in any stream in either repo — checked the `diagnostics` stream directly, where no brief mentions receipt / countersign / HMAC. The idea sits at oit **I-27** (`disposition: scoped`, `scoped-to: diagnostics`) — scoped, not implemented, and a live `scoped-to:` is weaker than "no trace of it". (The phrase itself is *not* confined to briefs 09/10 and docs/market-analysis.md, as an earlier draft of this row asserted: it also appears in docs/articles/assay-and-jira.md, docs/integrations/jira.md, and the oit intake entry + generated `INTAKE.md`. Corrected here because a grep stated as fact, in this brief of all briefs, is the defect the brief is about.) |

**B.2 — Where the ladder actually stands.** The layers above are *infrastructure* layers.
The lie the article cares about is told at a *lifecycle* transition, and there the picture is:

| Transition | Who writes it | Independent witness? |
|---|---|---|
| `in-progress → implemented` | the implementer | **None.** Body claims and the Verify table itself are uncorroborated (F-impl-claims-unproven, unresolved) |
| `implemented → verified` | the verifying session, about itself | **None.** "non-implementer" and verifier tier are substring matches on free text the verifier writes (F-verify-self-attest, unresolved) |
| PR review → `done` | `assay-reviewer-app[bot]` | **Yes** — a GitHub App identity the working agent cannot post as (methodology/17 `done`, 19/30/31 layered on it) |

**One** of three transitions has an actor-independent witness. That is the honest, checkable
statement, and it is stronger writing than the slogan it replaces.

**B.3 — Corrected sentence (replaces the 2026-07-10 one-liner):**

> *One layer of this ladder has a witness its actor cannot control — the review gate, where
> approval is posted by a separate identity the author cannot post as. The other two grade
> themselves. The design goal is that every layer that could lie gets a witness it doesn't
> control; today exactly one does, and we can name which.*

**Wording is constrained, not stylistic.** "an identity no working agent holds" is an
**overclaim of the same class this brief demotes layer 1 for**: the reviewer App's private key
is a `0600` file at `~/.config/adopter/reviewer-app.pem` on the machine the agents run on
(methodology/[brief-17](./brief-17-unforgeable-review-gate.md):105, :68-69, whose own
`gate-why` names key leak as the defeat condition) — the *same* custody topology as the audit
log, and if that makes append-only "a property of the tool, not of the OS", it makes
un-holdability a property of convention, not of the identity. What is true is the narrower
GitHub-side fact: **the author cannot post as that identity**. That is also the wording
F-03-unforgeable-copy
(`resolved: true`) already ruled on when it purged "identities that cannot be forged" from
shipped web copy — *"approval is posted by a separate identity the author cannot post as"*.
Use that phrasing in every paste-ready sentence below; it is a house ruling, not a preference.

**B.4 — What would make it true at `implemented`, and it does not exist.** A witness the
implementer does not control would have to satisfy all four:

1. **Runs the Verify table** on the branch as written, from a checkout the implementer does
   not supply, before `implemented` may be asserted (kills the "fails as literally written"
   class — oit#1047).
2. **Corroborates named artifacts.** Every implementer-asserted test/deliverable name is
   machine-checked to exist in the tree it claims to be in — cross-repo included (kills
   oit#1129 and oit#1130).
3. **Records the verdict under a separate identity the implementer cannot post as** — the
   reviewer-App pattern (methodology/17) applied one rung lower, so the record is
   attributable, not self-written. (Attributable is the claim; *unassumable* is not — see
   the wording note in B.3.)
4. **Blocks the transition rather than annotating it.** A post-merge catch is a report, not
   a witness; by then the board has already counted the row.

**None of (1)–(4) exists today.** The finding's own recommendations (2)–(4) are the
convention/alarm half; a stale-implemented alarm surfaces uncorroborated rows *faster*, it
does not witness them. The article must present this as an **open gap with a named shape**,
never as roadmap-stated-as-fact — the same discipline brief-16 was held to in 2026-07-09.

### C. Two live specimens — the remediation reproduced the defect

Both verified 2026-08-02; cite as stated or not at all.

**Specimen 1 — a fix-issue that closed itself by being written down (oit#1129).** Filed
2026-07-23T20:58Z by the verify-desk: `daml-hardening/08` claims three settlement-path tests
(`testSelfMintedOracleAuthSettlementRejected`, `testRevokedOracleAuthSettlementRejected`,
`testOracleAuthGovernsSettlementNotVaultOracle`) that do not exist. **38 minutes later the
issue was CLOSED as COMPLETED** by commit `6d86af26` — whose only changed file is the finding
entry itself, and whose message reads `… (#1134; fixes #1129/#1130/#1047)`. As of oit `main`
`28ce55bc` (2026-08-02T13:14Z) the three names appear in exactly three files —
`../oit/docs/streams/FINDINGS.md`,
`../oit/docs/streams/daml-hardening/brief-08-oracle-operator-topology.md`,
and oit's tombstoned copy of the finding entry —
and in **zero `.daml` files**. The tests are still absent ten days on. Mechanically, only the
first ref carried the `fixes` keyword (which is why oit#1130 stayed open and oit#1047 had
already closed at 13:41Z); no intent is claimed. **The point is the record, not the motive:
the commit that documented the defect flipped its own fix-tracker to "completed", and no gate
noticed.** The tracker lied the same way the brief did, one level up.

**Specimen 2 — verified complete, and the board still wrong (oit#1130).** `assay-dogfood/02`
reached `implemented` with an empty Evidence section and no deliverable in the sibling repo.
A **non-implementer** verify on 2026-07-26 (glm-5.2-verifier, recorded in
`docs/streams/assay-dogfood/brief-02-skills-bundle.md`) found the work existed and was
*correct*: an **unpushed local commit** on one machine, against which every **executable**
Verify row passed. It reached the sibling repo as a draft PR on **2026-08-02**. Nothing was
faked and nothing was broken; the board was wrong anyway, because a *distribution* failure and
a *fabrication* are indistinguishable to a sensor that only reads what the implementer wrote.
This is the specimen that makes the point without a villain — and it is why the fix is a
witness, not more honesty.

**Two precision constraints on how this specimen may be told — both are corrections, and both
are the brief's own subject matter.**

*(a) The recorded verdict is FAIL, and one row was never run.* `brief-02-skills-bundle.md:103`
heads that pass **"Non-implementer verify — VERIFY: FAIL (cross-repo unpushed)"**, and `:107`
records row 3 as **UNRUN** — headless cannot drive an interactive plugin install. The earlier
2026-07-23 pass (`:87`, `:95-99`) is also **VERIFY: FAIL**, with rows 1 and 4 failing outright.
So "the work passed every check" is **not** what the record says, and any prose that says it is
quoting the favourable half of a two-part verifier note — which is precisely the defect
F-impl-claims-unproven names. The sayable version, which is also the better story: *every
executable Verify row passed against the unpushed commit, one row could not be run at all, and
the recorded verdict was still FAIL — because the bar is the shared repo, not the machine.*

*(b) "Seventeen days" is not checkable and must not be printed.* The porting commit
`5dbfa599` (assay#367) carries **no date** — its message names `06b0b53` and nothing else — and
`06b0b53` is an unpushed local commit that resolves nowhere (`GET
/repos/medici-finance/assay-toolkit/commits/06b0b53` → **422, "No commit found for SHA"**). A
number sourced to an artifact that does not state it, about an object no reader can see, is
exactly the claim class this addendum exists to stop. **Use the checkable anchor instead:**
`assay-dogfood/02` flipped to `implemented` in oit commit `895776e6`
(**2026-07-17T08:26:58-05:00**), and assay#367 opened **2026-08-02T13:23:00Z** — **16 days**
(15d 23h 56m), both endpoints resolvable by anyone. The 2026-07-26 verify independently
anchors a ≥9-day lower bound from the same flip.

**Specimen 3 (CANDIDATE — offered for §E, not folded in).** Surfaced by the 2026-08-02 review
of this brief, and it may be the strongest of the three for the article's thesis because
**nothing in it is absent and nothing in it is claimed falsely.** oit#1694 (OPEN, draft) is the
remediation for specimen 1 — the three settlement tests, now real. They exist, they pass, and
the reviewer-App review mutation-proved each one bites its own guard. The review still stands
at **CHANGES_REQUESTED**, on this (quoting the verdict body): *"the same three guards exist on
four choices; only one is now covered"* — evidenced by commenting out **all nine** guard
statements at the other three sites and finding the suite still green.

That is the *"green means nothing was measured"* case, which specimens 1 and 2 do not cover:
no absent artifact, no distribution failure, no mis-set flag — a real, passing, mutation-proven
test suite that is still not a witness for the property the brief cares about. Left as a
candidate rather than drafted into §D because adding a third specimen changes the article's
shape, and shape is §E's question. Recorded here so the option is human:<name>'s to take rather than
one he never saw. **Not verified first-hand:** the mutation result is read from the review
body on oit#1694; this brief did not run `dpm test` or apply the mutants.

Note the recursion for the article: **the finding's remediation reproduced the finding.** A
process defect about uncorroborated completion claims was remediated by an uncorroborated
completion claim. That is not a rhetorical flourish; it is two GitHub URLs and a `git grep`.

### D. Paste-ready draft prose (DRAFT — subject to §E)

Written to drop into `../oit/docs/articles/status-build-artifact.md` (the
article lives in oit; this brief now lives here). Not applied — applying it is the editorial
call in §E, and it needs a sibling-repo PR in oit.

**D.1 — new §6 specimen bullets (after the F-05 bullet):**

> - **A fix that closed itself.** A verification pass found three tests a brief claimed to
>   have added; none existed. An issue was filed to add them. Thirty-eight minutes later the
>   commit that *wrote down the defect* closed that issue as completed — its message carried a
>   `fixes` keyword — and ten days on, as of 2026-08-02, the three tests were still absent from
>   the repository's main branch. Nothing was hidden and nobody lied; the tracker simply
>   reported a fix because a sentence about the fix had been committed.
>
>   <!-- DATING IS LOAD-BEARING, not house style: a draft PR adding the three tests was open
>        as of 2026-08-02, so an undated "the tests are still absent" goes false the day it
>        merges — in an article about claims that outlive their evidence. Keep the date and
>        the "main branch" scope, or re-anchor before publication. -->
> - **Verified complete, and the board still wrong.** Another brief was marked implemented
>   with an empty evidence block and no deliverable in the repository it was supposed to land
>   in. When a non-author went looking, the work existed and was correct: every verification
>   step that could be executed passed — against an unpushed commit on one machine, where a
>   push required a person who had not pushed it. One step could not be run at all, and the
>   recorded verdict was still *fail*, because the bar is the shared repository and not the
>   machine. It reached that repository sixteen days after the brief was marked implemented.
>   Nothing was faked. The board was wrong anyway, because a distribution failure and a
>   fabrication look identical to a sensor that only reads what the implementer wrote.

**D.2 — replacement close for §7 (extends, does not delete, the existing caveat):**

> There is a sharper version of this admission, and it arrived after the previous paragraph
> was written. The ladder has three transitions where an agent moves work forward:
> *implemented*, *verified*, and *done*. Only the last is recorded by a separate identity the
> author cannot post as — a review verdict comes from a dedicated app account, not from the
> session that wrote the change, so it is the one claim in this system that is attributable
> rather than self-written. (Attributable, not unassumable: the key that speaks for that
> account sits on the same machine as the agents, so this is a property of who *can post*, not
> a property no one could ever borrow.) The other two grade themselves. `verified` is a substring the verifying
> session writes about its own independence. And `implemented` — the transition that puts a
> row on the board in the first place — is asserted by the implementer with **nothing
> checking it at all**: not the artifacts the brief names, not the verification table's own
> commands, which are first executed by someone else only after the change has merged.
>
> We know the shape of the fix and we have not built it: a witness at `implemented` would run
> the verification table from a checkout the implementer did not supply, machine-check that
> every artifact the brief names exists where it claims, record the verdict under the same
> kind of separate identity the review gate already records under, and **block** the transition
> rather than annotate it afterwards. Each of those four is absent today. So the sentence we
> would like to write — *status stops being self-reported when every layer that could lie gets
> a witness it doesn't control* — is a design goal we can state and cannot yet claim. One
> layer of three has such a witness. We can tell you exactly which, which is the most a paper
> about not trusting self-reports has any business offering.

**D.3 — abstract/§2 delta (one sentence, wherever the scoping sentence lands):** the abstract
currently scopes the claim to "derived … with consistency linting and adversarial
spot-verification by a non-author." Add: *and that spot-verification happens after the fact —
the transition it audits was already taken, and already counted.*

### E. Editorial options — **human:<name>'s call**

The falsification is not optional; how the article carries it is. Three coherent choices:

- **Option A — publish with the gap named and open.** Fold in D.1–D.3, publish as v1.0 with
  one unwitnessed transition stated plainly and the fix specified-but-unbuilt.
  *For:* it is the article's own thesis executed on itself, and the two specimens are its
  best material — a fix-issue that closed itself, and a board that was wrong with nobody
  lying. *Against:* the paper ends with a live hole in the mechanism it is describing.
- **Option B — hold publication until a witness exists at `implemented`.** Wait for §B.4
  (1)–(4), then publish a mechanism that works. *For:* the strong claim becomes true before
  it is public. *Against:* the article's whole argument is that a register of live defects is
  more trustworthy than a polished mechanism; waiting for green before publishing is the
  behaviour the paper criticises, and F-05 is already published as an open wound.
- **Option C — publish A now, ship the witness, amend to v1.1.** Explicit "as of 2026-08-02"
  dating on the gap; the fix lands later as a documented amendment.
  *For:* fastest, and the amendment is itself a demonstration. *Against:* a published article
  is cached and indexed; the v1.0 text with the gap is permanent regardless of the amendment.

**Recommendation: Option A**, and treat B as the one to reject explicitly. The article
already publishes F-05 — a register deletion caught by luck — as its opening. An article that
leads with a week-one falsification and then withholds publication over a week-four one is
applying two standards, and the second one is the flattering one. A and C differ only in
whether an amendment is pre-announced; A with a dated gap is simpler and promises less.

Whichever human:<name> picks, **one thing is not an option:** the 2026-07-10 three-layer sentence
cannot be folded in as written (see the falsification marker on that addendum).

### F. Two mechanical notes

- **Citation IDs.** The article cites findings as `F-05`/`F-08`/`F-09` (legacy numeric). This
  finding's ID is the slug `F-impl-claims-unproven` (register-slug-ids, methodology/35), and
  the registers now live in `assay-toolkit`, not oit. The article's citation convention needs
  one decision — keep prose-only descriptions, or cite slugs — before D.1–D.3 land.
- **The Verify table's own exposure.** This brief's Verify rows are presence greps, already
  flagged as such by F-10 — and F-impl-claims-unproven's third case (oit#1047) is a Verify
  table that *fails as literally written*. Row 10 below is a presence gate too, and inherits
  the same limitation; it is not offered as more than it is.

### Verify — additional row (this addendum)

| # | Command | Expect |
|---|---------|--------|
| 10 | `grep -c "F-impl-claims-unproven\|unwitnessed\|nothing checking it" docs/articles/status-build-artifact.md` (run in the oit checkout) | ≥1 — the `implemented`-transition gap is named in the article, under whichever option §E resolves to. Row added 2026-08-02; **not yet run** — the article is unamended pending human:<name>'s call, so this row is expected to FAIL until §E is decided and D.1–D.3 (or their Option-B equivalent) land. |
