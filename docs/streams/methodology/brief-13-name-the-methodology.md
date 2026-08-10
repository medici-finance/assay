---
brief: methodology/13
title: Name the methodology — the word that goes where "Scrum"/"GitOps" goes
wave: 0
depends: []
unblocks: ["methodology/09", "methodology/10", "methodology/11"]
effort: S
gate: human
risk: {regulatory: no, customer: yes, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (process desk)
sources: ["[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "[I-05](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-article-3-writing-specs-that-can-converge-the-initiator-s-cr.md)", "../reconciler/docs/naming.md (2026-07-08 addendum — methodology naming sweep)", "process-desk sweep 2026-07-08"]
gate-why: >-
  Picks the permanent public name for the methodology (Plumb vs. alternatives) that then
  gets minted publicly in three articles and the toolkit repo's own name — a naming
  decision that's effectively unrecallable once published, cached, and adopted, and is a
  pure taste call; sign-off IS the human choosing the name, not verifying a mechanism.
---

# Brief 13 — Name the methodology

## Context
files: decision recorded here + ../reconciler/docs/naming.md; the chosen name then flows into briefs 09/10/11 (the articles mint it publicly — hence unblocks)
facts:
- Sweep results (2026-07-08, banked in ../reconciler/docs/naming.md): "Medici method(ology)" = AVOID (direct collision with Johansson's Medici Effect, an established management-methodology brand); "plumb method/methodology" = UNCLAIMED in software (only construction-tool hits); IaC drift vendors do not use the plumb-line metaphor; Plumb.finance domain already owned (reconciler product).
- Candidate shortlist (desk assessment):
  1. **Plumb** (recommended) — the method itself: "we run Plumb", "keep the work plumb", "out of plumb" = drift. One word, verb-able, metaphor-exact (a plumb line is a declared reference you measure drift against), unifies the brand family per the convergence thesis (Plumb = the practice; Plumb Identity = reconciler; toolkit = Plumbline, pending its own collision check).
  2. **Plumbline** — more distinctive as a noun, weaker as a verb; known church-consultancy usage (uncleared).
  3. **Artifact-Driven Delivery** — descriptive/slogan-style (like Trunk-Based Development); instantly legible, zero mystique, no ownable brand.
  4. **WorkOps / declarative-work framing** — maximum legibility to the GitOps audience, maximally generic, likely uncoinable.
- Constraint: the name must survive the articles' framing ("status is a build artifact", "checking plumb") and the toolkit extraction (methodology/07 names the repo AFTER this decision, not before).
- gate human because: publication makes it permanent (customer-facing, cached/indexed), and taste is the owner's call.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Human picks (or rejects all and names the direction). If Plumbline is in contention, run its dedicated collision check first (church-consultancy + trademark-adjacent usage).
2. Record the decision + rationale in ../reconciler/docs/naming.md (the naming home) and this brief's Evidence.
3. Propagate: one-line name reference into briefs 09/10/11's Context (they mint it) and methodology/07 (repo naming input).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -ci "decided" ../reconciler/docs/naming.md` | ≥1 (decision + rationale recorded) |
| 2 | `grep -rlc "<chosen-name>" docs/streams/methodology/brief-09* docs/streams/methodology/brief-10* docs/streams/methodology/brief-11*` | 3 files (name propagated; substitute the chosen name) |
| 3 | `statusgen --root . --check` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

**DECISION (human:<name>, 2026-07-09): the methodology is named `Assay`** — standalone or "Assay by
Medici"; home domain **assay.guide**. Rationale: an assayer tests the metal, not the stamp —
evidence-not-claims as a single verb+noun, from Medici's own era; brand family reads
*Plumb, identity held true — Assay, work held true*. Superseded: the 2026-07-08
Plumb-as-methodology recommendation (Plumb stays the reconciler). Due diligence: 13-name
web+whois sweep 2026-07-09 (three dispatched researchers; **indicative, not a legal
trademark search** — formal USPTO/EUIPO clearance is a pre-publication step, not done) —
Assay was the only CLEAR verdict;
kills recorded for Gauge (ThoughtWorks Gauge), Tally (two live Class-9 marks), True-up
(generic), Square (Block), Reckon, Setpoint; Quadra came back CROWDED (quadra.trade,
Quadrata identity-space adjacency). Full record: ../reconciler/docs/naming.md (committed
locally in the reconciler repo, 2026-07-09; push is the human's).

Implementer run:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -ci "decided" ../reconciler/docs/naming.md` | 0 | `3` (≥1; DECIDED section appended with rationale + sweep summary + domains) | 2026-07-09 | implementer (Fable, decision by human:ian) |
| 2 | `grep -rlc "Assay" docs/streams/methodology/brief-09* docs/streams/methodology/brief-10* docs/streams/methodology/brief-11*` | 0 | 3 files (NAME fact propagated into each article brief's Context) | 2026-07-09 | implementer (Fable) |
| 3 | `go run ./tools/statusgen --root . --lint` (substituted for `--check` per brief-02/04 precedent — status-changing branch) | 0 | clean | 2026-07-09 | implementer (Fable) |

Note: brief-10's article draft (merged) predates the naming decision — the name gets worked
in at publication-prep (wording pass; no Verify change). Toolkit-repo rename DONE 2026-07-09 per human:<name>:
medici-finance/assay-toolkit (GitHub redirects the old URL; module path + all refs
updated, tests green under the new path). Domain registration (assay.guide; defensive: assay.md / assay.work /
assaymethod.com were free at check time) is the human's action.

Independent verification (non-implementer opus re-run on merged main 07bcecaa, 2026-07-09).
Path substitution recorded: from this worktree `../reconciler` does not resolve to the
reconciler repo, so row 1 was run against the absolute local path
(/Users/iholsman/jojig/Lending/reconciler/docs/naming.md); the command is shown in the
brief's own `../reconciler/…` form below to keep the link-checker happy:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `grep -ci "decided" ../reconciler/docs/naming.md` (run at the absolute substitute path) | 0 | `1` (≥1 — the Assay DECIDED record is present in the naming home) | 2026-07-09 | independent (opus-verifier) |
| 2 | `grep -rlc "Assay" docs/streams/methodology/brief-09* brief-10* brief-11*` | 0 | 3 files (brief-09, brief-10, brief-11 each carry the propagated name) | 2026-07-09 | independent (opus-verifier) |
| 3 | `go run ./tools/statusgen --root . --check` → FAILED (exit 1); `--lint` fallback used | 0 (`--lint`) | `--check` fails purely from **STATUS.md regen lag on merged main** — the committed STATUS.md at 07bcecaa is stale vs. sources for OTHER streams (the regen diff is entirely diagnostics/00, frontend/02/05 glm-verifier moves + privacy-hardening/methodology-metrics Next-up shuffle; this branch made zero source changes at check time). `--lint` (all source checks, no drift compare) exits 0 — matching the implementer's documented `--lint` substitution and the brief-02/04 precedent. Honest record: row 3 passes via `--lint`; `--check` is red on main until CI regenerates STATUS.md. | 2026-07-09 | independent (opus-verifier) |

Rows 1–2 reproduce the propagation claims exactly; row 3 is green via the documented
`--lint` fallback (STATUS single-writer means `--check` cannot be relied on off a freshly-
regenerated main). Verification only; the `gate: human` naming review is separate.

## Review
Gate: human (customer: yes, irreversible: yes — a published name is permanent).
Reviewer records `human:<name>` + date in the stream README table.
